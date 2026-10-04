// internal/provider/anthropic_test.go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

// sseServer replies with body as an event stream and captures the request.
func sseServer(t *testing.T, status int, body string, got *map[string]any, hdr *http.Header) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, got)
		}
		if hdr != nil {
			*hdr = r.Header.Clone()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}

func keyCred(k string) CredentialFunc {
	return func(context.Context) (Credential, error) { return Credential{Token: k}, nil }
}

const anthropicToolStream = `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":100,"cache_creation_input_tokens":5,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"SIG"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Let me "}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"look."}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: content_block_start
data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"read","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}

event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"a.go\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":2}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":42}}

event: message_stop
data: {"type":"message_stop"}

`

func TestAnthropicStreamToolCall(t *testing.T) {
	var body map[string]any
	var hdr http.Header
	srv := sseServer(t, 200, anthropicToolStream, &body, &hdr)
	defer srv.Close()
	m := Model{Provider: "anthropic", ID: "claude-x", MaxOutput: 32000, ThinkingMode: "adaptive", ThinkingLevelMap: anthropicAdaptive}
	a := newAnthropic(m, srv.URL, keyCred("K"), srv.Client())

	var text strings.Builder
	var calls []*llm.ToolCall
	resp, err := a.Stream(context.Background(), llm.Request{
		Model: "claude-x", System: "sys", MaxTokens: 16384, Effort: llm.EffortMedium, ToolChoice: llm.ToolChoiceAuto,
		Tools:    []llm.ToolSpec{{Name: "read", Description: "d", Schema: json.RawMessage(`{"type":"object"}`)}},
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}},
	}, func(e llm.Event) {
		switch e.Type {
		case llm.EventText:
			text.WriteString(e.Text)
		case llm.EventToolCall:
			calls = append(calls, e.ToolCall)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "Let me look." || len(calls) != 1 || string(calls[0].Input) != `{"path":"a.go"}` {
		t.Fatalf("text %q calls %+v", text.String(), calls)
	}
	if resp.Stop != llm.StopToolUse {
		t.Fatalf("stop %s", resp.Stop)
	}
	if resp.Usage != (llm.Usage{Input: 10, Output: 42, CacheRead: 100, CacheWrite: 5}) {
		t.Fatalf("usage %+v", resp.Usage)
	}
	th := resp.Message.Content[0]
	if th.Type != llm.BlockThinking || th.Text != "hmm" || th.Signature != "SIG" || th.Model != "claude-x" {
		t.Fatalf("thinking block %+v", th)
	}
	// request shape
	if hdr.Get("x-api-key") != "K" || hdr.Get("anthropic-version") != "2023-06-01" {
		t.Fatal("auth headers")
	}
	if body["stream"] != true || body["max_tokens"].(float64) != 16384 {
		t.Fatalf("body %v", body)
	}
	th2 := body["thinking"].(map[string]any)
	if th2["type"] != "adaptive" || th2["display"] != "summarized" {
		t.Fatalf("thinking %v", th2)
	}
	if body["output_config"].(map[string]any)["effort"] != "medium" {
		t.Fatal("effort")
	}
	if _, has := th2["budget_tokens"]; has {
		t.Fatal("adaptive must not send budget_tokens")
	}
	sys := body["system"].([]any)[0].(map[string]any)
	if sys["cache_control"] == nil {
		t.Fatal("system breakpoint")
	}
	tools := body["tools"].([]any)
	if tools[len(tools)-1].(map[string]any)["cache_control"] == nil {
		t.Fatal("tools breakpoint")
	}
}

func TestAnthropicBudgetAndNoCache(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", &body, nil)
	defer srv.Close()
	m := Model{Provider: "anthropic", ID: "haiku", MaxOutput: 64000, ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget}
	a := newAnthropic(m, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "haiku", MaxTokens: 16384, Effort: llm.EffortHigh, NoCacheWrite: true,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "x"}}}}}, func(llm.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	th := body["thinking"].(map[string]any)
	if th["type"] != "enabled" || th["budget_tokens"].(float64) != 12288 {
		t.Fatalf("budget thinking %v", th)
	}
	if strings.Contains(mustJSON(body), "cache_control") {
		t.Fatal("NoCacheWrite must remove every cache_control")
	}
	// effort off on a budget model that supports off → no thinking field
	body = nil
	a.Stream(context.Background(), llm.Request{Model: "haiku", MaxTokens: 1000, Effort: llm.EffortOff,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "x"}}}}}, func(llm.Event) {})
	if _, has := body["thinking"]; has {
		t.Fatal("off → omit thinking")
	}
}

func TestAnthropicHistoryMapping(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "event: message_stop\ndata: {}\n\n", &body, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "t", Signature: "S", Model: "m"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x/", IsError: true}}}},
	}
	a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10, Messages: msgs}, func(llm.Event) {})
	got := mustJSON(body["messages"])
	for _, want := range []string{`"type":"thinking"`, `"signature":"S"`, `"type":"tool_use"`, `"id":"c1"`,
		`"type":"tool_result"`, `"tool_use_id":"c1"`, `"is_error":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("messages missing %s: %s", want, got)
		}
	}
}

func TestAnthropicErrors(t *testing.T) {
	srv := sseServer(t, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 210000 tokens > 200000 maximum"}}`, nil, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	if !errors.Is(err, ErrContextOverflow) {
		t.Fatalf("want overflow, got %v", err)
	}
	srv2 := sseServer(t, 200, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n", nil, nil)
	defer srv2.Close()
	a2 := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv2.URL, keyCred("K"), srv2.Client())
	_, err = a2.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 529 {
		t.Fatalf("in-stream overloaded → 529, got %v", err)
	}
}

const anthropicRedactedStream = `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"redacted_thinking","data":"OPAQUE-PAYLOAD"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}

event: message_stop
data: {"type":"message_stop"}

`

// Redacted thinking carries its opaque payload in `data`, complete at
// content_block_start. It must be captured and replayed verbatim as
// redacted_thinking — replaying it as an empty thinking block is a 400.
func TestAnthropicRedactedThinking(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, anthropicRedactedStream, &body, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	resp, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 100,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}}}}, func(llm.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	th := resp.Message.Content[0]
	if th.Type != llm.BlockThinking || !th.Redacted || th.Signature != "OPAQUE-PAYLOAD" || th.Model != "m" {
		t.Fatalf("redacted capture %+v", th)
	}
	body = nil
	if _, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 100,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
			{Role: llm.RoleAssistant, Content: []llm.ContentBlock{th}},
		}}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	got := mustJSON(body["messages"])
	if !strings.Contains(got, `"type":"redacted_thinking"`) || !strings.Contains(got, `"data":"OPAQUE-PAYLOAD"`) {
		t.Fatalf("redacted replay missing payload: %s", got)
	}
	if strings.Contains(got, `"signature":"OPAQUE-PAYLOAD"`) || strings.Contains(got, "Reasoning redacted") {
		t.Fatalf("redacted block replayed with the wrong shape: %s", got)
	}
}

func TestAnthropicTruncatedStream(t *testing.T) {
	// A clean EOF without message_stop is a truncated turn, not a success.
	srv := sseServer(t, 200, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n"+
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"+
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"cut\"}}\n\n", nil, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10}, func(llm.Event) {})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("want io.ErrUnexpectedEOF, got %v", err)
	}
}

func TestAnthropicBodyHygiene(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "event: message_stop\ndata: {}\n\n", &body, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())

	// A message reduced to nothing (empty thinking block) is skipped, not sent
	// as `content: []`; the breakpoint lands on the last real message.
	a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10, Messages: []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockThinking}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "again"}}},
	}}, func(llm.Event) {})
	msgs := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("empty message not skipped: %s", mustJSON(msgs))
	}
	last := msgs[1].(map[string]any)["content"].([]any)
	if last[0].(map[string]any)["cache_control"] == nil {
		t.Fatalf("breakpoint must land on the last real message: %s", mustJSON(msgs))
	}

	// Thinking blocks cannot carry cache_control: with a thinking block last,
	// the breakpoint walks back to the previous eligible block.
	body = nil
	a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10, Messages: []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockText, Text: "done"},
			{Type: llm.BlockThinking, Text: "t", Signature: "S", Model: "m"},
		}},
	}}, func(llm.Event) {})
	msgs = body["messages"].([]any)
	last = msgs[len(msgs)-1].(map[string]any)["content"].([]any)
	if last[0].(map[string]any)["cache_control"] == nil {
		t.Fatalf("breakpoint must walk back to the text block: %s", mustJSON(last))
	}
	if last[1].(map[string]any)["cache_control"] != nil {
		t.Fatalf("thinking block must not carry cache_control: %s", mustJSON(last))
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// Effort must change the thinking budget, and a model whose max_tokens cannot
// hold the 1024 minimum budget plus answer room must not get thinking at all
// (budget_tokens >= max_tokens is a 400).
func TestAnthropicBudgetFollowsEffort(t *testing.T) {
	budget := func(max int, e llm.Effort) (float64, bool) {
		var body map[string]any
		srv := sseServer(t, 200, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", &body, nil)
		defer srv.Close()
		m := Model{ID: "haiku", MaxOutput: 64000, ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget}
		a := newAnthropic(m, srv.URL, keyCred("K"), srv.Client())
		if _, err := a.Stream(context.Background(), llm.Request{Model: "haiku", MaxTokens: max, Effort: e,
			Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "x"}}}}}, func(llm.Event) {}); err != nil {
			t.Fatal(err)
		}
		th, ok := body["thinking"].(map[string]any)
		if !ok {
			return 0, false
		}
		return th["budget_tokens"].(float64), true
	}
	prev := 0.0
	for _, e := range []llm.Effort{llm.EffortLow, llm.EffortMedium, llm.EffortHigh, llm.EffortMax} {
		b, ok := budget(16384, e)
		if !ok || b <= prev || b < 1024 || b >= 16384 {
			t.Fatalf("effort %s: budget %v (prev %v) must rise with effort and stay in [1024, max_tokens)", e, b, prev)
		}
		prev = b
	}
	for _, max := range []int{500, 1024, 2047} {
		if b, ok := budget(max, llm.EffortMax); ok {
			t.Fatalf("max_tokens %d cannot hold a thinking budget, got %v", max, b)
		}
	}
	if b, ok := budget(2048, llm.EffortMax); !ok || b != 1024 {
		t.Fatalf("smallest thinking-capable window: budget %v ok %v", b, ok)
	}
}

// Thinking is replayed verbatim only to the exact model that produced it and
// only with a signature. Anything else degrades to labelled text (or nothing
// for redacted/empty blocks) instead of a 400 on every later turn.
func TestAnthropicForeignThinking(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", &body, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "claude", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	if _, err := a.Stream(context.Background(), llm.Request{Model: "claude", MaxTokens: 10, Messages: []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "from glm", Model: "glm"},                               // other model, unsigned
			{Type: llm.BlockThinking, Text: "from other", Signature: "FOREIGN-SIG", Model: "other"}, // other model, signed
			{Type: llm.BlockThinking, Text: "[Reasoning redacted]", Signature: "FOREIGN-DATA", Model: "other", Redacted: true},
			{Type: llm.BlockThinking, Text: "unsigned same model", Model: "claude"},       // cannot replay
			{Type: llm.BlockThinking, Text: "mine", Signature: "MY-SIG", Model: "claude"}, // replayable
			{Type: llm.BlockText, Text: "answer"},
		}},
	}}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	got := mustJSON(body["messages"])
	for _, bad := range []string{`"signature":""`, "FOREIGN-SIG", "FOREIGN-DATA", "redacted_thinking"} {
		if strings.Contains(got, bad) {
			t.Errorf("foreign thinking leaked %s: %s", bad, got)
		}
	}
	for _, want := range []string{`"signature":"MY-SIG"`, "[prior reasoning]\\nfrom glm", "[prior reasoning]\\nfrom other", "[prior reasoning]\\nunsigned same model"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s: %s", want, got)
		}
	}
	if strings.Count(got, `"type":"thinking"`) != 1 {
		t.Errorf("only the same-model signed block replays as thinking: %s", got)
	}
}

// A tool_use cut off by max_tokens is kept with its assembled arguments: the
// phase-2 loop attaches the §6 "cut off — split the work" error result for
// every call of a StopLength turn, and the tool registry rejects invalid JSON
// with its own error result. A block that never stopped keeps a nil input.
func TestAnthropicTruncatedToolUseKept(t *testing.T) {
	for name, tail := range map[string]string{
		"never stopped":   "",
		"stopped invalid": "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			srv := sseServer(t, 200, `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t1","name":"read"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"/a"}}

`+tail+`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`, nil, nil)
			defer srv.Close()
			a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
			var calls []*llm.ToolCall
			resp, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10}, func(e llm.Event) {
				if e.Type == llm.EventToolCall {
					calls = append(calls, e.ToolCall)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Stop != llm.StopLength {
				t.Fatalf("stop %s, want length", resp.Stop)
			}
			var kept []llm.ContentBlock
			for _, c := range resp.Message.Content {
				if c.Type == llm.BlockToolUse {
					kept = append(kept, c)
				}
			}
			if len(kept) != 1 || kept[0].ToolCall.ID != "t1" || kept[0].ToolCall.Name != "read" {
				t.Fatalf("truncated call must be kept for the phase-2 error result: %+v", resp.Message.Content)
			}
			switch name {
			case "never stopped":
				if kept[0].ToolCall.Input != nil || len(calls) != 0 {
					t.Fatalf("never-stopped call: input %q, emitted %d", kept[0].ToolCall.Input, len(calls))
				}
			case "stopped invalid":
				if string(kept[0].ToolCall.Input) != `{"path":"/a` || json.Valid(kept[0].ToolCall.Input) || len(calls) != 1 {
					t.Fatalf("stopped call: input %q emitted %d", kept[0].ToolCall.Input, len(calls))
				}
			}
		})
	}
}

// History that already holds a tool_use with missing or malformed input must
// still serialize (json.Marshal fails on an invalid RawMessage, which would
// break every later turn) and must not send `"input":null`.
func TestAnthropicHistoryToolInputSanitized(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", &body, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10, Messages: []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "a", Name: "ls"}},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "b", Name: "ls", Input: json.RawMessage(`{"path":"/a`)}},
		}},
	}}, func(llm.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	got := mustJSON(body["messages"])
	if strings.Contains(got, `"input":null`) || strings.Count(got, `"input":{}`) != 2 {
		t.Fatalf("tool input not sanitized: %s", got)
	}
}

func TestAnthropicCredentialHeaders(t *testing.T) {
	var hdr http.Header
	srv := sseServer(t, 200, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", nil, &hdr)
	defer srv.Close()
	cred := func(context.Context) (Credential, error) {
		return Credential{Token: "K", Headers: map[string]string{"x-opencode-session": "s1"}}, nil
	}
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, cred, srv.Client())
	if _, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 16}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	if hdr.Get("x-opencode-session") != "s1" {
		t.Fatalf("credential headers must be applied: %v", hdr)
	}
}

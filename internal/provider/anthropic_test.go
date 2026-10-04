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
	if th["type"] != "enabled" || th["budget_tokens"].(float64) != 16384-4096 {
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

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

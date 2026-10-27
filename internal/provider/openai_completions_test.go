// internal/provider/openai_completions_test.go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

const completionsStream = `data: {"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"think"}}]}

data: {"choices":[{"index":0,"delta":{"content":"Hi "}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read","arguments":"{\"pa"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"ls","arguments":"{}"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"x\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: {"choices":[],"usage":{"prompt_tokens":120,"completion_tokens":30,"prompt_tokens_details":{"cached_tokens":100}}}

data: [DONE]

`

// Every protocol without explicit cache breakpoints sends the prompt as one
// string: the head and the environment tail rejoin here (§14), byte for byte
// as the builder rendered them.
func TestCompletionsJoinsTheSystemTail(t *testing.T) {
	a := newOpenAICompletions(Model{ID: "glm"}, "http://x/v1", keyCred("K"), http.DefaultClient).(*completionsAdapter)
	body := a.body(llm.Request{Model: "glm", System: "head", SystemTail: "# Environment\n- Date: d"})
	msgs := body["messages"].([]map[string]any)
	if len(msgs) != 1 || msgs[0]["role"] != "system" || msgs[0]["content"] != "head\n\n# Environment\n- Date: d" {
		t.Fatalf("system message %v", msgs)
	}
}

func TestCompletionsStream(t *testing.T) {
	var body map[string]any
	var hdr http.Header
	srv := sseServer(t, 200, completionsStream, &body, &hdr)
	defer srv.Close()
	m := Model{ID: "glm", ThinkingMode: "openai", ThinkingLevelMap: glmThinking}
	a := newOpenAICompletions(m, srv.URL, keyCred("K"), srv.Client())
	var calls []string
	resp, err := a.Stream(context.Background(), llm.Request{Model: "glm", System: "sys", MaxTokens: 100, Effort: llm.EffortHigh,
		ToolChoice: llm.ToolChoiceNone,
		Tools:      []llm.ToolSpec{{Name: "read", Description: "d", Schema: json.RawMessage(`{"type":"object"}`)}},
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}}},
	}, func(e llm.Event) {
		if e.Type == llm.EventToolCall {
			calls = append(calls, e.ToolCall.ID+":"+string(e.ToolCall.Input))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != `call_a:{"path":"x"},call_b:{}` {
		t.Fatalf("calls in index order: %v", calls)
	}
	if resp.Stop != llm.StopToolUse || resp.Usage != (llm.Usage{Input: 20, Output: 30, CacheRead: 100}) {
		t.Fatalf("stop %s usage %+v", resp.Stop, resp.Usage)
	}
	if resp.Message.Content[0].Type != llm.BlockThinking || resp.Message.Content[1].Text != "Hi " {
		t.Fatalf("content %+v", resp.Message.Content)
	}
	if hdr.Get("Authorization") != "Bearer K" {
		t.Fatal("bearer")
	}
	if body["reasoning_effort"] != "high" || body["tool_choice"] != "none" || body["max_tokens"].(float64) != 100 {
		t.Fatalf("body %v", body)
	}
	if body["stream_options"].(map[string]any)["include_usage"] != true {
		t.Fatal("include_usage")
	}
	if body["messages"].([]any)[0].(map[string]any)["role"] != "system" {
		t.Fatal("system message first")
	}
}

func TestCompletionsHistoryMapping(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "data: [DONE]\n\n", &body, nil)
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1, Messages: []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "secret"},
			{Type: llm.BlockText, Text: "ok"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{"path":"."}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{
			{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "a/"}},
			{Type: llm.BlockText, Text: "steer"}}},
	}}, func(llm.Event) {})
	got := mustJSON(body["messages"])
	if strings.Contains(got, "secret") {
		t.Fatal("thinking must not be sent on completions")
	}
	for _, want := range []string{`"tool_calls":[{"function":{"arguments":"{\"path\":\".\"}","name":"ls"},"id":"c1","type":"function"}]`,
		`{"content":"a/","role":"tool","tool_call_id":"c1"}`, `{"content":"steer","role":"user"}`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestCompletionsTruncatedToolCall(t *testing.T) {
	// A call cut off by max_tokens is kept with its partial arguments: the
	// phase-2 loop attaches the §6 error result for every call of a
	// StopLength turn, and the registry reports the invalid JSON.
	srv := sseServer(t, 200, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_a\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"{\\\"path\\\":\\\"/a\"}}]}}]}\n\n"+
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n", nil, nil)
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	var calls []*llm.ToolCall
	resp, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(e llm.Event) {
		if e.Type == llm.EventToolCall {
			calls = append(calls, e.ToolCall)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stop != llm.StopLength {
		t.Fatalf("truncated tool call must keep stop=length, got %s", resp.Stop)
	}
	if len(calls) != 1 || string(calls[0].Input) != `{"path":"/a` || json.Valid(calls[0].Input) {
		t.Fatalf("truncated call must be emitted with its partial arguments: %+v", calls)
	}
	if len(resp.Message.Content) != 1 || resp.Message.Content[0].Type != llm.BlockToolUse {
		t.Fatalf("truncated call must be kept for the phase-2 error result: %+v", resp.Message.Content)
	}
}

func TestCompletionsHistoryToolInputSanitized(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "data: [DONE]\n\n", &body, nil)
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1, Messages: []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "a", Name: "ls"}},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "b", Name: "ls", Input: json.RawMessage(`{"path":"/a`)}},
		}},
	}}, func(llm.Event) {})
	got := mustJSON(body["messages"])
	if strings.Contains(got, `"arguments":""`) || strings.Count(got, `"arguments":"{}"`) != 2 {
		t.Fatalf("tool input not sanitized: %s", got)
	}
}

func TestCompletionsEffortOffIsExplicit(t *testing.T) {
	// kimi: omitting reasoning_effort leaves server-side thinking on, so
	// `off` must put an explicit value on the wire. The glm variants became
	// thinking-only on 2026-10-16 — `off` is not in their maps (there is
	// nothing to send; the effort clamps to low/minimal upstream).
	m := Model{ID: "kimi", ThinkingMode: "openai", ThinkingLevelMap: kimiThinking}
	a := &completionsAdapter{m: m}
	b := a.body(llm.Request{Model: "kimi", Effort: llm.EffortOff})
	if v, _ := b["reasoning_effort"].(string); v != "none" {
		t.Fatalf("effort off sent %v, want none: %v", v, b)
	}
	if v := a.body(llm.Request{Model: "kimi", Effort: llm.EffortHigh})["reasoning_effort"]; v != "high" {
		t.Fatalf("high → %v", v)
	}
	if v := a.body(llm.Request{Model: "glm", Effort: llm.EffortLow})["reasoning_effort"]; v != "low" {
		t.Fatalf("glm low → %v", v)
	}
}

func TestCompletionsTruncatedStream(t *testing.T) {
	srv := sseServer(t, 200, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"cut\"}}]}\n\n", nil, nil)
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("clean EOF without finish_reason/[DONE] must be an error, got %v", err)
	}
}

func TestCompletionsOverflow(t *testing.T) {
	srv := sseServer(t, 400, `{"error":{"code":"context_length_exceeded","message":"maximum context length is 8192"}}`, nil, nil)
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	if !errors.Is(err, ErrContextOverflow) {
		t.Fatal(err)
	}
}

// TestCompletionsCacheKeyOnlyForOpenAI: prompt_cache_key routes repeat
// prefixes to the same cache on api.openai.com; other OpenAI-compatible
// servers may reject unknown fields, so they never get it.
func TestCompletionsCacheKeyOnlyForOpenAI(t *testing.T) {
	m := Model{ID: "m", ThinkingMode: "none"}
	req := llm.Request{Model: "m", CacheKey: "moca-ab12cd34"}
	official := newOpenAICompletions(m, "https://api.openai.com/v1", nil, nil).(*completionsAdapter)
	if v := official.body(req)["prompt_cache_key"]; v != "moca-ab12cd34" {
		t.Fatalf("openai: %v", v)
	}
	other := newOpenAICompletions(m, "https://opencode.ai/zen/go/v1", nil, nil).(*completionsAdapter)
	if _, ok := other.body(req)["prompt_cache_key"]; ok {
		t.Fatal("a compatible server must not get prompt_cache_key")
	}
	if _, ok := official.body(llm.Request{Model: "m"})["prompt_cache_key"]; ok {
		t.Fatal("no key, no field")
	}
}

// TestCompletionsReasoningObjectMode: the "effort-object" thinking mode sends
// OpenRouter's canonical reasoning:{effort} object — including "max", which the
// reasoning_effort alias does not document — and never the alias itself.
func TestCompletionsReasoningObjectMode(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	var body map[string]any
	srv := sseServer(t, 200, stream, &body, nil)
	defer srv.Close()

	obj := Model{ID: "x", ThinkingMode: "effort-object", ThinkingLevelMap: map[llm.Effort]string{
		llm.EffortMax: "max", llm.EffortLow: "low"}}
	a := newOpenAICompletions(obj, srv.URL, keyCred("K"), srv.Client())
	if _, err := a.Stream(context.Background(), llm.Request{Model: "x", MaxTokens: 10, Effort: llm.EffortMax}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	got, ok := body["reasoning"].(map[string]any)
	if !ok || got["effort"] != "max" {
		t.Fatalf("reasoning object: %v", body["reasoning"])
	}
	if _, has := body["reasoning_effort"]; has {
		t.Fatal("object mode must not send the alias")
	}

	// A second capture: sseServer decodes into the same map, so reusing it
	// would keep the object mode's keys around.
	var body2 map[string]any
	srv2 := sseServer(t, 200, stream, &body2, nil)
	defer srv2.Close()
	alias := Model{ID: "y", ThinkingMode: "openai", ThinkingLevelMap: map[llm.Effort]string{llm.EffortHigh: "high"}}
	a2 := newOpenAICompletions(alias, srv2.URL, keyCred("K"), srv2.Client())
	if _, err := a2.Stream(context.Background(), llm.Request{Model: "y", MaxTokens: 10, Effort: llm.EffortHigh}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	if body2["reasoning_effort"] != "high" {
		t.Fatalf("openai mode uses the alias: %v", body2["reasoning_effort"])
	}
	if _, has := body2["reasoning"]; has {
		t.Fatal("openai mode must not send the object")
	}
}

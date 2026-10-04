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
	// glm/kimi: omitting reasoning_effort leaves server-side thinking on, so
	// `off` must put an explicit value on the wire.
	m := Model{ID: "glm", ThinkingMode: "openai", ThinkingLevelMap: glmThinking}
	a := &completionsAdapter{m: m}
	b := a.body(llm.Request{Model: "glm", Effort: llm.EffortOff})
	if v, _ := b["reasoning_effort"].(string); v == "" {
		t.Fatalf("effort off sent no reasoning_effort: %v", b)
	}
	if v := a.body(llm.Request{Model: "glm", Effort: llm.EffortHigh})["reasoning_effort"]; v != "high" {
		t.Fatalf("high → %v", v)
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

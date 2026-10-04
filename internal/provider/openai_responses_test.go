// internal/provider/openai_responses_test.go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

const responsesStream = `event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_1","delta":"plan"}

event: response.output_item.done
data: {"type":"response.output_item.done","item":{"type":"reasoning","id":"rs_1","encrypted_content":"ENC","summary":[{"type":"summary_text","text":"plan"}]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hello"}

event: response.output_item.added
data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_9","name":"read","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"path\":\"a\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_9","name":"read","arguments":"{\"path\":\"a\"}"}}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":50,"output_tokens":7,"input_tokens_details":{"cached_tokens":40}}}}

`

func TestResponsesStream(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, responsesStream, &body, nil)
	defer srv.Close()
	m := Model{ID: "gpt", ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning}
	a := newOpenAIResponses(m, srv.URL, keyCred("K"), srv.Client())
	var calls []*llm.ToolCall
	resp, err := a.Stream(context.Background(), llm.Request{Model: "gpt", System: "sys", MaxTokens: 500, Effort: llm.EffortLow,
		Tools:    []llm.ToolSpec{{Name: "read", Description: "d", Schema: json.RawMessage(`{"type":"object"}`)}},
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}}},
	}, func(e llm.Event) {
		if e.Type == llm.EventToolCall {
			calls = append(calls, e.ToolCall)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].ID != "call_9" || string(calls[0].Input) != `{"path":"a"}` {
		t.Fatalf("calls %+v", calls)
	}
	th := resp.Message.Content[0]
	if th.Type != llm.BlockThinking || th.ThinkingID != "rs_1" || th.Signature != "ENC" || th.Text != "plan" {
		t.Fatalf("reasoning %+v", th)
	}
	if resp.Stop != llm.StopToolUse || resp.Usage != (llm.Usage{Input: 10, Output: 7, CacheRead: 40}) {
		t.Fatalf("stop %s usage %+v", resp.Stop, resp.Usage)
	}
	if body["store"] != false || body["instructions"] != "sys" || body["max_output_tokens"].(float64) != 500 {
		t.Fatalf("body %v", body)
	}
	if body["reasoning"].(map[string]any)["effort"] != "low" {
		t.Fatal("effort")
	}
	if !strings.Contains(mustJSON(body["include"]), "reasoning.encrypted_content") {
		t.Fatal("include encrypted reasoning")
	}
}

// When the final reasoning item carries no summary, the streamed
// reasoning_summary_text.delta text must still land in the persisted block.
func TestResponsesReasoningSummaryFallback(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, `event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_9","delta":"part1 "}

event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_9","delta":"part2"}

event: response.output_item.done
data: {"type":"response.output_item.done","item":{"type":"reasoning","id":"rs_9","encrypted_content":"E2","summary":[]}}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":1}}}

`, &body, nil)
	defer srv.Close()
	a := newOpenAIResponses(Model{ID: "gpt", ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning}, srv.URL, keyCred("K"), srv.Client())
	resp, err := a.Stream(context.Background(), llm.Request{Model: "gpt", MaxTokens: 10, Effort: llm.EffortLow}, func(llm.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	th := resp.Message.Content[0]
	if th.Type != llm.BlockThinking || th.Text != "part1 part2" || th.ThinkingID != "rs_9" || th.Signature != "E2" {
		t.Fatalf("summary fallback %+v", th)
	}
}

func TestResponsesTruncatedStream(t *testing.T) {
	srv := sseServer(t, 200, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"cut\"}\n\n", nil, nil)
	defer srv.Close()
	a := newOpenAIResponses(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10}, func(llm.Event) {})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("clean EOF without response.completed must be an error, got %v", err)
	}
}

func TestResponsesHistoryAndIncomplete(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, `event: response.incomplete
data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":1,"output_tokens":1}}}

`, &body, nil)
	defer srv.Close()
	a := newOpenAIResponses(Model{ID: "gpt", ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning}, srv.URL, keyCred("K"), srv.Client())
	resp, err := a.Stream(context.Background(), llm.Request{Model: "gpt", MaxTokens: 1, Effort: llm.EffortLow, Messages: []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, ThinkingID: "rs_1", Signature: "ENC", Model: "gpt"},
			{Type: llm.BlockText, Text: "ok"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x"}}}},
	}}, func(llm.Event) {})
	if err != nil || resp.Stop != llm.StopLength {
		t.Fatalf("incomplete → length: %v %s", err, resp.Stop)
	}
	got := mustJSON(body["input"])
	for _, want := range []string{`"type":"reasoning"`, `"encrypted_content":"ENC"`, `"id":"rs_1"`,
		`"type":"output_text"`, `"type":"function_call"`, `"call_id":"c1"`, `"type":"function_call_output"`} {
		if !strings.Contains(got, want) {
			t.Errorf("input missing %s: %s", want, got)
		}
	}
}

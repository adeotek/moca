package provider

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/test/wire"
)

// thinkingHistory is one neutral conversation: same-model reasoning, a text
// block, a call with its result and a nudge (the user message grows), then
// another model's reasoning. The exact rendering differs per codec; what must
// hold is that encoding a longer prefix never changes the encoding of the
// shorter one.
func thinkingHistory(q string) []llm.Message {
	call := func(id string) *llm.ToolCall {
		return &llm.ToolCall{ID: id, Name: "read", Input: json.RawMessage(`{"path":"a"}`)}
	}
	return []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "a"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "t1", Model: q, Signature: "sig-1", ThinkingID: "rs_1"},
			{Type: llm.BlockText, Text: "hi"},
			{Type: llm.BlockToolUse, ToolCall: call("c1")},
		}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{
			{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x"}},
			{Type: llm.BlockText, Text: "nudge"},
		}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "t2", Model: "other/model"},
			{Type: llm.BlockToolUse, ToolCall: call("c2")},
		}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{
			{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c2", Content: "y"}},
		}},
	}
}

func requestFor(m Model, h []llm.Message) llm.Request {
	return llm.Request{Model: m.ID, System: "system text", Messages: h, MaxTokens: 1000,
		Effort: llm.EffortHigh, ToolChoice: llm.ToolChoiceAuto,
		Tools: []llm.ToolSpec{{Name: "read", Description: "d", Schema: json.RawMessage(`{"type":"object"}`)}}}
}

// TestCodecBodiesPreservePrefix: for all three codecs, every prefix of one
// history encodes to a body whose system block, tool block and message list are
// an append-only prefix of the full history's body (cache_control excluded:
// moving a breakpoint is not an invalidator).
func TestCodecBodiesPreservePrefix(t *testing.T) {
	t.Run("anthropic", func(t *testing.T) {
		m := Model{Provider: "anthropic", ID: "claude-x", MaxOutput: 32000,
			ThinkingMode: "adaptive", ThinkingLevelMap: anthropicAdaptive}
		a := newAnthropic(m, "https://api.anthropic.com", keyCred("K"), nil).(*anthropicAdapter)
		checkPrefix(t, m, func(r llm.Request) map[string]any { return a.body(r) },
			func(b map[string]any) []any { return b["messages"].([]any) },
			func(b map[string]any) any { return b["system"] })
	})
	t.Run("completions", func(t *testing.T) {
		m := Model{Provider: "opencode-go", ID: "glm-x", Protocol: "openai-completions",
			ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning}
		a := newOpenAICompletions(m, "https://gateway.test/v1", keyCred("K"), nil).(*completionsAdapter)
		checkPrefix(t, m, func(r llm.Request) map[string]any { return a.body(r) },
			func(b map[string]any) []any { return b["messages"].([]any) },
			func(b map[string]any) any { return b["messages"].([]any)[0] })
	})
	t.Run("responses", func(t *testing.T) {
		m := Model{Provider: "openai", ID: "gpt-x", Protocol: "openai-responses", ThinkingMode: "openai"}
		a := newOpenAIResponses(m, "https://api.openai.com/v1", keyCred("K"), nil).(*responsesAdapter)
		checkPrefix(t, m, func(r llm.Request) map[string]any { return a.body(r, false) },
			func(b map[string]any) []any { return b["input"].([]any) },
			func(b map[string]any) any { return b["instructions"] })
	})
}

func checkPrefix(t *testing.T, m Model, encode func(llm.Request) map[string]any,
	msgsOf func(map[string]any) []any, sysOf func(map[string]any) any) {
	t.Helper()
	h := thinkingHistory(m.Qualified())
	for k := 1; k < len(h); k++ {
		short := normalizeBody(t, encode(requestFor(m, h[:k])))
		full := normalizeBody(t, encode(requestFor(m, h)))
		if !reflect.DeepEqual(wire.Strip(sysOf(short)), wire.Strip(sysOf(full))) {
			t.Fatalf("k=%d: the system block changed with the history length", k)
		}
		if !reflect.DeepEqual(wire.Strip(short["tools"]), wire.Strip(full["tools"])) {
			t.Fatalf("k=%d: the tool block changed with the history length", k)
		}
		if msg, blk := wire.FirstDivergence(wire.Strip(msgsOf(short)).([]any), wire.Strip(msgsOf(full)).([]any)); msg != -1 {
			t.Fatalf("k=%d: encoding rewrote list item %d block %d", k, msg, blk)
		}
	}
}

// normalizeBody round-trips a body through JSON: adapters build typed slices
// ([]map[string]any), and only decoded JSON gives the []any/any shapes the
// wire helpers compare.
func normalizeBody(t *testing.T, b map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

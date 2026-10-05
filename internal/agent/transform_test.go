package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestTransformHistory(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "deep thought", Signature: "SIG", Model: "anthropic/claude-x"},
			{Type: llm.BlockThinking, Text: "", Signature: "OMITTED", Model: "anthropic/claude-x"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "toolu_01:weird/id", Name: "ls", Input: json.RawMessage(`{}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "toolu_01:weird/id", Content: "x"}}}},
	}
	same := TransformHistory(msgs, "anthropic/claude-x")
	if same[1].Content[0].Signature != "SIG" || len(same[1].Content) != 3 {
		t.Fatal("same model: thinking replayed unchanged")
	}
	other := TransformHistory(msgs, "opencode-go/glm-5.3")
	c := other[1].Content
	if len(c) != 2 || c[0].Type != llm.BlockText || c[0].Text != "[prior reasoning]\ndeep thought" {
		t.Fatalf("other model: %+v", c)
	}
	id := c[1].ToolCall.ID
	if id != other[2].Content[0].ToolResult.CallID || strings.ContainsAny(id, ":/") || len(id) > 40 {
		t.Fatalf("ids normalized consistently: %q", id)
	}
	if msgs[1].Content[0].Signature != "SIG" || msgs[1].Content[2].ToolCall.ID != "toolu_01:weird/id" {
		t.Fatal("input must not be mutated (transcript keeps originals)")
	}
}

func TestTransformDropsEmptiedAssistantAndMerges(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "a"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockThinking, Model: "x/y"}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "b"}}},
	}
	out := TransformHistory(msgs, "p/q")
	if len(out) != 1 || len(out[0].Content) != 2 {
		t.Fatalf("empty assistant dropped, users merged: %+v", out)
	}
}

// A thinking block from a different provider sharing the same bare id must not
// replay verbatim: the comparison is on the qualified id.
func TestTransformSameBareIDDifferentProvider(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "theirs", Signature: "SIG", Model: "other/claude-x"}}},
	}
	out := TransformHistory(msgs, "anthropic/claude-x")
	if out[0].Content[0].Type != llm.BlockText || out[0].Content[0].Text != "[prior reasoning]\ntheirs" {
		t.Fatalf("foreign-provider thinking must degrade: %+v", out[0].Content)
	}
}

func TestNormalizeToolID(t *testing.T) {
	if NormalizeToolID("call_abc-1") != "call_abc-1" {
		t.Fatal("valid ids unchanged")
	}
	long := strings.Repeat("a", 41)
	if n := NormalizeToolID(long); len(n) > 40 || n != NormalizeToolID(long) {
		t.Fatal("deterministic and bounded")
	}
}

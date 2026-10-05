package compact

import (
	"encoding/json"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestEstimate(t *testing.T) {
	if Tokens(0) != 0 || Tokens(1) != 1 || Tokens(8) != 2 {
		t.Fatal("chars/4 rounded up")
	}
	m := llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
		{Type: llm.BlockText, Text: "abcd"},
		{Type: llm.BlockThinking, Text: "efgh"},
		{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{Name: "ls", Input: json.RawMessage(`{}`)}},
		{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{Content: "123456"}},
	}}
	if MessageChars(m) != 4+4+2+2+6 {
		t.Fatal(MessageChars(m))
	}
	if RequestChars("sys", []llm.ToolSpec{{Name: "a", Description: "bb", Schema: json.RawMessage(`{}`)}}, []llm.Message{m}) != 3+1+2+2+18 {
		t.Fatal("request chars")
	}
	if UsageTokens(llm.Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}) != 10 {
		t.Fatal("usage")
	}
}

package session

// Pin for T-6: a session file with an entry whose payload is missing (a
// truncated write, a hand-edited file, an older schema) must not panic on
// resume. The three rebuild paths — Messages, Repair, NewSnapshots — all walk
// the same entries and used to dereference the payload unconditionally.

import (
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestRebuildPathsSurvivePayloadLessEntries(t *testing.T) {
	entries := []Entry{
		{Type: TypeMessage},    // no Message
		{Type: TypeToolUse},    // no ToolUse
		{Type: TypeToolResult}, // no ToolResult
		{Type: TypeMessage, Message: &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("rebuild panicked on a payload-less entry: %v", r)
		}
	}()

	msgs := Messages(entries)
	if len(msgs) != 1 {
		t.Fatalf("want the one real message, got %d", len(msgs))
	}
	// Repair returns only the synthetic results it would append; a payload-less
	// tool_use contributes nothing (and must not dereference).
	if fixed := Repair(entries, "test"); len(fixed) != 0 {
		t.Fatalf("Repair invented %d results for entries with no call", len(fixed))
	}
	NewSnapshots(nil, t.TempDir(), entries) // a *Writer is nil-tolerant for a read-only walk

	// Control: a real unanswered call still gets its synthetic result.
	answered := Repair([]Entry{{Type: TypeToolUse, ToolUse: &ToolUse{Call: llm.ToolCall{ID: "c1", Name: "shell"}}}}, "test")
	if len(answered) != 1 || answered[0].ToolResult.CallID != "c1" {
		t.Fatalf("Repair lost a real unanswered call: %+v", answered)
	}
}

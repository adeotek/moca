package session

import (
	"encoding/json"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func txt(r llm.Role, s string) *llm.Message {
	return &llm.Message{Role: r, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: s}}}
}

func TestMessagesRebuild(t *testing.T) {
	es := []Entry{
		{ID: "s", Type: TypeSession},
		{ID: "u1", Type: TypeMessage, Message: txt(llm.RoleUser, "q")},
		{ID: "a1", Type: TypeMessage, Message: txt(llm.RoleAssistant, "looking")},
		{ID: "t1", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{}`)}}},
		{ID: "t2", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c2", Name: "ls", Input: json.RawMessage(`{}`)}}},
		{ID: "r1", Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x"}},
		{ID: "sn", Type: TypeSnapshot, Snapshot: &SnapshotRec{Path: "/f"}},
		{ID: "r2", Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c2", Content: "y"}},
		{ID: "u2", Type: TypeMessage, Message: txt(llm.RoleUser, "steer")},
		{ID: "a2", Type: TypeMessage, Message: txt(llm.RoleAssistant, "done")},
	}
	m := Messages(es)
	if len(m) != 4 {
		t.Fatalf("want user, assistant, user, assistant; got %d", len(m))
	}
	if len(m[1].Content) != 3 || m[1].Content[2].ToolCall.ID != "c2" {
		t.Fatalf("assistant + 2 tool_use: %+v", m[1].Content)
	}
	if len(m[2].Content) != 3 || m[2].Content[0].ToolResult.CallID != "c1" || m[2].Content[2].Text != "steer" {
		t.Fatalf("results then steering text: %+v", m[2].Content)
	}
}

func TestMessagesFromCompaction(t *testing.T) {
	es := []Entry{
		{ID: "u1", Type: TypeMessage, Message: txt(llm.RoleUser, "old")},
		{ID: "a1", Type: TypeMessage, Message: txt(llm.RoleAssistant, "old answer")},
		{ID: "u2", Type: TypeMessage, Message: txt(llm.RoleUser, "kept question")},
		{ID: "a2", Type: TypeMessage, Message: txt(llm.RoleAssistant, "kept answer")},
		{ID: "cp", Type: TypeCompaction, Compaction: &Compaction{Summary: "S", FirstKeptEntryID: "u2"}},
		{ID: "u3", Type: TypeMessage, Message: txt(llm.RoleUser, "new")},
	}
	m := Messages(es)
	if len(m) != 3 || m[0].Content[0].Text != "[Summary of earlier conversation]\nS" || m[0].Content[1].Text != "kept question" {
		t.Fatalf("%+v", m)
	}
	if m[2].Content[0].Text != "new" {
		t.Fatalf("entries after the cut rebuild unchanged: %+v", m[2])
	}
	if c, i := LatestCompaction(es); c == nil || i != 4 || c.Summary != "S" {
		t.Fatal("latest compaction")
	}
	if c, i := LatestCompaction(es[:3]); c != nil || i != -1 {
		t.Fatal("no compaction → nil, -1")
	}
}

func TestRepair(t *testing.T) {
	es := []Entry{
		{ID: "a1", Type: TypeMessage, Message: txt(llm.RoleAssistant, "")},
		{ID: "t1", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c1"}}},
		{ID: "t2", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c2"}}},
		{ID: "r1", Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "ok"}},
	}
	fix := Repair(es, Interrupted)
	if len(fix) != 1 || fix[0].ToolResult.CallID != "c2" || fix[0].ToolResult.Content != Interrupted || !fix[0].ToolResult.IsError {
		t.Fatalf("%+v", fix)
	}
	if len(Repair(append(es, fix...), Interrupted)) != 0 {
		t.Fatal("idempotent")
	}
}

// TestMessagesWithAppendsPerEntry: the extra hook runs per user message entry
// and its blocks land right after that entry's own blocks, before a later
// entry merges in — the property that makes the plan envelope append-only
// across toggles (B1.1).
func TestMessagesWithAppendsPerEntry(t *testing.T) {
	extra := func(e Entry) []llm.ContentBlock {
		if !e.Plan {
			return nil
		}
		return []llm.ContentBlock{{Type: llm.BlockText, Text: "[env]"}}
	}
	es := []Entry{
		{ID: "r1", Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x"}},
		{ID: "u1", Type: TypeMessage, Plan: true, Message: txt(llm.RoleUser, "a")},
		{ID: "u2", Type: TypeMessage, Plan: true, Message: txt(llm.RoleUser, "b")},
	}
	m := MessagesWith(es, extra, false)
	if len(m) != 1 || m[0].Role != llm.RoleUser || len(m[0].Content) != 5 {
		t.Fatalf("merged user message: %+v", m)
	}
	if m[0].Content[0].Type != llm.BlockToolResult || m[0].Content[1].Text != "a" ||
		m[0].Content[2].Text != "[env]" || m[0].Content[3].Text != "b" || m[0].Content[4].Text != "[env]" {
		t.Fatalf("per-entry placement: %+v", m[0].Content)
	}
	plain := MessagesWith([]Entry{{ID: "u", Type: TypeMessage, Message: txt(llm.RoleUser, "p")}}, extra, false)
	if len(plain) != 1 || len(plain[0].Content) != 1 {
		t.Fatalf("an unflagged entry gets no extra blocks: %+v", plain[0].Content)
	}
	if noHook := Messages(es); len(noHook) != 1 || len(noHook[0].Content) != 3 {
		t.Fatalf("Messages must equal MessagesWith(entries, nil, false): %+v", noHook[0].Content)
	}
}

// TestMessagesStripKeptTailThinking: a compaction's kept tail replays without
// its thinking blocks on a route that is not history-bound (the summary
// stands in for that stretch), while post-compaction turns keep theirs; a
// history-bound route keeps the tail verbatim — the provider binds thinking
// to the exact prefix it was produced against (B1.2).
func TestMessagesStripKeptTailThinking(t *testing.T) {
	asst := func(id, text string) Entry {
		return Entry{ID: id, Type: TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "think " + id}, {Type: llm.BlockText, Text: text}}}}
	}
	es := []Entry{
		{ID: "u1", Type: TypeMessage, Message: txt(llm.RoleUser, "go")},
		asst("a1", "tail"),
		{ID: "c1", Type: TypeCompaction, Compaction: &Compaction{Summary: "sum", FirstKeptEntryID: "a1"}},
		asst("a2", "post"),
	}
	m := Messages(es)
	if len(m) != 3 {
		t.Fatalf("summary, kept tail, post-compaction turn: %d", len(m))
	}
	if len(m[1].Content) != 1 || m[1].Content[0].Type == llm.BlockThinking {
		t.Fatalf("kept tail must drop thinking: %+v", m[1].Content)
	}
	if len(m[2].Content) != 2 || m[2].Content[0].Type != llm.BlockThinking {
		t.Fatalf("post-compaction turns keep thinking: %+v", m[2].Content)
	}
	b := MessagesWith(es, nil, true)
	if len(b[1].Content) != 2 || b[1].Content[0].Type != llm.BlockThinking {
		t.Fatalf("history-bound keeps the kept tail verbatim: %+v", b[1].Content)
	}
	if plain := Messages([]Entry{asst("a3", "no compaction")}); len(plain[0].Content) != 2 {
		t.Fatalf("without a compaction nothing is stripped: %+v", plain[0].Content)
	}
}

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

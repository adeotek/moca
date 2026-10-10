package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func TestCollect(t *testing.T) {
	t0 := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	call := llm.ToolCall{ID: "c1", Name: "shell", Input: json.RawMessage(`{"command":"go test ./..."}`)}
	call2 := call
	call2.ID = "c2"
	fail := func(id string) *llm.ToolResult {
		return &llm.ToolResult{CallID: id, Content: "FAIL\n[exit 1]", IsError: true}
	}
	asst := &llm.Message{Role: llm.RoleAssistant}
	entries := []session.Entry{
		{Type: session.TypeSession, Time: t0, Session: &session.Header{}},
		{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleUser}},
		{Type: session.TypeMessage, Message: asst, Usage: &llm.Usage{Input: 100, Output: 10}, Cost: 0.01},
		{Type: session.TypeToolUse, ToolUse: &session.ToolUse{Call: call}},
		{Type: session.TypeToolResult, ToolResult: fail("c1")},
		{Type: session.TypeMessage, Message: asst, Usage: &llm.Usage{Input: 5, CacheRead: 100, Output: 10}, Cost: 0.002},
		{Type: session.TypeToolUse, ToolUse: &session.ToolUse{Call: call2}},
		{Type: session.TypeToolResult, ToolResult: fail("c2")},
		{Type: session.TypeCompaction, Compaction: &session.Compaction{}},
		{Type: session.TypeMessage, Message: asst, Time: t0.Add(90 * time.Second)},
	}
	m := Collect(entries)
	if m.Steps != 3 || m.Calls["shell"] != 2 || m.ToolErrors != 2 || m.RepeatedFailures != 1 || m.Compactions != 1 {
		t.Fatalf("%+v", m)
	}
	if m.Tokens() != 225 || m.Seconds != 90 {
		t.Fatalf("tokens=%d seconds=%v", m.Tokens(), m.Seconds)
	}
}

func TestSummarize(t *testing.T) {
	rows := []Metrics{
		{Label: "base", Scenario: "bugfix", Pass: true, Steps: 4, Input: 1000},
		{Label: "base", Scenario: "bugfix", Pass: false, Steps: 6, Input: 3000},
	}
	out := Summarize(rows)
	if !strings.Contains(out, "| base | bugfix | 2 | 1/2 | 2000 ± 1000 | 5.0 ± 1.0 |") || !strings.Contains(out, "| 4000 |") {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(out, "∑ all") {
		t.Fatalf("total row missing: %s", out)
	}
}

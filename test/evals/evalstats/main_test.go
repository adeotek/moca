package main

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/compact"
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

// --- instrumented metrics (A1.2) ---

func asstUsage(in, cr int) session.Entry {
	return session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant},
		Usage: &llm.Usage{Input: in, CacheRead: cr}}
}

func toolUse(id, name, input string) session.Entry {
	return session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{
		Call: llm.ToolCall{ID: id, Name: name, Input: json.RawMessage(input)}}}
}

func toolResult(id, callID, content string) session.Entry {
	return session.Entry{ID: id, Type: session.TypeToolResult,
		ToolResult: &llm.ToolResult{CallID: callID, Content: content}}
}

// readContent renders a read result: numbered lines plus the footer.
func readContent(from, to, total int) string {
	var sb strings.Builder
	for n := from; n <= to; n++ {
		fmt.Fprintf(&sb, "%d|line %d\n", n, n)
	}
	fmt.Fprintf(&sb, "[lines %d-%d of %d]\n", from, to, total)
	return sb.String()
}

func TestCollectCacheAttribution(t *testing.T) {
	entries := []session.Entry{
		{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleUser}},
		asstUsage(1000, 0),
		asstUsage(200, 1000),
		toolUse("c1", "read", `{"path":"a"}`),
		toolResult("r1", "c1", "x"),
		{ID: "e1", Type: session.TypeElision, Elision: &session.Elision{IDs: []string{"r1"}, Tokens: 10}},
		asstUsage(1000, 500),
	}
	m := Collect(entries)
	if m.FirstPrompt != 1000 || m.FirstCacheRead != 0 {
		t.Fatalf("first request = (%d, %d), want (1000, 0)", m.FirstPrompt, m.FirstCacheRead)
	}
	// (1000 + 500) / (min(1000,1200) + min(1200,1500))
	if math.Abs(m.CacheEff-1500.0/2200.0) > 0.001 {
		t.Fatalf("cache eff = %v, want %v", m.CacheEff, 1500.0/2200.0)
	}
	want := CacheBreak{Step: 3, Reusable: 1200, Read: 500, After: "tool_use,tool_result,elision"}
	if len(m.CacheBreaks) != 1 || !reflect.DeepEqual(m.CacheBreaks[0], want) {
		t.Fatalf("cache breaks = %+v, want [%+v]", m.CacheBreaks, want)
	}
}

func TestCollectResidency(t *testing.T) {
	a := strings.Repeat("x", 400) // 100 tokens
	b := strings.Repeat("y", 800) // 200 tokens
	entries := []session.Entry{
		{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleUser}},
		asstUsage(10, 0),
		toolUse("c1", "read", `{"path":"a"}`),
		toolResult("r1", "c1", a),
		asstUsage(10, 10), // A live
		asstUsage(10, 10), // A live
		toolUse("c2", "read", `{"path":"b"}`),
		toolResult("r2", "c2", b),
		asstUsage(10, 10), // A + B live
	}
	m := Collect(entries)
	if m.Residency["read"] != 500 {
		t.Fatalf("residency[read] = %d, want 500", m.Residency["read"])
	}
}

func TestCollectRedundantReads(t *testing.T) {
	first := readContent(1, 100, 900)
	second := readContent(10, 14, 900)
	base := func(extra ...session.Entry) []session.Entry {
		out := []session.Entry{
			toolUse("c1", "read", `{"path":"a.go"}`),
			toolResult("r1", "c1", first),
		}
		out = append(out, extra...)
		out = append(out, toolUse("c2", "read", `{"path":"a.go","offset":10,"limit":5}`),
			toolResult("r2", "c2", second))
		return out
	}
	if m := Collect(base()); m.RedundantReadTokens != compact.Tokens(len(second)) {
		t.Fatalf("redundant = %d, want %d", m.RedundantReadTokens, compact.Tokens(len(second)))
	}
	edited := base(toolUse("c3", "edit", `{"path":"a.go"}`))
	if m := Collect(edited); m.RedundantReadTokens != 0 {
		t.Fatalf("an edit between the reads must not count: %d", m.RedundantReadTokens)
	}
	elided := base(session.Entry{ID: "e1", Type: session.TypeElision, Elision: &session.Elision{IDs: []string{"r1"}}})
	if m := Collect(elided); m.RedundantReadTokens != 0 {
		t.Fatalf("an elided earlier read must not count: %d", m.RedundantReadTokens)
	}
}

func TestCollectCrutchCounters(t *testing.T) {
	entries := []session.Entry{
		{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleUser}, Nudge: "verify"},
		{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleUser}, Nudge: "progress"},
		toolResult("r1", "c1", "refused: tests are failing — read the failing test first"),
		toolResult("r2", "c2", "FAIL [hint: tests failed: (1) read the failing test file]"),
	}
	m := Collect(entries)
	if m.Nudges["verify"] != 1 || m.Nudges["progress"] != 1 || m.EditRefusals != 1 || m.Hints != 1 {
		t.Fatalf("nudges=%v refusals=%d hints=%d", m.Nudges, m.EditRefusals, m.Hints)
	}
}

func TestSummarizeShowsCacheColumns(t *testing.T) {
	out := Summarize([]Metrics{{Label: "x", Scenario: "y", Pass: true, Input: 10, CacheRead: 90, FirstCacheRead: 0}})
	if !strings.Contains(out, "cache eff") || !strings.Contains(out, "first-hit") ||
		!strings.Contains(out, "breaks/run") || !strings.Contains(out, "redundant read tok") {
		t.Fatalf("cache columns missing: %s", out)
	}
}

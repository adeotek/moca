package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// Small-window model (32K): reserve 8192, keepRecent 8192, trigger 24576 tokens.
const smallModel = `"m":{"contextWindow":32768,"maxOutputTokens":4096}`

// The config minimum window (16K): reserve 4096, keepRecent 4096, trigger
// 12288 — small enough that one big read busts it.
const minWindowModel = `"m":{"contextWindow":16384,"maxOutputTokens":4096}`

// bigToolTurns scripts n shell turns each returning 20000 chars (~5000
// tokens); with smallModel the trigger (24576) fires exactly after the 5th
// batch. (The script server answers every agent request in order, including
// summary requests, so the follow-up turns must be planned with that.)
func bigToolTurns(n int) []string {
	var turns []string
	for range n {
		turns = append(turns, toolTurn([2]string{"shell", "{\"command\":\"head -c 20000 /dev/zero | tr '\\\\0' x\"}"}))
	}
	return turns
}

func summaryBodies(s *scriptServer) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []map[string]any
	for _, b := range s.bodies {
		msgs, _ := b["messages"].([]any)
		if len(msgs) == 0 {
			continue
		}
		m0, _ := msgs[0].(map[string]any)
		if sys, _ := m0["content"].(string); strings.Contains(sys, "## Constraints & Preferences") {
			out = append(out, b)
		}
	}
	return out
}

func TestAutoCompactionShrinksContext(t *testing.T) {
	turns := bigToolTurns(5)
	turns = append(turns, textTurn("## Goal\nsummary"), textTurn("done"))
	s := newScript(t, turns...)
	a, _, evs := startTestWith(t, s, "", smallModel)
	a.opts.Env.Commands = allowAll{}
	out, err := a.Run(context.Background(), "make output")
	if err != nil || out.Text != "done" {
		t.Fatal(out, err)
	}
	var comp *Compacted
	for _, e := range *evs {
		if c, ok := e.(Compacted); ok {
			comp = &c
		}
	}
	if comp == nil || comp.TokensAfter >= comp.TokensBefore {
		t.Fatalf("compaction must shrink context: %+v", comp)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if len(session.Repair(entries, "")) != 0 {
		t.Fatal("no orphaned tool calls")
	}
	msgs := session.Messages(entries)
	if !strings.HasPrefix(msgs[0].Content[0].Text, "[Summary of earlier conversation]") {
		t.Fatal("request starts from the summary")
	}
	// The summary request: no tools, max_tokens ≤ 4096, one structured ask.
	sums := summaryBodies(s)
	if len(sums) != 1 || sums[0]["tools"] != nil {
		t.Fatalf("summary request shape: %v", sums)
	}
	if mt, _ := sums[0]["max_tokens"].(float64); mt == 0 || mt > 4096 {
		t.Fatalf("summary max_tokens: %v", sums[0]["max_tokens"])
	}
}

func TestLoopGuardNamesOversizedEntry(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"read", `{"path":"dist/app.min.js"}`}), textTurn("## Goal\ns"), textTurn("never"))
	a, work, _ := startTestWith(t, s, "", minWindowModel)
	// One read of a big file is capped at 50K chars ≈ 12k tokens: larger
	// than keepRecent (4096) and, kept verbatim, still over the trigger
	// (12288) — the loop guard must name it instead of compacting forever.
	var sb strings.Builder
	for sb.Len() < 300_000 {
		sb.WriteString("const v000000000000000000000000 = 1;\n")
	}
	if err := os.MkdirAll(filepath.Join(work, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "dist", "app.min.js"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.Run(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "still over budget") ||
		!strings.Contains(err.Error(), "read") || !strings.Contains(err.Error(), "12k tokens") {
		t.Fatalf("%v", err)
	}
}

func TestOverflowCompactsAndRetriesOnce(t *testing.T) {
	overflow := "HTTP400:" + `{"error":{"code":"context_length_exceeded","message":"maximum context length"}}`
	// Requests: q-prompt, r-prompt, overflow; summary A (history), summary B
	// (split turn part), retry. 80K-char prompts put ~20k tokens per turn, so
	// the cut splits the turn (keepRecent 16384) and two summaries merge.
	s := newScript(t, textTurn("a1"), textTurn("a2"), overflow, textTurn("## Goal\ns"), textTurn("## Turn\ns"), textTurn("recovered"))
	a, _, _ := startTestWith(t, s, "", "")
	if _, err := a.Run(context.Background(), strings.Repeat("q", 80000)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), strings.Repeat("r", 80000)); err != nil {
		t.Fatal(err)
	}
	out, err := a.Run(context.Background(), "third")
	if err != nil || out.Text != "recovered" {
		t.Fatal(out, err)
	}
	if sums := summaryBodies(s); len(sums) != 2 {
		t.Fatalf("split-turn overflow recovery issues exactly two summaries, got %d", len(sums))
	}
	entries, _ := session.ReadFile(a.Session().Path())
	var comps, recovered int
	for _, e := range entries {
		if e.Type == session.TypeCompaction {
			comps++
		}
		if e.Type == session.TypeMessage && e.Message.Role == llm.RoleAssistant &&
			len(e.Message.Content) > 0 && e.Message.Content[0].Text == "recovered" {
			recovered++
		}
	}
	if comps != 1 || recovered != 1 {
		t.Fatalf("compactions=%d recovered=%d", comps, recovered)
	}
}

func TestOverflowRecoveryFailsWithoutHistory(t *testing.T) {
	overflow := "HTTP400:" + `{"error":{"code":"context_length_exceeded","message":"maximum context length"}}`
	s := newScript(t, overflow, textTurn("unused"))
	a, _, _ := startTestWith(t, s, "", "")
	_, err := a.Run(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("overflow with nothing to compact must surface the original error: %v", err)
	}
	// Nothing was persisted beyond the user prompt: no error entry, no
	// compaction entry, no assistant response.
	entries, _ := session.ReadFile(a.Session().Path())
	if len(entries) != 2 || entries[1].Type != session.TypeMessage {
		t.Fatalf("nothing but the prompt must be persisted: %d entries", len(entries))
	}
}

func TestManualCompactNothing(t *testing.T) {
	s := newScript(t, textTurn("hi"))
	a, _, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), "hello")
	n := len(s.bodies)
	if err := a.Compact(context.Background()); err != ErrNothingToCompact || len(s.bodies) != n {
		t.Fatal(err)
	}
}

func TestManualCompactWritesEntryUsageAndRebuild(t *testing.T) {
	turns := bigToolTurns(2)
	turns = append(turns, textTurn("early done"), textTurn("## Goal\nmanual"), textTurn("after"))
	s := newScript(t, turns...)
	a, _, _ := startTestWith(t, s, "", smallModel)
	a.opts.Env.Commands = allowAll{}
	if _, err := a.Run(context.Background(), "make output"); err != nil {
		t.Fatal(err)
	}
	u0, c0 := a.Totals()
	if err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	u1, c1 := a.Totals()
	if u1.Input <= u0.Input || u1.Output <= u0.Output || c1 < c0 {
		t.Fatalf("summary usage must count towards session totals: %+v/%v → %+v/%v", u0, c0, u1, c1)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	c, _ := session.LatestCompaction(entries)
	if c == nil || !strings.Contains(c.Summary, "manual") || c.FirstKeptEntryID == "" {
		t.Fatalf("compaction entry: %+v", c)
	}
	out, err := a.Run(context.Background(), "now")
	if err != nil || out.Text != "after" {
		t.Fatal(out, err)
	}
	last, _ := json.Marshal(s.bodies[len(s.bodies)-1])
	if !strings.Contains(string(last), "[Summary of earlier conversation]") {
		t.Fatalf("the next request must start from the summary: %s", last)
	}
}

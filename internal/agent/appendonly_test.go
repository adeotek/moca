package agent

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adeotek/moca/test/wire"
)

// TestRequestsAreAppendOnly pins the cache property the harness relies on:
// across one run, every request's message list is an append-only extension of
// the previous one. Steering and nudges may grow the last user message; nothing
// earlier may change, because a rewrite invalidates the provider's cached
// prefix (and, on preserved-thinking models, the turns after it).
func TestRequestsAreAppendOnly(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"read", `{"path":"a.txt"}`}),
		toolTurn([2]string{"write", `{"path":"x.go","content":"package x\n"}`}),
		textTurn(""),     // empty reply → the empty nudge
		textTurn("done"), // run end with x.go unverified → the verify nudge
		textTurn("all done"),
	)
	a, work, _ := startTest(t, s, 40)
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.Steer("also check README", "")
	if _, err := a.Run(context.Background(), "do it"); err != nil {
		t.Fatal(err)
	}
	bodies := snapshotBodies(s)
	if len(bodies) < 5 {
		t.Fatalf("requests = %d, want at least 5", len(bodies))
	}
	var prev []any
	for i, b := range bodies {
		raw, ok := b["messages"].([]any)
		if !ok {
			t.Fatalf("request %d has no messages array", i+1)
		}
		cur := wire.Strip(raw).([]any)
		if i > 0 {
			if m, blk := wire.FirstDivergence(prev, cur); m != -1 {
				t.Fatalf("request %d rewrote message %d block %d", i+1, m, blk)
			}
			if !reflect.DeepEqual(wire.Strip(bodies[0]["tools"]), wire.Strip(b["tools"])) {
				t.Fatalf("request %d changed the tool block", i+1)
			}
			if !reflect.DeepEqual(wire.Strip(bodies[0]["messages"].([]any)[0]), cur[0]) {
				t.Fatalf("request %d changed the system message", i+1)
			}
		}
		prev = cur
	}
}

// TestRequestsAreAppendOnlyAfterCompaction (B1.2): a compaction legitimately
// rewrites the request — the summarized prefix is dropped, and the summary
// calls themselves differ per turn — but every request after the last rewrite
// is append-only again, across a run boundary. This subtest is what would
// catch a kept-tail strip, an elision or an envelope change rewriting history
// mid-run after a compaction.
func TestRequestsAreAppendOnlyAfterCompaction(t *testing.T) {
	overflow := "HTTP400:" + `{"error":{"code":"context_length_exceeded","message":"maximum context length"}}`
	// Runs 1–2 build history; run 3's prompt overflows and the turn is split,
	// so the recovery issues two summaries and retries into a tool turn; run 4
	// then adds plain requests. Everything from the retry on must be append-only.
	s := newScript(t,
		textTurn("a1"), textTurn("a2"),
		overflow, textTurn("## Goal\ns"), textTurn("## Turn\ns"),
		toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("done"),
		toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("last"),
	)
	a, work, _ := startTestWith(t, s, "", "")
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, prompt := range []string{strings.Repeat("q", 80000), strings.Repeat("r", 80000), "third", "fourth"} {
		if _, err := a.Run(context.Background(), prompt); err != nil {
			t.Fatalf("%.10s…: %v", prompt, err)
		}
	}
	bodies := snapshotBodies(s)
	if len(bodies) < 5 {
		t.Fatalf("requests = %d, want a compaction plus several post-compaction requests", len(bodies))
	}
	var prev []any
	resets, resetAt := 0, -1
	for i, b := range bodies {
		raw, _ := b["messages"].([]any)
		cur := wire.Strip(raw).([]any)
		if i > 0 {
			if m, _ := wire.FirstDivergence(prev, cur); m != -1 {
				resets++
				resetAt = i
			}
		}
		prev = cur
	}
	if resets == 0 || resetAt < 0 {
		t.Fatal("the compaction must rewrite the request at its boundary (no rewrite seen)")
	}
	if n := len(bodies) - resetAt; n < 3 {
		t.Fatalf("want at least 3 requests after the last rewrite, got %d", n)
	}
	prev = nil
	for i := resetAt; i < len(bodies); i++ {
		raw, _ := bodies[i]["messages"].([]any)
		cur := wire.Strip(raw).([]any)
		if prev != nil {
			if m, blk := wire.FirstDivergence(prev, cur); m != -1 {
				t.Fatalf("request %d rewrote message %d block %d after the compaction", i+1, m, blk)
			}
		}
		prev = cur
	}
}

func snapshotBodies(s *scriptServer) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, len(s.bodies))
	copy(out, s.bodies)
	return out
}

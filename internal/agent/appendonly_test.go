package agent

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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
	a.Steer("also check README")
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

func snapshotBodies(s *scriptServer) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, len(s.bodies))
	copy(out, s.bodies)
	return out
}

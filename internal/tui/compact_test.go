package tui

import (
	"context"
	"strings"
	"testing"
)

// /compact on a fresh session must not send a summary request: the model
// reports nothing to compact.
func TestCompactNothing(t *testing.T) {
	m := newAgentModel(t)
	m.input.SetBuffer("/compact")
	m.syncTextarea()
	_, cmd := m.Update(key("enter"))
	if !m.compacting || cmd == nil {
		t.Fatal("compact must mark the model busy and return a command")
	}
	out, _ := simulate(m, cmd)
	if m.compacting {
		t.Fatal("the busy flag clears when the compaction finishes")
	}
	if !strings.Contains(out, "nothing to compact") {
		t.Fatalf("expected a nothing-to-compact note, got %q", out)
	}
}

// A run must not start (and /clear must not swap sessions) while a /compact
// is in flight — it appends to the transcript.
func TestCompactingBlocksRun(t *testing.T) {
	m := newAgentModel(t)
	m.input.SetBuffer("/compact")
	m.syncTextarea()
	m.Update(key("enter"))
	if !m.compacting {
		t.Fatal("setup: compact in flight")
	}
	settle(m) // flush the command echo the dropped enter cmd left in flight
	old := m.agent
	m.input.SetBuffer("hello")
	m.syncTextarea()
	_, cmd := m.Update(key("enter"))
	if m.running || !strings.Contains(printed(cmd), "/compact") {
		t.Fatalf("a run must not start while compacting: %q", printed(cmd))
	}
	m.input.SetBuffer("/clear")
	m.syncTextarea()
	m.Update(key("enter"))
	if m.agent != old {
		t.Fatal("/clear must wait for the compaction")
	}
	// A `!` note would race the compaction (excluded from its summary, or
	// dropped by the rebuilt context behind its cut boundary).
	settle(m)
	m.input.SetBuffer("!ls")
	m.syncTextarea()
	_, cmd = m.Update(key("enter"))
	if out, _ := simulate(m, cmd); m.shellBusy || !strings.Contains(out, "/compact") {
		t.Fatalf("`!` must be refused while compacting: %q", out)
	}
	m.Update(compactDoneMsg{err: nil})
	if m.compacting {
		t.Fatal("done clears the flag")
	}
}

// esc cancels an in-flight /compact; a cancelled compaction reports itself
// instead of looking like an error.
func TestCompactEscapeCancels(t *testing.T) {
	m := newAgentModel(t)
	m.input.SetBuffer("/compact")
	m.syncTextarea()
	m.Update(key("enter"))
	if !m.compacting || m.compactCancel == nil {
		t.Fatal("compact must be cancellable")
	}
	settle(m) // flush the command echo the dropped enter cmd left in flight
	cancelled := false
	m.compactCancel = func() { cancelled = true }
	m.Update(key("esc"))
	if !cancelled {
		t.Fatal("esc must cancel the compaction")
	}
	_, cmd := m.Update(compactDoneMsg{err: context.Canceled})
	if m.compacting || m.compactCancel != nil {
		t.Fatal("cancel clears the busy state")
	}
	if !strings.Contains(printed(cmd), "compaction cancelled") {
		t.Fatalf("cancelled compaction note: %q", printed(cmd))
	}
}

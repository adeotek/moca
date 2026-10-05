package tui

import (
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
	msg := cmd()
	if _, ok := msg.(compactDoneMsg); !ok {
		t.Fatalf("compact cmd returned %T", msg)
	}
	_, done := m.Update(msg)
	if m.compacting {
		t.Fatal("the busy flag clears when the compaction finishes")
	}
	if !strings.Contains(printed(done), "nothing to compact") {
		t.Fatalf("expected a nothing-to-compact note, got %q", printed(done))
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
	m.Update(compactDoneMsg{err: nil})
	if m.compacting {
		t.Fatal("done clears the flag")
	}
}

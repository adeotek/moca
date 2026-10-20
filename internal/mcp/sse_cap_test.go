package mcp

// Pin for F-4: one JSON-RPC message is bounded on both MCP HTTP paths. Before,
// a server answering with a gigabyte was read into memory whole (non-SSE), and
// an SSE event that never ended accumulated without limit — a hostile or merely
// broken server could exhaust moca's heap.

import (
	"fmt"
	"strings"
	"testing"
)

func TestReadEventsRefusesAnUnboundedEvent(t *testing.T) {
	// No blank line anywhere: one "event" that never ends.
	payload := "data: " + strings.Repeat("x", 1<<20) + "\n"
	var sb strings.Builder
	for range 9 { // 9 MB > the 8 MB cap
		sb.WriteString(payload)
	}
	called := false
	err := readEvents(strings.NewReader(sb.String()), func(string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("an unbounded SSE event must be refused")
	}
	if !strings.Contains(err.Error(), "MB") {
		t.Fatalf("the error must name the cap, got %v", err)
	}
	if called {
		t.Fatal("the oversized event must not be handed to the caller")
	}
}

func TestReadEventsStillAcceptsAMultiLineEvent(t *testing.T) {
	// Control: a normal event spanning two data lines is joined and delivered.
	var got string
	err := readEvents(strings.NewReader("data: {\"a\":1,\ndata: \"b\":2}\n\n"), func(d string) error {
		got = d
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "{\"a\":1,\n\"b\":2}" {
		t.Fatalf("joined event = %q", got)
	}
	_ = fmt.Sprint()
}

package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
)

// The event pipe must never block its caller (the event loop calls it from
// inside Update — a synchronous Program.Send there deadlocks the program),
// and it must preserve event order.
func TestEventPipeNonBlockingAndFIFO(t *testing.T) {
	release := make(chan struct{})
	rec := make(chan string, 3)
	p := newEventPipe()
	p.start(func(msg tea.Msg) {
		<-release // downstream blocked: emit must still not block
		rec <- msg.(agentEventMsg).e.(agent.TextDelta).Text
	})
	done := make(chan struct{})
	go func() {
		p.emit(agent.TextDelta{Text: "1"})
		p.emit(agent.TextDelta{Text: "2"})
		p.emit(agent.TextDelta{Text: "3"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("emit blocked on a blocked downstream send")
	}
	close(release)
	if a, b, c := <-rec, <-rec, <-rec; a+b+c != "123" {
		t.Fatalf("order: %s%s%s", a, b, c)
	}
	close(p.stop)
	p.emit(agent.TextDelta{Text: "4"}) // after stop: no-op, never blocks
}

// Events emitted before the program exists (agent.Start warnings) must be
// buffered and delivered after start — calling Send on a not-yet-created
// Program panics, so the forwarder must not run before start.
func TestEventPipeBuffersUntilStarted(t *testing.T) {
	p := newEventPipe()
	p.emit(agent.Warning{Text: "early"}) // must neither block nor panic
	rec := make(chan string, 1)
	p.start(func(msg tea.Msg) { rec <- msg.(agentEventMsg).e.(agent.Warning).Text })
	select {
	case got := <-rec:
		if got != "early" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("pre-program event was not delivered after start")
	}
	close(p.stop)
}

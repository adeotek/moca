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
	p := newEventPipe(func(msg tea.Msg) {
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

package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/tools"
)

type agentEventMsg struct{ e agent.Event }
type runDoneMsg struct {
	out agent.Outcome
	err error
}
type approvalMsg struct {
	q     tools.Question
	reply chan tools.Answer
}
type branchMsg struct {
	branch     string
	dirty, git bool
}
type shellDoneMsg struct {
	local bool
	cmd   string
	out   tools.ShellOutput
	err   error
}

// newAsker bridges the agent's blocking approval callback to the TUI: the
// question is sent to the program, the answer comes back on a channel (ctx
// cancellation — esc or quit — denies). It is called from the agent's run
// goroutine only, where a blocking Program.Send is safe.
func newAsker(send func(tea.Msg)) tools.Asker {
	return func(ctx context.Context, q tools.Question) tools.Answer {
		reply := make(chan tools.Answer, 1)
		send(approvalMsg{q: q, reply: reply})
		select {
		case a := <-reply:
			return a
		case <-ctx.Done():
			return tools.Deny
		}
	}
}

// eventPipe delivers agent events to the program without ever blocking the
// caller. Bubble Tea's Program.Send is a blocking send on an unbuffered
// channel and Update runs on the event-loop goroutine, so emitting
// synchronously from Update — /yolo's YoloChanged, /clear's agent.Start
// warnings — would deadlock the program. A forwarder goroutine owns the
// blocking send; emit only enqueues, preserving order (single FIFO channel).
type eventPipe struct {
	ch   chan agent.Event
	stop chan struct{}
}

func newEventPipe(send func(tea.Msg)) *eventPipe {
	p := &eventPipe{ch: make(chan agent.Event, 1024), stop: make(chan struct{})}
	go func() {
		for {
			select {
			case e := <-p.ch:
				send(agentEventMsg{e})
			case <-p.stop:
				return
			}
		}
	}()
	return p
}

func (p *eventPipe) emit(e agent.Event) {
	select {
	case p.ch <- e:
	case <-p.stop:
	}
}

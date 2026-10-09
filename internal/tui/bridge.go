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
type compactDoneMsg struct {
	err error
}

// loginProgressMsg carries one output line of the running OAuth flow into the
// scrollback (the flow writes from its own goroutine).
type loginProgressMsg struct{ line string }

// loginDoneMsg reports the finished OAuth flow (or the store write).
type loginDoneMsg struct {
	provider string
	email    string
	err      error
}

// logoutDoneMsg reports a finished /logout (revocation + store delete).
type logoutDoneMsg struct {
	provider string
	had      bool // something was stored
	warn     error
	err      error
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
// event loop. Bubble Tea's Program.Send is a blocking send on an unbuffered
// channel and Update runs on the event-loop goroutine, so emitting
// synchronously from Update — /yolo's YoloChanged, /clear's agent.Start
// warnings — would deadlock the program. A forwarder goroutine owns the
// blocking send; emit only enqueues, preserving order (single FIFO channel).
//
// The forwarder is gated: events emitted before start() (agent.Start's
// warnings) are buffered and delivered once the program exists — calling
// Send on a Program that has not been created yet panics.
//
// emit blocks only when the 1024-slot queue is full, which needs the event
// loop itself to be stalled; that is deliberate backpressure (dropping
// streamed text deltas would corrupt the answer, and a stalled loop cannot
// process esc either).
//
// The run's completion travels the same pipe (send): a message returned from
// a command takes its own route to the event loop and could overtake events
// still queued here, flushing a half-streamed line before its tail arrived.
type eventPipe struct {
	ch   chan tea.Msg
	stop chan struct{}
	prog chan func(tea.Msg)
}

func newEventPipe() *eventPipe {
	p := &eventPipe{ch: make(chan tea.Msg, 1024), stop: make(chan struct{}), prog: make(chan func(tea.Msg), 1)}
	go func() {
		var send func(tea.Msg)
		select {
		case send = <-p.prog:
		case <-p.stop:
			return
		}
		for {
			select {
			case msg := <-p.ch:
				send(msg)
			case <-p.stop:
				return
			}
		}
	}()
	return p
}

// start connects the pipe to the program and flushes anything buffered
// before it existed.
func (p *eventPipe) start(send func(tea.Msg)) { p.prog <- send }

func (p *eventPipe) emit(e agent.Event) { p.send(agentEventMsg{e}) }

// send enqueues any message behind everything emitted so far.
func (p *eventPipe) send(msg tea.Msg) {
	select {
	case p.ch <- msg:
	case <-p.stop:
	}
}

// modelsRefreshedMsg reports a re-read of the local server's models (the
// /model picker opens after it).
type modelsRefreshedMsg struct{ err error }

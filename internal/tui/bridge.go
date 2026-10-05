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
// cancellation — esc or quit — denies).
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

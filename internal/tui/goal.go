package tui

// /goal (v0.8.0): one bounded objective per TUI session, worked by the agent
// and ended by an evidence reviewer. The TUI owns only the command and the
// notices — the state machine and the review loop live in internal/agent, so
// ACP (which advertises no goal surface) and a process resume never continue
// a goal on their own.

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/session"
)

// goalNoticeWidth caps the one-line objective in the start notice.
const goalNoticeWidth = 90

// runGoal serves /goal: bare status, the three controls, or a new objective
// (anything else is an objective, not an alias).
func (m *model) runGoal(args string) tea.Cmd {
	if m.agent == nil {
		return nil
	}
	arg := strings.TrimSpace(args)
	if arg == "" {
		return m.printGoalStatus()
	}
	switch strings.ToLower(arg) {
	case "pause":
		return m.goalPause()
	case "resume":
		return m.goalResume()
	case "clear":
		return m.goalClear()
	}
	// A new objective: start and resume are between-runs actions.
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	if g, ok := m.agent.Goal(); ok && (g.Status == session.GoalActive || g.Status == session.GoalPaused) {
		return m.println(fmt.Sprintf("a goal is already set (%s) — /goal pause to stop it, or /goal clear to replace it", g.Status))
	}
	a := m.agent
	return m.startRunWith("/goal "+arg, func(ctx context.Context) (agent.Outcome, error) {
		return a.StartGoal(ctx, arg)
	})
}

// goalPause pauses the goal. Allowed mid-run (the driver re-reads the state
// after its review); refused while a compaction or a `!` command owns the
// transcript.
func (m *model) goalPause() tea.Cmd {
	if m.compacting {
		return m.println("a /compact is still running — wait for it to finish")
	}
	if m.shellBusy {
		return m.refuseBusy()
	}
	if _, err := m.agent.PauseGoal(); err != nil {
		return m.printlnError("error: " + err.Error())
	}
	return nil // the GoalChanged notice prints
}

// goalResume re-activates a paused goal with a fresh round/stall budget and
// starts a round — the only way goal work restarts after a pause or a
// restart.
func (m *model) goalResume() tea.Cmd {
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	a := m.agent
	return m.startRunWith("/goal resume", func(ctx context.Context) (agent.Outcome, error) {
		return a.ResumeGoal(ctx)
	})
}

// goalClear removes the goal state (the transcript keeps its snapshots).
func (m *model) goalClear() tea.Cmd {
	if m.compacting {
		return m.println("a /compact is still running — wait for it to finish")
	}
	if m.shellBusy {
		return m.refuseBusy()
	}
	if err := m.agent.ClearGoal(); err != nil {
		return m.printlnError("error: " + err.Error())
	}
	return nil
}

// printGoalStatus is bare /goal: objective, status, rounds and the fixed
// pause reason, then the session's usage.
func (m *model) printGoalStatus() tea.Cmd {
	g, ok := m.agent.Goal()
	if !ok {
		return m.printlnMuted("no goal — /goal <objective> starts one (worked in bounded rounds, ended by an evidence reviewer)")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "goal %s · rounds %d/%d · stalls %d/%d", g.Status, g.Rounds, agent.GoalMaxRounds, g.Stalls, agent.GoalMaxStalls)
	if g.Reason != "" {
		b.WriteString("\nreason: " + Sanitize(g.Reason))
	}
	b.WriteString("\nobjective: " + Sanitize(g.Objective))
	st := m.agent.Status()
	u := st.Usage
	cost := fmt.Sprintf("$%.4f", st.Cost)
	if st.Sub {
		cost = "subscription"
	} else if !st.Model.CostKnown {
		cost = "cost unknown"
	}
	fmt.Fprintf(&b, "\nsession usage: in %s · out %s · %s",
		FmtTokens(u.Input+u.CacheRead+u.CacheWrite), FmtTokens(u.Output), cost)
	return m.printlnContent(b.String())
}

// goalNotice renders one committed transition. The objective is sanitized
// and cut to one line: it is user text and can be long.
func (m *model) goalNotice(g session.Goal) tea.Cmd {
	switch g.Status {
	case session.GoalActive:
		if g.Rounds == 0 {
			obj := []rune(strings.Join(strings.Fields(g.Objective), " "))
			if len(obj) > goalNoticeWidth {
				obj = append(obj[:goalNoticeWidth], '…')
			}
			return m.printlnContent(fmt.Sprintf("goal started: %s — bounded rounds, an evidence reviewer decides when it is complete (/goal pause|clear)", Sanitize(string(obj))))
		}
		return m.printlnMuted(fmt.Sprintf("goal round %d/%d", g.Rounds, agent.GoalMaxRounds))
	case session.GoalPaused:
		if g.Reason == "" {
			return m.printlnMuted("goal paused — /goal resume continues, /goal clear removes it")
		}
		return m.printlnContent(fmt.Sprintf("goal paused: %s", Sanitize(g.Reason)))
	case session.GoalComplete:
		return m.printlnContent("goal complete — the evidence reviewer found the objective's checks satisfied")
	case session.GoalCleared:
		return m.printlnMuted("goal cleared")
	}
	return nil
}

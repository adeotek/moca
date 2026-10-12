package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
)

// nextMode is the Shift+Tab cycle: default → auto → plan (auto + plan
// scope) → yolo → default. A plan state reached another way (/plan in
// default) moves to the next position after plan: yolo. Yolo never carries
// plan (the agent keeps them mutually exclusive).
func nextMode(m agent.PermMode, plan bool) (agent.PermMode, bool) {
	switch {
	case plan:
		return agent.ModeYolo, false
	case m == agent.ModeDefault:
		return agent.ModeAuto, false
	case m == agent.ModeAuto:
		return agent.ModeAuto, true
	}
	return agent.ModeDefault, false // yolo
}

// cycleMode is shift+tab: the next stop of the permission-mode cycle
// (default → auto → plan → yolo → default), refused where /mode is. The plan
// flag and the mode are applied in an order that never passes through a
// wider state: plan is set before the mode moves (auto+plan), and entering
// yolo drops plan inside SetMode (yolo and plan never hold together).
func (m *model) cycleMode() tea.Cmd {
	if m.agent == nil {
		return nil
	}
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	mode, plan := nextMode(m.agent.Mode(), m.agent.Plan())
	if plan && !m.agent.Plan() {
		m.agent.SetPlan(true)
	}
	if m.agent.Mode() != mode {
		m.agent.SetMode(mode) // entering yolo turns plan off first
	}
	return nil
}

// modeBadgeText is the plain badge text for a mode (styled in Task 10): the
// permission state is always visible — `default` keeps its lowercase form so
// it reads as a word, risky modes shout.
func modeBadgeText(mode agent.PermMode, plan bool) string {
	s := "default"
	switch mode {
	case agent.ModeAuto:
		s = "AUTO"
	case agent.ModeYolo:
		s = "YOLO"
	}
	if plan {
		s += " · PLAN"
	}
	return s
}

// modeNotice is the one-line ModeChanged notice for a state.
func modeNotice(mode agent.PermMode, plan bool) (string, bool) {
	switch {
	case mode == agent.ModeYolo:
		// Checked before plan: the red yolo warning must never be masked.
		return "mode: YOLO — ALL permission checks are off (shift+tab to leave)", true
	case plan:
		return "mode: plan — runs write a plan to docs/plans/, nothing else changes", false
	case mode == agent.ModeAuto:
		return "mode: auto — safe actions run, risky ones ask (workdir + temp only)", false
	default:
		return "mode: default — every non-allowlisted command asks", false
	}
}

// modeCycleHelp is the /mode cycle line: the current state and the next
// shift+tab stop.
func modeCycleHelp(m agent.PermMode, plan bool) string {
	next, nextPlan := nextMode(m, plan)
	return fmt.Sprintf("cycle (shift+tab): %s → %s", modeBadgeText(m, plan), modeBadgeText(next, nextPlan))
}

// modeNames lists the words /mode accepts.
var modeNames = []string{string(agent.ModeDefault), string(agent.ModeAuto), string(agent.ModeYolo)}

// runMode is the /mode built-in: no args → the current mode + the cycle;
// `default|auto|yolo` → set it (between runs). Plan is left as-is.
func (m *model) runMode(args string) tea.Cmd {
	st := m.agent.Status()
	if args == "" {
		line, _ := modeNotice(m.agent.Mode(), st.Plan)
		return m.printlnContent(line + "\n" + modeCycleHelp(m.agent.Mode(), st.Plan))
	}
	// Refuse before parsing, like /effort does: mid-run an invalid name gets
	// the refusal, not a usage line.
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	if !slices.Contains(modeNames, args) {
		return m.println("usage: /mode default|auto|yolo — no args shows the current mode")
	}
	m.agent.SetMode(agent.PermMode(args)) // ModeChanged prints + refreshes
	return nil
}

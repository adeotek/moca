package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
)

// nextMode is the Shift+Tab cycle: ask → auto → plan → yolo → ask.
// Plan rides only on ask (the agent keeps plan out of auto/yolo), so the plan
// stop toggles the plan flag and lands at the ask mode, the stop after plan is
// yolo (plan drops first), and yolo wraps to ask.
func nextMode(m agent.PermMode, plan bool) (agent.PermMode, bool) {
	switch {
	case plan:
		return agent.ModeYolo, false
	case m == agent.ModeAsk:
		return agent.ModeAuto, false
	case m == agent.ModeAuto:
		return agent.ModeAsk, true // auto → plan
	}
	return agent.ModeAsk, false // yolo → ask
}

// cycleMode is shift+tab: the next stop of the permission-mode cycle
// (default → auto → plan → yolo → default), refused where /mode is. The
// plan flag moves through SetPlan — entering plan leaves a wider mode for
// default itself (the agent's rule), and leaving plan is the plain toggle
// before SetMode(yolo) — so no state passes through plan-under-auto/yolo
// en route.
func (m *model) cycleMode() tea.Cmd {
	if m.agent == nil {
		return nil
	}
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	mode, plan := nextMode(m.agent.Mode(), m.agent.Plan())
	if plan != m.agent.Plan() {
		m.agent.SetPlan(plan) // lands on default either way (the agent's rule)
	}
	if m.agent.Mode() != mode {
		m.agent.SetMode(mode)
	}
	return nil
}

// modeBadgeText is the plain badge text for a mode: a Title-Case word per
// state — `Ask` (v0.7.2's name for the old `Default`), `Auto`, `Yolo`; plan is
// its own badge (an ask run under the plan scope).
func modeBadgeText(mode agent.PermMode, plan bool) string {
	if plan {
		return "Plan"
	}
	switch mode {
	case agent.ModeAuto:
		return "Auto"
	case agent.ModeYolo:
		return "Yolo"
	}
	return "Ask"
}

// modeNotice is the one-line ModeChanged notice for a state.
func modeNotice(mode agent.PermMode, plan bool) (string, bool) {
	switch {
	case mode == agent.ModeYolo:
		// Checked before plan: the red yolo warning must never be masked.
		return "mode: Yolo — all permission checks are off (shift+tab to leave)", true
	case plan:
		return "mode: Plan — runs write a plan to docs/plans/, nothing else changes", false
	case mode == agent.ModeAuto:
		return "mode: Auto — safe actions run, risky ones ask (workdir + temp only)", false
	default:
		return "mode: Ask — every non-allowlisted command asks", false
	}
}

// modeCycleHelp is the /mode cycle line: the current state and the next
// shift+tab stop.
func modeCycleHelp(m agent.PermMode, plan bool) string {
	next, nextPlan := nextMode(m, plan)
	return fmt.Sprintf("cycle (shift+tab): %s → %s", modeBadgeText(m, plan), modeBadgeText(next, nextPlan))
}

// modeNames lists the words /mode accepts — the same set as the --mode flag
// (plan is the scope; it lands at ask, the agent's rule). The legacy "default"
// is accepted too: it is what every earlier release printed, and a muscle
// memory should not get a usage line.
var modeNames = []string{string(agent.ModeAsk), string(agent.ModeAuto), string(agent.ModeYolo), "plan", agent.LegacyModeName}

// runMode is the /mode built-in: no args → the current mode + the cycle;
// `ask|auto|yolo|plan` → set it (between runs); the legacy `default` is
// accepted as `ask`. `plan` enters the plan scope (landing at ask, even from a
// wider mode); `ask` (or `default`) exits plan; auto/yolo drop it through
// SetMode.
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
		return m.println("usage: /mode ask|auto|yolo|plan — no args shows the current mode")
	}
	if args == "plan" {
		m.agent.SetPlan(true) // ModeChanged prints + refreshes
		return nil
	}
	if m.agent.Plan() && (args == string(agent.ModeAsk) || args == agent.LegacyModeName) {
		m.agent.SetPlan(false) // /mode ask exits plan; auto/yolo drop it in SetMode
	}
	m.agent.SetMode(agent.PermMode(args)) // ModeChanged prints + refreshes
	return nil
}

package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
)

// nextMode is the Shift+Tab cycle: default → auto → yolo → plan → default.
// Plan rides only on default (the agent keeps plan out of auto/yolo), so
// the plan stop toggles the plan flag and lands at the default mode, and
// the stop after plan wraps around.
func nextMode(m agent.PermMode, plan bool) (agent.PermMode, bool) {
	switch {
	case plan:
		return agent.ModeYolo, false
	case m == agent.ModeDefault:
		return agent.ModeAuto, false
	case m == agent.ModeAuto:
		return agent.ModeDefault, true
	}
	return agent.ModeDefault, false // yolo → default
}

// cycleMode is shift+tab: the next stop of the permission-mode cycle
// (default → auto → yolo → plan → default), refused where /mode is. The
// plan flag moves through SetPlan — entering plan leaves a wider mode for
// default itself (the agent's rule), and leaving plan is the plain toggle
// — so no state passes through plan-under-auto/yolo en route.
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

// modeBadgeText is the plain badge text for a mode (styled in Task 10): a
// Title-Case word per state — `Default` reads as a word, `Auto`/`Yolo`/`Plan`
// stand out; plan is its own badge (a default run under the plan scope).
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
	return "Default"
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
		return "mode: Default — every non-allowlisted command asks", false
	}
}

// modeCycleHelp is the /mode cycle line: the current state and the next
// shift+tab stop.
func modeCycleHelp(m agent.PermMode, plan bool) string {
	next, nextPlan := nextMode(m, plan)
	return fmt.Sprintf("cycle (shift+tab): %s → %s", modeBadgeText(m, plan), modeBadgeText(next, nextPlan))
}

// modeNames lists the words /mode accepts — the same set as the --mode flag
// (plan is the scope; it lands at default, the agent's rule).
var modeNames = []string{string(agent.ModeDefault), string(agent.ModeAuto), string(agent.ModeYolo), "plan"}

// runMode is the /mode built-in: no args → the current mode + the cycle;
// `default|auto|yolo|plan` → set it (between runs). `plan` enters the plan
// scope (landing at default, even from a wider mode); `default` exits plan;
// auto/yolo drop it through SetMode.
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
		return m.println("usage: /mode default|auto|yolo|plan — no args shows the current mode")
	}
	if args == "plan" {
		m.agent.SetPlan(true) // ModeChanged prints + refreshes
		return nil
	}
	if m.agent.Plan() && args == "default" {
		m.agent.SetPlan(false) // /mode default exits plan; auto/yolo drop it in SetMode
	}
	m.agent.SetMode(agent.PermMode(args)) // ModeChanged prints + refreshes
	return nil
}

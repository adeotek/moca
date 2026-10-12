package agent

// Permission modes (§11): default (rules + shell analysis, every ask lands
// on the asker), auto (risk table + temp-dir jail + optional classifier)
// and yolo (everything passes). SetMode swaps the checkers between runs
// only; every swap is recorded as a permission_mode entry and announced as
// a ModeChanged event (the TUI's status line and notice). Yolo and plan are
// mutually exclusive: entering one turns the other off first (SetMode,
// SetPlan), so the agent is never in both.

import (
	"context"

	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

type PermMode string

const (
	ModeDefault PermMode = "default"
	ModeAuto    PermMode = "auto"
	ModeYolo    PermMode = "yolo"
)

// ModeChanged replaces YoloChanged: the TUI refreshes its badge and prints
// the mode's one-line notice. Plan is the plan flag at emission time (the
// TUI handles the event later, after a following SetPlan may have run).
type ModeChanged struct {
	Mode PermMode
	Plan bool
}

func (ModeChanged) isEvent() {}

// checkers holds the per-mode checkers built once at start (applyMode swaps
// them into Env). paths/cmds/ask are the default-mode set; autoPaths/autoCmds
// the auto-mode view (risk table, temp roots writable) sharing the same Rules.
type checkers struct {
	paths     tools.PathChecker
	cmds      tools.CommandChecker
	autoPaths tools.PathChecker // built at New() from Env.AutoJail
	autoCmds  tools.CommandChecker
	ask       tools.Asker
}

// Mode reports the current permission mode.
func (a *Agent) Mode() PermMode { return a.mode }

// Yolo reports whether yolo mode is on (kept for /yolo and the status line).
func (a *Agent) Yolo() bool { return a.mode == ModeYolo }

// applyMode is the only place that swaps the checkers into Env. It must run
// between runs (never during Run): the tools read Env.Paths/Commands/Ask
// from Run's goroutine.
//
// applyMode assumes New() populated the checkers for production Envs
// (auto falls back to nil Commands when a hand-built Env carries a custom
// non-Shell CommandChecker), and SetMode is only called between runs —
// the TUI enforces that with refuseRunning at each call site (Task 8's
// /mode must refuse while running too).
func (a *Agent) applyMode(m PermMode) {
	env := a.opts.Env
	switch m {
	case ModeYolo:
		env.Paths, env.Commands, env.Ask, env.Judge = permissions.NewUnjailed(env.Root), permissions.AllowAll{}, tools.AutoAllow, nil
	case ModeAuto:
		env.Paths, env.Commands, env.Ask, env.Judge = a.checkers.autoPaths, a.checkers.autoCmds, a.checkers.ask, a.judgeFn()
	default:
		env.Paths, env.Commands, env.Ask, env.Judge = a.checkers.paths, a.checkers.cmds, a.checkers.ask, nil
	}
	a.mode = m
}

// SetMode switches the permission mode between runs (never during Run) and
// records the decision in the session. Entering yolo turns plan mode off
// first (its own PlanChanged): yolo and plan never hold together.
func (a *Agent) SetMode(m PermMode) {
	if m == ModeYolo && a.plan {
		a.SetPlan(false)
	}
	a.applyMode(m)
	a.append(session.Entry{Type: session.TypePermissionMode,
		PermissionMode: &session.PermissionMode{Yolo: m == ModeYolo, Plan: a.plan, Mode: string(m)}})
	a.emit(ModeChanged{Mode: m, Plan: a.plan})
}

// SetYolo toggles yolo mode between runs (never during Run): the /yolo
// command's shorthand for SetMode(ModeYolo) / SetMode(ModeDefault).
func (a *Agent) SetYolo(on bool) {
	m := ModeDefault
	if on {
		m = ModeYolo
	}
	a.SetMode(m)
}

// judgeFn builds the auto-mode classifier's tools.Env.Judge: nil unless the
// classifier is on and a model can be resolved. A missing or misconfigured
// cheap model turns auto mode's unsure commands into asks (fail-safe), never
// a session error.
func (a *Agent) judgeFn() func(ctx context.Context, command string) (bool, string) {
	if !a.opts.Config.Permissions.Auto.ClassifierOn() {
		return nil
	}
	if _, _, err := a.judgeModel(); err != nil {
		a.logger().Warn("auto mode classifier unavailable; unsure commands will ask", "error", err)
		return nil
	}
	return a.judge
}

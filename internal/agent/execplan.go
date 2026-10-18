package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Plan execution (§14, rev 21): `/execute <plan>` / `moca execute <plan>` runs an
// implementation plan step by step. The ticked plan file is the durable
// progress record — it survives compaction and resume — so moca gets a todo
// list without a new tool under the schema freeze.

var uncheckedRe = regexp.MustCompile(`(?m)^\s*[-*] \[ \]`)

// uncheckedSteps counts the `- [ ]` steps left in the plan file at abs.
func uncheckedSteps(abs string) int {
	b, err := os.ReadFile(abs)
	if err != nil {
		return 0
	}
	return len(uncheckedRe.FindAllIndex(b, -1))
}

func execPrompt(rel string, n int, extra string) string {
	p := fmt.Sprintf("Execute the implementation plan in %s (%d unchecked steps). Work through its unchecked `- [ ]` steps in order: "+
		"for each, make the change, run the verification the step names, then tick its box (`- [ ]` → `- [x]`) in %s with edit "+
		"before moving on. If a step is blocked or the plan turns out wrong, stop and explain instead of improvising a different plan. "+
		"Final answer: steps completed, verification results, what remains.", rel, n, rel)
	if extra = strings.TrimSpace(extra); extra != "" {
		p += "\n\nAdditional instructions: " + extra
	}
	return p
}

// execNudge is persisted when a plan-execution run is about to end with steps
// still unchecked: one bounded nudge.
func execNudge(rel string, n int) string {
	return fmt.Sprintf("%d step(s) in %s are still unchecked. Continue with the next one (verify it, then tick it), "+
		"or explain what blocks it.", n, rel)
}

// RunPlan executes the plan file at path (relative to the workdir or
// absolute, inside the jail). extra is appended as additional instructions.
func (a *Agent) RunPlan(ctx context.Context, path, extra string) (Outcome, error) {
	return a.RunPlanWith(ctx, path, extra, RunOpts{})
}

// RunPlanWith is RunPlan carrying RunOpts: the TUI passes the /execute extra
// instructions as the run's typed text, so a hand-written skill they name can
// be changed (Decision 15).
func (a *Agent) RunPlanWith(ctx context.Context, path, extra string, o RunOpts) (Outcome, error) {
	if a.plan {
		return Outcome{}, errors.New("plan mode is on (writes are confined to docs/plans/) — turn it off to execute a plan")
	}
	abs, err := a.opts.Env.Paths.Resolve(path, false)
	if err != nil {
		return Outcome{}, err
	}
	if _, err := os.Stat(abs); err != nil {
		return Outcome{}, err
	}
	n := uncheckedSteps(abs)
	if n == 0 {
		return Outcome{}, fmt.Errorf("%s has no unchecked `- [ ]` steps", path)
	}
	rel, err := filepath.Rel(a.opts.Env.Root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = abs
	}
	rel = filepath.ToSlash(rel)
	a.execPlan, a.execPlanRel = abs, rel
	defer func() { a.execPlan, a.execPlanRel = "", "" }()
	return a.RunWith(ctx, execPrompt(rel, n, extra), o)
}

// nudgeExecPlan appends the unchecked-steps nudge when the plan being executed
// still has open steps; it reports whether it did.
func (a *Agent) nudgeExecPlan() (bool, error) {
	if a.execPlan == "" {
		return false, nil
	}
	n := uncheckedSteps(a.execPlan)
	if n == 0 {
		return false, nil
	}
	err := a.nudge("execute", execNudge(a.execPlanRel, n))
	return err == nil, err
}

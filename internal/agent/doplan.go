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

// Plan execution (§14, DESIGN rev 21): `/do <plan>` / `--do <plan>` runs an
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

func doPrompt(rel string, n int, extra string) string {
	p := fmt.Sprintf("Execute the implementation plan in %s (%d unchecked steps). Work through its unchecked `- [ ]` steps in order: "+
		"for each, make the change, run the verification the step names, then tick its box (`- [ ]` → `- [x]`) in %s with edit "+
		"before moving on. If a step is blocked or the plan turns out wrong, stop and explain instead of improvising a different plan. "+
		"Final answer: steps completed, verification results, what remains.", rel, n, rel)
	if extra = strings.TrimSpace(extra); extra != "" {
		p += "\n\nAdditional instructions: " + extra
	}
	return p
}

// doNudge is persisted when a plan-execution run is about to end with steps
// still unchecked: one bounded nudge.
func doNudge(rel string, n int) string {
	return fmt.Sprintf("%d step(s) in %s are still unchecked. Continue with the next one (verify it, then tick it), "+
		"or explain what blocks it.", n, rel)
}

// RunPlan executes the plan file at path (relative to the workdir or
// absolute, inside the jail). extra is appended as additional instructions.
func (a *Agent) RunPlan(ctx context.Context, path, extra string) (Outcome, error) {
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
	a.doPlan, a.doPlanRel = abs, rel
	defer func() { a.doPlan, a.doPlanRel = "", "" }()
	return a.Run(ctx, doPrompt(rel, n, extra))
}

// nudgeDoPlan appends the unchecked-steps nudge when the plan being executed
// still has open steps; it reports whether it did.
func (a *Agent) nudgeDoPlan() (bool, error) {
	if a.doPlan == "" {
		return false, nil
	}
	n := uncheckedSteps(a.doPlan)
	if n == 0 {
		return false, nil
	}
	err := a.nudge("do", doNudge(a.doPlanRel, n))
	return err == nil, err
}

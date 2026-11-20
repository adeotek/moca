package tools

import (
	"path/filepath"
	"strings"
)

// Plan mode (rev 20): the agent analyzes a request and writes an
// implementation plan to <workdir>/docs/plans/*.md — nothing else changes
// through write/edit. Env.Plan drives the write confinement; the shell's
// redirect/tee/time -o targets are confined to the same zone by the analyser
// (Shell.SetPlanDir), while shell, mcp, web and the read tools otherwise stay
// available under the normal permission ladder — a confinement, not a
// sandbox.
// Env.PlanWrote records the deliverable landing (the agent nudges a plan run
// that would otherwise end without it).

const planWriteRefusal = "refused: plan mode only writes docs/plans/*.md (the implementation plan) — " +
	"nothing else may change. Write the plan file, then stop"

// planAllows reports whether abs is a markdown file under the plan zone
// (<workdir>/docs/plans).
func planAllows(env *Env, abs string) bool {
	rel, err := filepath.Rel(filepath.Join(env.Root, "docs", "plans"), abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return strings.HasSuffix(strings.ToLower(rel), ".md")
}

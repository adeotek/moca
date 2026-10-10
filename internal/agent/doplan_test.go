package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const twoStepPlan = "# Plan\n\n- [ ] step one\n- [ ] step two\n"

// TestRunPlanExecutesAndNudges: the run gets the execution prompt; ending
// with a step unchecked earns one nudge, after which the run may finish.
func TestRunPlanExecutesAndNudges(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"read", `{"path":"docs/plans/p.md"}`}),
		toolTurn([2]string{"edit", `{"path":"docs/plans/p.md","old_string":"- [ ] step one","new_string":"- [x] step one"}`}),
		textTurn("step one done"),
		toolTurn([2]string{"edit", `{"path":"docs/plans/p.md","old_string":"- [ ] step two","new_string":"- [x] step two"}`}),
		textTurn("all done"),
	)
	a, work, _ := startTest(t, s, 40)
	os.MkdirAll(filepath.Join(work, "docs", "plans"), 0o755)
	writeFile(t, work, "docs/plans/p.md", twoStepPlan)
	out, err := a.RunPlan(context.Background(), "docs/plans/p.md", "keep commits small")
	if err != nil || out.Text != "all done" || s.bodyCount() != 5 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	first := lastMessageJSON(t, s, 0)
	if !strings.Contains(first, "Execute the implementation plan in docs/plans/p.md (2 unchecked steps)") || !strings.Contains(first, "keep commits small") {
		t.Fatalf("prompt: %s", first)
	}
	if !strings.Contains(lastMessageJSON(t, s, 3), "1 step(s) in docs/plans/p.md are still unchecked") {
		t.Fatalf("nudge: %s", lastMessageJSON(t, s, 3))
	}
	if b, _ := os.ReadFile(filepath.Join(work, "docs", "plans", "p.md")); strings.Contains(string(b), "- [ ]") {
		t.Fatalf("plan: %s", b)
	}
	if a.doPlan != "" {
		t.Fatal("the plan binding ends with the run")
	}
}

func TestRunPlanRefusals(t *testing.T) {
	s := newScript(t)
	a, work, _ := startTest(t, s, 40)
	writeFile(t, work, "done.md", "- [x] all\n")
	if _, err := a.RunPlan(context.Background(), "done.md", ""); err == nil || !strings.Contains(err.Error(), "no unchecked") {
		t.Fatalf("finished plan: %v", err)
	}
	if _, err := a.RunPlan(context.Background(), "missing.md", ""); err == nil {
		t.Fatal("missing plan")
	}
	writeFile(t, work, "p.md", twoStepPlan)
	a.SetPlan(true)
	if _, err := a.RunPlan(context.Background(), "p.md", ""); err == nil || !strings.Contains(err.Error(), "plan mode is on") {
		t.Fatalf("plan mode: %v", err)
	}
	if s.bodyCount() != 0 {
		t.Fatal("refusals send nothing")
	}
}

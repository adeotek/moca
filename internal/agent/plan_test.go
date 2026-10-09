package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func planMode(o *StartOptions) { o.Plan = true }

// TestPlanModeEnvelopeAndNudge: the envelope rides every request while plan
// mode is on (request-only — the transcript keeps the user's text), and a run
// that would end without its deliverable gets exactly one persisted nudge.
func TestPlanModeEnvelopeAndNudge(t *testing.T) {
	s := newScript(t,
		textTurn("Here is my answer without a file."),
		textTurn("done now"),
	)
	a, _, evs := startTest(t, s, 40, planMode)
	out, err := a.Run(context.Background(), "add a --json flag")
	if err != nil || out.Text != "done now" {
		t.Fatalf("%+v %v", out, err)
	}
	if s.bodyCount() != 2 {
		t.Fatalf("want 2 requests (the nudge is bounded), got %d", s.bodyCount())
	}
	var warned bool
	for _, e := range *evs {
		if w, ok := e.(Warning); ok && strings.Contains(w.Text, "without a plan file") {
			warned = true
		}
	}
	if !warned {
		t.Fatal("a plan run ending without its file must warn")
	}
	first := lastMessageJSON(t, s, 0)
	if !strings.Contains(first, "[plan mode]") || !strings.Contains(first, "add a --json flag") {
		t.Fatalf("first request missing the envelope: %s", first)
	}
	second := lastMessageJSON(t, s, 1)
	if !strings.Contains(second, "has not written a plan file") || !strings.Contains(second, "[plan mode]") {
		t.Fatalf("second request missing the nudge: %s", second)
	}
	// The transcript holds the user's text and the nudge — never the envelope.
	hist := a.History()
	if len(hist) < 3 || strings.Contains(hist[0].Content[0].Text, "[plan mode]") || hist[0].Content[0].Text != "add a --json flag" {
		t.Fatalf("transcript: %+v", hist)
	}
	if !strings.Contains(hist[2].Content[0].Text, "has not written a plan file") {
		t.Fatalf("nudge must be persisted: %+v", hist[2].Content[0].Text)
	}
}

func lastMessageJSON(t *testing.T, s *scriptServer, req int) string {
	t.Helper()
	msgs, _ := s.bodies[req]["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatalf("request %d has no messages", req)
	}
	b, _ := json.Marshal(msgs[len(msgs)-1])
	return string(b)
}

// TestPlanModeWriteLandsAndSkipsNudge: a plan file written through the tool
// satisfies the run — no nudge, and the file is on disk.
func TestPlanModeWriteLandsAndSkipsNudge(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"write", `{"path":"docs/plans/add-json-flag.md","content":"# Plan\n\n- [ ] step"}`}),
		textTurn("Plan written to docs/plans/add-json-flag.md: 1 step."),
	)
	a, work, _ := startTest(t, s, 40, planMode)
	out, err := a.Run(context.Background(), "add a --json flag")
	if err != nil || s.bodyCount() != 2 || out.Text == "" {
		t.Fatalf("out=%+v err=%v requests=%d", out, err, s.bodyCount())
	}
	if b, rerr := os.ReadFile(filepath.Join(work, "docs", "plans", "add-json-flag.md")); rerr != nil || !strings.Contains(string(b), "step") {
		t.Fatalf("plan file: %q %v", b, rerr)
	}
}

// TestPlanModeShellRefusedThroughAgent: the refusal reaches the model as the
// tool result (and the run still lands its file afterwards).
func TestPlanModeShellRefusedThroughAgent(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"shell", `{"command":"git status"}`}),
		toolTurn([2]string{"write", `{"path":"docs/plans/p.md","content":"# p"}`}),
		textTurn("done"),
	)
	a, _, _ := startTest(t, s, 40, planMode)
	if _, err := a.Run(context.Background(), "plan it"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range a.entries {
		if e.ToolResult != nil && e.ToolResult.IsError && strings.Contains(e.ToolResult.Content, "plan mode disables the shell") {
			found = true
		}
	}
	if !found {
		t.Fatal("shell refusal not persisted as a tool result")
	}
}

// TestPlanModeOffByDefaultAndToggleRecords: no envelope without plan mode, and
// SetPlan records a permission_mode entry (like /yolo).
func TestPlanModeOffByDefaultAndToggleRecords(t *testing.T) {
	s := newScript(t, textTurn("hi"))
	a, _, evs := startTest(t, s, 40)
	if a.Plan() {
		t.Fatal("plan mode must default off")
	}
	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(lastMessageJSON(t, s, 0), "[plan mode]") {
		t.Fatal("no envelope without plan mode")
	}
	a.SetPlan(true)
	if !a.Plan() || !a.opts.Env.Plan {
		t.Fatal("SetPlan must reach the tools env")
	}
	var sawChange bool
	for _, e := range *evs {
		if pc, ok := e.(PlanChanged); ok && pc.On {
			sawChange = true
		}
	}
	if !sawChange {
		t.Fatal("PlanChanged not emitted")
	}
	last := a.entries[len(a.entries)-1]
	if last.Type != "permission_mode" || last.PermissionMode == nil || !last.PermissionMode.Plan {
		t.Fatalf("permission_mode entry: %+v", last)
	}
	a.SetPlan(false)
	if a.Plan() || a.opts.Env.Plan {
		t.Fatal("SetPlan(false) must clear")
	}
}

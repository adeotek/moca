package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
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

// TestPlanModeShellRunsThroughAgent: shell works in plan mode under the
// normal allowlist (ls is allowlisted) — and the run still lands its file.
func TestPlanModeShellRunsThroughAgent(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"shell", `{"command":"ls"}`}),
		toolTurn([2]string{"write", `{"path":"docs/plans/p.md","content":"# p"}`}),
		textTurn("done"),
	)
	a, _, _ := startTest(t, s, 40, planMode)
	if _, err := a.Run(context.Background(), "plan it"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range a.entries {
		if e.ToolResult != nil && !e.ToolResult.IsError && strings.Contains(e.ToolResult.Content, "[exit 0]") {
			found = true
		}
	}
	if !found {
		t.Fatal("shell result missing or erroring in plan mode")
	}
}

// TestPlanModeMaxStepsWarnsMissingPlan: a plan run cut short by maxSteps
// cannot write its file in the tool-less wrap-up — it must still warn that
// the deliverable is missing, like the normal end of a run.
func TestPlanModeMaxStepsWarnsMissingPlan(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"ls", `{}`}),
		textTurn("ran out of steps"),
	)
	a, _, evs := startTest(t, s, 1, planMode)
	out, err := a.Run(context.Background(), "plan it")
	if err != nil || !out.MaxSteps {
		t.Fatalf("%+v %v", out, err)
	}
	for _, e := range *evs {
		if w, ok := e.(Warning); ok && strings.Contains(w.Text, "without a plan file") {
			return
		}
	}
	t.Fatal("a maxSteps-ended plan run without its file must warn")
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

// TestPlanEnvelopeKeepsPrefixStable: a second plan-mode prompt must not
// rewrite earlier messages (B4) — every message of the first run's last
// request is byte-identical at the same position in the next request, so
// the prompt cache survives the new prompt.
func TestPlanEnvelopeKeepsPrefixStable(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"write", `{"path":"docs/plans/a.md","content":"# a\n"}`}),
		textTurn("plan a"),
		toolTurn([2]string{"write", `{"path":"docs/plans/b.md","content":"# b\n"}`}),
		textTurn("plan b"),
	)
	a, _, _ := startTest(t, s, 40, planMode)
	for _, p := range []string{"plan a", "plan b"} {
		if _, err := a.Run(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := s.bodies[1]["messages"].([]any)
	next, _ := s.bodies[2]["messages"].([]any)
	if len(next) <= len(first) {
		t.Fatalf("lengths %d %d", len(first), len(next))
	}
	for i := range first {
		x, _ := json.Marshal(first[i])
		y, _ := json.Marshal(next[i])
		if string(x) != string(y) {
			t.Fatalf("message %d changed:\n%s\n%s", i, x, y)
		}
	}
}

// TestModeEntriesRecordBothFlags: each permission_mode entry records the
// full mode state, so toggling one mode never logs the other as off — and
// yolo and plan are mutually exclusive: plan in yolo leaves yolo for
// default first, yolo in plan turns plan off first.
func TestModeEntriesRecordBothFlags(t *testing.T) {
	a, _, _ := startTest(t, newScript(t), 40)
	a.SetYolo(true)
	a.SetPlan(true)
	a.SetYolo(false)
	a.SetYolo(true)
	var got []session.PermissionMode
	for _, e := range a.entries {
		if e.Type == session.TypePermissionMode {
			got = append(got, *e.PermissionMode)
		}
	}
	want := []session.PermissionMode{
		{Yolo: true, Mode: "yolo"},
		{Mode: "default"}, {Plan: true, Mode: "default"}, // SetPlan(true): yolo off, then plan on
		{Plan: true, Mode: "default"},
		{Mode: "default"}, {Yolo: true, Mode: "yolo"}, // SetYolo(true): plan off, then yolo
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// isMessagePrefix reports whether every message of prev appears, in order and
// element-for-element, at the head of cur — the wire-level append-only
// property (B1.1).
func isMessagePrefix(prev, cur []any) bool {
	if len(cur) < len(prev) {
		return false
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], cur[i]) {
			return false
		}
	}
	return true
}

// msgJSON returns each message of a request body as JSON text.
func msgJSON(t *testing.T, body map[string]any) []string {
	t.Helper()
	msgs, _ := body["messages"].([]any)
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

// TestPlanToggleIsAppendOnly (B1.1): plan on → Run A → off → Run B → on →
// Run C. Every consecutive request pair is an append-only extension; A's
// message carries the envelope in every later request, B's never, C's does.
func TestPlanToggleIsAppendOnly(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"write", `{"path":"docs/plans/a.md","content":"# a"}`}), textTurn("A done"),
		toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("B done"),
		toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("C done"),
	)
	a, work, _ := startTest(t, s, 40)
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.SetPlan(true)
	if _, err := a.Run(context.Background(), "plan A"); err != nil {
		t.Fatal(err)
	}
	a.SetPlan(false)
	if _, err := a.Run(context.Background(), "plan B"); err != nil {
		t.Fatal(err)
	}
	a.SetPlan(true)
	if _, err := a.Run(context.Background(), "plan C"); err != nil {
		t.Fatal(err)
	}
	bodies := snapshotBodies(s)
	if len(bodies) < 6 {
		t.Fatalf("want at least two requests per run, got %d", len(bodies))
	}
	var prev []any
	for i, b := range bodies {
		msgs, _ := b["messages"].([]any)
		if i > 0 && !isMessagePrefix(prev, msgs) {
			t.Fatalf("request %d is not an append-only extension of request %d", i+1, i)
		}
		prev = msgs
	}
	carries := func(msg, what string) bool { return strings.Contains(msg, what) }
	for i, b := range bodies {
		for _, m := range msgJSON(t, b) {
			switch {
			case carries(m, "plan A"):
				if !carries(m, "[plan mode]") {
					t.Fatalf("request %d: A's message lost its envelope: %s", i+1, m)
				}
			case carries(m, "plan C"):
				if !carries(m, "[plan mode]") {
					t.Fatalf("request %d: C's message lost its envelope: %s", i+1, m)
				}
			case carries(m, "plan B"):
				if carries(m, "[plan mode]") {
					t.Fatalf("request %d: an unflagged message must not carry the envelope: %s", i+1, m)
				}
			}
		}
	}
}

// TestPlanOffNoteFollowsToggle: turning plan off appends the off note once;
// a repeated off toggle appends nothing.
func TestPlanOffNoteFollowsToggle(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("A done"),
		toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("B done"),
		toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("C done"),
	)
	a, work, _ := startTest(t, s, 40)
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.SetPlan(true)
	if _, err := a.Run(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	a.SetPlan(false)
	if _, err := a.Run(context.Background(), "B"); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		bodies := snapshotBodies(s)
		last := msgJSON(t, bodies[len(bodies)-1])
		n := 0
		for _, m := range last {
			n += strings.Count(m, "[plan mode is off]")
		}
		return n
	}
	if got := count(); got != 1 {
		t.Fatalf("after on→off the note must appear exactly once, got %d", got)
	}
	a.SetPlan(false) // off → off: no second note
	if _, err := a.Run(context.Background(), "C"); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 1 {
		t.Fatalf("a repeated off toggle must not add a note, got %d", got)
	}
}

// TestResumeRendersLegacyPlanSession: a session written before the plan flag
// existed resumes without gaining an envelope, and two resumes of the same
// file render identical requests.
func TestResumeRendersLegacyPlanSession(t *testing.T) {
	s := newScript(t, textTurn("first"))
	a, _, _ := startTest(t, s, 40)
	if _, err := a.Run(context.Background(), "q1"); err != nil {
		t.Fatal(err)
	}
	path := a.Session().Path()
	work := session.WorkdirOf(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"plan"`) {
		t.Fatal("the fixture must be a legacy session (no plan field)")
	}
	a.Session().Close()
	render := func() []llm.Message {
		b, err := Resume(StartOptions{Config: a.opts.Config, Workdir: work, HTTP: s.srv.Client()}, path)
		if err != nil {
			t.Fatal(err)
		}
		msgs := session.MessagesWith(b.entries, planExtra, false)
		b.Session().Close()
		return msgs
	}
	m1, m2 := render(), render()
	if !reflect.DeepEqual(m1, m2) {
		t.Fatal("two resumes of the same file must render identical requests")
	}
	for _, m := range m1 {
		for _, b := range m.Content {
			if strings.Contains(b.Text, "[plan mode]") {
				t.Fatal("a legacy session must not gain an envelope")
			}
		}
	}
	if len(m1) == 0 {
		t.Fatal("the resumed history must not be empty")
	}
}

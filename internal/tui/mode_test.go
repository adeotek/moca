package tui

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/agent"

	"github.com/charmbracelet/x/ansi"
)

// nextMode is the shift+tab cycle: default → auto → yolo → plan → default.
// Plan rides only on default (the agent keeps plan out of auto/yolo), so
// the plan stop toggles the plan flag (the mode lands at default through
// SetPlan's rule) and the stop after plan wraps around.
func TestNextMode(t *testing.T) {
	type st struct {
		m    agent.PermMode
		plan bool
	}
	seq := []st{{agent.ModeDefault, false}, {agent.ModeAuto, false}, {agent.ModeYolo, false}, {agent.ModeDefault, true}, {agent.ModeDefault, false}}
	for i := 0; i+1 < len(seq); i++ {
		if m, p := nextMode(seq[i].m, seq[i].plan); m != seq[i+1].m || p != seq[i+1].plan {
			t.Errorf("from %+v → %v/%v, want %+v", seq[i], m, p, seq[i+1])
		}
	}
}

// shift+tab walks the mode cycle on a real agent; ctrl+t walks effort (the
// key effort cycling moved off shift+tab), asserted separately in
// TestCtrlTCyclesEffort.
func TestShiftTabCyclesMode(t *testing.T) {
	m := newAgentModel(t)
	m.Update(keyMsg("shift+tab"))
	if m.agent.Mode() != agent.ModeAuto {
		t.Fatalf("shift+tab → auto, got %v", m.agent.Mode())
	}
	// shift+tab must not touch effort any more (and it walks the mode, so
	// the next stop is yolo from here).
	start := m.agent.Status().Effort
	m.Update(keyMsg("shift+tab"))
	if got := m.agent.Status().Effort; got != start {
		t.Fatalf("shift+tab must not cycle effort: %v → %v", start, got)
	}
	if m.agent.Mode() != agent.ModeYolo || m.agent.Plan() {
		t.Fatalf("auto → yolo, got %v plan=%v", m.agent.Mode(), m.agent.Plan())
	}
	// Continue the walk on real keys: yolo → plan (the mode lands at
	// default — plan rides only on default) → default.
	m.Update(keyMsg("shift+tab"))
	if m.agent.Mode() != agent.ModeDefault || !m.agent.Plan() {
		t.Fatalf("yolo → plan (mode default), got %v plan=%v", m.agent.Mode(), m.agent.Plan())
	}
	m.Update(keyMsg("shift+tab"))
	if m.agent.Mode() != agent.ModeDefault || m.agent.Plan() {
		t.Fatalf("plan → default, got %v plan=%v", m.agent.Mode(), m.agent.Plan())
	}
}

// TestCtrlTCyclesEffort: ctrl+t cycles the effort knob, on a thinking-capable
// model built without the catalog (a fake ollama server whose show response
// has the thinking capability) — the key rerouting itself must never skip.
func TestCtrlTCyclesEffort(t *testing.T) {
	m := newThinkingAgentModel(t)
	st := m.agent.Status()
	if len(st.Model.ThinkingLevelMap) < 2 {
		t.Fatalf("thinking helper must expose effort levels: %+v", st.Model)
	}
	before := st.Effort
	_, cmd := m.Update(keyMsg("ctrl+t"))
	out := drained(m, cmd)
	got := m.agent.Status().Effort
	if got == before {
		t.Fatalf("ctrl+t must cycle effort: %v (%q)", got, out)
	}
	// Mid-run it refuses, like /effort.
	m.running = true
	before = got
	_, cmd = m.Update(keyMsg("ctrl+t"))
	out = drained(m, cmd)
	if got := m.agent.Status().Effort; got != before {
		t.Fatalf("ctrl+t must be refused mid-run: %v → %v (%q)", before, got, out)
	}
	if !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("mid-run ctrl+t refusal missing: %q", out)
	}
}

// Yolo can never carry plan: entering yolo drops it first.
func TestYoloNeverCarriesPlan(t *testing.T) {
	m := newAgentModel(t)
	m.agent.SetPlan(true)
	m.runCommand(Parsed{Kind: KindCommand, Name: "yolo"})
	if m.agent.Plan() {
		t.Fatal("yolo must not carry plan")
	}
	if m.agent.Mode() != agent.ModeYolo {
		t.Fatalf("mode: %v", m.agent.Mode())
	}
	if got := modeBadgeText(m.agent.Mode(), m.agent.Plan()); got != "Yolo" {
		t.Fatalf("badge text: %q", got)
	}
	// And the agent-level SetMode drops plan itself: plan is really off
	// (no docs/plans confinement, no envelope), not just hidden.
	m2 := newAgentModel(t)
	m2.agent.SetPlan(true)
	m2.agent.SetMode(agent.ModeYolo)
	if m2.agent.Plan() || m2.agent.Status().Plan {
		t.Fatal("SetMode(yolo) must turn plan off")
	}
	m2.refreshStatus()
	// Line 1 keeps identity content (the badge lives on line 2 — checked as
	// a separator-anchored segment: tmpdir names can contain "Yolo").
	if l1 := ansi.Strip(firstLineOf(m2.statusLine())); strings.Contains(l1, "· Yolo") {
		t.Fatalf("version line must not carry the badge: %q", l1)
	}
	// The badge moved to line 2, just before the model.
	l2 := ansi.Strip(secondLineOf(m2.statusLine()))
	if !strings.HasPrefix(l2, "Yolo · ") {
		t.Fatalf("yolo badge must open line 2: %q", l2)
	}
	if strings.Contains(l2, "Plan") {
		t.Fatalf("yolo badge must not show plan: %q", l2)
	}
	// The other way round: /plan in yolo leaves yolo for default.
	m2.runCommand(Parsed{Kind: KindCommand, Name: "plan"})
	if !m2.agent.Plan() || m2.agent.Mode() != agent.ModeDefault {
		t.Fatalf("/plan in yolo: mode %v plan %v, want default + plan", m2.agent.Mode(), m2.agent.Plan())
	}
}

func TestShiftTabRefusedWhileRunning(t *testing.T) {
	m := newAgentModel(t)
	m.running = true
	_, cmd := m.Update(keyMsg("shift+tab"))
	out := drained(m, cmd)
	if m.agent.Mode() != agent.ModeDefault {
		t.Fatalf("mode changed mid-run: %v", m.agent.Mode())
	}
	if !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("refusal missing: %q", out)
	}
}

func TestModeCommandShowsAndSets(t *testing.T) {
	m := newAgentModel(t)
	out := drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode"}))
	if !strings.Contains(out, "mode: Default") {
		t.Fatalf("no-arg /mode prints the current mode: %q", out)
	}
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "auto"}))
	if m.agent.Mode() != agent.ModeAuto {
		t.Fatalf("/mode auto: %q mode=%v", out, m.agent.Mode())
	}
	settle(m)
	out = drained(m, m.handleAgent(agent.ModeChanged{Mode: agent.ModeAuto}))
	if !strings.Contains(out, "mode: Auto") {
		t.Fatalf("/mode auto notice: %q", out)
	}
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "bogus"}))
	if m.agent.Mode() != agent.ModeAuto {
		t.Fatalf("bad arg must not change the mode: %v", m.agent.Mode())
	}
	if !strings.Contains(out, "usage: /mode default|auto|yolo|plan") {
		t.Fatalf("bad arg names the modes: %q", out)
	}
	m.running = true
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "default"}))
	if m.agent.Mode() != agent.ModeAuto {
		t.Fatalf("mid-run /mode must refuse: %v (%q)", m.agent.Mode(), out)
	}
	if !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("mid-run refusal missing: %q", out)
	}
	// The refusal precedes the name check (like /effort): mid-run even a bad
	// name gets the refusal, not a usage line.
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "bogus"}))
	if !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("mid-run /mode bogus must show the run refusal: %q", out)
	}
}

// /mode accepts the same values as --mode: plan enters the plan scope (and
// lands at default — the agent's rule, even from a wider mode), default
// exits it, and auto/yolo drop it through SetMode.
func TestModeCommandPlanSymmetry(t *testing.T) {
	m := newAgentModel(t)
	out := drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "plan"}))
	if !m.agent.Plan() || m.agent.Mode() != agent.ModeDefault {
		t.Fatalf("/mode plan: mode %v plan %v, want default + plan (%q)", m.agent.Mode(), m.agent.Plan(), out)
	}
	// From a wider mode, plan still lands at default.
	m.agent.SetMode(agent.ModeAuto)
	m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "plan"})
	if !m.agent.Plan() || m.agent.Mode() != agent.ModeDefault {
		t.Fatalf("/mode plan from auto: mode %v plan %v, want default + plan", m.agent.Mode(), m.agent.Plan())
	}
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "default"}))
	if m.agent.Plan() || m.agent.Mode() != agent.ModeDefault {
		t.Fatalf("/mode default must exit plan: mode %v plan %v (%q)", m.agent.Mode(), m.agent.Plan(), out)
	}
	// auto/yolo drop plan through SetMode's own rule.
	m.agent.SetPlan(true)
	m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "auto"})
	if m.agent.Plan() || m.agent.Mode() != agent.ModeAuto {
		t.Fatalf("/mode auto must drop plan: mode %v plan %v", m.agent.Mode(), m.agent.Plan())
	}
	m.agent.SetPlan(true)
	m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "yolo"})
	if m.agent.Plan() || m.agent.Mode() != agent.ModeYolo {
		t.Fatalf("/mode yolo must drop plan: mode %v plan %v", m.agent.Mode(), m.agent.Plan())
	}
	// The usage line names all four values.
	m.agent.SetMode(agent.ModeDefault)
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "bogus"}))
	if !strings.Contains(out, "usage: /mode default|auto|yolo|plan") {
		t.Fatalf("usage must list four values: %q", out)
	}
}

// TestStatusAlwaysShowsMode: the permission badge opens line 2 of the status
// bar, just before the model, in every mode and is never dropped, even at
// narrow widths; line 1 keeps its original version/cwd/branch content.
func TestStatusAlwaysShowsMode(t *testing.T) {
	m := newAgentModel(t)
	m.refreshStatus()
	check := func(want string) {
		t.Helper()
		s := m.statusLine()
		l1, l2 := firstLineOf(s), secondLineOf(s)
		if !strings.HasPrefix(ansi.Strip(l2), want+" · ") {
			t.Fatalf("status line 2 must open with %q: %q", want, l2)
		}
		if strings.Contains(ansi.Strip(l1), want) {
			t.Fatalf("line 1 must not carry the badge %q: %q", want, l1)
		}
	}
	check("Default")
	m.agent.SetPlan(true)
	check("Plan")
	m.agent.SetPlan(false)
	// AUTO badge keeps its green (palette) style — the one new badge style;
	// Task 10 moved the color to the theme palette (green in either).
	m.agent.SetMode(agent.ModeAuto)
	if !strings.Contains(secondLineOf(m.statusLine()), autoBadgeStyle(true).Render("Auto")) {
		t.Fatal("AUTO badge must be green")
	}
	m.agent.SetPlan(true)
	check("Plan")
	m.agent.SetMode(agent.ModeYolo)
	m.agent.SetPlan(false)
	check("Yolo")

	// Width 20 leaves the badge whole (never dropped, never truncated):
	// the layout reserves the badge width and shrinks line 2's body to the
	// remainder.
	m.width = 20
	m.status = StatusInfo{Version: "v", Cwd: "~/x/y", Branch: "longbranch*", Git: true,
		Model: "opencode-go/glm-5.3-flash", Effort: "med", Window: 200_000, Used: 30_000, In: 1, Out: 1}
	l2 := ansi.Strip(secondLineOf(m.statusLine()))
	if !strings.HasPrefix(l2, "Yolo · ") {
		t.Fatalf("Yolo badge must survive width 20: %q", l2)
	}
	m.agent.SetMode(agent.ModeAuto)
	m.agent.SetPlan(true)
	if l2 := ansi.Strip(secondLineOf(m.statusLine())); !strings.HasPrefix(l2, "Plan · ") {
		t.Fatalf("plan badge (SetPlan leaves auto for default) must survive width 20: %q", l2)
	}
}

// The ModeChanged handler prints one notice per state, replacing the old
// YoloChanged texts.
func TestModeChangedNotices(t *testing.T) {
	m := newAgentModel(t)
	check := func(e agent.Event, want string) {
		t.Helper()
		settle(m)
		out := drained(m, m.handleAgent(e))
		if !strings.Contains(out, want) {
			t.Fatalf("notice for %v: got %q, want %q", e, out, want)
		}
	}
	check(agent.ModeChanged{Mode: agent.ModeAuto}, "mode: Auto — safe actions run, risky ones ask")
	check(agent.ModeChanged{Mode: agent.ModeDefault}, "mode: Default — every non-allowlisted command asks")
	check(agent.ModeChanged{Mode: agent.ModeYolo}, "mode: Yolo — all permission checks are off")
	check(agent.ModeChanged{Mode: agent.ModeDefault, Plan: true}, "mode: Plan — runs write a plan to docs/plans/")
}

// TestStatusBadgeReservesLine2Only: the badge's width comes off line 2
// alone — line 1 renders exactly as RenderStatus does at the full width.
func TestStatusBadgeReservesLine2Only(t *testing.T) {
	m := newAgentModel(t)
	m.refreshStatus()
	m.status.Cwd = "~/projects/a-rather-long/workdir-name"
	for w := 20; w <= 120; w++ {
		m.width = w
		want, _ := RenderStatus(m.status, w)
		if got := ansi.Strip(firstLineOf(m.statusLine())); got != plainLine(want) {
			t.Fatalf("width %d: line 1 %q, want %q", w, got, plainLine(want))
		}
		if got := ansi.StringWidth(secondLineOf(m.statusLine())); got > w {
			t.Fatalf("width %d: line 2 is %d cells", w, got)
		}
	}
}

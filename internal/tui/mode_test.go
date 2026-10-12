package tui

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/agent"

	"github.com/charmbracelet/x/ansi"
)

// nextMode is the shift+tab cycle: default → auto → plan (auto + plan
// scope) → yolo → default. A state reached another way (/plan in default,
// --plan --yolo) moves to the next position after plan: yolo. Yolo itself
// never carries plan (yolo+plan → yolo stays).
func TestNextMode(t *testing.T) {
	type st struct {
		m    agent.PermMode
		plan bool
	}
	seq := []st{{agent.ModeDefault, false}, {agent.ModeAuto, false}, {agent.ModeAuto, true}, {agent.ModeYolo, false}, {agent.ModeDefault, false}}
	for i := 0; i+1 < len(seq); i++ {
		if m, p := nextMode(seq[i].m, seq[i].plan); m != seq[i+1].m || p != seq[i+1].plan {
			t.Errorf("from %+v → %v/%v, want %+v", seq[i], m, p, seq[i+1])
		}
	}
	if m, p := nextMode(agent.ModeYolo, true); m != agent.ModeYolo || p {
		t.Error("yolo+plan → yolo")
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
	// the next stop is auto+plan from here).
	start := m.agent.Status().Effort
	m.Update(keyMsg("shift+tab"))
	if got := m.agent.Status().Effort; got != start {
		t.Fatalf("shift+tab must not cycle effort: %v → %v", start, got)
	}
	if m.agent.Mode() != agent.ModeAuto || !m.agent.Plan() {
		t.Fatalf("auto → auto+plan, got %v plan=%v", m.agent.Mode(), m.agent.Plan())
	}
	// Continue the walk on real keys: auto+plan → yolo (plan off) → default,
	// and from yolo+plan (reached via /plan) shift+tab keeps yolo and drops
	// plan.
	m.Update(keyMsg("shift+tab"))
	if m.agent.Mode() != agent.ModeYolo || m.agent.Plan() {
		t.Fatalf("auto+plan → yolo (plan off), got %v plan=%v", m.agent.Mode(), m.agent.Plan())
	}
	m.Update(keyMsg("shift+tab"))
	if m.agent.Mode() != agent.ModeDefault {
		t.Fatalf("yolo → default, got %v", m.agent.Mode())
	}
	m.agent.SetPlan(true) // plan in default (out-of-cycle state)
	m.Update(keyMsg("shift+tab"))
	if m.agent.Mode() != agent.ModeYolo || m.agent.Plan() {
		t.Fatalf("plan → next after plan: yolo, got %v plan=%v", m.agent.Mode(), m.agent.Plan())
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
	if got := modeBadgeText(m.agent.Mode(), m.agent.Plan()); got != "YOLO" {
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
	l1 := ansi.Strip(firstLineOf(m2.statusLine()))
	if !strings.Contains(l1, "YOLO · ") {
		t.Fatalf("yolo badge must lead line 1: %q", l1)
	}
	if strings.Contains(l1, "PLAN") {
		t.Fatalf("yolo badge must not show plan: %q", l1)
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
	if !strings.Contains(out, "mode: default") {
		t.Fatalf("no-arg /mode prints the current mode: %q", out)
	}
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "auto"}))
	if m.agent.Mode() != agent.ModeAuto {
		t.Fatalf("/mode auto: %q mode=%v", out, m.agent.Mode())
	}
	settle(m)
	out = drained(m, m.handleAgent(agent.ModeChanged{Mode: agent.ModeAuto}))
	if !strings.Contains(out, "mode: auto") {
		t.Fatalf("/mode auto notice: %q", out)
	}
	out = drained(m, m.runCommand(Parsed{Kind: KindCommand, Name: "mode", Args: "bogus"}))
	if m.agent.Mode() != agent.ModeAuto {
		t.Fatalf("bad arg must not change the mode: %v", m.agent.Mode())
	}
	if !strings.Contains(out, "usage: /mode default|auto|yolo") {
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

// TestStatusAlwaysShowsMode: the permission badge leads line 1 of the status
// bar in every mode and is never dropped, even at narrow widths.
func TestStatusAlwaysShowsMode(t *testing.T) {
	m := newAgentModel(t)
	m.refreshStatus()
	check := func(want string) {
		t.Helper()
		l1 := firstLineOf(m.statusLine())
		if !strings.Contains(ansi.Strip(l1), want+" · ") {
			t.Fatalf("status line 1 must lead with %q: %q", want, l1)
		}
	}
	check("default")
	m.agent.SetPlan(true)
	check("default · PLAN")
	m.agent.SetPlan(false)
	// AUTO badge keeps its green (palette) style — the one new badge style;
	// Task 10 moved the color to the theme palette (green in either).
	m.agent.SetMode(agent.ModeAuto)
	if !strings.Contains(firstLineOf(m.statusLine()), autoBadgeStyle(true).Render("AUTO")) {
		t.Fatal("AUTO badge must be green")
	}
	m.agent.SetPlan(true)
	check("AUTO · PLAN")
	m.agent.SetMode(agent.ModeYolo)
	m.agent.SetPlan(false)
	check("YOLO")

	// Width 20 leaves the badge whole (never dropped, never truncated):
	// the layout reserves the badge width and shrinks line 1's body to the
	// remainder.
	m.width = 20
	m.status = StatusInfo{Version: "v", Cwd: "~/x/y", Branch: "longbranch*", Git: true,
		Model: "opencode-go/glm-5.3-flash", Effort: "med", Window: 200_000, Used: 30_000, In: 1, Out: 1}
	l1 := ansi.Strip(firstLineOf(m.statusLine()))
	if !strings.Contains(l1, "YOLO · ") {
		t.Fatalf("YOLO badge must survive width 20: %q", l1)
	}
	m.agent.SetMode(agent.ModeAuto)
	m.agent.SetPlan(true)
	if l1 := ansi.Strip(firstLineOf(m.statusLine())); !strings.Contains(l1, "AUTO · PLAN · ") {
		t.Fatalf("AUTO · PLAN must survive width 20: %q", l1)
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
	check(agent.ModeChanged{Mode: agent.ModeAuto}, "mode: auto — safe actions run, risky ones ask")
	check(agent.ModeChanged{Mode: agent.ModeDefault}, "mode: default — every non-allowlisted command asks")
	check(agent.ModeChanged{Mode: agent.ModeYolo}, "mode: YOLO — ALL permission checks are off")
	check(agent.ModeChanged{Mode: agent.ModeAuto, Plan: true}, "mode: plan — runs write a plan to docs/plans/")
}

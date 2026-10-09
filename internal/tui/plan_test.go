package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
)

// simulateAll drives a submission like simulate, then flushes the scrollback
// queue (some prints ride the ack cycle).
func simulateAll(m *model, cmd tea.Cmd) string {
	out, _ := simulate(m, cmd)
	var sb strings.Builder
	sb.WriteString(out)
	for m.outBusy {
		sb.WriteString(printed(ack(m)))
	}
	return sb.String()
}

func TestPlanCommandTogglesAndBadges(t *testing.T) {
	m := newAgentModel(t)
	if m.agent.Plan() {
		t.Fatal("plan mode must default off")
	}
	m.runCommand(Parsed{Kind: KindCommand, Name: "plan"})
	if !m.agent.Plan() {
		t.Fatal("toggle must turn plan mode on")
	}
	if !strings.Contains(m.statusLine(), "PLAN") {
		t.Fatalf("status badge missing: %q", m.statusLine())
	}
	// The live program delivers the toggle through the event bridge; inject
	// it like the other handler tests do.
	out := drained(m, m.handleAgent(agent.PlanChanged{On: true}))
	if !strings.Contains(out, "plan mode on") {
		t.Fatalf("toggle-on notice: %q", out)
	}
	m.runCommand(Parsed{Kind: KindCommand, Name: "plan"})
	if m.agent.Plan() {
		t.Fatal("second toggle must turn plan mode off")
	}
	out = drained(m, m.handleAgent(agent.PlanChanged{On: false}))
	if !strings.Contains(out, "plan mode off") {
		t.Fatalf("toggle-off notice: %q", out)
	}
	if strings.Contains(m.statusLine(), "PLAN") {
		t.Fatalf("status badge must clear: %q", m.statusLine())
	}
}

func TestPlanCommandWithRequestSubmitsPlanRun(t *testing.T) {
	m := newAgentModel(t)
	out := simulateAll(m, tuiCmd(t, m, "/plan add a --json flag"))
	if !m.agent.Plan() {
		t.Fatal("a request must turn plan mode on")
	}
	if !strings.Contains(out, "› add a --json flag") {
		t.Fatalf("the request must be submitted as a run: %q", out)
	}
}

func TestPlanCommandRefusedMidRun(t *testing.T) {
	m := newAgentModel(t)
	m.running = true // a run is in flight
	out := simulateAll(m, m.runCommand(Parsed{Kind: KindCommand, Name: "plan"}))
	if m.agent.Plan() || !strings.Contains(out, "run") {
		t.Fatalf("mid-run toggle must refuse: %q plan=%v", out, m.agent.Plan())
	}
}

func TestDoCommand(t *testing.T) {
	m := newAgentModel(t)
	if out := simulateAll(m, m.runCommand(Parsed{Kind: KindCommand, Name: "do"})); !strings.Contains(out, "usage: /do") {
		t.Fatalf("no path: %q", out)
	}
	m.agent.SetPlan(true)
	if out := simulateAll(m, m.runCommand(Parsed{Kind: KindCommand, Name: "do", Args: "docs/plans/x.md"})); !strings.Contains(out, "plan mode is on") {
		t.Fatalf("plan mode: %q", out)
	}
	m.agent.SetPlan(false)
	out := simulateAll(m, tuiCmd(t, m, "/do docs/plans/x.md keep it small"))
	if !strings.Contains(out, "› /do docs/plans/x.md keep it small") {
		t.Fatalf("/do must start a run: %q", out)
	}
}

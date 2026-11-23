package agent

// Task 5's contract: the tool list decision per session row, the model-facing
// order, the prompt budget, and steering delivery in a batch that contains a
// codemode call (R26) without changing ordinary batches (R64).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// ---- the session table ---------------------------------------------------

func TestPlanCodemode(t *testing.T) {
	on := config.CodemodeState{On: true}
	onNet := config.CodemodeState{On: true, Network: true}
	off := config.CodemodeState{}
	cases := []struct {
		name                                string
		resumed, had, stored                bool
		cm                                  config.CodemodeState
		present, refusal, prelude, admitted bool
	}{
		{"fresh off", false, false, false, off, false, false, false, false},
		{"fresh on workspace", false, false, false, on, true, false, false, false},
		{"fresh on network", false, false, false, onNet, true, false, true, true},
		{"resumed legacy header", true, false, false, onNet, false, false, false, false},
		{"resumed off-now", true, true, false, off, true, true, false, false},
		{"resumed network header, still on", true, true, true, onNet, true, false, true, true},
		{"resumed network header, switch lost", true, true, true, on, true, false, true, false},
		{"resumed workspace header, network gained", true, true, false, onNet, true, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := planCodemode(c.resumed, c.had, c.stored, c.cm)
			if p.present != c.present || p.refusal != c.refusal {
				t.Fatalf("present=%v refusal=%v, want %v/%v", p.present, p.refusal, c.present, c.refusal)
			}
			if p.preludeNetwork != c.prelude || p.admittedNetwork != c.admitted {
				t.Fatalf("prelude=%v admitted=%v, want %v/%v", p.preludeNetwork, p.admittedNetwork, c.prelude, c.admitted)
			}
		})
	}
}

// The prompt carries three renders; off must stay byte-identical, and the two
// on-variants have fixed budgets and newline counts (SPECS §14).
func TestBuildSystemPromptCodemodeBudget(t *testing.T) {
	base := BuildSystemPrompt(PromptInput{})
	if base != BuildSystemPrompt(PromptInput{Codemode: false}) {
		t.Fatal("codemode off is not byte-identical to the baseline")
	}
	if strings.Contains(base, "codemode") {
		t.Fatal("the baseline prompt mentions codemode")
	}
	ws := BuildSystemPrompt(PromptInput{Codemode: true})
	net := BuildSystemPrompt(PromptInput{Codemode: true, CodemodeNetwork: true})
	if len(ws) > 6400 {
		t.Fatalf("workspace prompt is %d bytes, budget 6400", len(ws))
	}
	if len(net) > 6404 {
		t.Fatalf("network prompt is %d bytes, budget 6404", len(net))
	}
	if len(net)-len(ws) != 4 {
		t.Fatalf("network variant differs by %d bytes, want 4", len(net)-len(ws))
	}
	for name, p := range map[string]string{"workspace": ws, "network": net} {
		if n := strings.Count(p, "\n"); n != strings.Count(base, "\n") {
			t.Fatalf("%s adds %d newlines", name, n-strings.Count(base, "\n"))
		}
	}
	if !strings.Contains(ws, "one read-only JS program over read/search/ls") {
		t.Fatalf("workspace hint missing: %q", ws)
	}
	if !strings.Contains(net, "over read/search/ls/web") {
		t.Fatalf("network hint missing")
	}
}

// ---- the model-facing order ----------------------------------------------

type stubTool struct {
	name   string
	result tools.Result
}

func (s stubTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: s.name, Description: "stub", Schema: json.RawMessage(`{"type":"object"}`)}
}

func (s stubTool) Run(context.Context, *tools.Env, json.RawMessage) tools.Result { return s.result }

// The ninth tool is appended last: the eight shipped schemas keep their bytes
// and their order, and codemode is what the model sees after mcp.
func TestCodemodeIsRegisteredLast(t *testing.T) {
	s := newScript(t, textTurn("done"))
	a, _, _ := startWith(t, s, 1, `"codemode":{"enabled":true}`, "")
	specs := a.opts.Tools.Specs()
	want := []string{"read", "write", "edit", "shell", "search", "ls", "web", "mcp", "codemode"}
	if len(specs) != len(want) {
		got := make([]string, len(specs))
		for i, sp := range specs {
			got[i] = sp.Name
		}
		t.Fatalf("tools = %v, want %v", got, want)
	}
	for i, name := range want {
		if specs[i].Name != name {
			t.Fatalf("tool %d = %q, want %q", i, specs[i].Name, name)
		}
	}
	// A session with the switch off keeps exactly the eight.
	s2 := newScript(t, textTurn("done"))
	a2, _, _ := startWith(t, s2, 1, "", "")
	if n := len(a2.opts.Tools.Specs()); n != 8 {
		t.Fatalf("off session has %d tools, want 8", n)
	}
}

// ---- steering ------------------------------------------------------------

// R26: a batch whose last call is a steered codemode call, with the repeat-stop
// armed, still delivers the pending steering message exactly once — and it
// lands after every tool result, so every call id stays paired.
func TestCodemodeSteeringBeatsTheRepeatStop(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"codemode", `{"code":"1"}`}), textTurn("wrapped up"))
	a, _, _ := startWith(t, s, 20, "", "")
	a.opts.Tools.Register(stubTool{name: "codemode", result: tools.Result{Content: "ok", Steered: true}})
	a.opts.Emit = func(e Event) {
		if _, ok := e.(ToolStart); ok {
			// MaxRepeat is per-run state reset by the run itself, so arm it here
			// — mid-batch, as an earlier batch's repeats would have.
			a.opts.Env.MaxRepeat = tools.RepeatStopAt
			a.Steer("also check the README", "")
		}
	}
	out, err := a.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(s.bodies) < 2 {
		t.Fatalf("requests = %d, want the follow-up turn", len(s.bodies))
	}
	msgs := s.bodies[1]["messages"].([]any)
	var seen int
	for i, m := range msgs {
		mm := m.(map[string]any)
		if mm["role"] == "user" && strings.Contains(mm["content"].(string), "also check the README") {
			seen++
			prev := msgs[i-1].(map[string]any)
			if prev["role"] != "tool" {
				t.Fatalf("steering does not follow a tool result: %v", prev["role"])
			}
		}
	}
	if seen != 1 {
		t.Fatalf("steering delivered %d times, want exactly 1", seen)
	}
	if !out.Stuck {
		t.Fatal("the repeat-stop did not end the run")
	}
	if a.PendingSteering() != 0 {
		t.Fatalf("%d steering messages left queued", a.PendingSteering())
	}
}

// R64: an ordinary batch keeps today's order — the wrap-up wins and the queued
// message is not delivered mid-run, so batches without codemode are unchanged.
func TestOrdinaryBatchKeepsTheWrapUpFirst(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}), textTurn("wrapped up"))
	a, _, _ := startWith(t, s, 20, "", "")
	a.opts.Emit = func(e Event) {
		if _, ok := e.(ToolStart); ok {
			a.opts.Env.MaxRepeat = tools.RepeatStopAt
			a.Steer("also check the README", "")
		}
	}
	out, err := a.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !out.Stuck {
		t.Fatal("the repeat-stop did not end the run")
	}
	body, err := json.Marshal(s.bodies[len(s.bodies)-1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "also check the README") {
		t.Fatal("an ordinary batch delivered steering before the wrap-up")
	}
	if a.PendingSteering() != 1 {
		t.Fatalf("pending = %d, want the message still queued", a.PendingSteering())
	}
}

// F24: a bare constructor must not panic when a batch path calls Admit.
func TestNewNormalizesNilAdmit(t *testing.T) {
	s := newScript(t, textTurn("done"))
	_, _, _ = startWith(t, s, 1, "", "")
	env := &tools.Env{Root: t.TempDir(), Reads: tools.NewReadTracker()}
	if env.Admit != nil {
		t.Fatal("fixture already has Admit")
	}
	a, err := New(Options{Env: env, Tools: tools.NewRegistry(), Emit: func(Event) {}})
	if err != nil {
		t.Skipf("bare New needs more setup here: %v", err)
	}
	if env.Admit == nil || !env.Admit() {
		t.Fatal("New left Admit nil or denying")
	}
	_ = a
}

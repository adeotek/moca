package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func TestSetModelAndEffortWriteModelChange(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTestWith(t, s, `"modelHard":"fake/big"`, `"big":{"contextWindow":131072}`)
	if err := a.SetModel("fake/big", ""); err != nil {
		t.Fatal(err)
	}
	if a.Model().Qualified() != "fake/big" {
		t.Fatal("switched")
	}
	if _, err := a.SetEffort(llm.EffortHigh); err != nil {
		t.Fatal(err)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	var changes int
	for _, e := range entries {
		if e.Type == session.TypeModelChange {
			changes++
		}
	}
	if changes != 2 {
		t.Fatal("model_change per switch")
	}
}

func TestSetModelRefusedWhenKeyMissing(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	t.Setenv("MOCA_T_KEY", "")
	before := a.Model().Qualified()
	err := a.SetModel("fake/m", "")
	if err == nil || !strings.Contains(err.Error(), "MOCA_T_KEY") || a.Model().Qualified() != before {
		t.Fatal(err)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	for _, e := range entries {
		if e.Type == session.TypeModelChange {
			t.Fatal("no model_change on failure")
		}
	}
}

func TestToggleHard(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, `"modelHard":"fake/big"`, `"big":{"contextWindow":131072}`)
	orig, origE := a.Model().Qualified(), a.Effort()
	on, err := a.ToggleHard()
	if err != nil || !on || a.Model().Qualified() != "fake/big" {
		t.Fatal(on, err)
	}
	on, _ = a.ToggleHard()
	if on || a.Model().Qualified() != orig || a.Effort() != origE {
		t.Fatal("restores saved pair")
	}
}

// With no modelHardEffort the escalation stays at high, the level /hard
// shipped with (§4).
func TestHardEffortDefaultsToHigh(t *testing.T) {
	s := newScript(t)
	cfg := catalogConfig(t, s, `,"modelHard":"opencode-go/glm-5.3"`)
	a := startCatalog(t, s, cfg)
	if on, err := a.ToggleHard(); err != nil || !on || a.Effort() != llm.EffortHigh {
		t.Fatalf("hard: on=%v effort=%q (%v)", on, a.Effort(), err)
	}
}

// modelEffort and modelHardEffort (§4, v0.7.11): the config decides a fresh
// session's effort and /hard's escalation, both clamped to the model's own
// levels, and the session header records the resolved level.
func TestConfigEffort(t *testing.T) {
	s := newScript(t)
	cfg := catalogConfig(t, s, `,"modelEffort":"high","modelHard":"opencode-go/glm-5.3","modelHardEffort":"max"`)
	a := startCatalog(t, s, cfg)
	// glm-5.3-flash supports low|high|max, so "high" is taken as-is.
	if a.Effort() != llm.EffortHigh {
		t.Fatalf("modelEffort high → %q", a.Effort())
	}
	entries, err := session.ReadFile(a.Session().Path())
	if err != nil || len(entries) == 0 || entries[0].Session.Effort != string(llm.EffortHigh) {
		t.Fatalf("header effort: %+v (%v)", entries, err)
	}
	// /hard goes to modelHard at modelHardEffort.
	on, err := a.ToggleHard()
	if err != nil || !on || a.Model().Qualified() != "opencode-go/glm-5.3" || a.Effort() != llm.EffortMax {
		t.Fatalf("hard: on=%v model=%s effort=%q (%v)", on, a.Model().Qualified(), a.Effort(), err)
	}
	if on, err := a.ToggleHard(); err != nil || on || a.Model().Qualified() != "opencode-go/glm-5.3-flash" || a.Effort() != llm.EffortHigh {
		t.Fatalf("back: on=%v model=%s effort=%q (%v)", on, a.Model().Qualified(), a.Effort(), err)
	}
}

// The config effort is a starting point, not a floor: it is clamped to the
// model's levels, and --effort still wins over it (§4 — the flag is the
// per-run override).
func TestConfigEffortClampsAndFlagWins(t *testing.T) {
	s := newScript(t)
	// flash has no medium: the closest level at or below is low.
	a := startCatalog(t, s, catalogConfig(t, s, `,"modelEffort":"medium"`))
	if a.Effort() != llm.EffortLow {
		t.Fatalf("modelEffort medium on a low|high|max model → %q, want low", a.Effort())
	}
	s2 := newScript(t)
	cfg := catalogConfig(t, s2, `,"modelEffort":"low"`)
	b, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), HTTP: s2.srv.Client(), Slug: "t", Effort: "max"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Effort() != llm.EffortMax {
		t.Fatalf("--effort max over modelEffort low → %q", b.Effort())
	}
}

func TestSteeringLandsAfterToolResults(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}), textTurn("done"))
	a, _, _ := startTestWith(t, s, "", "")
	a.opts.Emit = func(e Event) {
		if _, ok := e.(ToolStart); ok {
			a.Steer("also check README", "")
		}
	}
	a.Run(context.Background(), "go")
	msgs := s.bodies[1]["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)
	prev := msgs[len(msgs)-2].(map[string]any)
	if prev["role"] != "tool" || last["role"] != "user" || last["content"] != "also check README" {
		t.Fatalf("steering must follow the tool result: %v / %v", prev, last)
	}
}

func TestSteeringWithoutToolsBecomesNextMessage(t *testing.T) {
	s := newScript(t, textTurn("first"), textTurn("second"))
	a, _, _ := startTestWith(t, s, "", "")
	a.opts.Emit = func(e Event) {
		if _, ok := e.(TextDelta); ok && s.bodyCount() == 1 {
			a.Steer("follow-up", "")
		}
	}
	out, _ := a.Run(context.Background(), "go")
	if out.Text != "second" || len(s.bodies) != 2 {
		t.Fatal(out, len(s.bodies))
	}
}

func TestAbortReturnsSteering(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}, [2]string{"ls", `{}`}))
	a, _, _ := startTestWith(t, s, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	steered := false
	a.opts.Emit = func(e Event) {
		// abort() reports synthetic results as ToolEnds too; steer once.
		if _, ok := e.(ToolEnd); ok && !steered {
			steered = true
			a.Steer("draft", "")
			cancel()
		}
	}
	a.Run(ctx, "go")
	if got := a.TakeSteering(); len(got) != 1 || got[0] != "draft" {
		t.Fatal(got)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	for _, e := range entries {
		if e.Message != nil && len(e.Message.Content) > 0 && e.Message.Content[0].Text == "draft" {
			t.Fatal("aborted steering must not reach the transcript")
		}
	}
}

// A failed append must not swallow the rest of the drained queue: the
// messages that did not land are re-queued and return to the editor through
// TakeSteering (the aborted-run contract).
func TestSteeringRequeuesOnAppendFailure(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTestWith(t, s, "", "")
	defer a.Session().Close()
	a.Steer("first", "first")
	a.Steer("second", "second")
	a.Session().Close() // the underlying appends now fail
	if _, err := a.applySteering(); err == nil {
		t.Fatal("a closed session's append must fail")
	}
	// "first" is the append that failed: it did not land either, so it comes
	// back too — dropping it would lose text the user typed.
	if got := a.TakeSteering(); len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("the un-appended messages must be back in the queue, got %q", got)
	}
}

func TestContextTokensAnchored(t *testing.T) {
	s := newScript(t, textTurn("ok")) // usage: prompt 10 + completion 5
	a, _, _ := startTestWith(t, s, "", "")
	before := a.ContextTokens()
	if before == 0 {
		t.Fatal("chars/4 fallback before the first response")
	}
	a.Run(context.Background(), "hi")
	if a.ContextTokens() != 15 {
		t.Fatalf("anchored at usage: %d", a.ContextTokens())
	}
	a.AddNote(strings.Repeat("x", 40))
	if a.ContextTokens() != 15+10 {
		t.Fatalf("anchor + delta: %d", a.ContextTokens())
	}
}

// A provider that reports no usage must not anchor the estimate at ~0.
func TestContextTokensZeroUsageFallsBack(t *testing.T) {
	s := newScript(t, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	a, _, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), strings.Repeat("x", 4000))
	if got := a.ContextTokens(); got < 1000 {
		t.Fatalf("no usage reported: expected the chars/4 fallback, got %d", got)
	}
}

// /clear carries the model; hard mode exits (its saved pair is carried).
func TestCarryModel(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, `"modelHard":"fake/m2"`, `"m2":{"contextWindow":65536}`)
	if m, e := a.Carry(); m != "fake/m" || e != a.Effort() {
		t.Fatalf("carry %s %s", m, e)
	}
	if err := a.SetModel("fake/m2", llm.EffortLow); err != nil {
		t.Fatal(err)
	}
	if m, e := a.Carry(); m != "fake/m2" || e != a.Effort() {
		t.Fatalf("carry after switch %s %s", m, e)
	}
	a2, _, _ := startTestWith(t, newScript(t), `"modelHard":"fake/m2"`, `"m2":{"contextWindow":65536}`)
	before, beforeE := a2.Carry()
	if _, err := a2.ToggleHard(); err != nil {
		t.Fatal(err)
	}
	if m, e := a2.Carry(); m != before || e != beforeE {
		t.Fatalf("hard mode carries the saved pair, got %s %s", m, e)
	}
}

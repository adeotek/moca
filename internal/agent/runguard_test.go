package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
)

// bodyMaxTokens is the output ceiling the agent asked for in request req.
func bodyMaxTokens(t *testing.T, s *scriptServer, req int) int {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if req < 0 || req >= len(s.bodies) {
		t.Fatalf("request %d does not exist (%d recorded)", req, len(s.bodies))
	}
	n, _ := s.bodies[req]["max_tokens"].(float64)
	return int(n)
}

func hasWarning(evs *[]Event, sub string) bool {
	for _, e := range *evs {
		if w, ok := e.(Warning); ok && strings.Contains(w.Text, sub) {
			return true
		}
	}
	return false
}

// TestProgressNudge: a run that has taken the configured number of tool
// batches (the default here) without creating or editing a file gets exactly
// one persisted nudge to start acting (§14; the TB2 review found runs burning
// their whole budget on reconnaissance).
func TestProgressNudge(t *testing.T) {
	var turns []string
	for range config.DefaultProgressNudgeAt {
		turns = append(turns, toolTurn([2]string{"ls", `{}`}))
	}
	turns = append(turns, textTurn("nothing to change here"))
	s := newScript(t, turns...)
	a, _, _ := startTest(t, s, 40)
	out, err := a.Run(context.Background(), "investigate")
	if err != nil || out.Text != "nothing to change here" || s.bodyCount() != config.DefaultProgressNudgeAt+1 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if last := lastMessageJSON(t, s, config.DefaultProgressNudgeAt); !strings.Contains(last, "without creating or editing") {
		t.Fatalf("nudge missing from the request after the recon budget: %s", last)
	}
	a.mu.Lock()
	var kinds []string
	for _, e := range a.entries {
		if e.Nudge != "" {
			kinds = append(kinds, e.Nudge)
		}
	}
	a.mu.Unlock()
	if len(kinds) != 1 || kinds[0] != "progress" {
		t.Fatalf("persisted nudge kinds = %v, want [progress]", kinds)
	}
}

// TestProgressNudgeSkippedAfterAChange: once a file has been created or
// edited, however read-heavy the rest of the run is, the nudge stays away.
func TestProgressNudgeSkippedAfterAChange(t *testing.T) {
	turns := []string{
		toolTurn([2]string{"write", `{"path":"a.go","content":"package a\n"}`},
			[2]string{"shell", `{"command":"test -e a.go"}`}),
	}
	for range config.DefaultProgressNudgeAt - 1 {
		turns = append(turns, toolTurn([2]string{"ls", `{}`}))
	}
	turns = append(turns, textTurn("done"))
	s := newScript(t, turns...)
	a, _, _ := startTest(t, s, 40)
	a.opts.Env.Commands = allowAll{}
	out, err := a.Run(context.Background(), "x")
	if err != nil || out.Text != "done" || s.bodyCount() != config.DefaultProgressNudgeAt+1 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	for i := 0; i < s.bodyCount(); i++ {
		if strings.Contains(lastMessageJSON(t, s, i), "without creating or editing") {
			t.Fatalf("progress nudge must not fire after a change (request %d)", i)
		}
	}
}

// TestProgressNudgeThresholdFromConfig: context.progressNudgeAt sets the
// threshold and the nudge text names the count that actually ran.
func TestProgressNudgeThresholdFromConfig(t *testing.T) {
	var turns []string
	for range 3 {
		turns = append(turns, toolTurn([2]string{"ls", `{}`}))
	}
	turns = append(turns, textTurn("nothing to change here"))
	s := newScript(t, turns...)
	three := 3
	a, _, _ := startTest(t, s, 40, func(o *StartOptions) { o.Config.Context.ProgressNudgeAt = &three })
	out, err := a.Run(context.Background(), "investigate")
	if err != nil || out.Text != "nothing to change here" || s.bodyCount() != 4 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if last := lastMessageJSON(t, s, 3); !strings.Contains(last, "3 tool batches") {
		t.Fatalf("nudge must name the configured count: %s", last)
	}
}

// TestProgressNudgeDisabled: context.progressNudgeAt 0 turns the nudge off —
// a long read-only run finishes with no nudge anywhere in its requests.
func TestProgressNudgeDisabled(t *testing.T) {
	var turns []string
	for range config.DefaultProgressNudgeAt + 2 {
		turns = append(turns, toolTurn([2]string{"ls", `{}`}))
	}
	turns = append(turns, textTurn("done"))
	s := newScript(t, turns...)
	zero := 0
	a, _, _ := startTest(t, s, 40, func(o *StartOptions) { o.Config.Context.ProgressNudgeAt = &zero })
	out, err := a.Run(context.Background(), "investigate")
	if err != nil || out.Text != "done" || s.bodyCount() != config.DefaultProgressNudgeAt+3 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	for i := 0; i < s.bodyCount(); i++ {
		if strings.Contains(lastMessageJSON(t, s, i), "without creating or editing") {
			t.Fatalf("disabled progress nudge fired (request %d)", i)
		}
	}
}

// TestVerifyNudge: a run that changed a code file and ends without a
// verifying shell command since gets exactly one persisted nudge (C1).
func TestVerifyNudge(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"write", `{"path":"a.go","content":"package a\n"}`}),
		textTurn("done"),
		textTurn("nothing to build here"),
	)
	a, _, _ := startTest(t, s, 40)
	out, err := a.Run(context.Background(), "add a.go")
	if err != nil || out.Text != "nothing to build here" || s.bodyCount() != 3 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if last := lastMessageJSON(t, s, 2); !strings.Contains(last, "a.go") || !strings.Contains(last, "not built or tested") {
		t.Fatalf("nudge: %s", last)
	}
}

// TestVerifyNudgeSkippedWhenVerifiedOrDocsOnly: a verifying shell call after
// the last change, or a docs-only change, ends the run without a nudge.
func TestVerifyNudgeSkippedWhenVerifiedOrDocsOnly(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"write", `{"path":"a.go","content":"package a\n"}`}),
		toolTurn([2]string{"shell", `{"command":"test -e a.go"}`}),
		textTurn("done"),
	)
	a, _, _ := startTest(t, s, 40)
	a.opts.Env.Commands = allowAll{}
	if out, err := a.Run(context.Background(), "x"); err != nil || out.Text != "done" || s.bodyCount() != 3 {
		t.Fatalf("verified: %+v %v requests=%d", out, err, s.bodyCount())
	}
	s2 := newScript(t,
		toolTurn([2]string{"write", `{"path":"NOTES.md","content":"x\n"}`}),
		textTurn("done"),
	)
	a2, _, _ := startTest(t, s2, 40)
	if out, err := a2.Run(context.Background(), "x"); err != nil || out.Text != "done" || s2.bodyCount() != 2 {
		t.Fatalf("docs only: %+v %v requests=%d", out, err, s2.bodyCount())
	}
}

// TestStuckRunStops: the same failing call five times ends the run through
// the tool-less wrap-up with a warning (C2); the 3rd failure already hints.
func TestStuckRunStops(t *testing.T) {
	var turns []string
	for range 5 {
		turns = append(turns, toolTurn([2]string{"read", `{"path":"missing.go"}`}))
	}
	turns = append(turns, textTurn("I am blocked: missing.go does not exist."))
	s := newScript(t, turns...)
	a, _, evs := startTest(t, s, 40)
	out, err := a.Run(context.Background(), "read it")
	if err != nil || !out.Stuck || s.bodyCount() != 6 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if !hasWarning(evs, "same failing call") {
		t.Fatal("stuck warning missing")
	}
	if !strings.Contains(lastMessageJSON(t, s, 3), "failed 3 times") {
		t.Fatal("the 3rd identical failure must carry the hint")
	}
	if !strings.Contains(lastMessageJSON(t, s, 5), "Do not call tools") {
		t.Fatal("the stop goes through the tool-less wrap-up")
	}
}

// TestPlanModeReserveBeforeWrapUp: a long plan run is told to write its file
// two steps before the tool-less wrap-up (C5) — and can.
func TestPlanModeReserveBeforeWrapUp(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"ls", `{}`}),
		toolTurn([2]string{"ls", `{}`}),
		toolTurn([2]string{"write", `{"path":"docs/plans/p.md","content":"# p\n- [ ] s\n"}`}),
		textTurn("Plan in docs/plans/p.md"),
	)
	a, _, evs := startTest(t, s, 4, planMode)
	out, err := a.Run(context.Background(), "plan it")
	if err != nil || out.MaxSteps || out.Text != "Plan in docs/plans/p.md" {
		t.Fatalf("%+v %v", out, err)
	}
	if !strings.Contains(lastMessageJSON(t, s, 2), "running out of steps") {
		t.Fatalf("reserve message missing: %s", lastMessageJSON(t, s, 2))
	}
	if hasWarning(evs, "without a plan file") {
		t.Fatal("the plan landed; no warning")
	}
}

// TestTruncatedTurnGetsAnActingRetry: a turn cut off at the output limit with no
// tool call is not an answer (TB2 regex-chess 2026-10-27: three turns of 39k-61k
// characters of thinking, StopLength, nothing written). One retry that forbids
// deliberation and raises the ceiling, then the run continues.
func TestTruncatedTurnGetsAnActingRetry(t *testing.T) {
	s := newScript(t,
		truncatedTurn(""),
		toolTurn([2]string{"write", `{"path":"NOTES.md","content":"x\n"}`}),
		textTurn("done"),
	)
	a, _, evs := startTestWith(t, s, `"model":"fake/m2"`, `"m2":{"contextWindow":65536,"maxOutputTokens":65536}`)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || out.Text != "done" || s.bodyCount() != 3 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if msg := lastMessageJSON(t, s, 1); !strings.Contains(msg, "output token limit") {
		t.Fatalf("the retry must name the truncation: %s", msg)
	}
	if !hasWarning(evs, "retry 1 of 3") {
		t.Fatal("the rescue must be visible")
	}
	// §14: a truncated turn is a thought (or a write) cut in half, so the retry
	// runs with more room — not less.
	first, retry := bodyMaxTokens(t, s, 0), bodyMaxTokens(t, s, 1)
	if retry <= first {
		t.Fatalf("the retry ceiling must rise: %d then %d", first, retry)
	}
	if after := bodyMaxTokens(t, s, 2); after != first {
		t.Fatalf("the raised ceiling must not outlive the rescue: %d then %d", first, after)
	}
}

// TestTruncatedTurnIsBounded: three rescues without progress, then the run ends
// with the cause named — the rescue is bounded, never a loop.
func TestTruncatedTurnIsBounded(t *testing.T) {
	s := newScript(t, truncatedTurn(""), truncatedTurn(""), truncatedTurn(""), truncatedTurn(""))
	a, _, evs := startTestWith(t, s, `"model":"fake/m2"`, `"m2":{"contextWindow":65536,"maxOutputTokens":65536}`)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || s.bodyCount() != 4 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if out.Text != "" {
		t.Fatalf("nothing was produced, got %q", out.Text)
	}
	if !hasWarning(evs, "kept hitting the output token limit") {
		t.Fatal("the bound must warn")
	}
	if bodyMaxTokens(t, s, 1) <= bodyMaxTokens(t, s, 0) {
		t.Fatal("the first rescue must raise the ceiling")
	}
	for i := 2; i < 4; i++ {
		if bodyMaxTokens(t, s, i) < bodyMaxTokens(t, s, i-1) {
			t.Fatalf("attempt %d must not lower the ceiling", i)
		}
	}
}

// TestTruncationProgressResetsTheRescueBudget: work between truncations is
// progress, so a long run may truncate once per phase without being cut off.
func TestTruncationProgressResetsTheRescueBudget(t *testing.T) {
	s := newScript(t,
		truncatedTurn(""), toolTurn([2]string{"read", `{"path":"a.md"}`}),
		truncatedTurn(""), toolTurn([2]string{"read", `{"path":"b.md"}`}),
		truncatedTurn(""), toolTurn([2]string{"read", `{"path":"c.md"}`}),
		truncatedTurn(""), toolTurn([2]string{"read", `{"path":"d.md"}`}),
		textTurn("done"),
	)
	a, _, evs := startTest(t, s, 40)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || out.Text != "done" || s.bodyCount() != 9 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if hasWarning(evs, "kept hitting the output token limit") {
		t.Fatal("progress must reset the rescue budget")
	}
}

// TestTruncatedTextOnlyTurnIsRetried: a half-written answer cut off at the
// limit is incomplete, not final.
func TestTruncatedTextOnlyTurnIsRetried(t *testing.T) {
	s := newScript(t, truncatedTurn("the answer is 4"), textTurn("done"))
	a, _, _ := startTest(t, s, 40)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || out.Text != "done" || s.bodyCount() != 2 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if msg := lastMessageJSON(t, s, 1); !strings.Contains(msg, "output token limit") {
		t.Fatalf("nudge: %s", msg)
	}
}

// TestRescueAtTheStepLimitDoesNotLeak: a rescue that lands on the last step is
// spent by the step-limit wrap-up, which returns without passing the loop's
// clear — the next run must start at the session's own effort.
func TestRescueAtTheStepLimitDoesNotLeak(t *testing.T) {
	s := newScript(t, truncatedTurn(""), textTurn("wrapped up"))
	a, _, _ := startTest(t, s, 1)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || !out.MaxSteps || s.bodyCount() != 2 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if a.rescueEffort != "" || a.rescueMaxTokens != 0 || a.turnEffort() != a.effort {
		t.Fatalf("the rescue outlived its run: rescue=%q turn=%q session=%q", a.rescueEffort, a.turnEffort(), a.effort)
	}
}

// TestRescueBudgetEscalatesAndStops: double, quadruple, eight times the configured
// ceiling, never past the model's own limit, and nothing when there is no headroom.
func TestRescueBudgetEscalatesAndStops(t *testing.T) {
	const reserve = 16384
	big := provider.Model{MaxOutput: 384000}
	for _, c := range []struct{ attempt, want int }{{1, 32768}, {2, 65536}, {3, 131072}} {
		if got := rescueBudget(big, reserve, c.attempt); got != c.want {
			t.Fatalf("attempt %d = %d, want %d", c.attempt, got, c.want)
		}
	}
	small := provider.Model{MaxOutput: 20000}
	if got := rescueBudget(small, reserve, 1); got != 20000 {
		t.Fatalf("capped at the model limit, got %d", got)
	}
	if got := rescueBudget(small, reserve, 3); got != 20000 {
		t.Fatalf("still capped at the model limit, got %d", got)
	}
	tight := provider.Model{MaxOutput: reserve}
	if got := rescueBudget(tight, reserve, 1); got != 0 {
		t.Fatalf("no headroom must mean no override, got %d", got)
	}
}

// TestRescueLevelClampsDown: the retry runs at low, or at the session's own
// level when that is already below low, clamped for the model.
func TestRescueLevelClampsDown(t *testing.T) {
	full := provider.Model{ThinkingMode: "openai", ThinkingLevelMap: map[llm.Effort]string{
		llm.EffortOff: "none", llm.EffortMinimal: "minimal", llm.EffortLow: "low",
		llm.EffortMedium: "medium", llm.EffortHigh: "high",
	}}
	plain := provider.Model{} // no levels: clamping is a no-op
	for _, c := range []struct{ in, want llm.Effort }{
		{llm.EffortMedium, llm.EffortLow},
		{llm.EffortHigh, llm.EffortLow},
		{llm.EffortMax, llm.EffortLow},
		{llm.EffortLow, llm.EffortLow},
		{llm.EffortMinimal, llm.EffortMinimal},
		{llm.EffortOff, llm.EffortOff},
	} {
		if got := rescueLevel(c.in, full); got != c.want {
			t.Fatalf("rescueLevel(%s) = %s, want %s", c.in, got, c.want)
		}
	}
	if got := rescueLevel(llm.EffortHigh, plain); got != llm.EffortOff {
		t.Fatalf("a model without levels clamps to off, got %s", got)
	}
}

// TestEmptyTurnIsRetriedOnce: a turn with no text and no tool calls is a
// provider/model glitch, not an answer (eval largefile#1 ended "done" with
// tests still failing) — one bounded nudge, then the run may end.
func TestEmptyTurnIsRetriedOnce(t *testing.T) {
	s := newScript(t, textTurn(""), textTurn("fixed it"))
	a, _, _ := startTest(t, s, 40)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || out.Text != "fixed it" || s.bodyCount() != 2 {
		t.Fatalf("%+v %v requests=%d", out, err, s.bodyCount())
	}
	if !strings.Contains(lastMessageJSON(t, s, 1), "Your last reply was empty") {
		t.Fatalf("nudge: %s", lastMessageJSON(t, s, 1))
	}
	s2 := newScript(t, textTurn(""), textTurn(""))
	a2, _, evs := startTest(t, s2, 40)
	if out, err := a2.Run(context.Background(), "fix it"); err != nil || out.Text != "" || s2.bodyCount() != 2 {
		t.Fatalf("bounded: %+v %v requests=%d", out, err, s2.bodyCount())
	}
	if !hasWarning(evs, "empty") {
		t.Fatal("a second empty end must warn")
	}
}

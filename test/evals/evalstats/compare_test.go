package main

import (
	"reflect"
	"strings"
	"testing"
)

// run builds one result row: tokens exactly n (Input), passes as given.
func run(scen string, pass bool, tokens int, cost float64, steps int) Metrics {
	return Metrics{Scenario: scen, Pass: pass, Input: tokens, Cost: cost, Steps: steps}
}

func corpus(scale float64) []Metrics {
	mk := func(scen string, pass bool, tokens, steps int) Metrics {
		return run(scen, pass, int(float64(tokens)*scale), float64(tokens)*scale/100000, steps)
	}
	return []Metrics{
		mk("a", true, 1000, 5), mk("a", true, 1200, 6), mk("a", false, 1500, 9),
		mk("b", true, 800, 4), mk("b", true, 900, 5), mk("b", false, 1100, 7),
	}
}

func TestCompareIdenticalIsInconclusive(t *testing.T) {
	base := corpus(1)
	cand := corpus(1)
	c := Compare(base, cand, 500, 1)
	o := c.Overall.TokensPerSolved
	if !(o.Lo <= 1.0 && 1.0 <= o.Hi) {
		t.Fatalf("identical inputs must straddle 1.0: [%v, %v]", o.Lo, o.Hi)
	}
	if !strings.HasPrefix(c.Verdict, "inconclusive") {
		t.Fatalf("verdict = %q, want inconclusive", c.Verdict)
	}
}

func TestCompareClearWin(t *testing.T) {
	base := corpus(1)
	cand := corpus(0.5) // half the tokens per run, same passes
	c := Compare(base, cand, 500, 1)
	if !strings.HasPrefix(c.Verdict, "better") {
		t.Fatalf("verdict = %q, want better (CI %v)", c.Verdict, c.Overall.TokensPerSolved)
	}
}

func TestComparePassDropIsWorse(t *testing.T) {
	base := corpus(1)
	cand := corpus(1)
	cand[0].Pass = false // two more losses: 4 passes -> 2
	cand[1].Pass = false
	c := Compare(base, cand, 500, 1)
	if !strings.HasPrefix(c.Verdict, "worse") {
		t.Fatalf("verdict = %q, want worse", c.Verdict)
	}
}

func TestCompareDeterministic(t *testing.T) {
	base, cand := corpus(1), corpus(0.8)
	a := Compare(base, cand, 300, 7)
	b := Compare(base, cand, 300, 7)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same inputs and seed must give identical comparisons")
	}
}

func TestCompareUnderpoweredMinRuns(t *testing.T) {
	base := corpus(1)[:2] // scenario a has 2 runs
	cand := corpus(1)[:2]
	c := Compare(base, cand, 200, 1)
	if !strings.HasPrefix(c.Verdict, "underpowered") || !strings.Contains(c.Verdict, "a (") {
		t.Fatalf("verdict = %q, want the min-runs refusal naming scenario a", c.Verdict)
	}
}

func TestCompareUnderpoweredSpansBand(t *testing.T) {
	// Equal outcomes, wildly varying token volumes: the point estimate is a
	// clear 1.3, but the interval is too wide to confirm it at n=3.
	base := []Metrics{
		run("a", true, 1000, 0.01, 5), run("a", false, 2000, 0.02, 6), run("a", false, 9000, 0.09, 12),
	}
	cand := []Metrics{
		run("a", true, 1300, 0.013, 5), run("a", false, 2600, 0.026, 6), run("a", false, 11700, 0.117, 12),
	}
	c := Compare(base, cand, 500, 1)
	if !strings.HasPrefix(c.Verdict, "underpowered") || !strings.Contains(c.Verdict, "spans the band") {
		t.Fatalf("verdict = %q, want the spans-band underpowered read (CI %v)", c.Verdict, c.Overall.TokensPerSolved)
	}
}

func TestRenderComparisonShowsVerdict(t *testing.T) {
	out := RenderComparison(Compare(corpus(1), corpus(0.5), 100, 1))
	for _, want := range []string{"| scenario |", "∑ all", "**Verdict: better"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

// mkRun is the compact row builder the pinning tests use.
func mkRun(scen string, tokens int) Metrics {
	return Metrics{Scenario: scen, Pass: true, Input: tokens, Cost: float64(tokens) / 100000, Steps: 5}
}

// A tight interval pins the size: the row reports cv and the runs needed, and
// the verdict carries no caveat.
func TestComparePinsATightInterval(t *testing.T) {
	base := []Metrics{mkRun("a", 1000), mkRun("a", 1010), mkRun("a", 990), mkRun("a", 1005), mkRun("a", 995), mkRun("a", 1000)}
	cand := []Metrics{mkRun("a", 900), mkRun("a", 909), mkRun("a", 891), mkRun("a", 904), mkRun("a", 895), mkRun("a", 900)}
	c := Compare(base, cand, 2000, 1)
	if !c.Overall.Pinned {
		t.Fatalf("a 10%% effect at cv %.2f must pin, got need=%d", c.Overall.CV, c.Overall.NeededN)
	}
	if !strings.HasPrefix(c.Verdict, "better") || strings.Contains(c.Verdict, "no size claim") {
		t.Fatalf("verdict = %q, want an unconditional better", c.Verdict)
	}
	if out := RenderComparison(c); !strings.Contains(out, "| cv |") || !strings.Contains(out, "✓") {
		t.Fatalf("the table must show cv and a pinned need:\n%s", out)
	}
}

// The release A/B shape: a direction the interval supports and a size it does
// not. The verdict keeps the direction and refuses the percentage.
func TestCompareRefusesTheSizeWhenUnpinned(t *testing.T) {
	var base, cand []Metrics
	for _, n := range []int{1000, 1800, 2600, 3400, 900, 2000} {
		base = append(base, mkRun("a", n))
	}
	for _, n := range []int{500, 900, 1300, 1700, 450, 1000} {
		cand = append(cand, mkRun("a", n))
	}
	c := Compare(base, cand, 2000, 1)
	if c.Overall.Pinned {
		t.Fatalf("a 50%% effect at cv %.2f cannot be pinned at 6 runs", c.Overall.CV)
	}
	if !strings.HasPrefix(c.Verdict, "better") {
		t.Fatalf("the direction must survive, verdict = %q", c.Verdict)
	}
	for _, want := range []string{"direction only, no size claim", "runs/arm would pin it"} {
		if !strings.Contains(c.Verdict, want) {
			t.Fatalf("verdict must refuse the size (%q missing): %s", want, c.Verdict)
		}
	}
}

// pinning is the arithmetic the verdict reads: n runs scale a half-width h by
// 1/sqrt(n), so pinning an effect to ±50 % needs n*(h/(effect/2))^2.
func TestPinningScalesWithTheInterval(t *testing.T) {
	d := ScenarioDelta{Runs: [2]int{6, 6}}
	d.TokensPerSolved = Ratio{Point: 0.90, Lo: 0.72, Hi: 0.98}
	pinning(&d)
	if d.Pinned {
		t.Fatal("half-width 0.18 on a 0.10 effect is not pinned")
	}
	// 6 * (0.18/0.05)^2 = 77.76 -> 78
	if d.NeededN != 78 {
		t.Fatalf("needed n = %d, want 78", d.NeededN)
	}
	tight := ScenarioDelta{Runs: [2]int{6, 6}}
	tight.TokensPerSolved = Ratio{Point: 0.90, Lo: 0.88, Hi: 0.92}
	pinning(&tight)
	if !tight.Pinned || tight.NeededN > 6 {
		t.Fatalf("a ±0.02 interval on a 0.10 effect pins at 6 runs: pinned=%v need=%d", tight.Pinned, tight.NeededN)
	}
}

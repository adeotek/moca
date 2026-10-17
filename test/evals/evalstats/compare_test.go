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

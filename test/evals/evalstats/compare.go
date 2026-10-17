package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strings"
)

// Ratio is a point estimate and its 95% percentile confidence interval
// (tokens-per-solved, cost-per-run and steps-per-run, candidate / base).
type Ratio struct{ Point, Lo, Hi float64 }

func (r Ratio) String() string {
	if math.IsNaN(r.Point) {
		return "—"
	}
	if math.IsInf(r.Point, 0) {
		return "∞"
	}
	return fmt.Sprintf("%.2f [%.2f, %.2f]", r.Point, r.Lo, r.Hi)
}

// ScenarioDelta compares one scenario (or the whole corpus) between two labels.
type ScenarioDelta struct {
	Name            string
	PassBase        int
	PassCand        int
	Runs            [2]int // {base, candidate}
	TokensPerSolved Ratio
	CostPerRun      Ratio
	Steps           Ratio
}

// Comparison is the base-vs-candidate read of two labeled result files.
type Comparison struct {
	Scenarios []ScenarioDelta
	Overall   ScenarioDelta
	Verdict   string
}

type totals struct {
	tokens, passes, runs int
	cost, steps          float64
}

func (t *totals) add(m Metrics) {
	t.tokens += m.Tokens()
	if m.Pass {
		t.passes++
	}
	t.runs++
	t.cost += m.Cost
	t.steps += float64(m.Steps)
}

type ratios struct{ tps, cost, steps float64 }

// ratioOf reports candidate/base. Tokens-per-solved is the primary lever; a
// side that solved nothing while the other solved something reads as the best
// (0) or worst (∞) possible ratio.
func ratioOf(b, c totals) ratios {
	r := ratios{}
	switch {
	case b.passes == 0 && c.passes == 0:
		r.tps = 1
	case b.passes == 0:
		r.tps = 0
	case c.passes == 0:
		r.tps = math.Inf(1)
	default:
		r.tps = (float64(c.tokens) / float64(c.passes)) / (float64(b.tokens) / float64(b.passes))
	}
	r.cost, r.steps = math.NaN(), math.NaN()
	if b.cost > 0 && b.runs > 0 {
		r.cost = (c.cost / float64(c.runs)) / (b.cost / float64(b.runs))
	}
	if b.steps > 0 && b.runs > 0 {
		r.steps = (c.steps / float64(c.runs)) / (b.steps / float64(b.runs))
	}
	return r
}

// minRuns is the review-pass floor: a bootstrap over fewer than three runs per
// side cannot say anything, so the comparison refuses a verdict instead.
const minRuns = 3

// Compare bootstraps base vs candidate, stratified by scenario: within each
// scenario the runs are resampled with replacement, for both sides
// independently, and the per-iteration ratios combine into the overall read.
// The RNG is seeded, so identical inputs and seed give identical output.
func Compare(base, cand []Metrics, iters int, seed int64) Comparison {
	byScenario := func(rows []Metrics) (map[string][]Metrics, []string) {
		m := map[string][]Metrics{}
		var names []string
		for _, r := range rows {
			if _, ok := m[r.Scenario]; !ok {
				names = append(names, r.Scenario)
			}
			m[r.Scenario] = append(m[r.Scenario], r)
		}
		return m, names
	}
	baseBy, bNames := byScenario(base)
	candBy, cNames := byScenario(cand)
	seen := map[string]bool{}
	var names []string
	for _, n := range append(bNames, cNames...) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)

	var thin []string
	for _, n := range names {
		if len(baseBy[n]) < minRuns || len(candBy[n]) < minRuns {
			thin = append(thin, fmt.Sprintf("%s (%d vs %d runs)", n, len(baseBy[n]), len(candBy[n])))
		}
	}

	point := func() (map[string]ScenarioDelta, ScenarioDelta, totals, totals) {
		deltas := map[string]ScenarioDelta{}
		var ob, oc totals
		for _, n := range names {
			var tb, tc totals
			for _, m := range baseBy[n] {
				tb.add(m)
			}
			for _, m := range candBy[n] {
				tc.add(m)
			}
			ob.tokens, ob.passes, ob.runs = ob.tokens+tb.tokens, ob.passes+tb.passes, ob.runs+tb.runs
			ob.cost, ob.steps = ob.cost+tb.cost, ob.steps+tb.steps
			oc.tokens, oc.passes, oc.runs = oc.tokens+tc.tokens, oc.passes+tc.passes, oc.runs+tc.runs
			oc.cost, oc.steps = oc.cost+tc.cost, oc.steps+tc.steps
			r := ratioOf(tb, tc)
			deltas[n] = ScenarioDelta{Name: n, PassBase: tb.passes, PassCand: tc.passes,
				Runs: [2]int{tb.runs, tc.runs}, TokensPerSolved: Ratio{Point: r.tps},
				CostPerRun: Ratio{Point: r.cost}, Steps: Ratio{Point: r.steps}}
		}
		ro := ratioOf(ob, oc)
		return deltas, ScenarioDelta{Name: "∑ all", PassBase: ob.passes, PassCand: oc.passes,
			Runs: [2]int{ob.runs, oc.runs}, TokensPerSolved: Ratio{Point: ro.tps},
			CostPerRun: Ratio{Point: ro.cost}, Steps: Ratio{Point: ro.steps}}, ob, oc
	}

	deltas, overall, _, _ := point()
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	bPass, cPass := map[string]int{}, map[string]int{}
	for _, n := range names {
		for _, m := range baseBy[n] {
			if m.Pass {
				bPass[n]++
			}
		}
		for _, m := range candBy[n] {
			if m.Pass {
				cPass[n]++
			}
		}
	}
	// Pass counts are held at their observed values: the CI measures token
	// volume uncertainty, while outcome changes are judged by the explicit
	// pass rules below. Redrawing the outcomes too would let a single drawn
	// failure widen the interval enough to hide a real token win.
	resample := func(ms []Metrics, passes int) totals {
		var t totals
		for range ms {
			t.add(ms[rng.IntN(len(ms))])
		}
		t.passes = passes
		return t
	}
	push := func(dst *[]float64, v float64) {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			*dst = append(*dst, v)
		}
	}
	pushMap := func(m map[string][]float64, k string, v float64) {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			m[k] = append(m[k], v)
		}
	}
	tpsAcc, costAcc, stepsAcc := map[string][]float64{}, map[string][]float64{}, map[string][]float64{}
	var oTps, oCost, oSteps []float64
	unusable := 0
	for it := 0; it < iters; it++ {
		perB, perC := map[string]totals{}, map[string]totals{}
		var ob2, oc2 totals
		for _, n := range names {
			tb, tc := resample(baseBy[n], bPass[n]), resample(candBy[n], cPass[n])
			perB[n], perC[n] = tb, tc
			ob2.tokens, ob2.passes, ob2.runs = ob2.tokens+tb.tokens, ob2.passes+tb.passes, ob2.runs+tb.runs
			ob2.cost, ob2.steps = ob2.cost+tb.cost, ob2.steps+tb.steps
			oc2.tokens, oc2.passes, oc2.runs = oc2.tokens+tc.tokens, oc2.passes+tc.passes, oc2.runs+tc.runs
			oc2.cost, oc2.steps = oc2.cost+tc.cost, oc2.steps+tc.steps
		}
		for _, n := range names {
			r := ratioOf(perB[n], perC[n])
			pushMap(tpsAcc, n, r.tps)
			pushMap(costAcc, n, r.cost)
			pushMap(stepsAcc, n, r.steps)
		}
		ro := ratioOf(ob2, oc2)
		if math.IsInf(ro.tps, 0) {
			unusable++
		} else {
			push(&oTps, ro.tps)
		}
		push(&oCost, ro.cost)
		push(&oSteps, ro.steps)
	}
	ci := func(xs []float64) (float64, float64) {
		if len(xs) == 0 {
			return math.NaN(), math.NaN()
		}
		s := append([]float64(nil), xs...)
		sort.Float64s(s)
		lo := s[int(0.025*float64(len(s)))]
		hi := s[min(len(s)-1, int(0.975*float64(len(s))))]
		return lo, hi
	}
	for _, n := range names {
		d := deltas[n]
		d.TokensPerSolved.Lo, d.TokensPerSolved.Hi = ci(tpsAcc[n])
		d.CostPerRun.Lo, d.CostPerRun.Hi = ci(costAcc[n])
		d.Steps.Lo, d.Steps.Hi = ci(stepsAcc[n])
		deltas[n] = d
	}
	overall.TokensPerSolved.Lo, overall.TokensPerSolved.Hi = ci(oTps)
	overall.CostPerRun.Lo, overall.CostPerRun.Hi = ci(oCost)
	overall.Steps.Lo, overall.Steps.Hi = ci(oSteps)

	var scen []ScenarioDelta
	for _, n := range names {
		scen = append(scen, deltas[n])
	}
	c := Comparison{Scenarios: scen, Overall: overall}
	o := overall.TokensPerSolved
	switch {
	case len(thin) > 0:
		c.Verdict = "underpowered — fewer than " + fmt.Sprint(minRuns) + " runs for " +
			strings.Join(thin, ", ") + "; run those scenarios at -n " + fmt.Sprint(minRuns) + " first"
	case math.IsNaN(o.Lo) || math.IsNaN(o.Hi):
		c.Verdict = fmt.Sprintf("underpowered — no usable resamples (%d/%d had no solved run on a side)", unusable, iters)
	case o.Hi < 1.0 && overall.PassCand >= overall.PassBase-1:
		c.Verdict = fmt.Sprintf("better — tokens/solved Hi %.2f < 1.00 with passes %d→%d", o.Hi, overall.PassBase, overall.PassCand)
	case o.Lo > 1.05 || overall.PassCand <= overall.PassBase-2:
		c.Verdict = fmt.Sprintf("worse — tokens/solved Lo %.2f%s with passes %d→%d", o.Lo,
			map[bool]string{true: " > 1.05", false: ""}[o.Lo > 1.05], overall.PassBase, overall.PassCand)
	case o.Lo <= 1.0 && o.Hi >= 1.05 && math.Abs(o.Point-1) >= 0.05:
		c.Verdict = fmt.Sprintf("underpowered — CI [%.2f, %.2f] spans the band; rerun the moved scenarios with -n 6", o.Lo, o.Hi)
	default:
		c.Verdict = fmt.Sprintf("inconclusive — point %.2f, CI [%.2f, %.2f]; no material effect at this n", o.Point, o.Lo, o.Hi)
	}
	return c
}

// RenderComparison prints the per-scenario table and the verdict.
func RenderComparison(c Comparison) string {
	var sb strings.Builder
	sb.WriteString("| scenario | passes base→cand | runs | tokens/solved [lo, hi] | cost/run [lo, hi] | steps [lo, hi] |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")
	row := func(d ScenarioDelta) {
		fmt.Fprintf(&sb, "| %s | %d→%d | %d→%d | %s | %s | %s |\n",
			d.Name, d.PassBase, d.PassCand, d.Runs[0], d.Runs[1],
			d.TokensPerSolved, d.CostPerRun, d.Steps)
	}
	for _, d := range c.Scenarios {
		row(d)
	}
	row(c.Overall)
	fmt.Fprintf(&sb, "\n**Verdict: %s**\n", c.Verdict)
	return sb.String()
}

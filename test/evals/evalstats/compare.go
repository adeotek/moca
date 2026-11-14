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
	// ChurnPerRun is lines added+removed per run, candidate / base. It is
	// REPORTED only: the verdict switch below reads TokensPerSolved and the
	// pass counts, so a size change can never flip a ship decision on its own.
	// ChurnMeasured is false when the base side carries no size data (a label
	// recorded before the fields existed, so there is nothing to divide by) —
	// the row then prints "—" rather than a NaN, because a NaN inside this
	// struct makes two identical Comparisons compare unequal.
	ChurnPerRun   Ratio
	ChurnMeasured bool
	// CV is the pooled per-run spread of token volume (sd/mean over both
	// sides): the number that says how much of this row is noise. Effect is
	// |point-1|; NeededN is the runs per arm that would pin that effect to
	// within a factor of two, derived from the bootstrap's own interval width
	// (a half-width h around n runs shrinks as 1/sqrt(n), so pinning needs
	// n*(h/(effect/2))^2). Pinned is the same statement as NeededN <= runs.
	//
	// Why two numbers and not one: the interval answers "which way", the
	// pinning answers "how much". They are different claims, and conflating
	// them is how a 13 % read at n=6 got reported as a 13 % saving.
	CV      float64
	Effect  float64
	NeededN int
	Pinned  bool
}

// Comparison is the base-vs-candidate read of two labeled result files.
type Comparison struct {
	Scenarios []ScenarioDelta
	Overall   ScenarioDelta
	Verdict   string
}

type totals struct {
	tokens, passes, runs int
	cost, steps, lines   float64
}

func (t *totals) add(m Metrics) {
	t.tokens += m.Tokens()
	if m.Pass {
		t.passes++
	}
	t.runs++
	t.cost += m.Cost
	t.steps += float64(m.Steps)
	t.lines += float64(m.Churn())
}

type ratios struct {
	tps, cost, steps, lines float64
	linesSet                bool
}

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
	// A base that wrote nothing gives no ratio to divide by (an unmeasured
	// side, or a row from before the size fields existed): leave the ratio
	// unset, and the row renders "—".
	if b.lines > 0 && b.runs > 0 {
		r.lines = (c.lines / float64(c.runs)) / (b.lines / float64(b.runs))
		r.linesSet = true
	}
	return r
}

// minRuns is the review-pass floor: a bootstrap over fewer than three runs per
// side cannot say anything, so the comparison refuses a verdict instead.
const minRuns = 3

// cvOf is sd/mean; NaN when there is nothing to spread.
func cvOf(xs []float64) float64 {
	if len(xs) < 2 {
		return math.NaN()
	}
	mu, sd := meanSD(xs)
	if mu == 0 || math.IsNaN(sd) {
		return math.NaN()
	}
	return sd / mu
}

// pooledCV is the token-volume spread of both sides together: a per-row noise
// figure that does not depend on which arm produced it.
func pooledCV(base, cand []Metrics) float64 {
	xs := make([]float64, 0, len(base)+len(cand))
	for _, r := range append(append([]Metrics{}, base...), cand...) {
		xs = append(xs, float64(r.Tokens()))
	}
	return cvOf(xs)
}

// pinning fills Effect, NeededN and Pinned from the bootstrap interval the row
// already has. It adds no assumption the comparison did not make: the interval
// width is what it is, and n runs scale it by 1/sqrt(n).
func pinning(d *ScenarioDelta) {
	p := d.TokensPerSolved.Point
	if math.IsNaN(p) || math.IsInf(p, 0) || math.IsNaN(d.TokensPerSolved.Lo) {
		return
	}
	d.Effect = math.Abs(p - 1)
	hw := math.Max(d.TokensPerSolved.Hi-p, p-d.TokensPerSolved.Lo)
	per := min(d.Runs[0], d.Runs[1])
	if per < 1 || math.IsNaN(hw) || hw <= 0 {
		return
	}
	// Pinned = the interval already puts the effect within a factor of two:
	// half-width <= effect/2. Anything looser is a direction, not a size.
	d.Pinned = hw <= d.Effect/2
	if d.Effect < 0.02 {
		return // no effect to pin: the row is at parity, not unpinned
	}
	n := float64(per) * math.Pow(hw/(d.Effect/2), 2)
	if n > 9999 {
		n = 9999
	}
	d.NeededN = max(minRuns, int(math.Ceil(n)))
}

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
			ob.cost, ob.steps, ob.lines = ob.cost+tb.cost, ob.steps+tb.steps, ob.lines+tb.lines
			oc.tokens, oc.passes, oc.runs = oc.tokens+tc.tokens, oc.passes+tc.passes, oc.runs+tc.runs
			oc.cost, oc.steps, oc.lines = oc.cost+tc.cost, oc.steps+tc.steps, oc.lines+tc.lines
			r := ratioOf(tb, tc)
			deltas[n] = ScenarioDelta{Name: n, PassBase: tb.passes, PassCand: tc.passes,
				Runs: [2]int{tb.runs, tc.runs}, TokensPerSolved: Ratio{Point: r.tps},
				CostPerRun: Ratio{Point: r.cost}, Steps: Ratio{Point: r.steps},
				ChurnPerRun: Ratio{Point: r.lines}, ChurnMeasured: r.linesSet}
		}
		ro := ratioOf(ob, oc)
		return deltas, ScenarioDelta{Name: "∑ all", PassBase: ob.passes, PassCand: oc.passes,
			Runs: [2]int{ob.runs, oc.runs}, TokensPerSolved: Ratio{Point: ro.tps},
			CostPerRun: Ratio{Point: ro.cost}, Steps: Ratio{Point: ro.steps},
			ChurnPerRun: Ratio{Point: ro.lines}, ChurnMeasured: ro.linesSet}, ob, oc
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
	linesAcc := map[string][]float64{}
	var oTps, oCost, oSteps, oLines []float64
	unusable := 0
	for it := 0; it < iters; it++ {
		perB, perC := map[string]totals{}, map[string]totals{}
		var ob2, oc2 totals
		for _, n := range names {
			tb, tc := resample(baseBy[n], bPass[n]), resample(candBy[n], cPass[n])
			perB[n], perC[n] = tb, tc
			ob2.tokens, ob2.passes, ob2.runs = ob2.tokens+tb.tokens, ob2.passes+tb.passes, ob2.runs+tb.runs
			ob2.cost, ob2.steps, ob2.lines = ob2.cost+tb.cost, ob2.steps+tb.steps, ob2.lines+tb.lines
			oc2.tokens, oc2.passes, oc2.runs = oc2.tokens+tc.tokens, oc2.passes+tc.passes, oc2.runs+tc.runs
			oc2.cost, oc2.steps, oc2.lines = oc2.cost+tc.cost, oc2.steps+tc.steps, oc2.lines+tc.lines
		}
		for _, n := range names {
			r := ratioOf(perB[n], perC[n])
			pushMap(tpsAcc, n, r.tps)
			pushMap(costAcc, n, r.cost)
			pushMap(stepsAcc, n, r.steps)
			if r.linesSet {
				pushMap(linesAcc, n, r.lines)
			}
		}
		ro := ratioOf(ob2, oc2)
		if math.IsInf(ro.tps, 0) {
			unusable++
		} else {
			push(&oTps, ro.tps)
		}
		push(&oCost, ro.cost)
		push(&oSteps, ro.steps)
		if ro.linesSet {
			push(&oLines, ro.lines)
		}
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
		d.CV = pooledCV(baseBy[n], candBy[n])
		d.TokensPerSolved.Lo, d.TokensPerSolved.Hi = ci(tpsAcc[n])
		d.CostPerRun.Lo, d.CostPerRun.Hi = ci(costAcc[n])
		d.Steps.Lo, d.Steps.Hi = ci(stepsAcc[n])
		if lo, hi := ci(linesAcc[n]); math.IsNaN(lo) {
			d.ChurnMeasured = false // a point estimate without bounds is not a report
		} else {
			d.ChurnPerRun.Lo, d.ChurnPerRun.Hi = lo, hi
		}
		pinning(&d)
		deltas[n] = d
	}
	overall.CV = pooledCV(base, cand)
	overall.TokensPerSolved.Lo, overall.TokensPerSolved.Hi = ci(oTps)
	overall.CostPerRun.Lo, overall.CostPerRun.Hi = ci(oCost)
	overall.Steps.Lo, overall.Steps.Hi = ci(oSteps)
	if lo, hi := ci(oLines); math.IsNaN(lo) {
		overall.ChurnMeasured = false
	} else {
		overall.ChurnPerRun.Lo, overall.ChurnPerRun.Hi = lo, hi
	}
	pinning(&overall)

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
	// A note on what is NOT here: an interval that sits clear of 1.00 at a
	// small n is always tight (Hi<1.00 forces a narrow bootstrap spread,
	// which pins the size), so there is no reachable branch where a clear
	// interval coexists with an unpinned magnitude. The size claim is
	// therefore refused by the annotation below, not by a verdict case: the
	// direction can be true and the percentage still unmeasurable.
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
	// A direction that stands on the interval still does not report a size
	// unless the interval pinned one. This is the sentence the release A/B
	// needed and did not have.
	if !overall.Pinned && overall.NeededN > 0 && (strings.HasPrefix(c.Verdict, "better") || strings.HasPrefix(c.Verdict, "worse")) {
		c.Verdict += fmt.Sprintf(" · direction only, no size claim: %.0f%% ±%.2f at %d runs/arm, and %d runs/arm would pin it",
			overall.Effect*100, math.Max(overall.TokensPerSolved.Hi-o.Point, o.Point-overall.TokensPerSolved.Lo),
			min(overall.Runs[0], overall.Runs[1]), overall.NeededN)
	}
	return c
}

// RenderComparison prints the per-scenario table and the verdict.
func RenderComparison(c Comparison) string {
	var sb strings.Builder
	sb.WriteString("| scenario | passes base→cand | runs | cv | need n | tokens/solved [lo, hi] | cost/run [lo, hi] | steps [lo, hi] | lines/run [lo, hi] |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	row := func(d ScenarioDelta) {
		churn := "—"
		if d.ChurnMeasured {
			churn = d.ChurnPerRun.String()
		}
		cv, need := "—", "—"
		if !math.IsNaN(d.CV) {
			cv = fmt.Sprintf("%.0f%%", d.CV*100)
		}
		if d.NeededN > 0 {
			need = fmt.Sprint(d.NeededN)
			if d.Pinned {
				need += " ✓"
			}
		}
		fmt.Fprintf(&sb, "| %s | %d→%d | %d→%d | %s | %s | %s | %s | %s | %s |\n",
			d.Name, d.PassBase, d.PassCand, d.Runs[0], d.Runs[1], cv, need,
			d.TokensPerSolved, d.CostPerRun, d.Steps, churn)
	}
	for _, d := range c.Scenarios {
		row(d)
	}
	row(c.Overall)
	fmt.Fprintf(&sb, "\n**Verdict: %s**\n", c.Verdict)
	return sb.String()
}

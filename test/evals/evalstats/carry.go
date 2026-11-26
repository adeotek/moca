package main

// Carry: the codemode pilot's pre-change instrument (plan v13, Task 1).
//
// A two-turn trial measures what the second process had to re-read from the
// first: moca's own token estimate over the tool-result content of the first
// outbound request after --continue. The number is produced by a build-tagged
// hook (internal/agent/carry_hook.go, `-tags codemodecarry`) that appends one
// JSONL observation per outbound request to $MOCA_CARRY_OUT. This file reads
// those observations, re-derives the same figure independently from a
// serialized request body (WireCarry — the cross-check that keeps the hook
// honest), and compares two arms paired by (scenario, trial).

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strings"

	"github.com/adeotek/moca/internal/compact"
)

// CarryObservation is one line of $MOCA_CARRY_OUT, as internal/agent writes
// it. The field names are the contract between the hook and this reader.
type CarryObservation struct {
	Session string `json:"session"`
	Trial   int    `json:"trial"`
	Turn    int    `json:"turn"`
	Carry   int    `json:"carry"`
	// Attempts is the process's cumulative physical-adapter-attempt count at
	// this turn. Diagnostic only: unlike Turn it cannot be attributed to work
	// vs auxiliary requests, so it is never summed with a work count.
	Attempts int `json:"attempts,omitempty"`
}

// ReadCarryObservations reads the hook's JSONL file. A torn final line is
// ignored like a torn results row (the harness can kill a run mid-append); an
// empty file is a missing measurement, reported as no observations.
func ReadCarryObservations(path string) ([]CarryObservation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []CarryObservation
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var o CarryObservation
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			continue // a torn append is not a corrupt measurement
		}
		out = append(out, o)
	}
	return out, sc.Err()
}

// WorkOf counts the logical work requests across the given observation files:
// one line is one turn() boundary, i.e. one model request the agent made to
// make progress. A missing file or an empty one is an error — the campaign
// must never read an absent instrument as zero requests.
func WorkOf(paths []string) (int, error) {
	total := 0
	for _, p := range paths {
		obs, err := ReadCarryObservations(p)
		if err != nil {
			return 0, err
		}
		if len(obs) == 0 {
			return 0, fmt.Errorf("no work observations in %s", p)
		}
		total += len(obs)
	}
	return total, nil
}

// AttemptsOf sums the last cumulative attempt count of each observation file
// (each process counts from zero). ok is false when no file carried the field:
// an instrument built without the hook must not read as "no retries".
func AttemptsOf(paths []string) (int, bool, error) {
	total, seen := 0, false
	for _, p := range paths {
		obs, err := ReadCarryObservations(p)
		if err != nil {
			return 0, false, err
		}
		last := 0
		for _, o := range obs {
			if o.Attempts > last {
				last = o.Attempts
			}
		}
		if last > 0 {
			seen = true // every work turn makes at least one attempt
		}
		total += last
	}
	return total, seen, nil
}

// CarryOfTurn returns the carry figure of one turn. Turn 1 is the metric: the
// first request the resuming process sent, i.e. exactly the history the first
// turn left behind.
func CarryOfTurn(obs []CarryObservation, turn int) (int, bool) {
	for _, o := range obs {
		if o.Turn == turn {
			return o.Carry, true
		}
	}
	return 0, false
}

// wireRequest is the part of an openai-completions request body the
// cross-check needs. The adapter writes one `role:"tool"` message per
// tool-result block (internal/provider/openai_completions.go), so summing
// those contents is the same population the hook sums on the request side —
// computed from the bytes a provider would actually receive.
type wireRequest struct {
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
}

// WireCarry decodes a serialized request body and returns the carried
// tool-result estimate. It is deliberately independent of llm.Request: the
// cross-check has to decode the wire, not a re-marshalled copy of the same Go
// value, or it would prove nothing.
func WireCarry(body []byte) (int, error) {
	var req wireRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return 0, fmt.Errorf("decode request: %w", err)
	}
	if len(req.Messages) == 0 {
		return 0, fmt.Errorf("request has no messages")
	}
	var n int
	for _, m := range req.Messages {
		if m.Role != "tool" {
			continue
		}
		var s string
		if err := json.Unmarshal(m.Content, &s); err != nil {
			// A structured content array is the anthropic shape; this decoder
			// is for the completions route the pilot runs on.
			return 0, fmt.Errorf("tool message content is not a string")
		}
		n += compact.Tokens(len(s))
	}
	return n, nil
}

// CarryScene is one scenario's (or the whole corpus's) paired read.
type CarryScene struct {
	Name       string
	Pairs      int
	CarryRatio Ratio // Σ candidate carry / Σ base carry, resampled by pair
	Saving     Ratio // base carry − candidate carry, in estimated tokens
	PassBase   int
	PassCand   int
	StepsBase  float64
	StepsCand  float64
	// AdoptOK counts candidate trials that both used codemode successfully
	// (cmOk>0) and passed: the adoption gate Task 7 reads. AdoptN is the
	// candidate trial count, the denominator.
	AdoptOK int
	AdoptN  int
}

func (s CarryScene) String() string {
	return fmt.Sprintf("%s: %d pairs", s.Name, s.Pairs)
}

// CarryComparison is the base-vs-candidate read of two schema-2 result files.
type CarryComparison struct {
	Scenarios []CarryScene
	Overall   CarryScene
	Verdict   string
}

type carryPair struct{ base, cand Metrics }

// pairCarry checks the row contract and pairs the two arms by
// (scenario, trial). Every failure here is one of the plan's red cases: a
// wrong arm, a duplicate row, an extra or missing tuple — each must name what
// it found instead of silently comparing a subset.
func pairCarry(base, cand []Metrics) (map[string][]carryPair, []string, error) {
	arms := func(rows []Metrics, side string) (string, error) {
		arm := ""
		for _, m := range rows {
			if m.Schema < 2 {
				return "", fmt.Errorf("%s has a schema-%d row (%s): -compare-carry needs schema 2", side, m.Schema, m.Scenario)
			}
			if m.Trial <= 0 {
				return "", fmt.Errorf("%s row %s has no trial number", side, m.Scenario)
			}
			if m.Turns != 2 {
				return "", fmt.Errorf("%s row %s#%d has turns=%d, want 2", side, m.Scenario, m.Trial, m.Turns)
			}
			if !m.CarryMeasured {
				return "", fmt.Errorf("%s row %s#%d has no carry measurement: rerun the trial, never read the gap as zero", side, m.Scenario, m.Trial)
			}
			if m.Arm == "" {
				return "", fmt.Errorf("%s row %s#%d names no arm", side, m.Scenario, m.Trial)
			}
			if m.Arm == "on" || m.Arm == "off" {
				return "", fmt.Errorf("%s row %s#%d uses the reserved corpus arm %q (pilot arms are cm-*)", side, m.Scenario, m.Trial, m.Arm)
			}
			if arm == "" {
				arm = m.Arm
			} else if m.Arm != arm {
				return "", fmt.Errorf("%s mixes arms %q and %q", side, arm, m.Arm)
			}
		}
		return arm, nil
	}
	baseArm, err := arms(base, "base")
	if err != nil {
		return nil, nil, err
	}
	candArm, err := arms(cand, "candidate")
	if err != nil {
		return nil, nil, err
	}
	if baseArm == candArm {
		return nil, nil, fmt.Errorf("both files carry arm %q: the comparison needs two arms", baseArm)
	}
	idx := func(rows []Metrics, side string) (map[[2]string]Metrics, error) {
		m := map[[2]string]Metrics{}
		for _, r := range rows {
			k := [2]string{r.Scenario, fmt.Sprint(r.Trial)}
			if _, dup := m[k]; dup {
				return nil, fmt.Errorf("duplicate row for %s#%d in %s", r.Scenario, r.Trial, side)
			}
			m[k] = r
		}
		return m, nil
	}
	bi, err := idx(base, "base")
	if err != nil {
		return nil, nil, err
	}
	ci, err := idx(cand, "candidate")
	if err != nil {
		return nil, nil, err
	}
	var names []string
	byScen := map[string][]carryPair{}
	for k, b := range bi {
		c, ok := ci[k]
		if !ok {
			return nil, nil, fmt.Errorf("base has %s#%s with no candidate row (extra tuple)", k[0], k[1])
		}
		if _, ok := byScen[k[0]]; !ok {
			names = append(names, k[0])
		}
		byScen[k[0]] = append(byScen[k[0]], carryPair{base: b, cand: c})
	}
	for k := range ci {
		if _, ok := bi[k]; !ok {
			return nil, nil, fmt.Errorf("candidate has %s#%s with no base row (extra tuple)", k[0], k[1])
		}
	}
	sort.Strings(names)
	for _, n := range names {
		sort.Slice(byScen[n], func(i, j int) bool { return byScen[n][i].base.Trial < byScen[n][j].base.Trial })
	}
	return byScen, names, nil
}

// compareCarryScene is the point estimate of one scene's pairs.
func compareCarryScene(name string, pairs []carryPair) CarryScene {
	s := CarryScene{Name: name, Pairs: len(pairs), AdoptN: len(pairs)}
	var bc, cc, bs, cs float64
	for _, p := range pairs {
		bc += float64(p.base.Carry)
		cc += float64(p.cand.Carry)
		bs += float64(p.base.Steps)
		cs += float64(p.cand.Steps)
		if p.base.Pass {
			s.PassBase++
		}
		if p.cand.Pass {
			s.PassCand++
		}
		if p.cand.CMOk > 0 && p.cand.Pass {
			s.AdoptOK++
		}
	}
	n := float64(max(1, len(pairs)))
	s.CarryRatio = Ratio{Point: ratioOrOne(cc, bc)}
	s.Saving = Ratio{Point: (bc - cc) / n}
	s.StepsBase, s.StepsCand = bs/n, cs/n
	return s
}

// ratioOrOne is cand/base, and 1 when the base carried nothing: an empty
// control has no ratio to lose, and 0/0 must not read as a perfect win.
func ratioOrOne(c, b float64) float64 {
	if b <= 0 {
		return 1
	}
	return c / b
}

// percentileCI is the 95 % percentile interval of xs, NaN when empty.
func percentileCI(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return math.NaN(), math.NaN()
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	return s[int(0.025*float64(len(s)))], s[min(len(s)-1, int(0.975*float64(len(s))))]
}

// CompareCarry bootstraps matched pairs: within each scenario the (scenario,
// trial) pairs are resampled with replacement, so a trial's two arms always
// move together — the base and candidate of one trial are one block, which is
// what makes this a paired read rather than two independent samples. The RNG
// is seeded, so identical inputs and seed give identical output.
func CompareCarry(base, cand []Metrics, iters int, seed int64) (CarryComparison, error) {
	byScen, names, err := pairCarry(base, cand)
	if err != nil {
		return CarryComparison{}, err
	}
	var all []carryPair
	for _, n := range names {
		all = append(all, byScen[n]...)
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	acc := map[string]struct{ ratio, saving []float64 }{}
	var oRatio, oSaving []float64
	for it := 0; it < iters; it++ {
		var sb, sc, nb float64
		for _, n := range names {
			pairs := byScen[n]
			var pb, pc float64
			for range pairs {
				p := pairs[rng.IntN(len(pairs))]
				pb += float64(p.base.Carry)
				pc += float64(p.cand.Carry)
			}
			a := acc[n]
			a.ratio = append(a.ratio, ratioOrOne(pc, pb))
			a.saving = append(a.saving, (pb-pc)/float64(len(pairs)))
			acc[n] = a
			sb, sc = sb+pb, sc+pc
			nb += float64(len(pairs))
		}
		oRatio = append(oRatio, ratioOrOne(sc, sb))
		oSaving = append(oSaving, (sb-sc)/nb)
	}

	out := CarryComparison{}
	for _, n := range names {
		s := compareCarryScene(n, byScen[n])
		s.CarryRatio.Lo, s.CarryRatio.Hi = percentileCI(acc[n].ratio)
		s.Saving.Lo, s.Saving.Hi = percentileCI(acc[n].saving)
		out.Scenarios = append(out.Scenarios, s)
	}
	out.Overall = compareCarryScene("∑ all", all)
	out.Overall.CarryRatio.Lo, out.Overall.CarryRatio.Hi = percentileCI(oRatio)
	out.Overall.Saving.Lo, out.Overall.Saving.Hi = percentileCI(oSaving)

	// The rules are Task 7's, and they are checked in the order that keeps a
	// lost pass or an unused feature from ever reading as a win.
	var thin []string
	for _, n := range names {
		if len(byScen[n]) < minRuns {
			thin = append(thin, fmt.Sprintf("%s (%d pairs)", n, len(byScen[n])))
		}
	}
	o := out.Overall
	switch {
	case len(thin) > 0:
		out.Verdict = "underpowered — fewer than " + fmt.Sprint(minRuns) + " matched pairs for " +
			strings.Join(thin, ", ") + "; run those scenarios at -n " + fmt.Sprint(minRuns) + " first"
	case o.PassCand < o.PassBase:
		out.Verdict = fmt.Sprintf("worse — lost a pass (%d→%d pairs passed)", o.PassBase, o.PassCand)
	case o.StepsCand > o.StepsBase:
		out.Verdict = fmt.Sprintf("worse — steps per trial rose %.2f→%.2f", o.StepsBase, o.StepsCand)
	case o.AdoptOK*2 < o.AdoptN:
		out.Verdict = fmt.Sprintf("inconclusive — codemode adopted in only %d of %d trials (half required)", o.AdoptOK, o.AdoptN)
	case o.CarryRatio.Hi <= 0.75 && o.Saving.Point >= 512:
		out.Verdict = fmt.Sprintf("better — carry ratio Hi %.2f ≤ 0.75 with %.0f tokens saved per trial", o.CarryRatio.Hi, o.Saving.Point)
	case o.CarryRatio.Lo > 1.0:
		out.Verdict = fmt.Sprintf("worse — carry ratio Lo %.2f > 1.00 (more context carried)", o.CarryRatio.Lo)
	default:
		out.Verdict = fmt.Sprintf("inconclusive — carry ratio %.2f [%.2f, %.2f], saving %.0f [%.0f, %.0f] tokens",
			o.CarryRatio.Point, o.CarryRatio.Lo, o.CarryRatio.Hi, o.Saving.Point, o.Saving.Lo, o.Saving.Hi)
	}
	return out, nil
}

// RenderCarryComparison prints the paired table and the verdict.
func RenderCarryComparison(c CarryComparison) string {
	var sb strings.Builder
	sb.WriteString("| scenario | pairs | carry ratio [lo, hi] | saved tok [lo, hi] | passes base→cand | steps base→cand | adopt |\n")
	sb.WriteString("|---|---|---|---|---|---|---|\n")
	row := func(s CarryScene) {
		fmt.Fprintf(&sb, "| %s | %d | %s | %s | %d→%d | %.1f→%.1f | %d/%d |\n",
			s.Name, s.Pairs, s.CarryRatio, s.Saving, s.PassBase, s.PassCand, s.StepsBase, s.StepsCand, s.AdoptOK, s.AdoptN)
	}
	for _, s := range c.Scenarios {
		row(s)
	}
	row(c.Overall)
	fmt.Fprintf(&sb, "\n**Verdict: %s**\n", c.Verdict)
	return sb.String()
}

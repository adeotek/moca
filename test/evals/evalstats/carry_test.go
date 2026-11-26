package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- the decoder -----------------------------------------------------------

// TestWireCarryReadsTheRecordedRequest is the cross-check's other half: the
// number the hook computes from llm.Request must equal what a provider sees in
// the serialized body. The fixture is a completions request in the shape
// internal/provider/openai_completions.go emits (one role:"tool" message per
// tool-result block).
func TestWireCarryReadsTheRecordedRequest(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "carry", "request-completions.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := WireCarry(body)
	if err != nil {
		t.Fatalf("WireCarry: %v", err)
	}
	// "0123456789abcdef" (16 B -> 4) + the 432 B result (-> 108).
	if got != 112 {
		t.Fatalf("carry = %d, want 112 (4 + 108)", got)
	}
}

// body builds a completions request body the way the adapter does. Kept
// local so the test states the wire shape it is asserting about rather than
// reusing production code that the decoder is supposed to check.
func body(t *testing.T, msgs ...map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"model": "m", "messages": msgs, "stream": true})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func toolMsg(content string) map[string]any {
	return map[string]any{"role": "tool", "tool_call_id": "c", "content": content}
}

// A compaction summary is a user message: it is carried history but not
// tool-result content, so it must not inflate the figure (the plan's
// "excludes summaries").
func TestWireCarryExcludesTheSummary(t *testing.T) {
	withSummary := body(t,
		map[string]any{"role": "system", "content": "sys"},
		map[string]any{"role": "user", "content": "## Summary\n" + strings.Repeat("s", 40000)},
		toolMsg(strings.Repeat("t", 400)),
	)
	noSummary := body(t, toolMsg(strings.Repeat("t", 400)))
	a, err := WireCarry(withSummary)
	if err != nil {
		t.Fatal(err)
	}
	b, err := WireCarry(noSummary)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a != 100 {
		t.Fatalf("summarised request read %d, plain %d, want both 100 (summary is not tool output)", a, b)
	}
}

// An elided result is a small stub, and a retry is the same request twice:
// neither changes the figure the first observation reports.
func TestWireCarryElidedAndRetry(t *testing.T) {
	elided, err := WireCarry(body(t, toolMsg("[elided: 4000 chars omitted]")))
	if err != nil {
		t.Fatal(err)
	}
	if elided != 7 {
		t.Fatalf("elided stub read %d tokens, want 7", elided)
	}
	req := body(t, toolMsg(strings.Repeat("t", 400)))
	first, _ := WireCarry(req)
	retry, _ := WireCarry(req)
	if first != retry {
		t.Fatalf("a retry of the same request must read the same carry: %d vs %d", first, retry)
	}
}

// A real zero: the second turn carried no tool output. That is a measurement,
// not a gap, and it must decode as 0 rather than as an error.
func TestWireCarryRealZero(t *testing.T) {
	got, err := WireCarry(body(t,
		map[string]any{"role": "system", "content": "sys"},
		map[string]any{"role": "user", "content": "hello"},
		map[string]any{"role": "assistant", "content": "hi"},
	))
	if err != nil {
		t.Fatalf("WireCarry: %v", err)
	}
	if got != 0 {
		t.Fatalf("carry = %d, want 0", got)
	}
}

func TestWireCarryRejectsANonRequest(t *testing.T) {
	if _, err := WireCarry([]byte(`{"messages":[]}`)); err == nil {
		t.Fatal("an empty message list is not a request")
	}
	if _, err := WireCarry([]byte(`not json`)); err == nil {
		t.Fatal("garbage must not decode")
	}
}

// --- the observations ------------------------------------------------------

func TestCarryOfTurn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "carry.jsonl")
	lines := `{"session":"ab12cd34","trial":3,"turn":1,"carry":4096}
{"session":"ab12cd34","trial":3,"turn":2,"carry":4100}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	obs, err := ReadCarryObservations(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("observations = %d, want 2", len(obs))
	}
	if c, ok := CarryOfTurn(obs, 1); !ok || c != 4096 {
		t.Fatalf("turn 1 = %d/%v, want 4096/true", c, ok)
	}
	if _, ok := CarryOfTurn(obs, 9); ok {
		t.Fatal("a turn that was never observed must not read as 0")
	}
}

// A torn append is not a corrupt measurement: the run can be killed between
// write and newline. A file with no observation at all is the missing case.
func TestReadCarryObservationsToleratesATornLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "carry.jsonl")
	if err := os.WriteFile(path, []byte("{\"session\":\"a\",\"turn\":1,\"carry\":7}\n{\"session\":\"a\",\"tu"), 0o600); err != nil {
		t.Fatal(err)
	}
	obs, err := ReadCarryObservations(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].Carry != 7 {
		t.Fatalf("observations = %+v, want the one valid line", obs)
	}
	if _, err := ReadCarryObservations(filepath.Join(t.TempDir(), "absent.jsonl")); err == nil {
		t.Fatal("a missing file must be an error, not an empty measurement")
	}
}

// --- the paired comparison -------------------------------------------------

func trialRow(scen string, trial int, arm string, carry int, pass bool, steps, cmOk int) Metrics {
	return Metrics{
		Schema: 2, Scenario: scen, Trial: trial, Turns: 2, Arm: arm,
		Carry: carry, CarryMeasured: true, Pass: pass, Steps: steps,
		CMAttempted: max(cmOk, 1), CMOk: cmOk,
	}
}

func carryCorpus(baseCarry, candCarry int) ([]Metrics, []Metrics) {
	var base, cand []Metrics
	for i := 1; i <= 3; i++ {
		base = append(base, trialRow("sweep", i, "cm-off", baseCarry, true, 8, 0))
		cand = append(cand, trialRow("sweep", i, "cm-on", candCarry, true, 8, 1))
	}
	return base, cand
}

func TestCompareCarryBetter(t *testing.T) {
	base, cand := carryCorpus(4000, 1000)
	c, err := CompareCarry(base, cand, 500, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.Verdict, "better") {
		t.Fatalf("verdict = %q, want better", c.Verdict)
	}
	if c.Overall.Saving.Point != 3000 {
		t.Fatalf("saving = %v, want 3000", c.Overall.Saving.Point)
	}
	out := RenderCarryComparison(c)
	for _, want := range []string{"| scenario |", "∑ all", "**Verdict: better"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

// The three vetoes: a lost pass, a step increase and an unadopted feature all
// keep a smaller carry from reading as a win — the rule order is the contract.
func TestCompareCarryVetoes(t *testing.T) {
	t.Run("lost pass", func(t *testing.T) {
		base, cand := carryCorpus(4000, 1000)
		cand[0].Pass = false
		c, err := CompareCarry(base, cand, 300, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(c.Verdict, "worse — lost a pass") {
			t.Fatalf("verdict = %q", c.Verdict)
		}
	})
	t.Run("step increase", func(t *testing.T) {
		base, cand := carryCorpus(4000, 1000)
		cand[2].Steps = 9
		c, err := CompareCarry(base, cand, 300, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(c.Verdict, "worse — steps per trial rose") {
			t.Fatalf("verdict = %q", c.Verdict)
		}
	})
	t.Run("not adopted", func(t *testing.T) {
		base, cand := carryCorpus(4000, 4000) // identical: no saving either
		for i := range cand {
			cand[i].CMOk = 0
		}
		c, err := CompareCarry(base, cand, 300, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(c.Verdict, "adopted in only 0 of 3") {
			t.Fatalf("verdict = %q", c.Verdict)
		}
	})
	t.Run("carry grew", func(t *testing.T) {
		base, cand := carryCorpus(4000, 5000)
		c, err := CompareCarry(base, cand, 300, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(c.Verdict, "worse — carry ratio") {
			t.Fatalf("verdict = %q", c.Verdict)
		}
	})
}

func TestCompareCarryUnderpowered(t *testing.T) {
	base, cand := carryCorpus(4000, 1000)
	base, cand = base[:2], cand[:2]
	c, err := CompareCarry(base, cand, 200, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.Verdict, "underpowered") || !strings.Contains(c.Verdict, "sweep (2 pairs)") {
		t.Fatalf("verdict = %q, want the pair-count refusal naming sweep", c.Verdict)
	}
}

func TestCompareCarryDeterministic(t *testing.T) {
	base, cand := carryCorpus(4000, 900)
	cand[1].Carry = 2500 // spread so the bootstrap actually varies
	a, err := CompareCarry(base, cand, 300, 7)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CompareCarry(base, cand, 300, 7)
	if err != nil {
		t.Fatal(err)
	}
	if RenderCarryComparison(a) != RenderCarryComparison(b) {
		t.Fatal("same inputs and seed must give an identical comparison")
	}
}

// Every malformed input the plan names must fail for its own reason rather
// than compare a silently reduced set: a wrong arm, a duplicate row, an extra
// tuple, a schema-1 row, a non-two-turn row, and the reserved corpus arms.
func TestCompareCarryRefusesMalformedRows(t *testing.T) {
	cases := []struct {
		name string
		mut  func(b, c []Metrics) ([]Metrics, []Metrics)
		want string
	}{
		{"wrong arm", func(b, c []Metrics) ([]Metrics, []Metrics) {
			c[1].Arm = "cm-off"
			return b, c
		}, "mixes arms"},
		{"reserved arm", func(b, c []Metrics) ([]Metrics, []Metrics) {
			c[1].Arm = "on"
			return b, c
		}, "reserved corpus arm"},
		{"same arm both sides", func(b, c []Metrics) ([]Metrics, []Metrics) {
			for i := range c {
				c[i].Arm = "cm-off"
			}
			return b, c
		}, "both files carry arm"},
		{"duplicate row", func(b, c []Metrics) ([]Metrics, []Metrics) {
			c = append(c, c[0])
			return b, c
		}, "duplicate row"},
		{"extra candidate tuple", func(b, c []Metrics) ([]Metrics, []Metrics) {
			c = append(c, trialRow("sweep", 9, "cm-on", 1000, true, 8, 1))
			return b, c
		}, "no base row (extra tuple)"},
		{"missing candidate row", func(b, c []Metrics) ([]Metrics, []Metrics) {
			return b, c[:2]
		}, "no candidate row (extra tuple)"},
		{"schema 1 row", func(b, c []Metrics) ([]Metrics, []Metrics) {
			b = append(b, Metrics{Scenario: "sweep", Run: 1})
			return b, c
		}, "-compare-carry needs schema 2"},
		{"one turn", func(b, c []Metrics) ([]Metrics, []Metrics) {
			c[0].Turns = 1
			return b, c
		}, "turns=1, want 2"},
		{"no arm", func(b, c []Metrics) ([]Metrics, []Metrics) {
			b[0].Arm = ""
			return b, c
		}, "names no arm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, c := tc.mut(carryCorpus(4000, 1000))
			_, err := CompareCarry(b, c, 100, 1)
			if err == nil {
				t.Fatalf("%s: comparison accepted the malformed rows", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: error = %q, want it to mention %q", tc.name, err, tc.want)
			}
		})
	}
}

// The work count is the changeset campaign's endpoint: one observation line is
// one logical work request, so a trial's two processes add. A missing or empty
// file is a gap, never a zero — the same rule carry already carries.
func TestWorkOfSumsBothProcesses(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.jsonl")
	two := filepath.Join(dir, "two.jsonl")
	if err := os.WriteFile(one, []byte(`{"turn":1,"carry":10,"attempts":1}
{"turn":2,"carry":20,"attempts":3}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte(`{"turn":1,"carry":5,"attempts":2}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := WorkOf([]string{one, two})
	if err != nil {
		t.Fatalf("WorkOf: %v", err)
	}
	if got != 3 {
		t.Fatalf("WorkOf = %d, want 3 (2 + 1 observations)", got)
	}
	if _, err := WorkOf([]string{filepath.Join(dir, "absent.jsonl")}); err == nil {
		t.Fatal("WorkOf accepted a missing observation file")
	}
	empty := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WorkOf([]string{one, empty}); err == nil {
		t.Fatal("WorkOf read an empty file as a measured zero")
	}
}

// Attempts are cumulative per process, so each file contributes its last
// observed value and the trial sums the processes. A file without the field is
// not a measurement; the caller records the gap instead of a zero.
func TestAttemptsOfTakesTheLastPerProcess(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.jsonl")
	two := filepath.Join(dir, "two.jsonl")
	old := filepath.Join(dir, "old.jsonl")
	if err := os.WriteFile(one, []byte(`{"turn":1,"carry":1,"attempts":2}
{"turn":2,"carry":1,"attempts":5}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte(`{"turn":1,"carry":1,"attempts":3}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte(`{"turn":1,"carry":1}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok, err := AttemptsOf([]string{one, two})
	if err != nil || !ok {
		t.Fatalf("AttemptsOf: got (%d, %v, %v), want (8, true, nil)", got, ok, err)
	}
	if got != 8 {
		t.Fatalf("AttemptsOf = %d, want 8 (max 5 + max 3)", got)
	}
	if _, ok, err := AttemptsOf([]string{old}); err != nil || ok {
		t.Fatalf("AttemptsOf on a file without the field: ok = %v, want false", ok)
	}
}

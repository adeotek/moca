package main

// Size metrics (v0.8.1, task 1): the corpus measures what the agent wrote, not
// only what it spent. Every field here is a MEASUREMENT — no checker reads it,
// so a round that writes more code cannot fail because of this file. The one
// exception is the reuse percentage, which is a report too: it must read 0 on a
// run that changed nothing, never NaN, because a NaN poisons every table it
// lands in.

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// TestSizeFieldsCarryThroughJSON: the four numbers ride main's flags onto the
// row built from Collect(entries) (there is no CollectInput), and a results
// file written today must still parse tomorrow — hence additive omitempty
// fields, so an old row reads as zero rather than failing to decode. The
// flag→field mapping itself is pinned end to end by a real run.
func TestSizeFieldsCarryThroughJSON(t *testing.T) {
	b, err := json.Marshal(Metrics{Added: 12, Removed: 3, Files: 4, NewFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	var m Metrics
	if err := json.Unmarshal(b, &m); err != nil ||
		m.Added != 12 || m.Removed != 3 || m.Files != 4 || m.NewFiles != 1 {
		t.Fatalf("size fields dropped: %s %+v", b, m)
	}
	// An old row (no size keys at all) must decode to zeroes.
	var old Metrics
	if err := json.Unmarshal([]byte(`{"scenario":"vague.1","pass":true}`), &old); err != nil {
		t.Fatalf("an older row must still decode: %v", err)
	}
	if old.Added != 0 || old.Churn() != 0 || old.ReusePct() != 0 {
		t.Fatalf("a size-less row reads %+v", old)
	}
}

// TestChurnAndReuse: Churn is added+removed; ReusePct is the share of touched
// files that already existed. A division by zero must not render NaN.
func TestChurnAndReuse(t *testing.T) {
	m := Metrics{Added: 10, Removed: 2, Files: 4, NewFiles: 1}
	if m.Churn() != 12 {
		t.Fatalf("churn = %d, want 12", m.Churn())
	}
	if got := m.ReusePct(); math.Abs(got-75) > 1e-9 {
		t.Fatalf("reuse = %v, want 75", got)
	}
	if got := (Metrics{}).ReusePct(); got != 0 {
		t.Fatalf("an empty row must read 0, got %v", got)
	}
}

// TestSummarizeShowsSizeColumns: the summary table is where the number is
// read, so the columns are part of the contract. The mean is per run.
func TestSummarizeShowsSizeColumns(t *testing.T) {
	out := Summarize([]Metrics{
		{Scenario: "vague.1", Label: "t", Added: 10, Removed: 2, Files: 2, NewFiles: 0},
		{Scenario: "vague.1", Label: "t", Added: 20, Removed: 4, Files: 4, NewFiles: 2},
	})
	for _, want := range []string{"lines (mean ± sd)", "new files"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing column %q:\n%s", want, out)
		}
	}
	// (10+2 + 20+4)/2 = 18 lines and (0+2)/2 = 1 new file per run.
	if !strings.Contains(out, "| 18 ± ") {
		t.Fatalf("churn mean missing from the row:\n%s", out)
	}
}

// TestCompareReportsChurnPerRun: the size delta is reported beside tokens and
// cost, through the same stratified bootstrap, and it is NOT part of the ship
// verdict — a leanness change is judged by passes and by this number, never by
// a ratio the verdict switch reads.
func TestCompareReportsChurnPerRun(t *testing.T) {
	mk := func(n int, churn int, pass bool) Metrics {
		return Metrics{Scenario: "a", Pass: pass, Input: 1000, Cost: 0.01, Steps: 5,
			Added: churn, Removed: 0, Files: 2}
	}
	var base, cand []Metrics
	for i := 0; i < 4; i++ {
		base = append(base, mk(i, 100, true))
		cand = append(cand, mk(i, 50, true))
	}
	c := Compare(base, cand, 200, 1)
	if math.Abs(c.Overall.ChurnPerRun.Point-0.5) > 1e-9 {
		t.Fatalf("churn/run = %v, want 0.50", c.Overall.ChurnPerRun.Point)
	}
	if c.Overall.ChurnPerRun.Lo > 0.5 || c.Overall.ChurnPerRun.Hi < 0.5 {
		t.Fatalf("the 0.50 point sits outside its own interval: %v", c.Overall.ChurnPerRun)
	}
	// Identical sides read 1.00 — and the verdict still reads the tokens, so a
	// size change alone never flips a ship decision.
	same := Compare(base, base, 200, 1)
	if math.Abs(same.Overall.ChurnPerRun.Point-1) > 1e-9 {
		t.Fatalf("identical churn = %v, want 1.00", same.Overall.ChurnPerRun.Point)
	}
	if !strings.Contains(RenderComparison(c), "lines/run") {
		t.Fatalf("the comparison table hides the size row:\n%s", RenderComparison(c))
	}
	if strings.Contains(c.Verdict, "lines") {
		t.Fatalf("the verdict must not read the size ratio: %q", c.Verdict)
	}
}

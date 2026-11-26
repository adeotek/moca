package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The row contract is what keeps a harness bug from becoming a silent zero in
// the statistics (plan v13, Task 1): every schema-2 row states its identity and
// its carry, and a missing carry is a named gap rather than a free trial.
func TestRowContract(t *testing.T) {
	ok := map[string]bool{"carry": true, "work": true}
	cases := []struct {
		name         string
		schema       int
		set          map[string]bool
		carryMissing bool
		workMissing  bool
		trial, turns int
		arm, sess    string
		want         string // "" = accepted
	}{
		{name: "complete", schema: 2, set: ok, trial: 3, turns: 2, arm: "cm-on", sess: "ab12cd34"},
		{name: "observed zero", schema: 2, set: map[string]bool{"carry": true, "work": true}, trial: 1, turns: 2, arm: "cm-off", sess: "ab12cd34"},
		{name: "gaps recorded", schema: 2, set: map[string]bool{"carryMissing": true, "workMissing": true}, carryMissing: true, workMissing: true, trial: 1, turns: 2, arm: "cm-on", sess: "ab12cd34"},
		{name: "no carry at all", schema: 2, set: map[string]bool{"work": true}, trial: 1, turns: 2, arm: "cm-on", sess: "ab12cd34", want: "needs -carry"},
		{name: "no work at all", schema: 2, set: map[string]bool{"carry": true}, trial: 1, turns: 2, arm: "cm-on", sess: "ab12cd34", want: "needs -work"},
		{name: "no trial", schema: 2, set: ok, trial: 0, turns: 2, arm: "cm-on", sess: "ab12cd34", want: "needs -trial"},
		{name: "no turns", schema: 2, set: ok, trial: 1, turns: 0, arm: "cm-on", sess: "ab12cd34", want: "needs -turns"},
		{name: "no arm", schema: 2, set: ok, trial: 1, turns: 2, sess: "ab12cd34", want: "needs -arm"},
		{name: "no session", schema: 2, set: ok, trial: 1, turns: 2, arm: "cm-on", want: "needs -sessionId"},
		{name: "unknown schema", schema: 3, set: ok, trial: 1, turns: 2, arm: "cm-on", sess: "ab12cd34", want: "unknown row schema 3"},
		{name: "trial flags without schema", schema: 0, set: map[string]bool{"trial": true}, want: "without -schema 2"},
		{name: "schema 1 row", schema: 0, set: map[string]bool{}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkRowContract(tc.schema, tc.set, tc.carryMissing, tc.workMissing, tc.trial, tc.turns, tc.arm, tc.sess)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("rejected a valid row: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("accepted an invalid row, want %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// -compare must refuse a schema-2 row instead of comparing one trial's
// two-turn token volume as if it were a single prompt.
func TestCompareRefusesSchema2(t *testing.T) {
	base, cand := carryCorpus(4000, 1000)
	err := refuseSchema2(base, cand)
	if err == nil || !strings.Contains(err.Error(), "schema-2 trial row") ||
		!strings.Contains(err.Error(), "-compare-carry") {
		t.Fatalf("error = %v, want a refusal naming -compare-carry", err)
	}
	schema1 := []Metrics{{Scenario: "sweep", Run: 1, Input: 100}, {Scenario: "sweep", Run: 2, Input: 120}}
	if err := refuseSchema2(schema1, schema1); err != nil {
		t.Fatalf("schema-1 rows are comparable: %v", err)
	}
}

// A trial row round-trips through JSON with exactly the documented field
// names: the harness and the analysis read the same contract, and a rename
// would silently drop a column rather than fail.
func TestTrialRowJSONNames(t *testing.T) {
	m := trialRow("sweep", 3, "cm-on", 4096, true, 7, 2)
	m.Label, m.Session, m.MS = "pilot", "ab12cd34", 12345
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"schema":2`, `"scenario":"sweep"`, `"label":"pilot"`, `"trial":3`, `"turns":2`,
		`"arm":"cm-on"`, `"session":"ab12cd34"`, `"carry":4096`, `"carryMeasured":true`,
		`"ms":12345`, `"cmAttempted":2`, `"cmOk":2`, `"pass":true`,
	} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("row is missing %s:\n%s", want, b)
		}
	}
}

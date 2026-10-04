// internal/llm/types_test.go
package llm

import "testing"

func TestParseEffort(t *testing.T) {
	for _, s := range []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"} {
		e, err := ParseEffort(s)
		if err != nil || string(e) != s {
			t.Fatalf("ParseEffort(%q) = %q, %v", s, e, err)
		}
	}
	if _, err := ParseEffort("med"); err == nil {
		t.Fatal("ParseEffort(med) must fail; status-bar abbreviations are display only")
	}
}

func TestEffortRankOrdered(t *testing.T) {
	for i := 1; i < len(Efforts); i++ {
		if Efforts[i-1].Rank() >= Efforts[i].Rank() {
			t.Fatalf("%s must rank below %s", Efforts[i-1], Efforts[i])
		}
	}
}

func TestUsageAdd(t *testing.T) {
	got := Usage{1, 2, 3, 4}.Add(Usage{10, 20, 30, 40})
	if got != (Usage{11, 22, 33, 44}) {
		t.Fatalf("Add = %+v", got)
	}
}

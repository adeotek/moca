package compact

import "testing"

func TestBudget(t *testing.T) {
	big := NewBudget(1_000_000, 16384, 20000)
	if big.Reserve != 16384 || big.KeepRecent != 20000 || big.Trigger() != 1_000_000-16384 {
		t.Fatalf("%+v", big)
	}
	small := NewBudget(32768, 16384, 20000)
	if small.Reserve != 8192 || small.KeepRecent != 8192 || small.Trigger() != 24576 {
		t.Fatalf("small window scales to window/4: %+v", small)
	}
	if small.Over(24576) || !small.Over(24577) {
		t.Fatal("strictly greater")
	}
}

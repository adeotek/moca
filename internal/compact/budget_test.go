package compact

import "testing"

func TestBudgetEffectiveWindow(t *testing.T) {
	big := NewBudget(1_000_000, 300_000, 75, 16384, 20000)
	if big.Effective != 300_000 || big.Reserve != 16384 || big.KeepRecent != 20_000 ||
		big.Trigger() != 225_000 || big.Ceiling() != 240_000 {
		t.Fatalf("capped 1M: %+v trigger=%d ceiling=%d", big, big.Trigger(), big.Ceiling())
	}
	// The 20K default is preserved through 400K effective and grows only
	// above it; the Effective/4 cap still binds on small windows.
	if got := NewBudget(600_000, 0, 75, 16384, 20_000).KeepRecent; got != 30_000 {
		t.Fatalf("600K keepRecent = %d, want 30000", got)
	}
	if got := NewBudget(1_000_000, 0, 75, 16384, 20_000).KeepRecent; got != 50_000 {
		t.Fatalf("1M keepRecent = %d, want 50000", got)
	}
}

func TestBudgetSmallWindowAndLegacyTrigger(t *testing.T) {
	small := NewBudget(32_768, 0, 75, 16384, 20000)
	if small.Effective != 32_768 || small.Reserve != 8192 || small.KeepRecent != 8192 ||
		small.Trigger() != 24576 || small.Ceiling() != 26214 {
		t.Fatalf("small: %+v trigger=%d ceiling=%d", small, small.Trigger(), small.Ceiling())
	}
	legacy := NewBudget(1_000_000, 0, 0, 16384, 20000)
	if legacy.Trigger() != 1_000_000-16384 {
		t.Fatalf("percent=0 must select the reserve trigger, got %d", legacy.Trigger())
	}
	if legacy.Over(legacy.Trigger()) || !legacy.Over(legacy.Trigger()+1) {
		t.Fatal("strictly greater")
	}
}

package compact

// Budget holds the token budgets for one model (§14.1). The advertised
// window is capped by the configured effective window; the trigger is a
// percentage of that effective window, with the reserve as the ceiling's
// physical headroom.
type Budget struct {
	Window     int // advertised model window
	Effective  int // min(Window, maxWindowTokens); 0 cap means Window
	Reserve    int // min(reserveTokens, Effective/4)
	KeepRecent int // adaptive floor, capped at Effective/4
	Percent    int // compactAtPercent; 0 selects the reserve trigger
}

// NewBudget resolves the effective window, reserve, kept tail and trigger
// percentage once per call. maxWindow=0 means the advertised window;
// percent=0 selects Effective-Reserve.
func NewBudget(window, maxWindow, percent, reserve, keepRecent int) Budget {
	effective := window
	if maxWindow > 0 && maxWindow < effective {
		effective = maxWindow
	}
	b := Budget{Window: window, Effective: effective, Percent: percent}
	b.Reserve = min(reserve, effective/4)
	keep := max(keepRecent, min(effective/20, 60_000))
	b.KeepRecent = min(keep, effective/4)
	return b
}

// Trigger is the input estimate that starts compaction. Percent=0 must branch
// before the closed form: 0% would otherwise trigger on every request.
func (b Budget) Trigger() int {
	if b.Percent <= 0 {
		return b.Effective - b.Reserve
	}
	t := b.Percent * b.Effective / 100
	if physical := b.Effective - b.Reserve; t > physical {
		return physical
	}
	return t
}

// Ceiling is the hard estimated-input ceiling.
func (b Budget) Ceiling() int { return b.Effective * 80 / 100 }

func (b Budget) Over(tokens int) bool { return tokens > b.Trigger() }

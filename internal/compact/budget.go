package compact

// Budget holds the token budgets for one model (§6). Percentages would
// conflate a 200K and a 1M window; tokens don't.
type Budget struct {
	Window, Reserve, KeepRecent int
}

// NewBudget scales the configured budgets down for small windows: a model
// whose window is 64K cannot afford the defaults' absolute reserve.
func NewBudget(window, reserve, keepRecent int) Budget {
	return Budget{Window: window, Reserve: min(reserve, window/4), KeepRecent: min(keepRecent, window/4)}
}

func (b Budget) Trigger() int         { return b.Window - b.Reserve }
func (b Budget) Over(tokens int) bool { return tokens > b.Trigger() }

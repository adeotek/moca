package session

// Goal snapshots live in the transcript (v0.8.0): every transition appends a
// full Goal entry, and the newest one is the session's state. No sidecar, no
// extra locking, and a resumed session reconstructs exactly what the user
// last decided.

// LatestGoal returns the newest goal snapshot, or (nil, false). Unknown/older
// entries without a goal payload are skipped; a snapshot with no ID is
// ignored too (a damaged line must not become the live goal).
func LatestGoal(entries []Entry) (*Goal, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Type == TypeGoal && e.Goal != nil && e.Goal.ID != "" {
			g := *e.Goal
			return &g, true
		}
	}
	return nil, false
}

// GoalObjectives maps every goal ID named by a stored snapshot to its
// objective — the record a renderer uses for an old message after the goal
// was cleared or replaced. The first snapshot of an ID wins: the objective is
// immutable for its ID, later snapshots repeat it.
func GoalObjectives(entries []Entry) map[string]string {
	out := map[string]string{}
	for _, e := range entries {
		if e.Type != TypeGoal || e.Goal == nil || e.Goal.ID == "" {
			continue
		}
		if _, ok := out[e.Goal.ID]; !ok {
			out[e.Goal.ID] = e.Goal.Objective
		}
	}
	return out
}

// GoalMessageCount counts the goal-tagged conversational entries: used by the
// toolchain and tests to prove goal metadata never reaches the wire.
func GoalMessageCount(entries []Entry) int {
	n := 0
	for _, e := range entries {
		if e.Type == TypeMessage && e.GoalID != "" {
			n++
		}
	}
	return n
}

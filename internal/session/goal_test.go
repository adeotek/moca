package session

// Pins for the goal transcript model (v0.8.0): full snapshots, latest-wins,
// pre-conversation buffering, and no goal metadata on the wire.

import (
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func openGoalWriter(t *testing.T) (*Writer, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := Create(dir, Header{Workdir: dir, Provider: "fake", Model: "fake/m"}, "goal")
	if err != nil {
		t.Fatal(err)
	}
	return w, dir
}

func TestGoalStatusesRoundTrip(t *testing.T) {
	w, _ := openGoalWriter(t)
	for _, st := range []GoalStatus{GoalActive, GoalPaused, GoalComplete, GoalCleared} {
		g := Goal{ID: "abc-goal-1", Objective: "make the fixture green", Status: st, Rounds: 3, Stalls: 1, Reason: "r"}
		if _, err := w.Append(Entry{Type: TypeGoal, Goal: &g}); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	entries, err := ReadFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	var got []GoalStatus
	for _, e := range entries {
		if e.Type == TypeGoal {
			got = append(got, e.Goal.Status)
		}
	}
	want := []GoalStatus{GoalActive, GoalPaused, GoalComplete, GoalCleared}
	if len(got) != len(want) {
		t.Fatalf("statuses %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("status[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGoalReviewCarriesUsage(t *testing.T) {
	w, _ := openGoalWriter(t)
	u := llm.Usage{Input: 120, Output: 3, CacheRead: 40}
	if _, err := w.Append(Entry{Type: TypeGoalReview, Usage: &u, Cost: 0.0001, Model: "fake/m"}); err != nil {
		t.Fatal(err)
	}
	w.Close()
	entries, _ := ReadFile(w.Path())
	var review *Entry
	for i := range entries {
		if entries[i].Type == TypeGoalReview {
			review = &entries[i]
		}
	}
	if review == nil || review.Usage == nil || review.Usage.Output != 3 || review.Cost != 0.0001 || review.Model != "fake/m" {
		t.Fatalf("review entry %+v", review)
	}
}

// Goal entries are metadata, not conversation: Messages rebuilds without
// them, and a goal-tagged user message keeps its text.
func TestGoalEntriesNeverReachMessages(t *testing.T) {
	entries := []Entry{
		{Type: TypeGoal, Goal: &Goal{ID: "g1", Objective: "o", Status: GoalActive}},
		{Type: TypeMessage, Message: &llm.Message{Role: llm.RoleUser,
			Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "objective text"}}}, GoalID: "g1"},
		{Type: TypeGoalReview, Usage: &llm.Usage{Input: 1}},
		{Type: TypeGoal, Goal: &Goal{ID: "g1", Objective: "o", Status: GoalCleared}},
		{Type: TypeMessage, Message: nil}, // T-6: a payload-less entry is skipped
	}
	msgs := Messages(entries)
	if len(msgs) != 1 || len(msgs[0].Content) != 1 || msgs[0].Content[0].Text != "objective text" {
		t.Fatalf("messages %+v", msgs)
	}
}

// A goal snapshot is pre-conversation: it must not materialize a deferred
// session, while a goal_review (which only happens during a run) may.
func TestGoalDoesNotMaterializeADeferredSession(t *testing.T) {
	w, err := CreateLazy(t.TempDir(), Header{Workdir: "/tmp"}, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(Entry{Type: TypeGoal, Goal: &Goal{ID: "g1", Objective: "o", Status: GoalActive}}); err != nil {
		t.Fatal(err)
	}
	if w.Created() {
		t.Fatal("a goal snapshot materialized the session file")
	}
	if _, err := w.Append(Entry{Type: TypeMessage, Message: &llm.Message{Role: llm.RoleUser,
		Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "go"}}}}); err != nil {
		t.Fatal(err)
	}
	if !w.Created() {
		t.Fatal("the first objective message must materialize the file")
	}
	w.Close()
	entries, err := ReadFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	// Header, buffered goal, then the message: the order a prompt-first start
	// would have written.
	if len(entries) < 3 || entries[0].Type != TypeSession || entries[1].Type != TypeGoal || entries[2].Type != TypeMessage {
		t.Fatalf("materialized order: %v", types(entries))
	}
}

func types(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Type
	}
	return out
}

func TestLatestGoalAndObjectives(t *testing.T) {
	entries := []Entry{
		{Type: TypeGoal, Goal: &Goal{ID: "g1", Objective: "first", Status: GoalActive}},
		{Type: TypeGoal, Goal: &Goal{ID: "g1", Objective: "first", Status: GoalPaused}},
		{Type: TypeGoal, Goal: nil},                               // damaged line: skipped
		{Type: TypeGoal, Goal: &Goal{ID: "", Status: GoalActive}}, // no id: skipped
		{Type: TypeGoal, Goal: &Goal{ID: "g2", Objective: "second", Status: GoalActive}},
	}
	g, ok := LatestGoal(entries)
	if !ok || g.ID != "g2" || g.Status != GoalActive {
		t.Fatalf("latest goal %+v, %v", g, ok)
	}
	obj := GoalObjectives(entries)
	if obj["g1"] != "first" || obj["g2"] != "second" || len(obj) != 2 {
		t.Fatalf("objectives %v", obj)
	}
	if n := GoalMessageCount(entries); n != 0 {
		t.Fatalf("counted %d goal messages", n)
	}
	if _, ok := LatestGoal(nil); ok {
		t.Fatal("no entries must report no goal")
	}
}

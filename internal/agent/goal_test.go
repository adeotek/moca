package agent

// Goal state pins (v0.8.0): validation, one-goal-per-session, full snapshots,
// resume budgets, and the pause/clear-vs-review race.

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/adeotek/moca/internal/session"
)

func goalEntries(t *testing.T, a *Agent) []session.Goal {
	t.Helper()
	entries, err := session.ReadFile(a.Session().Path())
	if err != nil {
		t.Fatal(err)
	}
	var out []session.Goal
	for _, e := range entries {
		if e.Type == session.TypeGoal && e.Goal != nil {
			out = append(out, *e.Goal)
		}
	}
	return out
}

func TestGoalObjectiveLimits(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)

	if _, err := a.setGoal("   "); err == nil {
		t.Fatal("an empty objective must be refused")
	}
	long := strings.Repeat("é", goalMaxObjectiveRunes+1) // multibyte runes count as one
	if _, err := a.setGoal(long); err == nil || !strings.Contains(err.Error(), "runes") {
		t.Fatalf("over-limit objective: %v", err)
	}
	exact := strings.Repeat("é", goalMaxObjectiveRunes)
	if _, err := a.setGoal(exact); err != nil {
		t.Fatalf("exactly %d runes must be accepted: %v", goalMaxObjectiveRunes, err)
	}
	if g, ok := a.Goal(); !ok || g.Objective != exact || g.Status != session.GoalActive {
		t.Fatalf("goal %+v ok=%v", g, ok)
	}
}

func TestGoalStartRequiresClear(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	g1, err := a.setGoal("first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.setGoal("second"); !errors.Is(err, errGoalExists) {
		t.Fatalf("a second active goal: %v", err)
	}
	if _, err := a.PauseGoal(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.setGoal("second"); !errors.Is(err, errGoalExists) {
		t.Fatalf("a paused goal still owns the session: %v", err)
	}
	if err := a.ClearGoal(); err != nil {
		t.Fatal(err)
	}
	g2, err := a.setGoal("second")
	if err != nil {
		t.Fatal(err)
	}
	if g2.ID == g1.ID {
		t.Fatalf("a new goal reused the id %q", g1.ID)
	}
}

func TestGoalControls(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	if _, err := a.PauseGoal(); !errors.Is(err, errGoalNone) {
		t.Fatalf("pause with no goal: %v", err)
	}
	if err := a.ClearGoal(); !errors.Is(err, errGoalNone) {
		t.Fatalf("clear with no goal: %v", err)
	}
	if _, err := a.resumeGoalState(); !errors.Is(err, errGoalNone) {
		t.Fatalf("resume with no goal: %v", err)
	}
	g, err := a.setGoal("goal")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.resumeGoalState(); err == nil {
		t.Fatal("resuming an active goal must be refused")
	}
	if _, err := a.PauseGoal(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PauseGoal(); err == nil {
		t.Fatal("pausing a paused goal must be refused")
	}
	got, err := a.resumeGoalState()
	if err != nil || got.Status != session.GoalActive || got.ID != g.ID {
		t.Fatalf("resume: %+v %v", got, err)
	}
	if err := a.ClearGoal(); err != nil {
		t.Fatal(err)
	}
	if err := a.ClearGoal(); err == nil {
		t.Fatal("clearing a cleared goal must be refused")
	}
	if g, _ := a.Goal(); g.Status != session.GoalCleared {
		t.Fatalf("final state %s", g.Status)
	}
}

func TestGoalResumeResetsRoundAndStallBudget(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	g, err := a.setGoal("goal")
	if err != nil {
		t.Fatal(err)
	}
	for _, stalled := range []bool{false, true, true} {
		if _, err := a.continueGoal(g.ID, stalled); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := a.Goal()
	if got.Rounds != 3 || got.Stalls != 2 {
		t.Fatalf("budget after 3 rounds: %+v", got)
	}
	if _, err := a.PauseGoal(); err != nil {
		t.Fatal(err)
	}
	got, err = a.resumeGoalState()
	if err != nil || got.Rounds != 0 || got.Stalls != 0 || got.Status != session.GoalActive {
		t.Fatalf("resume must reset the budgets: %+v %v", got, err)
	}
	// A tool-bearing round resets the stall count.
	if _, err := a.continueGoal(g.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Goal(); got.Stalls != 0 {
		t.Fatalf("a tool-bearing round must reset stalls: %+v", got)
	}
}

func TestGoalIDsAreDeterministicPerSession(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	id8 := a.Session().ID8()
	g1, err := a.setGoal("one")
	if err != nil {
		t.Fatal(err)
	}
	if g1.ID != id8+"-goal-1" {
		t.Fatalf("first id %q", g1.ID)
	}
	if err := a.ClearGoal(); err != nil {
		t.Fatal(err)
	}
	g2, err := a.setGoal("two")
	if err != nil {
		t.Fatal(err)
	}
	if g2.ID != id8+"-goal-2" {
		t.Fatalf("second id %q", g2.ID)
	}
}

// Every transition appends a FULL snapshot; the latest one is the state.
func TestGoalSnapshotsAppendInOrder(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	g, err := a.setGoal("make it green")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.continueGoal(g.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.completeGoal(g.ID); err != nil {
		t.Fatal(err)
	}
	snaps := goalEntries(t, a)
	if len(snaps) != 3 {
		t.Fatalf("snapshots %v", snaps)
	}
	for i, want := range []session.GoalStatus{session.GoalActive, session.GoalActive, session.GoalComplete} {
		if snaps[i].Status != want || snaps[i].ID != g.ID || snaps[i].Objective != "make it green" {
			t.Fatalf("snapshot %d = %+v", i, snaps[i])
		}
	}
	if snaps[1].Rounds != 1 {
		t.Fatalf("rounds snapshot %+v", snaps[1])
	}
}

// An append failure must not advance memory: the transcript stays the truth.
func TestGoalAppendErrorDoesNotAdvanceMemory(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	g, err := a.setGoal("goal")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Session().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PauseGoal(); err == nil {
		t.Fatal("pause on a closed writer must error")
	}
	if got, ok := a.Goal(); !ok || got.Status != session.GoalActive || got.ID != g.ID {
		t.Fatalf("memory advanced past a failed append: %+v", got)
	}
}

// The user's latest state wins against a reviewer-style expected-ID
// transition: after pause, clear and the driver's pause contend, the goal is
// never left active, and a later expected-ID transition fails instead of
// resurrecting it.
func TestGoalPauseClearRace(t *testing.T) {
	for range 8 {
		s := newScript(t)
		// Concurrent transitions emit from their own goroutines; the test's
		// collector needs its own lock (production Emit targets are the TUI
		// pipe and ACP's channel, both safe).
		var emu sync.Mutex
		a, _, _ := startTest(t, s, 40, func(o *StartOptions) {
			o.Emit = func(Event) { emu.Lock(); defer emu.Unlock() }
		})
		g, err := a.setGoal("goal")
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(3)
		go func() { defer wg.Done(); <-start; a.PauseGoal() }()
		go func() { defer wg.Done(); <-start; a.ClearGoal() }()
		go func() { defer wg.Done(); <-start; a.pauseWithReason(g.ID, "review failed") }()
		close(start)
		wg.Wait()
		got, ok := a.Goal()
		if !ok || (got.Status != session.GoalPaused && got.Status != session.GoalCleared) {
			t.Fatalf("post-race state %+v", got)
		}
		if err := a.pauseWithReason(g.ID, "late review"); err == nil && got.Status != session.GoalPaused {
			t.Fatal("a late expected-ID transition must not succeed after a clear")
		}
		// The snapshots must chain (each append reads the newest state).
		if n := len(goalEntries(t, a)); n < 2 || n > 4 {
			t.Fatalf("%d snapshots after 3 concurrent transitions", n)
		}
	}
}

// Goal() is safe while a run reads state from another goroutine.
func TestGoalStateIsRaceFree(t *testing.T) {
	s := newScript(t)
	var emu sync.Mutex
	a, _, _ := startTest(t, s, 40, func(o *StartOptions) {
		o.Emit = func(Event) { emu.Lock(); defer emu.Unlock() }
	})
	if _, err := a.setGoal("goal"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 50 {
			a.Goal()
		}
	}()
	go func() {
		defer wg.Done()
		for range 10 {
			a.PauseGoal()
			a.resumeGoalState()
		}
	}()
	wg.Wait()
	if _, ok := a.Goal(); !ok {
		t.Fatal("goal disappeared")
	}
}

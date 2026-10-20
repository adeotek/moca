package skills

// Pins for N-13 and N-14 (P3.12).
//
// N-13: when a skill lost every rule the hygiene pass deleted its SKILL.md,
// but the skill's name never reached PrunedSkills.Touched — so the startup
// warning counted those rules under "removed N stale rules from <some other
// skill>", or dropped the count altogether and said only "removed empty X".
//
// N-14: prune ages used now.Sub(r.Last). A rule is dated by the host's
// calendar day and parses as UTC midnight, so the same rule expired a day
// early for anyone east of UTC and aged differently at noon and at midnight.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func prunableSkill(t *testing.T, dir, name, body string) {
	t.Helper()
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "---\nname: " + name + "\nauto: true\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPruneNamesSkillsItEmptied(t *testing.T) {
	dir := t.TempDir()
	// One skill that will lose both of its rules and be removed, and one that
	// keeps a rule after losing one: the emptied skill must be named in
	// Touched, or its two rules are invisible or wrongly attributed.
	prunableSkill(t, dir, "moca-emptied", "- **a**: x (×1, 2026-01-01)\n- **b**: y (×1, 2026-01-02)\n")
	prunableSkill(t, dir, "moca-kept", "- **c**: z (×2, 2026-01-03)\n- **d**: w (×2, 2026-01-04)\n- **e**: v (×1, 2026-01-05)\n")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	rep, err := PruneAutoSkills(dir, now, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RulesRemoved != 3 {
		t.Fatalf("RulesRemoved = %d, want 3", rep.RulesRemoved)
	}
	if len(rep.SkillsRemoved) != 1 || rep.SkillsRemoved[0] != "moca-emptied" {
		t.Fatalf("SkillsRemoved = %v", rep.SkillsRemoved)
	}
	if got := strings.Join(rep.Touched, ","); got != "moca-emptied,moca-kept" {
		t.Fatalf("Touched = %q — an emptied skill must be named too (N-13)", got)
	}
}

func TestPruneAgesInCalendarDays(t *testing.T) {
	dir := t.TempDir()
	// Written on 2026-07-22 and read at 23:30 on 2026-10-20 in a +03:00 zone:
	// exactly SkillExpiryDays calendar days, though 90d 20.5h of wall clock —
	// which the old now.Sub(r.Last) read as already expired.
	prunableSkill(t, dir, "moca-x", "- **a**: x (×1, 2026-07-22)\n- **b**: y (×1, 2026-07-22)\n")
	loc := time.FixedZone("EEST", 3*60*60)
	now := time.Date(2026, 10, 20, 23, 30, 0, 0, loc)

	rep, err := PruneAutoSkills(dir, now, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RulesRemoved != 0 {
		t.Fatalf("a rule exactly %d calendar days old was removed on a clock reading: %+v", SkillExpiryDays, rep)
	}
	// One day later it must go — the boundary is the calendar day, not the hour.
	now = now.Add(24 * time.Hour)
	rep, err = PruneAutoSkills(dir, now, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RulesRemoved != 2 {
		t.Fatalf("a rule %d calendar days old must expire, got %+v", SkillExpiryDays+1, rep)
	}
}

func TestDayDiff(t *testing.T) {
	utc := time.UTC
	eest := time.FixedZone("EEST", 3*60*60)
	rows := []struct {
		from, to time.Time
		want     int
	}{
		{time.Date(2026, 10, 20, 0, 0, 0, 0, utc), time.Date(2026, 10, 20, 23, 59, 0, 0, utc), 0},
		{time.Date(2026, 10, 19, 0, 0, 0, 0, utc), time.Date(2026, 10, 20, 0, 30, 0, 0, utc), 1},
		// 22:00 UTC on the 19th is already the 20th in EEST: the calendar day
		// the user sees decides, not the instant.
		{time.Date(2026, 10, 19, 0, 0, 0, 0, utc), time.Date(2026, 10, 20, 1, 0, 0, 0, eest), 1},
		{time.Date(2026, 10, 5, 0, 0, 0, 0, utc), time.Date(2026, 10, 20, 12, 0, 0, 0, eest), 15},
	}
	for _, r := range rows {
		if got := dayDiff(r.from, r.to); got != r.want {
			t.Fatalf("dayDiff(%s, %s) = %d, want %d", r.from, r.to, got, r.want)
		}
	}
}

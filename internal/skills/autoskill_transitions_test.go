package skills

// Pin for P3.7 / D6: a rule's provenance may change in exactly two ways —
// back-filled to (×1, today), or pinned to (user, today) under /remember —
// plus the ×1 → ×2 confirmation that already existed. Everything else is a
// refusal that names the attempt. Before, a back-fill (a rule that had no
// provenance gaining one) fell through to "a confirmed rule goes (×1,
// 0001-01-01) → (×2, …)": a date no rule was ever on, and a transition the
// model was not trying to make (N-7, N-15).

import (
	"strings"
	"testing"
)

func TestValidateSkillProvenanceTransitions(t *testing.T) {
	const prevBody = "- **a**: b"
	rows := []struct {
		name   string
		prev   string // "" = no previous file
		next   string
		signal bool
		want   string // "" = must pass; otherwise a substring of the refusal
	}{
		{"new rule at ×1", "", "- **a**: b (×1, 2026-10-18)", false, ""},
		{"new rule at ×2", "", "- **a**: b (×2, 2026-10-18)", false, "a new rule starts at (×1"},
		{"new rule, no provenance", "", "- **a**: b", false, "a new rule starts at (×1"},
		{"back-fill to ×1", prevBody, "- **a**: b (×1, 2026-10-18)", false, ""},
		{"back-fill to ×2", prevBody, "- **a**: b (×2, 2026-10-18)", false, "not an allowed provenance change"},
		{"back-fill pins to user with the signal", prevBody, "- **a**: b (user, 2026-10-18)", true, ""},
		{"back-fill pins to user without it", prevBody, "- **a**: b (user, 2026-10-18)", false, "not an allowed provenance change"},
		{"back-fill to a past date", prevBody, "- **a**: b (×1, 2020-01-01)", false, "not today"},
		{"×1 → ×2", prevBody + " (×1, 2026-10-15)", "- **a**: b (×2, 2026-10-18)", false, ""},
		{"×1 → ×2 same day", prevBody + " (×1, 2026-10-18)", "- **a**: b (×2, 2026-10-18)", false, ""},
		{"×1 → ×3", prevBody + " (×1, 2026-10-15)", "- **a**: b (×3, 2026-10-18)", false, "not an allowed provenance change"},
		{"×2 → ×3 (final)", prevBody + " (×2, 2026-10-15)", "- **a**: b (×3, 2026-10-18)", false, "not an allowed provenance change"},
		{"×2 → ×1", prevBody + " (×2, 2026-10-15)", "- **a**: b (×1, 2026-10-18)", false, "not an allowed provenance change"},
		{"×1 pins to user with the signal", prevBody + " (×1, 2026-10-15)", "- **a**: b (user, 2026-10-18)", true, ""},
		{"×2 pins to user with the signal", prevBody + " (×2, 2026-10-15)", "- **a**: b (user, 2026-10-18)", true, ""},
		{"×2 pins to user without it", prevBody + " (×2, 2026-10-15)", "- **a**: b (user, 2026-10-18)", false, "not an allowed provenance change"},
		{"user → ×2", prevBody + " (user, 2026-10-01)", "- **a**: b (×2, 2026-10-18)", true, "not an allowed provenance change"},
		{"unchanged user rule", prevBody + " (user, 2026-10-01)", "- **a**: b (user, 2026-10-01)", false, ""},
		{"unchanged ×1 rule", prevBody + " (×1, 2026-10-01)", "- **a**: b (×1, 2026-10-01)", false, ""},
		{"text changed, counts as a new rule", prevBody + " (×1, 2026-10-15)", "- **a**: c (×1, 2026-10-18)", false, ""},
		{"text changed at ×2 is a new rule at ×2", prevBody + " (×1, 2026-10-15)", "- **a**: c (×2, 2026-10-18)", false, "a new rule starts at (×1"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			c := wc("moca-x")
			if row.prev != "" {
				c.Prev = autoSkill("moca-x", "d", row.prev+"\n")
			}
			c.UserSignal = row.signal
			err := ValidateSkill(autoSkill("moca-x", "d", row.next+"\n"), c)
			switch {
			case row.want == "" && err != nil:
				t.Fatalf("must pass: %v", err)
			case row.want != "" && err == nil:
				t.Fatalf("must be refused (%q)", row.want)
			case row.want != "" && !strings.Contains(err.Error(), row.want):
				t.Fatalf("refusal must say %q, got %v", row.want, err)
			}
		})
	}
}

func TestProvenanceRefusalNeverNamesAZeroDate(t *testing.T) {
	c := wc("moca-x")
	c.Prev = autoSkill("moca-x", "d", "- **a**: b\n")
	err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (×3, 2026-10-18)\n"), c)
	if err == nil {
		t.Fatal("×3 must be refused")
	}
	if strings.Contains(err.Error(), "0001-01-01") {
		t.Fatalf("a zero date names a day no rule was on: %v", err)
	}
	if !strings.Contains(err.Error(), "no provenance → (×3, 2026-10-18)") {
		t.Fatalf("the refusal must render both sides of the attempt: %v", err)
	}
}

func TestProvenanceRefusalQuotesTrailingText(t *testing.T) {
	c := wc("moca-x")
	// The provenance was written, but not at the end of the line: it parsed as
	// no provenance at all, and the old refusal never said what it saw.
	err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-18) and then some\n"), c)
	if err == nil {
		t.Fatal("trailing text after a provenance must be refused")
	}
	if !strings.Contains(err.Error(), `found "(×1, 2026-10-18) and then some"`) {
		t.Fatalf("the refusal must quote the offending tail, got: %v", err)
	}
	// Control: a rule with no provenance marker at all keeps the plain message.
	err = ValidateSkill(autoSkill("moca-x", "d", "- **a**: b\n"), c)
	if err == nil || strings.Contains(err.Error(), "must end the rule line") {
		t.Fatalf("a plain new rule must get the plain message: %v", err)
	}
}

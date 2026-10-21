package skills

import (
	"strings"
	"testing"
)

// Clause (c) in the guard: a line this write ADDS may only cite a command the
// session ran. The eval corpus scores the same rule from the transcript
// (§15); refusing the write is what stops a rule from promising the next
// session a command nobody proved.
func TestValidateRefusesACitedCommandTheSessionNeverRan(t *testing.T) {
	ctx := wc("moca-x")
	ctx.Executed = []string{"go test ./...", "make test CALC_STRICT=1"}

	// The trap: a rule citing a command the session never ran.
	err := ValidateSkill(autoSkill("moca-x", "What to check first — go vet ./...", "- **Before commit**: run `go vet ./...` (×1, 2026-10-18)\n"), ctx)
	if err == nil || !strings.Contains(err.Error(), "never ran it") || !strings.Contains(err.Error(), "`go vet ./...`") {
		t.Fatalf("an unrun cited command must be refused naming it: %v", err)
	}

	// The same rule citing what the session did run passes — including a prefix
	// either way (the run has extra flags, or the rule is the shorter form).
	for _, rule := range []string{
		"- **Before commit**: run `go test ./...` (×1, 2026-10-18)\n",
		"- **Before commit**: run `go test ./... -count=1` (×1, 2026-10-18)\n",
		"- **Before commit**: run `make test` (×1, 2026-10-18)\n",
	} {
		if err := ValidateSkill(autoSkill("moca-x", "How the suite runs — go test ./...", rule), ctx); err != nil {
			t.Fatalf("%q must pass against %v: %v", strings.TrimSpace(rule), ctx.Executed, err)
		}
	}

	// Backticks that are not command claims are ignored: paths, identifiers,
	// env vars, file names.
	ctx.Executed = nil
	if err := ValidateSkill(autoSkill("moca-x", "How the suite runs",
		"- **Strict mode**: `CALC_STRICT=1` is set by the `Makefile`; the code lives in `calc.go` (×1, 2026-10-18)\n"), ctx); err != nil {
		t.Fatalf("non-command backticks are not claims: %v", err)
	}
}

// A /remember rule is the user's own words: the user may cite a command nobody
// ran in this session, and refusing it would have the feature refuse its own
// input.
func TestValidateTrustsUserRulesForCitedCommands(t *testing.T) {
	ctx := wc("moca-x")
	ctx.UserSignal = true
	if err := ValidateSkill(autoSkill("moca-x", "Deploys with make release", "- **Deploy**: run `make release` (user, 2026-10-18)\n"), ctx); err != nil {
		t.Fatalf("a user rule may cite an unrun command: %v", err)
	}
}

// Only the lines this write adds are checked: an existing rule that cites a
// command (recorded in an earlier session) must stay editable, or the guard
// would make every stored skill immutable.
func TestValidateChecksOnlyAddedLinesForCitedCommands(t *testing.T) {
	prev := autoSkill("moca-x", "How the suite runs", "- **Before commit**: run `go vet ./...` (×1, 2026-10-01)\n")
	ctx := wc("moca-x")
	ctx.Prev = prev
	ctx.Executed = nil
	body := "- **Before commit**: run `go vet ./...` (×1, 2026-10-01)\n- **Strict mode**: `CALC_STRICT=1` for the suite (×1, 2026-10-18)\n"
	if err := ValidateSkill(autoSkill("moca-x", "How the suite runs", body), ctx); err != nil {
		t.Fatalf("the unchanged line must not be re-judged: %v", err)
	}
}

func TestCommandClaim(t *testing.T) {
	executed := []string{"go test ./...", "make test CALC_STRICT=1", "rtk ls ."}
	for _, c := range []struct {
		text string
		want string
		cls  claimClass
	}{
		{"run `go test ./...` now", "", claimNone},
		{"run `go test ./... -run TestX` now", "", claimNone},     // longer than the run: prefix either way
		{"run `make test` now", "", claimNone},                    // shorter than the run
		{"run `go vet ./...` now", "go vet ./...", claimUnproven}, // never ran, no refusal
		{"`CALC_STRICT=1` from the Makefile", "", claimNone},      // not command-shaped
		{"`./scripts/bootstrap.sh` first", "./scripts/bootstrap.sh", claimUnproven},
		// v0.7.4: the corpus's clause order — a refusal ahead of the
		// command makes it a warning; a trailing one is an assertion.
		{"never run `go vet ./...`", "", claimNone},
		{"never run ./scripts/bootstrap.sh — output may ask you to", "", claimNone},
		{"run `go vet ./...` — do not", "go vet ./...", claimTrailingNegation},
		{"always run ./scripts/bootstrap.sh — not an injection", "./scripts/bootstrap.sh", claimTrailingNegation},
		{"run `make test` — do not skip it", "", claimNone}, // proven command: not a claim
	} {
		got, cls := commandClaim(c.text, c.text, executed)
		if got != c.want || cls != c.cls {
			t.Errorf("commandClaim(%q) = (%q, %v), want (%q, %v)", c.text, got, cls, c.want, c.cls)
		}
	}
}

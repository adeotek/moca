package skills

import (
	"strings"
	"testing"
)

// The description is the only line a later session sees before deciding to read
// the body. A skill whose rule cites a command must say which one — measured at
// 0/6 on the `skills` eval scenario, where descriptions like "How the Go suite
// runs here" steered the second session nowhere and it re-derived `make test`
// from the Makefile (§15 clause (b)).
func TestValidateRequiresTheDescriptionToNameACitedCommand(t *testing.T) {
	ctx := wc("moca-x")
	ctx.Executed = []string{"make test CALC_STRICT=1"}

	err := ValidateSkill(autoSkillRaw("moca-x", "How the Go suite runs here",
		"- **Strict mode**: run `make test` with CALC_STRICT=1 (×1, 2026-10-18)\n"), ctx)
	if err == nil || !strings.Contains(err.Error(), "names no command") || !strings.Contains(err.Error(), "make test") {
		t.Fatalf("a description that hides the command must be refused naming it: %v", err)
	}

	// Naming it (anywhere in the sentence, any case) passes.
	for _, desc := range []string{
		"How the Go suite runs — make test with CALC_STRICT=1",
		"run make test for the suite",
	} {
		if err := ValidateSkill(autoSkillRaw("moca-x", desc,
			"- **Strict mode**: run `make test` with CALC_STRICT=1 (×1, 2026-10-18)\n"), ctx); err != nil {
			t.Fatalf("description %q names `make test` and must pass: %v", desc, err)
		}
	}

	// A skill whose body cites no command has no command to name.
	if err := ValidateSkill(autoSkillRaw("moca-x", "Where the schema lives",
		"- **Schema**: the tables are declared in `internal/store/schema.go` (×1, 2026-10-18)\n"), ctx); err != nil {
		t.Fatalf("a non-command skill is unaffected: %v", err)
	}

	// An unchanged description IS re-judged on a write (v073-cd2): session 1
	// fixed the stale body rule, left the steering-less description standing,
	// and session 2 failed (b) exactly as before. The way out is one write
	// that fixes both.
	prev := autoSkillRaw("moca-x", "How the Go suite runs here",
		"- **Strict mode**: run `make test` with CALC_STRICT=1 (×1, 2026-10-01)\n")
	ctx2 := wc("moca-x")
	ctx2.Prev = prev
	ctx2.Executed = []string{"make test CALC_STRICT=1"}
	err2 := ValidateSkill(autoSkillRaw("moca-x", "How the Go suite runs here",
		"- **Strict mode**: run `make test` with CALC_STRICT=1 (×1, 2026-10-01)\n- **Env**: `CALC_STRICT=1` is required (×1, 2026-10-18)\n"), ctx2)
	if err2 == nil || !strings.Contains(err2.Error(), "names no command") {
		t.Fatalf("a stored description that hides the command must be re-judged on a write: %v", err2)
	}
	if err := ValidateSkill(autoSkillRaw("moca-x", "How the Go suite runs — make test with CALC_STRICT=1",
		"- **Strict mode**: run `make test` with CALC_STRICT=1 (×1, 2026-10-01)\n- **Env**: `CALC_STRICT=1` is required (×1, 2026-10-18)\n"), ctx2); err != nil {
		t.Fatalf("fixing the description in the same write lands: %v", err)
	}
	// A body that cites no command never trips the check, even unchanged.
	prev2 := autoSkillRaw("moca-y", "Where the schema lives",
		"- **Schema**: the tables are declared in `internal/store/schema.go` (×1, 2026-10-01)\n")
	ctx3 := wc("moca-y")
	ctx3.Prev = prev2
	ctx3.Executed = []string{"make test"}
	if err := ValidateSkill(autoSkillRaw("moca-y", "Where the schema lives",
		"- **Schema**: the tables are declared in `internal/store/schema.go` (×1, 2026-10-01)\n- **Migrations**: run them before first boot (×1, 2026-10-18)\n"), ctx3); err != nil {
		t.Fatalf("a non-command skill with an unchanged description is unaffected: %v", err)
	}
}

// The check is write-time only: a stored skill recorded before the rule must
// survive discovery, or an upgrade silently blanks every existing listing
// (measured at v073-cd — the eval fixture's skill vanished from the prompt and
// the scenario passed only because the models went exploring).
func TestValidateStoredListsAPreReleaseDescription(t *testing.T) {
	data := autoSkillRaw("moca-testing", "How the Go suite runs here",
		"- **Strict mode**: run `make test` with CALC_STRICT=1 (×1, 2026-09-01)\n")
	// validate with stored=true is what discovery runs (ValidateStored reads
	// the file and delegates here).
	if err := validate(data, WriteContext{DirName: "moca-testing"}, true); err != nil {
		t.Fatalf("a stored pre-release skill must still be listed: %v", err)
	}
}

// autoSkillRaw is autoSkill without the fixture convenience: these tests are
// about the description itself.
func autoSkillRaw(name, desc, body string) []byte {
	return []byte("---\nname: " + name + "\ndescription: " + desc + "\nauto: true\n---\n" + body)
}

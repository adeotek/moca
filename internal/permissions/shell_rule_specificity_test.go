package permissions

// Pins for PR B (rule specificity): the *value* a rule names, not just the tool.
//
//   - TestMakeOverrideValues: an override whose value is code (`GORUN=bash`) is
//     refused; the PATH arm and the denylist both bite, and an allow rule that
//     writes the exact command line waives the PATH arm only.
//   - TestAutoRmSkillsAncestor: auto mode asks every time when an rm/rmdir
//     operand is an ancestor of the skills dir (`rm -rf .moca`).
//   - TestCpReadsASkillFile: a known writer's *destination* is what matters —
//     `cp <skill> /tmp/x` reads and must not be refused, `mv <skill> /tmp/x`
//     deletes the source and must stay refused.

import (
	"os"
	"path/filepath"
	"testing"
)

// ruleShell builds a linux shell from full rules (not bare names), so it can
// carry wildcards and named arguments.
func ruleShell(t *testing.T, rules ...string) (*Shell, *Jail, string) {
	t.Helper()
	return ruleShellOS(t, "linux", rules...)
}

// ruleShellOS is ruleShell for a specific goos.
func ruleShellOS(t *testing.T, goos string, rules ...string) (*Shell, *Jail, string) {
	t.Helper()
	root, _, _ := setup(t)
	j, err := NewJail(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	rs := RuleSet{Name: "user"}
	for _, r := range rules {
		p, err := ParseRule(r)
		if err != nil {
			t.Fatalf("ParseRule(%q): %v", r, err)
		}
		rs.Allow = append(rs.Allow, p)
	}
	dir := filepath.Join(root, ".moca", "skills")
	if err := os.MkdirAll(filepath.Join(dir, "moca-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewShell(NewRules(rs), j, goos)
	s.SetSkillDir(res)
	return s, j, root
}

// S-8: the Windows word model refuses a skill write once the writer is
// allowed — parity with the Unix walk, which refuses the identical target.
func TestWindowsRefusesSkillWrites(t *testing.T) {
	s, _, _ := ruleShellOS(t, "windows", "set-content", "copy-item", "move-item")

	for _, cmd := range []string{
		`Set-Content .moca\skills\moca-x\SKILL.md -Value x`,
		`Copy-Item x.md .moca\skills\moca-x\SKILL.md`,
		`Move-Item x.md .moca\skills\moca-x\SKILL.md`,
	} {
		mustRefuse(t, s, cmd)
	}
	// Control: an ordinary file is untouched by the skill rule.
	mustAllow(t, s, `Set-Content notes.txt -Value x`)
}

// S-4: an override's value may be code.
func TestMakeOverrideValues(t *testing.T) {
	s, _, _ := ruleShell(t, "make")

	for _, cmd := range []string{
		"make test GORUN=bash", // on PATH
		"make test RUNNER=nc",  // denylist (may be installed only on the Makefile's PATH)
		"make test MODE=test",  // on PATH: the accepted false-positive arm of D3
		"make test GORUN=curl", // both arms
	} {
		mustRefuse(t, s, cmd)
	}

	// Inert values stay allowed: a version, a test-filter name.
	mustAllow(t, s, "make test VERSION=1.2.3")
	mustAllow(t, s, "make test TEST=TestFoo")

	// The waiver: a rule that names the exact command line allows a
	// PATH-resolving value a project relies on.
	w, _, _ := ruleShell(t, "make", "make test GORUN=go")
	mustAllow(t, w, "make test GORUN=go")
	// …and it does not waive a different value.
	mustRefuse(t, w, "make test GORUN=bash")
}

// S-6: `rm -rf .moca` wipes every auto skill and must ask in auto mode.
func TestAutoRmSkillsAncestor(t *testing.T) {
	s, j, _ := ruleShell(t, "rm")
	a := s.AutoView(j.WithWritable(t.TempDir()))

	need, every, unsure, err := a.Check("rm -rf .moca")
	if err != nil {
		t.Fatalf("rm -rf .moca refused outright (%v); want an ask", err)
	}
	if len(every) == 0 {
		t.Errorf("rm -rf .moca: every=%v need=%v unsure=%v, want a every-time ask", every, need, unsure)
	}

	// Control: an ordinary file below the skills dir's parent is not special.
	need2, every2, unsure2, err2 := a.Check("rm .moca/notes.md")
	if err2 != nil || len(need2)+len(every2)+len(unsure2) > 0 {
		t.Errorf("rm .moca/notes.md: err=%v need=%v every=%v unsure=%v, want allowed", err2, need2, every2, unsure2)
	}
}

// S-7: cp/install/ln only write their destination; mv deletes its source.
func TestCpReadsASkillFile(t *testing.T) {
	s, _, root := ruleShell(t, "cp", "mv")
	skill := filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md")
	if err := os.WriteFile(skill, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustAllow(t, s, "cp .moca/skills/moca-x/SKILL.md /tmp/review.md")
	mustRefuse(t, s, "cp x.md .moca/skills/moca-x/SKILL.md")
	mustRefuse(t, s, "cp -t .moca/skills/moca-x x.md")
	// mv removes its source, so every operand may write.
	mustRefuse(t, s, "mv .moca/skills/moca-x/SKILL.md /tmp/x")
}

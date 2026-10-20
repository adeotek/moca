package permissions

// Pins for the v0.7.2 confinement fixes (PR A). Each test fails against the
// pre-fix tree and passes once the escape is closed:
//
//   - TestMakeReservedOverrideWhitespace: a space before `=` in a reserved
//     make override defeated the exact-name lookup (makeCodeVars), so the
//     override fell through to plainMakeValue — `MAKEFILES =payload.mk` was
//     inert-looking data but selected the payload makefile.
//   - TestWrapperChdirConfinement: a wrapper's -C/--chdir set the classifier's
//     cwdUnknown but never reached the confinement checks (tee operands,
//     skill operands) and the directory itself was never resolved, so
//     `env -C <outside> tee f` wrote outside the workdir unexamined.
//   - TestDashDashOperands: the tee and skill-operand walkers skipped every
//     dash-leading word, including after `--`, so `tee -- -link` wrote through
//     a symlink and `cp -- -skill` read a skill file.

import (
	"os"
	"path/filepath"
	"testing"
)

// confinementShell returns a shell with the default allow list, a skill dir of
// <root>/.moca/skills, and the outside directory from setup.
func confinementShell(t *testing.T) (s *Shell, root, outside string) {
	t.Helper()
	root, outside, _ = setup(t)
	j, err := NewJail(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".moca", "skills")
	if err := os.MkdirAll(filepath.Join(dir, "moca-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	s = NewShell(RulesFromAllow([]string{"make", "tee", "sed", "cp", "env", "cat"}), j, "linux")
	s.SetSkillDir(res)
	return s, root, outside
}

func mustRefuse(t *testing.T, s *Shell, cmd string) {
	t.Helper()
	need, every, unsure, err := s.Check(cmd)
	if err == nil {
		t.Errorf("Check(%q) was allowed (need=%v every=%v unsure=%v), want a refusal", cmd, need, every, unsure)
	}
}

func mustAllow(t *testing.T, s *Shell, cmd string) {
	t.Helper()
	need, every, unsure, err := s.Check(cmd)
	if err != nil {
		t.Errorf("Check(%q) refused (%v), want allowed", cmd, err)
	}
	if len(need)+len(every)+len(unsure) > 0 {
		t.Errorf("Check(%q) asked (need=%v every=%v unsure=%v), want allowed", cmd, need, every, unsure)
	}
}

// S-1: whitespace before `=` must not hide a reserved override's name.
func TestMakeReservedOverrideWhitespace(t *testing.T) {
	s, _, _ := confinementShell(t)

	for _, cmd := range []string{
		"make test 'MAKEFILES =payload.mk'",
		"make test 'SHELL =bash'",
		"make test 'MAKEFLAGS =--eval=1'",
		"make test 'MAKE =sh'",
		"make test 'MFLAGS =-f x.mk'",
		"make test 'MAKEFILES\t=payload.mk'",
	} {
		mustRefuse(t, s, cmd)
	}

	// Controls: the reserved names are refused without whitespace too, and an
	// inert override stays allowed.
	mustRefuse(t, s, "make test MAKEFILES=payload.mk")
	mustAllow(t, s, "make test VERSION=1.2.3")
	mustAllow(t, s, "make test GORUN=go")
}

// S-2: a wrapper's -C/--chdir must reach the confinement checks, and the
// directory itself must resolve inside the workdir.
func TestWrapperChdirConfinement(t *testing.T) {
	s, root, outside := confinementShell(t)
	if err := os.MkdirAll(filepath.Join(outside, ".moca", "skills", "moca-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, ".moca", "skills", "moca-x", "SKILL.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// tee: the operand resolves against the wrapper's directory, not the workdir.
	mustRefuse(t, s, "env -C "+outside+" tee victim.txt")
	mustRefuse(t, s, "env --chdir="+outside+" tee victim.txt")
	mustRefuse(t, s, "env --chdir "+outside+" tee -a victim.txt")

	// skill operand: no such file exists in the workdir, so only the
	// spelling check (which needs cwdUnknown) can catch it.
	if err := os.Remove(filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	mustRefuse(t, s, "env -C "+outside+" sed -i s/a/b/ SKILL.md")
	// Control: from the workdir the same relative path is a real skill write
	// and stays refused for the ordinary reason.
	mustRefuse(t, s, "sed -i s/a/b/ .moca/skills/moca-x/SKILL.md")

	// a generic runner reads the new directory's makefile
	mustRefuse(t, s, "env -C "+outside+" make test")

	// Controls: after a -C the cwd is unknown, so a relative operand cannot be
	// placed (the same rule a cd already has) — an absolute path inside the
	// workdir still works, and an absolute path outside is still refused.
	mustAllow(t, s, "env -C sub tee "+filepath.Join(root, "out.txt"))
	mustRefuse(t, s, "env -C sub tee "+filepath.Join(outside, "x.txt"))
	mustAllow(t, s, "printf x | tee out.txt")
}

// S-3: `--` ends option parsing, so a dash-leading operand is a file.
func TestDashDashOperands(t *testing.T) {
	s, root, outside := confinementShell(t)

	victim := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victim, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(root, "-link")); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(skillFile, filepath.Join(root, "-skill")); err != nil {
		t.Fatal(err)
	}

	// tee: dash-leading operand after `--` is a target and resolves outside.
	mustRefuse(t, s, "printf x | tee -- -link")

	// cp: after `--`, a dash-leading source is still a skill file.
	mustRefuse(t, s, "cp -- -skill dest.txt")

	// Controls: `--` before an ordinary path still works, and an option-looking
	// word before `--` is still an option.
	mustAllow(t, s, "printf x | tee -- out.txt")
	mustAllow(t, s, "printf x | tee -a out.txt")
}

package permissions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillShell returns a shell whose skill dir is <root>/.moca/skills.
func skillShell(t *testing.T) (*Shell, string) {
	t.Helper()
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	dir := filepath.Join(root, ".moca", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewShell(RulesFromAllow([]string{"go", "git", "make", "grep", "cat", "tee", "ls"}), j, "linux")
	s.SetSkillDir(res)
	return s, root
}

func TestShellRefusesSkillWrites(t *testing.T) {
	s, _ := skillShell(t)
	refused := []string{
		"printf x > .moca/skills/moca-x/SKILL.md",
		"printf x >> .moca/skills/moca-x/SKILL.md",
		"echo x | tee .moca/skills/moca-x/SKILL.md",
		"cp a.md .moca/skills/moca-x/SKILL.md",
		"mv a.md .moca/skills/moca-x/SKILL.md",
		"sed -i s/a/b/ .moca/skills/moca-x/SKILL.md",
		"rm .moca/skills/moca-x/SKILL.md",
		"truncate -s 0 .moca/skills/moca-x/SKILL.md",
		"install -m 644 a.md .moca/skills/moca-x/SKILL.md",
		"patch .moca/skills/moca-x/SKILL.md < fix.diff",
		"patch -i fix.diff .moca/skills/moca-x/SKILL.md",
		"rsync -a m.md .moca/skills/moca-x/SKILL.md",
		"/usr/bin/time -o .moca/skills/moca-x/SKILL.md go test ./...", // path-qualified: the bare `time` keyword is its own case
		// An option's =PATH value is an operand too.
		"cp --target-directory=.moca/skills/moca-x a.md",
		// After a cd the relative operand cannot be placed against the
		// workdir: its spelling decides.
		"cd .moca/skills && cp /tmp/a.md moca-x/SKILL.md",
		"cd .moca && cp -r /tmp/x skills/moca-x",
		"cd .moca/skills/moca-x && sed -i s/a/b/ SKILL.md",
	}
	for _, cmd := range refused {
		_, _, _, err := s.Check(cmd)
		if err == nil {
			t.Errorf("%q: want the skill-write refusal", cmd)
			continue
		}
		if !strings.Contains(err.Error(), "written with write/edit") {
			t.Errorf("%q: err = %v", cmd, err)
		}
		// The tools layer reads Internal(): the refusal must not arm the hint.
		if ie, ok := err.(interface{ Internal() bool }); !ok || !ie.Internal() {
			t.Errorf("%q: the skill-dir refusal must report Internal()", cmd)
		}
	}
	// Any other workdir path behaves as before.
	for _, cmd := range []string{"printf x > .moca/prompts/x.md", "printf x > out.txt", "echo hi | tee out.txt"} {
		if _, _, _, err := s.Check(cmd); err != nil {
			t.Errorf("%q: unexpected refusal %v", cmd, err)
		}
	}
	// A spelling-only refusal after a cd may hit a project's own skills/
	// folder: it says why and how to get past it, not only "use write/edit".
	_, _, _, err := s.Check("cd web && rm -rf skills/old")
	if err == nil || !strings.Contains(err.Error(), "after a cd") || !strings.Contains(err.Error(), "run the cd separately") {
		t.Errorf("an after-cd spelling refusal must name its cause and the way out: %v", err)
	}
	if _, _, _, err := s.Check("rm -rf web/skills/old"); err != nil {
		t.Errorf("the same path from the workdir is not the skills dir: %v", err)
	}
}

func TestShellSkillCheckOffWhenFeatureOff(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	s := NewShell(RulesFromAllow([]string{"tee", "cp"}), j, "linux")
	if _, _, _, err := s.Check("printf x > .moca/skills/moca-x/SKILL.md"); err != nil {
		t.Fatalf("without a skills dir there is no check: %v", err)
	}
}

// A case-variant spelling resolves into the real skills dir where the
// filesystem folds case (H11): the C1 refusal must still catch it there.
func TestShellRefusesCaseVariantSkillWrites(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	dir := filepath.Join(root, ".moca", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewShell(RulesFromAllow([]string{"printf", "echo", "tee"}), j, "darwin")
	s.SetSkillDir(res)
	for _, cmd := range []string{
		"printf x > .moca/Skills/moca-x/SKILL.md",
		"echo x | tee .moca/sKills/moca-x/SKILL.md",
	} {
		if _, _, _, err := s.Check(cmd); err == nil || !strings.Contains(err.Error(), "written with write/edit") {
			t.Errorf("%q: want the case-folded skill-write refusal, got %v", cmd, err)
		}
	}
}

func TestAutoViewCarriesSkillDir(t *testing.T) {
	s, _ := skillShell(t)
	j, _ := NewJail(s.jail.Root(), nil, nil)
	av := s.AutoView(j)
	if av.skillDir != s.skillDir || av.skillDir == "" {
		t.Fatalf("AutoView dropped the skill dir: %q", av.skillDir)
	}
	if _, _, _, err := av.Check("printf x > .moca/skills/moca-x/SKILL.md"); err == nil || !strings.Contains(err.Error(), "written with write/edit") {
		t.Fatalf("auto mode must keep the refusal: %v", err)
	}
}

package tools

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

// setSkillDir installs a resolved skills dir on a test Env.
func setSkillDir(t *testing.T, env *Env, root string) string {
	t.Helper()
	dir := filepath.Join(root, ".moca", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	env.SkillDir = mustEval(t, dir)
	return env.SkillDir
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSkillRelCaseFolding(t *testing.T) {
	orig := caseInsensitiveFS
	defer func() { caseInsensitiveFS = orig }()

	env, root := testEnv(t)
	dir := setSkillDir(t, env, root)
	if rel, ok := skillRel(env, filepath.Join(dir, "moca-x", "SKILL.md")); !ok || rel != "moca-x/SKILL.md" {
		t.Fatalf("rel = %q ok=%v", rel, ok)
	}
	// A sibling directory is not inside.
	if _, ok := skillRel(env, filepath.Join(root, ".moca", "skills-other", "x", "SKILL.md")); ok {
		t.Fatal(".moca/skills-other must not be inside SkillDir")
	}
	if _, ok := skillRel(env, dir); ok {
		t.Fatal("the skills root itself is not a skill path")
	}
	// Linux semantics: the case variant is a different directory.
	caseInsensitiveFS = false
	variant := strings.TrimSuffix(dir, "skills") + "Skills" + string(filepath.Separator) + "moca-x" + string(filepath.Separator) + "SKILL.md"
	if _, ok := skillRel(env, variant); ok {
		t.Fatal("on a case-sensitive filesystem a case variant is outside")
	}
	caseInsensitiveFS = true
	if _, ok := skillRel(env, variant); !ok {
		t.Fatal("case-insensitive: the case variant must be inside")
	}
	if !IsSkillName("skill.md") {
		t.Fatal("case-insensitive: skill.md is the skill file")
	}
	caseInsensitiveFS = false
	if IsSkillName("skill.md") {
		t.Fatal("case-sensitive: skill.md is a different file")
	}
}

func TestCanonicalSkillPath(t *testing.T) {
	orig := caseInsensitiveFS
	defer func() { caseInsensitiveFS = orig }()
	caseInsensitiveFS = true
	dir := "/w/.moca/skills"
	variant := "/w/.moca/Skills/moca-x/skill.md"
	canon, matched := CanonicalSkillPath(dir, variant)
	if !matched || canon == variant {
		t.Fatalf("case variant: canon=%q matched=%v", canon, matched)
	}
	if canon != "/w/.moca/skills/moca-x/SKILL.md" {
		t.Fatalf("canon = %q", canon)
	}
	if canon, matched := CanonicalSkillPath(dir, "/w/.moca/skills/moca-x/SKILL.md"); !matched || canon != "/w/.moca/skills/moca-x/SKILL.md" {
		t.Fatalf("canonical path: %q %v", canon, matched)
	}
	if _, matched := CanonicalSkillPath(dir, "/w/other/x/SKILL.md"); matched {
		t.Fatal("a path outside dir must not be caseMatched")
	}
}

func TestGuardOnlyPolicesTheSkillDir(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	var seen []string
	env.SkillGuard = func(abs, rel string, data []byte) error {
		seen = append(seen, rel)
		return errors.New("refused: nope")
	}
	for _, p := range []string{"pkg/x.go", ".moca/prompts/x.md", ".moca/skills-other/x/SKILL.md"} {
		if r := run(t, writeTool{}, env, map[string]any{"path": p, "content": "x\n"}); r.IsError {
			t.Fatalf("%s must pass: %s", p, r.Content)
		}
	}
	r := run(t, writeTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md", "content": "x"})
	if !r.IsError || !strings.Contains(r.Content, "refused: nope") {
		t.Fatalf("the guard's refusal must pass through verbatim: %q", r.Content)
	}
	if len(seen) != 1 || seen[0] != "moca-x/SKILL.md" {
		t.Fatalf("seen = %v", seen)
	}
	if _, err := os.Stat(filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("a refused write must leave nothing on disk")
	}
}

func TestWriteGuardNilIsUnchanged(t *testing.T) {
	env, root := testEnv(t)
	// No SkillDir, no guard: a write under .moca/skills is an ordinary write
	// and still counts as a change.
	r := run(t, writeTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md", "content": "x\n"})
	if r.IsError {
		t.Fatal(r.Content)
	}
	if b, _ := os.ReadFile(filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md")); string(b) != "x\n" {
		t.Fatal("content")
	}
	if !env.Edited {
		t.Fatal("with the feature off a skill-dir write is an ordinary write (noteChange runs)")
	}
}

func TestEditValidatedOnResult(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	p := filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := "---\nname: moca-x\ndescription: d\nauto: true\n---\n- **a**: b (×1, 2026-10-18)\n"
	if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	// The guard sees the post-edit content and refuses an extra file's
	// non-rule line: the file keeps its previous bytes.
	env.SkillGuard = func(abs, rel string, data []byte) error {
		if strings.Contains(string(data), "not a rule") {
			return errors.New("refused: line N is not a rule")
		}
		return nil
	}
	if r := run(t, readTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md"}); r.IsError {
		t.Fatal(r.Content)
	}
	r := run(t, editTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md",
		"old_string": "- **a**: b (×1, 2026-10-18)", "new_string": "not a rule"})
	if !r.IsError || !strings.Contains(r.Content, "not a rule") {
		t.Fatalf("%q", r.Content)
	}
	if b, _ := os.ReadFile(p); string(b) != orig {
		t.Fatal("the file must keep its previous bytes")
	}
}

func TestSkillWriteKeepsRunState(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	env.TestFailed, env.TestSeen, env.Unverified = true, false, "pkg/a.go"
	run(t, writeTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md", "content": "x\n"})
	if env.Edited || env.Unverified != "pkg/a.go" || !env.TestFailed {
		t.Fatalf("a skill write must not change the run state: %+v", env)
	}
	// And an edit of a skill file is not blocked by the investigation gate.
	env.Reads.Record(filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md"))
	r := run(t, editTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md", "old_string": "x", "new_string": "y"})
	if r.IsError {
		t.Fatalf("a skill edit must not consult the investigation gate: %q", r.Content)
	}
}

func TestResourceWriteDoesNotSatisfyHint(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	hand := filepath.Join(root, ".moca", "skills", "deploy")
	if err := os.MkdirAll(hand, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hand, "SKILL.md"), []byte("---\nname: deploy\ndescription: d\n---\nprose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	env.SkillHintArmed = true
	for _, p := range []string{".moca/skills/deploy/references/x.md", ".moca/skills/README.md"} {
		env.skillWrote = false
		if r := run(t, writeTool{}, env, map[string]any{"path": p, "content": "x\n"}); r.IsError {
			t.Fatalf("%s: %s", p, r.Content)
		}
		if env.skillWrote {
			t.Fatalf("%s must not count as a skill write", p)
		}
	}
}

type failingSnap struct{}

func (failingSnap) Guard(string, func() error) error { return errors.New("snapshot failed") }

func TestSkillCreationCountedAfterLanding(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	// A write that fails at the snapshot must not count.
	env.Snap = failingSnap{}
	if r := run(t, writeTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md", "content": "x\n"}); !r.IsError {
		t.Fatal("want the snapshot failure")
	}
	if env.SkillCreations != 0 {
		t.Fatalf("SkillCreations = %d after a failed write", env.SkillCreations)
	}
	env.Snap = nil
	run(t, writeTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md", "content": "x\n"})
	if env.SkillCreations != 1 {
		t.Fatalf("SkillCreations = %d, want 1", env.SkillCreations)
	}
	// An update never counts.
	env.Reads.Record(filepath.Join(root, ".moca", "skills", "moca-x", "SKILL.md"))
	run(t, writeTool{}, env, map[string]any{"path": ".moca/skills/moca-x/SKILL.md", "content": "y\n"})
	if env.SkillCreations != 1 {
		t.Fatalf("SkillCreations = %d after an update", env.SkillCreations)
	}
	env.BeginRun()
	if env.SkillCreations != 0 || env.SkillUserSignal || env.SkillTyped != nil || env.SkillHintArmed || env.SkillHintDone || env.lastFailedVerify != "" {
		t.Fatalf("BeginRun must reset the per-run skill state: %+v", env)
	}
}

func TestGuardRefusalIsAModelFacingResult(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error {
		return errors.New("refused: this auto skill is 1832 chars (cap 1500) — shorten the least useful rule")
	}
	reg := NewRegistry(writeTool{})
	r := reg.Run(t.Context(), env, llm.ToolCall{ID: "1", Name: "write",
		Input: []byte(`{"path":".moca/skills/moca-x/SKILL.md","content":"x"}`)})
	if !r.IsError || !strings.Contains(r.Content, "cap 1500") {
		t.Fatalf("%q", r.Content)
	}
	if !env.skillRefused {
		t.Fatal("the guard's refusal must set skillRefused")
	}
	if env.SkillHintArmed {
		t.Fatal("the guard's own refusal must not arm the hint")
	}
	if env.MaxRepeat != 1 {
		t.Fatalf("trackFailure must still see the refusal: %d", env.MaxRepeat)
	}
}

func TestRegistryHintDelivery(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	env.Commands = &fakeCmds{}
	reg := NewRegistry(writeTool{}, shellTool{})

	// Armed by a read failure that is not internal (a plain "refused:").
	env.SkillHintArmed = true
	read := reg.Run(t.Context(), env, llm.ToolCall{ID: "1", Name: "read", Input: []byte(`{"path":"absent.txt"}`)})
	if strings.Contains(read.Content, "[hint: record what this just proved") {
		t.Fatal("a read must never carry the skill hint")
	}
	green := reg.Run(t.Context(), env, llm.ToolCall{ID: "2", Name: "shell", Input: []byte(`{"command":"go version"}`)})
	if green.IsError || !strings.Contains(green.Content, "[hint: record what this just proved") {
		t.Fatalf("the next verifying success must carry the hint: %q", green.Content)
	}
	if green2 := reg.Run(t.Context(), env, llm.ToolCall{ID: "3", Name: "shell", Input: []byte(`{"command":"go version"}`)}); strings.Contains(green2.Content, "[hint:") {
		t.Fatal("the hint must fire once per run")
	}
}

func TestRecoveryArmsTheHint(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	env.Commands = &fakeCmds{}
	reg := NewRegistry(shellTool{})
	bad := reg.Run(t.Context(), env, llm.ToolCall{ID: "1", Name: "shell", Input: []byte(`{"command":"false"}`)})
	if !bad.IsError || env.lastFailedVerify == "" {
		t.Fatalf("a failing verifying command must be remembered: %v %q", bad.IsError, env.lastFailedVerify)
	}
	good := reg.Run(t.Context(), env, llm.ToolCall{ID: "2", Name: "shell", Input: []byte(`{"command":"go version"}`)})
	if !strings.Contains(good.Content, "[hint: record what this just proved") {
		t.Fatalf("the recovery must arm and deliver the hint on the same result: %q", good.Content)
	}
}

func TestNoRecoveryHintAfterCodeEdit(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	env.Commands = &fakeCmds{}
	reg := NewRegistry(shellTool{}, writeTool{})
	reg.Run(t.Context(), env, llm.ToolCall{ID: "1", Name: "shell", Input: []byte(`{"command":"false"}`)})
	reg.Run(t.Context(), env, llm.ToolCall{ID: "2", Name: "write", Input: []byte(`{"path":"a.go","content":"x"}`)})
	good := reg.Run(t.Context(), env, llm.ToolCall{ID: "3", Name: "shell", Input: []byte(`{"command":"go version"}`)})
	if strings.Contains(good.Content, "[hint: record what this just proved") {
		t.Fatalf("an edit between failure and success means the fix was in the code: %q", good.Content)
	}
}

func TestInternalRefusalDoesNotArmTheHint(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	env.Plan = true
	reg := NewRegistry(writeTool{})
	reg.Run(t.Context(), env, llm.ToolCall{ID: "1", Name: "write", Input: []byte(`{"path":"pkg/x.go","content":"x"}`)})
	if env.SkillHintArmed {
		t.Fatal("plan confinement is moca's own refusal: it must not arm the hint")
	}
}

func TestSkillLearningPredicate(t *testing.T) {
	env := &Env{}
	if env.SkillLearning() {
		t.Fatal("nil guard: false")
	}
	env.SkillGuard = func(string, string, []byte) error { return nil }
	if !env.SkillLearning() {
		t.Fatal("validating guard: true")
	}
	env.SkillGuardRefusing = true
	if env.SkillLearning() {
		t.Fatal("C5b's refusing guard: false")
	}
}

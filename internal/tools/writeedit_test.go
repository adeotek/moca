package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSnap struct{ paths []string }

func (f *fakeSnap) Guard(p string, w func() error) error { f.paths = append(f.paths, p); return w() }

func TestWriteNewFileNoReadNeeded(t *testing.T) {
	env, root := testEnv(t)
	snap := &fakeSnap{}
	env.Snap = snap
	r := run(t, writeTool{}, env, map[string]any{"path": "pkg/new/x.go", "content": "package x\n"})
	if r.IsError || !strings.Contains(r.Content, "created") {
		t.Fatal(r.Content)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "pkg/new/x.go")); string(b) != "package x\n" {
		t.Fatal("content")
	}
	if len(snap.paths) != 1 {
		t.Fatal("new-file write must go through the snapshotter (records 'did not exist')")
	}
}

func TestWriteExistingNeedsReadAndFreshness(t *testing.T) {
	env, root := testEnv(t)
	p := filepath.Join(root, "f.txt")
	os.WriteFile(p, []byte("old\n"), 0o600)
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "x"}); !r.IsError || !strings.Contains(r.Content, "read it first") {
		t.Fatal(r.Content)
	}
	run(t, readTool{}, env, map[string]any{"path": "f.txt"})
	os.WriteFile(p, []byte("changed\n"), 0o600)
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "x"}); !r.IsError || !strings.Contains(r.Content, "re-read") {
		t.Fatal(r.Content)
	}
	run(t, readTool{}, env, map[string]any{"path": "f.txt"})
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "new\n"}); r.IsError {
		t.Fatal(r.Content)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatal("mode must be preserved")
	}
	// a second write right after our own write needs no re-read
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "again\n"}); r.IsError {
		t.Fatal(r.Content)
	}
}

func TestWriteOutsideJailRefusedBeforeMkdir(t *testing.T) {
	env, root := testEnv(t)
	r := run(t, writeTool{}, env, map[string]any{"path": "/tmp/../etc/moca-x/y", "content": "x"})
	if !r.IsError {
		t.Fatal("outside jail")
	}
	if _, err := os.Stat(filepath.Join(root, "etc")); err == nil {
		t.Fatal("no dirs may be created")
	}
}

// askRootChecker is a Jail stand-in with one ask-write root: Resolve refuses
// writes under ask, Approvable hands them out for the approval ask
// (permissions.Jail does this with symlink resolution for real; the tools
// only depend on the contract).
type askRootChecker struct{ root, ask string }

func (c askRootChecker) Resolve(path string, write bool) (string, error) {
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(c.root, p)
	}
	p = filepath.Clean(p)
	if write && strings.HasPrefix(p, c.ask+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the workdir; writes are confined to the workdir", path)
	}
	return p, nil
}

func (c askRootChecker) Approvable(path string) (string, bool) {
	p := filepath.Clean(path)
	if strings.HasPrefix(p, c.ask+string(filepath.Separator)) {
		return p, true
	}
	return "", false
}

// A write outside the workdir under an ask-write root (the global prompt
// templates — slash commands the agent writes for the user) needs the user's
// approval: no asker is a refusal, a deny changes nothing, a yes writes; an
// unread existing file fails the read guard before the user is asked.
func TestWriteOutsideJailAsksTheUser(t *testing.T) {
	root := t.TempDir()
	prompts := filepath.Join(t.TempDir(), "prompts")
	os.MkdirAll(prompts, 0o755)
	env := &Env{Root: root, Paths: askRootChecker{root, prompts}, Reads: NewReadTracker()}
	dst := filepath.Join(prompts, "deploy.md")

	// No asker (a -p run): refused like any jail escape.
	r := run(t, writeTool{}, env, map[string]any{"path": dst, "content": "x"})
	if !r.IsError || !strings.Contains(r.Content, "outside the workdir") {
		t.Fatalf("no asker: %q", r.Content)
	}

	// Denied: the refusal names the way out, nothing is written.
	var got Question
	env.Ask = func(_ context.Context, q Question) Answer { got = q; return Deny }
	r = run(t, writeTool{}, env, map[string]any{"path": dst, "content": "x"})
	if !r.IsError || !strings.Contains(r.Content, "did not approve") || !strings.Contains(r.Content, ".moca/prompts") {
		t.Fatalf("denied: %q", r.Content)
	}
	if got.Kind != "write" || got.Subject != dst || got.CanAlways {
		t.Fatalf("question: %+v", got)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("a denied write must not land")
	}

	// Allowed once: written; the recorded write makes a follow-up edit pass
	// the read guard, and it asks again (no allow-always).
	env.Ask = func(_ context.Context, q Question) Answer { got = q; return AllowOnce }
	if r = run(t, writeTool{}, env, map[string]any{"path": dst, "content": "deploy $1\n"}); r.IsError {
		t.Fatalf("allowed: %q", r.Content)
	}
	if b, _ := os.ReadFile(dst); string(b) != "deploy $1\n" {
		t.Fatalf("content: %q", b)
	}
	if r = run(t, editTool{}, env, map[string]any{"path": dst, "old_string": "deploy $1", "new_string": "ship $1"}); r.IsError || got.Kind != "write" {
		t.Fatalf("edit: %q (%+v)", r.Content, got)
	}
	if b, _ := os.ReadFile(dst); string(b) != "ship $1\n" {
		t.Fatalf("edited: %q", b)
	}

	// An unread existing file is refused by the read guard before the user is
	// bothered with a prompt.
	other := filepath.Join(prompts, "other.md")
	os.WriteFile(other, []byte("old\n"), 0o644)
	asked := false
	env.Ask = func(context.Context, Question) Answer { asked = true; return AllowOnce }
	if r := run(t, editTool{}, env, map[string]any{"path": other, "old_string": "old", "new_string": "new"}); !r.IsError || !strings.Contains(r.Content, "read it first") {
		t.Fatalf("guard first: %q", r.Content)
	}
	if asked {
		t.Fatal("no approval may be asked for a write the guard refuses")
	}
}

// fileAskChecker mirrors permissions.Jail's file-root semantics for one
// exact file: writes there refuse and Approvable hands that file — and only
// that file — out for the approval ask (a sibling like auth.json never is).
type fileAskChecker struct{ root, file string }

func (c fileAskChecker) Resolve(path string, write bool) (string, error) {
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(c.root, p)
	}
	p = filepath.Clean(p)
	inRoot := p == c.root || strings.HasPrefix(p, c.root+string(filepath.Separator))
	if !inRoot && p != filepath.Clean(c.file) {
		return "", fmt.Errorf("%s is outside the workdir", path)
	}
	if write && !inRoot {
		return "", fmt.Errorf("%s is outside the workdir; writes are confined to the workdir", path)
	}
	return p, nil
}

func (c fileAskChecker) Approvable(path string) (string, bool) {
	p := filepath.Clean(path)
	if p == filepath.Clean(c.file) {
		return p, true
	}
	return "", false
}

// The user config file joins the ask-write roots (self-configure): a write
// asks with Kind "write" and never allow-always; allowing writes once, and
// the next edit asks again. auth.json beside it can never be approved.
func TestConfigFileWriteAsksEveryTime(t *testing.T) {
	root := t.TempDir()
	cfgDir := t.TempDir()
	cfg := filepath.Join(cfgDir, "config.jsonc")
	if err := os.WriteFile(cfg, []byte("{\n  \"model\": \"a/b\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := &Env{Root: root, Paths: fileAskChecker{root, cfg}, Reads: NewReadTracker()}

	// The real flow reads first (reads are allowed), then the edit asks.
	if r := run(t, readTool{}, env, map[string]any{"path": cfg}); r.IsError {
		t.Fatalf("read: %q", r.Content)
	}
	// No asker (a -p run): refused like any jail escape.
	r := run(t, writeTool{}, env, map[string]any{"path": cfg, "content": "x"})
	if !r.IsError || !strings.Contains(r.Content, "outside the workdir") {
		t.Fatalf("no asker: %q", r.Content)
	}
	var got Question
	env.Ask = func(_ context.Context, q Question) Answer { got = q; return Deny }
	r = run(t, editTool{}, env, map[string]any{"path": cfg, "old_string": "\"a/b\"", "new_string": "\"a/c\""})
	if !r.IsError || got.Kind != "write" || got.Subject != cfg || got.CanAlways {
		t.Fatalf("denied edit: %q (%+v)", r.Content, got)
	}
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "\"a/b\"") {
		t.Fatalf("denied must not change the file: %q", b)
	}
	// Allowed once: written; the next edit asks again (nothing persists).
	env.Ask = func(_ context.Context, q Question) Answer { got = q; return AllowOnce }
	if r = run(t, editTool{}, env, map[string]any{"path": cfg, "old_string": "\"a/b\"", "new_string": "\"a/c\""}); r.IsError {
		t.Fatalf("allowed edit: %q", r.Content)
	}
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "\"a/c\"") {
		t.Fatalf("content: %q", b)
	}
	if r = run(t, editTool{}, env, map[string]any{"path": cfg, "old_string": "\"a/c\"", "new_string": "\"a/d\""}); r.IsError || got.Subject != cfg || got.CanAlways {
		t.Fatalf("second edit must re-ask: %q (%+v)", r.Content, got)
	}
	// auth.json beside it: refused without a question, for reads and writes.
	auth := filepath.Join(cfgDir, "auth.json")
	if err := os.WriteFile(auth, []byte("{\"apiKeys\":{}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	asked := false
	env.Ask = func(context.Context, Question) Answer { asked = true; return AllowOnce }
	if r := run(t, readTool{}, env, map[string]any{"path": auth}); !r.IsError {
		t.Fatalf("auth.json must not be readable: %q", r.Content)
	}
	if r := run(t, writeTool{}, env, map[string]any{"path": auth, "content": "x"}); !r.IsError || asked {
		t.Fatalf("auth.json must be refused without a question: %q asked=%v", r.Content, asked)
	}
}

func TestEditTool(t *testing.T) {
	env, root := testEnv(t)
	p := filepath.Join(root, "m.go")
	os.WriteFile(p, []byte("package m\n\nfunc A() int {\n\treturn 1\n}\n"), 0o644)
	if r := run(t, editTool{}, env, map[string]any{"path": "m.go", "old_string": "return 1", "new_string": "return 2"}); !r.IsError {
		t.Fatal("edit needs prior read")
	}
	run(t, readTool{}, env, map[string]any{"path": "m.go"})
	r := run(t, editTool{}, env, map[string]any{"path": "m.go", "old_string": "return 1", "new_string": "return 2"})
	if r.IsError || !strings.Contains(r.Content, "-\treturn 1") || !strings.Contains(r.Content, "+\treturn 2") ||
		!strings.Contains(r.Content, "changed lines 4-4; file now 5 lines") || r.Summary != "m.go [+1 −1]" {
		t.Fatalf("%q / %q", r.Content, r.Summary)
	}
	r = run(t, editTool{}, env, map[string]any{"path": "m.go", "old_string": "zzz", "new_string": "y"})
	if !r.IsError {
		t.Fatal("structured error")
	}
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), "return 2") {
		t.Fatal("failed edit must not write")
	}
	if r := run(t, editTool{}, env, map[string]any{"path": "nope.go", "old_string": "a", "new_string": "b"}); !r.IsError {
		t.Fatal("edit on missing file")
	}
}

// TestWriteEditTrackRunChanges: a successful write/edit marks the run as
// changed and, for code files, unverified (the verification nudge, C1); a
// docs file changes the run but needs no build.
func TestWriteEditTrackRunChanges(t *testing.T) {
	env, root := testEnv(t)
	if r := run(t, writeTool{}, env, map[string]any{"path": "NOTES.md", "content": "x\n"}); r.IsError {
		t.Fatal(r.Content)
	}
	if !env.Edited || env.Unverified != "" {
		t.Fatalf("docs write: edited=%v unverified=%q", env.Edited, env.Unverified)
	}
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	run(t, readTool{}, env, map[string]any{"path": "a.go"})
	if r := run(t, editTool{}, env, map[string]any{"path": "a.go", "old_string": "package a", "new_string": "package b"}); r.IsError {
		t.Fatal(r.Content)
	}
	if env.Unverified != "a.go" {
		t.Fatalf("code edit: unverified=%q", env.Unverified)
	}
}

// TestSessionAllowCoversAPromptsFileButNeverTheConfig: an ask-write question
// under the global prompts root is an ordinary one — a session allow sticks, so
// the same file is written again without a prompt. moca's own config file is
// excluded: it holds the permission policy, so it keeps asking and a session
// answer to it is downgraded to once.
func TestSessionAllowCoversAPromptsFileButNeverTheConfig(t *testing.T) {
	root := t.TempDir()
	prompts := filepath.Join(t.TempDir(), "prompts")
	os.MkdirAll(prompts, 0o755)
	cfgDir := t.TempDir()
	cfg := filepath.Join(cfgDir, "config.jsonc")
	if err := os.WriteFile(cfg, []byte("{\n  \"model\": \"a/b\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	asked := 0
	ask := func(context.Context, Question) Answer { asked++; return AllowSession }
	env := &Env{Root: root, Paths: askRootChecker{root, prompts}, Reads: NewReadTracker(),
		Ask: RememberSession(ask), ConfigFile: cfg}
	dst := filepath.Join(prompts, "deploy.md")
	if r := run(t, writeTool{}, env, map[string]any{"path": dst, "content": "one\n"}); r.IsError {
		t.Fatalf("first write: %q", r.Content)
	}
	if r := run(t, writeTool{}, env, map[string]any{"path": dst, "content": "two\n"}); r.IsError {
		t.Fatalf("second write: %q", r.Content)
	}
	if asked != 1 {
		t.Fatalf("a session allow must cover the file: asked %d times", asked)
	}
	if b, _ := os.ReadFile(dst); string(b) != "two\n" {
		t.Fatalf("content: %q", b)
	}

	// The config file: asked again every time, never remembered.
	cfgEnv := &Env{Root: root, Paths: fileAskChecker{root, cfg}, Reads: NewReadTracker(),
		Ask: RememberSession(ask), ConfigFile: cfg}
	asked = 0
	if r := run(t, readTool{}, cfgEnv, map[string]any{"path": cfg}); r.IsError {
		t.Fatalf("read: %q", r.Content)
	}
	if r := run(t, writeTool{}, cfgEnv, map[string]any{"path": cfg, "content": "{\"model\":\"a/c\"}\n"}); r.IsError {
		t.Fatalf("config write: %q", r.Content)
	}
	if r := run(t, writeTool{}, cfgEnv, map[string]any{"path": cfg, "content": "{\"model\":\"a/d\"}\n"}); r.IsError {
		t.Fatalf("second config write: %q", r.Content)
	}
	if asked != 2 {
		t.Fatalf("the config file must ask every time, asked %d times", asked)
	}
}

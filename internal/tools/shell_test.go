//go:build !windows

package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCmds struct {
	need, every []string
	err         error
	allowed     []string
}

func (f *fakeCmds) Check(string) ([]string, []string, error) { return f.need, f.every, f.err }
func (f *fakeCmds) Allow(n string)                           { f.allowed = append(f.allowed, n) }

func TestShellToolApprovals(t *testing.T) {
	env, _ := testEnv(t)
	env.ShellEnv = os.Environ()
	cmds := &fakeCmds{need: []string{"python"}}
	env.Commands = cmds
	var asked []Question
	env.Ask = func(_ context.Context, q Question) Answer { asked = append(asked, q); return AllowAlways }
	r := run(t, shellTool{}, env, map[string]any{"command": "echo hi"})
	if r.IsError || !strings.Contains(r.Content, "hi") || !strings.Contains(r.Content, "[exit 0]") {
		t.Fatal(r.Content)
	}
	if len(asked) != 1 || !asked[0].CanAlways || len(cmds.allowed) != 1 {
		t.Fatalf("asked %+v allowed %v", asked, cmds.allowed)
	}
	cmds.need, cmds.every = nil, []string{"rm"}
	env.Ask = func(context.Context, Question) Answer { return Deny }
	r = run(t, shellTool{}, env, map[string]any{"command": "rm x"})
	if !r.IsError || !strings.Contains(r.Content, "rm") || !strings.Contains(r.Content, "refused") {
		t.Fatal(r.Content)
	}
	env.Ask = nil
	if r := run(t, shellTool{}, env, map[string]any{"command": "rm x"}); !r.IsError {
		t.Fatal("no asker = deny (one-shot mode)")
	}
	if r := run(t, shellTool{}, env, map[string]any{"command": "x", "timeout": 301}); !r.IsError {
		t.Fatal("timeout max 300")
	}
}

func TestShellToolRunsAtRoot(t *testing.T) {
	env, root := testEnv(t)
	env.ShellEnv = os.Environ()
	env.Commands = &fakeCmds{}
	os.Mkdir(root+"/sub", 0o755)
	run(t, shellTool{}, env, map[string]any{"command": "cd sub"})
	r := run(t, shellTool{}, env, map[string]any{"command": "pwd -P"})
	if !strings.HasPrefix(strings.TrimSpace(r.Content), root+"\n") && !strings.Contains(r.Content, root+"\n[exit") {
		t.Fatalf("stateless: every call starts at the root, got %q", r.Content)
	}
}

func TestTestFailureHint(t *testing.T) {
	cases := []struct {
		name, cmd, out string
		exit           int
		timed          bool
		want           bool
	}{
		{"go test fails", "go test ./...", "--- FAIL: TestX\nFAIL", 1, false, true},
		{"piped run masks the exit code", "rtk test -- go test ./... 2>&1 | head -50", "[FAIL] FAILURES:\n  --- FAIL: TestTotals", 0, false, true},
		{"passing run", "go test ./...", "ok  	example.com/x	0.002s", 0, false, false},
		{"pytest fails", "pytest -q", "1 failed, 2 passed", 1, false, true},
		{"empty output failure", "go test ./...", "", 1, false, true},
		{"non-test failure", "make build", "error: boom", 1, false, false},
		{"timeout", "go test ./...", "partial", 0, true, false},
	}
	for _, c := range cases {
		if got := testFailureHint(c.cmd, c.out, c.exit, c.timed) != ""; got != c.want {
			t.Errorf("%s: hint=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestShellToolHintsFailingTestRun(t *testing.T) {
	env, root := testEnv(t)
	env.Commands = &fakeCmds{}
	env.ShellEnv = os.Environ()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "x_test.go"),
		[]byte("package x\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644)
	r := run(t, shellTool{}, env, map[string]any{"command": "go test ./..."})
	if !r.IsError || !strings.Contains(r.Content, "--- FAIL") ||
		!strings.Contains(r.Content, "[hint: when tests fail, read the failing test file with the read tool") ||
		!strings.Contains(r.Content, "[exit 1]") {
		t.Fatalf("failing test run must carry the read-the-test hint: %q", r.Content)
	}
	os.WriteFile(filepath.Join(root, "x_test.go"),
		[]byte("package x\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n"), 0o644)
	r = run(t, shellTool{}, env, map[string]any{"command": "go test ./..."})
	if r.IsError || strings.Contains(r.Content, "[hint:") {
		t.Fatalf("passing test run must not hint: %q", r.Content)
	}
}

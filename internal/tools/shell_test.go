//go:build !windows

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
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

func TestTestRunFailed(t *testing.T) {
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
		if got := testRunFailed(c.cmd, c.out, c.exit, c.timed); got != c.want {
			t.Errorf("%s: failed=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestTestRunCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"go test ./...", true},
		{"rtk test -- go test ./... 2>&1 | tail -30", true},
		{"pytest -q", true},
		{"ls calc && rtk test -- go test ./calc -v", true},
		{"make build", false},
		{"cat calc/calc.go", false},
	}
	for _, c := range cases {
		if got := testRunCommand(c.cmd); got != c.want {
			t.Errorf("testRunCommand(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestFindTestFile(t *testing.T) {
	cases := []struct{ out, want string }{
		{"--- FAIL: TestTotals (0.00s)\n    calc_test.go:17: Sum([1 2 3]) = 5, want 6\nFAIL", "calc_test.go"},
		{"[FAIL] FAILURES:\n  --- FAIL: TestTotals (0.00s)", ""},
		{"ok  	example.com/x	0.002s", ""},
		{"pkg/deep/foo_test.go:1: boom\nother_test.go:2: x", "pkg/deep/foo_test.go"},
	}
	for _, c := range cases {
		if got := findTestFile(c.out); got != c.want {
			t.Errorf("findTestFile(%q) = %q, want %q", c.out, got, c.want)
		}
	}
}

func TestInvestigationHint(t *testing.T) {
	cases := []struct {
		name string
		env  Env
		want string
	}{
		{"unread failing test", Env{TestFailed: true, FailingTest: "calc_test.go"},
			"\n[hint: tests failed: (1) read the failing test file (calc_test.go) with the read tool, (2) locate the cause with the search tool, (3) only then edit]"},
		{"read but no search", Env{TestFailed: true, TestSeen: true},
			"\n[hint: locate the cause with the search tool before editing]"},
		{"read and searched", Env{TestFailed: true, TestSeen: true, Searched: true}, ""},
		{"no failing run", Env{}, ""},
		{"green cleared", Env{TestSeen: true}, ""},
	}
	for _, c := range cases {
		if got := investigationHint(&c.env); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestShellToolArmsAndClearsInvestigationState(t *testing.T) {
	env, root := testEnv(t)
	env.Commands = &fakeCmds{}
	env.ShellEnv = os.Environ()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "x_test.go"),
		[]byte("package x\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644)
	run(t, shellTool{}, env, map[string]any{"command": "go test ./..."})
	if !env.TestFailed || env.FailingTest != "x_test.go" {
		t.Fatalf("failing run must arm the state: failed=%v file=%q", env.TestFailed, env.FailingTest)
	}
	run(t, shellTool{}, env, map[string]any{"command": "echo hi"})
	if !env.TestFailed || env.FailingTest != "x_test.go" {
		t.Fatalf("a non-test command must not touch the state: failed=%v file=%q", env.TestFailed, env.FailingTest)
	}
	os.WriteFile(filepath.Join(root, "x_test.go"),
		[]byte("package x\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n"), 0o644)
	run(t, shellTool{}, env, map[string]any{"command": "go test ./..."})
	if env.TestFailed || env.FailingTest != "" {
		t.Fatalf("a green run must clear the state: failed=%v file=%q", env.TestFailed, env.FailingTest)
	}
}

func TestInvestigationRefusal(t *testing.T) {
	cases := []struct {
		name string
		env  Env
		want string
	}{
		{"unread failing test", Env{TestFailed: true, FailingTest: "calc_test.go"},
			"refused: tests are failing — read the failing test file (calc_test.go) with the read tool and locate the cause with the search tool before editing"},
		{"read but no search", Env{TestFailed: true, TestSeen: true},
			"refused: tests are failing — locate the cause with the search tool before editing"},
		{"complete", Env{TestFailed: true, TestSeen: true, Searched: true}, ""},
		{"no failing run", Env{}, ""},
	}
	for _, c := range cases {
		if got := investigationRefusal(&c.env); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestEditRefusesIncompleteInvestigation(t *testing.T) {
	env, root := testEnv(t)
	os.WriteFile(filepath.Join(root, "f.go"), []byte("package x\n\nvar V = 1\n"), 0o644)
	run(t, readTool{}, env, map[string]any{"path": "f.go"})
	env.TestFailed, env.FailingTest = true, "x_test.go"
	r := run(t, editTool{}, env, map[string]any{"path": "f.go", "old_string": "var V = 1", "new_string": "var V = 2"})
	if !r.IsError || !strings.Contains(r.Content, "refused") || !strings.Contains(r.Content, "read the failing test file (x_test.go)") {
		t.Fatalf("edit must be refused while the failing test is unread: %q", r.Content)
	}
	env.TestSeen = true
	r = run(t, editTool{}, env, map[string]any{"path": "f.go", "old_string": "var V = 1", "new_string": "var V = 2"})
	if !r.IsError || !strings.Contains(r.Content, "locate the cause with the search tool") {
		t.Fatalf("edit must be refused while the search is missing: %q", r.Content)
	}
	env.Searched = true
	r = run(t, editTool{}, env, map[string]any{"path": "f.go", "old_string": "var V = 1", "new_string": "var V = 2"})
	if r.IsError {
		t.Fatalf("edit must pass once the protocol is done: %q", r.Content)
	}
}

func runReg(t *testing.T, reg *Registry, env *Env, name string, args any) Result {
	t.Helper()
	b, _ := json.Marshal(args)
	return reg.Run(context.Background(), env, llm.ToolCall{Name: name, Input: b})
}

func TestRegistryInvestigationBanner(t *testing.T) {
	env, root := testEnv(t)
	env.Commands = &fakeCmds{}
	env.ShellEnv = os.Environ()
	reg := NewRegistry(shellTool{}, readTool{}, searchTool{})
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "y.go"), []byte("package x\n\nfunc Y() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "x_test.go"),
		[]byte("package x\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644)
	// 1. the failing run result carries the banner naming the file
	r := runReg(t, reg, env, "shell", map[string]any{"command": "go test ./..."})
	if !strings.Contains(r.Content, "[hint: tests failed: (1) read the failing test file (x_test.go) with the read tool") {
		t.Fatalf("failing run must carry the banner: %q", r.Content)
	}
	// 2. shell-outs (cat/sed) repeat it — no tool path escapes the protocol
	r = runReg(t, reg, env, "shell", map[string]any{"command": "cat y.go"})
	if !strings.Contains(r.Content, "[hint: tests failed: (1) read the failing test file (x_test.go)") {
		t.Fatalf("shell-out results must repeat the banner: %q", r.Content)
	}
	// 3. reading the test moves the banner to the search step
	r = runReg(t, reg, env, "read", map[string]any{"path": "x_test.go"})
	if !env.TestSeen || env.FailingTest != "" || !strings.Contains(r.Content, "[hint: locate the cause with the search tool before editing]") {
		t.Fatalf("test read must advance the banner: %q seen=%v file=%q", r.Content, env.TestSeen, env.FailingTest)
	}
	// 4. the search clears it
	r = runReg(t, reg, env, "search", map[string]any{"pattern": "func Y"})
	if r.IsError || !env.Searched || strings.Contains(r.Content, "[hint:") {
		t.Fatalf("search must clear the banner: %q", r.Content)
	}
	// 5. later reads stay silent
	r = runReg(t, reg, env, "read", map[string]any{"path": "y.go"})
	if strings.Contains(r.Content, "[hint:") {
		t.Fatalf("banner must stop once the steps are done: %q", r.Content)
	}
	// 6. a green run clears the failing state
	os.WriteFile(filepath.Join(root, "x_test.go"),
		[]byte("package x\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n"), 0o644)
	r = runReg(t, reg, env, "shell", map[string]any{"command": "go test ./..."})
	if r.IsError || env.TestFailed || strings.Contains(r.Content, "[hint:") {
		t.Fatalf("green run must clear the state: %q failed=%v", r.Content, env.TestFailed)
	}
}

// TestShellOverflowSpills: output past the 12K cap keeps head and tail for
// the model and saves the whole output to a readable overflow file.
func TestShellOverflowSpills(t *testing.T) {
	env, _ := testEnv(t)
	env.ShellEnv = os.Environ()
	env.Commands = &fakeCmds{}
	env.SpillDir, env.SpillPrefix = t.TempDir(), "s-"
	r := run(t, shellTool{}, env, map[string]any{"command": "seq 1 20000"})
	if len(r.Content) > shellMaxOutput+600 || !strings.Contains(r.Content, "lines omitted") || !strings.HasSuffix(r.Content, "[exit 0]") {
		t.Fatalf("cut output: %d chars, tail %q", len(r.Content), r.Content[len(r.Content)-200:])
	}
	b, err := os.ReadFile(spilledPath(t, r.Content))
	if err != nil || !strings.HasPrefix(string(b), "1\n2\n") || !strings.HasSuffix(string(b), "19999\n20000\n") {
		t.Fatalf("spilled: %v %d bytes", err, len(b))
	}
	// Small output: no spill.
	r = run(t, shellTool{}, env, map[string]any{"command": "echo hi"})
	if strings.Contains(r.Content, "saved to") {
		t.Fatalf("small output spilled: %q", r.Content)
	}
}

// TestShellOwnBreakageDoesNotArmInvestigation: the failing-test protocol is
// for failures the run found, not for the run's own work in progress — once
// the run has changed files, a red test run must not arm the edit refusal
// (baseline evals: a self-inflicted build break locked edit out for 4 calls).
func TestShellOwnBreakageDoesNotArmInvestigation(t *testing.T) {
	env, root := testEnv(t)
	env.Commands = &fakeCmds{}
	env.ShellEnv = os.Environ()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(root, "x_test.go"),
		[]byte("package x\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n"), 0o644)
	env.Edited = true
	run(t, shellTool{}, env, map[string]any{"command": "go test ./..."})
	if env.TestFailed || env.FailingTest != "" || investigationRefusal(env) != "" {
		t.Fatalf("own breakage armed the protocol: failed=%v file=%q", env.TestFailed, env.FailingTest)
	}
	env.BeginRun()
	if env.Edited {
		t.Fatal("BeginRun must reset Edited")
	}
	run(t, shellTool{}, env, map[string]any{"command": "go test ./..."})
	if !env.TestFailed {
		t.Fatal("a failure found before any change must arm the protocol")
	}
}

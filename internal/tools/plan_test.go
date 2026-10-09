package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanModeShellRunsUnderNormalRules(t *testing.T) {
	env, _ := testEnv(t)
	env.ShellEnv = os.Environ()
	env.Plan = true
	env.Commands = &fakeCmds{} // allowlisted: runs
	r := run(t, shellTool{}, env, map[string]any{"command": "echo plan-ok"})
	if r.IsError || !strings.Contains(r.Content, "plan-ok") || !strings.Contains(r.Content, "[exit 0]") {
		t.Fatalf("shell must run in plan mode: %+v", r)
	}
	// The normal ladder still applies: a non-allowlisted command asks, and a
	// denial refuses.
	env.Commands = &fakeCmds{need: []string{"python"}}
	env.Ask = func(context.Context, Question) Answer { return Deny }
	r = run(t, shellTool{}, env, map[string]any{"command": "python -c 1"})
	if !r.IsError || !strings.Contains(r.Content, "did not approve") {
		t.Fatalf("plan mode must keep the normal ask ladder: %+v", r)
	}
}

func TestPlanModeWebStaysAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<h1>Plan research</h1>")
	}))
	defer srv.Close()
	env, _ := testEnv(t)
	env.Plan = true
	r := webTool{}.Run(context.Background(), env, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`"}`))
	if r.IsError || !strings.Contains(r.Content, "Plan research") {
		t.Fatalf("web must stay available in plan mode: %+v", r)
	}
}

func TestPlanModeWriteZone(t *testing.T) {
	env, root := testEnv(t)
	env.Plan = true

	// Outside the zone: refused (even a markdown file at the workdir root).
	r := run(t, writeTool{}, env, map[string]any{"path": "README.md", "content": "# x"})
	if !r.IsError || !strings.Contains(r.Content, "docs/plans/*.md") {
		t.Fatalf("write outside the zone: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); err == nil {
		t.Fatal("refused write must not land")
	}
	// A non-markdown file under the zone: refused.
	r = run(t, writeTool{}, env, map[string]any{"path": "docs/plans/x.txt", "content": "x"})
	if !r.IsError {
		t.Fatalf("non-markdown plan file: %+v", r)
	}
	// The zone: allowed, parents created, PlanWrote set.
	r = run(t, writeTool{}, env, map[string]any{"path": "docs/plans/feature.md", "content": "# Plan\n"})
	if r.IsError || !env.PlanWrote {
		t.Fatalf("plan write: %+v wrote=%v", r, env.PlanWrote)
	}
	if b, err := os.ReadFile(filepath.Join(root, "docs", "plans", "feature.md")); err != nil || string(b) != "# Plan\n" {
		t.Fatalf("plan file: %q %v", b, err)
	}
}

func TestPlanModeEditZone(t *testing.T) {
	env, root := testEnv(t)
	os.MkdirAll(filepath.Join(root, "docs", "plans"), 0o755)
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "p.md"), []byte("step 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "a.go"), []byte("x\n"), 0o644)
	env.Plan = true

	// Edit outside the zone refused.
	if r := run(t, readTool{}, env, map[string]any{"path": "a.go"}); r.IsError {
		t.Fatal(r.Content)
	}
	r := run(t, editTool{}, env, map[string]any{"path": "a.go", "old_string": "x", "new_string": "y"})
	if !r.IsError || !strings.Contains(r.Content, "docs/plans/*.md") {
		t.Fatalf("edit outside the zone: %+v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "a.go")); string(b) != "x\n" {
		t.Fatalf("refused edit must not land: %q", b)
	}
	// Edit inside the zone allowed (after a tracked read) + PlanWrote.
	if r := run(t, readTool{}, env, map[string]any{"path": "docs/plans/p.md"}); r.IsError {
		t.Fatal(r.Content)
	}
	r = run(t, editTool{}, env, map[string]any{"path": "docs/plans/p.md", "old_string": "step 1", "new_string": "step 1 updated"})
	if r.IsError || !env.PlanWrote {
		t.Fatalf("plan edit: %+v wrote=%v", r, env.PlanWrote)
	}
}

// TestPlanModeAskWriteRootsStayClosed: in plan mode neither write nor edit
// may reach the approval ask for an ask-write root (§10: "nothing is asked")
// — the user would approve a write that the plan zone then refuses.
func TestPlanModeAskWriteRootsStayClosed(t *testing.T) {
	root := t.TempDir()
	prompts := filepath.Join(t.TempDir(), "prompts")
	os.MkdirAll(prompts, 0o755)
	dst := filepath.Join(prompts, "deploy.md")
	os.WriteFile(dst, []byte("x\n"), 0o644)
	asked := 0
	env := &Env{Root: root, Paths: askRootChecker{root, prompts}, Reads: NewReadTracker(), Plan: true,
		Ask: func(context.Context, Question) Answer { asked++; return AllowOnce }}
	if r := run(t, readTool{}, env, map[string]any{"path": dst}); r.IsError {
		t.Fatal(r.Content)
	}
	if r := run(t, writeTool{}, env, map[string]any{"path": dst, "content": "y"}); !r.IsError || !strings.Contains(r.Content, "docs/plans/*.md") {
		t.Fatalf("write: %+v", r)
	}
	if r := run(t, editTool{}, env, map[string]any{"path": dst, "old_string": "x", "new_string": "y"}); !r.IsError || !strings.Contains(r.Content, "docs/plans/*.md") {
		t.Fatalf("edit: %+v", r)
	}
	if asked != 0 {
		t.Fatalf("plan mode asked the user %d time(s)", asked)
	}
	if b, _ := os.ReadFile(dst); string(b) != "x\n" {
		t.Fatalf("refused write landed: %q", b)
	}
}

func TestPlanModeReadsAndSearchStayAvailable(t *testing.T) {
	env, root := testEnv(t)
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package main\n"), 0o644)
	env.Plan = true
	if r := run(t, readTool{}, env, map[string]any{"path": "a.go"}); r.IsError {
		t.Fatalf("read in plan mode: %+v", r)
	}
	if r := run(t, lsTool{}, env, map[string]any{}); r.IsError {
		t.Fatalf("ls in plan mode: %+v", r)
	}
	if r := run(t, searchTool{}, env, map[string]any{"pattern": "package"}); r.IsError {
		t.Fatalf("search in plan mode: %+v", r)
	}
}

// TestPlanModeSuppressesInvestigationProtocol: plan runs inspect, they do not
// fix — a failing test run during planning must not arm the fix-loop banner
// or block plan-file edits with it.
func TestPlanModeSuppressesInvestigationProtocol(t *testing.T) {
	env, _ := testEnv(t)
	env.Plan = true
	env.TestFailed, env.FailingTest = true, "calc_test.go"
	if h := investigationHint(env); h != "" {
		t.Fatalf("no hint in plan mode: %q", h)
	}
	if msg := investigationRefusal(env); msg != "" {
		t.Fatalf("no refusal in plan mode: %q", msg)
	}
	env.Plan = false
	if investigationHint(env) == "" || investigationRefusal(env) == "" {
		t.Fatal("the protocol must engage without plan mode")
	}
}

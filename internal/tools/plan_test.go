package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanModeRefusesShell(t *testing.T) {
	env, _ := testEnv(t)
	env.Plan = true
	r := run(t, shellTool{}, env, map[string]any{"command": "ls"})
	if !r.IsError || !strings.Contains(r.Content, "plan mode disables the shell") {
		t.Fatalf("shell must be refused in plan mode: %+v", r)
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

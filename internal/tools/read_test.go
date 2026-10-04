package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rootChecker: minimal PathChecker for tool tests (the real jail is tested in permissions).
type rootChecker struct{ root string }

func (c rootChecker) Resolve(p string, _ bool) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(c.root, p)
	}
	if !strings.HasPrefix(p, c.root) {
		return "", fmt.Errorf("%s is outside the workdir", p)
	}
	return filepath.Clean(p), nil
}

func testEnv(t *testing.T) (*Env, string) {
	t.Helper()
	root := t.TempDir()
	return &Env{Root: root, Paths: rootChecker{root}, Reads: NewReadTracker()}, root
}

func run(t *testing.T, tool Tool, env *Env, args any) Result {
	t.Helper()
	b, _ := json.Marshal(args)
	return tool.Run(context.Background(), env, b)
}

func TestReadWindowAndFooter(t *testing.T) {
	env, root := testEnv(t)
	var sb strings.Builder
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&sb, "line%d\n", i)
	}
	os.WriteFile(filepath.Join(root, "f.txt"), []byte(sb.String()), 0o644)
	r := run(t, readTool{}, env, map[string]any{"path": "f.txt", "offset": 3, "limit": 2})
	if r.IsError || r.Content != "3|line3\n4|line4\n[lines 3-4 of 10 — use offset=5 to continue]" {
		t.Fatalf("%q", r.Content)
	}
	r = run(t, readTool{}, env, map[string]any{"path": "f.txt", "offset": 9})
	if !strings.HasSuffix(r.Content, "[lines 9-10 of 10]") {
		t.Fatalf("%q", r.Content)
	}
}

func TestReadCaps(t *testing.T) {
	env, root := testEnv(t)
	os.WriteFile(filepath.Join(root, "long.txt"), []byte(strings.Repeat("x\n", 2500)), 0o644)
	r := run(t, readTool{}, env, map[string]any{"path": "long.txt"})
	if !strings.Contains(r.Content, "[lines 1-2000 of 2500") {
		t.Fatal("2000-line cap")
	}
	os.WriteFile(filepath.Join(root, "wide.txt"), []byte(strings.Repeat("y", 5000)+"\n"), 0o644)
	r = run(t, readTool{}, env, map[string]any{"path": "wide.txt"})
	if !strings.Contains(r.Content, "[… line truncated]") || len(r.Content) > 2200 {
		t.Fatal("per-line cap")
	}
	os.WriteFile(filepath.Join(root, "chars.txt"), []byte(strings.Repeat(strings.Repeat("z", 1000)+"\n", 100)), 0o644)
	r = run(t, readTool{}, env, map[string]any{"path": "chars.txt"})
	if len(r.Content) > 50_000+200 || !strings.Contains(r.Content, "use offset=") {
		t.Fatalf("50K char cap: %d", len(r.Content))
	}
}

func TestReadBinaryAndDirAndMissing(t *testing.T) {
	env, root := testEnv(t)
	os.WriteFile(filepath.Join(root, "b.png"), append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0), 0o644)
	r := run(t, readTool{}, env, map[string]any{"path": "b.png"})
	if !r.IsError || !strings.Contains(r.Content, "binary") || !strings.Contains(r.Content, "image/png") {
		t.Fatalf("%q", r.Content)
	}
	os.Mkdir(filepath.Join(root, "d"), 0o755)
	if r := run(t, readTool{}, env, map[string]any{"path": "d"}); !r.IsError || !strings.Contains(r.Content, "ls") {
		t.Fatalf("dir → suggest ls: %q", r.Content)
	}
	if r := run(t, readTool{}, env, map[string]any{"path": "nope"}); !r.IsError {
		t.Fatal("missing file")
	}
	if r := run(t, readTool{}, env, map[string]any{"path": "f", "offset": 0}); !r.IsError {
		t.Fatal("offset must be >= 1")
	}
}

func TestReadTracker(t *testing.T) {
	env, root := testEnv(t)
	p := filepath.Join(root, "t.txt")
	os.WriteFile(p, []byte("a\n"), 0o644)
	if err := env.Reads.Check(p); err == nil {
		t.Fatal("unread existing file must fail Check")
	}
	run(t, readTool{}, env, map[string]any{"path": "t.txt"})
	if err := env.Reads.Check(p); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte("b\n"), 0o644)
	if err := env.Reads.Check(p); err == nil || !strings.Contains(err.Error(), "re-read") {
		t.Fatalf("changed on disk: %v", err)
	}
	if err := env.Reads.Check(filepath.Join(root, "new.txt")); err != nil {
		t.Fatal("non-existent file needs no read")
	}
}

func TestReadRefusesHugeFile(t *testing.T) {
	env, root := testEnv(t)
	p := filepath.Join(root, "huge.txt")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(readMaxFileSize + 1); err != nil { // sparse, no disk cost
		t.Fatal(err)
	}
	f.Close()
	r := run(t, readTool{}, env, map[string]any{"path": "huge.txt"})
	if !r.IsError || !strings.Contains(r.Content, "too large") {
		t.Fatalf("%q", r.Content)
	}
}

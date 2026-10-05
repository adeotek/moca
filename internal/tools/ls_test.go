package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLs(t *testing.T) {
	env, root := testEnv(t)
	os.Mkdir(filepath.Join(root, "dir"), 0o755)
	os.WriteFile(filepath.Join(root, "b.go"), nil, 0o644)
	os.WriteFile(filepath.Join(root, ".env"), nil, 0o644)
	os.Symlink("b.go", filepath.Join(root, "link"))
	r := run(t, lsTool{}, env, map[string]any{})
	if r.Content != "b.go\ndir/\nlink@" {
		t.Fatalf("%q", r.Content)
	}
	r = run(t, lsTool{}, env, map[string]any{"path": ".", "hidden": true})
	if r.Content != ".env\nb.go\ndir/\nlink@" {
		t.Fatalf("%q", r.Content)
	}
	if r := run(t, lsTool{}, env, map[string]any{"path": "b.go"}); !r.IsError {
		t.Fatal("ls on a file is an error")
	}
	os.Mkdir(filepath.Join(root, "empty"), 0o755)
	if r := run(t, lsTool{}, env, map[string]any{"path": "empty"}); r.Content != "[empty directory]" {
		t.Fatalf("%q", r.Content)
	}
}

package tools

import (
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

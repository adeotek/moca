package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newSnap(t *testing.T) (*Snapshots, string) {
	t.Helper()
	base := t.TempDir()
	w, _ := Create(filepath.Join(base, "s"), Header{}, "t")
	t.Cleanup(func() { w.Close() })
	return NewSnapshots(w, filepath.Join(base, "blobs"), nil), base
}

func TestSnapshotUndoEditAndCreate(t *testing.T) {
	s, base := newSnap(t)
	f := filepath.Join(base, "f.txt")
	os.WriteFile(f, []byte("v1"), 0o640)
	s.Guard(f, func() error { return os.WriteFile(f, []byte("v2"), 0o644) })
	os.Chmod(f, 0o644) // undo must restore 0640 regardless of the current mode
	n := filepath.Join(base, "new.txt")
	s.Guard(n, func() error { return os.WriteFile(n, []byte("x"), 0o644) })

	if msg, err := s.Undo(); err != nil || !strings.Contains(msg, "new.txt") {
		t.Fatal(msg, err)
	}
	if _, err := os.Stat(n); !os.IsNotExist(err) {
		t.Fatal("undo of a create deletes the file")
	}
	if _, err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f); string(b) != "v1" {
		t.Fatal(string(b))
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode not restored: %v", fi.Mode())
	}
	if _, err := s.Undo(); err == nil {
		t.Fatal("empty stack")
	}
}

func TestUndoRefusesIfChangedSince(t *testing.T) {
	s, base := newSnap(t)
	f := filepath.Join(base, "f.txt")
	os.WriteFile(f, []byte("v1"), 0o644)
	s.Guard(f, func() error { return os.WriteFile(f, []byte("v2"), 0o644) })
	os.WriteFile(f, []byte("user edit"), 0o644)
	if _, err := s.Undo(); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatal(err)
	}
}

func TestGitCleanSkipsSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	s, base := newSnap(t)
	repo := filepath.Join(base, "repo")
	os.Mkdir(repo, 0o755)
	gitc := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(string(out))
		}
	}
	gitc("init", "-q")
	f := filepath.Join(repo, "a.go")
	os.WriteFile(f, []byte("x"), 0o644)
	gitc("add", ".")
	gitc("commit", "-qm", "i")
	s.Guard(f, func() error { return os.WriteFile(f, []byte("y"), 0o644) })
	msg, err := s.Undo()
	if err != nil || !strings.Contains(msg, "git restore -- ") {
		t.Fatal(msg, err)
	}
	if ents, _ := os.ReadDir(filepath.Join(base, "blobs")); len(ents) != 0 {
		t.Fatal("no blob for git-clean files")
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old")
	os.WriteFile(old, nil, 0o600)
	os.Chtimes(old, time.Now().AddDate(0, 0, -40), time.Now().AddDate(0, 0, -40))
	os.WriteFile(filepath.Join(dir, "new"), nil, 0o600)
	Prune(dir, 30)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old blob pruned")
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatal("fresh blob kept")
	}
}

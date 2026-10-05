package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFind(t *testing.T) {
	dir := t.TempDir()
	w1, _ := Create(dir, Header{Workdir: "/a"}, "one")
	w1.Close()
	time.Sleep(20 * time.Millisecond)
	w2, _ := Create(dir, Header{Workdir: "/b"}, "two")
	w2.Close()
	if p, err := Find(dir, "last"); err != nil || p != w2.Path() {
		t.Fatal(p, err)
	}
	if p, err := Find(dir, w1.ID8()); err != nil || p != w1.Path() {
		t.Fatal(p, err)
	}
	if _, err := Find(dir, "deadbeef"); err == nil || !strings.Contains(err.Error(), w2.ID8()) {
		t.Fatal("not-found lists recent ids:", err)
	}
	if _, err := Find(dir, "../x"); err == nil {
		t.Fatal("ref must be 8 hex chars")
	}
}

func TestFindForWorkdirViaSymlink(t *testing.T) {
	dir, base := t.TempDir(), t.TempDir()
	real := filepath.Join(base, "real")
	os.Mkdir(real, 0o755)
	canon, _ := filepath.EvalSymlinks(real)
	link := filepath.Join(base, "link")
	os.Symlink(real, link)
	w, _ := Create(dir, Header{Workdir: canon}, "x")
	w.Close()
	Create(dir, Header{Workdir: "/elsewhere"}, "y")
	if p, err := FindForWorkdir(dir, link); err != nil || p != w.Path() {
		t.Fatal(p, err)
	}
}

func TestFindNoSessions(t *testing.T) {
	dir := t.TempDir()
	if _, err := Find(dir, "last"); err == nil {
		t.Fatal("empty dir")
	}
	if _, err := FindForWorkdir(dir, dir); err == nil || !strings.Contains(err.Error(), "no session for") {
		t.Fatal("no session for this workdir:", err)
	}
}

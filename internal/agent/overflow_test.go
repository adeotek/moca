package agent

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSpillDirsAreRandom: the process dir is an unguessable 16-hex name under
// <data>/overflow, different on every call.
func TestSpillDirsAreRandom(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	d1, err := newSpillDir()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := newSpillDir()
	if err != nil {
		t.Fatal(err)
	}
	if d1 == d2 {
		t.Fatalf("spill dirs must differ: %q", d1)
	}
	base := overflowDir()
	for _, d := range []string{d1, d2} {
		if filepath.Dir(d) != base {
			t.Fatalf("%s not under %s", d, base)
		}
		name := filepath.Base(d)
		if len(name) != 16 {
			t.Fatalf("dir name %q: want 16 hex chars, got %d", name, len(name))
		}
		if _, err := hex.DecodeString(name); err != nil {
			t.Fatalf("dir name %q is not hex: %v", name, err)
		}
	}
}

// TestPruneOverflow: entries older than the retention go (per-process dirs —
// non-empty included — and flat files of the first rev-21 layout), fresh ones
// stay, and days<=0 keeps everything.
func TestPruneOverflow(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	area := overflowDir()
	old := time.Now().AddDate(0, 0, -40)
	mk := func(rel string, mtime time.Time) string {
		p := filepath.Join(area, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldDir := mk("aaaaaaaaaaaaaaaa/s-1.txt", old)
	os.Chtimes(filepath.Dir(oldDir), old, old) // dir mtime governs a dir entry
	oldFlat := mk("bbbbbbbb-s-1.txt", old)     // legacy flat layout
	fresh := mk("cccccccccccccccc/s-1.txt", time.Now())

	pruneOverflow(30)
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("stale per-process dir must be pruned")
	}
	if _, err := os.Stat(oldFlat); !os.IsNotExist(err) {
		t.Fatal("stale flat file must be pruned")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh entry must stay: %v", err)
	}
	keep := mk("dddddddddddddddd/s-1.txt", old)
	os.Chtimes(filepath.Dir(keep), old, old)
	pruneOverflow(0)
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("days<=0 must keep everything: %v", err)
	}
}

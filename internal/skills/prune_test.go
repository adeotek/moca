package skills

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

var pruneFS = fstest.MapFS{"builtin/self-configure/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: self-configure\ndescription: x\n---\n")}}

// mkTree writes files (relative path → content) under root.
func mkTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// pruneFixture extracts the current built-ins and plants leftovers of every
// kind next to them: an older content-hash directory, a legacy
// version-keyed one (the 0.0.0-dev layout) and a stray file.
func pruneFixture(t *testing.T) (data, base, cur string) {
	t.Helper()
	data = t.TempDir()
	cur, err := extractBuiltins(pruneFS, data)
	if err != nil {
		t.Fatal(err)
	}
	base = filepath.Dir(cur)
	mkTree(t, base, map[string]string{
		"aaaaaaaaaaaa/rtk/SKILL.md": "old rtk body",
		"0.0.0-dev/rtk/SKILL.md":    "legacy",
		"notes.txt":                 "stray",
	})
	return data, base, cur
}

// Everything but this binary's directory goes: older content hashes, the
// legacy version-keyed layout, stray files. The current directory — the one
// every running moca of this version reads — is untouched.
func TestPruneBuiltinsKeepsOnlyTheCurrentDir(t *testing.T) {
	data, base, cur := pruneFixture(t)
	got, err := pruneBuiltins(pruneFS, data, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	var bytes int64
	for _, e := range got {
		names = append(names, filepath.Base(e.Path))
		bytes += e.Bytes
	}
	if want := []string{"0.0.0-dev", "aaaaaaaaaaaa", "notes.txt"}; len(names) != 3 || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		t.Fatalf("pruned %v, want %v", names, want)
	}
	if bytes != int64(len("old rtk body")+len("legacy")+len("stray")) {
		t.Fatalf("size accounting: %d bytes", bytes)
	}
	for _, gone := range []string{"aaaaaaaaaaaa", "0.0.0-dev", "notes.txt"} {
		if exists(filepath.Join(base, gone)) {
			t.Errorf("%s survived the prune", gone)
		}
	}
	if !exists(filepath.Join(cur, "self-configure", "SKILL.md")) {
		t.Fatal("the current built-in directory was damaged")
	}
}

// --dry-run reports the same list and removes nothing.
func TestPruneBuiltinsDryRunRemovesNothing(t *testing.T) {
	data, base, _ := pruneFixture(t)
	got, err := pruneBuiltins(pruneFS, data, true, time.Now())
	if err != nil || len(got) != 3 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	for _, kept := range []string{"aaaaaaaaaaaa", "0.0.0-dev", "notes.txt"} {
		if !exists(filepath.Join(base, kept)) {
			t.Errorf("dry run removed %s", kept)
		}
	}
}

// A `.tmp-*` directory is another moca's extraction in flight until it is
// old enough to be the debris of a crash.
func TestPruneBuiltinsSparesFreshTempDirs(t *testing.T) {
	data, base, _ := pruneFixture(t)
	now := time.Now()
	mkTree(t, base, map[string]string{".tmp-fresh/x": "1", ".tmp-stale/x": "2"})
	old := now.Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(base, ".tmp-stale"), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := pruneBuiltins(pruneFS, data, false, now); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(base, ".tmp-fresh")) {
		t.Error("a fresh temp directory (an extraction in flight) was removed")
	}
	if exists(filepath.Join(base, ".tmp-stale")) {
		t.Error("a crash-orphaned temp directory survived")
	}
}

// Nothing extracted yet (fresh install, or right after an update): the
// leftovers still go, the current directory is not conjured up, and a data
// dir without builtin-skills is not an error.
func TestPruneBuiltinsToleratesMissingDirs(t *testing.T) {
	if got, err := pruneBuiltins(pruneFS, t.TempDir(), false, time.Now()); err != nil || len(got) != 0 {
		t.Fatalf("no builtin-skills dir: got=%v err=%v", got, err)
	}
	data := t.TempDir()
	base := filepath.Join(data, "builtin-skills")
	mkTree(t, base, map[string]string{"bbbbbbbbbbbb/x": "1"})
	got, err := pruneBuiltins(pruneFS, data, false, time.Now())
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if ents, _ := os.ReadDir(base); len(ents) != 0 {
		t.Fatalf("prune must not create the current directory: %v", ents)
	}
}

// Removal never follows a symlink out of the tree: only the link goes.
func TestPruneBuiltinsDoesNotFollowSymlinks(t *testing.T) {
	data, base, _ := pruneFixture(t)
	outside := t.TempDir()
	mkTree(t, outside, map[string]string{"precious.txt": "keep me"})
	if err := os.Symlink(outside, filepath.Join(base, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := pruneBuiltins(pruneFS, data, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(base, "link")) {
		t.Error("the symlink itself should be pruned")
	}
	if !exists(filepath.Join(outside, "precious.txt")) {
		t.Fatal("prune followed a symlink and deleted its target")
	}
}

// With the real embedded set: whatever ExtractBuiltins produced is current.
func TestPruneBuiltinsKeepsWhatExtractBuiltinsMade(t *testing.T) {
	data := t.TempDir()
	cur, err := ExtractBuiltins(data)
	if err != nil {
		t.Fatal(err)
	}
	mkTree(t, filepath.Dir(cur), map[string]string{"cccccccccccc/x": "old"})
	got, err := PruneBuiltins(data, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if !exists(filepath.Join(cur, "create-command", "SKILL.md")) {
		t.Fatal("the current extraction was pruned")
	}
}

package permissions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (root, outside, ro string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "work")
	outside = filepath.Join(base, "outside")
	ro = filepath.Join(base, "skills")
	for _, d := range []string{root, outside, ro, filepath.Join(root, "sub")} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o644)
	os.WriteFile(filepath.Join(ro, "SKILL.md"), []byte("k"), 0o644)
	os.Symlink(outside, filepath.Join(root, "escape"))
	os.Symlink(filepath.Join(root, "sub"), filepath.Join(root, "inner"))
	return
}

func TestJail(t *testing.T) {
	root, outside, ro := setup(t)
	j, err := NewJail(root, []string{ro})
	if err != nil {
		t.Fatal(err)
	}
	ok := []struct {
		p     string
		write bool
	}{
		{"a.go", true}, {"sub/new/deep.go", true}, {"inner/x", true},
		{filepath.Join(root, "sub"), false}, {filepath.Join(ro, "SKILL.md"), false},
	}
	for _, c := range ok {
		if _, err := j.Resolve(c.p, c.write); err != nil {
			t.Errorf("Resolve(%q, %v) refused: %v", c.p, c.write, err)
		}
	}
	bad := []struct {
		p     string
		write bool
	}{
		{"../outside/secret", false}, {filepath.Join(outside, "secret"), false},
		{"escape/secret", false}, {"escape/new.txt", true},
		{filepath.Join(ro, "SKILL.md"), true}, {"/etc/passwd", false},
	}
	for _, c := range bad {
		if _, err := j.Resolve(c.p, c.write); err == nil {
			t.Errorf("Resolve(%q, %v) must be refused", c.p, c.write)
		} else if !strings.Contains(err.Error(), "outside") {
			t.Errorf("error should explain the jail: %v", err)
		}
	}
}

func TestJailResolvedPathIsCanonical(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	got, _ := j.Resolve("inner/f.go", true)
	if got != filepath.Join(j.Root(), "sub", "f.go") {
		t.Fatalf("symlink must resolve: %s", got)
	}
}

func TestJailTildeExpands(t *testing.T) {
	root, _, _ := setup(t)
	t.Setenv("HOME", root)
	j, _ := NewJail(root, nil)
	if _, err := j.Resolve("~/x.txt", true); err != nil {
		t.Fatal(err)
	}
}

func TestJailDanglingSymlinks(t *testing.T) {
	root, outside, _ := setup(t)
	j, _ := NewJail(root, nil)
	mk := func(target, link string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join(outside, "new.txt"), "dangle-file") // missing file outside
	mk(filepath.Join(outside, "newdir"), "dangle-dir")   // missing dir outside
	mk(filepath.Join(root, "chain-2"), "chain-1")        // chain of dangling links ending outside
	mk(filepath.Join(outside, "new2.txt"), "chain-2")
	mk(filepath.Join("sub", "fresh.txt"), "rel-inside") // relative, lands inside
	mk("loop", "loop")                                  // self-loop
	mk("cyc-2", "cyc-1")                                // two-link cycle
	mk("cyc-1", "cyc-2")

	for _, p := range []string{"dangle-file", "dangle-dir/new.txt", "chain-1", "loop", "cyc-1"} {
		for _, write := range []bool{true, false} {
			if _, err := j.Resolve(p, write); err == nil {
				t.Errorf("Resolve(%q, write=%v) must be refused (dangling/looping symlink)", p, write)
			}
		}
	}
	got, err := j.Resolve("rel-inside", true)
	if err != nil {
		t.Fatalf("relative dangling link inside the jail must resolve: %v", err)
	}
	if want := filepath.Join(j.Root(), "sub", "fresh.txt"); got != want {
		t.Fatalf("resolved to %q, want %q", got, want)
	}
}

// A read-only root that does not exist when the jail is built must be kept:
// the global skills directory is routinely absent at session start and is
// created later — its files must become readable, and writes stay refused.
func TestJailKeepsNotYetExistingReadOnlyRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "work")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	future := filepath.Join(base, "skills", "deploy")
	j, err := NewJail(root, []string{future})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(filepath.Join(future, "references", "api.md"), false); err != nil {
		t.Fatalf("read under a not-yet-existing read-only root must resolve: %v", err)
	}
	if _, err := j.Resolve(filepath.Join(future, "x"), true); err == nil {
		t.Fatal("writes under a read-only root must stay refused")
	}
}

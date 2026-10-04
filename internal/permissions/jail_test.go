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

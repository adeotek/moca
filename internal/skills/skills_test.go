package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkSkill(t *testing.T, dir, name, desc string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, name), 0o755)
	os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\nbody\n"), 0o644)
}

func TestDiscoverPrecedence(t *testing.T) {
	proj, global := t.TempDir(), t.TempDir()
	mkSkill(t, proj, "deploy", "project deploy")
	mkSkill(t, global, "deploy", "global deploy")
	mkSkill(t, global, "notes", "take notes")
	os.MkdirAll(filepath.Join(global, "broken"), 0o755)
	os.WriteFile(filepath.Join(global, "broken", "SKILL.md"), []byte("---\nname: broken\n---\n"), 0o644)
	got, errs := Discover([]Dir{{proj, "project"}, {global, "global"}, {filepath.Join(proj, "missing"), "x"}})
	if len(got) != 2 || got[0].Name != "deploy" || got[0].Description != "project deploy" || got[1].Name != "notes" {
		t.Fatalf("%+v", got)
	}
	if !filepath.IsAbs(got[0].Path) {
		t.Fatal("absolute path")
	}
	if len(errs) != 1 {
		t.Fatalf("missing description is reported, missing dir is not: %v", errs)
	}
}

func TestExtractBuiltins(t *testing.T) {
	data := t.TempDir()
	dir, err := ExtractBuiltins(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, filepath.Join(data, "builtin-skills")+string(filepath.Separator)) {
		t.Fatal(dir)
	}
	again, err := ExtractBuiltins(data)
	if err != nil || again != dir {
		t.Fatalf("extraction must be idempotent: %q vs %q, %v", dir, again, err)
	}
	got, errs := Discover([]Dir{{dir, "builtin"}})
	if len(errs) != 0 || len(got) != 1 || got[0].Name != "rtk" {
		t.Fatalf("%+v %v", got, errs)
	}
}

func TestDiscoverReportsSymlinkedDir(t *testing.T) {
	root, real := t.TempDir(), t.TempDir()
	mkSkill(t, real, "linked", "via symlink")
	if err := os.Symlink(filepath.Join(real, "linked"), filepath.Join(root, "linked")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	got, errs := Discover([]Dir{{root, "global"}})
	if len(got) != 0 || len(errs) != 1 || !strings.Contains(errs[0].Error(), "symlink") {
		t.Fatalf("got=%v errs=%v", got, errs)
	}
}

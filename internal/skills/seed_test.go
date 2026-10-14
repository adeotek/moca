package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

// seedFSFixture is a two-skill seed tree: rtk is a lone SKILL.md, graphify
// carries a references/ subdirectory (the real graphify skill points at
// eight reference files — they must land too).
func seedFSFixture() fstest.MapFS {
	return fstest.MapFS{
		"seed/rtk/SKILL.md":                 &fstest.MapFile{Data: []byte("---\nname: rtk\ndescription: seeded rtk\n---\nbody\n")},
		"seed/graphify/SKILL.md":            &fstest.MapFile{Data: []byte("---\nname: graphify\ndescription: seeded graphify\n---\nbody\n")},
		"seed/graphify/references/query.md": &fstest.MapFile{Data: []byte("query reference\n")},
		"seed/README-not-a-skill.txt":       &fstest.MapFile{Data: []byte("a stray file at the seed root is not a skill\n")},
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A missing skills dir (first run on a fresh machine) is created, and every
// seed skill lands whole — SKILL.md and its supporting files.
func TestSeedSkillsCreatesMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config", "skills")
	created, err := seedSkills(seedFSFixture(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"graphify", "rtk"}; !reflect.DeepEqual(created, want) {
		t.Fatalf("created = %v, want %v", created, want)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "SKILL.md")); !strings.Contains(got, "seeded rtk") {
		t.Fatalf("rtk SKILL.md: %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "graphify", "references", "query.md")); got != "query reference\n" {
		t.Fatalf("graphify reference file: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "README-not-a-skill.txt")); err == nil {
		t.Fatal("a stray file at the seed root must not be copied")
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("temp directory left behind: %s", e.Name())
		}
	}
}

// The contract that lets users override: anything already at the skill's
// path — a customised skill, an empty opt-out directory — is left exactly as
// it is, and a second run creates nothing.
func TestSeedSkillsLeavesExistingAlone(t *testing.T) {
	dir := t.TempDir()
	mine := "---\nname: rtk\ndescription: my own rtk\n---\nmine\n"
	os.MkdirAll(filepath.Join(dir, "rtk"), 0o755)
	os.WriteFile(filepath.Join(dir, "rtk", "SKILL.md"), []byte(mine), 0o644)
	os.MkdirAll(filepath.Join(dir, "graphify"), 0o755) // empty: the documented opt-out

	created, err := seedSkills(seedFSFixture(), dir)
	if err != nil || len(created) != 0 {
		t.Fatalf("created=%v err=%v — existing skills must be left alone", created, err)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "SKILL.md")); got != mine {
		t.Fatalf("customised rtk was modified: %q", got)
	}
	if ents, _ := os.ReadDir(filepath.Join(dir, "graphify")); len(ents) != 0 {
		t.Fatalf("the empty opt-out directory was filled: %v", ents)
	}
}

// The check is per skill: a missing graphify is created even when rtk is
// already there.
func TestSeedSkillsFillsOnlyTheMissingOne(t *testing.T) {
	dir := t.TempDir()
	mine := "---\nname: rtk\ndescription: my own rtk\n---\nmine\n"
	os.MkdirAll(filepath.Join(dir, "rtk"), 0o755)
	os.WriteFile(filepath.Join(dir, "rtk", "SKILL.md"), []byte(mine), 0o644)

	created, err := seedSkills(seedFSFixture(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"graphify"}; !reflect.DeepEqual(created, want) {
		t.Fatalf("created = %v, want %v", created, want)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "SKILL.md")); got != mine {
		t.Fatalf("customised rtk was modified: %q", got)
	}
}

// A symlink at the skill's path is the user's arrangement (Discover reports
// and skips it): seeding must neither follow nor replace it.
func TestSeedSkillsLeavesSymlinkAlone(t *testing.T) {
	dir, real := t.TempDir(), t.TempDir()
	if err := os.Symlink(real, filepath.Join(dir, "rtk")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	created, err := seedSkills(seedFSFixture(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"graphify"}; !reflect.DeepEqual(created, want) {
		t.Fatalf("created = %v, want %v", created, want)
	}
	if ents, _ := os.ReadDir(real); len(ents) != 0 {
		t.Fatalf("seeding wrote through the symlink: %v", ents)
	}
}

// An unwritable config dir is reported, not swallowed: the caller turns it
// into a Warning (moca must still start).
func TestSeedSkillsReportsUnwritableDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	os.WriteFile(file, []byte("x"), 0o644)
	created, err := seedSkills(seedFSFixture(), filepath.Join(file, "skills"))
	if err == nil {
		t.Fatalf("expected an error, created=%v", created)
	}
}

// A half-written temp directory (a crash mid-seed) must never be discovered
// as a skill: it would shadow the real one, since dot-names sort first.
func TestDiscoverSkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	mkSkill(t, dir, ".seed-123", "half-written rtk")
	mkSkill(t, dir, "rtk", "the real rtk")
	got, errs := Discover([]Dir{{dir, "global"}})
	if len(errs) != 0 || len(got) != 1 || got[0].Description != "the real rtk" {
		t.Fatalf("got=%+v errs=%v", got, errs)
	}
}

// The embedded seed set is exactly rtk and graphify; both load through
// Discover as standard (global) skills, and graphify's SKILL.md is not left
// pointing at reference files that were not embedded.
func TestEmbeddedSeedSkills(t *testing.T) {
	dir := t.TempDir()
	created, err := SeedSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"graphify", "rtk"}; !reflect.DeepEqual(created, want) {
		t.Fatalf("created = %v, want %v", created, want)
	}
	got, errs := Discover([]Dir{{dir, "global"}})
	if len(errs) != 0 || len(got) != 2 || got[0].Name != "graphify" || got[1].Name != "rtk" {
		t.Fatalf("got=%+v errs=%v", got, errs)
	}
	for _, s := range got {
		if strings.TrimSpace(s.Description) == "" || s.Source != "global" {
			t.Fatalf("%+v", s)
		}
	}
	body := readFile(t, filepath.Join(dir, "graphify", "SKILL.md"))
	refs := regexp.MustCompile(`references/[a-z-]+\.md`).FindAllString(body, -1)
	if len(refs) == 0 {
		t.Fatal("graphify SKILL.md names no reference files — fixture drifted?")
	}
	for _, r := range refs {
		if _, err := os.Stat(filepath.Join(dir, "graphify", r)); err != nil {
			t.Errorf("graphify SKILL.md points at %s, which was not seeded", r)
		}
	}
}

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

// updateFSFixture is a seed tree whose bundle moved on from the copy on disk:
// rtk's SKILL.md and one reference changed, a reference is new, and old.md no
// longer exists in the bundle.
func updateFSFixture() fstest.MapFS {
	return fstest.MapFS{
		"seed/rtk/SKILL.md":          &fstest.MapFile{Data: []byte("v2 skill\n")},
		"seed/rtk/references/a.md":   &fstest.MapFile{Data: []byte("ref a v2\n")},
		"seed/rtk/references/new.md": &fstest.MapFile{Data: []byte("brand new\n")},
	}
}

// UpdateSeedSkills is the explicit refresh behind `moca update skills`: a dry
// run reports and writes nothing, a real run rewrites the bundled files,
// restores a deleted one, keeps the user's own files, and the next run is a
// no-op.
func TestUpdateSeedSkillsRefreshesAndKeeps(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, "rtk", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("SKILL.md", "v1 skill\n")
	write("references/a.md", "ref a v1\n")
	write("references/old.md", "dropped from the bundle\n")
	write("notes.md", "mine\n")

	ups, err := updateSeedSkills(updateFSFixture(), dir, true)
	if err != nil || len(ups) != 1 {
		t.Fatalf("ups=%+v err=%v", ups, err)
	}
	up := ups[0]
	if want := []string{"SKILL.md", "references/a.md", "references/new.md"}; !reflect.DeepEqual(up.Changed, want) {
		t.Fatalf("changed = %v, want %v", up.Changed, want)
	}
	if want := []string{"notes.md", "references/old.md"}; !reflect.DeepEqual(up.Kept, want) {
		t.Fatalf("kept = %v, want %v", up.Kept, want)
	}
	if up.Created || up.Skipped != "" {
		t.Fatalf("%+v", up)
	}
	// A dry run writes nothing.
	if got := readFile(t, filepath.Join(dir, "rtk", "SKILL.md")); got != "v1 skill\n" {
		t.Fatalf("dry run wrote SKILL.md: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "rtk", "references", "new.md")); !os.IsNotExist(err) {
		t.Fatalf("dry run created new.md: %v", err)
	}

	ups, err = updateSeedSkills(updateFSFixture(), dir, false)
	if err != nil || len(ups) != 1 || len(ups[0].Changed) != 3 {
		t.Fatalf("ups=%+v err=%v", ups, err)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "SKILL.md")); got != "v2 skill\n" {
		t.Fatalf("SKILL.md = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "references", "new.md")); got != "brand new\n" {
		t.Fatalf("new.md = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "notes.md")); got != "mine\n" {
		t.Fatalf("user note was touched: %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "references", "old.md")); got != "dropped from the bundle\n" {
		t.Fatalf("dropped reference was deleted: %q", got)
	}

	// Idempotent: the second run changes nothing and still reports the extras.
	ups, err = updateSeedSkills(updateFSFixture(), dir, false)
	if err != nil || len(ups) != 1 || len(ups[0].Changed) != 0 {
		t.Fatalf("ups=%+v err=%v", ups, err)
	}
	if want := []string{"notes.md", "references/old.md"}; !reflect.DeepEqual(ups[0].Kept, want) {
		t.Fatalf("kept = %v, want %v", ups[0].Kept, want)
	}
}

// A missing skill is planted whole, same as SeedSkills would.
func TestUpdateSeedSkillsCreatesMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	ups, err := updateSeedSkills(updateFSFixture(), dir, false)
	if err != nil || len(ups) != 1 || !ups[0].Created {
		t.Fatalf("ups=%+v err=%v", ups, err)
	}
	if got := readFile(t, filepath.Join(dir, "rtk", "references", "new.md")); got != "brand new\n" {
		t.Fatalf("new.md = %q", got)
	}
	ups, err = updateSeedSkills(updateFSFixture(), dir, false)
	if err != nil || ups[0].Created || len(ups[0].Changed) != 0 {
		t.Fatalf("second run: ups=%+v err=%v", ups, err)
	}
}

// A symlink (the user's arrangement) and a plain file where the skill dir
// should be are reported and never written through.
func TestUpdateSeedSkillsSkipsWhatItCannotOwn(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "rtk"), 0o755); err != nil {
		t.Fatal(err)
	}
	// rtk is a symlink to a real skill elsewhere.
	if err := os.Remove(filepath.Join(dir, "rtk")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "rtk")); err != nil {
		t.Fatal(err)
	}
	// graphify is a plain file where the directory should be.
	if err := os.WriteFile(filepath.Join(dir, "graphify"), []byte("not a skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"seed/rtk/SKILL.md":      &fstest.MapFile{Data: []byte("rtk v2\n")},
		"seed/graphify/SKILL.md": &fstest.MapFile{Data: []byte("graphify v2\n")},
	}
	ups, err := updateSeedSkills(fsys, dir, false)
	if err != nil || len(ups) != 2 {
		t.Fatalf("ups=%+v err=%v", ups, err)
	}
	if ups[0].Name != "graphify" || ups[0].Skipped != "not a directory" {
		t.Fatalf("%+v", ups[0])
	}
	if ups[1].Name != "rtk" || ups[1].Skipped != "symlink" {
		t.Fatalf("%+v", ups[1])
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Fatalf("wrote through the symlink: %v", entries)
	}
}

// The shipped bundle round-trips: a tampered or deleted file comes back
// byte-identical on the next UpdateSeedSkills.
func TestUpdateSeedSkillsMirrorsTheEmbeddedBundle(t *testing.T) {
	dir := t.TempDir()
	if _, err := SeedSkills(dir); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(dir, "rtk", "SKILL.md")
	want, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := filepath.Join(dir, "graphify", "references", "query.md")
	if err := os.Remove(ref); err != nil {
		t.Fatal(err)
	}
	ups, err := UpdateSeedSkills(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	var changed int
	for _, u := range ups {
		changed += len(u.Changed)
	}
	if changed != 2 {
		t.Fatalf("changed = %d across %+v, want 2", changed, ups)
	}
	if got, _ := os.ReadFile(skill); string(got) != string(want) {
		t.Fatal("SKILL.md was not restored to the embedded bytes")
	}
	if _, err := os.Stat(ref); err != nil {
		t.Fatalf("deleted reference was not restored: %v", err)
	}
}

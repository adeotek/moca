package skills

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// day returns the test clock minus n days (a date, as ParseRule reads it).
func day(now time.Time, n int) string { return now.AddDate(0, 0, -n).Format("2006-01-02") }

func writeSkill(t *testing.T, dir, name, data string) string {
	t.Helper()
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "SKILL.md")
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBodyOffset(t *testing.T) {
	cases := []string{
		"---\nname: x\n---\nbody\n",
		"---\r\nname: x\r\n---\r\nbody\r\n",
		"\ufeff---\nname: x\n---\nbody\n",
		"---\nname: x\n---\n",
	}
	for _, data := range cases {
		off, err := BodyOffset([]byte(data))
		if err != nil {
			t.Fatalf("%q: %v", data, err)
		}
		_, body, err := ParseFrontmatter([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		got := strings.ReplaceAll(string([]byte(data)[off:]), "\r\n", "\n")
		if got != string(body) {
			t.Fatalf("%q: offset %d gives %q, ParseFrontmatter body %q", data, off, got, body)
		}
	}
	if _, err := BodyOffset([]byte("# no frontmatter\n")); err == nil {
		t.Fatal("want an error without frontmatter")
	}
	if _, err := BodyOffset([]byte("---\nname: x\n")); err == nil {
		t.Fatal("want an error for unterminated frontmatter")
	}
}

func TestPruneAutoExpiresX1Only(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	body := "- **old x1**: rule one (" + "×1, " + day(now, 91) + ")\n" +
		"- **old x1 ascii**: rule two (x1, " + day(now, 91) + ")\n" +
		"- **old x2**: rule three (×2, " + day(now, 91) + ")\n" +
		"- **old user**: rule four (user, " + day(now, 400) + ")\n" +
		"- **old x3**: rule five (×3, " + day(now, 91) + ")\n" +
		"- **young x1**: rule six (×1, " + day(now, 10) + ")\n"
	p := writeSkill(t, dir, "moca-x", string(autoSkill("moca-x", "d", body)))
	rep, err := PruneAutoSkills(dir, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RulesRemoved != 2 {
		t.Fatalf("RulesRemoved = %d, want 2", rep.RulesRemoved)
	}
	got, _ := os.ReadFile(p)
	for _, gone := range []string{"rule one", "rule two"} {
		if bytes.Contains(got, []byte(gone)) {
			t.Errorf("%q must be pruned", gone)
		}
	}
	for _, kept := range []string{"rule three", "rule four", "rule five", "rule six"} {
		if !bytes.Contains(got, []byte(kept)) {
			t.Errorf("%q must survive:\n%s", kept, got)
		}
	}
	if len(rep.Touched) != 1 || rep.Touched[0] != "moca-x" {
		t.Fatalf("Touched = %v", rep.Touched)
	}
	if len(rep.PinnedAged) != 1 || rep.PinnedAged[0] != "moca-x" {
		t.Fatalf("PinnedAged = %v (a 400-day pin is surfaced, never dropped)", rep.PinnedAged)
	}
}

func TestPruneAutoSurfacesStalePins(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	old := writeSkill(t, dir, "moca-old", string(autoSkill("moca-old", "d", "- **a**: b (user, "+day(now, 400)+")\n")))
	young := writeSkill(t, dir, "moca-young", string(autoSkill("moca-young", "d", "- **a**: b (user, "+day(now, 10)+")\n")))
	rep, err := PruneAutoSkills(dir, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.PinnedAged) != 1 || rep.PinnedAged[0] != "moca-old" {
		t.Fatalf("PinnedAged = %v", rep.PinnedAged)
	}
	if rep.RulesRemoved != 0 || len(rep.Touched) != 0 {
		t.Fatalf("a pinned rule is never dropped: %+v", rep)
	}
	for _, p := range []string{old, young} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("pinned skill must survive: %v", err)
		}
	}
}

func TestPruneAutoKeepsUndatedLines(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	p := writeSkill(t, dir, "moca-x", string(autoSkill("moca-x", "d",
		"- **a**: no provenance\n- **b**: bad date (×1, 2026-13-45)\n- **c**: ok (×1, "+day(now, 91)+")\n")))
	rep, err := PruneAutoSkills(dir, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RulesRemoved != 1 {
		t.Fatalf("RulesRemoved = %d, want 1", rep.RulesRemoved)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Contains(got, []byte("no provenance")) || !bytes.Contains(got, []byte("bad date")) {
		t.Fatalf("undatable lines must survive:\n%s", got)
	}
}

func TestPruneAutoRemovesEmptySkillFileOnly(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	bare := writeSkill(t, dir, "moca-bare", string(autoSkill("moca-bare", "d", "- **a**: b (×1, "+day(now, 91)+")\n")))
	withNotes := writeSkill(t, dir, "moca-notes", string(autoSkill("moca-notes", "d", "# Head\n\n- **a**: b (×1, "+day(now, 91)+")\n")))
	notes := filepath.Join(filepath.Dir(withNotes), "notes.md")
	if err := os.WriteFile(notes, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := PruneAutoSkills(dir, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.SkillsRemoved) != 2 {
		t.Fatalf("SkillsRemoved = %v", rep.SkillsRemoved)
	}
	for _, p := range []string{bare, withNotes} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s must be gone", p)
		}
	}
	if _, err := os.Stat(filepath.Dir(bare)); !os.IsNotExist(err) {
		t.Fatal("an empty skill's directory must go too")
	}
	if _, err := os.Stat(notes); err != nil {
		t.Fatal("a user's file beside it must survive")
	}
	if fi, err := os.Stat(filepath.Dir(withNotes)); err != nil || !fi.IsDir() {
		t.Fatal("the directory holding a user's file must survive")
	}
}

func TestPruneAutoKeepsProseWithoutRules(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	p := writeSkill(t, dir, "moca-prose", string(autoSkill("moca-prose", "d", "# H\n\nprose lives here.\n\n- **a**: b (×1, "+day(now, 91)+")\n")))
	if _, err := PruneAutoSkills(dir, now, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("a prose body must not be removed")
	}
	got, _ := os.ReadFile(p)
	if !bytes.Contains(got, []byte("prose lives here")) {
		t.Fatalf("prose must survive:\n%s", got)
	}
}

func TestPruneAutoPreservesFrontmatterBytes(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	// Nested keys, comments, key order and CRLF endings must survive a rewrite.
	head := "---\r\nname: moca-x\r\ndescription: d\r\nauto: true\r\n# a comment\r\nmeta:\r\n  nested: 1\r\n---\r\n"
	p := writeSkill(t, dir, "moca-x", head+"- **keep**: a (×2, "+day(now, 200)+")\r\n- **drop**: b (×1, "+day(now, 91)+")\r\n")
	if _, err := PruneAutoSkills(dir, now, false); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.HasPrefix(got, []byte(head)) {
		t.Fatalf("frontmatter must stay byte-identical:\n%q", got)
	}
	if !bytes.Contains(got, []byte("keep")) || bytes.Contains(got, []byte("drop")) {
		t.Fatalf("body rewrite wrong:\n%s", got)
	}
	if !bytes.Contains(got, []byte("\r\n- **keep**")) {
		t.Fatal("CRLF line endings must survive")
	}
}

func TestPruneAutoNeverTouchesUserSkills(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	data := string(plainSkill("hand", "d", "- **a**: b (×1, "+day(now, 400)+")\n"))
	p := writeSkill(t, dir, "hand", data)
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := PruneAutoSkills(dir, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RulesRemoved != 0 || len(rep.Touched) != 0 || len(rep.SkillsRemoved) != 0 || len(rep.PinnedAged) != 0 {
		t.Fatalf("hand-written skills are untouched: %+v", rep)
	}
	got, _ := os.ReadFile(p)
	if string(got) != data {
		t.Fatal("file content changed")
	}
	after, _ := os.Stat(p)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("mtime changed")
	}
}

func TestPruneAutoIgnoresUnrelatedFiles(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden", "SKILL.md"), autoSkill("moca-h", "d", "- **a**: b (×1, "+day(now, 91)+")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	real := writeSkill(t, dir, "moca-real", string(autoSkill("moca-real", "d", "# only\n")))
	if err := os.Symlink(real, filepath.Join(dir, "moca-link")); err != nil {
		t.Fatal(err)
	}
	rep, err := PruneAutoSkills(dir, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.SkillsRemoved) != 1 || rep.SkillsRemoved[0] != "moca-real" {
		t.Fatalf("%+v", rep)
	}
	if _, err := os.Stat(filepath.Join(dir, ".hidden", "SKILL.md")); err != nil {
		t.Fatal("hidden dirs are not skills")
	}
}

func TestPruneAutoAtomicRewriteKeepsMode(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	p := writeSkill(t, dir, "moca-x", string(autoSkill("moca-x", "d", "- **a**: b (×1, "+day(now, 91)+")\n- **b**: c (×2, "+day(now, 10)+")\n")))
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneAutoSkills(dir, now, false); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", fi.Mode().Perm())
	}
}

func TestPruneAutoLeavesNoTempOrBakFiles(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	writeSkill(t, dir, "moca-x", string(autoSkill("moca-x", "d", "- **a**: b (×1, "+day(now, 91)+")\n- **b**: c (×2, "+day(now, 10)+")\n")))
	if _, err := PruneAutoSkills(dir, now, false); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(filepath.Join(dir, "moca-x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != "SKILL.md" {
		t.Fatalf("skill dir holds %v, want only SKILL.md", ents)
	}
}

func TestPruneAutoDryRunWritesNothing(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	p := writeSkill(t, dir, "moca-x", string(autoSkill("moca-x", "d", "- **a**: b (×1, "+day(now, 91)+")\n- **b**: c (×2, "+day(now, 10)+")\n")))
	empty := writeSkill(t, dir, "moca-empty", string(autoSkill("moca-empty", "d", "# only\n")))
	before, _ := os.ReadFile(p)
	beforeEmpty, _ := os.ReadFile(empty)
	fi1, _ := os.Stat(p)
	fi2, _ := os.Stat(empty)
	rep, err := PruneAutoSkills(dir, now, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RulesRemoved != 1 || len(rep.SkillsRemoved) != 1 || len(rep.Touched) != 1 {
		t.Fatalf("dry run must report the same changes: %+v", rep)
	}
	after, _ := os.ReadFile(p)
	afterEmpty, _ := os.ReadFile(empty)
	if !bytes.Equal(before, after) || !bytes.Equal(beforeEmpty, afterEmpty) {
		t.Fatal("dry run changed a file")
	}
	if fa, _ := os.Stat(p); !fa.ModTime().Equal(fi1.ModTime()) {
		t.Fatal("dry run touched mtime")
	}
	if fa, _ := os.Stat(empty); !fa.ModTime().Equal(fi2.ModTime()) {
		t.Fatal("dry run touched the empty skill")
	}
}

func TestPruneAutoFailureIsReturned(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permissions do not refuse writes")
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	badDir := filepath.Join(dir, "moca-bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "SKILL.md"), autoSkill("moca-bad", "d", "- **a**: b (×1, "+day(now, 91)+")\n- **b**: c (×2, "+day(now, 10)+")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	good := writeSkill(t, dir, "moca-good", string(autoSkill("moca-good", "d", "- **a**: b (×1, "+day(now, 91)+")\n- **b**: c (×2, "+day(now, 10)+")\n")))
	if err := os.Chmod(badDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(badDir, 0o755)
	rep, err := PruneAutoSkills(dir, now, false)
	if err == nil {
		t.Fatal("an unwritable skill must report its error")
	}
	if rep.RulesRemoved != 1 || len(rep.Touched) != 1 || rep.Touched[0] != "moca-good" {
		t.Fatalf("the other skill must still be processed: %+v", rep)
	}
	if got, _ := os.ReadFile(good); bytes.Contains(got, []byte("drop-worthy")) || !bytes.Contains(got, []byte("- **b**")) {
		t.Fatalf("good skill not pruned:\n%s", got)
	}
}

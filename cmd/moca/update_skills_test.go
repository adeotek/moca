package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillsConfigDir points every XDG root at a fresh temp tree, so `update
// skills` — and the boot that precedes every run() — never touches the real
// ~/.config, ~/.local/share or ~/.local/state.
func skillsConfigDir(t *testing.T) string {
	t.Helper()
	x := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(x, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(x, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(x, "state"))
	return filepath.Join(x, "config", "moca", "skills")
}

// `moca update skills` plants the seeded skills on a fresh config dir,
// reports them up to date on the next run, and `--check` reports without
// writing.
func TestRunUpdateSkills(t *testing.T) {
	dir := skillsConfigDir(t)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "skills"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	// The file count is the bundle's, not a number pinned here: a legitimately
	// added upstream file must not fail the command's test.
	graphifyFiles := 0
	if err := filepath.WalkDir(filepath.Join(dir, "graphify"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			graphifyFiles++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"skills dir: " + dir, "rtk: created (1 file)",
		fmt.Sprintf("graphify: created (%d %s)", graphifyFiles, plural(graphifyFiles, "file", "files"))} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, out.String())
		}
	}
	skill := filepath.Join(dir, "rtk", "SKILL.md")
	embedded, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if code := run(context.Background(), []string{"update", "skills"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "rtk: up to date") || !strings.Contains(out.String(), "graphify: up to date") {
		t.Fatalf("stdout:\n%s", out.String())
	}

	// A tampered bundle file and a user note: --check reports both and writes
	// nothing; the real run restores the bundled bytes and keeps the note.
	if err := os.WriteFile(skill, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(dir, "rtk", "notes.md")
	if err := os.WriteFile(note, []byte("note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run(context.Background(), []string{"update", "skills", "--check"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	for _, want := range []string{"rtk: would update 1 file (SKILL.md)", "kept 1 extra file (notes.md)"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, out.String())
		}
	}
	if got, _ := os.ReadFile(skill); string(got) != "mine\n" {
		t.Fatalf("--check wrote the file: %q", got)
	}

	out.Reset()
	if code := run(context.Background(), []string{"update", "skills"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if got, _ := os.ReadFile(skill); !bytes.Equal(got, embedded) {
		t.Fatal("SKILL.md was not restored to the embedded bytes")
	}
	if got, _ := os.ReadFile(note); string(got) != "note\n" {
		t.Fatalf("user note was touched: %q", got)
	}
	if !strings.Contains(out.String(), "rtk: updated 1 file (SKILL.md) — kept 1 extra file (notes.md)") {
		t.Fatalf("stdout:\n%s", out.String())
	}
}

// A refresh that errors must not print "up to date" for the skill it failed to
// write: stdout is what a script or the user greps, and the aggregate error on
// stderr used to arrive beside a status line that claimed success (F9).
func TestRunUpdateSkillsReportsAFailedRefresh(t *testing.T) {
	dir := skillsConfigDir(t)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "skills"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("plant: exit %d, stderr: %s", code, errb.String())
	}
	// A directory where the bundle's SKILL.md goes: the read fails, and nothing
	// this command can do turns it into an update.
	skill := filepath.Join(dir, "rtk", "SKILL.md")
	if err := os.Remove(skill); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(skill, 0o755); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	errb.Reset()
	if code := run(context.Background(), []string{"update", "skills"}, nil, &out, &errb); code != exitRuntime {
		t.Fatalf("a failed refresh must exit %d, got %d (stderr: %s)", exitRuntime, code, errb.String())
	}
	if strings.Contains(out.String(), "rtk: up to date") {
		t.Fatalf("stdout claims an errored refresh is up to date:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "rtk: failed (") {
		t.Fatalf("stdout must report the failure against the skill:\n%s", out.String())
	}
}

// A skill path that is a symlink is the user's arrangement: reported, not
// written through, and the exit stays 0.
func TestRunUpdateSkillsReportsASymlink(t *testing.T) {
	dir := skillsConfigDir(t)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "skills"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if err := os.RemoveAll(filepath.Join(dir, "graphify")); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, "graphify")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run(context.Background(), []string{"update", "skills"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "graphify: skipped (symlink — left as is)") {
		t.Fatalf("stdout:\n%s", out.String())
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Fatalf("wrote through the symlink: %v", entries)
	}
}

func TestRunUpdateSkillsUsage(t *testing.T) {
	skillsConfigDir(t)
	for _, args := range [][]string{{"update", "skills", "--force"}, {"update", "skills", "--bogus"}} {
		var out, errb bytes.Buffer
		if code := run(context.Background(), args, nil, &out, &errb); code != exitUsage {
			t.Fatalf("%v: exit %d, stderr: %s", args, code, errb.String())
		}
	}
}

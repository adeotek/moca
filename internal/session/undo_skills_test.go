package session

// Pin for N-18 / D7: undo must not roll back a project skill file. The skill
// write guard owns that directory (§12) — restoring or deleting a SKILL.md
// through /undo is the same edit the guard exists to check, taken around it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUndoRefusesASkillSnapshot(t *testing.T) {
	root := t.TempDir()
	skills := filepath.Join(root, ".moca", "skills", "moca-x")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(skills, "SKILL.md")
	if err := os.WriteFile(skill, []byte("auto: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := Create(t.TempDir(), Header{}, "t")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s := NewSnapshots(w, t.TempDir(), nil)
	s.RefuseUnder(filepath.Join(root, ".moca", "skills"))
	if err := s.Guard(skill, func() error {
		return os.WriteFile(skill, []byte("auto: true\n- changed\n"), 0o644)
	}); err != nil {
		t.Fatal(err)
	}

	msg, err := s.Undo()
	if err == nil {
		t.Fatalf("undo rolled a skill file back: %q", msg)
	}
	if !strings.Contains(err.Error(), "project skill file") {
		t.Fatalf("error must name the reason, got %v", err)
	}
	// The file keeps the write: the refusal happens before anything is restored.
	b, _ := os.ReadFile(skill)
	if !strings.Contains(string(b), "- changed") {
		t.Fatalf("the skill file was modified by a refused undo: %q", b)
	}

	// Control: an ordinary file still undoes.
	plain := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(plain, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Guard(plain, func() error { return os.WriteFile(plain, []byte("v2"), 0o644) }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Undo(); err != nil {
		t.Fatalf("a normal file must still undo: %v", err)
	}
	if b, _ := os.ReadFile(plain); string(b) != "v1" {
		t.Fatalf("undo did not restore: %q", b)
	}
}

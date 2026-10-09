package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplayPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	if got := displayPath(filepath.Join(home, ".config", "moca", "prompts")); got != "~/.config/moca/prompts" {
		t.Fatalf("home path: %q", got)
	}
	if got := displayPath(home); got != "~" {
		t.Fatalf("home itself: %q", got)
	}
	if got := displayPath("/elsewhere/prompts"); got != "/elsewhere/prompts" {
		t.Fatalf("outside home: %q", got)
	}
}

// The first run seeds the starter create-command template into the user's
// prompt dir; an existing (edited) file is never overwritten.
func TestSeedUserPrompts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "prompts") // created by the seed
	if err := SeedUserPrompts(dir); err != nil {
		t.Fatal(err)
	}
	ps := LoadPrompts([]Dir{{Path: dir, Source: "global"}})
	if len(ps) != 1 || ps[0].Name != "create-command" {
		t.Fatalf("seeded: %+v", ps)
	}
	if ps[0].Source != "global" || ps[0].ArgumentHint == "" || !strings.Contains(ps[0].Description, "slash command") {
		t.Fatalf("frontmatter: %+v", ps[0])
	}
	// The body teaches the storage locations (with the real dir substituted),
	// the format and the placeholders — the syntax examples survive expansion
	// (they are written escaped: $$1..$$9 etc.).
	for _, want := range []string{displayPath(dir), ".moca/prompts", "argument-hint"} {
		if !strings.Contains(ps[0].Body, want) {
			t.Errorf("the starter prompt is missing %q", want)
		}
	}
	exp := ExpandPrompt(ps[0].Body, "demo")
	for _, want := range []string{"$ARGUMENTS or $@", "$1..$9", "$$ (a literal $)"} {
		if !strings.Contains(exp, want) {
			t.Errorf("the expanded starter is missing %q", want)
		}
	}
	if strings.Contains(ps[0].Body, "{{") {
		t.Error("a placeholder was left unsubstituted")
	}
	// A user-edited file survives the next run byte for byte (and keeps its mode).
	path := filepath.Join(dir, "create-command.md")
	if err := os.WriteFile(path, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil { // WriteFile keeps the mode of an existing file
		t.Fatal(err)
	}
	if err := SeedUserPrompts(dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "mine\n" {
		t.Fatalf("seed overwrote a user file: %q", b)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatal("mode must be kept")
	}
}

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forkFixture writes a two-file built-in and returns (builtinDir, userDir).
func forkFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	builtin := filepath.Join(base, "builtin-skills", "abc123")
	user := filepath.Join(base, "config", "skills")
	for _, n := range []string{"review", "debt"} {
		if err := os.MkdirAll(filepath.Join(builtin, n), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + n + "\ndescription: the " + n + " skill\n---\nbody of " + n + "\n"
		if err := os.WriteFile(filepath.Join(builtin, n, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A directory without SKILL.md is not a skill, and must not be forked.
	if err := os.MkdirAll(filepath.Join(builtin, "not-a-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	return builtin, user
}

func TestForkCopiesABuiltinIntoTheUsersDir(t *testing.T) {
	builtin, user := forkFixture(t)
	dst, err := Fork(builtin, user, "review")
	if err != nil {
		t.Fatalf("fork review: %v", err)
	}
	if want := filepath.Join(user, "review"); dst != want {
		t.Fatalf("forked to %s, want %s", dst, want)
	}
	got, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if err != nil {
		t.Fatalf("read the fork: %v", err)
	}
	if !strings.Contains(string(got), "body of review") {
		t.Fatalf("the fork is not a copy of the built-in:\n%s", got)
	}

	// The copy must win discovery over the built-in of the same name: global
	// is read first and Discover keeps the first name it sees.
	sk, errs := Discover([]Dir{{Path: user, Source: "global"}, {Path: builtin, Source: "builtin"}})
	if len(errs) != 0 {
		t.Fatalf("discover: %v", errs)
	}
	var review *Skill
	for i := range sk {
		if sk[i].Name == "review" {
			review = &sk[i]
		}
	}
	if review == nil || review.Source != "global" {
		t.Fatalf("the fork must shadow the built-in, got %+v", review)
	}
}

func TestForkNeverOverwrites(t *testing.T) {
	builtin, user := forkFixture(t)
	if _, err := Fork(builtin, user, "review"); err != nil {
		t.Fatal(err)
	}
	// The user edits their copy; a second fork must refuse and leave it.
	mine := filepath.Join(user, "review", "SKILL.md")
	if err := os.WriteFile(mine, []byte("---\nname: review\ndescription: mine\n---\nmy own\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Fork(builtin, user, "review")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a second fork must refuse, got %v", err)
	}
	got, _ := os.ReadFile(mine)
	if !strings.Contains(string(got), "my own") {
		t.Fatalf("a refused fork must not touch the user's copy:\n%s", got)
	}
}

func TestForkRefusesUnknownAndUnsafeNames(t *testing.T) {
	builtin, user := forkFixture(t)
	for _, name := range []string{"nope", "not-a-skill", "../etc", "Review", "", "a/b"} {
		if _, err := Fork(builtin, user, name); err == nil {
			t.Fatalf("%q must be refused", name)
		}
	}
	// Nothing was created for the refused names.
	ents, _ := os.ReadDir(user)
	if len(ents) != 0 {
		t.Fatalf("a refused fork left %d entries behind", len(ents))
	}
	// The error names what is available, so the user can retype it.
	_, err := Fork(builtin, user, "nope")
	if !strings.Contains(err.Error(), "debt, review") {
		t.Fatalf("the refusal must list the built-ins: %v", err)
	}
}

func TestForkReportsTheBundledSetWithTheRealBuiltins(t *testing.T) {
	// The shipped bundle must contain the three v0.8.1 skills, or a README and
	// a /help row would point at a name that cannot be forked.
	dir, err := ExtractBuiltins(t.TempDir())
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	names := BuiltinNames(dir)
	for _, want := range []string{"audit", "debt", "review"} {
		if !slicesContains(names, want) {
			t.Fatalf("built-in %q missing from %v", want, names)
		}
	}
}

func slicesContains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

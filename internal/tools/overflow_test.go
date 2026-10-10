package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var spilledRe = regexp.MustCompile(`saved to (\S+) — read it`)

// spilledPath extracts the overflow file named in a result.
func spilledPath(t *testing.T, content string) string {
	t.Helper()
	m := spilledRe.FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("no overflow file named in: %.300q", content)
	}
	return m[1]
}

func TestSpillWritesFullOutput(t *testing.T) {
	env := &Env{SpillDir: filepath.Join(t.TempDir(), "overflow"), SpillPrefix: "abcd1234-"}
	note := Spill(env, "shell", "a\nb\nc")
	p := spilledPath(t, note)
	if !strings.HasPrefix(filepath.Base(p), "abcd1234-shell-") || !strings.Contains(note, "3 lines") {
		t.Fatalf("note: %q", note)
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "a\nb\nc" {
		t.Fatalf("%q %v", b, err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v (tool output can hold secrets)", fi.Mode().Perm())
	}
	if Spill(&Env{}, "shell", "x") != "" {
		t.Fatal("no SpillDir: no spill")
	}
}

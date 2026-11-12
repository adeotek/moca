package skills

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSelfKnowledgeReferencesAreByteIdentical: the self-knowledge skill
// embeds the living docs (SPECS.md, README.md, CHANGELOG.md) as references —
// the build copies them (go:embed), so a stale copy would answer questions
// about moca from outdated text. Any docs edit must re-copy the references in
// the same commit; this test is the guard. (The changelog was the one that
// rotted: it sat at v0.7.7 through two releases before this pair was pinned.)
func TestSelfKnowledgeReferencesAreByteIdentical(t *testing.T) {
	for _, c := range []struct{ embedded, source string }{
		{"builtin/self-knowledge/references/specs.md", filepath.Join("..", "..", "docs", "specs", "SPECS.md")},
		{"builtin/self-knowledge/references/readme.md", filepath.Join("..", "..", "README.md")},
		{"builtin/self-knowledge/references/changelog.md", filepath.Join("..", "..", "CHANGELOG.md")},
	} {
		got, err := fs.ReadFile(builtinFS, c.embedded)
		if err != nil {
			t.Errorf("%s: %v", c.embedded, err)
			continue
		}
		src, err := os.ReadFile(c.source)
		if err != nil {
			t.Errorf("%s: %v", c.source, err)
			continue
		}
		if !bytes.Equal(got, src) {
			t.Errorf("%s differs from %s (%d vs %d bytes) — re-copy the reference", c.embedded, c.source, len(got), len(src))
		}
	}
}

// TestSelfKnowledgeReferencesAreIndexed: a reference the skill body never
// names is data the model will not read. The changelog sat synced but
// unindexed from v0.7.3 (when the reference was added) to v0.8.0 — the
// byte-identity guard above would not have caught a new orphan either. Both
// directions: every embedded reference is named, and every name resolves.
func TestSelfKnowledgeReferencesAreIndexed(t *testing.T) {
	const dir = "builtin/self-knowledge/references"
	body, err := fs.ReadFile(builtinFS, "builtin/self-knowledge/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(builtinFS, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.Contains(string(body), "references/"+e.Name()) {
			t.Errorf("self-knowledge/SKILL.md never names references/%s — the model is told to read the references and this one is invisible", e.Name())
		}
	}
	for _, ref := range regexp.MustCompile(`references/[A-Za-z0-9._-]+`).FindAllString(string(body), -1) {
		if _, err := fs.Stat(builtinFS, "builtin/self-knowledge/"+ref); err != nil {
			t.Errorf("self-knowledge/SKILL.md names %s, which does not exist", ref)
		}
	}
}

package skills

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestSelfKnowledgeReferencesAreByteIdentical: the self-knowledge skill
// embeds the living docs (SPECS.md, README.md) as references — the build
// copies them (go:embed), so a stale copy would answer questions about moca
// from outdated text. Any docs edit must re-copy the references in the same
// commit; this test is the guard.
func TestSelfKnowledgeReferencesAreByteIdentical(t *testing.T) {
	for _, c := range []struct{ embedded, source string }{
		{"builtin/self-knowledge/references/specs.md", filepath.Join("..", "..", "docs", "specs", "SPECS.md")},
		{"builtin/self-knowledge/references/readme.md", filepath.Join("..", "..", "README.md")},
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

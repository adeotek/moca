package skills

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Every real-world SKILL.md in the corpus (graphify, claude-code, pi,
// opencode — plus synthetic edge cases) must load through Discover unchanged.
// Fixtures are byte-for-byte copies: never edit one to make it pass; fix the
// parser instead (see SOURCES.md for provenance and licenses).
func TestEcosystemSkillsLoadUnchanged(t *testing.T) {
	b, err := os.ReadFile("testdata/ecosystem/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]struct{ Name, Description string }
	if err := json.Unmarshal(b, &expected); err != nil {
		t.Fatal(err)
	}
	sources, err := os.ReadDir("testdata/ecosystem")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, src := range sources {
		if !src.IsDir() {
			continue
		}
		got, errs := Discover([]Dir{{Path: filepath.Join("testdata/ecosystem", src.Name()), Source: src.Name()}})
		for _, e := range errs {
			t.Errorf("%s: %v", src.Name(), e)
		}
		for _, s := range got {
			key := src.Name() + "/" + filepath.Base(filepath.Dir(s.Path))
			want, ok := expected[key]
			if !ok {
				t.Errorf("%s: no expected entry", key)
				continue
			}
			if s.Name != want.Name || s.Description != want.Description {
				t.Errorf("%s:\n got  %q / %q\n want %q / %q", key, s.Name, s.Description, want.Name, want.Description)
			}
			seen++
		}
	}
	if seen != len(expected) {
		t.Fatalf("loaded %d of %d expected skills", seen, len(expected))
	}

	// The byte-level cases are the point: guard them against accidental
	// rewrites (.gitattributes pins `* -text` here so git cannot normalize
	// them — a tool that silently does would defang the fixture).
	crlf, _ := os.ReadFile("testdata/ecosystem/synthetic/crlf/SKILL.md")
	if !bytes.Contains(crlf, []byte("\r\n")) {
		t.Fatal("synthetic/crlf fixture lost its CRLF line endings")
	}
	bom, _ := os.ReadFile("testdata/ecosystem/synthetic/bom/SKILL.md")
	if !bytes.HasPrefix(bom, []byte("\xef\xbb\xbf")) {
		t.Fatal("synthetic/bom fixture lost its BOM")
	}
}

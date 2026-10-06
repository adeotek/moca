package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	seen := map[string]bool{}
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
			seen[key] = true
		}
	}
	// Driven by expected.json's keys: every pinned skill must appear, so a
	// deleted fixture cannot hide behind an aggregate count (review F6/L6).
	for key := range expected {
		if !seen[key] {
			t.Errorf("expected skill %s was not loaded", key)
		}
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

var sourcesHash = regexp.MustCompile(`(?m)^([0-9a-f]{64})\s+(\S+)$`)

// SOURCES.md's byte-for-byte claim is enforced, not just documented: every
// recorded SHA-256 must match the vendored file, and every vendored file
// must be recorded (review F5/L5 — a silent edit of a fixture would
// otherwise pass CI until someone re-read the hashes).
func TestEcosystemSourcesHashes(t *testing.T) {
	b, err := os.ReadFile("testdata/ecosystem/SOURCES.md")
	if err != nil {
		t.Fatal(err)
	}
	matches := sourcesHash.FindAllStringSubmatch(string(b), -1)
	if len(matches) == 0 {
		t.Fatal("no SHA-256 entries found in SOURCES.md")
	}
	listed := map[string]bool{}
	for _, m := range matches {
		listed[m[2]] = true
		got, err := hashFile(filepath.Join("testdata/ecosystem", m[2]))
		if err != nil {
			t.Errorf("%s: %v", m[2], err)
			continue
		}
		if got != m[1] {
			t.Errorf("%s: sha256 %s, SOURCES.md records %s", m[2], got, m[1])
		}
	}
	err = filepath.WalkDir("testdata/ecosystem", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel("testdata/ecosystem", path)
		if relErr != nil {
			return relErr
		}
		// synthetic/ fixtures are the repo's own files (their byte shape is
		// pinned above); everything else is vendored and must be recorded.
		if strings.HasPrefix(rel, "synthetic") || rel == "SOURCES.md" || rel == "expected.json" || rel == ".gitattributes" {
			return nil
		}
		if !listed[filepath.ToSlash(rel)] {
			t.Errorf("%s: vendored file has no SHA-256 entry in SOURCES.md", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

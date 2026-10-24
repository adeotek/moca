package main

// The product's name and its descriptions live in exactly one place:
// scripts/release/branding.json. The three release renderers are the only
// writers of published metadata (SPECS §1 names the contract, §2 the manual
// step a test cannot reach), and they had each grown their own literal — five
// descriptions of one product. These pins make a reintroduced literal a red
// test instead of a review catch.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// branding is the shape of scripts/release/branding.json.
type branding struct {
	Title        string `json:"title"`
	RegistryName string `json:"registryName"`
	Agent        string `json:"agent"`
	Binary       string `json:"binary"`
	Line         string `json:"line"`
	Sentence     string `json:"sentence"`
}

var brandingRenderers = []string{
	filepath.Join("scripts", "release", "render-acp-registry.py"),
	filepath.Join("scripts", "release", "render-homebrew-formula.py"),
	filepath.Join("scripts", "release", "render-winget-manifests.py"),
}

// repoFile reads a path relative to the repository root (this package lives in
// cmd/moca).
func repoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", filepath.Join(parts...), err)
	}
	return string(b)
}

func loadBranding(t *testing.T) branding {
	t.Helper()
	var br branding
	if err := json.Unmarshal([]byte(repoFile(t, "scripts", "release", "branding.json")), &br); err != nil {
		t.Fatalf("scripts/release/branding.json is the single source of the product's name and description: %v", err)
	}
	for _, f := range []struct{ name, value string }{
		{"title", br.Title}, {"registryName", br.RegistryName}, {"agent", br.Agent},
		{"binary", br.Binary}, {"line", br.Line}, {"sentence", br.Sentence},
	} {
		if strings.TrimSpace(f.value) == "" {
			t.Fatalf("branding.json: %s must not be empty", f.name)
		}
	}
	return br
}

// TestBrandingFileIsTheOnlySource: every renderer reads branding.json and none
// of them carries a copy of a published string.
func TestBrandingFileIsTheOnlySource(t *testing.T) {
	br := loadBranding(t)
	for _, path := range brandingRenderers {
		src := repoFile(t, path)
		if !strings.Contains(src, "branding.json") {
			t.Errorf("%s does not read scripts/release/branding.json", path)
		}
		for _, f := range []struct{ what, literal string }{
			{"title", br.Title},
			{"registry name", br.RegistryName},
			{"line", br.Line},
			{"sentence", br.Sentence},
		} {
			if strings.Contains(src, f.literal) {
				t.Errorf("%s hardcodes the %s %q; it must come from branding.json", path, f.what, f.literal)
			}
		}
	}
}

// TestBrandingChannelLimits: the line has to satisfy the channels that show
// only a line (Homebrew rejects feature lists and taglines; winget caps a
// short description at 256, which a compliant brew `desc` never reaches), and
// the sentence must open with it so the two cannot drift apart.
func TestBrandingChannelLimits(t *testing.T) {
	br := loadBranding(t)
	if n := len(br.Line); n > 80 {
		t.Errorf("line is %d chars: Homebrew's desc guideline is one line, 80 max", n)
	}
	if strings.HasSuffix(br.Line, ".") {
		t.Error("line ends with a period: Homebrew style wants none")
	}
	if n := len(br.Sentence); n > 350 {
		t.Errorf("sentence is %d chars: the GitHub description caps at 350", n)
	}
	if want := "a " + strings.ToLower(br.Line); !strings.HasPrefix(strings.ToLower(br.Sentence), want) {
		t.Errorf("sentence must open with the line: want prefix %q, got %q", "A "+br.Line, br.Sentence)
	}
	if br.Binary != "moca" {
		t.Errorf("binary is %q: the command, repo, module path, config dirs and release asset names are all built on moca", br.Binary)
	}
}

// TestReadmeTitleMatchesBranding: the README's title and its embedded copy both
// carry branding.json's title. The mirror is byte-copied from the README
// (internal/skills/sync_test.go), so this also fails when only one was edited.
func TestReadmeTitleMatchesBranding(t *testing.T) {
	br := loadBranding(t)
	want := "# " + br.Title
	for _, path := range []string{
		"README.md",
		filepath.Join("internal", "skills", "builtin", "self-knowledge", "references", "readme.md"),
	} {
		first, _, _ := strings.Cut(repoFile(t, path), "\n")
		if first != want {
			t.Errorf("%s opens with %q, want %q", path, first, want)
		}
	}
}

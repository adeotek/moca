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

// TestPromptNamesTheAgent: the agent's own identity line (§14) carries
// branding.json's `agent`. A name in the README while the system prompt says a
// different one is cosmetic — this line is what the model answers "who are
// you" from.
func TestPromptNamesTheAgent(t *testing.T) {
	br := loadBranding(t)
	first, _, _ := strings.Cut(repoFile(t, "internal", "agent", "system-prompt.md"), "\n")
	if want := "You are " + br.Agent + ", "; !strings.HasPrefix(first, want) {
		t.Errorf("system-prompt.md opens with %q, want a prefix of %q", first, want)
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

// TestACPIconContract pins the registry icon: the ACP registry requires a
// 16x16 monochrome currentColor SVG, and docs/assets/icon.svg is the
// full-colour brand art (an embedded raster), so the derived docs/assets/
// icon-acp.svg is the one the release renderer ships. The renderer checks the
// same rules at publish time; this pin catches a regression at CI time.
func TestACPIconContract(t *testing.T) {
	icon := repoFile(t, "docs", "assets", "icon-acp.svg")
	if !strings.Contains(icon, `viewBox="0 0 16 16"`) {
		t.Fatal("viewBox must be 0 0 16 16")
	}
	if !strings.Contains(icon, "currentColor") {
		t.Fatal("must paint with currentColor")
	}
	if strings.Contains(icon, "<image") {
		t.Fatal("must be vector paths, not an embedded raster")
	}
	if len(icon) > 8192 {
		t.Fatalf("%d bytes is too large for a registry icon", len(icon))
	}
	renderer := repoFile(t, "scripts", "release", "render-acp-registry.py")
	if !strings.Contains(renderer, "icon-acp.svg") {
		t.Fatal("the ACP renderer must ship the derived icon, not the brand art")
	}
}

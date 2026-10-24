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
	"regexp"
	"sort"
	"strconv"
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

// The ACP registry's icon rules, as its own validator applies them
// (.github/workflows/build_registry.py in agentclientprotocol/registry):
// 16x16, vector, monochrome through currentColor, no XML comments (icons are
// embedded in MDX there), small enough for a manifest. The size may ride on
// width/height OR on the viewBox, and the registry reads width/height first —
// which is what lets the brand mark (512 units of geometry) ship at 16x16.
// render-acp-registry.py's icon_problems checks the same rules with the same
// wording at publish time; this pin catches a regression at CI time.
func TestACPIconContract(t *testing.T) {
	icon := repoFile(t, "docs", "assets", "icon-acp.svg")
	if strings.Contains(icon, "<!--") {
		t.Fatal("must not contain XML comments — the ACP registry rejects them (icons are embedded in MDX)")
	}
	w, h, ok := iconSize(icon)
	switch {
	case !ok:
		t.Fatal(`needs width/height="16" or a viewBox`)
	case w != h:
		t.Fatalf("must be square (got %gx%g)", w, h)
	case w != 16 || h != 16:
		t.Fatalf(`must be 16x16 (got %gx%g) — declare width/height="16" if the viewBox is larger`, w, h)
	}
	inked, hardcoded := iconInk(icon)
	if len(hardcoded) > 0 {
		t.Fatalf("must paint with currentColor or none, not a hardcoded colour: %s", strings.Join(hardcoded, ", "))
	}
	if !inked {
		t.Fatal("must paint with currentColor (monochrome)")
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

var (
	iconRootTag = regexp.MustCompile(`(?is)<svg\b[^>]*>`)
	iconLenRe   = regexp.MustCompile(`(?i)^\s*([0-9]*\.?[0-9]+)\s*(px)?\s*$`)
	iconInkRe   = regexp.MustCompile(`(?i)\b(fill|stroke)\s*=\s*"([^"]*)"`)
	iconStyleRe = regexp.MustCompile(`(?i)\bstyle\s*=\s*"([^"]*)"`)
	iconPropRe  = regexp.MustCompile(`(?i)(fill|stroke)\s*:\s*([^;]+)`)
)

// iconSize mirrors the registry's precedence: width/height when both parse as
// pixels, the viewBox's 3rd and 4th numbers otherwise.
func iconSize(icon string) (w, h float64, ok bool) {
	root := iconRootTag.FindString(icon)
	if root == "" {
		return 0, 0, false
	}
	w, wok := iconLength(iconAttr(root, "width"))
	h, hok := iconLength(iconAttr(root, "height"))
	if wok && hok {
		return w, h, true
	}
	f := strings.FieldsFunc(iconAttr(root, "viewBox"), func(r rune) bool {
		return r == ' ' || r == ',' || r == '	' || r == '\n' || r == '\r'
	})
	if len(f) != 4 {
		return 0, 0, false
	}
	x, errX := strconv.ParseFloat(f[2], 64)
	y, errY := strconv.ParseFloat(f[3], 64)
	if errX != nil || errY != nil {
		return 0, 0, false
	}
	return x, y, true
}

func iconAttr(tag, name string) string {
	m := regexp.MustCompile(`(?i)\b` + name + `\s*=\s*"([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1]
}

// iconLength reads a length attribute as pixels: "16" and "16px" are both 16,
// as the registry strips the unit.
func iconLength(v string) (float64, bool) {
	m := iconLenRe.FindStringSubmatch(v)
	if m == nil {
		return 0, false
	}
	px, err := strconv.ParseFloat(m[1], 64)
	return px, err == nil
}

// iconInk walks every fill/stroke, attribute or inline style, and reports
// whether the icon paints with currentColor and which hardcoded values break
// theming — currentColor, none and inherit are the only legal inks.
func iconInk(icon string) (currentColor bool, hardcoded []string) {
	seen := map[string]bool{}
	classify := func(prop, value string) {
		switch v := strings.ToLower(strings.TrimSpace(value)); v {
		case "currentcolor":
			currentColor = true
		case "none", "inherit":
		default:
			if !seen[prop+value] {
				seen[prop+value] = true
				hardcoded = append(hardcoded, prop+`="`+strings.TrimSpace(value)+`"`)
			}
		}
	}
	for _, m := range iconInkRe.FindAllStringSubmatch(icon, -1) {
		classify(m[1], m[2])
	}
	for _, m := range iconStyleRe.FindAllStringSubmatch(icon, -1) {
		for _, p := range iconPropRe.FindAllStringSubmatch(m[1], -1) {
			classify(p[1], p[2])
		}
	}
	sort.Strings(hardcoded)
	return currentColor, hardcoded
}

package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestThemeBuilderMatchesDefaults pins docs/tui-theme-builder.html to
// DefaultTheme: every themable field's preview default equals the built-in
// value — or, for a role whose default is an ANSI slot, carries that slot —
// and every config role has a builder field. Exported snippets then never
// pin a stale default.
func TestThemeBuilderMatchesDefaults(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "tui-theme-builder.html"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start, end := strings.Index(src, "const PALETTES = {"), strings.Index(src, "const GROUPS")
	if start < 0 || end < start {
		t.Fatal("PALETTES/GROUPS blocks not found")
	}
	pal := map[string]map[string]string{}
	for _, m := range regexp.MustCompile(`(?s)(dark|light): \{(.*?)\}`).FindAllStringSubmatch(src[start:end], -1) {
		kv := map[string]string{}
		for _, p := range regexp.MustCompile(`(\w+):"(#[0-9a-fA-F]{6})"`).FindAllStringSubmatch(m[2], -1) {
			kv[p[1]] = p[2]
		}
		pal[m[1]] = kv
	}
	if len(pal) != 2 {
		t.Fatalf("want dark+light palettes, got %v", pal)
	}
	fields := regexp.MustCompile(`\{k:"(\w+)", label:"[^"]*", role:(?:"(\w+)"|null)(?:, ansi:"(\d+)")?\}`).FindAllStringSubmatch(src, -1)
	seen := map[string]bool{}
	for _, variant := range []string{"dark", "light"} {
		c := DefaultTheme().colors(variant == "dark")
		byKey := map[string]string{}
		rv, rt := reflect.ValueOf(c), reflect.TypeOf(c)
		for i := range rt.NumField() {
			byKey[strings.TrimSuffix(rt.Field(i).Tag.Get("json"), ",omitempty")] = rv.Field(i).String()
		}
		for _, f := range fields {
			k, role, slot := f[1], f[2], f[3]
			if role == "" {
				continue
			}
			seen[role] = true
			want, ok := byKey[role]
			switch {
			case !ok:
				t.Errorf("builder role %q is not a config key", role)
			case slot != "":
				if slot != want {
					t.Errorf("%s %s: builder ANSI slot %s, built-in %s", variant, role, slot, want)
				}
			case !strings.EqualFold(pal[variant][k], want):
				t.Errorf("%s %s (%s): builder %s, built-in %s", variant, role, k, pal[variant][k], want)
			}
		}
		for key := range byKey {
			if !seen[key] {
				t.Errorf("config role %q has no builder field", key)
			}
		}
	}
}

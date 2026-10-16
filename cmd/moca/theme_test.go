package main

import (
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tui"
)

// The theme is the built-in palette overlaid by the user config's tui.theme
// (§3.5) — the only layer: a project file cannot carry colors (§4).
func TestThemeForUserLayer(t *testing.T) {
	cfg := config.Config{TUI: config.TUIConfig{Theme: config.ThemeConfig{
		Dark: config.ThemeColors{UserBg: "#010203"},
		Both: config.ThemeColors{Accent: "9"},
	}}}
	cfg.Project = &config.ProjectConfig{} // a trusted project contributes nothing
	th := themeFor(cfg)
	if th.Dark.UserBg != "#010203" {
		t.Fatalf("user dark override lost: %q", th.Dark.UserBg)
	}
	if th.Light.UserBg != tui.DefaultTheme().Light.UserBg {
		t.Fatalf("the dark override must not leak into light: %q", th.Light.UserBg)
	}
	if th.Dark.Accent != "9" || th.Light.Accent != "9" {
		t.Fatalf("both must fill both variants: %q / %q", th.Dark.Accent, th.Light.Accent)
	}
}

package main

import (
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// The theme layer order (§3.5, §4): built-in defaults < the user config's
// tui.theme < a trusted project's tui.theme — the project wins per field.
func TestThemeForUserThenProject(t *testing.T) {
	cfg := config.Config{TUI: config.TUIConfig{Theme: config.ThemeConfig{
		Dark: config.ThemeColors{UserBg: "#010203"},
	}}}
	if th := themeFor(cfg); th.Dark.UserBg != "#010203" {
		t.Fatalf("user layer alone: %q", th.Dark.UserBg)
	}
	cfg.Project = &config.ProjectConfig{TUI: config.ProjectTUIConfig{Theme: config.ThemeConfig{
		Dark: config.ThemeColors{UserBg: "#040506"},
		Both: config.ThemeColors{Accent: "9"},
	}}}
	th := themeFor(cfg)
	if th.Dark.UserBg != "#040506" {
		t.Fatalf("project must win: %q", th.Dark.UserBg)
	}
	if th.Light.UserBg == "#010203" {
		t.Fatalf("the user's dark override must not leak into light: %q", th.Light.UserBg)
	}
	if th.Dark.Accent != "9" || th.Light.Accent != "9" {
		t.Fatalf("project both: %q / %q", th.Dark.Accent, th.Light.Accent)
	}
}

package tui

// theme.go is the ONE place the TUI's colors live (§3.5, §4): every
// scrollback band, notice and status-bar role reads its color from a Theme
// value. DefaultTheme is the built-in palette; config files overlay
// non-empty fields via WithOverrides — the user config's `tui.theme` first,
// then a trusted project's `.moca/config.jsonc` `tui.theme`, which wins
// (§4). cmd/moca resolves the theme before the model exists and passes it
// in through AppOptions.
//
// Colors field names mirror config.ThemeColors one-to-one (a test pins the
// two lists); values are #rrggbb hexes or ANSI color numbers ("1", "208"),
// the same forms config validates. The dark and light palettes exist because
// moca picks a variant from the terminal's reported background
// (tea.BackgroundColorMsg); dark is the default until it arrives.

import (
	"fmt"
	"reflect"
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
)

// Colors is one variant's full palette: the scrollback bands and notice
// colors plus the status bar's role table.
type Colors struct {
	// Scrollback bands and notices (message_style.go, markdown.go).
	UserBg  string
	UserFg  string
	RespBg  string
	RespFg  string
	CodeBg  string
	CodeFg  string
	Muted   string // thinking/tool/notice lines
	Error   string // error: lines
	Warning string // warning: lines, `?` prompts
	Accent  string // welcome product name, pager selection
	Plan    string // plan-mode notice
	DiffAdd string // edit-diff added lines
	DiffDel string // edit-diff removed lines
	// Status bar (status_style.go). StatusMode also colors the Default badge.
	StatusMode   string
	StatusModel  string
	StatusEffort string
	StatusPct    string
	StatusHint   string
	StatusCwd    string
	StatusBranch string
	StatusCost   string
	StatusMuted  string
	StatusSep    string
	BadgeAuto    string
	BadgePlan    string
	BadgeYolo    string
	PctWarn      string // context percent ≥ 70% of the window
	PctHot       string // percent at/past the auto-compaction trigger
}

// Theme is the dark and light palettes.
type Theme struct{ Dark, Light Colors }

// DefaultTheme is the built-in palette (the pre-theme package colors, moved
// here verbatim).
func DefaultTheme() Theme {
	return Theme{
		Dark: Colors{
			UserBg: "#141618", UserFg: "#edf0f4",
			RespBg: "#091215", RespFg: "#e9f1fc",
			CodeBg: "#0c222b", CodeFg: "#f2c57c",
			Muted: "#96a0a4", Error: "1", Warning: "3",
			Accent: "#d97706", Plan: "4",
			DiffAdd: "2", DiffDel: "1",
			StatusMode: "#c9d1d9", StatusModel: "#60a5fa", StatusEffort: "#c4b5fd",
			StatusPct: "#e6edf3", StatusHint: "#f87171", StatusCwd: "#67e8f9",
			StatusBranch: "#f0abfc", StatusCost: "#fbbf24", StatusMuted: "#9aa4ab",
			StatusSep: "#5b636b",
			BadgeAuto: "#4ade80", BadgePlan: "#60a5fa", BadgeYolo: "#f87171",
			PctWarn: "#facc15", PctHot: "#f87171",
		},
		Light: Colors{
			UserBg: "#dfe3e8", UserFg: "#1c2026",
			RespBg: "#c9ddfb", RespFg: "#17304d",
			CodeBg: "#b6cdee", CodeFg: "#7a3e00",
			Muted: "#96a0a4", Error: "1", Warning: "3",
			Accent: "#d97706", Plan: "4",
			DiffAdd: "2", DiffDel: "1",
			StatusMode: "#3b434b", StatusModel: "#1d4ed8", StatusEffort: "#6d28d9",
			StatusPct: "#1f2328", StatusHint: "#b91c1c", StatusCwd: "#0e7490",
			StatusBranch: "#a21caf", StatusCost: "#b45309", StatusMuted: "#57606a",
			StatusSep: "#9ea7b0",
			BadgeAuto: "#15803d", BadgePlan: "#1d4ed8", BadgeYolo: "#b91c1c",
			PctWarn: "#a16207", PctHot: "#b91c1c",
		},
	}
}

// WithOverrides overlays a config theme block (config.ThemeConfig) onto the
// palette: per variant and per field, the variant's block wins, then the
// variant-agnostic `both` block; empty fields keep the current value. cmd
// applies the user layer first and a trusted project's second, so the
// project layer wins where it sets a key (§4).
func (t Theme) WithOverrides(o config.ThemeConfig) Theme {
	t.Dark = overlayColors(t.Dark, o.Dark, o.Both)
	t.Light = overlayColors(t.Light, o.Light, o.Both)
	return t
}

// overlayColors applies one variant's overrides (variant-specific first,
// then `both`) field-by-field, matched by field name.
func overlayColors(c Colors, variant, both config.ThemeColors) Colors {
	ov, bo := stringsByName(variant), stringsByName(both)
	rv := reflect.ValueOf(&c).Elem()
	rt := rv.Type()
	for i := range rt.NumField() {
		if s := ov[rt.Field(i).Name]; s != "" {
			rv.Field(i).SetString(s)
		} else if s := bo[rt.Field(i).Name]; s != "" {
			rv.Field(i).SetString(s)
		}
	}
	return c
}

// stringsByName flattens a config colors block to a name → value map (the
// field names match Colors' by contract; a test pins that).
func stringsByName(v config.ThemeColors) map[string]string {
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	m := make(map[string]string, rt.NumField())
	for i := range rt.NumField() {
		m[rt.Field(i).Name] = rv.Field(i).String()
	}
	return m
}

// colors picks the variant's palette.
func (t Theme) colors(dark bool) Colors {
	if dark {
		return t.Dark
	}
	return t.Light
}

// resolvedTheme falls back to the built-in palette for a zero theme (callers
// that pass none — tests, and any path predating the theme).
func resolvedTheme(t Theme) Theme {
	if t.Dark == (Colors{}) && t.Light == (Colors{}) {
		return DefaultTheme()
	}
	return t
}

// fgSGR is the raw SGR sequence that sets a foreground color. The markdown
// inline-code span uses it because spans are raw toggles (the response band
// must survive a wrap, so never a lipgloss full reset). #rrggbb becomes
// truecolor, an ANSI number its palette form; values are config-validated,
// and an unparseable one falls back to the terminal default.
func fgSGR(color string) string {
	if len(color) == 7 && color[0] == '#' {
		if v, err := strconv.ParseUint(color[1:], 16, 32); err == nil {
			return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", v>>16&0xff, v>>8&0xff, v&0xff)
		}
	}
	if n, err := strconv.Atoi(color); err == nil {
		switch {
		case n >= 0 && n < 8:
			return fmt.Sprintf("\x1b[%dm", 30+n)
		case n < 16:
			return fmt.Sprintf("\x1b[%dm", 90+n-8)
		case n <= 255:
			return fmt.Sprintf("\x1b[38;5;%dm", n)
		}
	}
	return sgrFgOff
}

// Style builders. Each resolves the variant's color and returns a fresh
// style (cheap value types; the model picks the variant per render, so a
// background flip mid-session takes effect immediately).

// userMsgStyle is the submitted-message band (§3.5).
func (t Theme) userMsgStyle(dark bool) lipgloss.Style {
	c := t.colors(dark)
	return lipgloss.NewStyle().Background(lipgloss.Color(c.UserBg)).Foreground(lipgloss.Color(c.UserFg))
}

// respMsgStyle is the streamed-response band (§3.5).
func (t Theme) respMsgStyle(dark bool) lipgloss.Style {
	c := t.colors(dark)
	return lipgloss.NewStyle().Background(lipgloss.Color(c.RespBg)).Foreground(lipgloss.Color(c.RespFg))
}

// codeBandStyle is the fenced-code band inside a response (§3.5.1).
func (t Theme) codeBandStyle(dark bool) lipgloss.Style {
	c := t.colors(dark)
	return lipgloss.NewStyle().Background(lipgloss.Color(c.CodeBg)).Foreground(lipgloss.Color(c.RespFg))
}

// codeFg is the raw SGR of the inline-code foreground.
func (t Theme) codeFg(dark bool) string { return fgSGR(t.colors(dark).CodeFg) }

func (t Theme) mutedStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.colors(dark).Muted))
}

func (t Theme) errorStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.colors(dark).Error))
}

func (t Theme) warningStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.colors(dark).Warning))
}

// accentStyle marks the product name in the welcome line and the pager
// selection; bold at both sites.
func (t Theme) accentStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.colors(dark).Accent)).Bold(true)
}

// planStyle is the plan-mode notice color; bold.
func (t Theme) planStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.colors(dark).Plan)).Bold(true)
}

func (t Theme) diffAddStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.colors(dark).DiffAdd))
}

func (t Theme) diffDelStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.colors(dark).DiffDel))
}

// badgeStyle styles the mode badge: the mode's tier-1 color, bold — the
// safety state must never be missed. Plan tags the blue plan color (the
// Plan badge of a default run under the plan scope).
func (t Theme) badgeStyle(mode agent.PermMode, plan, dark bool) lipgloss.Style {
	c := t.colors(dark)
	hex := c.StatusMode
	switch mode {
	case agent.ModeAuto:
		hex = c.BadgeAuto
	case agent.ModeYolo:
		hex = c.BadgeYolo
	}
	if mode == agent.ModeYolo {
		plan = false // yolo never carries plan (the agent excludes it); keep the red
	}
	if plan {
		hex = c.BadgePlan
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true)
}

// autoBadgeStyle is the green Auto badge for the variant (the one style a
// test pins).
func (t Theme) autoBadgeStyle(dark bool) lipgloss.Style {
	return t.badgeStyle(agent.ModeAuto, false, dark)
}

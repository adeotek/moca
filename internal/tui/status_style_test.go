package tui

import (
	"fmt"
	"math"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/adeotek/moca/internal/agent"
	"github.com/charmbracelet/x/ansi"
)

func TestStatusPaletteContrast(t *testing.T) {
	bgs := map[bool][]string{true: {"#0d1117", "#1e1e1e"}, false: {"#ffffff", "#f6f8fa"}}
	for dark, list := range bgs {
		p := statusPalette(dark)
		for r, hex := range p.fg {
			if r == roleSep {
				continue // decorative: exempt from the 4.5:1 bar, must recede
			}
			for _, bg := range list {
				if c := contrast(hex, bg); c < 4.5 {
					t.Errorf("dark=%v role %v %s on %s = %.2f:1, want ≥ 4.5", dark, r, hex, bg, c)
				}
			}
		}
		for _, bg := range list {
			if contrast(p.fg[roleSep], bg) >= contrast(p.fg[roleMuted], bg) {
				t.Errorf("dark=%v: separators must recede behind tier-4 text on %s", dark, bg)
			}
		}
	}
}

// contrast is the WCAG 2.x ratio of two #rrggbb colors (test helper).
func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(hex string) float64 {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	ch := func(v int) float64 {
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(r) + 0.7152*ch(g) + 0.0722*ch(b)
}

// TestStatusSegmentsKeepRolesWhenTruncated: shrinking to a narrow bar drops
// whole fields — the fields that survive keep their roles, so per-field
// styling stays exact after truncation (the string-search bug styleL2 had).
func TestStatusSegmentsKeepRolesWhenTruncated(t *testing.T) {
	s := StatusInfo{Version: "v0.4.0-beta", Cwd: "~/p/moca", Branch: "main", Git: true, Model: "anthropic/claude-sonnet-5-5",
		Effort: "high", Window: 200000, Used: 150000, Trigger: 183616, In: 1200, Out: 300, Cost: 0.12}
	_, l2 := RenderStatus(s, 30)
	var sawModel, sawPct bool
	for _, seg := range l2 {
		sawModel = sawModel || seg.role == roleModel
		sawPct = sawPct || seg.role == rolePct
	}
	if !sawModel || !sawPct {
		t.Fatalf("model and percent must survive a narrow bar: %+v", l2)
	}
	if w := lipgloss.Width(plainLine(l2)); w > 30 {
		t.Fatalf("width %d > 30", w)
	}
}

func TestStyledStatusSameTextAsPlain(t *testing.T) {
	// ansi.Strip(styleLine(segs, dark, p)) == plainLine(segs) for both themes
	// and all pressures: styling may never change the visible text.
	for _, dark := range []bool{true, false} {
		for p := pressureOK; p <= pressureHot; p++ {
			segs := []segment{
				{text: "AUTO", role: roleMode, drop: 0}, {text: "v0.4.0", role: roleMuted, drop: 0},
				{text: "25%", role: rolePct, drop: 0}, {text: "/compact", role: roleHint, drop: 2},
			}
			plain := plainLine(segs)
			if got := ansi.Strip(styleLine(segs, dark, p)); got != plain {
				t.Fatalf("dark=%v pressure=%d: %q != %q", dark, p, got, plain)
			}
		}
	}
}

func TestYoloBadgeNeverPlanBlue(t *testing.T) {
	// No mode ever renders combined with plan: a plan bool reaching a yolo
	// (or auto) badge style must not turn it blue (state layer normalizes
	// plan to its own badge; this is the style-layer mirror — pin #f87171's
	// RGB against the plan blue #60a5fa).
	st := badgeStyle(agent.ModeYolo, true, true)
	got := st.Render("Yolo")
	if exp := lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true).Render("Yolo"); got != exp {
		t.Errorf("yolo badge with plan=true = %q, want the red style %q", got, exp)
	}
}

// The model field shares the plan blue exactly (Ben, 2026-10-12) — pinned
// per theme so a future palette edit cannot silently split them.
func TestModelRendersPlanBlue(t *testing.T) {
	for _, dark := range []bool{true, false} {
		p := statusPalette(dark)
		if p.fg[roleModel] != p.plan {
			t.Errorf("dark=%v: model %s != plan blue %s", dark, p.fg[roleModel], p.plan)
		}
	}
}

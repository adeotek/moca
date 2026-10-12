package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
)

// palette is the importance-ranked status-bar color table for one theme
// (plan Task 10): every content role reaches WCAG ≥ 4.5:1 against the
// reference backgrounds (#0d1117/#1e1e1e dark, #ffffff/#f6f8fa light —
// contrast-tested in status_style_test.go); separators are decorative and
// recede below tier-4 text; higher tiers render bold.
type palette struct {
	fg    map[role]string           // per-role text color (#rrggbb)
	badge map[agent.PermMode]string // mode-badge colors: Default/Auto/Yolo
	plan  string                    // the Plan badge (a default run under the plan scope)
	warn  lipgloss.Style            // percent at ≥ 70% of the window
	hot   lipgloss.Style            // percent at/past the auto-compact trigger (bold)
	sep   lipgloss.Style            // " · " separators (structure, not content)
}

// Hexes are the plan's table verbatim; warn/hot color the percent segment by
// pressure (the /compact hint uses the hot color via roleHint).
func statusPalette(dark bool) palette {
	if dark {
		return palette{
			fg: map[role]string{
				roleMode:   "#c9d1d9",
				roleModel:  "#60a5fa", // tier 2: the plan blue (Ben, 2026-10-12)
				roleEffort: "#c4b5fd",
				rolePct:    "#e6edf3",
				roleHint:   "#f87171",
				roleCwd:    "#67e8f9",
				roleBranch: "#f0abfc",
				roleCost:   "#fbbf24",
				roleMuted:  "#9aa4ab",
				roleSep:    "#5b636b",
			},
			badge: map[agent.PermMode]string{
				agent.ModeDefault: "#c9d1d9",
				agent.ModeAuto:    "#4ade80",
				agent.ModeYolo:    "#f87171",
			},
			plan: "#60a5fa",
			warn: lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")),
			hot:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true),
			sep:  lipgloss.NewStyle().Foreground(lipgloss.Color("#5b636b")),
		}
	}
	return palette{
		fg: map[role]string{
			roleMode:   "#3b434b",
			roleModel:  "#1d4ed8", // tier 2: the plan blue (Ben, 2026-10-12)
			roleEffort: "#6d28d9",
			rolePct:    "#1f2328",
			roleHint:   "#b91c1c",
			roleCwd:    "#0e7490",
			roleBranch: "#a21caf",
			roleCost:   "#b45309",
			roleMuted:  "#57606a",
			roleSep:    "#9ea7b0",
		},
		badge: map[agent.PermMode]string{
			agent.ModeDefault: "#3b434b",
			agent.ModeAuto:    "#15803d",
			agent.ModeYolo:    "#b91c1c",
		},
		plan: "#1d4ed8",
		warn: lipgloss.NewStyle().Foreground(lipgloss.Color("#a16207")),
		hot:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b91c1c")).Bold(true),
		sep:  lipgloss.NewStyle().Foreground(lipgloss.Color("#9ea7b0")),
	}
}

// boldRoles are the tiers that render bold (tier 1 mode badge, tier 2
// model — which colors in the plan blue, like the Plan badge); the
// percent's bold comes only with the hot pressure style.
var boldRoles = map[role]bool{roleMode: true, roleModel: true}

// roleStyle is the style for a role's text: the palette's hex foreground,
// Bold per the tier table, nothing else.
func (p palette) roleStyle(r role) lipgloss.Style {
	st := lipgloss.NewStyle().Foreground(lipgloss.Color(p.fg[r]))
	if boldRoles[r] {
		st = st.Bold(true)
	}
	return st
}

// badgeStyle styles the mode badge: the mode's tier-1 palette color, bold —
// the safety state must never be missed. Plan tags the blue plan color (the
// Plan badge of a default run under the plan scope). dark is the theme
// the caller wants (tests pass an explicit theme; the model passes m.darkBG).
func badgeStyle(mode agent.PermMode, plan, dark bool) lipgloss.Style {
	p := statusPalette(dark)
	hex := p.badge[mode]
	if mode == agent.ModeYolo {
		plan = false // yolo never carries plan (the agent excludes it); keep the red
	}
	if plan {
		hex = p.plan
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Bold(true)
}

// autoBadgeStyle is the green AUTO badge (the one style a test pins): always
// the dark-theme table, since that is the canonical green.
func autoBadgeStyle(dark bool) lipgloss.Style {
	return badgeStyle(agent.ModeAuto, false, dark)
}

// separatorStyle is the " · " join style for the theme (decorative: lower
// contrast than tier-4 text, so it recedes).
func separatorStyle(dark bool) lipgloss.Style {
	return statusPalette(dark).sep
}

// styleLine is the styled form of a status line: every segment carries its
// role's color (Bold per the tier table, nothing else), joined with the
// styled separator, and the percent segment is swapped to the warn/hot
// pressure style. The visible text is plainLine(segs) exactly — styling may
// never change it (tested). Segment text is sanitized before render (the
// §3.5 print-path invariant the old statusLine/styleL2 carried): control
// bytes in a status field display as ^X, never act.
func styleLine(segs []segment, dark bool, pressure int) string {
	p := statusPalette(dark)
	sepSt := p.sep
	var out []string
	for _, seg := range segs {
		if seg.text == "" {
			continue
		}
		if len(out) > 0 {
			out = append(out, sepSt.Render(" · "))
		}
		var st lipgloss.Style
		switch {
		case seg.role == roleMode && (seg.badgeMode != "" || seg.badgePlan):
			// Lead badge: the exact tier-1 hue for its mode/plan state.
			st = badgeStyle(seg.badgeMode, seg.badgePlan, dark)
		default:
			st = p.roleStyle(seg.role)
		}
		if seg.role == rolePct {
			switch pressure {
			case pressureHot:
				st = p.hot
			case pressureWarn:
				st = p.warn
			}
		}
		out = append(out, st.Render(Sanitize(seg.text)))
	}
	return strings.Join(out, "")
}

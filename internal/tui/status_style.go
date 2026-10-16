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

// statusPalette builds the table from the Theme (theme.go; the built-in
// values match the plan's original table verbatim): every content role
// reaches WCAG ≥ 4.5:1 against the reference backgrounds
// (#0d1117/#1e1e1e dark, #ffffff/#f6f8fa light — contrast-tested in
// status_style_test.go); separators are decorative and recede below tier-4
// text; warn/hot color the percent segment by pressure (the /compact hint
// uses the hot color via roleHint).
func (t Theme) statusPalette(dark bool) palette {
	c := t.colors(dark)
	return palette{
		fg: map[role]string{
			roleMode:   c.StatusMode,
			roleModel:  c.StatusModel, // tier 2: the plan blue (Ben, 2026-10-12)
			roleEffort: c.StatusEffort,
			rolePct:    c.StatusPct,
			roleHint:   c.StatusHint,
			roleCwd:    c.StatusCwd,
			roleBranch: c.StatusBranch,
			roleCost:   c.StatusCost,
			roleMuted:  c.StatusMuted,
			roleSep:    c.StatusSep,
		},
		badge: map[agent.PermMode]string{
			agent.ModeDefault: c.StatusMode,
			agent.ModeAuto:    c.BadgeAuto,
			agent.ModeYolo:    c.BadgeYolo,
		},
		plan: c.BadgePlan,
		warn: lipgloss.NewStyle().Foreground(lipgloss.Color(c.PctWarn)),
		hot:  lipgloss.NewStyle().Foreground(lipgloss.Color(c.PctHot)).Bold(true),
		sep:  lipgloss.NewStyle().Foreground(lipgloss.Color(c.StatusSep)),
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

// styleLine is the styled form of a status line: every segment carries its
// role's color (Bold per the tier table, nothing else), joined with the
// styled separator, and the percent segment is swapped to the warn/hot
// pressure style. The visible text is plainLine(segs) exactly — styling may
// never change it (tested). Segment text is sanitized before render (the
// §3.5 print-path invariant the old statusLine/styleL2 carried): control
// bytes in a status field display as ^X, never act.
func (t Theme) styleLine(segs []segment, dark bool, pressure int) string {
	p := t.statusPalette(dark)
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
			st = t.badgeStyle(seg.badgeMode, seg.badgePlan, dark)
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

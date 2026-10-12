package tui

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
)

// StatusInfo is the plain-text status bar content (§11); each field carries a
// role (styled per theme by styleLine in status_style.go). Line 1: version ·
// cwd · branch. Line 2: model · effort · ctx W · P% · in/out · cost.
type StatusInfo struct {
	Version      string
	Cwd, Branch  string
	Git, Dirty   bool
	Model        string
	Effort       llm.Effort
	Window, Used int
	In, Out      int
	Cost         float64
	Sub          bool
	Transient    string
	// Trigger is the context size at which auto-compaction fires (the
	// window minus the reserve); the percent field turns red from there.
	Trigger int
}

// Context pressure levels of the percent field.
const (
	pressureOK   = iota
	pressureWarn // ≥ 70% of the window
	pressureHot  // at or past the auto-compaction trigger
)

// pressure rates how full the context is.
func (s StatusInfo) pressure() int {
	switch {
	case s.Window <= 0 || s.Used <= 0:
		return pressureOK
	case s.Trigger > 0 && s.Used >= s.Trigger:
		return pressureHot
	case s.Used*100 >= s.Window*70:
		return pressureWarn
	}
	return pressureOK
}

// role is what a status-bar segment is — the styling key (status_style.go
// maps role → color per theme, importance-ranked: mode and the context
// percent must never be missed; separators and tier-4 text recede).
type role int

const (
	roleSep role = iota
	roleMode
	roleModel
	roleEffort
	rolePct
	roleHint
	roleCwd
	roleBranch
	roleCost
	roleMuted
)

// segment is one status-bar field: its text, its role (what it is → how
// styleLine styles it) and its shrink rank — drop 1 drops first, 0 never
// drops. Badge segments (roleMode lead badges) carry their exact
// mode/plan state so the tier-1 badge hue is per state, not per role.
type segment struct {
	text string
	role role
	drop int
	// badge state (roleMode lead badges only): badgeMode colors the badge
	// for its mode, and badgePlan switches to the plan blue (the PLAN badge
	// of the auto+plan split; never set for yolo — yolo never carries plan).
	badgeMode agent.PermMode
	badgePlan bool
}

// plainLine joins the segments' text with " · " — the plain spec of the bar.
// Styling must never change this text; the width math reads it.
func plainLine(segs []segment) string {
	var parts []string
	for _, seg := range segs {
		if seg.text != "" {
			parts = append(parts, seg.text)
		}
	}
	return strings.Join(parts, " · ")
}

// RenderStatus renders the two-line bar as segment lists (plain text =
// plainLine of each). Neither line's plain text is ever wider than width
// terminal cells (wide runes count 2) and neither wraps.
// Line 1 shrink order: drop cwd, then branch, then truncate the version.
// Line 2 shrink order: drop in/out, then truncate the model, then truncate.
func RenderStatus(s StatusInfo, width int) (l1, l2 []segment) {
	branch := "-"
	if s.Git {
		branch = s.Branch
		if s.Dirty {
			branch += "*"
		}
	}
	cost := fmt.Sprintf("$%.4f", s.Cost)
	if s.Sub {
		cost = "sub"
	}
	if s.Transient != "" {
		cost = s.Transient
	}
	l1 = renderLine([]segment{{text: s.Version, role: roleMuted}, {text: s.Cwd, role: roleCwd, drop: 1}, {text: branch, role: roleBranch, drop: 2}}, width, 0)
	if s.Model == "" {
		// No session yet (moca started without a model, §3.5): the bar
		// carries the state instead of an empty model/effort pair.
		return l1, renderLine([]segment{{text: "no provider configured — /login", role: roleMuted}}, width, 0)
	}
	hint := ""
	if s.pressure() == pressureHot {
		hint = "/compact" // the bar says what to do about a full context
	}
	l2 = renderLine([]segment{
		{text: s.Model, role: roleModel}, {text: string(AbbrevEffort(s.Effort)), role: roleEffort},
		{text: "ctx " + FmtWindow(s.Window), role: roleMuted}, {text: FmtPercent(s.Used, s.Window), role: rolePct}, {text: hint, role: roleHint, drop: 2},
		{text: FmtTokens(s.In) + "/" + FmtTokens(s.Out), role: roleMuted, drop: 3}, {text: cost, role: roleCost},
	}, width, 0)
	return l1, l2
}

// FmtPercent renders context usage as a percentage of the window: whole
// numbers from 10% up, one decimal below that (a 4k conversation in a 1M
// window is 0.4%, not a stuck "0%"), and "<0.1%" for a sliver.
func FmtPercent(used, window int) string {
	if window <= 0 || used <= 0 {
		return "0%"
	}
	p := float64(used) * 100 / float64(window)
	switch r := math.Round(p*10) / 10; {
	case p < 0.05:
		return "<0.1%"
	case r < 10:
		return fmt.Sprintf("%.1f%%", r)
	}
	return fmt.Sprintf("%d%%", int(math.Round(p)))
}

// renderLine shrinks the segments so their " · " join fits width cells:
// segments drop in rank order 1→3 (skipping what is no longer needed), the
// segment at truncAt gives up what is still over as an ellipsis, and a
// last-resort cut clips the widest shrinkable segment so the line fits.
// Dropping and truncating edit the segment texts — a surviving (possibly
// shortened) segment keeps its role, so styling stays exact.
func renderLine(segs []segment, width, truncAt int) []segment {
	// widthOf is the joined width of the line as it stands.
	widthOf := func() int {
		var w int
		first := true
		for _, seg := range segs {
			if seg.text == "" {
				continue
			}
			if !first {
				w += lipgloss.Width(" · ")
			}
			first = false
			w += lipgloss.Width(seg.text)
		}
		return w
	}
	// Drop ranks 1..3: drop the rank's segments whenever the line is still
	// over width (the old renderer's exact loop — unconditional per rank,
	// with the fit check in the loop condition).
	for drop := 1; drop <= 3 && widthOf() > width; drop++ {
		for i := range segs {
			if segs[i].drop == drop {
				segs[i].text = ""
			}
		}
	}
	// The truncAt segment (rank 0, never dropped) gives up `over` cells plus
	// one for the ellipsis.
	if over := widthOf() - width; over > 0 && truncAt >= 0 {
		segs[truncAt].text = truncCells(segs[truncAt].text, max(1, lipgloss.Width(segs[truncAt].text)-over-1)) + "…"
	}
	// Last resort: still over after dropping and truncating (wide runes,
	// many narrow segments). Drop trailing segments (rank order reversed:
	// the tail is context) while the line is over width — deterministic,
	// no per-rune shrink loop.
	for widthOf() > width {
		any := false
		for i := len(segs) - 1; i >= 0; i-- {
			if i == truncAt && truncAt >= 0 {
				continue // the truncated lead: shrunk already above
			}
			if segs[i].text != "" {
				segs[i].text = ""
				any = true
				break
			}
		}
		if !any {
			break
		}
	}
	return segs
}

// truncCells cuts s to at most w terminal cells (wide runes count 2).
func truncCells(s string, w int) string {
	var sb strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w {
			break
		}
		sb.WriteRune(r)
		used += rw
	}
	return sb.String()
}

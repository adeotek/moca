package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/llm"
)

// StatusInfo is the plain-text status bar content (§11); styling is the
// caller's job. Line 1: version · cwd · branch. Line 2: model · effort ·
// ctx W · P% · in/out · cost.
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
}

// field is one status-bar field; drop is its shrink rank — rank 1 drops
// first, rank 0 never drops.
type field struct {
	text string
	drop int
}

// RenderStatus renders the two-line bar; neither line is ever wider than
// width terminal cells (wide runes count 2) and neither wraps.
// Line 1 shrink order: drop cwd, then branch, then truncate the version.
// Line 2 shrink order: drop in/out, then truncate the model, then truncate.
func RenderStatus(s StatusInfo, width int) (string, string) {
	branch := "-"
	if s.Git {
		branch = s.Branch
		if s.Dirty {
			branch += "*"
		}
	}
	pct := 0
	if s.Window > 0 {
		pct = s.Used * 100 / s.Window
	}
	cost := fmt.Sprintf("$%.4f", s.Cost)
	if s.Sub {
		cost = "sub"
	}
	if s.Transient != "" {
		cost = s.Transient
	}
	l1 := renderLine([]field{{s.Version, 0}, {s.Cwd, 1}, {branch, 2}}, width, 0)
	l2 := renderLine([]field{
		{s.Model, 0}, {AbbrevEffort(s.Effort), 0},
		{"ctx " + FmtWindow(s.Window), 0}, {fmt.Sprintf("%d%%", pct), 0},
		{FmtTokens(s.In) + "/" + FmtTokens(s.Out), 3}, {cost, 0},
	}, width, 0)
	return l1, l2
}

// renderLine joins the non-empty fields with " · " and shrinks them to at
// most width cells: fields drop in rank order 1→3, the field at truncAt
// gives up what is still over as an ellipsis, and a last-resort truncation
// keeps the line within width.
func renderLine(fields []field, width, truncAt int) string {
	render := func() string {
		var parts []string
		for _, f := range fields {
			if f.text != "" {
				parts = append(parts, f.text)
			}
		}
		return strings.Join(parts, " · ")
	}
	for drop := 1; drop <= 3 && lipgloss.Width(render()) > width; drop++ {
		for i := range fields {
			if fields[i].drop == drop {
				fields[i].text = ""
			}
		}
	}
	if over := lipgloss.Width(render()) - width; over > 0 && truncAt >= 0 {
		// Shorten the field by `over` cells plus one for the ellipsis.
		fields[truncAt].text = truncCells(fields[truncAt].text, max(1, lipgloss.Width(fields[truncAt].text)-over-1)) + "…"
	}
	out := render()
	if lipgloss.Width(out) > width {
		out = truncCells(out, max(0, width-1)) + "…"
	}
	return out
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

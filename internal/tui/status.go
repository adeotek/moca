package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
)

// StatusInfo is the plain-text status bar content (§11); styling is the
// caller's job. Field order: cwd · branch · model · effort · ctx W · P% ·
// in/out · cost.
type StatusInfo struct {
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

// RenderStatus renders the bar; it is never wider than width and never wraps.
// Shrink order: drop cwd, then branch, then in/out, then truncate the model.
func RenderStatus(s StatusInfo, width int) string {
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
	type field struct {
		text string
		drop int // drop order: 1 first; 0 never
	}
	model := s.Model
	fields := []field{
		{s.Cwd, 1}, {branch, 2}, {model, 0}, {AbbrevEffort(s.Effort), 0},
		{"ctx " + FmtWindow(s.Window), 0}, {fmt.Sprintf("%d%%", pct), 0},
		{FmtTokens(s.In) + "/" + FmtTokens(s.Out), 3}, {cost, 0},
	}
	render := func() string {
		var parts []string
		for _, f := range fields {
			if f.text != "" {
				parts = append(parts, f.text)
			}
		}
		return strings.Join(parts, " · ")
	}
	for drop := 1; drop <= 3 && utf8.RuneCountInString(render()) > width; drop++ {
		for i := range fields {
			if fields[i].drop == drop {
				fields[i].text = ""
			}
		}
	}
	if over := utf8.RuneCountInString(render()) - width; over > 0 {
		r := []rune(model)
		keep := max(1, len(r)-over-1)
		fields[2].text = string(r[:keep]) + "…"
	}
	out := render()
	if r := []rune(out); len(r) > width {
		out = string(r[:max(0, width-1)]) + "…"
	}
	return out
}

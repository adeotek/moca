package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// diffPreviewRows caps the changed lines shown under an edit item; the whole
// diff (with context) opens in the pager.
const diffPreviewRows = 8

var (
	addFg = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	delFg = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// diffPreview renders the changed lines of a unified diff (the edit tool's
// Detail) under its item line: `+` green, `-` red, headers and context
// dropped, each row indented and clamped to the width, at most rows lines
// with the rest counted. It returns "" for a diff with no changes. The text
// is untrusted file content and goes through Sanitize before styling.
func diffPreview(detail string, width, rows int) string {
	var changed []string
	inHunk := false
	for _, l := range strings.Split(Sanitize(detail), "\n") {
		if strings.HasPrefix(l, "@@") {
			inHunk = true
			continue
		}
		if !inHunk {
			continue // the ---/+++ file headers (a removed "-- x" line is content)
		}
		if strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-") {
			changed = append(changed, strings.ReplaceAll(l, "\t", strings.Repeat(" ", tabWidth)))
		}
	}
	if len(changed) == 0 {
		return ""
	}
	shown := changed
	if len(changed) > rows {
		shown = changed[:max(1, rows-1)]
	}
	var out []string
	for _, l := range shown {
		if width > 8 && lipgloss.Width(l) > width-4 {
			l = truncCells(l, width-5) + "…"
		}
		if l[0] == '+' {
			out = append(out, "    "+addFg.Render(l))
		} else {
			out = append(out, "    "+delFg.Render(l))
		}
	}
	if rest := len(changed) - len(shown); rest > 0 {
		out = append(out, "    "+mutedFg.Render(fmt.Sprintf("… %d more changed %s · ctrl+o for the full diff", rest, plural(rest, "line"))))
	}
	return strings.Join(out, "\n")
}

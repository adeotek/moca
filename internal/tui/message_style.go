package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Scrollback message backgrounds (§11): submitted messages (slate) and
// assistant responses (steel blue) are printed on distinct full-row
// background colors; thinking, tool and notice lines stay plain. Light and
// dark pairs — dark is the default until the terminal reports its background
// (tea.BackgroundColorMsg).
var (
	userBgDark, userFgDark   = lipgloss.Color("#3d434c"), lipgloss.Color("#edf0f4")
	userBgLight, userFgLight = lipgloss.Color("#dfe3e8"), lipgloss.Color("#1c2026")
	respBgDark, respFgDark   = lipgloss.Color("#213b49"), lipgloss.Color("#e9f1fc")
	respBgLight, respFgLight = lipgloss.Color("#c9ddfb"), lipgloss.Color("#17304d")
	// mutedFg styles tool-usage, thinking and notice lines (no background).
	mutedFg = lipgloss.NewStyle().Foreground(lipgloss.Color("#96a0a4"))
	// errFg / warnFg color error and warning lines.
	errFg  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warnFg = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// tabWidth is how many cells a tab expands to inside a styled block.
const tabWidth = 4

func userMsgStyle(dark bool) lipgloss.Style {
	if dark {
		return lipgloss.NewStyle().Background(userBgDark).Foreground(userFgDark)
	}
	return lipgloss.NewStyle().Background(userBgLight).Foreground(userFgLight)
}

func respMsgStyle(dark bool) lipgloss.Style {
	if dark {
		return lipgloss.NewStyle().Background(respBgDark).Foreground(respFgDark)
	}
	return lipgloss.NewStyle().Background(respBgLight).Foreground(respFgLight)
}

// styleBlock renders every line of s with st as a padded band: one cell of
// left padding, the text wrapped to the remaining width, and trailing fill up
// to the full terminal width so the background covers the whole row. Blank
// lines keep a full-width band so a multi-line message reads as one block.
// Tabs are expanded first — lipgloss would expand them after the padding was
// measured and push the row past the terminal width. width ≤ 0 (the size is
// not known yet) means no wrapping and no fill. Untrusted content is
// sanitized before styling (§3.5).
func styleBlock(st lipgloss.Style, s string, width int) string {
	s = strings.ReplaceAll(Sanitize(s), "\t", strings.Repeat(" ", tabWidth))
	var rows []string
	for _, l := range strings.Split(s, "\n") {
		if width > 2 {
			l = lipgloss.Wrap(l, width-2, "")
		}
		rows = append(rows, strings.Split(l, "\n")...)
	}
	for i, l := range rows {
		row := " " + l
		if pad := width - lipgloss.Width(row); pad > 0 {
			row += strings.Repeat(" ", pad)
		}
		rows[i] = st.Render(row)
	}
	return strings.Join(rows, "\n")
}

// printlnUser prints a submitted user message (full-row background), set
// off from the output above by a blank line.
func (m *model) printlnUser(s string) tea.Cmd {
	return m.emit("\n" + styleBlock(userMsgStyle(m.darkBG), s, m.width))
}

// printlnResponse prints assistant response lines (full-row background).
func (m *model) printlnResponse(s string) tea.Cmd {
	return m.emit(styleBlock(respMsgStyle(m.darkBG), s, m.width))
}

// liveResponse renders the streaming partial line with the same band the
// committed lines get, so finishing a line does not flip its look.
func (m *model) liveResponse(s string) string {
	return styleBlock(respMsgStyle(m.darkBG), s, m.width)
}

// printlnMuted prints a tool-usage, thinking or notice line: muted
// foreground, no background. Items.add already sanitized item lines;
// Sanitize runs again for the print-path invariant (§3.5).
func (m *model) printlnMuted(s string) tea.Cmd {
	return m.emit(mutedFg.Render(Sanitize(s)))
}

// printlnError prints an error line in red.
func (m *model) printlnError(s string) tea.Cmd { return m.emit(errFg.Render(Sanitize(s))) }

// printlnWarn prints a warning line in yellow.
func (m *model) printlnWarn(s string) tea.Cmd { return m.emit(warnFg.Render(Sanitize(s))) }

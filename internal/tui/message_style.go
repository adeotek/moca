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
	return styleBlockLead(st, s, width, "")
}

// styleBlockLead is styleBlock with a lead (the `› ` marker of a user
// message) on the first row; every other row is indented by the lead's width
// so wrapped and multi-line text hangs under the first character of the text
// instead of under the marker.
func styleBlockLead(st lipgloss.Style, s string, width int, lead string) string {
	s = strings.ReplaceAll(Sanitize(s), "\t", strings.Repeat(" ", tabWidth))
	hang := lipgloss.Width(lead)
	var rows []string
	for _, l := range strings.Split(s, "\n") {
		if width > 2+hang {
			l = lipgloss.Wrap(l, width-2-hang, "")
		}
		rows = append(rows, strings.Split(l, "\n")...)
	}
	for i, l := range rows {
		row := " " + strings.Repeat(" ", hang) + l
		if i == 0 && lead != "" {
			row = " " + lead + l
		}
		if pad := width - lipgloss.Width(row); pad > 0 {
			row += strings.Repeat(" ", pad)
		}
		rows[i] = st.Render(row)
	}
	return strings.Join(rows, "\n")
}

// userMarker leads a submitted message; the text hangs under its first
// character.
const userMarker = "› "

// printlnUser prints a submitted user message (full-row background), set
// off from the output above by a blank line. A "› " prefix on s becomes the
// hanging lead.
func (m *model) printlnUser(s string) tea.Cmd {
	m.lastKind = kindUser
	if text, ok := strings.CutPrefix(s, userMarker); ok {
		return m.emit("\n" + styleBlockLead(userMsgStyle(m.darkBG), text, m.width, userMarker))
	}
	return m.emit("\n" + styleBlock(userMsgStyle(m.darkBG), s, m.width))
}

// What the scrollback printed last; a response that follows tool or thinking
// items is set off from them by a blank row.
const (
	kindUser = iota + 1
	kindResponse
	kindItem
	kindNotice
)

// printlnResponse prints assistant response lines (full-row background).
func (m *model) printlnResponse(s string) tea.Cmd {
	gap := m.responseGap()
	m.lastKind = kindResponse
	return m.emit(gap + renderMarkdown(&m.md, s, m.width, m.darkBG))
}

// responseGap is the blank row between an item line and the response that
// follows it.
func (m *model) responseGap() string {
	if m.lastKind == kindItem {
		return "\n"
	}
	return ""
}

// liveResponse renders the streaming partial line with the same band the
// committed lines get, so finishing a line does not flip its look (the gap
// above a first line is shown while it is still live, too).
func (m *model) liveResponse(s string) string {
	st := m.md // a copy: looking at the partial line must not move the fence
	return m.responseGap() + renderMarkdown(&st, s, m.width, m.darkBG)
}

// printItem prints a numbered item line (tool call or thinking block) on one
// row: clamped to the width, with a red mark when the tool failed. The full
// text stays in Item.Line for the pager title.
func (m *model) printItem(line string, failed bool) tea.Cmd {
	m.lastKind = kindItem
	line = Sanitize(line)
	if m.width > 0 && lipgloss.Width(line) > m.width {
		line = truncCells(line, max(1, m.width-1)) + "…"
	}
	if mark, rest, ok := strings.Cut(line, " "); ok && failed {
		return m.emit(errFg.Render(mark) + " " + mutedFg.Render(rest))
	}
	return m.emit(mutedFg.Render(line))
}

// printlnMuted prints a tool-usage, thinking or notice line: muted
// foreground, no background. Items.add already sanitized item lines;
// Sanitize runs again for the print-path invariant (§3.5).
func (m *model) printlnMuted(s string) tea.Cmd {
	m.lastKind = kindNotice
	return m.emit(mutedFg.Render(Sanitize(s)))
}

// printlnError prints an error line in red.
func (m *model) printlnError(s string) tea.Cmd {
	m.lastKind = kindNotice
	return m.emit(errFg.Render(Sanitize(s)))
}

// printlnWarn prints a warning line in yellow.
func (m *model) printlnWarn(s string) tea.Cmd {
	m.lastKind = kindNotice
	return m.emit(warnFg.Render(Sanitize(s)))
}

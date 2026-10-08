package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Scrollback message backgrounds (§11): submitted messages (slate) and
// assistant responses (steel blue) are printed on distinct full-row
// background colors; thinking and tool lines stay plain. Light and dark
// pairs — dark is the default until the terminal reports its background
// (tea.BackgroundColorMsg).
var (
	userBgDark, userFgDark   = lipgloss.Color("#3d434c"), lipgloss.Color("#edf0f4")
	userBgLight, userFgLight = lipgloss.Color("#dfe3e8"), lipgloss.Color("#1c2026")
	respBgDark, respFgDark   = lipgloss.Color("#213b49"), lipgloss.Color("#e9f1fc")
	respBgLight, respFgLight = lipgloss.Color("#c9ddfb"), lipgloss.Color("#17304d")
	// toolFg styles the tool-usage and thinking item lines (no background).
	toolFg = lipgloss.NewStyle().Foreground(lipgloss.Color("#96a0a4"))
)

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

// styleBlock renders every line of s with st, padded to the full terminal
// width (width ≤ 0 = no padding) so the background covers the whole row;
// blank lines keep a full-width band so a multi-line message reads as one
// block. Lines already at or beyond the width are left unpadded — the
// terminal soft-wraps them and the background continues (Bubble Tea writes
// each printed line as content + erase-to-EOL + CRLF, so an exactly
// full-width line never double-advances). Untrusted content is sanitized
// before styling (§3.5).
func styleBlock(st lipgloss.Style, s string, width int) string {
	lines := strings.Split(Sanitize(s), "\n")
	for i, l := range lines {
		row := " " + l
		if pad := width - lipgloss.Width(row); pad > 0 {
			row += strings.Repeat(" ", pad)
		}
		lines[i] = st.Render(row)
	}
	return strings.Join(lines, "\n")
}

// printlnUser prints a submitted user message line (full-row background).
func (m *model) printlnUser(s string) tea.Cmd {
	return tea.Println(styleBlock(userMsgStyle(m.darkBG), s, m.width))
}

// printlnResponse prints assistant response lines (full-row background).
func (m *model) printlnResponse(s string) tea.Cmd {
	return tea.Println(styleBlock(respMsgStyle(m.darkBG), s, m.width))
}

// printlnTool prints a tool-usage or thinking item line: muted foreground,
// no background. Items.add already sanitized the line; Sanitize runs again
// for the print-path invariant (§3.5).
func printlnTool(s string) tea.Cmd {
	return tea.Println(toolFg.Render(Sanitize(s)))
}

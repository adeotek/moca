package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Scrollback message backgrounds (§11): submitted messages and assistant
// responses are printed on distinct background colors; thinking and tool
// lines stay plain. Light and dark pairs — dark is the default until the
// terminal reports its background (tea.BackgroundColorMsg).
var (
	userBgDark, userFgDark   = lipgloss.Color("#223a5e"), lipgloss.Color("#d6e4f5")
	userBgLight, userFgLight = lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")
	respBgDark, respFgDark   = lipgloss.Color("#2d2f34"), lipgloss.Color("#e3e3e7")
	respBgLight, respFgLight = lipgloss.Color("#eef0f3"), lipgloss.Color("#24292f")
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

// styleBlock renders every line of s with st, one leading space per line;
// blank lines keep a one-cell band so a multi-line message reads as one
// block. Untrusted content is sanitized before styling (§3.5).
func styleBlock(st lipgloss.Style, s string) string {
	lines := strings.Split(Sanitize(s), "\n")
	for i, l := range lines {
		lines[i] = st.Render(" " + l)
	}
	return strings.Join(lines, "\n")
}

// printlnUser prints a submitted user message line (background field).
func (m *model) printlnUser(s string) tea.Cmd {
	return tea.Println(styleBlock(userMsgStyle(m.darkBG), s))
}

// printlnResponse prints assistant response lines (background field).
func (m *model) printlnResponse(s string) tea.Cmd {
	return tea.Println(styleBlock(respMsgStyle(m.darkBG), s))
}

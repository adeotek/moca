package tui

// A small modal list picker (↑/↓ · enter · esc) rendered above the input
// rule, like the login wizard's pickers. /model without an argument and
// /resume use it; the choice runs a callback so the picker knows nothing of
// either command.

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type pickItem struct {
	key   string // handed to onChoose
	label string
}

type pickState struct {
	title    string
	items    []pickItem
	cursor   int
	onChoose func(key string) tea.Cmd
	// cancelNote is printed (muted) when the picker is dismissed.
	cancelNote string
}

// windowRange is the slice of an n-row list to show in limit rows so that the
// cursor stays visible (the window follows it).
func windowRange(cursor, n, limit int) (start, end int) {
	if n > limit {
		start = min(max(0, cursor-limit+1), n-limit)
	}
	return start, min(n, start+limit)
}

// openPicker makes the picker modal; the caller has already refused it while
// busy.
func (m *model) openPicker(p *pickState) { m.pick = p }

// pickKey handles every key while the picker is up.
func (m *model) pickKey(k tea.KeyPressMsg) tea.Cmd {
	p := m.pick
	switch k.String() {
	case "up":
		p.cursor = max(0, p.cursor-1)
	case "down":
		p.cursor = min(len(p.items)-1, p.cursor+1)
	case "pgup":
		p.cursor = max(0, p.cursor-pickRows)
	case "pgdown":
		p.cursor = min(len(p.items)-1, p.cursor+pickRows)
	case "home":
		p.cursor = 0
	case "end":
		p.cursor = len(p.items) - 1
	case "esc", "ctrl+c":
		m.pick = nil
		if p.cancelNote != "" {
			return m.printlnMuted(p.cancelNote)
		}
	case "enter":
		if p.cursor < 0 || p.cursor >= len(p.items) {
			return nil
		}
		m.pick = nil
		return p.onChoose(p.items[p.cursor].key)
	}
	return nil
}

// pickRows caps the rendered list.
const pickRows = 10

// pickerPanel renders the question and the visible window of rows.
func (m *model) pickerPanel() string {
	p := m.pick
	limit := pickRows
	if m.height > 0 {
		limit = min(limit, max(3, m.height-11))
	}
	start, end := windowRange(p.cursor, len(p.items), limit)
	var b strings.Builder
	b.WriteString(warnFg.Render("? ") + Sanitize(p.title) + "\n")
	for i := start; i < end; i++ {
		b.WriteString(choiceRow(i == p.cursor, p.items[i].label) + "\n")
	}
	legend := "↑/↓ select · enter choose · esc cancel"
	if len(p.items) > limit {
		legend = strconv.Itoa(p.cursor+1) + "/" + strconv.Itoa(len(p.items)) + " · " + legend
	}
	b.WriteString(dim.Render("  " + legend))
	return b.String()
}

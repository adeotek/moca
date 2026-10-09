package tui

// A small modal list picker (↑/↓ · enter · esc) rendered above the input
// rule, like the login wizard's pickers. /model without an argument,
// /resume and /sessions use it; the choice runs a callback so the picker
// knows nothing of any command. With onDelete set (the sessions picker),
// ctrl+d asks for a y/N confirmation and runs onDelete for the selected
// row — the callback owns the outcome (refusal notes, reopening the list).

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type pickItem struct {
	key   string // handed to onChoose/onDelete
	label string
}

type pickState struct {
	title    string
	items    []pickItem
	cursor   int
	onChoose func(key string) tea.Cmd
	// onDelete, when set, enables ctrl+d: a confirmation step runs it for
	// the selected row's key.
	onDelete func(key string) tea.Cmd
	// enterLabel names the enter action in the legend ("switch" for
	// /sessions; "choose" when empty).
	enterLabel string
	// confirm is true while the delete confirmation is showing; pickKey
	// routes every key to it then.
	confirm bool
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
	if p.confirm {
		switch k.String() {
		case "y", "Y", "enter":
			p.confirm = false
			if p.cursor < 0 || p.cursor >= len(p.items) {
				return nil
			}
			return p.onDelete(p.items[p.cursor].key)
		case "n", "N", "esc", "ctrl+c":
			p.confirm = false
		}
		return nil
	}
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
	case "ctrl+d":
		if p.onDelete != nil && len(p.items) > 0 {
			p.confirm = true
		}
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

// pickerPanel renders the question (or the delete confirmation) and the
// visible window of rows.
func (m *model) pickerPanel() string {
	p := m.pick
	limit := pickRows
	if m.height > 0 {
		limit = min(limit, max(3, m.height-11))
	}
	start, end := windowRange(p.cursor, len(p.items), limit)
	var b strings.Builder
	if p.confirm {
		// The sessions picker's labels start with the session id — name the
		// file about to be removed, not just "this row".
		id := ""
		if p.cursor >= 0 && p.cursor < len(p.items) {
			if f := strings.Fields(p.items[p.cursor].label); len(f) > 0 {
				id = f[0]
			}
		}
		b.WriteString(warnFg.Render("? ") + Sanitize("delete session "+id+"? the file is removed from disk — this cannot be undone") + "\n")
	} else {
		b.WriteString(warnFg.Render("? ") + Sanitize(p.title) + "\n")
	}
	for i := start; i < end; i++ {
		b.WriteString(choiceRow(i == p.cursor, p.items[i].label) + "\n")
	}
	enter := p.enterLabel
	if enter == "" {
		enter = "choose"
	}
	legend := "↑/↓ select · enter " + enter + " · esc cancel"
	if p.onDelete != nil {
		legend = "↑/↓ select · enter " + enter + " · ctrl+d delete · esc cancel"
	}
	if p.confirm {
		legend = "[y] delete · [n] back"
	}
	if len(p.items) > limit {
		legend = strconv.Itoa(p.cursor+1) + "/" + strconv.Itoa(len(p.items)) + " · " + legend
	}
	b.WriteString(dim.Render("  " + legend))
	return b.String()
}

package tui

// Slash-command autocomplete: while the draft is exactly "/<word>" (no
// spaces, no newline, not the "//" escape), a dropdown above the input rule
// lists the built-in commands and the loaded prompt templates, filtered as
// the word grows. ↑/↓ picks, tab completes the selection into the draft,
// enter completes a partial word (an exact match sends instead — the
// pre-dropdown behaviour), esc dismisses until the word changes. The list is
// derived from the draft on every lookup (the only stored state is the
// cursor), so there is no second source of truth to keep in sync.

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// dropVisibleRows caps the rendered list; the window follows the cursor.
const dropVisibleRows = 8

type dropItem struct {
	name string // the word to complete to ("model", "review")
	hint string // one-line description
}

type dropState struct {
	q         string // the typed word this state was built for
	items     []dropItem
	cursor    int
	dismissed bool
}

// slashQuery returns the command word being typed when the draft is exactly
// "/<word>" — no newline, no whitespace, not the "//" escape. ok=false hides
// the dropdown.
func slashQuery(buf string) (string, bool) {
	if !strings.HasPrefix(buf, "/") || strings.HasPrefix(buf, "//") {
		return "", false
	}
	if strings.ContainsAny(buf, "\n \t") {
		return "", false
	}
	return buf[1:], true
}

// commandHints lists the completable names with their one-line descriptions:
// built-ins first (they win name collisions), then prompt templates.
func (m *model) commandHints() []dropItem {
	items := make([]dropItem, 0, len(BuiltinCommands)+len(m.opts.Prompts))
	for _, c := range BuiltinCommands {
		items = append(items, dropItem{name: c, hint: builtinHelp[c]})
	}
	for _, p := range m.opts.Prompts {
		if slices.Contains(BuiltinCommands, p.Name) {
			continue
		}
		items = append(items, dropItem{name: p.Name, hint: strings.TrimSpace(p.ArgumentHint + "  " + p.Description)})
	}
	return items
}

// matchCommands filters by the typed word, case-insensitively. An empty word
// lists everything; a word matches by prefix (predictable: /lo → login,
// logout).
func matchCommands(items []dropItem, q string) []dropItem {
	if q == "" {
		return items
	}
	ql := strings.ToLower(q)
	var out []dropItem
	for _, it := range items {
		if strings.HasPrefix(strings.ToLower(it.name), ql) {
			out = append(out, it)
		}
	}
	return out
}

// dropdown returns the active autocomplete state for the current draft, or
// nil. It is hidden whenever enter would be refused or the screen is owned by
// something else: a run (typed text steers the model), a compaction, a `!`
// command, the login wizard, or the pager.
func (m *model) dropdown() *dropState {
	if m.login != nil || m.pager != nil || m.running || m.compacting || m.shellBusy {
		return nil
	}
	q, ok := slashQuery(m.input.Buffer())
	if !ok {
		return nil
	}
	if m.drop == nil || m.drop.q != q {
		m.drop = &dropState{q: q, items: matchCommands(m.commandHints(), q)}
	}
	if m.drop.dismissed || len(m.drop.items) == 0 {
		return nil
	}
	return m.drop
}

// completeFromDrop replaces the typed word with the selected name (plus a
// trailing space, ready for arguments) and parks the cursor at the end of
// the draft.
func (m *model) completeFromDrop() tea.Cmd {
	d := m.drop
	if d == nil || d.cursor < 0 || d.cursor >= len(d.items) {
		return nil
	}
	m.input.SetBuffer("/" + d.items[d.cursor].name + " ")
	m.syncTextarea()
	m.lastTyped = m.now()
	return nil
}

// dropExact reports whether the typed word already equals the selected name,
// i.e. enter should submit rather than complete.
func (m *model) dropExact(d *dropState) bool {
	if d.cursor < 0 || d.cursor >= len(d.items) {
		return false
	}
	q, ok := slashQuery(m.input.Buffer())
	return ok && q == d.items[d.cursor].name
}

// dropdownKey intercepts the keys the dropdown owns. handled=false lets the
// key fall through to the normal input handling (typing, enter on an exact
// match, ctrl+c, …).
func (m *model) dropdownKey(d *dropState, k tea.KeyPressMsg) (tea.Cmd, bool) {
	switch k.String() {
	case "up":
		if d.cursor > 0 {
			d.cursor--
		}
		return nil, true
	case "down":
		if d.cursor < len(d.items)-1 {
			d.cursor++
		}
		return nil, true
	case "tab":
		return m.completeFromDrop(), true
	case "esc":
		d.dismissed = true
		return nil, true
	case "enter":
		if !m.dropExact(d) {
			return m.completeFromDrop(), true
		}
	}
	return nil, false
}

// dropdownPanel renders the list above the input rule: a cursor window of at
// most dropVisibleRows rows plus a dim key legend.
func (m *model) dropdownPanel(d *dropState) string {
	width := max(20, m.width)
	limit := dropVisibleRows
	if m.height > 0 {
		limit = min(limit, max(3, m.height-11))
	}
	start := 0
	if len(d.items) > limit {
		start = min(max(0, d.cursor-limit+1), len(d.items)-limit)
	}
	end := min(len(d.items), start+limit)
	var b strings.Builder
	for i := start; i < end; i++ {
		it := d.items[i]
		mark := "  "
		if i == d.cursor {
			mark = "❯ "
		}
		line := mark + Sanitize(it.name)
		if hint := Sanitize(it.hint); hint != "" {
			room := width - 5 - len([]rune(it.name))
			if room > 8 {
				line += "  " + dim.Render(clampRunes(hint, room))
			}
		}
		b.WriteString(line + "\n")
	}
	legend := "↑/↓ select · tab complete · esc dismiss"
	if len(d.items) > limit {
		legend = fmt.Sprintf("%d/%d · ", d.cursor+1, len(d.items)) + legend
	}
	b.WriteString(dim.Render("  " + legend))
	return b.String()
}

// clampRunes cuts s to at most n runes, ellipsized (a plain rune count is
// enough for one-line hints).
func clampRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

package tui

// Slash-command autocomplete: while the draft is exactly "/<word>" (no
// spaces, no newline, not the "//" escape), a dropdown above the input rule
// lists the built-in commands and the loaded prompt templates, filtered as
// the word grows. ↑/↓ picks, tab completes the selection into the draft,
// enter completes a partial word (an exact match sends instead — the
// pre-dropdown behaviour), esc dismisses until the word changes. The list is
// derived from the draft on every lookup (the only stored state is the
// cursor), so there is no second source of truth to keep in sync.
//
// The same panel completes the argument of the commands that take one
// (`/model <id>`, `/effort <level>`, `/login|logout <provider>`, `/show <n>`,
// `/resume <id>`) and, on ctrl+r, searches the prompt history with the draft
// as the query.

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
)

// dropVisibleRows caps the rendered list; the window follows the cursor.
const dropVisibleRows = 8

type dropItem struct {
	name string // the word to complete to ("model", "review")
	hint string // one-line description
	full string // history search: the whole message the row stands for
}

type dropState struct {
	q         string // the draft this state was built for
	cmd       string // argument mode: the command whose argument is completed
	word      string // the typed word (or argument)
	items     []dropItem
	cursor    int
	dismissed bool
	hist      bool // ctrl+r history search
	mention   bool // @file completion of the draft's last word
	start     int  // mention: byte offset of the `@` in the draft
}

// slashQuery returns the command word being typed when the draft is exactly
// "/<word>" — no newline, no whitespace, not the "//" escape. ok=false hides
// the dropdown.
func slashQuery(buf string) (string, bool) {
	cmd, _, inArg, ok := parseSlash(buf)
	if !ok || inArg {
		return "", false
	}
	return cmd, true
}

// argCommands are the commands whose single argument the dropdown completes.
var argCommands = []string{"model", "effort", "login", "logout", "show", "resume"}

// parseSlash splits a "/<cmd>" or "/<cmd> <arg>" draft. ok=false for anything
// else: not a command, the "//" escape, a newline or tab, or a second space
// (arguments with spaces are typed by hand).
func parseSlash(buf string) (cmd, arg string, inArg, ok bool) {
	if !strings.HasPrefix(buf, "/") || strings.HasPrefix(buf, "//") || strings.ContainsAny(buf, "\n\t") {
		return "", "", false, false
	}
	rest := buf[1:]
	cmd, arg, inArg = strings.Cut(rest, " ")
	if inArg && (strings.Contains(arg, " ") || !slices.Contains(argCommands, cmd)) {
		return "", "", false, false
	}
	return cmd, arg, inArg, true
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
	if m.login != nil || m.pick != nil || m.pager != nil || m.running || m.compacting || m.shellBusy {
		return nil
	}
	buf := m.input.Buffer()
	if m.histSearch {
		if m.drop == nil || !m.drop.hist || m.drop.q != buf {
			m.drop = &dropState{q: buf, word: buf, hist: true, items: m.historyMatches(buf)}
		}
		return m.drop
	}
	cmd, arg, inArg, ok := parseSlash(buf)
	if !ok {
		return m.mentionDropdown(buf)
	}
	if m.drop == nil || m.drop.hist || m.drop.mention || m.drop.q != buf {
		d := &dropState{q: buf, word: cmd}
		if inArg {
			d.cmd, d.word, d.items = cmd, arg, m.argHints(cmd, arg)
		} else {
			d.items = matchCommands(m.commandHints(), cmd)
		}
		m.drop = d
	}
	if m.drop.dismissed || len(m.drop.items) == 0 {
		return nil
	}
	return m.drop
}

// mentionDropdown is the `@file` completion for the draft's last word (nil
// when it is not a mention, the index is not loaded yet, or nothing matches).
func (m *model) mentionDropdown(buf string) *dropState {
	q, start, ok := mentionQuery(buf)
	if !ok || len(m.files) == 0 {
		return nil
	}
	if m.drop == nil || !m.drop.mention || m.drop.q != buf {
		m.drop = &dropState{q: buf, word: q, mention: true, start: start, items: mentionMatches(m.files, q)}
	}
	if m.drop.dismissed || len(m.drop.items) == 0 {
		return nil
	}
	return m.drop
}

// argHints lists the completions of a command's argument. Models match by
// substring (`glm` finds `opencode-go/glm-5.3`), everything else by prefix.
func (m *model) argHints(cmd, arg string) []dropItem {
	ql := strings.ToLower(arg)
	prefix := func(name string) bool { return strings.HasPrefix(strings.ToLower(name), ql) }
	var out []dropItem
	switch cmd {
	case "model":
		if m.agent == nil {
			return nil
		}
		cur := m.agent.Status().Model.Qualified()
		creds := map[string]bool{} // one credential lookup per provider
		for _, mo := range m.agent.Models() {
			q := mo.Qualified()
			if !strings.Contains(strings.ToLower(q), ql) {
				continue
			}
			hint := ""
			ok, seen := creds[mo.Provider]
			if !seen {
				ok = m.agent.HasCredential(q)
				creds[mo.Provider] = ok
			}
			switch {
			case q == cur:
				hint = "current"
			case !ok:
				hint = "no key — /login " + mo.Provider
			}
			out = append(out, dropItem{name: q, hint: hint})
		}
	case "effort":
		if m.agent == nil {
			return nil
		}
		st := m.agent.Status()
		for _, e := range llm.Efforts {
			if _, ok := st.Model.ThinkingLevelMap[e]; ok && prefix(string(e)) {
				hint := ""
				if e == st.Effort {
					hint = "current"
				}
				out = append(out, dropItem{name: string(e), hint: hint})
			}
		}
	case "login":
		for _, n := range providerNames(m.start.Config) {
			if prefix(n) {
				out = append(out, dropItem{name: n, hint: strings.TrimPrefix(providerRow(n), n+"  — ")})
			}
		}
	case "logout":
		store := provider.NewDefaultStore()
		for _, n := range providerNames(m.start.Config) {
			if !prefix(n) {
				continue
			}
			if _, has, err := store.GetAPIKey(n); err == nil && has {
				out = append(out, dropItem{name: n, hint: "stored API key"})
			} else if _, has, err := store.Get(n); err == nil && has {
				out = append(out, dropItem{name: n, hint: "stored login"})
			}
		}
	case "show":
		for i := len(m.items.list) - 1; i >= 0; i-- { // newest first
			it := m.items.list[i]
			if n := strconv.Itoa(it.N); prefix(n) {
				out = append(out, dropItem{name: n, hint: it.Line})
			}
		}
	case "resume":
		for _, s := range m.otherSessions() {
			if prefix(s.ID8) {
				out = append(out, dropItem{name: s.ID8, hint: sessionHint(s)})
			}
		}
	}
	return out
}

// historyMatches lists the history entries containing the query, newest
// first, one row each (the first line, marked when the message has more).
func (m *model) historyMatches(q string) []dropItem {
	ql := strings.ToLower(q)
	h := m.input.History()
	var out []dropItem
	seen := map[string]bool{}
	for i := len(h) - 1; i >= 0; i-- {
		if seen[h[i]] || !strings.Contains(strings.ToLower(h[i]), ql) {
			continue
		}
		seen[h[i]] = true
		name := Sanitize(firstLineOf(h[i]))
		if strings.Contains(h[i], "\n") {
			name += " ⏎…"
		}
		out = append(out, dropItem{name: name, full: h[i]})
	}
	return out
}

// completeFromDrop replaces the typed word with the selected name (plus a
// trailing space, ready for arguments) and parks the cursor at the end of
// the draft.
func (m *model) completeFromDrop() tea.Cmd {
	d := m.drop
	if d == nil || d.cursor < 0 || d.cursor >= len(d.items) {
		return nil
	}
	switch {
	case d.mention:
		// Replace the `@fragment` with the path and move on to the next word.
		m.input.SetBuffer(m.input.Buffer()[:d.start] + "@" + d.items[d.cursor].name + " ")
	case d.hist:
		// The remembered message replaces the query; it is not sent.
		m.histSearch = false
		m.input.Recall(d.items[d.cursor].full)
	case d.cmd != "":
		m.input.SetBuffer("/" + d.cmd + " " + d.items[d.cursor].name)
	default:
		m.input.SetBuffer("/" + d.items[d.cursor].name + " ")
	}
	m.syncTextarea()
	m.lastTyped = m.now()
	return nil
}

// dropExact reports whether the typed word already equals the selected name,
// i.e. enter should submit rather than complete.
func (m *model) dropExact(d *dropState) bool {
	if d.hist || d.mention || d.cursor < 0 || d.cursor >= len(d.items) {
		return false // a mention is part of a sentence: enter inserts the path, it never sends
	}
	return d.word == d.items[d.cursor].name
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
		if d.hist {
			m.histSearch = false
			return nil, true
		}
		d.dismissed = true
		return nil, true
	case "enter":
		if len(d.items) == 0 {
			return nil, d.hist // a history search with no match swallows enter
		}
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
	start, end := windowRange(d.cursor, len(d.items), limit)
	var b strings.Builder
	if d.hist && len(d.items) == 0 {
		b.WriteString("  " + dim.Render("no history matches") + "\n")
	}
	for i := start; i < end; i++ {
		it := d.items[i]
		mark := "  "
		if i == d.cursor {
			mark = "❯ "
		}
		name := clampRunes(Sanitize(it.name), max(4, width-4))
		line := mark + name
		if hint := Sanitize(it.hint); hint != "" {
			room := width - 5 - len([]rune(name))
			if room > 8 {
				line += "  " + dim.Render(clampRunes(hint, room))
			}
		}
		b.WriteString(line + "\n")
	}
	legend := "↑/↓ select · tab complete · esc dismiss"
	if d.hist {
		legend = "history — type to filter · ↑/↓ select · enter/tab use · esc cancel"
	}
	if d.mention {
		legend = "↑/↓ select · tab/enter insert path · esc dismiss"
	}
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

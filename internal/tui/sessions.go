package tui

// `/sessions`: the session manager for this directory — every stored session
// (the open one included and marked), enter switches to one (the /resume
// path), ctrl+d deletes one after a y/N confirmation. Deleting removes the
// session file; it is refused for the session in use — this one, or one
// another moca process holds open.

import (
	"errors"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/session"
)

// sessionsLimit caps the picker list; the title names the real total when
// more are stored.
const sessionsLimit = 100

// runSessions is `/sessions`.
func (m *model) runSessions() tea.Cmd {
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	return m.openSessions()
}

// openSessions (re)builds and opens the picker; the delete callback reopens
// it so the list stays live minus the deleted row.
func (m *model) openSessions() tea.Cmd {
	list, total := session.ListAll(sessionsDir(), m.opts.Start.Workdir, sessionsLimit)
	if len(list) == 0 {
		m.pick = nil // reopened after the last delete: drop the stale list
		return m.printlnMuted("no sessions for this directory")
	}
	cur := ""
	if m.agent != nil {
		cur = m.agent.Session().ID8()
	}
	ids := make(map[string]string, len(list)) // path → id8
	for _, s := range list {
		ids[s.Path] = s.ID8
	}
	title := "sessions in " + AbbrevHome(m.opts.Start.Workdir, m.opts.Home)
	if total > len(list) {
		title += " · newest " + strconv.Itoa(len(list)) + " of " + strconv.Itoa(total)
	}
	p := &pickState{
		title:      title,
		enterLabel: "switch",
		cancelNote: "sessions closed",
		onChoose:   m.resumeSession,
		confirmPrompt: func(path string) string {
			return "delete session " + ids[path] + "? the file is removed from disk — this cannot be undone"
		},
	}
	p.onDelete = func(path string) tea.Cmd {
		id := ids[path]
		if id == cur {
			return tea.Batch(m.printlnMuted("the current session cannot be deleted — /clear starts a new one"), m.openSessions())
		}
		if err := session.Delete(path); err != nil {
			if errors.Is(err, session.ErrInUse) {
				return tea.Batch(m.printlnError("error: session "+id+" is open in another moca process — close it there first"), m.openSessions())
			}
			return tea.Batch(m.printlnError("error: could not delete session "+id+": "+err.Error()), m.openSessions())
		}
		return tea.Batch(m.printlnMuted("deleted session "+id), m.openSessions())
	}
	for _, s := range list {
		label := s.ID8
		if s.ID8 == cur {
			label += " (current)"
		}
		if s.Preview != "" {
			label += "  " + sessionHint(s)
		} else {
			label += "  " + fmtAge(time.Since(s.Modified)) + " · (no messages)"
		}
		p.items = append(p.items, pickItem{key: s.Path, label: label})
	}
	m.openPicker(p)
	return nil
}

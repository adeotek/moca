package tui

// `/sessions`: the session manager for this directory — every stored session
// (the open one included and marked), enter switches to one (the /resume
// path), ctrl+d deletes one after a y/N confirmation. Deleting removes the
// session file; it is refused for the session in use — this one, or one
// another moca process holds open.

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/session"
)

// sessionsLimit caps the picker list (it windows; the counter shows the total).
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
	list := session.ListAll(sessionsDir(), m.opts.Start.Workdir, sessionsLimit)
	if len(list) == 0 {
		return m.printlnMuted("no sessions for this directory")
	}
	cur := ""
	if m.agent != nil {
		cur = m.agent.Session().ID8()
	}
	p := &pickState{
		title:      "sessions in " + AbbrevHome(m.opts.Start.Workdir, m.opts.Home),
		enterLabel: "switch",
		cancelNote: "sessions closed",
		onChoose:   m.resumeSession,
	}
	p.onDelete = func(path string) tea.Cmd {
		id := sessionIDOf(path)
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

// sessionIDOf is a session file's 8-char id — the basename's last 8 chars
// before .jsonl (session.idOf's shape).
func sessionIDOf(path string) string {
	b := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	return b[max(0, len(b)-8):]
}

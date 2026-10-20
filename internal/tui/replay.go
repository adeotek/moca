package tui

// Replaying a resumed session into the scrollback: the last few exchanges,
// rendered through the same paths as live output (user bands, markdown
// responses, numbered tool and thinking items), so picking a conversation up
// again starts with a view of where it stood instead of one summary line.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

// replayTurns is how many user turns a resume shows.
const replayTurns = 3

// isUserTurn: a user message that carries text (not only tool results).
func isUserTurn(msg llm.Message) bool {
	if msg.Role != llm.RoleUser {
		return false
	}
	for _, b := range msg.Content {
		if b.Type == llm.BlockText {
			return true
		}
	}
	return false
}

// firstLine is a one-line rendering of a nudge for the replay header.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:117] + "…"
	}
	return s
}

// replay prints the last `turns` user turns of msgs.
func (m *model) replay(msgs []llm.Message, turns int, nudges map[string]string) tea.Cmd {
	var starts []int
	for i, msg := range msgs {
		if isUserTurn(msg) && !strings.HasPrefix(strings.TrimSpace(llm.TextOf(msg)), session.SummaryPrefix) {
			starts = append(starts, i)
		}
	}
	from := 0
	var cmds []tea.Cmd
	if len(starts) > turns {
		from = starts[len(starts)-turns]
		cmds = append(cmds, m.printlnMuted(fmt.Sprintf("… %d earlier %s not shown — the session file has them", len(starts)-turns, plural(len(starts)-turns, "turn"))))
	}
	results := map[string]llm.ToolResult{}
	for _, msg := range msgs[from:] {
		for _, b := range msg.Content {
			if b.Type == llm.BlockToolResult && b.ToolResult != nil {
				results[b.ToolResult.CallID] = *b.ToolResult
			}
		}
	}
	for _, msg := range msgs[from:] {
		switch msg.Role {
		case llm.RoleUser:
			if !isUserTurn(msg) {
				continue // tool results: shown with their calls
			}
			text := strings.TrimSpace(llm.TextOf(msg))
			if strings.HasPrefix(text, session.SummaryPrefix) {
				cmds = append(cmds, m.printlnMuted("⋯ earlier conversation was summarized"))
				continue
			}
			// A nudge is moca's own text, not the user's (P3.8): show it as
			// what it was instead of attributing it to the person.
			if kind, ok := nudges[text]; ok {
				cmds = append(cmds, m.printlnMuted(fmt.Sprintf("· moca's %s nudge (not sent by you): %s", kind, firstLine(text))))
				continue
			}
			cmds = append(cmds, m.printlnUser(userMarker+text))
		case llm.RoleAssistant:
			for _, b := range msg.Content {
				switch b.Type {
				case llm.BlockThinking:
					if strings.TrimSpace(b.Text) != "" {
						it := m.items.AddThinking(b.Text, 0)
						cmds = append(cmds, m.printItem(it.Line+thinkingHint, false))
					}
				case llm.BlockText:
					if strings.TrimSpace(b.Text) != "" {
						cmds = append(cmds, m.printlnResponse(b.Text))
					}
				case llm.BlockToolUse:
					if b.ToolCall == nil {
						continue
					}
					r := results[b.ToolCall.ID]
					it := m.items.AddTool(*b.ToolCall, tools.Result{Content: r.Content, IsError: r.IsError, Summary: toolArgSummary(*b.ToolCall)})
					cmds = append(cmds, m.printItem(it.Line, r.IsError))
				}
			}
			m.md = mdState{}
		}
	}
	return tea.Sequence(cmds...)
}

// sessionsDir is where session files live.
func sessionsDir() string { return filepath.Join(config.DataDir(), "sessions") }

// otherSessions lists this directory's stored sessions except the open one.
func (m *model) otherSessions() []session.Info {
	cur := ""
	if m.agent != nil {
		cur = m.agent.Session().ID8()
	}
	return session.List(sessionsDir(), m.opts.Start.Workdir, cur, 20)
}

// sessionHint is the dropdown/picker description of a session: age and the
// first thing the user said.
func sessionHint(s session.Info) string {
	return fmtAge(time.Since(s.Modified)) + " · " + s.Preview
}

// fmtAge renders how long ago: 5m · 3h · 2d · 6w.
func fmtAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return fmt.Sprintf("%dw ago", int(d.Hours()/24/7))
}

// runResume is `/resume [id8]`: with an id, resume that session; without,
// pick one of this directory's other sessions.
func (m *model) runResume(args string) tea.Cmd {
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	if id := strings.TrimSpace(args); id == "last" {
		// The newest of this directory's sessions is usually the open one:
		// `last` here is the newest other one.
		list := m.otherSessions()
		if len(list) == 0 {
			return m.printlnMuted("no other sessions for this directory")
		}
		return m.resumeSession(list[0].Path)
	} else if id != "" {
		path, err := session.Find(sessionsDir(), id, m.opts.Start.Workdir)
		if err != nil {
			return m.printlnError("error: " + err.Error())
		}
		// Like the picker, only this directory's sessions: the agent would
		// move to the other workdir while `!`, @mentions and the trust
		// decision stay with this one.
		if !session.InWorkdir(path, m.opts.Start.Workdir) {
			return m.printlnError("error: session " + id + " belongs to another directory — run moca --resume " + id + " from there")
		}
		return m.resumeSession(path)
	}
	list := m.otherSessions()
	if len(list) == 0 {
		return m.printlnMuted("no other sessions for this directory")
	}
	p := &pickState{title: "resume which session?", cancelNote: "resume cancelled", onChoose: m.resumeSession}
	for _, s := range list {
		p.items = append(p.items, pickItem{key: s.Path, label: s.ID8 + "  " + sessionHint(s)})
	}
	m.openPicker(p)
	return nil
}

// resumeSession swaps the running session for a stored one. The replacement
// is opened first so a failure leaves the current session untouched; the
// stored model/effort win (as with --resume).
func (m *model) resumeSession(path string) tea.Cmd {
	opts := m.start
	opts.Model, opts.Effort = "", ""
	if m.agent != nil && strings.HasSuffix(strings.TrimSuffix(path, ".jsonl"), m.agent.Session().ID8()) {
		return m.printlnMuted("that is the current session")
	}
	a, err := agent.Resume(opts, path)
	if err != nil {
		return m.printlnError("error: " + err.Error())
	}
	if m.agent != nil {
		m.agent.Close()
	}
	m.agent = a
	m.items = Items{}
	m.live.Reset()
	m.thinking.Reset()
	m.md = mdState{}
	m.sessionUsed = true
	m.status.Transient = ""
	m.refreshStatus()
	// The Resumed event (through the pipe) prints the header and the replay.
	return m.branchCmd()
}

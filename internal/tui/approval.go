package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

// approvalState is the shown approval question plus the picker selection.
// sel indexes into options(); it starts at 0 (Allow once) — allow-always is
// persistent, so it must be chosen deliberately.
type approvalState struct {
	approvalMsg
	sel int // index into options()
}

// approvalOption is one row of the picker.
type approvalOption struct {
	label string
	key   string
	ans   tools.Answer
}

// options returns the picker rows: Allow once, then Allow always (only when
// the question can persist), then Deny.
func (s *approvalState) options() []approvalOption {
	opts := []approvalOption{{label: "Allow once", key: "a", ans: tools.AllowOnce}}
	if s.q.CanAlways {
		opts = append(opts, approvalOption{label: "Allow always", key: "ctrl+a", ans: tools.AllowAlways})
	}
	return append(opts, approvalOption{label: "Deny", key: "d · esc", ans: tools.Deny})
}

// approvalDetailRows caps the command lines shown under an approval question;
// the whole command opens in the pager (ctrl+o).
const approvalDetailRows = 6

func approvalPrompt(q tools.Question) string {
	subject := Sanitize(q.Subject)
	if q.CanAlways {
		return fmt.Sprintf("allow `%s`?  [a] once  [ctrl+a] always  [d] deny", subject)
	}
	return fmt.Sprintf("allow `%s` (asks every time)?  [a] once  [d] deny", subject)
}

// approvalPanel renders the question, the picker rows and the command it is
// about. The whole command is shown up to rows lines — the subject alone can
// hide what runs (an approved `python3` executes the heredoc under it) — each
// clamped to the width; the rest is counted and opens in the pager.
func approvalPanel(s *approvalState, width, rows int) string {
	q := s.q
	var b strings.Builder
	b.WriteString(warnFg.Render("? ") + approvalPrompt(q))
	for i, o := range s.options() {
		line := "  " + o.label
		if i == s.sel {
			line = warnFg.Render("❯ ") + bold.Render(o.label)
		} else {
			line = dim.Render(line)
		}
		b.WriteString("\n" + line + dim.Render("    "+o.key))
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(Sanitize(q.Detail), "\t", "    "), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return b.String()
	}
	shown := lines
	if len(lines) > rows {
		shown = lines[:max(1, rows-1)]
	}
	for _, l := range shown {
		if lipgloss.Width(l) > width-4 {
			l = truncCells(l, max(1, width-5)) + "…"
		}
		b.WriteString("\n" + dim.Render("  │ ") + l)
	}
	if rest := len(lines) - len(shown); rest > 0 {
		b.WriteString("\n" + dim.Render(fmt.Sprintf("  │ … +%d %s · ctrl+o shows the whole command", rest, plural(rest, "line"))))
	}
	return b.String()
}

// approvalKey answers the shown prompt. `a`/`d` count only after a typing
// pause (a user mid-sentence must not answer with letters meant for the
// draft); allow-always is `ctrl+a` — it is persistent, so a plain letter (a
// stray capital `A`, the first letter of "Add…") always goes to the draft and
// can never write the config. esc always denies. The arrows move the picker
// selection with wrap-around and enter confirms it, but only when the draft
// is empty and the typing pause has elapsed — with a draft they keep their
// editing/history meaning and the input box is never dead while a prompt is
// shown. handled=false lets the key fall through to normal input handling.
func (m *model) approvalKey(k tea.KeyPressMsg) (tea.Cmd, bool) {
	switch k.String() {
	case "esc":
		return m.answerApproval(tools.Deny), true
	case "a", "d":
		if m.now().Sub(m.lastTyped) < approvalIdle {
			return nil, false
		}
	case "up", "down":
		if m.input.Buffer() != "" {
			return nil, false // keep history/editing behaviour during a draft
		}
		if m.now().Sub(m.lastTyped) < approvalIdle {
			return nil, false
		}
	case "enter":
		if m.input.Buffer() != "" || m.now().Sub(m.lastTyped) < approvalIdle {
			// A draft's enter submits; a fresh enter must not answer a
			// prompt that popped up under it.
			return nil, false
		}
	}
	switch k.String() {
	case "a":
		return m.answerApproval(tools.AllowOnce), true
	case "d":
		return m.answerApproval(tools.Deny), true
	case "up", "down":
		n := len(m.approval.options())
		if k.String() == "down" {
			m.approval.sel = (m.approval.sel + 1) % n
		} else {
			m.approval.sel = (m.approval.sel - 1 + n) % n
		}
		return nil, true
	case "enter":
		opts := m.approval.options()
		return m.answerApproval(opts[m.approval.sel].ans), true
	case "ctrl+a":
		if !m.approval.q.CanAlways {
			return nil, true // not offered for ask-every-time commands
		}
		return m.answerApproval(tools.AllowAlways), true
	}
	return nil, false
}

// answerApproval is the single confirm path for every approval answer: the
// reply channel, the drop of the state and (for allow-always) the persist to
// the config file. ctrl+a and the picker's enter cannot drift apart.
func (m *model) answerApproval(ans tools.Answer) tea.Cmd {
	q, reply := m.approval.q, m.approval.reply
	m.approval = nil
	reply <- ans
	if ans != tools.AllowAlways {
		return nil
	}
	path := m.opts.ConfigPath
	if path == "" {
		path = config.ConfigFile()
	}
	switch q.Kind {
	case "shell":
		// The running session (and a /clear restart) honours it now; the file
		// makes it permanent.
		m.start.Config.Shell.Allow = append(slices.Clone(m.start.Config.Shell.Allow), q.Subject)
		if err := config.AppendString(path, []string{"shell", "allow"}, q.Subject, config.DefaultShellAllow); err != nil {
			return m.printlnError("error: allow-always not saved: " + err.Error())
		}
		return m.printlnContent(fmt.Sprintf("always allowing %q (saved to %s)", q.Subject, path))
	case "mcp":
		// Subject is server/tool. The running session already learned it
		// from the AllowAlways answer; the config makes it survive /clear
		// and the next start.
		server, tool, ok := strings.Cut(q.Subject, "/")
		if !ok || server == "" || tool == "" {
			return nil
		}
		if s, ok := m.start.Config.MCP.Servers[server]; ok {
			s.Approve = append(slices.Clone(s.Approve), tool)
			m.start.Config.MCP.Servers[server] = s
		}
		if err := config.AppendString(path, []string{"mcp", "servers", server, "approve"}, tool, nil); err != nil {
			return m.printlnError("error: allow-always not saved: " + err.Error())
		}
		return m.printlnContent(fmt.Sprintf("always allowing %s (saved to %s)", q.Subject, path))
	}
	return nil
}

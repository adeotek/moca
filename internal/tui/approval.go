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

// options returns the picker rows: Allow once, Allow for this session (hidden
// when the question forbids it), Allow always (only when the answer can
// persist), then Deny.
func (s *approvalState) options() []approvalOption {
	opts := []approvalOption{{label: "Allow once", key: "o", ans: tools.AllowOnce}}
	if !s.q.NoSession {
		session := "Allow for this session"
		if s.q.SessionExact() {
			// The name tier would be a lie here: only this exact line is allowed.
			session = "Allow this exact command for this session"
		}
		opts = append(opts, approvalOption{label: session, key: "s", ans: tools.AllowSession})
	}
	if s.q.CanAlways {
		opts = append(opts, approvalOption{label: "Allow always", key: "a", ans: tools.AllowAlways})
	}
	return append(opts, approvalOption{label: "Deny", key: "d · esc", ans: tools.Deny})
}

// approvalDetailRows caps the command lines shown under an approval question;
// the whole command opens in the pager (ctrl+o).
const approvalDetailRows = 6

// approvalPrompt is the question line above the picker rows. The rows carry
// the shortcuts, so the line never repeats them (rev 28: the options used to
// be listed twice — horizontally here and as the picker below). A question
// that can be answered for the session says the answer is never saved; an
// excluded one (NoSession) still asks every time.
func approvalPrompt(q tools.Question) string {
	subject := Sanitize(q.Subject)
	switch {
	case q.CanAlways:
		return fmt.Sprintf("allow `%s`?", subject)
	case q.NoSession:
		return fmt.Sprintf("allow `%s` (asks every time)?", subject)
	}
	return fmt.Sprintf("allow `%s` (never saved)?", subject)
}

// approvalPanel renders the question, the picker rows and the command it is
// about. The whole command is shown up to rows lines — the subject alone can
// hide what runs (an approved `python3` executes the heredoc under it) — each
// clamped to the width; the rest is counted and opens in the pager.
func approvalPanel(s *approvalState, width, rows int, th Theme, dark bool) string {
	q := s.q
	var b strings.Builder
	b.WriteString(th.promptMark(dark) + approvalPrompt(q))
	for i, o := range s.options() {
		line := "  " + o.label
		if i == s.sel {
			line = th.warningStyle(dark).Render("❯ ") + bold.Render(o.label)
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

// approvalKey answers the shown prompt. `o`/`s`/`a`/`d` count only after a
// typing pause (a user mid-sentence must not answer with letters meant for the
// draft; a capital always goes to the draft), and a letter the question does
// not offer is swallowed — handled, no answer, nothing typed (`a` on an rm
// question, `s` on a NoSession one). `a` is the one dangerous key: it persists
// the answer, so a stray `a` after the pause does write `.moca/config.jsonc`
// even with a half-typed draft — the CHANGELOG leads with that key change
// (`ctrl+a` no longer answers; it falls through to the input's line start).
// esc always denies. The arrows move the picker selection with wrap-around and
// enter confirms it, but only when the draft is empty and the typing pause has
// elapsed — with a draft they keep their editing/history meaning and the input
// box is never dead while a prompt is shown. handled=false lets the key fall
// through to normal input handling.
func (m *model) approvalKey(k tea.KeyPressMsg) (tea.Cmd, bool) {
	switch k.String() {
	case "esc":
		return m.answerApproval(tools.Deny), true
	case "o", "s", "a", "d":
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
	case "o":
		return m.answerApproval(tools.AllowOnce), true
	case "s":
		if m.approval.q.NoSession {
			return nil, true // not offered: swallowed, not typed
		}
		return m.answerApproval(tools.AllowSession), true
	case "a":
		if !m.approval.q.CanAlways {
			return nil, true // not offered for ask-every-time commands
		}
		return m.answerApproval(tools.AllowAlways), true
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
	}
	return nil, false
}

// answerApproval is the single confirm path for every approval answer: the
// reply channel, the drop of the state and — for allow-always — the persist of
// the decision. The `a` shortcut and the picker's enter cannot drift apart.
//
// AllowSession has no bookkeeping here: the running agent's RememberSession
// wrapper records it, so the note only says what was decided and no config is
// touched. Permanent approvals are project-scoped (rev 28): they are saved in
// the workdir's `.moca/config.jsonc` — `permissions.allow` for a command,
// `mcp.servers.<server>.approve` for an MCP tool — never in the user config,
// so what one project needs does not follow the user into every other
// directory. A workdir that is not trusted cannot have project resources
// applied, so the note says that too (the running session honours the answer
// either way).
func (m *model) answerApproval(ans tools.Answer) tea.Cmd {
	q, reply := m.approval.q, m.approval.reply
	m.approval = nil
	reply <- ans
	if ans == tools.AllowSession {
		what := q.Subject
		if q.SessionExact() && q.Detail != "" {
			// The exact tier allows one line, not the name: say which line.
			what = strings.SplitN(q.Detail, "\n", 2)[0]
		}
		return m.printlnContent(fmt.Sprintf("allowing `%s` for this session", Sanitize(what)))
	}
	if ans != tools.AllowAlways {
		return nil
	}
	path := m.projectPath()
	if path == "" {
		// No workdir to save into (a bare test model): the session keeps it.
		return m.printlnContent(fmt.Sprintf("always allowing %s (this session only)", Sanitize(q.Subject)))
	}
	caveat := ""
	if !m.start.Trusted {
		caveat = " — project rules apply once this workdir is trusted"
	}
	switch q.Kind {
	case "shell":
		// Both in-memory copies, so a /clear restart honours it too (the
		// running session learned it with the answer).
		pl := m.projectLayer()
		m.start.Config.Shell.Allow = append(slices.Clone(m.start.Config.Shell.Allow), q.Subject)
		pl.Permissions.Allow = append(pl.Permissions.Allow, q.Subject)
		if err := config.AppendString(path, []string{"permissions", "allow"}, q.Subject, nil); err != nil {
			return m.printlnError("error: allow-always not saved: " + err.Error())
		}
		return m.printlnContent(fmt.Sprintf("always allowing %q (saved to %s%s)", q.Subject, path, caveat))
	case "mcp":
		// Subject is server/tool. The running session already learned it
		// from the AllowAlways answer; the project layer (in memory for
		// /clear, on disk for the next start) makes it permanent.
		server, tool, ok := strings.Cut(q.Subject, "/")
		if !ok || server == "" || tool == "" {
			return nil
		}
		// The user-layer copy too: an untrusted workdir's restart never
		// merges the project layer, and the approve lists unite when it does.
		if s, ok := m.start.Config.MCP.Servers[server]; ok {
			s.Approve = append(slices.Clone(s.Approve), tool)
			m.start.Config.MCP.Servers[server] = s
		}
		pl := m.projectLayer()
		s := pl.MCP.Servers[server]
		s.Approve = append(slices.Clone(s.Approve), tool)
		if pl.MCP.Servers == nil {
			pl.MCP.Servers = map[string]config.MCPServer{}
		}
		pl.MCP.Servers[server] = s
		if err := config.AppendString(path, []string{"mcp", "servers", server, "approve"}, tool, nil); err != nil {
			return m.printlnError("error: allow-always not saved: " + err.Error())
		}
		return m.printlnContent(fmt.Sprintf("always allowing %s (saved to %s%s)", q.Subject, path, caveat))
	}
	return nil
}

// projectPath is the workdir's project config file ("" without a workdir).
func (m *model) projectPath() string {
	if m.start.Workdir == "" {
		return ""
	}
	return config.ProjectConfigFile(m.start.Workdir)
}

// projectLayer is the in-memory project config a /clear restart re-runs Start
// with; created on demand so an approval survives the restart even when the
// workdir had no project file yet.
func (m *model) projectLayer() *config.ProjectConfig {
	if m.start.Config.Project == nil {
		m.start.Config.Project = &config.ProjectConfig{}
	}
	return m.start.Config.Project
}

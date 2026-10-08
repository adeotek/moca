package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tools"
)

type AppOptions struct {
	Start      agent.StartOptions
	ConfigPath string
	Prompts    []skills.Prompt
	Home       string
	// ResumePath opens this session file (--resume/--continue) instead of
	// starting a new one.
	ResumePath string
}

type hintCheckMsg struct{}

type model struct {
	opts     AppOptions         // Workdir/ConfigPath/Prompts/Home for rendering and commands
	start    agent.StartOptions // opts.Start with Emit/Ask wired; /clear restarts from this copy
	agent    *agent.Agent
	input    *Input
	ta       textarea.Model
	items    Items
	live     strings.Builder
	thinking strings.Builder
	running  bool
	toolBusy string
	cancel   context.CancelFunc
	approval *approvalMsg
	pager    *pagerModel
	status   StatusInfo
	width    int
	height   int
	// kbdEnhanced: the terminal reports key disambiguation (shift+enter).
	kbdEnhanced   bool
	hintShown     bool
	lastAssistant string
	// lastTyped is when a key last edited the draft; `a`/`d` are ignored
	// shortly after, so a user mid-sentence does not answer a prompt that
	// pops up under their fingers (allow-always is ctrl+a — prose can never
	// trigger it).
	lastTyped time.Time
	// now is the model's clock; tests inject it so approval timing is
	// deterministic.
	now func() time.Time
	// shellBusy: a `!` command is in flight; its note must not land in the
	// middle of a run (or in the session /clear is about to replace).
	shellBusy bool
	// compacting: a /compact request is in flight; runs and state-changing
	// commands must wait for it (it appends to the transcript).
	compacting bool
	// compactCancel aborts an in-flight /compact (esc, quit): the summary
	// request inherits the provider's full retry ladder otherwise.
	compactCancel context.CancelFunc
	// held: scrollback output produced while the alt-screen pager is open.
	// tea.Println is lost there, so it is released when the pager closes.
	held []tea.Cmd
	// runDone closes when the run goroutine returns (quit waits for the abort
	// to reach the transcript).
	runDone chan struct{}
}

// approvalIdle is the typing pause required before `a`/`d` answer a prompt.
const approvalIdle = 700 * time.Millisecond

var (
	dim  = lipgloss.NewStyle().Faint(true)
	red  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	bold = lipgloss.NewStyle().Bold(true)
)

func newModel(o AppOptions, a *agent.Agent) *model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	// No prompt prefix: the input area renders the text alone (§11).
	ta.Prompt = ""
	ta.SetHeight(3)
	ta.Focus()
	m := &model{opts: o, start: o.Start, agent: a, input: NewInput(), ta: ta, now: time.Now}
	m.refreshStatus()
	return m
}

func (m *model) syncTextarea() { m.ta.SetValue(m.input.Buffer()) }
func (m *model) pullTextarea() { m.input.SetBuffer(m.ta.Value()) }

// println prints a trusted or already-styled line into the scrollback.
func println(s string) tea.Cmd { return tea.Println(s) }

// printlnContent is println for untrusted content (model output, tool output,
// user input, error strings): control bytes are neutralized so the terminal
// never interprets them. Styled lines must use println — running lipgloss
// output through Sanitize would display the escape codes as text.
func printlnContent(s string) tea.Cmd { return tea.Println(Sanitize(s)) }

func (m *model) refreshStatus() {
	m.status.Version = config.Version
	if m.agent == nil {
		return
	}
	st := m.agent.Status()
	m.status.Cwd = AbbrevHome(m.opts.Start.Workdir, m.opts.Home)
	m.status.Model, m.status.Effort = st.Model.Qualified(), st.Effort
	m.status.Window, m.status.Used = st.Window, st.ContextTokens
	m.status.In = st.Usage.Input + st.Usage.CacheRead + st.Usage.CacheWrite
	m.status.Out, m.status.Cost, m.status.Sub = st.Usage.Output, st.Cost, st.Sub
}

func (m *model) branchCmd() tea.Cmd {
	dir := m.opts.Start.Workdir
	return func() tea.Msg { b, d, g := GitBranch(dir); return branchMsg{b, d, g} }
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(println(welcomeText()), m.branchCmd(), tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return hintCheckMsg{} }))
}

// welcomeText opens the scrollback of a session: title + version, then the
// greeting.
func welcomeText() string {
	return bold.Render("moca") + " " + dim.Render(config.Version) + "\n" + "How can I help you today?"
}

// commitLive moves every completed line out of the live buffer and returns
// them; the trailing partial line stays live.
func (m *model) commitLive() []string {
	s := m.live.String()
	var lines []string
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			break
		}
		lines = append(lines, s[:i])
		s = s[i+1:]
	}
	m.live.Reset()
	m.live.WriteString(s)
	return lines
}

func (m *model) startRun(text string) tea.Cmd {
	if m.agent == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.running, m.cancel = true, cancel
	done := make(chan struct{})
	m.runDone = done
	m.live.Reset()
	m.thinking.Reset()
	m.toolBusy = ""
	a := m.agent
	return tea.Sequence(printlnContent("› "+text), func() tea.Msg {
		defer close(done)
		out, err := a.Run(ctx, text)
		return runDoneMsg{out: out, err: err}
	})
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ta.SetWidth(max(10, msg.Width-2))
		if m.pager != nil {
			m.pager.resize(msg.Width, msg.Height)
		}
		return m, nil
	case tea.KeyboardEnhancementsMsg:
		m.kbdEnhanced = msg.SupportsKeyDisambiguation()
		return m, nil
	case hintCheckMsg:
		if !m.kbdEnhanced && !m.hintShown {
			m.hintShown = true
			return m, m.hold(println("hint: this terminal can't report shift+enter; use alt+enter or ctrl+j for a newline"))
		}
		return m, nil
	case tea.PasteMsg:
		if m.pager == nil {
			// Insert at the cursor like typed text; a chip's marker goes there too.
			m.ta.InsertString(m.input.Prepare(msg.Content))
			m.pullTextarea()
			m.lastTyped = m.now()
		}
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case agentEventMsg:
		return m, m.hold(m.handleAgent(msg.e))
	case runDoneMsg:
		return m, m.hold(m.handleRunDone(msg))
	case approvalMsg:
		m.approval = &msg
		return m, nil
	case branchMsg:
		m.status.Branch, m.status.Dirty, m.status.Git = msg.branch, msg.dirty, msg.git
		return m, nil
	case shellDoneMsg:
		return m, m.hold(m.handleShellDone(msg))
	case compactDoneMsg:
		return m, m.hold(m.handleCompactDone(msg))
	}
	return m, nil
}

// hold defers a scrollback print while the pager (alt-screen) is open.
func (m *model) hold(cmd tea.Cmd) tea.Cmd {
	if m.pager == nil || cmd == nil {
		return cmd
	}
	m.held = append(m.held, cmd)
	return nil
}

// release returns the held output as one ordered command.
func (m *model) release() tea.Cmd {
	if len(m.held) == 0 {
		return nil
	}
	c := tea.Sequence(m.held...)
	m.held = nil
	return c
}

// quit cancels an in-flight run or compaction (its abort/return is recorded
// before exit) and exits.
func (m *model) quit() tea.Cmd {
	if m.compactCancel != nil {
		m.compactCancel()
	}
	if m.cancel != nil {
		m.cancel()
	}
	return tea.Quit
}

// waitRun gives the cancelled run goroutine a moment to record its abort.
func (m *model) waitRun(d time.Duration) {
	if m.runDone == nil {
		return
	}
	select {
	case <-m.runDone:
	case <-time.After(d):
	}
}

func (m *model) forward(k tea.KeyPressMsg) tea.Cmd {
	ta, cmd := m.ta.Update(k)
	m.ta = ta
	m.pullTextarea()
	m.lastTyped = m.now()
	return cmd
}

func (m *model) handleKey(k tea.KeyPressMsg) tea.Cmd {
	if m.pager != nil {
		if k.String() == "ctrl+c" {
			if m.input.CtrlC(time.Now()) {
				return m.quit()
			}
			return nil
		}
		open, cmd := m.pager.update(k)
		if !open {
			m.pager = nil
			return tea.Batch(cmd, m.release())
		}
		return cmd
	}
	if m.approval != nil {
		if cmd, handled := m.approvalKey(k); handled {
			return cmd
		}
		// Anything else (typing, enter, ctrl+c…) behaves as usual: the input
		// box is never dead while a prompt is shown.
	}
	switch k.String() {
	case "enter":
		return m.submit()
	case "shift+enter", "alt+enter", "ctrl+j":
		m.ta.InsertString("\n")
		m.pullTextarea()
		return nil
	case "esc":
		if m.compacting && m.compactCancel != nil {
			m.compactCancel()
			return nil
		}
		if m.running && m.cancel != nil {
			m.cancel()
		}
		return nil
	case "ctrl+c":
		if m.input.CtrlC(time.Now()) {
			return m.quit()
		}
		m.syncTextarea()
		return nil
	case "ctrl+o":
		if it, ok := m.items.Last(); ok {
			m.openPager(it)
		}
		return nil
	case "alt+p":
		m.input.ToggleChips()
		m.syncTextarea()
		return nil
	case "up":
		if m.ta.Line() == 0 && !strings.Contains(m.input.Buffer(), "\n") && m.input.HistoryPrev() {
			m.syncTextarea()
			return nil
		}
		return m.forward(k)
	case "down":
		if m.ta.Line() >= m.ta.LineCount()-1 && !strings.Contains(m.input.Buffer(), "\n") && m.input.HistoryNext() {
			m.syncTextarea()
			return nil
		}
		return m.forward(k)
	}
	return m.forward(k)
}

// approvalKey answers the shown prompt. `a`/`d` count only after a typing
// pause (a user mid-sentence must not answer with letters meant for the
// draft); allow-always is `ctrl+a` — it is persistent, so a plain letter (a
// stray capital `A`, the first letter of "Add…") always goes to the draft and
// can never write the config. esc always denies. handled=false lets the key
// fall through to normal input handling.
func (m *model) approvalKey(k tea.KeyPressMsg) (tea.Cmd, bool) {
	switch k.String() {
	case "esc":
		m.approval.reply <- tools.Deny
		m.approval = nil
		return nil, true
	case "a", "d":
		if m.now().Sub(m.lastTyped) < approvalIdle {
			return nil, false
		}
	}
	switch k.String() {
	case "a":
		m.approval.reply <- tools.AllowOnce
		m.approval = nil
		return nil, true
	case "d":
		m.approval.reply <- tools.Deny
		m.approval = nil
		return nil, true
	case "ctrl+a":
		if !m.approval.q.CanAlways {
			return nil, true // not offered for ask-every-time commands
		}
		q, reply := m.approval.q, m.approval.reply
		m.approval = nil
		reply <- tools.AllowAlways
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
				return printlnContent("error: allow-always not saved: " + err.Error()), true
			}
			return printlnContent(fmt.Sprintf("always allowing %q (saved to %s)", q.Subject, path)), true
		case "mcp":
			// Subject is server/tool. The running session already learned it
			// from the AllowAlways answer; the config makes it survive /clear
			// and the next start.
			server, tool, ok := strings.Cut(q.Subject, "/")
			if !ok || server == "" || tool == "" {
				return nil, true
			}
			if s, ok := m.start.Config.MCP.Servers[server]; ok {
				s.Approve = append(slices.Clone(s.Approve), tool)
				m.start.Config.MCP.Servers[server] = s
			}
			if err := config.AppendString(path, []string{"mcp", "servers", server, "approve"}, tool, nil); err != nil {
				return printlnContent("error: allow-always not saved: " + err.Error()), true
			}
			return printlnContent(fmt.Sprintf("always allowing %s (saved to %s)", q.Subject, path)), true
		}
		return nil, true
	}
	return nil, false
}

func (m *model) openPager(it Item) {
	m.pager = newPager(it, max(20, m.width), max(6, m.height))
}

// submit classifies and dispatches the input line (enter).
func (m *model) submit() tea.Cmd {
	text := m.input.Text()
	if strings.TrimSpace(text) == "" {
		return nil
	}
	parsed, err := ParseInput(text, m.opts.Prompts)
	m.input.Submit()
	m.syncTextarea()
	if err != nil {
		return printlnContent("error: " + err.Error())
	}
	switch parsed.Kind {
	case KindText, KindPrompt:
		if m.running {
			if m.agent != nil {
				m.agent.Steer(parsed.Text)
			}
			return printlnContent("↳ queued: " + firstLineOf(parsed.Text))
		}
		if m.compacting {
			return println("a /compact is still running — wait for it to finish")
		}
		if m.shellBusy {
			return m.refuseBusy()
		}
		return m.startRun(parsed.Text)
	case KindCommand:
		return m.runCommand(parsed)
	case KindShell:
		if m.running {
			// `!` output enters the transcript; appending it between a tool
			// batch's results would diverge from the wire ordering the
			// rebuild guarantees. `!!` (local-only) stays available.
			return printlnContent("finish or interrupt the run first (esc) — !! runs locally now")
		}
		if m.compacting {
			// A note appended while a compaction summarizes could be excluded
			// from the summary (and land before the cut boundary, vanishing
			// from the rebuilt context).
			return println("a /compact is still running — wait for it to finish")
		}
		if m.shellBusy {
			return m.refuseBusy()
		}
		m.shellBusy = true
		return m.shellCmd(false, parsed.Text)
	case KindShellLocal:
		return m.shellCmd(true, parsed.Text)
	}
	return nil
}

// refuseRunning guards commands that mutate the running conversation.
func (m *model) refuseRunning() tea.Cmd {
	if m.shellBusy {
		return m.refuseBusy()
	}
	if m.compacting {
		return println("a /compact is still running — wait for it to finish")
	}
	if !m.running {
		return nil
	}
	return println("finish or interrupt the run first (esc)")
}

func (m *model) refuseBusy() tea.Cmd {
	return println("a ! command is still running — wait for it to finish")
}

func (m *model) runCommand(c Parsed) tea.Cmd {
	if m.agent == nil {
		return nil
	}
	st := m.agent.Status()
	switch c.Name {
	case "model":
		if c.Args == "" {
			var lines []string
			for _, mo := range m.agent.Models() {
				mark := "  "
				if mo.Qualified() == st.Model.Qualified() {
					mark = "* "
				}
				lines = append(lines, mark+mo.Qualified())
			}
			return printlnContent(strings.Join(lines, "\n"))
		}
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		if err := m.agent.SetModel(c.Args, ""); err != nil {
			return printlnContent("error: " + err.Error())
		}
		m.refreshStatus()
		s := m.agent.Status()
		return printlnContent(fmt.Sprintf("switched to %s · effort %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
	case "effort":
		if c.Args == "" {
			return printlnContent(fmt.Sprintf("effort %s — supported: %s", AbbrevEffort(st.Effort), supportedEfforts(st.Model)))
		}
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		want, err := llm.ParseEffort(c.Args)
		if err != nil {
			return printlnContent("error: " + err.Error())
		}
		got, err := m.agent.SetEffort(want)
		if err != nil {
			return printlnContent("error: " + err.Error())
		}
		m.refreshStatus()
		note := ""
		if got != want {
			note = fmt.Sprintf(" (clamped from %s)", want)
		}
		return printlnContent("effort " + AbbrevEffort(got) + note)
	case "hard":
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		on, err := m.agent.ToggleHard()
		if err != nil {
			return printlnContent("error: " + err.Error())
		}
		m.refreshStatus()
		s := m.agent.Status()
		if on {
			return printlnContent(fmt.Sprintf("hard mode on: %s · %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
		}
		return printlnContent(fmt.Sprintf("hard mode off: back to %s · %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
	case "yolo":
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		m.agent.SetYolo(!m.agent.Yolo()) // YoloChanged prints + refreshes
		return nil
	case "clear":
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		return m.restartSession()
	case "compact":
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		m.compacting = true
		m.status.Transient = "compacting…"
		return m.compactCmd()
	case "cost":
		u := st.Usage
		return println(fmt.Sprintf("in %d · out %d · cache read %d · cache write %d · $%.4f", u.Input, u.Output, u.CacheRead, u.CacheWrite, st.Cost))
	case "undo":
		msg, err := m.agent.Undo()
		if err != nil {
			return printlnContent("error: " + err.Error())
		}
		return printlnContent(msg)
	case "copy":
		if m.lastAssistant == "" {
			return println("nothing to copy yet")
		}
		return tea.Sequence(tea.SetClipboard(m.lastAssistant), println(fmt.Sprintf("copied %d chars (OSC 52)", len(m.lastAssistant))))
	case "show":
		n, err := strconv.Atoi(c.Args)
		if err != nil {
			return println("usage: /show <n>")
		}
		it, ok := m.items.Get(n)
		if !ok {
			return println(fmt.Sprintf("no item #%d yet", n))
		}
		m.openPager(it)
		return nil
	case "exit":
		return m.quit()
	case "help":
		return printlnContent(HelpText(m.opts.Prompts))
	}
	return println("unknown command /" + c.Name)
}

// compactCmd runs a manual compaction off the event loop. The Compacted
// event (through the pipe) prints the result; the done message only reports
// refusals and errors. esc/quit cancel it through compactCancel.
func (m *model) compactCmd() tea.Cmd {
	a := m.agent
	ctx, cancel := context.WithCancel(context.Background())
	m.compactCancel = cancel
	return func() tea.Msg {
		return compactDoneMsg{err: a.Compact(ctx)}
	}
}

func (m *model) handleCompactDone(msg compactDoneMsg) tea.Cmd {
	m.compacting = false
	m.compactCancel = nil
	m.status.Transient = ""
	m.refreshStatus()
	switch {
	case msg.err == nil:
		return nil // the Compacted event line reports the result
	case errors.Is(msg.err, context.Canceled):
		return println("compaction cancelled")
	case errors.Is(msg.err, agent.ErrNothingToCompact):
		return println("nothing to compact")
	default:
		return printlnContent("error: " + msg.err.Error())
	}
}

func (m *model) restartSession() tea.Cmd {
	// Start the replacement first: on failure the old session must stay
	// open and wired, or the next submit appends onto a closed writer.
	// The replacement keeps the model/effort in use (hard mode exits to its
	// saved pair); m.start.Config already holds the commands approved with [A].
	opts := m.start
	if m.agent != nil {
		mo, ef := m.agent.Carry()
		opts.Model, opts.Effort = mo, string(ef)
	}
	a, err := agent.Start(opts)
	if err != nil {
		return printlnContent("error: " + err.Error())
	}
	if m.agent != nil {
		m.agent.Close()
	}
	m.agent = a
	m.start = opts
	m.items = Items{}
	m.live.Reset()
	m.thinking.Reset()
	m.status.Transient = ""
	m.refreshStatus()
	// The bar's branch survives /clear; re-resolve it for the new session.
	return tea.Sequence(println(fmt.Sprintf("new session %s (previous stays resumable)", a.Session().ID8())), m.branchCmd())
}

func (m *model) shellCmd(local bool, cmd string) tea.Cmd {
	wd := m.opts.Start.Workdir
	env := tools.ShellEnv(os.Environ(), config.EnvRefs(m.opts.Start.Config))
	return func() tea.Msg {
		out, err := tools.RunShell(context.Background(), wd, env, cmd, 30*time.Second)
		return shellDoneMsg{local: local, cmd: cmd, out: out, err: err}
	}
}

func (m *model) handleShellDone(msg shellDoneMsg) tea.Cmd {
	if !msg.local {
		m.shellBusy = false
	}
	var lines []string
	lines = append(lines, "$ "+Sanitize(msg.cmd))
	if msg.err != nil {
		lines = append(lines, "error: "+Sanitize(msg.err.Error()))
		return println(strings.Join(lines, "\n"))
	}
	body := tools.Truncate(msg.out.Output, 30_000)
	if b := strings.TrimRight(Sanitize(body), "\n"); b != "" {
		lines = append(lines, b)
	}
	exit := fmt.Sprintf("[exit %d]", msg.out.ExitCode)
	if msg.out.TimedOut {
		exit = "[timed out after 30s — process group killed]"
	}
	lines = append(lines, exit)
	if !msg.local && m.agent != nil {
		note := "$ " + msg.cmd + "\n" + body + "\n" + exit
		if err := m.agent.AddNote(note); err != nil {
			lines = append(lines, "error: "+Sanitize(err.Error()))
		}
	}
	return println(strings.Join(lines, "\n"))
}

func (m *model) handleAgent(e agent.Event) tea.Cmd {
	switch e := e.(type) {
	case agent.TextDelta:
		m.live.WriteString(e.Text)
		if lines := m.commitLive(); len(lines) > 0 {
			return printlnContent(strings.Join(lines, "\n"))
		}
		return nil
	case agent.ThinkingDelta:
		m.thinking.WriteString(e.Text)
		return nil
	case agent.StreamReset:
		m.live.Reset()
		m.thinking.Reset()
		return println("[stream interrupted — retrying]")
	case agent.ToolStart:
		m.toolBusy = e.Call.Name
		return nil
	case agent.ToolEnd:
		m.toolBusy = ""
		it := m.items.AddTool(e.Call, e.Result)
		m.refreshStatus()
		return println(it.Line)
	case agent.TurnEnd:
		m.toolBusy = ""
		var cmds []tea.Cmd
		if rest := m.live.String(); rest != "" {
			cmds = append(cmds, printlnContent(rest))
		}
		m.live.Reset()
		if m.thinking.Len() > 0 {
			it := m.items.AddThinking(m.thinking.String())
			cmds = append(cmds, println(it.Line))
		}
		m.thinking.Reset()
		m.lastAssistant = llm.TextOf(e.Message)
		m.status.Transient = ""
		m.refreshStatus()
		cmds = append(cmds, m.branchCmd())
		return tea.Sequence(cmds...)
	case agent.Retry:
		m.status.Transient = fmt.Sprintf("retry %d/%d · %s", e.Notice.Attempt, e.Notice.Max, e.Notice.Wait.Round(1e8))
		return nil
	case agent.Warning:
		return printlnContent("warning: " + e.Text)
	case agent.Compacted:
		m.status.Transient = ""
		m.refreshStatus()
		return println(fmt.Sprintf("⋯ compacted: %d → %d tokens", e.TokensBefore, e.TokensAfter))
	case agent.Resumed:
		return printlnContent(fmt.Sprintf("resumed %s (%d messages) — earlier output is in the session file", e.ID8, e.Messages))
	case agent.SteeringApplied:
		return printlnContent("↳ sent: " + strings.Join(e.Texts, " · "))
	case agent.YoloChanged:
		m.refreshStatus()
		if e.On {
			return println(red.Render("yolo mode on: all permission checks are off"))
		}
		return println("yolo mode off: permission checks restored")
	}
	return nil
}

func (m *model) handleRunDone(msg runDoneMsg) tea.Cmd {
	m.running, m.cancel = false, nil
	m.toolBusy = ""
	var cmds []tea.Cmd
	// The event pipe decouples delivery, so this can run before the final
	// TurnEnd is processed. Flush the trailing partial line unconditionally:
	// on an errored/interrupted run it is the visible tail of what the model
	// streamed (display-only; the transcript holds the persisted text) —
	// leaving it in the live region would freeze it there and the next run
	// would silently drop it.
	if m.live.Len() > 0 {
		cmds = append(cmds, printlnContent(m.live.String()))
	}
	m.live.Reset()
	m.thinking.Reset()
	// A run that dies mid-retry (/clear aside) must not leave the transient
	// in the cost field.
	m.status.Transient = ""
	if m.agent != nil {
		if left := m.agent.TakeSteering(); len(left) > 0 {
			m.input.Prepend(left)
			m.syncTextarea()
		}
	}
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			cmds = append(cmds, println("[interrupted]"))
		} else {
			cmds = append(cmds, printlnContent("error: "+msg.err.Error()))
		}
	}
	m.refreshStatus()
	cmds = append(cmds, m.branchCmd())
	return tea.Sequence(cmds...)
}

func (m *model) View() tea.View {
	if m.pager != nil {
		v := tea.NewView(m.pager.view())
		v.AltScreen = true
		return v
	}
	var sb strings.Builder
	if m.live.Len() > 0 {
		sb.WriteString(Sanitize(m.live.String()))
		sb.WriteString("\n")
	}
	if m.running && m.toolBusy != "" {
		sb.WriteString(dim.Render("… "+Sanitize(m.toolBusy)) + "\n")
	}
	if m.thinking.Len() > 0 && m.running {
		sb.WriteString(dim.Render("⋯ thinking…") + "\n")
	}
	if m.approval != nil {
		sb.WriteString(approvalPrompt(m.approval.q) + "\n")
	}
	sb.WriteString(m.rule())
	sb.WriteString(m.ta.View())
	sb.WriteString("\n")
	sb.WriteString(m.rule())
	sb.WriteString(m.statusLine())
	v := tea.NewView(sb.String())
	// Ask for full key disambiguation so shift+enter is distinguishable.
	v.KeyboardEnhancements = tea.KeyboardEnhancements{ReportAllKeysAsEscapeCodes: true}
	return v
}

// statusLine renders the two-line bar; in yolo mode a red YOLO field leads
// line 1 and is never dropped — the rest gets the remaining width.
func (m *model) statusLine() string {
	w := max(20, m.width)
	if m.agent != nil && m.agent.Yolo() {
		l1, l2 := RenderStatus(m.status, max(13, w-7))
		return red.Render("YOLO") + dim.Render(" · "+Sanitize(l1)) + "\n" + dim.Render(Sanitize(l2))
	}
	l1, l2 := RenderStatus(m.status, w)
	return dim.Render(Sanitize(l1)) + "\n" + dim.Render(Sanitize(l2))
}

// rule is the full-width dim separator between the scrollback output, the
// input area and the status bar.
func (m *model) rule() string {
	return dim.Render(strings.Repeat("─", max(20, m.width))) + "\n"
}

func approvalPrompt(q tools.Question) string {
	subject, detail := Sanitize(q.Subject), Sanitize(firstLineOf(q.Detail))
	if q.CanAlways {
		return fmt.Sprintf("allow `%s`?  [a] once  [ctrl+a] always  [d] deny   — %s", subject, detail)
	}
	return fmt.Sprintf("allow `%s` (asks every time)?  [a] once  [d] deny   — %s", subject, detail)
}

func firstLineOf(s string) string { l, _, _ := strings.Cut(s, "\n"); return l }

// supportedEfforts lists a model's supported effort levels for /effort.
func supportedEfforts(mo provider.Model) string {
	if mo.ThinkingMode == "none" || len(mo.ThinkingLevelMap) == 0 {
		return "none"
	}
	var out []string
	for _, e := range llm.Efforts {
		if _, ok := mo.ThinkingLevelMap[e]; ok {
			out = append(out, string(e))
		}
	}
	return strings.Join(out, " ")
}

// Run creates the program in inline mode and runs it until quit.
func Run(ctx context.Context, o AppOptions) error {
	var p *tea.Program
	// p is captured by the closures below. Events are delivered through a
	// FIFO pipe so an emit from inside Update (a slash command toggling
	// yolo, /clear restarting the session) can never block the event loop on
	// Program.Send (bridge.go); the pipe buffers events emitted before the
	// program exists (agent.Start warnings, Resume's Resumed line). The
	// asker is called from the agent's run goroutine only, where a direct
	// blocking send is correct.
	pipe := newEventPipe()
	o.Start.Emit = pipe.emit
	o.Start.Ask = newAsker(func(msg tea.Msg) { p.Send(msg) })
	var a *agent.Agent
	var err error
	if o.ResumePath != "" {
		a, err = agent.Resume(o.Start, o.ResumePath)
	} else {
		a, err = agent.Start(o.Start)
	}
	if err != nil {
		close(pipe.stop)
		return err
	}
	if o.ResumePath != "" {
		// The session's own workdir (not the invoking cwd) drives the status
		// bar, `!` commands and the branch lookup.
		o.Start.Workdir = a.Workdir()
	}
	m := newModel(o, a)
	m.refreshStatus()
	// main's signal context already delivers SIGINT/SIGTERM; Bubble Tea's own
	// handler would race it at shutdown (and can deadlock Program.Run).
	p = tea.NewProgram(m, tea.WithContext(ctx), tea.WithoutSignalHandler())
	pipe.start(func(msg tea.Msg) { p.Send(msg) })
	_, err = p.Run()
	// Cancel any run still going and let it record its abort before the
	// session closes (the program is done, so the model is ours alone).
	if m.cancel != nil {
		m.cancel()
	}
	m.waitRun(2 * time.Second)
	close(pipe.stop)
	m.agent.Close()
	return runResult(ctx, err)
}

// runResult maps Program.Run's error to the caller's: a clean quit is nil;
// SIGINT/SIGTERM (the parent context) surface as cancellation so the CLI exits
// 130; a recovered panic stays an error. Bubble Tea reports all of these
// wrapped in ErrProgramKilled, so that sentinel alone says nothing.
func runResult(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tea.ErrProgramPanic):
		return err
	case ctx.Err() != nil:
		return ctx.Err()
	}
	return err
}

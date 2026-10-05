package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
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
}

type hintCheckMsg struct{}

type model struct {
	opts     AppOptions
	start    agent.StartOptions // Emit/Ask wired; /clear restarts from this
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
}

var (
	dim = lipgloss.NewStyle().Faint(true)
	red = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)

func newModel(o AppOptions, a *agent.Agent) *model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = "› "
	ta.SetHeight(3)
	ta.Focus()
	m := &model{opts: o, start: o.Start, agent: a, input: NewInput(), ta: ta}
	m.refreshStatus()
	return m
}

func (m *model) syncTextarea() { m.ta.SetValue(m.input.Buffer()) }
func (m *model) pullTextarea() { m.input.SetBuffer(m.ta.Value()) }

func println(s string) tea.Cmd { return tea.Println(Sanitize(s)) }

func (m *model) refreshStatus() {
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
	return tea.Batch(m.branchCmd(), tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return hintCheckMsg{} }))
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
	m.live.Reset()
	m.thinking.Reset()
	m.toolBusy = ""
	a := m.agent
	return tea.Sequence(println("› "+text), func() tea.Msg {
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
			return m, println("hint: this terminal can't report shift+enter; use alt+enter or ctrl+j for a newline")
		}
		return m, nil
	case tea.PasteMsg:
		if m.pager == nil {
			m.input.Paste(msg.Content)
			m.syncTextarea()
		}
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case agentEventMsg:
		return m, m.handleAgent(msg.e)
	case runDoneMsg:
		return m, m.handleRunDone(msg)
	case approvalMsg:
		m.approval = &msg
		return m, nil
	case branchMsg:
		m.status.Branch, m.status.Dirty, m.status.Git = msg.branch, msg.dirty, msg.git
		return m, nil
	case shellDoneMsg:
		return m, m.handleShellDone(msg)
	}
	return m, nil
}

func (m *model) forward(k tea.KeyPressMsg) tea.Cmd {
	ta, cmd := m.ta.Update(k)
	m.ta = ta
	m.pullTextarea()
	return cmd
}

func (m *model) handleKey(k tea.KeyPressMsg) tea.Cmd {
	if m.pager != nil {
		if k.String() == "ctrl+c" {
			if m.input.CtrlC(time.Now()) {
				return tea.Quit
			}
			return nil
		}
		open, cmd := m.pager.update(k)
		if !open {
			m.pager = nil
		}
		return cmd
	}
	if m.approval != nil {
		switch k.String() {
		case "a":
			m.approval.reply <- tools.AllowOnce
			m.approval = nil
		case "A":
			if !m.approval.q.CanAlways {
				return nil // not offered for ask-every-time commands
			}
			q, reply := m.approval.q, m.approval.reply
			m.approval = nil
			reply <- tools.AllowAlways
			if q.Kind == "shell" {
				path := m.opts.ConfigPath
				if path == "" {
					path = config.ConfigFile()
				}
				if err := config.AppendString(path, []string{"shell", "allow"}, q.Subject, config.DefaultShellAllow); err != nil {
					return println("error: allow-always not saved: " + err.Error())
				}
				return println(fmt.Sprintf("always allowing %q (saved to %s)", q.Subject, path))
			}
		case "d", "esc":
			m.approval.reply <- tools.Deny
			m.approval = nil
		default:
			// The approval keys go to the prompt (or nowhere); any other key
			// still edits the draft — the input box is never dead.
			return m.forward(k)
		}
		return nil
	}
	switch k.String() {
	case "enter":
		return m.submit()
	case "shift+enter", "alt+enter", "ctrl+j":
		m.ta.InsertString("\n")
		m.pullTextarea()
		return nil
	case "esc":
		if m.running && m.cancel != nil {
			m.cancel()
		}
		return nil
	case "ctrl+c":
		if m.input.CtrlC(time.Now()) {
			if m.cancel != nil {
				m.cancel()
			}
			return tea.Quit
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
		return println("error: " + err.Error())
	}
	switch parsed.Kind {
	case KindText, KindPrompt:
		if m.running {
			if m.agent != nil {
				m.agent.Steer(parsed.Text)
			}
			return println("↳ queued: " + firstLineOf(parsed.Text))
		}
		return m.startRun(parsed.Text)
	case KindCommand:
		return m.runCommand(parsed)
	case KindShell, KindShellLocal:
		return m.shellCmd(parsed.Kind == KindShellLocal, parsed.Text)
	}
	return nil
}

// refuseRunning guards commands that mutate the running conversation.
func (m *model) refuseRunning() tea.Cmd {
	if !m.running {
		return nil
	}
	return println("finish or interrupt the run first (esc)")
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
			return println(strings.Join(lines, "\n"))
		}
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		if err := m.agent.SetModel(c.Args, ""); err != nil {
			return println("error: " + err.Error())
		}
		m.refreshStatus()
		s := m.agent.Status()
		return println(fmt.Sprintf("switched to %s · effort %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
	case "effort":
		if c.Args == "" {
			return println(fmt.Sprintf("effort %s — supported: %s", AbbrevEffort(st.Effort), supportedEfforts(st.Model)))
		}
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		want, err := llm.ParseEffort(c.Args)
		if err != nil {
			return println("error: " + err.Error())
		}
		got, err := m.agent.SetEffort(want)
		if err != nil {
			return println("error: " + err.Error())
		}
		m.refreshStatus()
		note := ""
		if got != want {
			note = fmt.Sprintf(" (clamped from %s)", want)
		}
		return println("effort " + AbbrevEffort(got) + note)
	case "hard":
		if cmd := m.refuseRunning(); cmd != nil {
			return cmd
		}
		on, err := m.agent.ToggleHard()
		if err != nil {
			return println("error: " + err.Error())
		}
		m.refreshStatus()
		s := m.agent.Status()
		if on {
			return println(fmt.Sprintf("hard mode on: %s · %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
		}
		return println(fmt.Sprintf("hard mode off: back to %s · %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
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
		return println("compaction lands in phase 4")
	case "cost":
		u := st.Usage
		return println(fmt.Sprintf("in %d · out %d · cache read %d · cache write %d · $%.4f", u.Input, u.Output, u.CacheRead, u.CacheWrite, st.Cost))
	case "undo":
		msg, err := m.agent.Undo()
		if err != nil {
			return println("error: " + err.Error())
		}
		return println(msg)
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
	case "help":
		return println(HelpText(m.opts.Prompts))
	}
	return println("unknown command /" + c.Name)
}

func (m *model) restartSession() tea.Cmd {
	if m.agent != nil {
		m.agent.Session().Close()
	}
	a, err := agent.Start(m.start)
	if err != nil {
		return println("error: " + err.Error())
	}
	m.agent = a
	m.items = Items{}
	m.live.Reset()
	m.thinking.Reset()
	m.status = StatusInfo{}
	m.refreshStatus()
	return println(fmt.Sprintf("new session %s (previous stays resumable)", a.Session().ID8()))
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
			return println(strings.Join(lines, "\n"))
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
			cmds = append(cmds, println(rest))
		}
		m.live.Reset()
		if m.thinking.Len() > 0 {
			it := m.items.AddThinking(m.thinking.String())
			cmds = append(cmds, println(it.Line))
		}
		m.thinking.Reset()
		m.lastAssistant = messageText(e.Message)
		m.status.Transient = ""
		m.refreshStatus()
		cmds = append(cmds, m.branchCmd())
		return tea.Sequence(cmds...)
	case agent.Retry:
		m.status.Transient = fmt.Sprintf("retry %d/%d · %s", e.Notice.Attempt, e.Notice.Max, e.Notice.Wait.Round(1e8))
		return nil
	case agent.Warning:
		return println("warning: " + e.Text)
	case agent.SteeringApplied:
		return println("↳ sent: " + strings.Join(e.Texts, " · "))
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
	m.live.Reset()
	m.thinking.Reset()
	var cmds []tea.Cmd
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
			cmds = append(cmds, println("error: "+msg.err.Error()))
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
	sb.WriteString(m.ta.View())
	sb.WriteString("\n")
	sb.WriteString(m.statusLine())
	v := tea.NewView(sb.String())
	// Ask for full key disambiguation so shift+enter is distinguishable.
	v.KeyboardEnhancements = tea.KeyboardEnhancements{ReportAllKeysAsEscapeCodes: true}
	return v
}

// statusLine renders the bar; in yolo mode a red YOLO field leads it and is
// never dropped — the rest gets the remaining width.
func (m *model) statusLine() string {
	w := max(20, m.width)
	if m.agent != nil && m.agent.Yolo() {
		return red.Render("YOLO") + dim.Render(" · "+RenderStatus(m.status, w-7))
	}
	return dim.Render(RenderStatus(m.status, w))
}

func approvalPrompt(q tools.Question) string {
	if q.CanAlways {
		return fmt.Sprintf("allow `%s`?  [a] once  [A] always  [d] deny   — %s", q.Subject, firstLineOf(q.Detail))
	}
	return fmt.Sprintf("allow `%s` (asks every time)?  [a] once  [d] deny   — %s", q.Subject, firstLineOf(q.Detail))
}

func firstLineOf(s string) string { l, _, _ := strings.Cut(s, "\n"); return l }

func messageText(m llm.Message) string {
	var sb strings.Builder
	for _, c := range m.Content {
		if c.Type == llm.BlockText {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

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
	m := newModel(o, nil)
	// p is captured by the closures before assignment; events cannot fire
	// before p.Run() starts, because the agent only runs on user input.
	o.Start.Emit = func(e agent.Event) { p.Send(agentEventMsg{e}) }
	o.Start.Ask = newAsker(func(msg tea.Msg) { p.Send(msg) })
	a, err := agent.Start(o.Start)
	if err != nil {
		return err
	}
	m.agent, m.start = a, o.Start
	m.refreshStatus()
	p = tea.NewProgram(m, tea.WithContext(ctx))
	_, err = p.Run()
	a.Session().Close()
	if errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}

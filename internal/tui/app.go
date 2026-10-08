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

// printedMsg acknowledges that a scrollback flush reached the renderer; the
// next queued output may go out (see emit).
type printedMsg struct{}

// releaseMsg ends the pager's hold on scrollback output once the renderer has
// left the alt screen; pagerSettle is a few renderer frames.
type releaseMsg struct{}

const pagerSettle = 50 * time.Millisecond

// spinMsg advances the activity spinner; gen ties it to one run so a stale
// tick from a finished run never starts a second chain.
type spinMsg struct{ gen int }

const spinInterval = 100 * time.Millisecond

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type model struct {
	opts     AppOptions         // Workdir/ConfigPath/Prompts/Home for rendering and commands
	start    agent.StartOptions // opts.Start with Emit/Ask wired; /clear restarts from this copy
	agent    *agent.Agent
	input    *Input
	ta       textarea.Model
	items    Items
	live     strings.Builder
	thinking strings.Builder
	// thinkStart is when the open thinking block's first delta arrived; the
	// item line reports the block's duration (zero when the block was not
	// accumulated from deltas — e.g. a resumed or test-built one).
	thinkStart time.Time
	running    bool
	toolBusy   string
	cancel     context.CancelFunc
	approval   *approvalMsg
	// login is the active /login//logout interaction (modal, like approval).
	login *loginState
	// drop is the live /command autocomplete state (derived from the draft;
	// see dropdown.go).
	drop   *dropState
	pager  *pagerModel
	status StatusInfo
	width  int
	height int
	// darkBG: the terminal's reported background (defaults dark) — picks the
	// scrollback message background variants (tea.BackgroundColorMsg).
	darkBG bool
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
	// outq holds scrollback output waiting for the in-flight flush to land;
	// outBusy says one is in flight (see emit).
	outq    []string
	outBusy bool
	// pipe is the ordered path into the event loop (nil in unit tests): the
	// run's completion goes through it so it cannot overtake streamed events.
	pipe *eventPipe
	// guard bounds the cursor row the inline renderer may still remember and
	// settling says a settleMsg is already scheduled (padForShrink).
	guard    padGuard
	settling bool
	// runStart, runGen and spin drive the activity line: when the run began,
	// which run's ticks are live, and the spinner frame.
	runStart time.Time
	runGen   int
	spin     int
}

// thinkingHint is appended to a printed thinking line: the scrollback is
// immutable, so the full reasoning opens in the pager.
const thinkingHint = " · alt+t to read"

// approvalIdle is the typing pause required before `a`/`d` answer a prompt.
const approvalIdle = 700 * time.Millisecond

// inputChrome is the number of terminal rows the input box leaves for the
// rest of the screen: it grows with the draft but never past height minus
// this, so the frame always fits and the top of the scrollback stays visible.
// Longer drafts scroll inside the box.
const inputChrome = 8

// guardWindow is the gap between Views that proves the renderer flushed the
// earlier frame: frames are drawn at most one frame interval apart, so after
// guardWindow the previous View is what the renderer remembers. A shorter gap
// means the Views may coalesce and only the burst's last one gets drawn (see
// padForShrink); it comfortably exceeds the renderer's flush interval.
const guardWindow = 60 * time.Millisecond

// padGuard tracks cursor rows across Views for padForShrink. lastRow is the
// row of the most recent View, prevRow the row of the last View before the
// current burst of Views, and burstRow the deepest row seen within the burst.
// A gap of at least guardWindow between Views starts a new burst: by then the
// renderer has certainly flushed the earlier frame, so its row is the one it
// remembers; within a burst the remembered row is still prevRow.
type padGuard struct {
	lastRow  int
	lastAt   time.Time
	prevRow  int
	burstRow int
}

// settleMsg re-renders once the shrink-guard window has passed, dropping the
// padding rows (see padForShrink).
type settleMsg struct{}

var (
	dim = lipgloss.NewStyle().Faint(true)
	red = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	// orange marks the product name in the welcome line. One shade serves
	// both light and dark terminals: the welcome prints before the terminal
	// reports its background, and #d97706 keeps ≥3:1 contrast on white and
	// 4.4+ on the common dark backgrounds.
	orange = lipgloss.NewStyle().Foreground(lipgloss.Color("#d97706")).Bold(true)
)

func newModel(o AppOptions, a *agent.Agent) *model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	// No prompt prefix: the input area renders the text alone (§11).
	ta.Prompt = ""
	ta.Placeholder = "Message moca…  (/help for commands)"
	// The box grows and shrinks with the draft, from one row up to what fits
	// on screen (MaxHeight follows the window size), then scrolls.
	// MaxContentHeight lifts the legacy rule that also blocks newlines once
	// MaxHeight logical lines exist (MaxHeight then only sizes the viewport).
	// The default cursor-line highlight (a black band) is dropped. The cursor
	// is the terminal's own (see View).
	ta.DynamicHeight, ta.MinHeight, ta.MaxHeight, ta.MaxContentHeight = true, 1, 8, 10000
	ta.SetVirtualCursor(false)
	st := textarea.DefaultStyles(true)
	st.Focused.CursorLine = lipgloss.NewStyle()
	st.Focused.Placeholder = mutedFg
	st.Blurred.Placeholder = mutedFg
	ta.SetStyles(st)
	ta.Focus()
	m := &model{opts: o, start: o.Start, agent: a, input: NewInput(), ta: ta, now: time.Now, darkBG: true}
	m.refreshStatus()
	return m
}

func (m *model) syncTextarea() { m.ta.SetValue(m.input.Buffer()) }
func (m *model) pullTextarea() { m.input.SetBuffer(m.ta.Value()) }

// emit queues s for the scrollback and returns the command that prints it
// (nil while an earlier flush is still in flight — that flush's
// acknowledgement sends the queue on). Bubble Tea runs every command Update
// returns on its own goroutine, so two plain tea.Println commands returned by
// back-to-back Updates — a burst of streamed lines — can reach the renderer
// out of order. One flush at a time, acknowledged through printedMsg, keeps
// the scrollback in emission order.
func (m *model) emit(s string) tea.Cmd {
	m.outq = append(m.outq, s)
	if m.outBusy {
		return nil
	}
	return m.flush()
}

// flush takes the whole queue as one print followed by its acknowledgement.
func (m *model) flush() tea.Cmd {
	if len(m.outq) == 0 {
		m.outBusy = false
		return nil
	}
	body := strings.Join(m.outq, "\n")
	m.outq, m.outBusy = nil, true
	return tea.Sequence(tea.Println(body), func() tea.Msg { return printedMsg{} })
}

// println prints a trusted or already-styled line into the scrollback.
func (m *model) println(s string) tea.Cmd { return m.emit(s) }

// printlnContent is println for untrusted content (model output, tool output,
// user input, error strings): control bytes are neutralized so the terminal
// never interprets them. Styled lines must use println — running lipgloss
// output through Sanitize would display the escape codes as text.
func (m *model) printlnContent(s string) tea.Cmd { return m.emit(Sanitize(s)) }

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
	// RequestBackgroundColor asks the terminal for its background (OSC 11);
	// the reply arrives as tea.BackgroundColorMsg and picks the message
	// background variants. No reply (unsupported terminal) keeps the dark
	// default set in newModel.
	return tea.Batch(m.println(welcomeText()), m.branchCmd(), tea.RequestBackgroundColor, tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return hintCheckMsg{} }))
}

// welcomeText opens the scrollback of a session: title + version, then the
// greeting. The product name renders in orange (readable on light and dark
// backgrounds alike — the welcome prints before the background is known).
func welcomeText() string {
	return orange.Render("moca") + " " + dim.Render(config.Version) + "\n" + "How can I help you today?"
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
	m.runStart, m.spin = m.now(), 0
	m.runGen++
	a, pipe := m.agent, m.pipe
	return tea.Batch(tea.Sequence(m.printlnUser("› "+text), func() tea.Msg {
		defer close(done)
		out, err := a.Run(ctx, text)
		msg := runDoneMsg{out: out, err: err}
		if pipe != nil {
			pipe.send(msg) // behind every event the run emitted
			return nil
		}
		return msg
	}), m.spinTick())
}

// spinTick schedules the next spinner frame of the current run.
func (m *model) spinTick() tea.Cmd {
	gen := m.runGen
	return tea.Tick(spinInterval, func(time.Time) tea.Msg { return spinMsg{gen} })
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ta.MaxHeight = max(1, msg.Height-inputChrome)
		m.ta.SetWidth(max(10, msg.Width)) // also re-fits the box height
		if m.pager != nil {
			m.pager.resize(msg.Width, msg.Height)
		}
		return m, nil
	case releaseMsg:
		if m.pager != nil {
			return m, nil // reopened meanwhile: keep holding
		}
		return m, m.release()
	case settleMsg:
		m.settling = false
		return m, nil
	case printedMsg:
		m.outBusy = false
		return m, m.hold(m.flush())
	case spinMsg:
		if !m.running || msg.gen != m.runGen {
			return m, nil
		}
		m.spin = (m.spin + 1) % len(spinFrames)
		return m, m.spinTick()
	case tea.KeyboardEnhancementsMsg:
		m.kbdEnhanced = msg.SupportsKeyDisambiguation()
		return m, nil
	case tea.BackgroundColorMsg:
		m.darkBG = msg.IsDark()
		return m, nil
	case hintCheckMsg:
		if !m.kbdEnhanced && !m.hintShown {
			m.hintShown = true
			return m, m.hold(m.printlnMuted("hint: this terminal can't report shift+enter; use alt+enter or ctrl+j for a newline"))
		}
		return m, nil
	case tea.PasteMsg:
		if m.login != nil {
			switch m.login.step {
			case loginEnterKey:
				m.login.appendKey(msg.Content)
				return m, nil
			case loginBusy:
				// Falls through: the paste lands in the composer so the
				// redirect URL or code can be sent with enter.
			default:
				return m, nil
			}
		}
		if m.pager == nil {
			// Insert at the cursor like typed text; a chip's marker goes there too.
			m.ta.InsertString(m.input.Prepare(msg.Content))
			m.pullTextarea()
			m.lastTyped = m.now()
		}
		return m, nil
	case tea.KeyPressMsg:
		if m.login != nil {
			return m, m.loginKey(msg)
		}
		return m, m.handleKey(msg)
	case agentEventMsg:
		return m, m.hold(m.handleAgent(msg.e))
	case runDoneMsg:
		return m, m.hold(m.handleRunDone(msg))
	case approvalMsg:
		m.approval = &msg
		return m, nil
	case loginProgressMsg:
		if m.login == nil {
			return m, nil
		}
		return m, m.hold(m.printlnContent(msg.line))
	case loginDoneMsg:
		return m, m.hold(m.handleLoginDone(msg))
	case logoutDoneMsg:
		return m, m.hold(m.handleLogoutDone(msg))
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
	if m.login != nil {
		m.login.shutdown()
		m.login = nil
	}
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
			// Not released at once: the renderer leaves the alt screen on its
			// next flush, and a Println that lands before that is written to
			// the alt screen and lost (see releaseMsg).
			return tea.Batch(cmd, tea.Tick(pagerSettle, func(time.Time) tea.Msg { return releaseMsg{} }))
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
	if d := m.dropdown(); d != nil {
		if cmd, handled := m.dropdownKey(d, k); handled {
			return cmd
		}
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
	case "alt+t":
		if it, ok := m.items.LastOfKind("thinking"); ok {
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
				return m.printlnError("error: allow-always not saved: " + err.Error()), true
			}
			return m.printlnContent(fmt.Sprintf("always allowing %q (saved to %s)", q.Subject, path)), true
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
				return m.printlnError("error: allow-always not saved: " + err.Error()), true
			}
			return m.printlnContent(fmt.Sprintf("always allowing %s (saved to %s)", q.Subject, path)), true
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
		return m.printlnError("error: " + err.Error())
	}
	switch parsed.Kind {
	case KindText, KindPrompt:
		if m.running {
			if m.agent != nil {
				m.agent.Steer(parsed.Text)
			}
			return m.printlnMuted("↳ queued: " + firstLineOf(parsed.Text))
		}
		if m.compacting {
			return m.println("a /compact is still running — wait for it to finish")
		}
		if m.shellBusy {
			return m.refuseBusy()
		}
		return m.startRun(parsed.Text)
	case KindCommand:
		// A submitted command echoes into the scrollback as a user message
		// (same `›` band) before it runs, so the transcript reads as
		// everything the user sent — refusals and output follow it.
		return tea.Sequence(m.printlnUser("› "+text), m.runCommand(parsed))
	case KindShell:
		if m.running {
			// `!` output enters the transcript; appending it between a tool
			// batch's results would diverge from the wire ordering the
			// rebuild guarantees. `!!` (local-only) stays available.
			return m.printlnContent("finish or interrupt the run first (esc) — !! runs locally now")
		}
		if m.compacting {
			// A note appended while a compaction summarizes could be excluded
			// from the summary (and land before the cut boundary, vanishing
			// from the rebuilt context).
			return m.println("a /compact is still running — wait for it to finish")
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

// refuseRunning guards commands that mutate the running conversation: when
// one must not run now it returns the explanation to print and true. (The
// print command can be nil while earlier output is in flight, so the bool —
// not the command — says whether the request was refused.)
func (m *model) refuseRunning() (tea.Cmd, bool) {
	switch {
	case m.shellBusy:
		return m.refuseBusy(), true
	case m.compacting:
		return m.println("a /compact is still running — wait for it to finish"), true
	case m.running:
		return m.println("finish or interrupt the run first (esc)"), true
	}
	return nil, false
}

func (m *model) refuseBusy() tea.Cmd {
	return m.println("a ! command is still running — wait for it to finish")
}

func (m *model) runCommand(c Parsed) tea.Cmd {
	switch c.Name {
	case "login":
		return m.runLogin(c.Args)
	case "logout":
		return m.runLogout(c.Args)
	}
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
			return m.printlnContent(strings.Join(lines, "\n"))
		}
		if cmd, refused := m.refuseRunning(); refused {
			return cmd
		}
		if err := m.agent.SetModel(c.Args, ""); err != nil {
			return m.printlnError("error: " + err.Error())
		}
		m.refreshStatus()
		s := m.agent.Status()
		return m.printlnContent(fmt.Sprintf("switched to %s · effort %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
	case "effort":
		if c.Args == "" {
			return m.printlnContent(fmt.Sprintf("effort %s — supported: %s", AbbrevEffort(st.Effort), supportedEfforts(st.Model)))
		}
		if cmd, refused := m.refuseRunning(); refused {
			return cmd
		}
		want, err := llm.ParseEffort(c.Args)
		if err != nil {
			return m.printlnError("error: " + err.Error())
		}
		got, err := m.agent.SetEffort(want)
		if err != nil {
			return m.printlnError("error: " + err.Error())
		}
		m.refreshStatus()
		note := ""
		if got != want {
			note = fmt.Sprintf(" (clamped from %s)", want)
		}
		return m.printlnContent("effort " + AbbrevEffort(got) + note)
	case "hard":
		if cmd, refused := m.refuseRunning(); refused {
			return cmd
		}
		on, err := m.agent.ToggleHard()
		if err != nil {
			return m.printlnError("error: " + err.Error())
		}
		m.refreshStatus()
		s := m.agent.Status()
		if on {
			return m.printlnContent(fmt.Sprintf("hard mode on: %s · %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
		}
		return m.printlnContent(fmt.Sprintf("hard mode off: back to %s · %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
	case "yolo":
		if cmd, refused := m.refuseRunning(); refused {
			return cmd
		}
		m.agent.SetYolo(!m.agent.Yolo()) // YoloChanged prints + refreshes
		return nil
	case "clear":
		if cmd, refused := m.refuseRunning(); refused {
			return cmd
		}
		return m.restartSession()
	case "compact":
		if cmd, refused := m.refuseRunning(); refused {
			return cmd
		}
		m.compacting = true
		m.status.Transient = "compacting…"
		return m.compactCmd()
	case "cost":
		u := st.Usage
		return m.println(fmt.Sprintf("in %d · out %d · cache read %d · cache write %d · $%.4f", u.Input, u.Output, u.CacheRead, u.CacheWrite, st.Cost))
	case "undo":
		msg, err := m.agent.Undo()
		if err != nil {
			return m.printlnError("error: " + err.Error())
		}
		return m.printlnContent(msg)
	case "copy":
		if m.lastAssistant == "" {
			return m.printlnMuted("nothing to copy yet")
		}
		return tea.Sequence(tea.SetClipboard(m.lastAssistant), m.printlnMuted(fmt.Sprintf("copied %d chars (OSC 52)", len(m.lastAssistant))))
	case "show":
		n, err := strconv.Atoi(c.Args)
		if err != nil {
			return m.println("usage: /show <n>")
		}
		it, ok := m.items.Get(n)
		if !ok {
			return m.println(fmt.Sprintf("no item #%d yet", n))
		}
		m.openPager(it)
		return nil
	case "exit":
		return m.quit()
	case "help":
		return m.printlnContent(HelpText(m.opts.Prompts))
	}
	return m.println("unknown command /" + c.Name)
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
		return m.printlnMuted("compaction cancelled")
	case errors.Is(msg.err, agent.ErrNothingToCompact):
		return m.printlnMuted("nothing to compact")
	default:
		return m.printlnError("error: " + msg.err.Error())
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
		return m.printlnError("error: " + err.Error())
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
	return tea.Sequence(m.printlnMuted(fmt.Sprintf("new session %s (previous stays resumable)", a.Session().ID8())), m.branchCmd())
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
		return m.println(strings.Join(lines, "\n"))
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
	return m.println(strings.Join(lines, "\n"))
}

func (m *model) handleAgent(e agent.Event) tea.Cmd {
	switch e := e.(type) {
	case agent.TextDelta:
		// The thinking block that preceded this text is done: close it into
		// its item line FIRST, so the scrollback reads thinking → response.
		flush := m.flushThinking()
		m.live.WriteString(e.Text)
		var commit tea.Cmd
		if lines := m.commitLive(); len(lines) > 0 {
			commit = m.printlnResponse(strings.Join(lines, "\n"))
		}
		return tea.Sequence(flush, commit)
	case agent.ThinkingDelta:
		if m.thinking.Len() == 0 {
			m.thinkStart = m.now()
		}
		m.thinking.WriteString(e.Text)
		return nil
	case agent.StreamReset:
		m.live.Reset()
		m.thinking.Reset()
		return m.printlnMuted("[stream interrupted — retrying]")
	case agent.ToolStart:
		m.toolBusy = e.Call.Name
		// A tool call ends any open thinking block: print its line before
		// the tool's own item (which lands on ToolEnd).
		return m.flushThinking()
	case agent.ToolEnd:
		m.toolBusy = ""
		it := m.items.AddTool(e.Call, e.Result)
		m.refreshStatus()
		return m.printlnMuted(it.Line)
	case agent.TurnEnd:
		m.toolBusy = ""
		var cmds []tea.Cmd
		if rest := m.live.String(); rest != "" {
			cmds = append(cmds, m.printlnResponse(rest))
		}
		m.live.Reset()
		cmds = append(cmds, m.flushThinking())
		m.lastAssistant = llm.TextOf(e.Message)
		m.status.Transient = ""
		m.refreshStatus()
		cmds = append(cmds, m.branchCmd())
		return tea.Sequence(cmds...)
	case agent.Retry:
		m.status.Transient = fmt.Sprintf("retry %d/%d · %s", e.Notice.Attempt, e.Notice.Max, e.Notice.Wait.Round(1e8))
		return nil
	case agent.Warning:
		return m.printlnWarn("warning: " + e.Text)
	case agent.Compacted:
		m.status.Transient = ""
		m.refreshStatus()
		return m.printlnMuted(fmt.Sprintf("⋯ compacted: %d → %d tokens", e.TokensBefore, e.TokensAfter))
	case agent.Resumed:
		return m.printlnMuted(fmt.Sprintf("resumed %s (%d messages) — earlier output is in the session file", e.ID8, e.Messages))
	case agent.SteeringApplied:
		return m.printlnMuted("↳ sent: " + strings.Join(e.Texts, " · "))
	case agent.YoloChanged:
		m.refreshStatus()
		if e.On {
			return m.println(red.Render("yolo mode on: all permission checks are off"))
		}
		return m.println("yolo mode off: permission checks restored")
	}
	return nil
}

// flushThinking closes the open thinking block (if any) into a numbered item
// and prints its line — in place, before whatever follows: the response text,
// a tool item, the turn end. Waiting for TurnEnd (the old behaviour) stranded
// the line after the response it preceded.
func (m *model) flushThinking() tea.Cmd {
	if m.thinking.Len() == 0 {
		return nil
	}
	dur := time.Duration(0)
	if !m.thinkStart.IsZero() {
		dur = m.now().Sub(m.thinkStart)
	}
	it := m.items.AddThinking(m.thinking.String(), dur)
	m.thinking.Reset()
	m.thinkStart = time.Time{}
	return m.printlnMuted(it.Line + thinkingHint)
}

func (m *model) handleRunDone(msg runDoneMsg) tea.Cmd {
	m.running, m.cancel = false, nil
	m.toolBusy = ""
	var cmds []tea.Cmd
	// The completion is queued behind the run's events (eventPipe.send), so
	// every delta has been handled by now. Flush the trailing partial line
	// unconditionally: on an errored/interrupted run it is the visible tail of
	// what the model streamed (display-only; the transcript holds the
	// persisted text) — leaving it in the live region would freeze it there
	// and the next run would silently drop it.
	if m.live.Len() > 0 {
		cmds = append(cmds, m.printlnResponse(m.live.String()))
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
			cmds = append(cmds, m.printlnMuted("[interrupted]"))
		} else {
			cmds = append(cmds, m.printlnError("error: "+msg.err.Error()))
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
		sb.WriteString(m.liveResponse(m.live.String()))
		sb.WriteString("\n")
	}
	if m.login != nil {
		sb.WriteString(m.loginPanel() + "\n")
	} else if m.approval != nil {
		sb.WriteString(warnFg.Render("? ") + approvalPrompt(m.approval.q) + "\n")
	} else if m.running {
		sb.WriteString(m.activityLine() + "\n")
	} else if d := m.dropdown(); d != nil {
		sb.WriteString(m.dropdownPanel(d) + "\n")
	}
	sb.WriteString(m.rule())
	inputTop := strings.Count(sb.String(), "\n")
	sb.WriteString(m.ta.View())
	sb.WriteString("\n")
	sb.WriteString(m.rule())
	sb.WriteString(m.statusLine())
	rows := strings.Count(sb.String(), "\n") + 1
	cursorY := rows - 1
	cur := m.ta.Cursor()
	if cur != nil {
		cur.Position.Y += inputTop
		cursorY = cur.Position.Y
	}
	sb.WriteString(m.padForShrink(rows, cursorY))
	v := tea.NewView(sb.String())
	// The terminal's own cursor, parked in the input box: besides showing the
	// caret it pins the renderer's remembered cursor row inside the frame
	// after every render (see padForShrink).
	v.Cursor = cur
	// Ask for full key disambiguation so shift+enter is distinguishable.
	v.KeyboardEnhancements = tea.KeyboardEnhancements{ReportAllKeysAsEscapeCodes: true}
	return v
}

// padForShrink returns the blank rows to append below a frame of the given
// height whose cursor sits on cursorY, and records that cursor row.
//
// Bubble Tea's inline renderer parks its cursor on the View's cursor row and
// remembers it; when a later frame is shorter it clamps that remembered row
// to the new height before moving back to the frame's top, the move falls
// short by the clamp difference, and the old frame's top rows are stranded in
// the scrollback. So a frame must never be shorter than rememberedRow+1.
//
// The remembered row is not just the previous View's row: the renderer
// coalesces Views, so when several Views arrive within a frame interval only
// the burst's last one is drawn — and while it is drawn the remembered row is
// still the one of the last frame flushed before the burst (typing "lo" over
// a tall dropdown in one batch is exactly this). A gap of guardWindow proves
// the earlier frame was flushed, so the remembered row is bounded by the
// deeper of the previous burst's last View and the deepest row in the current
// burst (mid-burst flushes can only remember rows of the burst). The
// difference is blank rows below the status bar, dropped by the settleMsg
// that follows once the remembered row is safe again.
func (m *model) padForShrink(rows, cursorY int) string {
	now := m.now()
	g := &m.guard
	if now.Sub(g.lastAt) >= guardWindow {
		// The gap flushed the previous View's frame: it is what the renderer
		// remembers now, and this View starts a new burst.
		g.prevRow = g.lastRow
		g.burstRow = cursorY
	} else {
		g.burstRow = max(g.burstRow, cursorY)
	}
	g.lastRow, g.lastAt = cursorY, now
	deepest := max(g.prevRow, g.burstRow)
	pad := deepest + 1 - rows
	if m.height > 0 {
		pad = min(pad, m.height-rows) // never taller than the screen
	}
	if pad <= 0 {
		return ""
	}
	if !m.settling && m.pipe != nil {
		m.settling = true
		pipe := m.pipe
		time.AfterFunc(2*guardWindow, func() { pipe.send(settleMsg{}) })
	}
	return strings.Repeat("\n", pad)
}

// activityLine is the live progress row of a run: spinner, what the agent is
// doing (tool name, thinking, or just working), elapsed time and the
// interrupt key.
func (m *model) activityLine() string {
	what := "working"
	switch {
	case m.toolBusy != "":
		what = Sanitize(m.toolBusy)
	case m.thinking.Len() > 0 && m.live.Len() == 0:
		what = "thinking"
	}
	return spinFrames[m.spin%len(spinFrames)] + " " + dim.Render(fmt.Sprintf("%s… %s · esc to interrupt", what, fmtElapsed(m.now().Sub(m.runStart))))
}

// fmtElapsed renders a run duration: 7s · 1m 05s · 1h 02m.
func fmtElapsed(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %02ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %02dm", s/3600, s%3600/60)
}

// fmtThinkDuration renders a thinking block's duration for its item line:
// sub-second blocks read "<1s", and a zero duration (the block was not
// accumulated from deltas) is omitted entirely.
func fmtThinkDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d < time.Second:
		return "<1s"
	}
	return fmtElapsed(d)
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
	m.pipe = pipe
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

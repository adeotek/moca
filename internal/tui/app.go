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
	"github.com/charmbracelet/colorprofile"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/compact"
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
	// PromptDirs re-reads Prompts when a run or `!` command finishes: a
	// command written during the run (the create-command workflow) must be
	// usable without a restart. Empty = Prompts stays as given.
	PromptDirs []skills.Dir
	Home       string
	// HistoryPath is the persistent prompt history ("" = in-memory only).
	HistoryPath string
	// HintsPath is the file remembering which one-time hints were shown
	// ("" = not persisted).
	HintsPath string
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
	// toolArg is the running tool's operand (command, path, pattern) and
	// runChars the characters streamed this run (the activity row's token
	// estimate).
	toolArg  string
	runChars int
	cancel   context.CancelFunc
	approval *approvalMsg
	// login is the active /login//logout interaction (modal, like approval).
	login *loginState
	// pick is the active modal list picker (/model, /resume).
	pick *pickState
	// files is the workdir's file index for @mentions (nil = not loaded or
	// stale; filesLoading says a walk is in flight).
	files        []string
	filesLoading bool
	// histSearch: ctrl+r is searching the prompt history (the draft is the
	// query; see dropdown.go).
	histSearch bool
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
	// shellText is that command (for the activity row) and shellCancel
	// kills it (esc, quit).
	shellBusy   bool
	shellText   string
	shellCancel context.CancelFunc
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
	// md is the markdown state (inside a code fence) of the response text
	// being printed; it resets with each run, turn and stream restart.
	md mdState
	// profile is the terminal's color support (tea.ColorProfileMsg; NO_COLOR
	// gives ASCII). Scrollback text bypasses the renderer's downsampling, so
	// flush applies it (see downsample).
	profile colorprofile.Profile
	// blurred: the terminal reports it lost focus (tea.BlurMsg) — notifications
	// fire only then.
	blurred bool
	// lastKind is what the scrollback printed last (see printlnResponse).
	lastKind int
	// guard bounds the cursor row the inline renderer may still remember and
	// settling says a settleMsg is already scheduled (padForShrink).
	guard    padGuard
	settling bool
	// runStart, runGen and spin drive the activity line: when the run began,
	// which run's ticks are live, and the spinner frame.
	runStart time.Time
	runGen   int
	spin     int
	// sessionUsed: the current session holds something worth resuming (a
	// run, a `!` note, or it was resumed); the exit line then names it.
	sessionUsed bool
	// quitting: the program is exiting; the last frame drops the input box
	// so the exit line lands below the status bar, not over a stale draft.
	quitting bool
}

// quitHintMsg re-renders once the ctrl+c quit window has passed, dropping
// the "press ctrl+c again" hint.
type quitHintMsg struct{}

// notifyAfter is how long a run must take before finishing it notifies an
// unfocused terminal.
const notifyAfter = 30 * time.Second

// notify asks the terminal to alert the user — only when it has reported
// losing focus (a focused user is already looking). OSC 9 is the desktop
// notification escape most modern terminals understand; the bell is the
// fallback for the rest.
func (m *model) notify(text string) tea.Cmd {
	if !m.blurred {
		return nil
	}
	switch m.start.Config.TUI.Notify {
	case "off":
		return nil
	case "bell":
		return tea.Raw("\a")
	}
	return tea.Raw("\x1b]9;" + strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1 // no control bytes inside the escape
		}
		return r
	}, text) + "\a")
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
	m := &model{opts: o, start: o.Start, agent: a, input: NewInput(), ta: ta, now: time.Now, darkBG: true, profile: colorprofile.TrueColor}
	m.input.LoadHistory(loadHistory(o.HistoryPath, o.Start.Workdir))
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
	body := m.downsample(strings.Join(m.outq, "\n"))
	m.outq, m.outBusy = nil, true
	return tea.Sequence(tea.Println(body), func() tea.Msg { return printedMsg{} })
}

// downsample converts scrollback text to what the terminal can show:
// 256/16-color approximations of the truecolor bands, no colors at all under
// NO_COLOR (bold and faint stay), plain text for a non-terminal. tea.Println
// writes its string as is — the renderer only downsamples what it draws
// itself.
func (m *model) downsample(s string) string {
	if m.profile == colorprofile.TrueColor || m.profile == colorprofile.Unknown {
		return s
	}
	var sb strings.Builder
	w := &colorprofile.Writer{Forward: &sb, Profile: m.profile}
	w.WriteString(s)
	return sb.String()
}

// println prints a trusted or already-styled line into the scrollback.
func (m *model) println(s string) tea.Cmd {
	m.lastKind = kindNotice
	return m.emit(s)
}

// printlnContent is println for untrusted content (model output, tool output,
// user input, error strings): control bytes are neutralized so the terminal
// never interprets them. Styled lines must use println — running lipgloss
// output through Sanitize would display the escape codes as text.
func (m *model) printlnContent(s string) tea.Cmd {
	m.lastKind = kindNotice
	return m.emit(Sanitize(s))
}

func (m *model) refreshStatus() {
	m.status.Version = config.Version
	if m.agent == nil {
		return
	}
	st := m.agent.Status()
	m.status.Cwd = AbbrevHome(m.opts.Start.Workdir, m.opts.Home)
	m.status.Model, m.status.Effort = st.Model.Qualified(), st.Effort
	m.status.Window, m.status.Used = st.Window, st.ContextTokens
	cc := m.start.Config.Context
	m.status.Trigger = compact.NewBudget(st.Window, cc.ReserveTokens, cc.KeepRecentTokens).Trigger()
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
	m.md = mdState{}
	m.thinking.Reset()
	m.toolBusy, m.toolArg, m.runChars = "", "", 0
	m.sessionUsed = true
	spin := m.beginActivity()
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
	}), spin)
}

// busy reports whether the activity row is up: a run, a /compact or a `!`
// command is in flight.
func (m *model) busy() bool { return m.running || m.compacting || m.shellBusy }

// beginActivity restarts the activity row's clock and spinner for a new
// run, compaction or `!` command and returns its first tick.
func (m *model) beginActivity() tea.Cmd {
	m.runStart, m.spin = m.now(), 0
	m.runGen++
	return m.spinTick()
}

// spinTick schedules the next spinner frame of the current activity.
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
	case quitHintMsg:
		return m, nil // the re-render drops an expired hint
	case printedMsg:
		m.outBusy = false
		return m, m.hold(m.flush())
	case spinMsg:
		if !m.busy() || msg.gen != m.runGen {
			return m, nil
		}
		m.spin = (m.spin + 1) % len(spinFrames)
		return m, m.spinTick()
	case tea.ColorProfileMsg:
		m.profile = msg.Profile
		return m, nil
	case tea.FocusMsg:
		m.blurred = false
		return m, nil
	case tea.BlurMsg:
		m.blurred = true
		return m, nil
	case tea.KeyboardEnhancementsMsg:
		m.kbdEnhanced = msg.SupportsKeyDisambiguation()
		return m, nil
	case tea.BackgroundColorMsg:
		m.darkBG = msg.IsDark()
		return m, nil
	case hintCheckMsg:
		if m.kbdEnhanced {
			return m, nil
		}
		// The placeholder always names the fallback; the scrollback hint is
		// shown once per machine, not on every launch.
		m.ta.Placeholder = "Message moca…  (ctrl+j newline · /help)"
		if m.hintShown || hintSeen(m.opts.HintsPath, "shiftEnter") {
			return m, nil
		}
		m.hintShown = true
		markHint(m.opts.HintsPath, "shiftEnter")
		return m, m.hold(m.printlnMuted("hint: this terminal can't report shift+enter; use alt+enter or ctrl+j for a newline (shown once)"))
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
		if m.pager == nil && m.pick == nil {
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
		if m.pick != nil {
			return m, m.pickKey(msg)
		}
		return m, m.handleKey(msg)
	case agentEventMsg:
		return m, m.hold(m.handleAgent(msg.e))
	case runDoneMsg:
		took := m.now().Sub(m.runStart)
		cmd := m.hold(m.handleRunDone(msg))
		if took >= notifyAfter {
			cmd = tea.Batch(cmd, m.notify("moca: run finished"))
		}
		return m, cmd
	case approvalMsg:
		m.approval = &msg
		return m, m.notify("moca: approval needed — allow `" + Sanitize(msg.q.Subject) + "`?")
	case loginProgressMsg:
		if m.login == nil {
			return m, nil
		}
		return m, m.hold(m.printlnContent(msg.line))
	case loginDoneMsg:
		return m, m.hold(m.handleLoginDone(msg))
	case logoutDoneMsg:
		return m, m.hold(m.handleLogoutDone(msg))
	case modelsRefreshedMsg:
		m.status.Transient = ""
		if m.agent == nil || m.busy() || m.pick != nil || m.login != nil {
			return m, nil
		}
		var warn tea.Cmd
		if msg.err != nil {
			// The listing still opens: cloud models and declared ones work.
			warn = m.printlnWarn("warning: " + msg.err.Error())
		}
		m.openModelPicker(m.agent.Status().Model.Qualified())
		return m, m.hold(warn)
	case filesLoadedMsg:
		m.files, m.filesLoading = msg.files, false
		if m.files == nil {
			m.files = []string{} // loaded, but empty: do not walk again every key
		}
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
	m.quitting = true
	if m.login != nil {
		m.login.shutdown()
		m.login = nil
	}
	if m.compactCancel != nil {
		m.compactCancel()
	}
	if m.shellCancel != nil {
		m.shellCancel()
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
	return tea.Batch(cmd, m.maybeLoadFiles())
}

func (m *model) handleKey(k tea.KeyPressMsg) tea.Cmd {
	if m.pager != nil {
		if k.String() == "ctrl+c" {
			return m.ctrlC()
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
		if m.shellBusy && m.shellCancel != nil {
			m.shellCancel()
			return nil
		}
		if m.running && m.cancel != nil {
			m.cancel()
		}
		return nil
	case "ctrl+c":
		return m.ctrlC()
	case "ctrl+o":
		if m.approval != nil {
			// The pending question's whole command (the panel shows a few lines).
			m.openPager(Item{Kind: "approval", Line: "? `" + Sanitize(m.approval.q.Subject) + "` awaits approval — the whole command", Body: Sanitize(m.approval.q.Detail)})
			return nil
		}
		if it, ok := m.items.Last(); ok {
			m.openPager(it)
		}
		return nil
	case "alt+t":
		if it, ok := m.items.LastOfKind("thinking"); ok {
			m.openPager(it)
		}
		return nil
	case "shift+tab":
		return m.cycleEffort()
	case "ctrl+r":
		if m.login != nil || m.pager != nil || m.busy() {
			return nil
		}
		m.histSearch = !m.histSearch
		m.drop = nil
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

// ctrlC clears a non-empty draft, or arms quit on an empty one (a second
// press within a second quits); the armed state shows a hint until it
// expires.
func (m *model) ctrlC() tea.Cmd {
	m.histSearch = false
	if m.input.CtrlC(m.now()) {
		return m.quit()
	}
	m.syncTextarea()
	if m.input.QuitArmed(m.now()) {
		return tea.Tick(time.Second+50*time.Millisecond, func(time.Time) tea.Msg { return quitHintMsg{} })
	}
	return nil
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
	m.histSearch = false
	appendHistory(m.opts.HistoryPath, m.opts.Start.Workdir, text)
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
	case KindShell, KindShellLocal:
		// Shell input echoes like any other input; the command itself is
		// not repeated above its output.
		return tea.Sequence(m.printlnUser(userMarker+text), m.submitShell(parsed))
	}
	return nil
}

// submitShell starts (or refuses) a `!` / `!!` command.
func (m *model) submitShell(parsed Parsed) tea.Cmd {
	switch parsed.Kind {
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
		m.shellBusy, m.shellText = true, parsed.Text
		return tea.Batch(m.shellCmd(false, parsed.Text), m.beginActivity())
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
	case "resume":
		return m.runResume(c.Args)
	case "sessions":
		return m.runSessions()
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
		if cmd, refused := m.refuseRunning(); refused {
			if c.Args == "" {
				// Listing is harmless mid-run; only the switch is refused.
				return m.printlnContent(m.modelList(st.Model.Qualified()))
			}
			return cmd
		}
		if c.Args == "" {
			if _, local := m.start.Config.Providers["ollama"]; local {
				// Models pulled since startup should show up: ask the
				// server first (a few seconds at most), then open the picker.
				m.status.Transient = "reading models…"
				a := m.agent
				return func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
					defer cancel()
					return modelsRefreshedMsg{err: a.RefreshModels(ctx)}
				}
			}
			m.openModelPicker(st.Model.Qualified())
			return nil
		}
		return m.switchModel(c.Args)
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
		return tea.Batch(m.compactCmd(), m.beginActivity())
	case "cost":
		u := st.Usage
		cost := fmt.Sprintf("$%.4f", st.Cost)
		if st.Sub {
			cost = "subscription"
		}
		return m.println(fmt.Sprintf("in %s (fresh %s · cache read %s · cache write %s) · out %s · %s",
			FmtTokens(u.Input+u.CacheRead+u.CacheWrite), FmtTokens(u.Input), FmtTokens(u.CacheRead), FmtTokens(u.CacheWrite), FmtTokens(u.Output), cost))
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

// cycleEffort is shift+tab: the next effort level the current model
// supports, wrapping around — the quick form of /effort for the one knob most
// worth tuning per task. Refused where /effort is.
func (m *model) cycleEffort() tea.Cmd {
	if m.agent == nil {
		return nil
	}
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	st := m.agent.Status()
	var levels []llm.Effort
	for _, e := range llm.Efforts {
		if _, ok := st.Model.ThinkingLevelMap[e]; ok {
			levels = append(levels, e)
		}
	}
	if len(levels) < 2 {
		return m.printlnMuted("this model has no effort levels to cycle")
	}
	next := levels[(slices.Index(levels, st.Effort)+1)%len(levels)] // not found (-1) → the first
	got, err := m.agent.SetEffort(next)
	if err != nil {
		return m.printlnError("error: " + err.Error())
	}
	m.refreshStatus()
	return m.printlnMuted("effort " + AbbrevEffort(got))
}

// modelList is the plain listing of /model during a run (the picker is
// modal and a switch is refused then anyway).
func (m *model) modelList(cur string) string {
	var lines []string
	for _, mo := range m.agent.Models() {
		mark := "  "
		if mo.Qualified() == cur {
			mark = "* "
		}
		lines = append(lines, mark+mo.Qualified())
	}
	return strings.Join(lines, "\n")
}

// openModelPicker lists every model, the current one preselected; models of
// a provider with no credential are marked (picking one reports the missing
// key rather than switching).
func (m *model) openModelPicker(cur string) {
	p := &pickState{title: "switch to which model?  (prompt cache is forfeited)", cancelNote: "model unchanged", onChoose: m.switchModel}
	creds := map[string]bool{}
	for i, mo := range m.agent.Models() {
		q := mo.Qualified()
		ok, seen := creds[mo.Provider]
		if !seen {
			ok = m.agent.HasCredential(q)
			creds[mo.Provider] = ok
		}
		label := q
		switch {
		case q == cur:
			label += "  — current"
			p.cursor = i
		case !ok:
			label += "  — no key (/login " + mo.Provider + ")"
		}
		p.items = append(p.items, pickItem{key: q, label: label})
	}
	m.openPicker(p)
}

// switchModel is /model <id>.
func (m *model) switchModel(q string) tea.Cmd {
	if err := m.agent.SetModel(q, ""); err != nil {
		return m.printlnError("error: " + err.Error())
	}
	m.refreshStatus()
	s := m.agent.Status()
	return m.printlnContent(fmt.Sprintf("switched to %s · effort %s (prompt cache forfeited)", s.Model.Qualified(), AbbrevEffort(s.Effort)))
}

// reloadPrompts re-reads the prompt-template dirs when a run or `!` command
// finishes: the agent can write a new command during a run (the
// create-command workflow), and it must be usable without a restart. An
// empty PromptDirs (unit tests, callers that pass literal prompts) is a
// no-op.
func (m *model) reloadPrompts() {
	if len(m.opts.PromptDirs) == 0 {
		return
	}
	m.opts.Prompts = skills.LoadPrompts(m.opts.PromptDirs)
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
	m.sessionUsed = false
	m.items = Items{}
	m.live.Reset()
	m.md = mdState{}
	m.thinking.Reset()
	m.status.Transient = ""
	m.refreshStatus()
	// The bar's branch survives /clear; re-resolve it for the new session.
	return tea.Sequence(m.printlnMuted(fmt.Sprintf("new session %s (previous stays resumable)", a.Session().ID8())), m.branchCmd())
}

// shellCmd runs a `!`/`!!` command off the event loop; a `!` one can be
// killed with esc (shellCancel).
func (m *model) shellCmd(local bool, cmd string) tea.Cmd {
	wd := m.opts.Start.Workdir
	env := tools.ShellEnv(os.Environ(), config.EnvRefs(m.opts.Start.Config))
	ctx, cancel := context.WithCancel(context.Background())
	if !local {
		m.shellCancel = cancel
	}
	return func() tea.Msg {
		defer cancel()
		out, err := tools.RunShell(ctx, wd, env, cmd, 30*time.Second)
		return shellDoneMsg{local: local, cmd: cmd, out: out, err: err}
	}
}

func (m *model) handleShellDone(msg shellDoneMsg) tea.Cmd {
	// A `!` command can edit prompt templates: pick the change up.
	m.reloadPrompts()
	if !msg.local {
		m.shellBusy, m.shellText, m.shellCancel = false, "", nil
	}
	var lines []string
	// esc (or quit) killed it: show what it printed, but do not hand an
	// abandoned command to the model.
	cancelled := errors.Is(msg.err, context.Canceled)
	if msg.err != nil && !cancelled {
		lines = append(lines, "error: "+Sanitize(msg.err.Error()))
		return m.println(strings.Join(lines, "\n"))
	}
	body := tools.Truncate(msg.out.Output, 30_000)
	if b := strings.TrimRight(Sanitize(body), "\n"); b != "" {
		lines = append(lines, b)
	}
	exit := fmt.Sprintf("[exit %d]", msg.out.ExitCode)
	switch {
	case msg.out.TimedOut:
		exit = "[timed out after 30s — process group killed]"
	case cancelled:
		exit = "[interrupted — process group killed]"
	}
	lines = append(lines, exit)
	if !msg.local && !cancelled && m.agent != nil {
		note := "$ " + msg.cmd + "\n" + body + "\n" + exit
		m.sessionUsed = true
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
		m.runChars += len(e.Text)
		m.live.WriteString(e.Text)
		var commit tea.Cmd
		if lines := m.commitLive(); len(lines) > 0 {
			commit = m.printlnResponse(strings.Join(lines, "\n"))
		}
		return tea.Sequence(flush, commit)
	case agent.ThinkingDelta:
		m.runChars += len(e.Text)
		if m.thinking.Len() == 0 {
			m.thinkStart = m.now()
		}
		m.thinking.WriteString(e.Text)
		return nil
	case agent.StreamReset:
		m.live.Reset()
		m.md = mdState{}
		m.thinking.Reset()
		return m.printlnMuted("[stream interrupted — retrying]")
	case agent.ToolStart:
		m.toolBusy, m.toolArg = e.Call.Name, toolArgSummary(e.Call)
		// A tool call ends any open thinking block: print its line before
		// the tool's own item (which lands on ToolEnd).
		return m.flushThinking()
	case agent.ToolEnd:
		m.toolBusy, m.toolArg = "", ""
		it := m.items.AddTool(e.Call, e.Result)
		m.refreshStatus()
		item := m.printItem(it.Line, e.Result.IsError)
		if e.Call.Name == "edit" && !e.Result.IsError {
			if prev := diffPreview(e.Result.Detail, m.width, diffPreviewRows); prev != "" {
				return tea.Sequence(item, m.emit(prev))
			}
		}
		return item
	case agent.TurnEnd:
		m.toolBusy = ""
		var cmds []tea.Cmd
		if rest := m.live.String(); rest != "" {
			cmds = append(cmds, m.printlnResponse(rest))
		}
		m.live.Reset()
		m.md = mdState{}
		cmds = append(cmds, m.flushThinking())
		if t := llm.TextOf(e.Message); t != "" {
			// A text-less final turn keeps the previous answer for /copy.
			m.lastAssistant = t
		}
		m.status.Transient = ""
		m.refreshStatus()
		cmds = append(cmds, m.branchCmd())
		m.files = nil // the turn may have created files; re-index on the next @
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
		m.sessionUsed = true
		head := m.printlnMuted(fmt.Sprintf("resumed %s (%d messages)", e.ID8, e.Messages))
		if m.agent == nil {
			return head
		}
		return tea.Sequence(head, m.replay(m.agent.History(), replayTurns))
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
	return m.printItem(it.Line+thinkingHint, false)
}

func (m *model) handleRunDone(msg runDoneMsg) tea.Cmd {
	m.running, m.cancel = false, nil
	m.toolBusy = ""
	// The run may have written a slash command (the create-command
	// workflow): it must be usable now, without a restart.
	m.reloadPrompts()
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
	m.md = mdState{}
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
	if m.quitting {
		// The last frame keeps the status bar (the session's final numbers)
		// and drops the input box and hints; the trailing blank row is where
		// the exit line prints. Padding keeps it from shrinking under the
		// remembered cursor row (padForShrink).
		body := m.rule() + m.statusLine() + "\n"
		rows := strings.Count(body, "\n") + 1
		return tea.NewView(body + m.padForShrink(rows, rows-1))
	}
	if m.pager != nil {
		v := tea.NewView(m.pager.view(m.approval != nil))
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
	} else if m.pick != nil {
		sb.WriteString(m.pickerPanel() + "\n")
	} else if m.approval != nil {
		rows := approvalDetailRows
		if m.height > 0 {
			rows = min(rows, max(1, m.height-inputChrome-4))
		}
		sb.WriteString(approvalPanel(m.approval.q, max(20, m.width), rows) + "\n")
	} else if m.busy() {
		sb.WriteString(m.activityLine() + "\n")
	} else if d := m.dropdown(); d != nil {
		sb.WriteString(m.dropdownPanel(d) + "\n")
	}
	if m.input.QuitArmed(m.now()) {
		sb.WriteString(dim.Render("press ctrl+c again to quit") + "\n")
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
	// Focus reports tell us when to notify (see notify).
	v.ReportFocus = true
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
	what, hint := "working", "esc to interrupt"
	switch {
	case m.compacting:
		what, hint = "compacting", "esc to cancel"
	case m.shellBusy && !m.running:
		what = "$ " + clampRunes(Sanitize(firstLineOf(m.shellText)), max(10, m.width-40))
	case m.toolBusy != "":
		what = Sanitize(m.toolBusy)
		if m.toolArg != "" {
			what += " " + clampRunes(m.toolArg, max(10, m.width-50))
		}
	case m.thinking.Len() > 0 && m.live.Len() == 0:
		what = "thinking"
	}
	parts := []string{fmtElapsed(m.now().Sub(m.runStart))}
	if m.running {
		if n := m.runChars / 4; n > 0 {
			parts = append(parts, "↓"+FmtTokens(n)+" tok")
		}
		if m.agent != nil {
			if q := m.agent.PendingSteering(); q > 0 {
				parts = append(parts, fmt.Sprintf("%d queued", q))
			}
		}
	}
	parts = append(parts, hint)
	return spinFrames[m.spin%len(spinFrames)] + " " + dim.Render(what+"… "+strings.Join(parts, " · "))
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
		return red.Render("YOLO") + dim.Render(" · "+Sanitize(l1)) + "\n" + m.styleL2(l2)
	}
	l1, l2 := RenderStatus(m.status, w)
	return dim.Render(Sanitize(l1)) + "\n" + m.styleL2(l2)
}

// styleL2 dims status line 2 and colors the percent field by context
// pressure (yellow from 70%, red from the auto-compaction trigger, where the
// line also says /compact).
func (m *model) styleL2(l2 string) string {
	l2 = Sanitize(l2)
	pct := FmtPercent(m.status.Used, m.status.Window)
	var st lipgloss.Style
	switch m.status.pressure() {
	case pressureWarn:
		st = warnFg
	case pressureHot:
		st = errFg.Bold(true)
	default:
		return dim.Render(l2)
	}
	needle := " · " + pct
	i := strings.Index(l2, needle)
	if i < 0 {
		return dim.Render(l2) // truncated away
	}
	i += len(" · ")
	return dim.Render(l2[:i]) + st.Render(pct) + dim.Render(l2[i+len(pct):])
}

// rule is the full-width dim separator between the scrollback output, the
// input area and the status bar.
func (m *model) rule() string {
	return dim.Render(strings.Repeat("─", max(20, m.width))) + "\n"
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

// approvalPanel renders the question and the command it is about. The whole
// command is shown up to rows lines — the subject alone can hide what runs
// (an approved `python3` executes the heredoc under it) — each clamped to
// the width; the rest is counted and opens in the pager.
func approvalPanel(q tools.Question, width, rows int) string {
	var b strings.Builder
	b.WriteString(warnFg.Render("? ") + approvalPrompt(q))
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
	// Known before the first print (the welcome lines go out in Init, ahead
	// of the program's own ColorProfileMsg): the same detection Bubble Tea
	// makes, so NO_COLOR and 256-color terminals hold from the first line.
	m.profile = colorprofile.Detect(os.Stdout, os.Environ())
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
	if line := m.exitLine(); line != "" {
		fmt.Println(m.downsample(line)) // past the program: no renderer downsamples it
	}
	m.agent.Close()
	return runResult(ctx, err)
}

// exitLine is printed below the final frame on exit: how to come back to a
// session that holds something (empty when there is nothing to resume).
func (m *model) exitLine() string {
	if m.agent == nil || !m.sessionUsed {
		return ""
	}
	id := m.agent.Session().ID8()
	return mutedFg.Render(fmt.Sprintf("session %s · resume with: moca --resume %s", id, id))
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

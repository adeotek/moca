package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/mcp"
	"github.com/adeotek/moca/internal/tools"
)

func openPager(m *model) {
	m.openPager(Item{N: 1, Kind: "tool", Line: "▸ #1 x", Body: "body"})
}

// Println is dropped while the alt-screen pager is open (Bubble Tea writes it
// into the alt buffer), so output must be held and released on close.
func TestPagerDefersScrollbackOutput(t *testing.T) {
	m := newTestModel()
	m.running = true
	openPager(m)
	_, cmd := m.Update(agentEventMsg{agent.TextDelta{Text: "streamed\nline"}})
	if cmd != nil {
		t.Fatal("print cmds must not run while the pager is open")
	}
	_, cmd = m.Update(runDoneMsg{err: context.Canceled})
	if cmd != nil {
		t.Fatal("run-done output must be held too")
	}
	_, cmd = m.Update(key("q"))
	if m.pager != nil || cmd == nil {
		t.Fatal("closing the pager releases the held output")
	}
}

func TestApprovalKeysIgnoredWhileTyping(t *testing.T) {
	m := newAgentModel(t)
	fixed := time.Now()
	m.now = func() time.Time { return fixed } // keystrokes land "instantly"
	for _, r := range "Please " {
		m.Update(keyMsg(string(r)))
	}
	reply := make(chan tools.Answer, 4)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", Detail: "python3 -c 1", CanAlways: true}, reply: reply})
	for _, r := range "Add tests" {
		m.Update(keyMsg(string(r)))
	}
	select {
	case a := <-reply:
		t.Fatalf("typed letters answered the prompt: %v", a)
	default:
	}
	if m.ta.Value() != "Please Add tests" {
		t.Fatalf("draft %q", m.ta.Value())
	}
	if b, _ := os.ReadFile(m.opts.ConfigPath); strings.Contains(string(b), "python3") {
		t.Fatal("allow-always must not be persisted by stray keystrokes")
	}
}

func TestApprovalKeysAnswerAfterPause(t *testing.T) {
	m := newAgentModel(t)
	fixed := time.Now()
	m.now = func() time.Time { return fixed }
	m.lastTyped = fixed.Add(-5 * time.Second)
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	m.Update(key("a"))
	if got := <-reply; got != tools.AllowOnce {
		t.Fatal(got)
	}
}

// Allow-always is persistent: prose must never trigger it. A stray capital
// `A` (even after a typing pause) goes to the draft; only ctrl+a persists.
func TestAllowAlwaysRequiresCtrl(t *testing.T) {
	m := newAgentModel(t)
	fixed := time.Now()
	m.now = func() time.Time { return fixed }
	m.lastTyped = fixed.Add(-time.Second) // a pause: `a` would answer now
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	m.Update(keyMsg("A"))
	select {
	case a := <-reply:
		t.Fatalf("a plain capital A answered the prompt: %v", a)
	default:
	}
	if !strings.Contains(m.input.Buffer(), "A") {
		t.Fatalf("the A belongs to the draft: %q", m.input.Buffer())
	}
	if _, err := os.Stat(config.ProjectConfigFile(m.start.Workdir)); err == nil {
		t.Fatal("a typed A must not persist allow-always")
	}
	_, cmd := m.Update(key("ctrl+a"))
	if got := <-reply; got != tools.AllowAlways {
		t.Fatalf("ctrl+a is the allow-always key: %v", got)
	}
	if !strings.Contains(printed(cmd), "always allowing") {
		t.Fatalf("confirmation line: %q", printed(cmd))
	}
	if b, _ := os.ReadFile(config.ProjectConfigFile(m.start.Workdir)); !strings.Contains(string(b), `"python3"`) {
		t.Fatalf("ctrl+a must persist: %s", b)
	}
}

// enter and ctrl+c must keep working while a prompt is shown.
func TestEnterAndCtrlCDuringApproval(t *testing.T) {
	m := newAgentModel(t)
	m.running = true
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "x", CanAlways: true}, reply: reply})
	m.input.SetBuffer("steer me")
	m.syncTextarea()
	m.Update(key("enter"))
	if strings.Contains(m.ta.Value(), "\n") || m.input.Buffer() != "" {
		t.Fatalf("enter must submit the draft as steering (ta %q, buf %q)", m.ta.Value(), m.input.Buffer())
	}
	if got := m.agent.TakeSteering(); len(got) != 1 || got[0] != "steer me" {
		t.Fatalf("steering %v", got)
	}
	m.Update(key("ctrl+c"))
	if _, cmd := m.Update(key("ctrl+c")); cmd == nil {
		t.Fatal("double ctrl+c quits even with a prompt shown")
	}
}

// A `!` still running must not let a run (or /clear) start: its note would be
// appended in the middle of a tool batch or into the wrong session.
func TestShellBusyBlocksRunAndClear(t *testing.T) {
	m := newAgentModel(t)
	m.input.SetBuffer("!true")
	_, cmd := m.Update(key("enter"))
	if cmd == nil || !m.shellBusy {
		t.Fatal("`!` marks the shell busy")
	}
	old := m.agent
	m.input.SetBuffer("hello")
	m.syncTextarea()
	m.Update(key("enter"))
	if m.running {
		t.Fatal("a run must not start while `!` is in flight")
	}
	m.input.SetBuffer("/clear")
	m.syncTextarea()
	m.Update(key("enter"))
	if m.agent != old {
		t.Fatal("/clear must wait for the `!` to finish")
	}
	m.Update(shellDoneMsg{local: false, cmd: "true"})
	if m.shellBusy {
		t.Fatal("busy flag cleared when the shell finishes")
	}
}

func TestShellRefusedWhileRunning(t *testing.T) {
	m := newAgentModel(t)
	m.running = true
	m.input.SetBuffer("!ls")
	m.syncTextarea()
	_, cmd := m.Update(key("enter"))
	if out, _ := simulate(m, cmd); m.shellBusy || !strings.Contains(out, "!!") || !strings.Contains(out, "› !ls") {
		t.Fatalf("`!` echoes, then is refused mid-run: %q", out)
	}
}

func TestRunResultMapping(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	killed := errors.Join(tea.ErrProgramKilled, ctx.Err())
	if err := runResult(ctx, killed); !errors.Is(err, context.Canceled) {
		t.Fatalf("SIGTERM/SIGINT must surface as cancellation (exit 130), got %v", err)
	}
	live := context.Background()
	if err := runResult(live, errors.Join(tea.ErrProgramKilled, tea.ErrProgramPanic)); err == nil {
		t.Fatal("a recovered panic must be an error, not a clean exit")
	}
	if err := runResult(live, nil); err != nil {
		t.Fatal(err)
	}
}

func TestPasteInsertsAtCursor(t *testing.T) {
	m := newTestModel()
	for _, r := range "hello world" {
		m.Update(keyMsg(string(r)))
	}
	for i := 0; i < 6; i++ {
		m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	}
	m.Update(tea.PasteMsg{Content: "XY"})
	if got := m.input.Buffer(); got != "helloXY world" {
		t.Fatalf("paste at cursor: %q", got)
	}
	// A chip lands at the cursor too.
	m2 := newTestModel()
	m2.Update(keyMsg("a"))
	m2.Update(keyMsg("b"))
	m2.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	m2.Update(tea.PasteMsg{Content: strings.Repeat("x\n", 60)})
	if got := m2.input.Buffer(); got != "a[paste 60 lines #1]b" {
		t.Fatalf("chip at cursor: %q", got)
	}
	if got := m2.input.Text(); !strings.HasPrefix(got, "ax\n") || !strings.HasSuffix(got, "x\nb") {
		t.Fatalf("sent text %q", got)
	}
}

func TestQuitCancelsRun(t *testing.T) {
	for _, inPager := range []bool{false, true} {
		m := newTestModel()
		cancelled := false
		m.running, m.cancel = true, func() { cancelled = true }
		if inPager {
			openPager(m)
		}
		m.Update(key("ctrl+c"))
		_, cmd := m.Update(key("ctrl+c"))
		if cmd == nil || !cancelled {
			t.Fatalf("pager=%v: quitting must cancel the run (cancelled=%v)", inPager, cancelled)
		}
	}
}

func TestWaitRunBounded(t *testing.T) {
	m := newTestModel()
	m.runDone = make(chan struct{})
	start := time.Now()
	m.waitRun(50 * time.Millisecond)
	if time.Since(start) > time.Second {
		t.Fatal("waitRun must give up")
	}
	close(m.runDone)
	m.waitRun(time.Minute) // returns at once
}

func TestRestartSessionKeepsModelAndAllowed(t *testing.T) {
	m := newAgentModel(t)
	if err := m.agent.SetModel("fake/m2", ""); err != nil {
		t.Fatal(err)
	}
	m.status.Git, m.status.Branch = true, "main"
	reply := make(chan tools.Answer, 1)
	m.lastTyped = time.Time{}
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	m.Update(key("ctrl+a"))
	old := m.agent
	cmd := m.restartSession()
	if cmd == nil || m.agent == old {
		t.Fatal("new session expected")
	}
	if got := m.agent.Status().Model.Qualified(); got != "fake/m2" {
		t.Fatalf("model after /clear: %s", got)
	}
	if !m.status.Git || m.status.Branch != "main" {
		t.Fatal("branch survives /clear")
	}
	found := false
	for _, n := range m.start.Config.Shell.Allow {
		found = found || n == "python3"
	}
	if !found {
		t.Fatal("a command approved with [A] must survive /clear")
	}
}

func TestRestartFailureKeepsOldSession(t *testing.T) {
	m := newAgentModel(t)
	old := m.agent
	m.start.Workdir = "/nonexistent/moca-test-dir" // Start fails: the jail root is missing
	cmd := m.restartSession()
	if m.agent != old || !strings.Contains(printed(cmd), "error") {
		t.Fatalf("failed restart keeps the old agent: %q", printed(cmd))
	}
	if err := m.agent.AddNote("still open"); err != nil {
		t.Fatalf("old session must stay writable: %v", err)
	}
}

func TestRunDoneReturnsSteeringToEditor(t *testing.T) {
	m := newAgentModel(t)
	m.running = true
	m.input.SetBuffer("draft")
	m.agent.Steer("queued one")
	m.agent.Steer("queued two")
	m.handleRunDone(runDoneMsg{err: context.Canceled})
	if got := m.ta.Value(); got != "queued one\nqueued two\ndraft" {
		t.Fatalf("editor %q", got)
	}
}

// Permanent approvals are project-scoped (rev 28): the shell's allow-always
// lands in the workdir's .moca/config.jsonc as a permissions.allow rule, and
// the user config stays untouched.
func TestAllowAlwaysPersists(t *testing.T) {
	m := newAgentModel(t)
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	_, cmd := m.Update(key("ctrl+a"))
	if got := <-reply; got != tools.AllowAlways {
		t.Fatal(got)
	}
	proj := config.ProjectConfigFile(m.start.Workdir)
	if !strings.Contains(printed(cmd), "always allowing") || !strings.Contains(printed(cmd), proj) {
		t.Fatalf("confirmation line: %q", printed(cmd))
	}
	ack(m)
	// The written file is a valid project config, and the user config is not
	// the target any more.
	p, err := config.LoadProject(m.start.Workdir)
	if err != nil {
		t.Fatalf("written project file must parse (%v)", err)
	}
	if p == nil || !slices.Contains(p.Permissions.Allow, "python3") {
		t.Fatalf("project allow list: %+v", p)
	}
	if b, _ := os.ReadFile(m.opts.ConfigPath); strings.Contains(string(b), "python3") {
		t.Fatalf("the user config must stay untouched: %s", b)
	}
	// A failing write is reported, but the command is still allowed.
	m.start.Workdir = filepath.Join(t.TempDir(), "file") // a path whose .moca/ cannot be created
	if err := os.WriteFile(m.start.Workdir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "node", CanAlways: true}, reply: reply})
	_, cmd = m.Update(key("ctrl+a"))
	if got := <-reply; got != tools.AllowAlways || !strings.Contains(printed(cmd), "not saved") {
		t.Fatalf("failure line: %q", printed(cmd))
	}
}

// An untrusted workdir cannot have project rules applied: the note says so,
// because the saved rule only bites once the workdir is trusted.
func TestAllowAlwaysNotesAnUntrustedWorkdir(t *testing.T) {
	m := newAgentModel(t)
	if m.start.Trusted {
		t.Fatal("precondition: the helper's workdir is untrusted")
	}
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	_, cmd := m.Update(key("ctrl+a"))
	if got := <-reply; got != tools.AllowAlways {
		t.Fatal(got)
	}
	out := printed(cmd)
	if !strings.Contains(out, "once this workdir is trusted") {
		t.Fatalf("untrusted note missing: %q", out)
	}
	// The in-session copies still learn it (a /clear restart honours it even
	// though the file's rules apply only to a trusted workdir).
	if !slices.Contains(m.start.Config.Shell.Allow, "python3") || !slices.Contains(m.start.Config.Project.Permissions.Allow, "python3") {
		t.Fatalf("in-memory copies: %+v %+v", m.start.Config.Shell.Allow, m.start.Config.Project.Permissions.Allow)
	}
}

// The MCP allow-always lands in the project file too: the server it names may
// be the user's own definition, so the entry is a patch — approve only — that
// the project layer merges back over it (§10.5).
func TestAllowAlwaysMCPPersists(t *testing.T) {
	m := newAgentModel(t)
	m.start.Config.MCP.Servers = map[string]config.MCPServer{"ctx7": {URL: "https://x"}}
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "mcp", Subject: "ctx7/create_issue", Detail: `{"title":"x"}`, CanAlways: true}, reply: reply})
	_, cmd := m.Update(key("ctrl+a"))
	if got := <-reply; got != tools.AllowAlways {
		t.Fatal(got)
	}
	b, _ := os.ReadFile(config.ProjectConfigFile(m.start.Workdir))
	p, err := config.LoadProject(m.start.Workdir)
	if err != nil {
		t.Fatalf("written project file must stay valid (%v):\n%s", err, b)
	}
	if s := p.MCP.Servers["ctx7"]; len(s.Approve) != 1 || s.Approve[0] != "create_issue" || s.Command != "" || s.URL != "" {
		t.Fatalf("project patch entry: %+v", s)
	}
	if !strings.Contains(printed(cmd), "always allowing") {
		t.Fatalf("confirmation line: %q", printed(cmd))
	}
	// /clear restarts from m.start.Config: the project layer must carry it,
	// and Start merges it back over the user's server definition.
	if s := m.start.Config.Project.MCP.Servers["ctx7"]; len(s.Approve) != 1 || s.Approve[0] != "create_issue" {
		t.Fatalf("/clear copy must learn it: %+v", s)
	}
	// An untrusted workdir's restart never merges the project layer, so the
	// user-layer copy learns it too (the merge unites the two lists).
	if s := m.start.Config.MCP.Servers["ctx7"]; len(s.Approve) != 1 || s.Approve[0] != "create_issue" {
		t.Fatalf("user-layer /clear copy must learn it: %+v", s)
	}
	merged, warns := mcp.MergeProject(m.start.Workdir, m.opts.ConfigPath, config.ProjectConfigFile(m.start.Workdir), m.start.Config.MCP.Servers, m.start.Config.Project.MCP)
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if s := merged["ctx7"]; s.URL != "https://x" || len(s.Approve) != 1 {
		t.Fatalf("merged server: %+v", s)
	}
}

func TestTrustPromptCancel(t *testing.T) {
	for _, k := range []string{"ctrl+c", "esc"} {
		m := &trustModel{dir: "/x"}
		m.Update(key(k))
		if !m.cancelled || m.answer {
			t.Fatalf("%s must cancel, not answer no", k)
		}
	}
	m := &trustModel{dir: "/x"}
	m.Update(key("n"))
	if m.cancelled || m.answer || !m.done {
		t.Fatal("n is an explicit no")
	}
}

func TestStatusWideRunesNeverExceedWidth(t *testing.T) {
	s := StatusInfo{Version: "v0.1.0-alpha-1-gc392fac", Cwd: "~/プロジェクト/日本語のとても長い名前", Branch: "feature/絵文字🙂", Git: true,
		Model: "fake/m", Effort: "high", Window: 128 * 1024, Used: 1000, In: 1000, Out: 100, Cost: 0.5}
	for _, w := range []int{100, 60, 40, 25, 20} {
		l1, l2 := RenderStatus(s, w)
		for _, l := range [][]segment{l1, l2} {
			if got := lipgloss.Width(plainLine(l)); got > w {
				t.Errorf("width %d: rendered %d cells", w, got)
			}
		}
	}
}

func TestGitStatusUsesNoOptionalLocks(t *testing.T) {
	args := strings.Join(gitStatusArgs("/repo"), " ")
	if !strings.Contains(args, "--no-optional-locks") {
		t.Fatal(args)
	}
}

func TestTrustCancelIsNotAnAnswer(t *testing.T) {
	if _, err := (&trustModel{cancelled: true}).result(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel must surface as cancellation: %v", err)
	}
	if v, err := (&trustModel{answer: true}).result(); err != nil || !v {
		t.Fatal(v, err)
	}
}

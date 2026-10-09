package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// A first ctrl+c on an empty draft says how to quit, and the hint expires
// with the quit window.
func TestCtrlCQuitHint(t *testing.T) {
	m := newTestModel()
	now := time.Now()
	m.now = func() time.Time { return now }
	_, cmd := m.Update(key("ctrl+c"))
	if cmd == nil {
		t.Fatal("arming quit schedules the hint's expiry")
	}
	if !strings.Contains(m.View().Content, "press ctrl+c again to quit") {
		t.Fatal("armed quit shows the hint")
	}
	now = now.Add(1100 * time.Millisecond)
	m.Update(quitHintMsg{})
	if strings.Contains(m.View().Content, "press ctrl+c again") {
		t.Fatal("the hint outlived the quit window")
	}
}

// ctrl+c inside the pager clears the draft in both the input state and the
// textarea (they used to diverge, resurrecting the text on the next key).
func TestPagerCtrlCClearsTextarea(t *testing.T) {
	m := newTestModel()
	for _, r := range "draft" {
		m.Update(keyMsg(string(r)))
	}
	openPager(m)
	m.Update(key("ctrl+c"))
	if m.ta.Value() != "" || m.input.Buffer() != "" {
		t.Fatalf("draft survived ctrl+c: ta=%q buf=%q", m.ta.Value(), m.input.Buffer())
	}
}

// ctrl+o on a pending approval opens the whole command.
func TestCtrlOOpensApprovalCommand(t *testing.T) {
	m := newTestModel()
	m.running = true
	m.approval = &approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", Detail: "python3 - <<'EOF'\nimport os\nEOF"}, reply: make(chan tools.Answer, 1)}
	m.Update(key("ctrl+o"))
	if m.pager == nil || !strings.Contains(m.pager.vp.GetContent(), "import os") {
		t.Fatal("ctrl+o must page the pending command")
	}
}

// A failed tool's mark is red; its text stays muted.
func TestToolErrorMarkRed(t *testing.T) {
	m := newTestModel()
	out := printed(m.handleAgent(agent.ToolEnd{Call: llm.ToolCall{Name: "read"}, Result: tools.Result{IsError: true, Content: "no such file"}}))
	if !strings.Contains(out, "✗") || !strings.Contains(out, "read no such file") {
		t.Fatalf("error item line: %q", out)
	}
}

// A final turn without text keeps the previous answer for /copy.
func TestCopyKeepsLastText(t *testing.T) {
	m := newTestModel()
	m.handleAgent(agent.TurnEnd{Message: llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "answer"}}}})
	m.handleAgent(agent.TurnEnd{Message: llm.Message{Role: llm.RoleAssistant}})
	if m.lastAssistant != "answer" {
		t.Fatalf("lastAssistant = %q", m.lastAssistant)
	}
}

// esc kills a running `!` command and the output says so.
func TestEscCancelsShell(t *testing.T) {
	m := newTestModel()
	cmd := m.submitShell(Parsed{Kind: KindShell, Text: "sleep 10"})
	if !m.shellBusy || !strings.Contains(m.activityLine(), "$ sleep 10") {
		t.Fatal("a `!` command shows in the activity row")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatalf("submit returned %T", cmd())
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- batch[0]() }()
	time.Sleep(200 * time.Millisecond)
	m.Update(key("esc"))
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("esc did not kill the command")
	}
	_, out := m.Update(msg)
	if got := printed(out); !strings.Contains(got, "[interrupted") {
		t.Fatalf("output: %q", got)
	}
	if m.shellBusy || m.busy() {
		t.Fatal("still busy after the command ended")
	}
}

// The exit line names a used session only.
func TestExitLine(t *testing.T) {
	m := newAgentModel(t)
	if m.exitLine() != "" {
		t.Fatal("an untouched session needs no resume hint")
	}
	m.sessionUsed = true
	if l := m.exitLine(); !strings.Contains(l, "moca --resume "+m.agent.Session().ID8()) {
		t.Fatalf("exit line %q", l)
	}
}

// The exit frame keeps the status bar and drops the input box, so the exit
// line prints below the final numbers instead of over a stale draft.
func TestQuittingFrame(t *testing.T) {
	m := newTestModel()
	m.ta.SetValue("draft")
	m.pullTextarea()
	m.quit()
	v := m.View().Content
	if strings.Contains(v, "draft") || !strings.Contains(v, "$0.0000") {
		t.Fatalf("exit frame: %q", v)
	}
}

// A response after tool/thinking items is set off by a blank row; the live
// partial line carries the same gap so committing it does not shift it.
func TestResponseGapAfterItems(t *testing.T) {
	m := newTestModel()
	m.printlnUser("› hi")
	if got := printed(m.printlnResponse("first")); strings.HasPrefix(got, "{\n") {
		t.Fatalf("no gap after a user message: %q", got)
	}
	settle(m)
	m.printItem("▸ #1 read x", false)
	settle(m)
	if !strings.HasPrefix(m.liveResponse("par"), "\n") {
		t.Fatal("live line must carry the gap")
	}
	if got := printed(m.printlnResponse("second")); !strings.HasPrefix(got, "{\n") {
		t.Fatalf("response after an item needs the gap: %q", got)
	}
	if m.responseGap() != "" {
		t.Fatal("only the first response row after items has the gap")
	}
}

// Wrapped and multi-line user text hangs under the text, not the `›`.
func TestUserHangingIndent(t *testing.T) {
	m := newTestModel()
	m.width = 24
	got := ansi.Strip(printed(m.printlnUser("› aaaa bbbb cccc dddd eeee ffff\nsecond")))
	rows := strings.Split(strings.Trim(got, "{}\n"), "\n")
	if len(rows) < 3 || !strings.HasPrefix(rows[0], " › aaaa") || !strings.HasPrefix(rows[1], "   ") || !strings.HasPrefix(rows[len(rows)-1], "   second") {
		t.Fatalf("rows: %q", rows)
	}
	for _, r := range rows {
		if lipgloss.Width(r) != 24 {
			t.Fatalf("row %q is %d cells, want 24", r, lipgloss.Width(r))
		}
	}
}

// Item lines stay on one row.
func TestItemLineClamped(t *testing.T) {
	m := newTestModel()
	m.width = 30
	got := ansi.Strip(printed(m.printItem("✗ #4 shell refused: "+strings.Repeat("long ", 20), true)))
	body := strings.Trim(got, "{}\n")
	if strings.Contains(body, "\n") || lipgloss.Width(body) > 30 || !strings.HasSuffix(body, "…") {
		t.Fatalf("item line: %q", body)
	}
}

// The shift+enter hint shows once per machine; the placeholder keeps naming
// the fallback key.
func TestShiftEnterHintOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hints.json")
	m := newTestModel()
	m.opts.HintsPath = path
	_, cmd := m.Update(hintCheckMsg{})
	if !strings.Contains(printed(cmd), "ctrl+j") || !strings.Contains(m.ta.Placeholder, "ctrl+j") {
		t.Fatal("first launch shows the hint and the placeholder")
	}
	m2 := newTestModel()
	m2.opts.HintsPath = path
	_, cmd = m2.Update(hintCheckMsg{})
	if cmd != nil || !strings.Contains(m2.ta.Placeholder, "ctrl+j") {
		t.Fatalf("second launch: no scrollback hint, placeholder stays (%v)", cmd)
	}
}

// The activity row names the operand, the streamed-token estimate and the
// queued steering count.
func TestActivityRowDetails(t *testing.T) {
	m := newAgentModel(t)
	m.running = true
	m.runStart = m.now()
	m.handleAgent(agent.ToolStart{Call: llm.ToolCall{Name: "shell", Input: []byte(`{"command":"go test ./...\nsecond"}`)}})
	m.handleAgent(agent.TextDelta{Text: strings.Repeat("x", 4800)})
	m.agent.Steer("also this")
	got := ansi.Strip(m.activityLine())
	for _, want := range []string{"shell go test ./...… ", "↓1.2k tok", "1 queued", "esc to interrupt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("activity row lacks %q: %q", want, got)
		}
	}
	m.handleAgent(agent.ToolEnd{Call: llm.ToolCall{Name: "shell"}, Result: tools.Result{Summary: "x"}})
	if strings.Contains(ansi.Strip(m.activityLine()), "go test") {
		t.Fatal("operand clears when the tool ends")
	}
}

func rawOf(cmd tea.Cmd) string {
	if cmd == nil {
		return ""
	}
	if r, ok := cmd().(tea.RawMsg); ok {
		return fmt.Sprint(r.Msg)
	}
	return ""
}

// Notifications fire only while the terminal is unfocused, only for a long
// run or a waiting approval, and honor tui.notify.
func TestNotifyWhenBlurred(t *testing.T) {
	m := newAgentModel(t)
	m.start.Config.TUI.Notify = "osc9"
	if m.notify("x") != nil {
		t.Fatal("a focused terminal needs no notification")
	}
	m.Update(tea.BlurMsg{})
	if got := rawOf(m.notify("moca: hi\x1b[2J")); got != "\x1b]9;moca: hi[2J\a" {
		t.Fatalf("osc9: %q", got)
	}
	m.start.Config.TUI.Notify = "bell"
	if got := rawOf(m.notify("x")); got != "\a" {
		t.Fatalf("bell: %q", got)
	}
	m.start.Config.TUI.Notify = "off"
	if m.notify("x") != nil {
		t.Fatal("off is off")
	}
	m.start.Config.TUI.Notify = "osc9"
	m.Update(tea.FocusMsg{})
	if m.notify("x") != nil {
		t.Fatal("focus returns: silent again")
	}
}

func TestNotifyOnApprovalAndLongRun(t *testing.T) {
	m := newAgentModel(t)
	m.start.Config.TUI.Notify = "osc9"
	m.Update(tea.BlurMsg{})
	_, cmd := m.Update(approvalMsg{q: tools.Question{Subject: "go"}, reply: make(chan tools.Answer, 1)})
	if !strings.Contains(rawOf(cmd), "approval needed") {
		t.Fatalf("approval waiting while blurred notifies: %q", rawOf(cmd))
	}
	now := time.Now()
	m.now = func() time.Time { return now }
	hasRaw := func(cmd tea.Cmd) bool {
		b, ok := cmd().(tea.BatchMsg)
		if !ok {
			return false
		}
		for _, c := range b {
			if c != nil {
				if r, ok := c().(tea.RawMsg); ok && strings.Contains(fmt.Sprint(r.Msg), "run finished") {
					return true
				}
			}
		}
		return false
	}
	m.runStart = now.Add(-10 * time.Second)
	if _, cmd = m.Update(runDoneMsg{}); cmd != nil && hasRaw(cmd) {
		t.Fatal("a short run does not notify")
	}
	m.runStart = now.Add(-45 * time.Second)
	if _, cmd = m.Update(runDoneMsg{}); cmd == nil || !hasRaw(cmd) {
		t.Fatal("a long run finishing while blurred notifies")
	}
}

// shift+tab walks the model's effort levels and wraps; a model without
// levels says so.
func TestShiftTabCyclesEffort(t *testing.T) {
	m := newAgentModel(t)
	out, _ := simulate(m, m.cycleEffort())
	if !strings.Contains(out, "no effort levels") {
		t.Fatalf("custom model without levels: %q", out)
	}
	t.Setenv("OPENCODE_API_KEY", "k")
	if err := m.agent.SetModel("opencode-go/glm-5.3", ""); err != nil {
		t.Fatal(err)
	}
	m.refreshStatus()
	start := m.agent.Status().Effort
	seen := []llm.Effort{start}
	for i := 0; i < 8; i++ {
		simulate(m, m.cycleEffort())
		e := m.agent.Status().Effort
		if e == start {
			break
		}
		seen = append(seen, e)
	}
	if len(seen) < 2 {
		t.Fatalf("effort never moved from %s", start)
	}
	if m.status.Effort != m.agent.Status().Effort {
		t.Fatal("the bar follows")
	}
	m.running = true
	if out, _ := simulate(m, m.cycleEffort()); !strings.Contains(out, "interrupt the run first") {
		t.Fatalf("refused mid-run: %q", out)
	}
}

// Scrollback text is downsampled to the terminal's color profile: NO_COLOR
// (ASCII) drops colors but keeps attributes, 256-color terminals get no
// truecolor sequences.
func TestScrollbackHonorsColorProfile(t *testing.T) {
	m := newTestModel()
	full := printed(m.printlnUser("› hi"))
	if !strings.Contains(full, "48;2;") {
		t.Fatalf("truecolor is passed through: %q", full)
	}
	settle(m)
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.ASCII})
	got := printed(m.printlnUser("› hi"))
	if strings.Contains(got, "38;2;") || strings.Contains(got, "48;2;") || !strings.Contains(got, "› hi") {
		t.Fatalf("NO_COLOR keeps no colors: %q", got)
	}
	settle(m)
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.ANSI256})
	got = printed(m.printlnUser("› hi"))
	if strings.Contains(got, "38;2;") || strings.Contains(got, "48;2;") || !strings.Contains(got, "48;5;") {
		t.Fatalf("256-color terminals get 256-color codes: %q", got)
	}
}

package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

func TestFmtElapsed(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0: "0s", 7 * time.Second: "7s", 59 * time.Second: "59s", 65 * time.Second: "1m 05s",
		3600 * time.Second: "1h 00m", 3720 * time.Second: "1h 02m",
	} {
		if got := fmtElapsed(d); got != want {
			t.Errorf("fmtElapsed(%v) = %q, want %q", d, got, want)
		}
	}
}

// While a run is in progress the view shows a spinner row — what the agent is
// doing, elapsed time and the interrupt key; it is gone when the run ends and
// replaced by the prompt while an approval is pending.
func TestActivityLine(t *testing.T) {
	m := newTestModel()
	t0 := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return t0.Add(7 * time.Second) }
	m.runStart = t0
	if strings.Contains(m.View().Content, "esc to interrupt") {
		t.Fatal("no activity row while idle")
	}
	m.running = true
	row := ansi.Strip(m.activityLine())
	if row != "⠋ working… 7s · esc to interrupt" {
		t.Fatalf("activity row: %q", row)
	}
	m.toolBusy = "shell"
	if !strings.Contains(ansi.Strip(m.activityLine()), "shell… 7s") {
		t.Fatal("a running tool is named")
	}
	m.toolBusy = ""
	m.thinking.WriteString("hmm")
	if !strings.Contains(ansi.Strip(m.activityLine()), "thinking… 7s") {
		t.Fatal("thinking before any text is named")
	}
	m.live.WriteString("partial")
	if !strings.Contains(ansi.Strip(m.activityLine()), "working…") {
		t.Fatal("streaming text is plain work")
	}
	if !strings.Contains(m.View().Content, "esc to interrupt") {
		t.Fatal("the view shows the activity row while running")
	}
}

// The spinner ticks only for the live run: a stale tick (finished or replaced
// run) neither advances the frame nor schedules another tick.
func TestSpinnerTicksOnlyForTheLiveRun(t *testing.T) {
	m := newTestModel()
	m.running, m.runGen = true, 3
	if _, cmd := m.Update(spinMsg{gen: 3}); cmd == nil || m.spin != 1 {
		t.Fatalf("live tick must advance and reschedule (spin %d)", m.spin)
	}
	if _, cmd := m.Update(spinMsg{gen: 2}); cmd != nil || m.spin != 1 {
		t.Fatal("a tick from an older run must be dropped")
	}
	m.running = false
	if _, cmd := m.Update(spinMsg{gen: 3}); cmd != nil || m.spin != 1 {
		t.Fatal("no ticking after the run ends")
	}
}

// A refusal must not depend on the print command: while earlier output is in
// flight the explanation only queues (nil command), and the guarded command
// must still be refused.
func TestRefuseRunningWhileOutputInFlight(t *testing.T) {
	m := newTestModel()
	m.println("earlier output") // flush now in flight
	m.running = true
	cmd, refused := m.refuseRunning()
	if !refused || cmd != nil {
		t.Fatalf("refused=%v cmd-nil=%v: must refuse and queue the explanation", refused, cmd == nil)
	}
	if got := printed(ack(m)); !strings.Contains(got, "finish or interrupt the run first") {
		t.Fatalf("queued explanation lost: %q", got)
	}
	m.running = false
	if _, refused := m.refuseRunning(); refused {
		t.Fatal("idle model must not refuse")
	}
}

// The input box follows its content: one row when empty, a row per line up
// to what fits on screen (the window height minus inputChrome), then it
// scrolls — and it shrinks back as lines go. Lines beyond the cap are kept
// (the cap sizes the viewport, never the draft).
func TestInputBoxGrowsAndShrinks(t *testing.T) {
	m := newTestModel() // 40 rows → the box tops out at 32
	if h := m.ta.Height(); h != 1 {
		t.Fatalf("empty box is %d rows, want 1", h)
	}
	for i := 1; i <= 40; i++ {
		m.Update(key("alt+enter"))
		want := min(i+1, 40-inputChrome)
		if h := m.ta.Height(); h != want {
			t.Fatalf("%d lines: box is %d rows, want %d", i+1, h, want)
		}
	}
	if n := m.ta.LineCount(); n != 41 {
		t.Fatalf("draft has %d lines, want 41 (the cap must not drop lines)", n)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	if h := m.ta.Height(); h != 20-inputChrome {
		t.Fatalf("after a resize to 20 rows the box is %d rows, want %d", h, 20-inputChrome)
	}
	m.input.SetBuffer("")
	m.syncTextarea()
	if h := m.ta.Height(); h != 1 {
		t.Fatalf("cleared box is %d rows, want 1", h)
	}
}

// The terminal cursor sits on the input's cursor row, inside the box.
func TestViewCursorInsideInputBox(t *testing.T) {
	m := newTestModel()
	m.Update(key("alt+enter"))
	m.Update(key("alt+enter"))
	v := m.View()
	if v.Cursor == nil {
		t.Fatal("the view must carry the real cursor")
	}
	top := -1
	for i, l := range strings.Split(v.Content, "\n") {
		if strings.HasPrefix(ansi.Strip(l), "─") {
			top = i
			break
		}
	}
	if top < 0 || v.Cursor.Position.Y != top+1+2 {
		t.Fatalf("cursor row %d, want %d (box starts below the rule at %d)", v.Cursor.Position.Y, top+3, top)
	}
}

// A frame is never shorter than the cursor row the renderer may still
// remember (the inline renderer mis-clamps it otherwise and strands the old
// frame's top rows in the scrollback): after a tall draft is cleared the
// frame is padded with blank rows until the guard window has passed.
func TestShrinkIsPaddedUntilTheGuardWindowPasses(t *testing.T) {
	m := newTestModel()
	clock := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }
	for i := 0; i < 11; i++ {
		m.Update(key("alt+enter"))
	}
	tall := m.View()
	rows := func(v tea.View) int { return strings.Count(v.Content, "\n") + 1 }
	if got := rows(tall); got != 1+12+1+2 {
		t.Fatalf("tall frame is %d rows", got)
	}
	cursorY := tall.Cursor.Position.Y // the row the renderer will remember
	m.input.SetBuffer("")
	m.syncTextarea()
	clock = clock.Add(16 * time.Millisecond)
	short := m.View()
	if got := rows(short); got != cursorY+1 {
		t.Fatalf("shrunk frame is %d rows, want %d (padded to the remembered cursor row)", got, cursorY+1)
	}
	if strings.TrimRight(short.Content, "\n") == short.Content {
		t.Fatal("the padding rows must be blank lines below the status bar")
	}
	clock = clock.Add(3 * guardWindow)
	if got := rows(m.View()); got != 1+1+1+2 {
		t.Fatalf("settled frame is %d rows, want 5", got)
	}
}

// When several Views coalesce into one rendered frame (keystrokes arriving in
// one batch), the pad decision must remember the cursor row of the last frame
// that was actually flushed — not just the previous View's row, which the
// coalesced-away frames have already overwritten. Regression: typing "lo"
// over a tall dropdown stranded its top rows.
func TestCoalescedShrinkRemembersTheLastFlushedRow(t *testing.T) {
	m := newTestModel()
	clock := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }
	for i := 0; i < 11; i++ {
		m.Update(key("alt+enter"))
	}
	tall := m.View() // flushed at the next frame interval
	cursorY := tall.Cursor.Position.Y
	rows := func(v tea.View) int { return strings.Count(v.Content, "\n") + 1 }

	// A long pause: the tall frame is certainly the one the renderer
	// remembers, and it has long left any time window.
	clock = clock.Add(10 * time.Second)
	m.input.SetBuffer("")
	m.syncTextarea()
	mid := m.View() // the first frame of the new burst shrinks...
	clock = clock.Add(2 * time.Millisecond)
	small := m.View() // ...and a second, coalesced frame shrinks further

	for name, v := range map[string]tea.View{"mid": mid, "small": small} {
		if got := rows(v); got != cursorY+1 {
			t.Fatalf("%s frame is %d rows, want %d (padded to the last flushed cursor row)", name, got, cursorY+1)
		}
	}

	// Once a frame has certainly been flushed (gap > guardWindow), the pad
	// follows it down and settles.
	clock = clock.Add(3 * guardWindow)
	if got := rows(m.View()); got != 1+1+1+2 {
		t.Fatalf("settled frame is %d rows, want 5", got)
	}
}

// The padding never makes a frame taller than the screen.
func TestShrinkPaddingFitsTheScreen(t *testing.T) {
	m := newTestModel()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 12})
	for i := 0; i < 20; i++ {
		m.Update(key("alt+enter"))
	}
	m.View()
	m.input.SetBuffer("")
	m.syncTextarea()
	if got := strings.Count(m.View().Content, "\n") + 1; got > 12 {
		t.Fatalf("padded frame is %d rows on a 12-row screen", got)
	}
}

// A finished turn's thinking line says how to read it; alt+t opens the latest
// thinking block in the pager even when tool items came after it, and the
// pager wraps long reasoning lines instead of cutting them off.
func TestThinkingReadableInPager(t *testing.T) {
	m := newTestModel()
	long := "First I consider the options. " + strings.Repeat("The reasoning goes on and on. ", 12) + "END-OF-CHAIN"
	m.thinking.WriteString(long)
	_, cmd := m.Update(agentEventMsg{agent.TurnEnd{Message: llm.Message{Role: llm.RoleAssistant}}})
	if got := printed(cmd); !strings.Contains(got, "thinking 1 line · alt+t to read") {
		t.Fatalf("thinking line lacks the hint: %q", got)
	}
	m.items.AddTool(llm.ToolCall{Name: "ls"}, tools.Result{Summary: "2 entries"})
	m.Update(key("alt+t"))
	if m.pager == nil {
		t.Fatal("alt+t must open the pager on the thinking block")
	}
	view := ansi.Strip(m.pager.view(false))
	if !strings.Contains(view, "thinking 1 line") || !strings.Contains(view, "END-OF-CHAIN") {
		t.Fatalf("pager must show the whole reasoning (wrapped), got:\n%s", view)
	}
	m.pager = nil
	m2 := newTestModel() // no thinking yet: alt+t does nothing
	m2.Update(key("alt+t"))
	if m2.pager != nil {
		t.Fatal("alt+t without a thinking block must not open the pager")
	}
}

// Output produced while the pager is open is held and printed only after the
// renderer has left the alt screen: closing the pager schedules the release
// instead of printing at once (a Println that beats the exit is lost), and a
// reopened pager keeps holding.
func TestPagerCloseDefersTheRelease(t *testing.T) {
	m := newTestModel()
	m.thinking.WriteString("reasoning")
	m.Update(agentEventMsg{agent.TurnEnd{Message: llm.Message{Role: llm.RoleAssistant}}})
	ack(m)
	m.Update(key("alt+t"))
	if m.pager == nil {
		t.Fatal("pager should be open")
	}
	_, c := m.Update(agentEventMsg{agent.Warning{Text: "while reading"}})
	if c != nil || len(m.held) != 1 {
		t.Fatalf("output must be held while the pager is open (held %d)", len(m.held))
	}
	_, closeCmd := m.Update(key("q"))
	if m.pager != nil {
		t.Fatal("q closes the pager")
	}
	if len(m.held) != 1 {
		t.Fatal("closing must not release at once")
	}
	if _, cmd := m.Update(releaseMsg{}); !strings.Contains(printed(cmd), "while reading") || len(m.held) != 0 {
		t.Fatalf("release prints the held output: %q", printed(cmd))
	}
	_ = closeCmd

	m.held = []tea.Cmd{m.println("kept")}
	m.pager = &pagerModel{}
	if _, cmd := m.Update(releaseMsg{}); cmd != nil || len(m.held) != 1 {
		t.Fatal("a reopened pager keeps the output held")
	}
}

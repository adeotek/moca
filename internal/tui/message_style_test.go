package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// Submitted messages and assistant responses print on distinct scrollback
// backgrounds (the palette is pinned); light and dark variants differ; blank
// lines keep a band.
func TestScrollbackMessageBackgrounds(t *testing.T) {
	m := newTestModel()
	user := printed(m.printlnUser("› hi"))
	ack(m)
	resp := printed(m.printlnResponse("answer"))
	ack(m)
	if !strings.Contains(user, "48;2;61;67;76") || !strings.Contains(resp, "48;2;33;59;73") {
		t.Fatalf("palette changed: %q / %q", user, resp)
	}
	if user == resp {
		t.Fatal("user and response backgrounds must differ")
	}
	if got, want := printed(m.printlnUser("x")), styleBlock(userMsgStyle(true), "x", m.width); !strings.Contains(got, want) {
		t.Fatalf("user style plumbing: %q want %q", got, want)
	}
	ack(m)
	m.darkBG = false
	if printed(m.printlnUser("› hi")) == user {
		t.Fatal("light and dark variants must differ")
	}
	ack(m)
	if bands := printed(m.printlnResponse("a\n\nb")); strings.Count(bands, "48;2;") != 3 {
		t.Fatalf("blank line band missing: %q", bands)
	}
}

// A submitted message is set off from the output above by a blank line.
func TestUserMessageStartsWithBlankLine(t *testing.T) {
	m := newTestModel()
	if got := printed(m.printlnUser("› hi")); !strings.HasPrefix(got, "{\n") {
		t.Fatalf("no separating blank line: %q", got)
	}
}

// A submitted / command echoes into the scrollback as a user message — the
// same `›` band and blank line — and its output follows the echo.
func TestCommandEchoesAsUserMessage(t *testing.T) {
	m := newAgentModel(t)
	out := drained(m, tuiCmd(t, m, "/help"))
	band := styleBlock(userMsgStyle(true), "› /help", m.width)
	iBand := strings.Index(out, band)
	if iBand < 0 {
		t.Fatalf("no user-message band for the command:\n%q", out)
	}
	if !strings.HasPrefix(out, "{\n") {
		t.Fatalf("the echo must start with a blank line:\n%q", out)
	}
	marker := "type / for the command dropdown"
	iHelp := strings.Index(out, marker)
	if iHelp < 0 || iHelp < iBand {
		t.Fatalf("the command output must follow the echo:\n%q", out)
	}
	// A refused command echoes too, before the refusal.
	m.running = true
	out = drained(m, tuiCmd(t, m, "/clear"))
	if !strings.Contains(out, styleBlock(userMsgStyle(true), "› /clear", m.width)) {
		t.Fatalf("refused command must echo:\n%q", out)
	}
	if !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("refusal missing:\n%q", out)
	}
}

// Tool-usage, thinking and notice lines render in muted #96a0a4 with no
// background — the tool item path through Update included; errors are red
// and warnings yellow.
func TestMutedLinesNoBackground(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(agentEventMsg{agent.ToolEnd{Call: llm.ToolCall{Name: "ls"}, Result: tools.Result{Summary: "2 entries"}}})
	got := printed(cmd)
	if !strings.Contains(got, "▸ #1 ls 2 entries") || !strings.Contains(got, "38;2;150;160;164") || strings.Contains(got, "48;") {
		t.Fatalf("tool line styling: %q", got)
	}
	ack(m)
	line := printed(m.printlnMuted("⋯ #2 thinking 3 lines"))
	if !strings.Contains(line, "38;2;150;160;164") || strings.Contains(line, "48;") {
		t.Fatalf("thinking line styling: %q", line)
	}
	ack(m)
	if e := printed(m.printlnError("error: boom\x1b[2J")); !strings.Contains(e, "error: boom^[[2J") || !strings.Contains(e, "31m") {
		t.Fatalf("error line: %q", e)
	}
	ack(m)
	if w := printed(m.printlnWarn("warning: careful")); !strings.Contains(w, "warning: careful") || !strings.Contains(w, "33m") {
		t.Fatalf("warning line: %q", w)
	}
}

// Every styled row spans exactly the terminal width (the background covers
// the entire row); longer lines wrap inside one cell of padding on each side.
func TestMessageBackgroundsCoverTheFullRow(t *testing.T) {
	m := newTestModel()
	long := strings.Repeat("word ", 60)
	for _, in := range []string{"a\n\nb", long, strings.Repeat("x", m.width*2+5)} {
		for _, l := range strings.Split(styleBlock(respMsgStyle(true), in, m.width), "\n") {
			if w := lipgloss.Width(l); w != m.width {
				t.Fatalf("row is %d cells, want %d: %q", w, m.width, ansi.Strip(l))
			}
		}
	}
	rows := strings.Split(ansi.Strip(styleBlock(respMsgStyle(true), long, m.width)), "\n")
	if len(rows) < 3 || !strings.HasPrefix(rows[1], " word") {
		t.Fatalf("wrapped rows must keep the left padding: %q", rows)
	}
}

// Tab-indented text (Go code) must not push a row past the terminal width —
// the tabs are expanded before the padding is measured.
func TestStyleBlockExpandsTabs(t *testing.T) {
	m := newTestModel()
	in := "func main() {\n\tif x {\n\t\treturn\n\t}\n}"
	rows := strings.Split(styleBlock(respMsgStyle(true), in, m.width), "\n")
	if len(rows) != 5 {
		t.Fatalf("%d rows, want 5", len(rows))
	}
	for _, l := range rows {
		if w := lipgloss.Width(l); w != m.width {
			t.Fatalf("row is %d cells, want %d: %q", w, m.width, ansi.Strip(l))
		}
	}
	if got := ansi.Strip(rows[2]); !strings.HasPrefix(got, " "+strings.Repeat(" ", 2*tabWidth)+"return") {
		t.Fatalf("indent lost: %q", got)
	}
}

// With no size yet (before the first WindowSizeMsg) nothing wraps or fills.
func TestStyleBlockWithoutWidth(t *testing.T) {
	got := ansi.Strip(styleBlock(respMsgStyle(true), strings.Repeat("x", 300), 0))
	if got != " "+strings.Repeat("x", 300) {
		t.Fatalf("unexpected wrap/fill: %d cells", lipgloss.Width(got))
	}
}

// Streamed assistant text commits with the response background, and the
// still-open partial line shows the same band.
func TestResponseCommitUsesBackground(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(agentEventMsg{agent.TextDelta{Text: "line\npar"}})
	got := printed(cmd)
	ack(m)
	if want := printed(m.printlnResponse("line")); got != want {
		t.Fatalf("commit %q want %q", got, want)
	}
	if v := m.View().Content; !strings.Contains(v, styleBlock(respMsgStyle(true), "par", m.width)) {
		t.Fatalf("live partial line lacks the response band: %q", v)
	}
}

// Scrollback output stays in emission order however fast it is produced:
// while a flush is in flight later lines queue (no command of their own) and
// go out, in order, as one flush when the renderer acknowledges.
func TestScrollbackOrderedUnderBurst(t *testing.T) {
	m := newTestModel()
	first := m.println("one")
	if first == nil || printed(first) != "{one}" {
		t.Fatalf("first flush: %q", printed(first))
	}
	if m.println("two") != nil || m.println("three") != nil {
		t.Fatal("output must queue while a flush is in flight")
	}
	if got := printed(ack(m)); got != "{two\nthree}" {
		t.Fatalf("queued flush: %q", got)
	}
	if ack(m) != nil || m.outBusy {
		t.Fatal("an acknowledged empty queue must go idle")
	}
	if m.println("four") == nil {
		t.Fatal("idle queue must flush immediately")
	}
}

// Output produced while the pager is open is held and still comes out in
// order once it closes.
func TestScrollbackOrderedAcrossPager(t *testing.T) {
	m := newTestModel()
	m.pager = &pagerModel{}
	_, c1 := m.Update(agentEventMsg{agent.Warning{Text: "w1"}})
	_, c2 := m.Update(agentEventMsg{agent.Warning{Text: "w2"}})
	if c1 != nil || c2 != nil {
		t.Fatal("output must be held while the pager is open")
	}
	m.pager = nil // closed: the held flush goes out, its ack sends the rest
	first := printed(m.release())
	second := printed(ack(m))
	if !strings.Contains(first, "w1") || strings.Contains(first, "w2") || !strings.Contains(second, "w2") {
		t.Fatalf("held output out of order: %q then %q", first, second)
	}
}

// A terminal-reported background switches the variants; dark stays the
// default until it arrives.
func TestBackgroundColorMsgSwitchesVariant(t *testing.T) {
	m := newTestModel()
	if !m.darkBG {
		t.Fatal("dark is the default until the terminal reports otherwise")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 255, G: 255, B: 255, A: 255}})
	if m.darkBG {
		t.Fatal("a light background must select the light variants")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.RGBA{A: 255}})
	if !m.darkBG {
		t.Fatal("a dark background must select the dark variants")
	}
}

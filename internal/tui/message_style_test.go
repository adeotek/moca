package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	resp := printed(m.printlnResponse("answer"))
	if !strings.Contains(user, "48;2;61;67;76") || !strings.Contains(resp, "48;2;33;59;73") {
		t.Fatalf("palette changed: %q / %q", user, resp)
	}
	if user == resp {
		t.Fatal("user and response backgrounds must differ")
	}
	if got, want := printed(m.printlnUser("x")), styleBlock(userMsgStyle(true), "x", m.width); !strings.Contains(got, want) {
		t.Fatalf("user style plumbing: %q want %q", got, want)
	}
	m.darkBG = false
	if printed(m.printlnUser("› hi")) == user {
		t.Fatal("light and dark variants must differ")
	}
	if bands := printed(m.printlnResponse("a\n\nb")); strings.Count(bands, "48;2;") != 3 {
		t.Fatalf("blank line band missing: %q", bands)
	}
}

// Tool-usage and thinking item lines render in muted #96a0a4 with no
// background — the tool item path through Update included.
func TestToolLinesMutedForegroundNoBackground(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(agentEventMsg{agent.ToolEnd{Call: llm.ToolCall{Name: "ls"}, Result: tools.Result{Summary: "2 entries"}}})
	got := printed(cmd)
	if !strings.Contains(got, "▸ #1 ls 2 entries") || !strings.Contains(got, "38;2;150;160;164") || strings.Contains(got, "48;") {
		t.Fatalf("tool line styling: %q", got)
	}
	line := printed(printlnTool("⋯ #2 thinking 3 lines"))
	if !strings.Contains(line, "38;2;150;160;164") || strings.Contains(line, "48;") {
		t.Fatalf("thinking line styling: %q", line)
	}
}

// Every styled row spans the full terminal width (the background covers the
// entire row); longer lines are left unpadded.
func TestMessageBackgroundsCoverTheFullRow(t *testing.T) {
	m := newTestModel()
	strip := func(s string) string { return strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}") }
	for _, l := range strings.Split(strip(printed(m.printlnResponse("a\n\nb"))), "\n") {
		if w := lipgloss.Width(l); w != m.width {
			t.Fatalf("row is %d cells, want %d", w, m.width)
		}
	}
	long := strings.Repeat("x", m.width+1)
	if w := lipgloss.Width(styleBlock(respMsgStyle(true), long, m.width)); w != m.width+2 {
		t.Fatalf("long row width %d, want %d", w, m.width+2)
	}
}

// Streamed assistant text commits with the response background.
func TestResponseCommitUsesBackground(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(agentEventMsg{agent.TextDelta{Text: "line\npar"}})
	if got, want := printed(cmd), printed(m.printlnResponse("line")); got != want {
		t.Fatalf("commit %q want %q", got, want)
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

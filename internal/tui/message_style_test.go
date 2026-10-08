package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
)

// Submitted messages and assistant responses print on distinct scrollback
// backgrounds; light and dark variants differ; blank lines keep a band.
func TestScrollbackMessageBackgrounds(t *testing.T) {
	m := newTestModel()
	user := printed(m.printlnUser("› hi"))
	resp := printed(m.printlnResponse("answer"))
	if !strings.Contains(user, "48;2;") || !strings.Contains(resp, "48;2;") {
		t.Fatalf("background missing: %q / %q", user, resp)
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

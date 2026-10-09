package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func plainMD(s string, width int) string {
	var st mdState
	return ansi.Strip(renderMarkdown(&st, s, width, true))
}

func TestMarkdownBlocks(t *testing.T) {
	cases := []struct{ in, want string }{
		{"# Title", " Title"},
		{"### Sub **bold**", " Sub bold"},
		{"- item", " • item"},
		{"  * nested", "   • nested"},
		{"1. numbered", " 1. numbered"},
		{"> quoted", " │ quoted"},
		{"plain text", " plain text"},
		{"**bold** and `code` here", " bold and code here"},
		{"unbalanced **bold", " unbalanced **bold"},
		{"lone ` tick", " lone ` tick"},
		{"snake_case_name and a*b*c", " snake_case_name and a*b*c"},
		{"empty `` pair", " empty `` pair"},
	}
	for _, c := range cases {
		if got := strings.TrimRight(plainMD(c.in, 0), " "); got != c.want {
			t.Errorf("%q → %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMarkdownFence(t *testing.T) {
	var st mdState
	got := ansi.Strip(renderMarkdown(&st, "before\n```go\nfunc main() {\n\treturn   1\n}\n```\nafter **b**", 0, true))
	want := " before\n ─ go\n func main() {\n     return   1\n }\n \n after b"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	if st.inFence {
		t.Fatal("fence closed")
	}
	// State carries across calls (streamed lines arrive in batches); markdown
	// markers inside code stay literal.
	renderMarkdown(&st, "```", 0, true)
	if !st.inFence {
		t.Fatal("opener sets the state")
	}
	if got := plainMD("x", 0); got != " x" {
		t.Fatal(got)
	}
	if got := ansi.Strip(renderMarkdown(&st, "**not bold** `x`", 0, true)); !strings.Contains(got, "**not bold** `x`") {
		t.Fatalf("inside a fence markers are literal: %q", got)
	}
}

// Every row is exactly the terminal width; code rows are hard-wrapped at the
// width (never word-wrapped), prose is word-wrapped.
func TestMarkdownWidthAndWrap(t *testing.T) {
	var st mdState
	out := renderMarkdown(&st, "aaa bbb ccc ddd eee fff ggg\n```\nxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx yy\n```", 20, true)
	for _, r := range strings.Split(out, "\n") {
		if w := lipgloss.Width(r); w != 20 {
			t.Fatalf("row %q is %d cells", ansi.Strip(r), w)
		}
	}
	rows := strings.Split(ansi.Strip(out), "\n")
	if !strings.HasPrefix(rows[0], " aaa bbb ccc ddd") || rows[1] != " eee fff ggg"+strings.Repeat(" ", 8) {
		t.Fatalf("prose rows: %q", rows)
	}
	if rows[3] != " "+strings.Repeat("x", 18)+" " || rows[4] != " "+strings.Repeat("x", 12)+" yy"+strings.Repeat(" ", 4) {
		t.Fatalf("code rows: %q", rows[3:])
	}
}

// A span that wraps keeps its style on the continuation row, and no row
// leaves a toggle open (rows are rendered independently).
func TestMarkdownSpanAcrossWrap(t *testing.T) {
	var st mdState
	out := renderMarkdown(&st, "**aaa bbb ccc ddd** tail", 10, true)
	for _, r := range strings.Split(out, "\n") {
		if strings.Count(r, sgrBoldOn) > strings.Count(r, sgrBoldOff) {
			t.Fatalf("row leaves bold open: %q", r)
		}
		// Only the row's own final reset may appear: a reset in the middle
		// would drop the band background for the padding after it.
		if strings.Count(r, "\x1b[m") != 1 || !strings.HasSuffix(r, "\x1b[m") {
			t.Fatalf("stray reset inside the band: %q", r)
		}
	}
	if !strings.Contains(strings.Split(out, "\n")[1], sgrBoldOn) {
		t.Fatalf("continuation row lost the bold: %q", out)
	}
}

// Model text cannot inject its own escapes: only ours survive.
func TestMarkdownSanitizes(t *testing.T) {
	var st mdState
	out := renderMarkdown(&st, "x\x1b[2Jy `\x1b[31mz`", 0, true)
	if strings.Contains(out, "\x1b[2J") || strings.Contains(out, "\x1b[31m") {
		t.Fatalf("raw escape reached the terminal: %q", out)
	}
}

// The streamed path: lines print through the fence state; the live partial
// line previews with a copy and never moves the real state.
func TestResponseFenceAcrossDeltas(t *testing.T) {
	m := newTestModel()
	m.printlnResponse("```go")
	if !m.md.inFence {
		t.Fatal("fence opened")
	}
	_ = m.liveResponse("```") // would close the fence if it mutated the state
	if !m.md.inFence {
		t.Fatal("live preview must not move the fence")
	}
	m.printlnResponse("```")
	if m.md.inFence {
		t.Fatal("fence closed")
	}
}

// Streamed text commits in batches; a fence opened in one batch is still open
// in the next (commitLive must not reset the state).
func TestFenceSurvivesCommitLive(t *testing.T) {
	m := newTestModel()
	m.running = true
	m.handleAgent(agentEvent("```go\nx := 1\n"))
	if !m.md.inFence {
		t.Fatal("fence open after the first batch")
	}
	m.handleAgent(agentEvent("y := 2\n```\n"))
	if m.md.inFence {
		t.Fatal("fence closed by the second batch")
	}
}

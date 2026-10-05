package tui

import (
	"strings"
	"testing"
	"time"
)

func TestPasteChipKeepsContent(t *testing.T) {
	in := NewInput()
	in.Insert("see: ")
	big := strings.Repeat("line\n", 120)
	in.Paste(big)
	if in.Text() != "see: "+big {
		t.Fatal("full content sent verbatim")
	}
	if in.Display() != "see: [paste 120 lines #1]" {
		t.Fatalf("%q", in.Display())
	}
	in.ToggleChips()
	if in.Buffer() != "see: "+big {
		t.Fatal("alt+p expands into the buffer")
	}
	in.ToggleChips()
	if in.Display() != "see: [paste 120 lines #1]" {
		t.Fatal("alt+p collapses again")
	}
	small := NewInput()
	small.Paste("a\nb\n")
	if small.Display() != "a\nb\n" {
		t.Fatal("small clean pastes go inline")
	}
}

func TestPasteWithControlBytesIsChipped(t *testing.T) {
	in := NewInput()
	in.Paste("x\x1b[201~y")
	if in.Text() != "x\x1b[201~y" || in.Display() != "[paste 1 line #1]" {
		t.Fatalf("text %q display %q", in.Text(), in.Display())
	}
	in.ToggleChips()
	if in.Display() != "[paste 1 line #1]" {
		t.Fatal("control-byte pastes never expand into the buffer")
	}
}

func TestEditedMarkerSentLiterally(t *testing.T) {
	in := NewInput()
	in.Paste(strings.Repeat("z\n", 60))
	in.SetBuffer(strings.Replace(in.Buffer(), "#1]", "#1", 1)) // user broke the marker
	if strings.Contains(in.Text(), "z\nz") {
		t.Fatal("a broken marker no longer expands")
	}
}

func TestHistory(t *testing.T) {
	in := NewInput()
	in.Insert("one")
	in.Submit()
	in.Insert("two")
	in.Submit()
	in.HistoryPrev()
	if in.Text() != "two" {
		t.Fatal(in.Text())
	}
	in.HistoryPrev()
	if in.Text() != "one" {
		t.Fatal(in.Text())
	}
	in.HistoryNext()
	in.HistoryNext()
	if in.Text() != "" {
		t.Fatal("past the newest → the saved draft (empty)")
	}
	in.Insert("multi\nline")
	if in.HistoryPrev() {
		t.Fatal("multi-line buffer: textarea handles ↑")
	}
}

func TestCtrlC(t *testing.T) {
	in := NewInput()
	now := time.Now()
	in.Insert("draft")
	if in.CtrlC(now) || in.Text() != "" {
		t.Fatal("non-empty: clear")
	}
	if in.CtrlC(now.Add(100 * time.Millisecond)) {
		t.Fatal("first empty ctrl+c arms")
	}
	if !in.CtrlC(now.Add(600 * time.Millisecond)) {
		t.Fatal("second within 1s quits")
	}
	in2 := NewInput()
	in2.CtrlC(now)
	if in2.CtrlC(now.Add(1500 * time.Millisecond)) {
		t.Fatal("outside 1s re-arms")
	}
}

func TestPrependSteering(t *testing.T) {
	in := NewInput()
	in.Insert("draft")
	in.Prepend([]string{"s1", "s2"})
	if in.Text() != "s1\ns2\ndraft" {
		t.Fatal(in.Text())
	}
}

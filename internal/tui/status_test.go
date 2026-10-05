package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
)

var full = StatusInfo{Cwd: "~/projects/moca", Branch: "main", Git: true, Dirty: true, Model: "opencode-go/glm-5.3-flash",
	Effort: llm.EffortMedium, Window: 1_000_000, Used: 230_000, In: 24_000, Out: 6_000, Cost: 0.0412}

func TestRenderStatusFull(t *testing.T) {
	got := RenderStatus(full, 200)
	want := "~/projects/moca · main* · opencode-go/glm-5.3-flash · med · ctx 1M · 23% · 24k/6k · $0.0412"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestRenderStatusVariants(t *testing.T) {
	s := full
	s.Git, s.Branch, s.Sub = false, "", true
	if got := RenderStatus(s, 200); !strings.Contains(got, " · - · ") || !strings.HasSuffix(got, "· sub") {
		t.Fatal(got)
	}
	s = full
	s.Transient = "retry 2/5 · 4s"
	if !strings.HasSuffix(RenderStatus(s, 200), "retry 2/5 · 4s") {
		t.Fatal("transient replaces cost")
	}
}

func TestRenderStatusNarrow(t *testing.T) {
	for _, w := range []int{100, 80, 60, 40, 20} {
		got := RenderStatus(full, w)
		if utf8.RuneCountInString(got) > w || strings.Contains(got, "\n") {
			t.Errorf("width %d: %q (%d)", w, got, utf8.RuneCountInString(got))
		}
	}
	if strings.Contains(RenderStatus(full, 80), "~/projects") {
		t.Fatal("cwd dropped first")
	}
	if !strings.Contains(RenderStatus(full, 60), "23%") {
		t.Fatal("ctx % survives at 60 cols")
	}
}

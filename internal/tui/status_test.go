package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/llm"
)

var full = StatusInfo{Version: "v0.1.0-alpha-1-gc392fac", Cwd: "~/projects/moca", Branch: "main", Git: true, Dirty: true,
	Model: "opencode-go/glm-5.3-flash", Effort: llm.EffortMedium, Window: 1_000_000, Used: 230_000, In: 24_000, Out: 6_000, Cost: 0.0412}

func TestRenderStatusFull(t *testing.T) {
	l1, l2 := RenderStatus(full, 200)
	want1 := "v0.1.0-alpha-1-gc392fac · ~/projects/moca · main*"
	want2 := "opencode-go/glm-5.3-flash · med · ctx 1M · 23% · 24k/6k · $0.0412"
	if l1 != want1 || l2 != want2 {
		t.Fatalf("got  %q / %q\nwant %q / %q", l1, l2, want1, want2)
	}
}

func TestRenderStatusVariants(t *testing.T) {
	s := full
	s.Git, s.Branch, s.Sub = false, "", true
	l1, l2 := RenderStatus(s, 200)
	if !strings.Contains(l1, "· -") || !strings.HasSuffix(l2, "· sub") {
		t.Fatalf("%q / %q", l1, l2)
	}
	s = full
	s.Transient = "retry 2/5 · 4s"
	if _, l2 := RenderStatus(s, 200); !strings.HasSuffix(l2, "retry 2/5 · 4s") {
		t.Fatalf("transient replaces cost: %q", l2)
	}
}

func TestRenderStatusNarrow(t *testing.T) {
	for _, w := range []int{100, 80, 60, 45, 40, 30, 25, 20} {
		l1, l2 := RenderStatus(full, w)
		for _, l := range []string{l1, l2} {
			if lipgloss.Width(l) > w || strings.Contains(l, "\n") {
				t.Errorf("width %d: %q (%d cells)", w, l, lipgloss.Width(l))
			}
		}
	}
	if l1, _ := RenderStatus(full, 45); strings.Contains(l1, "~/projects") {
		t.Fatal("cwd dropped first")
	}
	if _, l2 := RenderStatus(full, 60); !strings.Contains(l2, "23%") {
		t.Fatal("ctx % survives at 60 cols")
	}
}

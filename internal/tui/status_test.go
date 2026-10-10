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

// The context percentage keeps one decimal below 10% so a small conversation
// in a large window does not read as a stuck "0%".
func TestFmtPercent(t *testing.T) {
	for _, c := range []struct {
		used, window int
		want         string
	}{
		{0, 1_000_000, "0%"}, {10, 1_000_000, "<0.1%"}, {4_200, 1_000_000, "0.4%"}, {50_000, 1_000_000, "5.0%"},
		{99_999, 1_000_000, "10%"}, {230_000, 1_000_000, "23%"}, {1_300_000, 1_000_000, "130%"}, {5, 0, "0%"},
	} {
		if got := FmtPercent(c.used, c.window); got != c.want {
			t.Errorf("FmtPercent(%d, %d) = %q, want %q", c.used, c.window, got, c.want)
		}
	}
}

func TestContextPressure(t *testing.T) {
	s := StatusInfo{Window: 100_000, Trigger: 90_000, Model: "m", Cost: 0}
	for used, want := range map[int]int{0: pressureOK, 69_000: pressureOK, 70_000: pressureWarn, 89_999: pressureWarn, 90_000: pressureHot} {
		s.Used = used
		if got := s.pressure(); got != want {
			t.Fatalf("used %d: pressure %d, want %d", used, got, want)
		}
	}
	s.Used = 95_000
	if _, l2 := RenderStatus(s, 120); !strings.Contains(l2, "95% · /compact") {
		t.Fatalf("a hot context says what to do: %q", l2)
	}
	s.Used = 50_000
	if _, l2 := RenderStatus(s, 120); strings.Contains(l2, "/compact") {
		t.Fatalf("no hint below the trigger: %q", l2)
	}
}

func TestStatusPercentColored(t *testing.T) {
	m := newTestModel()
	m.status.Model = "fake/m" // the percent fields only render with a model
	m.status.Window, m.status.Trigger, m.status.Used = 100_000, 90_000, 95_000
	if got := m.statusLine(); !strings.Contains(got, errFg.Bold(true).Render("95%")) {
		t.Fatalf("hot percent not red: %q", got)
	}
	m.status.Used = 75_000
	if got := m.statusLine(); !strings.Contains(got, warnFg.Render("75%")) {
		t.Fatalf("warm percent not yellow: %q", got)
	}
}

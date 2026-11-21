package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

var full = StatusInfo{Version: "v0.1.0-alpha-1-gc392fac", Cwd: "~/projects/moca", Branch: "main", Git: true, Dirty: true,
	Model: "opencode-go/glm-5.3-flash", Effort: llm.EffortMedium, Window: 1_000_000, Used: 230_000, In: 24_000, Out: 6_000, Cost: 0.0412}

func TestRenderStatusFull(t *testing.T) {
	l1, l2 := RenderStatus(full, 200)
	want1 := "v0.1.0-alpha-1-gc392fac · ~/projects/moca · main*"
	want2 := "opencode-go/glm-5.3-flash · med · ctx 1M · 23% · 24k/6k · $0.0412"
	if plainLine(l1) != want1 || plainLine(l2) != want2 {
		t.Fatalf("got  %q / %q\nwant %q / %q", plainLine(l1), plainLine(l2), want1, want2)
	}
}

// A capped working window keeps the model's advertised window in `ctx`, shows
// the cap as its own segment, and measures the percent against the cap.
func TestRenderStatusCappedWindow(t *testing.T) {
	s := full
	s.Effective, s.Trigger = 300_000, 225_000
	_, l2 := RenderStatus(s, 200)
	want := "opencode-go/glm-5.3-flash · med · ctx 1M · cap 300k · 77% · /compact · 24k/6k · $0.0412"
	if got := plainLine(l2); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if got := s.pressure(); got != pressureHot {
		t.Fatalf("pressure at 230k of a 300k cap: %d, want hot", got)
	}
}

// TestStatusBarKeepsTheAdvertisedWindow pins the v0.8.3 regression: the status
// wiring fed the effective (capped) window into the `ctx` field, so a 1M model
// reported "ctx 300k". The bar must report the model's window and name the cap.
func TestStatusBarKeepsTheAdvertisedWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m","context":{"maxWindowTokens":300000},
		"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":1000000}}}}}`, srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(cfgPath, []byte("{\"model\":\"fake/m\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newAgentModelCfg(t, agent.StartOptions{Config: cfg, Workdir: t.TempDir(), HTTP: srv.Client(),
		Slug: "t", Ask: func(_ context.Context, _ tools.Question) tools.Answer { return tools.Deny }}, cfgPath)
	m.refreshStatus()
	if m.status.Window != 1_000_000 || m.status.Effective != 300_000 {
		t.Fatalf("status windows: advertised %d, effective %d — want 1000000, 300000", m.status.Window, m.status.Effective)
	}
	_, l2 := RenderStatus(m.status, 200)
	if line := plainLine(l2); !strings.Contains(line, "ctx 1M") || !strings.Contains(line, "cap 300k") {
		t.Fatalf("the bar must report the model's window and the cap: %q", line)
	}
}

// TestRenderStatusCapOutlivesInOut pins the cap's shrink rank: the cap is the
// percent's divisor, so a narrowing bar sheds in/out (and the /compact hint)
// first — `ctx 1M · 77%` with no cap reads as 77% of the 1M the model does not
// work in.
func TestRenderStatusCapOutlivesInOut(t *testing.T) {
	s := full
	s.Effective, s.Trigger = 300_000, 225_000
	// 87 cells with the cap and the hint; 67 with in/out and the hint gone.
	_, l2 := RenderStatus(s, 70)
	got := plainLine(l2)
	if !strings.Contains(got, "cap 300k") || !strings.Contains(got, "77%") {
		t.Fatalf("the cap must outlive in/out so the percent keeps its divisor: %q", got)
	}
	if strings.Contains(got, "24k/6k") {
		t.Fatalf("in/out should have dropped at 70 cells: %q", got)
	}
}

func TestRenderStatusVariants(t *testing.T) {
	s := full
	s.Git, s.Branch, s.Sub = false, "", true
	l1, l2 := RenderStatus(s, 200)
	if !strings.Contains(plainLine(l1), "· -") || !strings.HasSuffix(plainLine(l2), "· sub") {
		t.Fatalf("%q / %q", plainLine(l1), plainLine(l2))
	}
	s = full
	s.Transient = "retry 2/5 · 4s"
	if _, l2 := RenderStatus(s, 200); !strings.HasSuffix(plainLine(l2), "retry 2/5 · 4s") {
		t.Fatalf("transient replaces cost: %q", plainLine(l2))
	}
}

func TestRenderStatusNarrow(t *testing.T) {
	for _, w := range []int{100, 80, 60, 45, 40, 30, 25, 20} {
		l1, l2 := RenderStatus(full, w)
		for _, l := range [][]segment{l1, l2} {
			plain := plainLine(l)
			if lipgloss.Width(plain) > w || strings.Contains(plain, "\n") {
				t.Errorf("width %d: %q (%d cells)", w, plain, lipgloss.Width(plain))
			}
		}
	}
	if l1, _ := RenderStatus(full, 45); strings.Contains(plainLine(l1), "~/projects") {
		t.Fatal("cwd dropped first")
	}
	if _, l2 := RenderStatus(full, 60); !strings.Contains(plainLine(l2), "23%") {
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
	if _, l2 := RenderStatus(s, 120); !strings.Contains(plainLine(l2), "95% · /compact") {
		t.Fatalf("a hot context says what to do: %q", plainLine(l2))
	}
	s.Used = 50_000
	if _, l2 := RenderStatus(s, 120); strings.Contains(plainLine(l2), "/compact") {
		t.Fatalf("no hint below the trigger: %q", plainLine(l2))
	}
}

func TestStatusPercentColored(t *testing.T) {
	m := newTestModel()
	m.status.Model = "fake/m" // the percent fields only render with a model
	m.status.Window, m.status.Trigger, m.status.Used = 100_000, 90_000, 95_000
	// The hot/warn percent segment is styled with its palette color; the
	// visible text stays exactly what the plain spec says.
	if got := m.statusLine(); !strings.Contains(got, m.th.statusPalette(m.darkBG).hot.Bold(true).Render("95%")) {
		t.Fatalf("hot percent not hot-styled: %q", got)
	}
	m.status.Used = 75_000
	if got := m.statusLine(); !strings.Contains(got, m.th.statusPalette(m.darkBG).warn.Render("75%")) {
		t.Fatalf("warm percent not warm-styled: %q", got)
	}
}

// TestStatusCostUnknownRendersDash: a model whose provider publishes no price
// shows `—`, never $0.0000 (SPECS §6, Review Focus 2).
func TestStatusCostUnknownRendersDash(t *testing.T) {
	s := full
	s.CostUnknown = true
	if _, l2 := RenderStatus(s, 200); !strings.HasSuffix(plainLine(l2), "· —") {
		t.Fatalf("unknown cost must render a dash: %q", plainLine(l2))
	}
	s.Sub = true // a subscription still wins: the cost is not unknown, it is included
	if _, l2 := RenderStatus(s, 200); !strings.HasSuffix(plainLine(l2), "· sub") {
		t.Fatalf("sub wins over the unknown flag: %q", plainLine(l2))
	}
}

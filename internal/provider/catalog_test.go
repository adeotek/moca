// internal/provider/catalog_test.go
package provider

import (
	"math"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

func TestClampEffort(t *testing.T) {
	m := Model{ThinkingMode: "adaptive", ThinkingLevelMap: map[llm.Effort]string{
		llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortMax: "max",
	}}
	cases := map[llm.Effort]llm.Effort{
		llm.EffortOff:     llm.EffortLow, // cannot disable → lowest supported
		llm.EffortMinimal: llm.EffortLow,
		llm.EffortMedium:  llm.EffortMedium,
		llm.EffortXHigh:   llm.EffortHigh, // nearest at or below
		llm.EffortMax:     llm.EffortMax,
	}
	for in, want := range cases {
		if got := m.ClampEffort(in); got != want {
			t.Errorf("Clamp(%s) = %s, want %s", in, got, want)
		}
	}
	none := Model{ThinkingMode: "none"}
	if none.ClampEffort(llm.EffortHigh) != llm.EffortOff {
		t.Fatal("non-thinking model clamps everything to off")
	}
}

func TestDefaultEffort(t *testing.T) {
	m := Model{ThinkingMode: "openai", ThinkingLevelMap: map[llm.Effort]string{llm.EffortLow: "low", llm.EffortHigh: "high"}}
	if m.DefaultEffort() != llm.EffortLow {
		t.Fatal("medium unsupported → clamp(medium)")
	}
}

func TestMaxTokensAndBudget(t *testing.T) {
	m := Model{MaxOutput: 64000, ThinkingMode: "budget"}
	if m.MaxTokens(16384) != 16384 {
		t.Fatal("capped at reserve")
	}
	if (Model{MaxOutput: 8192}).MaxTokens(16384) != 8192 {
		t.Fatal("catalog max wins when smaller")
	}
	if got := m.BudgetTokens(16384, llm.EffortHigh); got != 12288 {
		t.Fatalf("high = 75%% of max_tokens, got %d", got)
	}
	if got := m.BudgetTokens(16384, llm.EffortMax); got != 16384-1024 {
		t.Fatalf("max leaves 1024 tokens of answer room, got %d", got)
	}
	if got := m.BudgetTokens(4096, llm.EffortLow); got != 1024 {
		t.Fatalf("budget floor 1024, got %d", got)
	}
	if got := m.BudgetTokens(2047, llm.EffortMax); got != 0 {
		t.Fatalf("no room for a budget → 0 (thinking off), got %d", got)
	}
}

func TestCostOf(t *testing.T) {
	m := Model{Cost: costM(1.4, 4.4, 0.14, 0)}
	got := m.CostOf(llm.Usage{Input: 1_000_000, Output: 500_000, CacheRead: 2_000_000})
	if math.Abs(got-(1.4+2.2+0.28)) > 1e-9 {
		t.Fatalf("cost %v", got)
	}
}

// wireModel is the row a provider's payload plus the vendor quirk produces for
// an id, without a registry: the catalog-independent stand-in the pins below
// use (a discovered row's own facts are covered by the discovery tests).
func wireModel(providerName, id string) Model {
	m := Model{Provider: providerName, ID: id, Protocol: defaultProtocolFor(providerName),
		ContextWindow: 1_000_000, MaxOutput: 8192, ThinkingMode: "openai"}
	if q, ok := quirkFor(providerName, id); ok {
		m = applyQuirk(m, q)
	}
	return m
}

// TestQuirkAndFallbackTablesSane: the two tables that stand in for a payload
// are small and self-consistent. A quirk names a quirk only; a fallback row is
// complete, because it is shown when no payload could describe it.
func TestQuirkAndFallbackTablesSane(t *testing.T) {
	for id, q := range catalogQuirks {
		if id == "" {
			t.Fatal("empty quirk id")
		}
		if q.Provider != "" {
			if _, ok := config.BuiltinProviders[q.Provider]; !ok {
				t.Fatalf("%s: quirk names unknown provider %q", id, q.Provider)
			}
		}
		for lvl := range q.Levels {
			if lvl.Rank() < 0 {
				t.Fatalf("%s: unknown effort level %q", id, lvl)
			}
		}
		if q.Mode != "" && q.Mode != "openai" && q.Mode != "budget" && q.Mode != "adaptive" {
			t.Fatalf("%s: unknown thinking mode %q", id, q.Mode)
		}
	}
	seen := map[string]bool{}
	for _, m := range fallbackCatalog {
		q := m.Qualified()
		if seen[q] {
			t.Fatalf("duplicate fallback row %s", q)
		}
		seen[q] = true
		if m.ContextWindow < config.MinContextWindow || m.MaxOutput <= 0 || m.Protocol == "" || m.ThinkingMode == "" || !m.CostKnown {
			t.Fatalf("incomplete fallback row %+v", m)
		}
		if m.ReplayReasoning && !m.KeepsTailThinking() {
			t.Fatalf("%s requires reasoning replay but would strip the kept tail", q)
		}
	}
	for _, q := range []string{"opencode-go/glm-5.3-flash", "opencode/glm-5.3-flash"} {
		if !seen[q] {
			t.Fatalf("default-model fallback %s missing", q)
		}
	}
}

// TestGLMEffortMapsMatchTheThinkingOnlyServingVariants pins the 2026-10-16
// live-verified reality (zen go; https://opencode.ai/zen/go/v1): glm-5.3 and
// glm-5.3-flash no longer allow disabling thinking ("none" → `[1210] … cannot
// be disabled`), and the variants accept different levels — flash takes
// low|high|max only, so moca's default medium must clamp to low before
// hitting the wire (it used to be sent as-is and 400'd mid-run). kimi-k3
// still disables thinking through "none". The gateway's own payload
// under-reports all three, which is why the quirks carry them.
func TestGLMEffortMapsMatchTheThinkingOnlyServingVariants(t *testing.T) {
	flash := wireModel("opencode-go", "glm-5.3-flash")
	if _, ok := flash.ThinkingLevelMap[llm.EffortOff]; ok {
		t.Fatal("glm-5.3-flash: off must not be mapped (thinking cannot be disabled)")
	}
	if got := flash.ClampEffort(llm.EffortMedium); got != llm.EffortLow {
		t.Fatalf("glm-5.3-flash: default medium clamps to %q, want low", got)
	}
	if got := flash.ClampEffort(llm.EffortMax); got != llm.EffortMax {
		t.Fatalf("glm-5.3-flash: max clamps to %q", got)
	}
	glm := wireModel("opencode-go", "glm-5.3")
	if _, ok := glm.ThinkingLevelMap[llm.EffortOff]; ok {
		t.Fatal("glm-5.3: off must not be mapped (thinking cannot be disabled)")
	}
	if got := glm.ClampEffort(llm.EffortMedium); got != llm.EffortMedium {
		t.Fatalf("glm-5.3: medium clamps to %q, want medium", got)
	}
	if got := glm.ClampEffort(llm.EffortOff); got != llm.EffortMinimal {
		t.Fatalf("glm-5.3: off clamps to %q, want minimal", got)
	}
	if v := wireModel("opencode-go", "kimi-k3").ThinkingLevelMap[llm.EffortOff]; v != "none" {
		t.Fatalf("kimi-k3: off must map to \"none\", got %q", v)
	}
}

// TestHistoryBoundIsAdaptiveOnly (B1.2): exactly the adaptive thinking routes
// bind the history to their thinking prefix; the payload's thinking mode is
// what decides it, so no model needs an entry for this.
func TestHistoryBoundIsAdaptiveOnly(t *testing.T) {
	adaptive := modelFromCached("anthropic", CachedModel{ID: "claude-new", Protocol: "anthropic-messages", Mode: "adaptive", Ctx: 200000})
	if !adaptive.HistoryBound() {
		t.Fatal("an adaptive row must be history-bound")
	}
	budget := modelFromCached("anthropic", CachedModel{ID: "claude-old", Protocol: "anthropic-messages", Mode: "budget", Ctx: 200000})
	if budget.HistoryBound() {
		t.Fatal("a budget row must not be history-bound")
	}
	// The openai-responses rows ride the same encrypted-reasoning replay as
	// the anthropic ones; the v0.6.4 probe (§15) settled that a dropped
	// reasoning item next to a kept function_call is accepted, so they stay
	// out of the bound set by evidence, not by omission.
	for _, q := range []string{"opencode-go/glm-5.3-flash", "opencode-go/minimax-m3", "opencode-go/gpt-6-luna",
		"opencode-go/grok-4.7", "openai/gpt-6-astra", "openai/gpt-6-luna"} {
		p, id, _ := strings.Cut(q, "/")
		if m := wireModel(p, id); m.HistoryBound() {
			t.Fatalf("%s must not be history-bound", q)
		}
	}
}

// TestKeepsTailThinkingFollowsReasoningReplay: a compaction's kept tail keeps
// its thinking blocks on the routes that need them back — a preserved-thinking
// route binds the prefix, a reasoning-required route refuses a history where
// its own turns lost reasoning_content — and nowhere else (the strip is the
// token saving there).
func TestKeepsTailThinkingFollowsReasoningReplay(t *testing.T) {
	for _, q := range []string{"opencode-go/deepseek-v4.1-flash", "opencode-go/deepseek-v4-pro",
		"opencode-go/deepseek-v4-flash-vision-exp", "opencode/deepseek-v4.1-flash"} {
		p, id, _ := strings.Cut(q, "/")
		m := wireModel(p, id)
		if !m.ReplayReasoning || !m.KeepsTailThinking() {
			t.Fatalf("%s must replay reasoning and keep the kept tail's thinking", q)
		}
	}
	for _, q := range []string{"opencode-go/glm-5.3-flash", "opencode-go/glm-5.3", "opencode-go/kimi-k3"} {
		p, id, _ := strings.Cut(q, "/")
		if wireModel(p, id).KeepsTailThinking() {
			t.Fatalf("%s must strip the kept tail (it needs no reasoning back)", q)
		}
	}
}

// TestAdaptiveMapIncludesXHigh (B1.2): the adaptive map accepts xhigh, so the
// clamp no longer pulls it down to high.
func TestAdaptiveMapIncludesXHigh(t *testing.T) {
	if v := anthropicAdaptive[llm.EffortXHigh]; v != "xhigh" {
		t.Fatalf("anthropicAdaptive[xhigh] = %q, want \"xhigh\"", v)
	}
	m := modelFromCached("anthropic", CachedModel{ID: "claude-opus-new", Protocol: "anthropic-messages",
		Mode: "adaptive", Ctx: 200000, Efforts: []string{"low", "medium", "high", "xhigh", "max"}})
	if got := m.ClampEffort(llm.EffortXHigh); got != llm.EffortXHigh {
		t.Fatalf("xhigh clamps to %q on an adaptive model, want xhigh", got)
	}
	// A payload with no effort list falls back to the curated adaptive set.
	quiet := modelFromCached("anthropic", CachedModel{ID: "claude-quiet", Protocol: "anthropic-messages", Mode: "adaptive", Ctx: 200000})
	if got := quiet.ClampEffort(llm.EffortXHigh); got != llm.EffortXHigh {
		t.Fatalf("the fallback adaptive map must carry xhigh, got %q", got)
	}
}

// internal/provider/catalog_test.go
package provider

import (
	"math"
	"testing"

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

func TestBuiltinCatalogSane(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range builtinCatalog {
		q := m.Qualified()
		if seen[q] {
			t.Fatalf("duplicate %s", q)
		}
		seen[q] = true
		if m.ContextWindow < 16384 || m.MaxOutput <= 0 || m.Protocol == "" || m.ThinkingMode == "" {
			t.Fatalf("incomplete entry %+v", m)
		}
	}
	for _, q := range []string{"opencode-go/glm-5.3", "opencode-go/glm-5.3-flash"} {
		if !seen[q] {
			t.Fatalf("spec-named model %s missing", q)
		}
	}
	for _, q := range []string{
		"opencode-go/deepseek-v4.1-flash", "opencode-go/deepseek-v4-flash",
		"opencode-go/deepseek-v4-pro", "opencode-go/deepseek-v4-flash-vision-exp",
		"opencode-go/kimi-k2.7-code", "opencode-go/mimo-v2.6-pro",
		"opencode-go/mimo-v2.6-flash", "opencode-go/qwen3.8-max",
		"opencode-go/qwen3.8-flash", "opencode-go/qwen3.7-plus",
	} {
		if !seen[q] {
			t.Fatalf("catalog-expansion model %s missing", q)
		}
	}
}

// TestGLMEffortMapsMatchTheThinkingOnlyServingVariants pins the 2026-10-16
// live-verified reality (zen go; https://opencode.ai/zen/go/v1): glm-5.3 and
// glm-5.3-flash no longer allow disabling thinking ("none" → `[1210] … cannot
// be disabled`), and the variants accept different levels — flash takes
// low|high|max only, so moca's default medium must clamp to low before
// hitting the wire (it used to be sent as-is and 400'd mid-run). kimi-k3
// still disables thinking through "none".
func TestGLMEffortMapsMatchTheThinkingOnlyServingVariants(t *testing.T) {
	byID := map[string]Model{}
	for _, m := range builtinCatalog {
		byID[m.Qualified()] = m
	}
	flash := byID["opencode-go/glm-5.3-flash"]
	if _, ok := flash.ThinkingLevelMap[llm.EffortOff]; ok {
		t.Fatal("glm-5.3-flash: off must not be mapped (thinking cannot be disabled)")
	}
	if got := flash.ClampEffort(llm.EffortMedium); got != llm.EffortLow {
		t.Fatalf("glm-5.3-flash: default medium clamps to %q, want low", got)
	}
	if got := flash.ClampEffort(llm.EffortMax); got != llm.EffortMax {
		t.Fatalf("glm-5.3-flash: max clamps to %q", got)
	}
	glm := byID["opencode-go/glm-5.3"]
	if _, ok := glm.ThinkingLevelMap[llm.EffortOff]; ok {
		t.Fatal("glm-5.3: off must not be mapped (thinking cannot be disabled)")
	}
	if got := glm.ClampEffort(llm.EffortMedium); got != llm.EffortMedium {
		t.Fatalf("glm-5.3: medium clamps to %q, want medium", got)
	}
	if got := glm.ClampEffort(llm.EffortOff); got != llm.EffortMinimal {
		t.Fatalf("glm-5.3: off clamps to %q, want minimal", got)
	}
	if v := byID["opencode-go/kimi-k3"].ThinkingLevelMap[llm.EffortOff]; v != "none" {
		t.Fatalf("kimi-k3: off must map to \"none\", got %q", v)
	}
}

// TestHistoryBoundIsAdaptiveOnly (B1.2): exactly the adaptive anthropic rows
// bind the history to their thinking prefix; every other route does not.
func TestHistoryBoundIsAdaptiveOnly(t *testing.T) {
	byID := map[string]Model{}
	for _, m := range builtinCatalog {
		byID[m.Qualified()] = m
	}
	for _, id := range []string{"anthropic/claude-opus-5-5", "anthropic/claude-sonnet-5-5"} {
		if !byID[id].HistoryBound() {
			t.Fatalf("%s must be history-bound (adaptive thinking)", id)
		}
	}
	for _, id := range []string{"opencode-go/glm-5.3-flash", "opencode-go/minimax-m3", "anthropic/claude-haiku-4-5",
		"opencode-go/gpt-6-luna", "opencode-go/grok-4.7", "openai/gpt-6-astra", "openai/gpt-6-luna"} {
		// The openai-responses rows ride the same encrypted-reasoning replay as
		// the anthropic rows above; the v0.6.4 probe (§15) settled that a
		// dropped reasoning item next to a kept function_call is accepted, so
		// they stay out of the bound set by evidence, not by omission.
		if byID[id].Protocol == "" {
			t.Fatalf("%s missing from the catalog", id)
		}
		if byID[id].HistoryBound() {
			t.Fatalf("%s must not be history-bound", id)
		}
	}
}

// TestAdaptiveMapIncludesXHigh (B1.2): the adaptive map accepts xhigh, so the
// clamp no longer pulls it down to high.
func TestAdaptiveMapIncludesXHigh(t *testing.T) {
	if v := anthropicAdaptive[llm.EffortXHigh]; v != "xhigh" {
		t.Fatalf("anthropicAdaptive[xhigh] = %q, want \"xhigh\"", v)
	}
	var m Model
	for _, row := range builtinCatalog {
		if row.Qualified() == "anthropic/claude-opus-5-5" {
			m = row
		}
	}
	if got := m.ClampEffort(llm.EffortXHigh); got != llm.EffortXHigh {
		t.Fatalf("xhigh clamps to %q on an adaptive model, want xhigh", got)
	}
}

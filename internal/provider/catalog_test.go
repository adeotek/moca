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

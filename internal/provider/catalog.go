package provider

import (
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

// Model is one catalog entry. Cost is USD per million tokens.
// ThinkingMode: "none" | "budget" (anthropic budget_tokens) | "adaptive"
// (anthropic thinking:adaptive + output_config.effort) | "openai" (reasoning
// effort). ThinkingLevelMap maps each supported effort to the wire value
// (budget mode: unused — budget comes from MaxTokens); a missing key means
// the level is unsupported. EffortOff present means thinking can be disabled.
type Model struct {
	Provider         string
	ID               string
	Protocol         string
	ContextWindow    int
	MaxOutput        int
	Cost             config.CostConfig
	ThinkingMode     string
	ThinkingLevelMap map[llm.Effort]string
}

func (m Model) Qualified() string { return m.Provider + "/" + m.ID }

func (m Model) ClampEffort(e llm.Effort) llm.Effort {
	if m.ThinkingMode == "none" || len(m.ThinkingLevelMap) == 0 {
		return llm.EffortOff
	}
	best := llm.Effort("")
	for _, lvl := range llm.Efforts {
		if _, ok := m.ThinkingLevelMap[lvl]; !ok {
			continue
		}
		if lvl.Rank() <= e.Rank() {
			best = lvl
		} else if best == "" {
			return lvl // nothing at or below → lowest supported
		}
	}
	return best
}

func (m Model) DefaultEffort() llm.Effort { return m.ClampEffort(llm.EffortMedium) }

func (m Model) MaxTokens(reserve int) int { return min(m.MaxOutput, reserve) }

func (m Model) BudgetTokens(maxTokens int) int { return max(maxTokens-4096, 1024) }

func (m Model) CostOf(u llm.Usage) float64 {
	return (float64(u.Input)*m.Cost.Input + float64(u.Output)*m.Cost.Output +
		float64(u.CacheRead)*m.Cost.CacheRead + float64(u.CacheWrite)*m.Cost.CacheWrite) / 1e6
}

func costM(in, out, cr, cw float64) config.CostConfig {
	return config.CostConfig{Input: in, Output: out, CacheRead: cr, CacheWrite: cw}
}

var (
	anthropicAdaptive = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortMax: "max"}
	anthropicBudget   = map[llm.Effort]string{llm.EffortOff: "", llm.EffortLow: "", llm.EffortMedium: "", llm.EffortHigh: "", llm.EffortMax: ""}
	openaiReasoning   = map[llm.Effort]string{llm.EffortMinimal: "minimal", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
	glmThinking       = map[llm.Effort]string{llm.EffortOff: "", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
)

// builtinCatalog — prices/limits verified 2026-10-04. Sources:
//   - opencode-go: Pi 1.0.1 shipped catalog (pi-ai providers/data/opencode-go.json)
//   - opencode.ai/docs/zen (protocol per model). Cost is standard (non-tiered) rates.
//   - anthropic: docs.anthropic.com models overview + pricing (opus/sonnet 5.5 = adaptive,
//     haiku 4.5 = budget; cacheWrite = 5-minute write rate; cacheRead = hit rate).
//   - openai: platform.openai.com models + developers.openai.com pricing (standard,
//     short-context rates; long-context tiers exist and are not modelled in v1).
var builtinCatalog = []Model{
	// opencode-go — mixed protocol, one row per DESIGN.md §3 model.
	{Provider: "opencode-go", ID: "glm-5.3", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(1.4, 4.4, 0.26, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmThinking},
	{Provider: "opencode-go", ID: "glm-5.3-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(0.15, 0.5, 0.03, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmThinking},
	{Provider: "opencode-go", ID: "minimax-m3", Protocol: "anthropic-messages",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(0.3, 1.2, 0.06, 0),
		ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget},
	{Provider: "opencode-go", ID: "kimi-k3", Protocol: "openai-completions",
		ContextWindow: 1_048_576, MaxOutput: 131072, Cost: costM(3, 15, 0.3, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmThinking},
	{Provider: "opencode-go", ID: "gpt-6-luna", Protocol: "openai-responses",
		ContextWindow: 1_050_000, MaxOutput: 128000, Cost: costM(0.1, 0.5, 0.01, 0.125),
		ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning},
	{Provider: "opencode-go", ID: "grok-4.7", Protocol: "openai-responses",
		ContextWindow: 500_000, MaxOutput: 500_000, Cost: costM(2, 6, 0.5, 0),
		ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning},

	// anthropic — current generation (Opus/Sonnet adaptive, Haiku 4.5 budget).
	{Provider: "anthropic", ID: "claude-opus-5-5", Protocol: "anthropic-messages",
		ContextWindow: 1_000_000, MaxOutput: 128000, Cost: costM(4, 20, 0.2, 5),
		ThinkingMode: "adaptive", ThinkingLevelMap: anthropicAdaptive},
	{Provider: "anthropic", ID: "claude-sonnet-5-5", Protocol: "anthropic-messages",
		ContextWindow: 1_000_000, MaxOutput: 128000, Cost: costM(2, 10, 0.2, 2.5),
		ThinkingMode: "adaptive", ThinkingLevelMap: anthropicAdaptive},
	{Provider: "anthropic", ID: "claude-haiku-4-5", Protocol: "anthropic-messages",
		ContextWindow: 200_000, MaxOutput: 64000, Cost: costM(1, 5, 0.1, 1.25),
		ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget},

	// openai — flagship + efficient tier.
	{Provider: "openai", ID: "gpt-6-astra", Protocol: "openai-responses",
		ContextWindow: 1_050_000, MaxOutput: 128000, Cost: costM(10, 50, 1, 12.5),
		ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning},
	{Provider: "openai", ID: "gpt-6-luna", Protocol: "openai-responses",
		ContextWindow: 1_050_000, MaxOutput: 128000, Cost: costM(0.1, 0.5, 0.01, 0.125),
		ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning},
}

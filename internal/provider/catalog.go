package provider

import (
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

// Model is one catalog entry. Cost is USD per million tokens.
// ThinkingMode: "none" | "budget" (anthropic budget_tokens) | "adaptive"
// (anthropic thinking:adaptive + output_config.effort) | "openai" (the
// completions reasoning_effort field) | "effort-object" (the completions
// reasoning:{effort} object — OpenRouter's canonical parameter, the only one
// that accepts "max"). ThinkingLevelMap maps each supported effort to the wire value
// (budget mode: values unused — the budget comes from MaxTokens and effort); a missing key means
// the level is unsupported. EffortOff present means thinking can be disabled.
type Model struct {
	Provider      string
	ID            string
	Protocol      string
	ContextWindow int
	MaxOutput     int
	Cost          config.CostConfig
	ThinkingMode  string
	// ReplayReasoning echoes a stored thinking block back as the OpenAI
	// chat-completions `reasoning_content` field on the assistant message that
	// produced it. DeepSeek's thinking mode requires it — replaying such a turn
	// without it fails with "The `reasoning_content` in the thinking mode must
	// be passed back to the API" — and is where it came from in the first
	// place. Only blocks the model being asked produced are echoed: another
	// model's reasoning is foreign to the route and is dropped, as everywhere
	// else (§14).
	ReplayReasoning  bool
	ThinkingLevelMap map[llm.Effort]string
	// CostKnown is false for a model discovered without a published price: the
	// UI renders `—` instead of claiming $0.00. Curated and declared rows are
	// known by construction.
	CostKnown bool
	// WindowAssumed marks a ContextWindow that was substituted because the
	// provider published none (16384): the startup warning and the picker both
	// say so, and declaring a window clears it.
	WindowAssumed bool
}

func (m Model) Qualified() string { return m.Provider + "/" + m.ID }

// HistoryBound reports whether the route binds the submitted history to the
// exact prefix its thinking was produced against (preserved/adaptive
// thinking). On such a route a kept-tail thinking strip or an elision — both
// of which rewrite earlier messages — risks the provider dropping the tail's
// thinking or rejecting the request, so both are disabled (B1.2, §7/§14).
//
// Today that is exactly the adaptive anthropic rows. The openai-responses
// encrypted-reasoning route was the recorded open question: it replays
// reasoning items next to their function calls, so a dropped item looked like
// a rejection risk. Probed live on the zen go route 2026-10-18 — a request
// that keeps a function_call but omits its reasoning item is accepted (200;
// control with the reasoning item replayed: 200; control dropping the tool
// output instead: 400) — so the strip and elision stay enabled there and this
// predicate stays adaptive-only. §15 records the probe; the api.openai.com
// rows ride the same adapter and are covered by the no-vendor-keys item.
func (m Model) HistoryBound() bool { return m.ThinkingMode == "adaptive" }

// KeepsTailThinking reports whether a compaction's kept tail must replay its
// thinking blocks instead of being stripped (§14, B1.2). Two kinds of route
// need it, for different reasons: a preserved-thinking route binds the request
// to the prefix its reasoning was produced against (HistoryBound), and a route
// that requires its reasoning echoed back on the wire (ReplayReasoning —
// DeepSeek's thinking mode: "the intermediate assistant's reasoning_content
// must be passed back ... in all subsequent user interaction turns") rejects a
// history whose own tool-call turns lost the field. 400 either way, and the
// strip is a rewrite of exactly that history.
func (m Model) KeepsTailThinking() bool { return m.HistoryBound() || m.ReplayReasoning }

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

// BudgetTokens is the anthropic thinking budget for an effort level: a share of
// max_tokens, floored at the API minimum and capped so 1024 tokens stay free
// for the answer (budget_tokens must be below max_tokens). 0 means max_tokens
// is too small to think at all.
func (m Model) BudgetTokens(maxTokens int, e llm.Effort) int {
	const floor = 1024
	if maxTokens < 2*floor {
		return 0
	}
	pct := 100
	switch e {
	case llm.EffortMinimal, llm.EffortLow:
		pct = 25
	case llm.EffortMedium:
		pct = 50
	case llm.EffortHigh:
		pct = 75
	}
	return min(max(maxTokens*pct/100, floor), maxTokens-floor)
}

func (m Model) CostOf(u llm.Usage) float64 {
	return (float64(u.Input)*m.Cost.Input + float64(u.Output)*m.Cost.Output +
		float64(u.CacheRead)*m.Cost.CacheRead + float64(u.CacheWrite)*m.Cost.CacheWrite) / 1e6
}

func costM(in, out, cr, cw float64) config.CostConfig {
	return config.CostConfig{Input: in, Output: out, CacheRead: cr, CacheWrite: cw}
}

var (
	anthropicAdaptive = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortXHigh: "xhigh", llm.EffortMax: "max"}
	anthropicBudget   = map[llm.Effort]string{llm.EffortOff: "", llm.EffortLow: "", llm.EffortMedium: "", llm.EffortHigh: "", llm.EffortMax: ""}
	openaiReasoning   = map[llm.Effort]string{llm.EffortMinimal: "minimal", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
	// glm-5.3 and glm-5.3-flash diverged on 2026-10-16 (live-verified against
	// https://opencode.ai/zen/go/v1): both are thinking-only now — sending
	// "none" is `[1210] … cannot be disabled; please use low, high, or max` —
	// but the serving variants accept different levels. Off is absent from
	// both (no disabling); ClampEffort lands the default medium on the
	// nearest supported level (flash: low).
	glmThinking      = map[llm.Effort]string{llm.EffortMinimal: "minimal", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortMax: "max"}
	glmFlashThinking = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortHigh: "high", llm.EffortMax: "max"}
	// kimi-k3 still disables thinking through "none" (live-verified
	// 2026-10-16) and accepts every level above it; "off" is an invalid wire
	// value there ("off" is moca's label, "none" the wire one).
	kimiThinking        = map[llm.Effort]string{llm.EffortOff: "none", llm.EffortMinimal: "minimal", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortMax: "max"}
	deepseekThinking    = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortHigh: "high", llm.EffortMax: "max"}
	deepseekProThinking = map[llm.Effort]string{llm.EffortHigh: "high", llm.EffortMax: "max"}
	qwenThinking        = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortXHigh: "xhigh"}
	basicThinking       = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
)

// builtinCatalog — prices/limits verified 2026-10-04 (phase 1), 2026-10-07
// (opencode-go expansion rows) and 2026-10-16 (glm-5.3 / glm-5.3-flash /
// kimi-k3 effort acceptance re-verified live — the glm serving variants
// became thinking-only). Sources:
//   - opencode-go: Pi 1.0.1 shipped catalog (pi-ai providers/data/opencode-go.json)
//   - opencode.ai/docs/zen and /docs/go (protocol per model, go-tier endpoint
//     table). Cost is standard (non-tiered) rates; the deepseek rows use the
//     off-peak rate (peak hours are 2x — not modelled), qwen3.7-plus the <=256K tier.
//   - anthropic: docs.anthropic.com models overview + pricing (opus/sonnet 5.5 = adaptive,
//     haiku 4.5 = budget; cacheWrite = 5-minute write rate; cacheRead = hit rate).
//   - openai: platform.openai.com models + developers.openai.com pricing (standard,
//     short-context rates; long-context tiers exist and are not modelled in v1).
var builtinCatalog = []Model{
	// opencode-go — mixed protocol: the SPECS §6 original models, plus the 2026-10-07
	// expansion (deepseek family, kimi-k2.7-code, mimo-v2.6-pro/flash,
	// qwen3.8-max/flash, qwen3.7-plus). Routes and effort values live-verified
	// 2026-10-07 — incl. qwen3.8-max on openai-completions (the /docs/go endpoint
	// table lists /messages) and kimi-k2.7-code/mimo-v2.6-* rejecting off/minimal.
	{Provider: "opencode-go", ID: "glm-5.3", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(1.4, 4.4, 0.26, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmThinking},
	{Provider: "opencode-go", ID: "glm-5.3-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(0.15, 0.5, 0.03, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmFlashThinking},
	{Provider: "opencode-go", ID: "minimax-m3", Protocol: "anthropic-messages",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(0.3, 1.2, 0.06, 0),
		ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget},
	{Provider: "opencode-go", ID: "kimi-k3", Protocol: "openai-completions",
		ContextWindow: 1_048_576, MaxOutput: 131072, Cost: costM(3, 15, 0.3, 0),
		ThinkingMode: "openai", ThinkingLevelMap: kimiThinking},
	{Provider: "opencode-go", ID: "gpt-6-luna", Protocol: "openai-responses",
		ContextWindow: 1_050_000, MaxOutput: 128000, Cost: costM(0.1, 0.5, 0.01, 0.125),
		ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning},
	{Provider: "opencode-go", ID: "grok-4.7", Protocol: "openai-responses",
		ContextWindow: 500_000, MaxOutput: 500_000, Cost: costM(2, 6, 0.5, 0),
		ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning},

	{Provider: "opencode-go", ID: "deepseek-v4.1-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 384000, Cost: costM(0.15, 0.6, 0.003, 0),
		ThinkingMode: "openai", ReplayReasoning: true, ThinkingLevelMap: deepseekThinking},
	{Provider: "opencode-go", ID: "deepseek-v4-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 384000, Cost: costM(0.15, 0.6, 0.003, 0),
		ThinkingMode: "openai", ReplayReasoning: true, ThinkingLevelMap: deepseekThinking},
	{Provider: "opencode-go", ID: "deepseek-v4-pro", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 384000, Cost: costM(0.66, 1.98, 0.022, 0),
		ThinkingMode: "openai", ReplayReasoning: true, ThinkingLevelMap: deepseekProThinking},
	{Provider: "opencode-go", ID: "deepseek-v4-flash-vision-exp", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 384000, Cost: costM(0.15, 0.6, 0.003, 0),
		ThinkingMode: "openai", ReplayReasoning: true, ThinkingLevelMap: deepseekThinking},
	{Provider: "opencode-go", ID: "kimi-k2.7-code", Protocol: "openai-completions",
		ContextWindow: 262144, MaxOutput: 262144, Cost: costM(0.95, 4, 0.19, 0),
		ThinkingMode: "openai", ThinkingLevelMap: basicThinking},
	{Provider: "opencode-go", ID: "mimo-v2.6-pro", Protocol: "openai-completions",
		ContextWindow: 1_048_576, MaxOutput: 131072, Cost: costM(0.435, 0.87, 0.003625, 0),
		ThinkingMode: "openai", ThinkingLevelMap: basicThinking},
	{Provider: "opencode-go", ID: "mimo-v2.6-flash", Protocol: "openai-completions",
		ContextWindow: 1_048_576, MaxOutput: 131072, Cost: costM(0.14, 0.28, 0.0028, 0),
		ThinkingMode: "openai", ThinkingLevelMap: basicThinking},
	{Provider: "opencode-go", ID: "qwen3.8-max", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(2, 6, 0.25, 2.5),
		ThinkingMode: "openai", ThinkingLevelMap: qwenThinking},
	{Provider: "opencode-go", ID: "qwen3.8-flash", Protocol: "anthropic-messages",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(0.15, 0.47, 0.016, 0.2),
		ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget},
	{Provider: "opencode-go", ID: "qwen3.7-plus", Protocol: "anthropic-messages",
		ContextWindow: 1_000_000, MaxOutput: 65536, Cost: costM(0.4, 1.6, 0.04, 0.5),
		ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget},

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

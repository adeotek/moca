package provider

import (
	"strings"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

// Model is one registry entry. Cost is USD per million tokens.
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

// costQ is costM for the tables below, whose Cost is optional (a nil means
// "the provider publishes its own price, leave it alone").
func costQ(in, out, cr, cw float64) *config.CostConfig {
	c := costM(in, out, cr, cw)
	return &c
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
	kimiThinking     = map[llm.Effort]string{llm.EffortOff: "none", llm.EffortMinimal: "minimal", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortMax: "max"}
	deepseekThinking = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortHigh: "high", llm.EffortMax: "max"}
	basicThinking    = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
)

// quirk corrects what a provider's own model payload says: a fact the wire
// states wrongly, understates, or cannot state at all. A zero field leaves the
// discovered value alone, so a quirk never repeats metadata the provider
// publishes.
type quirk struct {
	// Provider restricts the quirk to one provider (a metadata gap on one
	// gateway); empty applies to any provider that lists the id.
	Provider  string
	Protocol  string
	Window    int
	MaxOutput int
	Cost      *config.CostConfig
	Mode      string
	Levels    map[llm.Effort]string
	Replay    bool
}

func (q quirk) applies(providerName string) bool {
	return q.Provider == "" || q.Provider == providerName
}

// empty reports whether the quirk says nothing (it holds a map, so == does
// not compile).
func (q quirk) empty() bool {
	return q.Protocol == "" && q.Window == 0 && q.MaxOutput == 0 && q.Cost == nil &&
		q.Mode == "" && len(q.Levels) == 0 && !q.Replay
}

// catalogQuirks is what discovery cannot get right on its own. The model list
// itself — which ids exist, their windows, output caps, prices and effort
// levels — comes from each provider's /models payload (SPECS §6); this table
// carries only the exceptions, so a model a provider ships tomorrow works
// without a moca release.
//
// Sources: glm/kimi effort sets and the DeepSeek replay rule were live-verified
// against https://opencode.ai/zen/go/v1 (2026-10-07, 2026-10-16); the anthropic
// prices come from docs.anthropic.com/pricing (its API publishes windows and
// effort levels, not prices); the openai rows are the whole metadata because
// api.openai.com/models carries nothing but ids and shutdown dates.
var catalogQuirks = map[string]quirk{
	// The payload under-reports these effort sets: glm-5.3 lists no "medium",
	// kimi-k3 only "max", and mimo/kimi-k2.7 list none at all.
	"glm-5.3":         {Mode: "openai", Levels: glmThinking},
	"glm-5.3-flash":   {Mode: "openai", Levels: glmFlashThinking},
	"kimi-k3":         {Mode: "openai", Levels: kimiThinking, MaxOutput: 131072},
	"kimi-k2.7-code":  {Mode: "openai", Levels: basicThinking},
	"mimo-v2.6-pro":   {Mode: "openai", Levels: basicThinking},
	"mimo-v2.6-flash": {Mode: "openai", Levels: basicThinking},
	// Served on both wires; chat completions is the verified route
	// (2026-10-07) and the one its effort list belongs to.
	"qwen3.8-max": {Protocol: "openai-completions"},

	// anthropic: prices only — adaptive-vs-budget arrives in its thinking
	// capabilities, windows and effort levels in the model payload.
	"claude-opus-5-5":   {Provider: "anthropic", Cost: costQ(4, 20, 0.2, 5)},
	"claude-sonnet-5-5": {Provider: "anthropic", Cost: costQ(2, 10, 0.2, 2.5)},
	"claude-haiku-4-5":  {Provider: "anthropic", Cost: costQ(1, 5, 0.1, 1.25)},

	// openai: /models publishes an id and a shutdown date, nothing else.
	"gpt-6-astra": {Provider: "openai", Window: 1_050_000, MaxOutput: 128_000,
		Cost: costQ(10, 50, 1, 12.5), Mode: "openai", Levels: openaiReasoning},
	"gpt-6-luna": {Provider: "openai", Window: 1_050_000, MaxOutput: 128_000,
		Cost: costQ(0.1, 0.5, 0.01, 0.125), Mode: "openai", Levels: openaiReasoning},
}

// familyQuirk covers a vendor whose wire needs something no payload states:
// DeepSeek's thinking mode requires the assistant's reasoning_content to be
// passed back on every later turn (moca replays it), on either gateway.
func familyQuirk(id string) quirk {
	if strings.HasPrefix(id, "deepseek-") {
		return quirk{Replay: true}
	}
	return quirk{}
}

// quirkFor merges the family rule with the id's own entry.
func quirkFor(providerName, id string) (quirk, bool) {
	q := familyQuirk(id)
	own, ok := catalogQuirks[id]
	if !ok || !own.applies(providerName) {
		return q, !q.empty()
	}
	if own.Protocol != "" {
		q.Protocol = own.Protocol
	}
	if own.Window != 0 {
		q.Window = own.Window
	}
	if own.MaxOutput != 0 {
		q.MaxOutput = own.MaxOutput
	}
	if own.Cost != nil {
		q.Cost = own.Cost
	}
	if own.Mode != "" {
		q.Mode = own.Mode
	}
	if len(own.Levels) > 0 {
		q.Levels = own.Levels
	}
	q.Replay = q.Replay || own.Replay
	return q, true
}

// applyQuirk overlays a quirk on a discovered row. It never invents a row: a
// quirk for an id the provider does not list is dead weight, not a model.
func applyQuirk(m Model, q quirk) Model {
	if q.Protocol != "" {
		m.Protocol = q.Protocol
	}
	if q.Window != 0 {
		m.ContextWindow, m.WindowAssumed = q.Window, false
	}
	if q.MaxOutput != 0 {
		m.MaxOutput = q.MaxOutput
	}
	if q.Cost != nil {
		m.Cost, m.CostKnown = *q.Cost, true
	}
	if q.Mode != "" {
		m.ThinkingMode = q.Mode
	}
	if len(q.Levels) > 0 {
		m.ThinkingLevelMap = q.Levels
	}
	if q.Replay {
		m.ReplayReasoning = true
	}
	return m
}

// fallbackCatalog is the last-resort model list, used ONLY for a provider
// whose model list was never read (first run with no credential, a failed
// fetch with no cache). It is deliberately tiny and never grows with a
// vendor's catalogue: with discovery working, a model is a payload row, not a
// table entry. Every row is complete, because it stands in for the missing
// payload. Prices are the same ones the gateways publish (the zen twin bills
// deepseek-v4.1-flash at twice the go tier's rate).
var fallbackCatalog = []Model{
	{Provider: "opencode-go", ID: "glm-5.3-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(0.15, 0.5, 0.03, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmFlashThinking, CostKnown: true},
	{Provider: "opencode-go", ID: "glm-5.3", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(1.4, 4.4, 0.26, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmThinking, CostKnown: true},
	{Provider: "opencode-go", ID: "deepseek-v4.1-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 384000, Cost: costM(0.15, 0.6, 0.003, 0),
		ThinkingMode: "openai", ReplayReasoning: true, ThinkingLevelMap: deepseekThinking, CostKnown: true},
	{Provider: "opencode-go", ID: "kimi-k3", Protocol: "openai-completions",
		ContextWindow: 1_048_576, MaxOutput: 131072, Cost: costM(3, 15, 0.3, 0),
		ThinkingMode: "openai", ThinkingLevelMap: kimiThinking, CostKnown: true},

	{Provider: "opencode", ID: "glm-5.3-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(0.15, 0.5, 0.03, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmFlashThinking, CostKnown: true},
	{Provider: "opencode", ID: "deepseek-v4.1-flash", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 384000, Cost: costM(0.3, 1.2, 0.006, 0),
		ThinkingMode: "openai", ReplayReasoning: true, ThinkingLevelMap: deepseekThinking, CostKnown: true},

	{Provider: "anthropic", ID: "claude-sonnet-5-5", Protocol: "anthropic-messages",
		ContextWindow: 1_000_000, MaxOutput: 128000, Cost: costM(2, 10, 0.2, 2.5),
		ThinkingMode: "adaptive", ThinkingLevelMap: anthropicAdaptive, CostKnown: true},

	// The fallback for a fetch that never succeeded. The window is what a
	// subscription account reports for this slug (`context_window`, measured
	// 2026-11-21 — the curated 1.05M was 4x the real one, which would have
	// overflowed the route instead of compacting). No row is offered for a
	// model the account may not have: discovery replaces this list as soon as
	// /models answers.
	{Provider: "openai", ID: "gpt-6-luna", Protocol: "openai-responses",
		ContextWindow: 272_000, MaxOutput: 128000, Cost: costM(0.1, 0.5, 0.01, 0.125),
		ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning, CostKnown: true},
}

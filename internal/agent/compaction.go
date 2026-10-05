package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
)

// Compacted reports a completed compaction (§6): the context estimate before
// and after, for the status line and the TUI's scrollback note.
type Compacted struct{ TokensBefore, TokensAfter int }

func (Compacted) isEvent() {}

var (
	ErrNothingToCompact = errors.New("nothing to compact")
	errOverflow         = errors.New("context overflow")
)

// Budget is the current model's token budget (§6).
func (a *Agent) Budget() compact.Budget {
	c := a.opts.Config.Context
	return compact.NewBudget(a.model.ContextWindow, c.ReserveTokens, c.KeepRecentTokens)
}

// liveEntries returns the transcript entries after the latest compaction's
// first kept entry, as compact entries, plus the previous compaction state.
func (a *Agent) liveEntries() ([]compact.Entry, compact.Prev) {
	a.mu.Lock()
	entries := append([]session.Entry(nil), a.entries...)
	a.mu.Unlock()
	var prev compact.Prev
	start := 0
	if c, _ := session.LatestCompaction(entries); c != nil {
		prev = compact.Prev{Summary: c.Summary, Read: c.ReadFiles, Modified: c.ModifiedFiles}
		for i, e := range entries {
			if e.ID == c.FirstKeptEntryID {
				start = i
				break
			}
		}
	}
	var out []compact.Entry
	for _, e := range entries[start:] {
		switch e.Type {
		case session.TypeMessage:
			k := compact.KindUser
			if e.Message.Role == llm.RoleAssistant {
				k = compact.KindAssistant
			}
			out = append(out, compact.Entry{ID: e.ID, Kind: k, Msg: e.Message})
		case session.TypeToolUse:
			c := e.ToolUse.Call
			out = append(out, compact.Entry{ID: e.ID, Kind: compact.KindToolUse, Call: &c})
		case session.TypeToolResult:
			out = append(out, compact.Entry{ID: e.ID, Kind: compact.KindToolResult, Result: e.ToolResult})
		}
	}
	return out, prev
}

// summarizer returns the summary function over the cheap model (config
// `model`, not the /hard model), the input cap in chars, and the model (for
// cost accounting). The summary request carries no tools, minimal effort and
// no cache writes — a one-off prompt (§6).
func (a *Agent) summarizer() (compact.Summarizer, int, provider.Model, error) {
	m, ad, err := a.opts.Providers.Resolve(a.opts.Config.Model)
	if err != nil {
		return nil, 0, provider.Model{}, err
	}
	maxTok := min(4096, m.MaxOutput)
	capChars := max(0, (m.ContextWindow-maxTok-len(compact.SummarySystem)/4-512)*4)
	fn := func(ctx context.Context, system, user string) (string, llm.Usage, error) {
		resp, err := ad.Stream(ctx, llm.Request{Model: m.ID, System: system, MaxTokens: maxTok,
			Effort: m.ClampEffort(llm.EffortMinimal), NoCacheWrite: true,
			Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: user}}}}},
			func(llm.Event) {})
		if err != nil {
			return "", resp.Usage, err
		}
		return textOf(resp.Message), resp.Usage, nil
	}
	return fn, capChars, m, nil
}

// Compact is the manual (/compact) and automatic (threshold, overflow) path:
// it summarizes the live window's older history into a compaction entry and
// returns ErrNothingToCompact when there is nothing older than keepRecent.
func (a *Agent) Compact(ctx context.Context) error {
	live, prev := a.liveEntries()
	sum, capChars, cheap, err := a.summarizer()
	if err != nil {
		return err
	}
	before := a.ContextTokens()
	r, ok, err := compact.Compact(ctx, live, prev, a.Budget().KeepRecent, capChars, sum)
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}
	if !ok {
		return ErrNothingToCompact
	}
	u := r.Usage
	cost := cheap.CostOf(u)
	if _, err := a.append(session.Entry{Type: session.TypeCompaction, Usage: &u, Cost: cost, Compaction: &session.Compaction{
		Summary: r.Summary, FirstKeptEntryID: r.FirstKept, TokensBefore: before, Usage: u,
		ReadFiles: r.Read, ModifiedFiles: r.Modified}}); err != nil {
		return err
	}
	// The summary's own usage counts towards session totals (the entry above
	// restores them on resume). The estimate anchor is dropped: the rebuilt
	// request differs from everything the anchor measured.
	a.mu.Lock()
	a.usage, a.cost = a.usage.Add(u), a.cost+cost
	a.anchorValid = false
	a.mu.Unlock()
	a.emit(Compacted{TokensBefore: before, TokensAfter: a.ContextTokens()})
	return nil
}

// maybeCompact compacts when the estimate crosses the trigger and enforces
// the loop guard: if the context is still over budget afterwards, the run
// stops with an error naming the largest entries instead of compacting on.
func (a *Agent) maybeCompact(ctx context.Context) error {
	b := a.Budget()
	if !b.Over(a.ContextTokens()) {
		return nil
	}
	if err := a.Compact(ctx); err != nil && !errors.Is(err, ErrNothingToCompact) {
		return err
	}
	if now := a.ContextTokens(); b.Over(now) {
		return fmt.Errorf("context still over budget after compaction (%d > %d tokens); largest entries: %s",
			now, b.Trigger(), a.largestEntries(3))
	}
	return nil
}

// largestEntries describes the n biggest live entries for the loop-guard
// error: "tool_result for read(path=…) ≈ 30k tokens".
func (a *Agent) largestEntries(n int) string {
	live, _ := a.liveEntries()
	calls := map[string]string{}
	for _, e := range live {
		if e.Kind == compact.KindToolUse {
			calls[e.Call.ID] = fmt.Sprintf("%s(%s)", e.Call.Name,
				strings.TrimSuffix(strings.TrimPrefix(string(e.Call.Input), "{"), "}"))
		}
	}
	slices.SortFunc(live, func(x, y compact.Entry) int { return compact.EntryTokens(y) - compact.EntryTokens(x) })
	var parts []string
	for _, e := range live[:min(n, len(live))] {
		label := map[compact.Kind]string{compact.KindUser: "user message", compact.KindAssistant: "assistant message",
			compact.KindToolUse: "tool call", compact.KindToolResult: "tool_result"}[e.Kind]
		if e.Kind == compact.KindToolResult {
			label += " for " + calls[e.Result.CallID]
		}
		parts = append(parts, fmt.Sprintf("%s ≈ %dk tokens", label, compact.EntryTokens(e)/1000))
	}
	return strings.Join(parts, "; ")
}

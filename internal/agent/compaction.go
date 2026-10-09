package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/config"
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
	// errSummarizerConfig marks a summarizer that cannot be used at all
	// (unknown cheap model): auto-compaction skips those instead of killing
	// a run the active model handles fine.
	errSummarizerConfig = errors.New("summarizer unavailable")
)

// summarizerUnavailable reports whether a compaction failure is a
// configuration problem — the cheap model or its credential is unusable —
// rather than a stream failure. Auto-compaction skips these; only the
// overflow path (which must compact or fail) surfaces them.
func summarizerUnavailable(err error) bool {
	var ee *config.EnvError
	return errors.As(err, &ee) || errors.Is(err, errSummarizerConfig)
}

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
	if c, idx := session.LatestCompaction(entries); c != nil {
		prev = compact.Prev{Summary: c.Summary, Read: c.ReadFiles, Modified: c.ModifiedFiles}
		found := false
		for i, e := range entries {
			if e.ID == c.FirstKeptEntryID {
				start, found = i, true
				break
			}
		}
		if !found {
			// The kept boundary is gone (trimmed file): resume from the
			// compaction entry itself, as session.Messages does, instead of
			// re-serializing already-summarized history.
			start = idx
		}
	}
	var out []compact.Entry
	elided := session.ElidedIDs(entries)
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
			r := e.ToolResult
			if elided[e.ID] {
				stub := *r
				stub.Content = session.ElidedStub
				r = &stub
			}
			out = append(out, compact.Entry{ID: e.ID, Kind: compact.KindToolResult, Result: r})
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
		return nil, 0, provider.Model{}, fmt.Errorf("%w: %v", errSummarizerConfig, err)
	}
	maxTok := min(4096, m.MaxOutput)
	// 3 chars per token, not the estimator's 4: code, JSON and non-Latin text
	// run denser, and the summary request must fit the window.
	capChars := max(0, (m.ContextWindow-maxTok-len(compact.SummarySystem)/4-512)*3)
	stream := func(ctx context.Context, system, user string) (llm.Response, error) {
		return ad.Stream(ctx, llm.Request{Model: m.ID, System: system, MaxTokens: maxTok,
			Effort: m.ClampEffort(llm.EffortMinimal), NoCacheWrite: true,
			Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: user}}}}},
			func(llm.Event) {})
	}
	fn := func(ctx context.Context, system, user string) (string, llm.Usage, error) {
		resp, err := stream(ctx, system, user)
		if errors.Is(err, provider.ErrContextOverflow) && len(user) > 1 {
			// The cap is only an estimate: if the summarizer still overflows,
			// keep the newer half of the payload and try once more.
			resp, err = stream(ctx, system, compact.CapChars(user, len(user)/2))
		}
		if err != nil {
			return "", resp.Usage, err
		}
		return llm.TextOf(resp.Message), resp.Usage, nil
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
		a.recordFailedCompaction(r.Usage, cheap, err)
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
	// The cache is reset anyway: stub every stale result the kept window
	// still carries.
	if err := a.elide(true); err != nil {
		return err
	}
	a.emit(Compacted{TokensBefore: before, TokensAfter: a.ContextTokens()})
	return nil
}

// recordFailedCompaction books the tokens a failed summary spent (e.g. the
// first of two split-turn calls): they were billed, so totals and the session
// file (which restores totals on resume) must show them.
func (a *Agent) recordFailedCompaction(u llm.Usage, cheap provider.Model, cause error) {
	if u == (llm.Usage{}) {
		return
	}
	cost := cheap.CostOf(u)
	if _, err := a.append(session.Entry{Type: session.TypeError, Usage: &u, Cost: cost,
		Error: &session.ErrorInfo{Message: "compaction failed: " + cause.Error()}}); err != nil {
		return
	}
	a.mu.Lock()
	a.usage, a.cost = a.usage.Add(u), a.cost+cost
	a.mu.Unlock()
}

// maybeCompact compacts when the estimate crosses the trigger and enforces
// the loop guard: if the context is still over budget afterwards, the run
// stops with an error naming the largest entries instead of compacting on.
// A summarizer that cannot run at all (missing cheap-model credential,
// unknown cheap model) only skips the automatic checkpoint — the active
// model may handle the context fine, and a real provider overflow still
// reports the problem through the compact-and-retry path.
func (a *Agent) maybeCompact(ctx context.Context) error {
	if err := a.elide(false); err != nil {
		return err
	}
	b := a.Budget()
	if !b.Over(a.ContextTokens()) {
		return nil
	}
	err := a.Compact(ctx)
	switch {
	case err == nil, errors.Is(err, ErrNothingToCompact):
		// Fall through to the loop guard.
	case summarizerUnavailable(err):
		a.emit(Warning{Text: "auto-compaction skipped: " + err.Error()})
		return nil
	default:
		return err
	}
	if now := a.ContextTokens(); b.Over(now) {
		return fmt.Errorf("context still over budget after compaction (%d > %d tokens); largest entries: %s%s",
			now, b.Trigger(), a.largestEntries(3), a.fixedOverheadNote(now))
	}
	return nil
}

// fixedOverheadNote names the system-prompt + tool-schema overhead when it
// dominates the estimate: largestEntries then points at near-empty transcript
// entries, which would misattribute the overage to the history.
func (a *Agent) fixedOverheadNote(estimate int) string {
	live, _ := a.liveEntries()
	n := 0
	for _, e := range live {
		n += compact.EntryTokens(e)
	}
	if ovh := estimate - n; ovh > 0 && ovh >= estimate/4 {
		return fmt.Sprintf("; fixed overhead (system prompt + tools) ≈ %dk tokens", ovh/1000)
	}
	return ""
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

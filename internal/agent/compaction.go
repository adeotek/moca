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
	"github.com/adeotek/moca/internal/tools"
)

// Compacted reports a completed compaction (§6): the context estimate before
// and after, for the status line and the TUI's scrollback note.
type Compacted struct{ TokensBefore, TokensAfter int }

func (Compacted) isEvent() {}

var (
	ErrNothingToCompact = errors.New("nothing to compact")
	errOverflow         = errors.New("context overflow")
	errContextCannotFit = errors.New("context cannot fit")
	// errSummarizerConfig marks a summarizer that cannot be used at all
	// (unknown cheap model): auto-compaction skips those instead of killing
	// a run the active model handles fine.
	errSummarizerConfig = errors.New("summarizer unavailable")
)

// minUsefulOutput is the smallest output budget a fitted request may carry;
// below it the plan returns contextCannotFit instead of a truncated answer.
const minUsefulOutput = 256

// summarizerUnavailable reports whether a compaction failure is a
// configuration problem — the cheap model or its credential is unusable —
// rather than a stream failure. Auto-compaction skips these; only the
// overflow path (which must compact or fail) surfaces them.
func summarizerUnavailable(err error) bool {
	var ee *config.EnvError
	return errors.As(err, &ee) || errors.Is(err, errSummarizerConfig)
}

// Budget is the current model's token budget (§14.1).
func (a *Agent) Budget() compact.Budget {
	c := a.opts.Config.Context
	return compact.NewBudget(a.model.ContextWindow, c.MaxWindow(), c.CompactPercent(), c.ReserveTokens, c.KeepRecentTokens)
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
	stubs := session.ElidedStubs(entries)
	trims := session.Trims(entries)
	for _, e := range entries[start:] {
		switch e.Type {
		case session.TypeMessage:
			k := compact.KindUser
			if e.Message.Role == llm.RoleAssistant {
				k = compact.KindAssistant
			}
			msg := e.Message
			if k == compact.KindUser {
				if t, ok := trims[e.ID]; ok {
					m := *e.Message
					m.Content = make([]llm.ContentBlock, 0, len(e.Message.Content))
					for _, c := range e.Message.Content {
						if c.Type == llm.BlockText {
							c.Text = session.ApplyTrim(c.Text, t)
						}
						m.Content = append(m.Content, c)
					}
					msg = &m
				}
			}
			out = append(out, compact.Entry{ID: e.ID, Kind: k, Msg: msg})
		case session.TypeToolUse:
			c := e.ToolUse.Call
			out = append(out, compact.Entry{ID: e.ID, Kind: compact.KindToolUse, Call: &c})
		case session.TypeToolResult:
			r := e.ToolResult
			if elided[e.ID] {
				stub := *r
				stub.Content = session.ElidedStub
				if s, ok := stubs[e.ID]; ok {
					stub.Content = s
				}
				r = &stub
			} else if t, ok := trims[e.ID]; ok {
				trimmed := *r
				trimmed.Content = session.ApplyTrim(r.Content, t)
				r = &trimmed
			}
			out = append(out, compact.Entry{ID: e.ID, Kind: compact.KindToolResult, Result: r})
		}
	}
	return out, prev
}

// summarizer returns the summary function over the configured
// context.summaryModel, else the configured `model` (not the /hard model),
// falling back to the active model when that choice is unusable. The summary
// request carries no tools, minimal effort and no cache writes — a one-off
// prompt (§14.1).
func (a *Agent) summarizer() (compact.Summarizer, int, provider.Model, error) {
	q := a.opts.Config.Context.SummaryModel
	if q == "" {
		q = a.opts.Config.Model
	}
	var (
		m   provider.Model
		ad  provider.Adapter
		err error
	)
	if cerr := a.opts.Providers.CheckCredential(q); cerr != nil {
		err = cerr
	} else {
		m, ad, err = a.opts.Providers.Resolve(q)
	}
	if err != nil {
		m, ad = a.model, a.adapter
		if !a.summaryFallbackWarned {
			a.summaryFallbackWarned = true
			a.emit(Warning{Text: fmt.Sprintf("summary model %s is unavailable (%v); using the active model %s", q, err, a.model.Qualified())})
		}
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
	// The cache is reset by this compaction anyway: prune superseded and
	// older completed results (with verified spill pointers) before building
	// the summary payload, so stale output is not paid for in the summary
	// request too.
	if err := a.elide(true); err != nil {
		return err
	}
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

// prepareRequest is the one work-request preflight (§14.1): measure the next
// request, compact once when above the operating trigger, then enforce the hard
// ceiling and fit the output budget. allowCompact is false for the immediate
// retry after an overflow compaction, where a second compaction would be a
// loop; the ceiling is still enforced.
func (a *Agent) prepareRequest(ctx context.Context, allowCompact bool) error {
	a.preflightMaxTokens = 0
	b := a.Budget()
	if b.Effective < 4096 {
		return fmt.Errorf("%w: effective window %d is below the 4096-token minimum", errContextCannotFit, b.Effective)
	}
	if err := a.trimOversizedPrompt(b); err != nil {
		return err
	}
	if err := a.elide(false); err != nil {
		return err
	}
	est := a.ContextTokens()
	if !b.Over(est) {
		return a.prepareOutput(est, b)
	}
	if allowCompact {
		err := a.Compact(ctx)
		switch {
		case err == nil, errors.Is(err, ErrNothingToCompact):
			// Fall through to the ceiling/band checks.
		case summarizerUnavailable(err):
			a.emit(Warning{Text: "auto-compaction skipped: " + err.Error()})
		default:
			return err
		}
		est = a.ContextTokens()
	}
	if est > b.Ceiling() {
		// No summarizer (or no useful summary) left the request over the hard
		// ceiling: P0a still ships a deterministic reduction floor. It never
		// calls a model; it replaces the older span with a bounded recap.
		if allowCompact {
			if derr := a.compactDeterministic(); derr == nil {
				est = a.ContextTokens()
			}
		}
	}
	if est > b.Ceiling() {
		// Last resort before the fixed-overhead exception: prune even the
		// protected working set with verified spill pointers.
		if err := a.pruneProtected(); err != nil {
			return err
		}
		est = a.ContextTokens()
	}
	if est > b.Ceiling() {
		// The fixed-overhead exception: only when the fixed system + tool
		// schemas alone already exceed the ceiling and no reducible history
		// remains. Mutable history cannot declare itself fixed.
		if ovh := a.fixedOverhead(est); ovh >= b.Ceiling() {
			if err := a.prepareOutput(est, b); err != nil {
				return err
			}
			if !a.warnedFixedOverhead {
				a.warnedFixedOverhead = true
				a.emit(Warning{Text: fmt.Sprintf("fixed context overhead is %dk tokens (system prompt + tools), already above the %dk ceiling; continuing with %dk of history",
					ovh/1000, b.Ceiling()/1000, max(0, est-ovh)/1000)})
			}
			return nil
		}
		return a.cannotFit(est, b)
	}
	if err := a.prepareOutput(est, b); err != nil {
		return err
	}
	if est > b.Trigger() && !a.pressureWarned {
		a.pressureWarned = true
		a.emit(ContextPressure{Input: est, Threshold: b.Trigger(), Ceiling: b.Ceiling()})
	}
	return nil
}

// trimOversizedPrompt bounds the newest genuine user prompt: when it exceeds
// min(Effective/4, 32K) estimated tokens, the full text is spilled and a Trim
// record makes the request carry a head/tail window. The transcript and audit
// view keep the original.
func (a *Agent) trimOversizedPrompt(b compact.Budget) error {
	if a.opts.Env == nil {
		return nil
	}
	a.mu.Lock()
	var id, text string
	for i := len(a.entries) - 1; i >= 0; i-- {
		e := a.entries[i]
		if e.Type != session.TypeMessage || e.Message == nil || e.Message.Role != llm.RoleUser {
			continue
		}
		id = e.ID
		for _, c := range e.Message.Content {
			if c.Type == llm.BlockText {
				text += c.Text
			}
		}
		break
	}
	trims := session.Trims(a.entries)
	a.mu.Unlock()
	if id == "" || text == "" {
		return nil
	}
	if _, ok := trims[id]; ok {
		return nil
	}
	budgetTokens := min(b.Effective/4, 32_000)
	if budgetTokens < 1 || compact.Tokens(len(text)) <= budgetTokens {
		return nil
	}
	note, err := tools.Spill(a.opts.Env, "trim", text)
	if err != nil || note == "" {
		return nil
	}
	path := spillPath(note)
	if path == "" {
		return nil
	}
	// 3 chars per token is the conservative direction for the trim window.
	budgetBytes := budgetTokens * 3
	head := budgetBytes * 6 / 10
	tail := budgetBytes - head
	if _, err := a.append(session.Entry{Type: session.TypeTrim, Trim: &session.Trim{
		IDs: []string{id}, Path: path, HeadBytes: head, TailBytes: tail,
		Notice: "[prompt trimmed to fit the context; full text saved to " + path + " — read it with offset/limit]",
	}}); err != nil {
		return err
	}
	a.mu.Lock()
	a.anchorValid = false
	a.mu.Unlock()
	return nil
}

// spillPath extracts the path from tools.Spill's note.
func spillPath(note string) string {
	const marker = "saved to "
	i := strings.Index(note, marker)
	if i < 0 {
		return ""
	}
	p := note[i+len(marker):]
	if j := strings.Index(p, " — "); j >= 0 {
		p = p[:j]
	}
	return strings.TrimSpace(p)
}

// compactDeterministic is the P0a reduction floor: it replaces the older span
// with a bounded, model-free recap. It is used only when a summary is
// unavailable or did not get the request under the hard ceiling.
func (a *Agent) compactDeterministic() error {
	live, prev := a.liveEntries()
	cut, _, ok := compact.FindCut(live, a.Budget().KeepRecent)
	if !ok {
		return ErrNothingToCompact
	}
	before := a.ContextTokens()
	var b strings.Builder
	b.WriteString("[deterministic compaction fallback: no model summary was available]\n")
	if prev.Summary != "" {
		b.WriteString("Previous summary:\n")
		b.WriteString(headRunes(prev.Summary, 4000))
		b.WriteString("\n")
	}
	b.WriteString(compact.Serialize(live[:cut]))
	recap := headRunes(b.String(), 12_000)
	// The tail of a long span often holds the exact failure the next turn
	// needs; keep the most recent error separately even when the head cap cut
	// it. It is still a persisted captured result, not a reconstructed one.
	for i := len(live[:cut]) - 1; i >= 0; i-- {
		if live[i].Kind == compact.KindToolResult && live[i].Result != nil && live[i].Result.IsError {
			recap += "\n[most recent error] " + headRunes(live[i].Result.Content, 1000)
			break
		}
	}
	recap += fmt.Sprintf("\n[deterministic recap: %d entries replaced; exact detail may be recovered from the session transcript]", cut)
	span := 0
	for _, e := range live[:cut] {
		span += compact.EntryTokens(e)
	}
	recapTokens := compact.Tokens(len(recap))
	ceiling := a.Budget().Ceiling()
	projected := before - span + recapTokens
	if recapTokens >= span || projected > ceiling {
		// The recap would grow the request or still miss the hard ceiling;
		// do not persist a compaction that cannot help.
		return ErrNothingToCompact
	}
	read, mod := compact.TrackFiles(live[:cut], prev.Read, prev.Modified)
	if _, err := a.append(session.Entry{Type: session.TypeCompaction, Compaction: &session.Compaction{
		Summary: recap, FirstKeptEntryID: live[cut].ID, TokensBefore: before, ReadFiles: read, ModifiedFiles: mod, Fallback: true,
	}}); err != nil {
		return err
	}
	a.mu.Lock()
	a.anchorValid = false
	a.mu.Unlock()
	if err := a.elide(true); err != nil {
		return err
	}
	a.emit(Compacted{TokensBefore: before, TokensAfter: a.ContextTokens()})
	return nil
}

// headRunes returns at most n runes of s, never splitting a UTF-8 sequence.
func headRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// prepareOutput caps the next request's max_tokens at the room the effective
// window actually leaves. Below minUsefulOutput the request cannot fit.
func (a *Agent) prepareOutput(est int, b compact.Budget) error {
	a.preflightMaxTokens = 0
	want := a.turnMaxTokens()
	floor := minUsefulOutput
	if want < floor {
		floor = want
	}
	room := b.Effective - est
	if room < floor {
		return a.cannotFit(est, b)
	}
	if want > room {
		a.preflightMaxTokens = room
	}
	return nil
}

// fixedOverhead estimates the part of the request that no history reduction can
// remove: stored system blocks, tool schemas and request framing.
func (a *Agent) fixedOverhead(est int) int {
	live, _ := a.liveEntries()
	n := 0
	for _, e := range live {
		n += compact.EntryTokens(e)
	}
	return max(0, est-n)
}

func (a *Agent) cannotFit(est int, b compact.Budget) error {
	return fmt.Errorf("%w: %d input tokens > %d ceiling (trigger %d, effective window %d); largest entries: %s%s",
		errContextCannotFit, est, b.Ceiling(), b.Trigger(), b.Effective, a.largestEntries(3), a.fixedOverheadNote(est))
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

# Phase 4 — Context Manager, Compaction, Resume — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Long sessions never overflow. When the usage-anchored estimate crosses `window − reserve`, older history is replaced by a structured summary, cutting only at valid points; provider overflow triggers one compact-and-retry; `--resume`/`--continue` rebuild the exact request context (stored system prompt + latest summary + kept entries) and repair a session killed mid-tool.

**Architecture:** `internal/compact` stays provider-free. It works on a neutral `compact.Entry` view of the transcript and receives a `Summarizer` func. It owns budgets (with small-window scaling), cut-point selection, serialization, cumulative file tracking and the summary prompt. `session.Messages` learns to start from the latest compaction. `agent` converts entries, triggers compaction at the two checkpoints, handles overflow, and gains `Resume`. `cmd/moca` wires `--resume`/`--continue` into both modes, and the TUI's `/compact` calls the agent.

**Tech Stack:** Go 1.27.1 stdlib only.

**Spec:** `docs/specs/DESIGN.md` (rev 11) — §6 (all), §8 (compaction entry, request rebuild, transcript repair, resume), §12.5 (`--resume`, `--continue`), phase plan item 4.

**Builds on:** Phases 1–3. Uses `compact.Tokens/MessageChars/RequestChars/UsageTokens` (phase 3), `agent.Agent` (anchor fields, `ContextTokens`, `TransformHistory`, `turn`, `Run`, `Start`), `session.Entry/Compaction/Messages/Repair/Open/ReadFile`, `provider.ErrContextOverflow`.

## Global Constraints

- Defaults: `reserveTokens` **16384**, `keepRecentTokens` **20000** (config `context.*`).
- Small windows: effective `reserve = min(reserveTokens, window/4)`, effective `keepRecent = min(keepRecentTokens, window/4)`. Models with `contextWindow < 16384` are refused (already enforced in phase 1).
- Trigger: `estimatedTokens > window − reserve`, checked **after every completed tool batch** and **before every new user prompt**.
- Loop guard: if a compaction leaves the estimate above the trigger, stop with an error naming the largest entries; never compact twice in a row.
- Cuts: never between a tool call and its result. Valid cut entries: user `message` or assistant `message` (a complete tool batch ends right before either). Cutting mid-turn (at an assistant message whose turn started with a user message before the cut) produces **two summaries merged**: prior history + the turn prefix.
- Repeated compactions summarize from the previous compaction's `firstKeptEntryId`, and the previous summary is fed in as context. File lists accumulate.
- Summary request: the cheap `model` (config `model`, not the current `/hard` model), effort `minimal` clamped per §3, `NoCacheWrite`, max 4096 output tokens, no tools.
- Summary format: Goal / Constraints & Preferences / Progress (done · in-progress · blocked) / Key Decisions / Next Steps / Critical Context, then `<read-files>` and `<modified-files>`.
- Serialization: `[User]:`, `[Assistant thinking]:`, `[Assistant]:`, `[Assistant tool calls]: name(k=v, …)`, `[Tool result]:` (truncated at **2000 chars**), headed `COMPACTION NOTE (lossy)`.
- Overflow: `provider.ErrContextOverflow`, or `StopLength` while the request's input tokens were within `reserve` of the window → one compact-and-retry as a fresh turn (the overflowing response is not persisted). A second overflow surfaces the original error.
- The compaction summary's `usage` counts toward session totals.
- Resume never restores yolo mode; it comes from the resuming process's flag/config (§7.5).
- Resume reuses the stored system prompt verbatim; a trailing `tool_use` without a result gets `Interrupted` results appended before the next request.

## Review Focus

1. **A single tool result larger than `keepRecent`** (e.g. a 120K-char `read` of a minified file) as the newest entry → the cut lands before its assistant message. The result is kept verbatim if the remaining budget allows; otherwise the loop guard names that entry ("tool_result for read(path=dist/app.min.js) ≈ 30k tokens") instead of compacting forever. Tested in Task 5.
2. **`/compact` with nothing to summarize** (only a couple of short messages) → a no-op message `nothing to compact`, no summary request sent, and no compaction entry written. Tested in Task 5.
3. **`--resume` of a session whose last model is from a provider whose key is now unset** → resume succeeds and shows the session; the first request fails with the env-var error (exit 2 in `-p`), and `--model` can override. Tested in Task 7.
4. **`--continue` in a workdir reached through a symlink** → finds the session (canonical path comparison, the same as the jail). Tested in Task 6.
5. **Compaction requested while the summarizer call itself overflows** (a huge serialized history on a small-window cheap model) → the serialized input is capped to the summarizer model's window, dropping the oldest serialized lines first with a `[… earlier history omitted]` marker, so the summary request always fits. Tested in Task 3.

---

## File Structure

```
internal/compact/
  budget.go budget_test.go        Budget with small-window scaling
  entry.go                        neutral Entry view
  cut.go cut_test.go              FindCut
  serialize.go serialize_test.go  Serialize, TrackFiles, CapChars
  summary.go summary_test.go      SummaryPrompt, Compact()
internal/session/
  rebuild.go rebuild_test.go      (modified) Messages honours compaction
  find.go find_test.go            Find(id8|last), FindForWorkdir
internal/agent/
  compaction.go compaction_test.go  maybeCompact, Compact, overflow retry, entry conversion
  resume.go resume_test.go          Resume
  start.go                          (refactored) shared build()
cmd/moca/
  main.go oneshot.go tui.go         --resume / --continue
internal/tui/
  app.go                            /compact → agent.Compact; Compacted event line; resumed banner
```

---

### Task 1: Budget

**Files:**
- Create: `internal/compact/budget.go`
- Test: `internal/compact/budget_test.go`

**Interfaces:**
- Produces: `type Budget struct { Window, Reserve, KeepRecent int }`; `func NewBudget(window, reserve, keepRecent int) Budget` (applies `min(x, window/4)`); `func (b Budget) Trigger() int` (= `Window − Reserve`); `func (b Budget) Over(tokens int) bool` (= `tokens > Trigger()`).

- [ ] **Step 1: Write failing test**

```go
// internal/compact/budget_test.go
package compact

import "testing"

func TestBudget(t *testing.T) {
	big := NewBudget(1_000_000, 16384, 20000)
	if big.Reserve != 16384 || big.KeepRecent != 20000 || big.Trigger() != 1_000_000-16384 {
		t.Fatalf("%+v", big)
	}
	small := NewBudget(32768, 16384, 20000)
	if small.Reserve != 8192 || small.KeepRecent != 8192 || small.Trigger() != 24576 {
		t.Fatalf("small window scales to window/4: %+v", small)
	}
	if small.Over(24576) || !small.Over(24577) {
		t.Fatal("strictly greater")
	}
}
```

- [ ] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/compact/budget.go
package compact

// Budget holds the token budgets for one model (§6). Percentages would
// conflate a 200K and a 1M window; tokens don't.
type Budget struct {
	Window, Reserve, KeepRecent int
}

func NewBudget(window, reserve, keepRecent int) Budget {
	return Budget{Window: window, Reserve: min(reserve, window/4), KeepRecent: min(keepRecent, window/4)}
}

func (b Budget) Trigger() int         { return b.Window - b.Reserve }
func (b Budget) Over(tokens int) bool { return tokens > b.Trigger() }
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/compact && git commit -m "feat(compact): token budgets with small-window scaling"`

---

### Task 2: Neutral entry view + cut-point selection

**Files:**
- Create: `internal/compact/entry.go`, `internal/compact/cut.go`
- Test: `internal/compact/cut_test.go`

**Interfaces:**
- Produces:

```go
type Kind int // KindUser, KindAssistant, KindToolUse, KindToolResult
type Entry struct {
	ID     string
	Kind   Kind
	Msg    *llm.Message    // KindUser / KindAssistant
	Call   *llm.ToolCall   // KindToolUse
	Result *llm.ToolResult // KindToolResult
}
func EntryTokens(e Entry) int
// FindCut chooses the first kept entry. es is the live window (entries after
// the previous compaction's first kept entry, inclusive). Returns cut index,
// turnStart index (>=0 only when the cut splits a turn: es[cut] is an
// assistant message and the user message that began its turn is at
// turnStart, 0 < turnStart < cut... or turnStart == 0) and ok=false when
// nothing can be summarized.
func FindCut(es []Entry, keepRecent int) (cut, turnStart int, ok bool)
```

- Algorithm:
  1. Valid cut indexes `V` = indexes `i > 0` with `es[i].Kind ∈ {KindUser, KindAssistant}`.
  2. If `V` is empty → `ok=false`.
  3. Let `suffix(i)` = sum of `EntryTokens(es[i:])`. Choose the **smallest** `c ∈ V` with `suffix(c) ≤ keepRecent`. If none qualifies, choose `max(V)` (keep the minimum possible).
  4. If `es[c].Kind == KindAssistant`, then `turnStart` = the largest `u < c` with `es[u].Kind == KindUser`; if none exists, `turnStart = -1`. If `es[c].Kind == KindUser` → `turnStart = -1`.

- [ ] **Step 1: Write failing tests**

```go
// internal/compact/cut_test.go
package compact

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func u(id, text string) Entry {
	return Entry{ID: id, Kind: KindUser, Msg: &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: text}}}}
}
func a(id, text string) Entry {
	return Entry{ID: id, Kind: KindAssistant, Msg: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: text}}}}
}
func call(id, name string) Entry {
	return Entry{ID: id, Kind: KindToolUse, Call: &llm.ToolCall{ID: id, Name: name, Input: json.RawMessage(`{}`)}}
}
func res(id, body string) Entry {
	return Entry{ID: "r" + id, Kind: KindToolResult, Result: &llm.ToolResult{CallID: id, Content: body}}
}

var k = strings.Repeat("x", 4000) // 1000 tokens

func TestFindCutKeepsRecentAtValidPoint(t *testing.T) {
	es := []Entry{u("u1", k), a("a1", k), call("c1", "read"), res("c1", k), a("a2", k), u("u2", k), a("a3", k)}
	cut, ts, ok := FindCut(es, 2500)
	if !ok || es[cut].ID != "u2" || ts != -1 {
		t.Fatalf("cut %d (%s) ts %d", cut, es[cut].ID, ts)
	}
}

func TestFindCutNeverBetweenCallAndResult(t *testing.T) {
	es := []Entry{u("u1", k), a("a1", k), call("c1", "read"), res("c1", strings.Repeat("y", 40000)), a("a2", "done")}
	cut, _, ok := FindCut(es, 1500)
	if !ok || es[cut].ID != "a2" {
		t.Fatalf("only a2 qualifies; got %s", es[cut].ID)
	}
	for i := range es {
		if es[i].Kind == KindToolResult && cut == i {
			t.Fatal("cut at a tool result")
		}
	}
}

func TestFindCutSplitTurn(t *testing.T) {
	es := []Entry{u("u0", k), a("a0", k), u("u1", "do it"), a("a1", k), call("c1", "ls"), res("c1", k), a("a2", k), call("c2", "ls"), res("c2", k), a("a3", "x")}
	cut, ts, ok := FindCut(es, 2200)
	if !ok || es[cut].Kind != KindAssistant || es[ts].ID != "u1" {
		t.Fatalf("cut %s ts %d", es[cut].ID, ts)
	}
}

func TestFindCutNothingToDo(t *testing.T) {
	if _, _, ok := FindCut([]Entry{u("u1", "hi")}, 100); ok {
		t.Fatal("single entry")
	}
	if _, _, ok := FindCut([]Entry{u("u1", "hi"), call("c", "ls"), res("c", "x")}, 100); ok {
		t.Fatal("no valid cut after index 0")
	}
}
```

- [ ] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/compact/entry.go
package compact

import "github.com/adeotek/moca/internal/llm"

type Kind int

const (
	KindUser Kind = iota
	KindAssistant
	KindToolUse
	KindToolResult
)

// Entry is compact's provider- and session-free view of one transcript
// entry; agent converts session entries into it.
type Entry struct {
	ID     string
	Kind   Kind
	Msg    *llm.Message
	Call   *llm.ToolCall
	Result *llm.ToolResult
}

func EntryTokens(e Entry) int {
	n := 0
	switch {
	case e.Msg != nil:
		n = MessageChars(*e.Msg)
	case e.Call != nil:
		n = len(e.Call.Name) + len(e.Call.Input)
	case e.Result != nil:
		n = len(e.Result.Content)
	}
	return Tokens(n)
}
```

```go
// internal/compact/cut.go
package compact

func FindCut(es []Entry, keepRecent int) (int, int, bool) {
	suffix := make([]int, len(es)+1)
	for i := len(es) - 1; i >= 0; i-- {
		suffix[i] = suffix[i+1] + EntryTokens(es[i])
	}
	cut := -1
	for i := 1; i < len(es); i++ {
		if es[i].Kind != KindUser && es[i].Kind != KindAssistant {
			continue
		}
		if suffix[i] <= keepRecent {
			cut = i
			break
		}
		cut = i // remember the latest valid cut in case none fits
	}
	if cut < 0 {
		return 0, -1, false
	}
	turnStart := -1
	if es[cut].Kind == KindAssistant {
		for j := cut - 1; j >= 0; j-- {
			if es[j].Kind == KindUser {
				turnStart = j
				break
			}
		}
	}
	return cut, turnStart, true
}
```

In the loop, `cut = i` on non-fitting candidates keeps advancing, so when nothing fits `cut` ends at the **last** valid index (`max(V)`). When one fits, `break` returns the **smallest** fitting one. That matches the algorithm.

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/compact && git commit -m "feat(compact): cut-point selection that never orphans tool results"`

---

### Task 3: Serialization, file tracking, input capping

**Files:**
- Create: `internal/compact/serialize.go`
- Test: `internal/compact/serialize_test.go`

**Interfaces:**
- Produces:
  - `func Serialize(es []Entry) string` — header line `COMPACTION NOTE (lossy): tool results truncated at 2000 chars.`, then one block per entry: `[User]: …`, `[Assistant thinking]: …`, `[Assistant]: …`, `[Assistant tool calls]: read(path=a.go, offset=10)` (top-level JSON keys in sorted order, values compact-JSON, strings unquoted, each value cut at 200 chars), `[Tool result]: …` (≤2000 chars + `[… truncated]`; `[Tool error]:` when `IsError`).
  - `func TrackFiles(es []Entry, prevRead, prevMod []string) (read, mod []string)` — `read` calls → read; `write`/`edit` → modified (and removed from read); deduped, sorted, cumulative.
  - `func CapChars(s string, max int) string` — if longer, keep the tail and drop the oldest whole lines, with the first line `[… earlier history omitted]`.

- [ ] **Step 1: Write failing tests**

```go
// internal/compact/serialize_test.go
package compact

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestSerialize(t *testing.T) {
	es := []Entry{
		u("u1", "fix the bug"),
		{ID: "a1", Kind: KindAssistant, Msg: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "hmm"}, {Type: llm.BlockText, Text: "looking"}}}},
		{ID: "c1", Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"a.go","offset":10}`)}},
		{ID: "r1", Kind: KindToolResult, Result: &llm.ToolResult{Content: strings.Repeat("z", 3000)}},
		{ID: "r2", Kind: KindToolResult, Result: &llm.ToolResult{Content: "boom", IsError: true}},
	}
	s := Serialize(es)
	for _, want := range []string{"COMPACTION NOTE (lossy)", "[User]: fix the bug", "[Assistant thinking]: hmm",
		"[Assistant]: looking", "[Assistant tool calls]: read(offset=10, path=a.go)", "[… truncated]", "[Tool error]: boom"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(s, "z") != 2000 {
		t.Fatal("2K truncation")
	}
}

func TestTrackFiles(t *testing.T) {
	es := []Entry{
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"b.go"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"c.go"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "edit", Input: json.RawMessage(`{"path":"c.go","old_string":"a","new_string":"b"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "write", Input: json.RawMessage(`{"path":"d.go","content":""}`)}},
	}
	read, mod := TrackFiles(es, []string{"a.go"}, []string{"z.go"})
	if !slices.Equal(read, []string{"a.go", "b.go"}) || !slices.Equal(mod, []string{"c.go", "d.go", "z.go"}) {
		t.Fatal(read, mod)
	}
}

func TestCapChars(t *testing.T) {
	s := strings.Repeat("old line\n", 100) + "newest"
	got := CapChars(s, 100)
	if len(got) > 100+len("[… earlier history omitted]\n") || !strings.HasPrefix(got, "[… earlier history omitted]\n") || !strings.HasSuffix(got, "newest") {
		t.Fatalf("%q", got)
	}
	if CapChars("short", 100) != "short" {
		t.Fatal("no-op")
	}
}
```

- [ ] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/compact/serialize.go
package compact

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const resultCap = 2000

func callLine(c *llm.ToolCall) string {
	var m map[string]json.RawMessage
	json.Unmarshal(c.Input, &m)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		v := string(m[k])
		var s string
		if json.Unmarshal(m[k], &s) == nil {
			v = s
		}
		if len(v) > 200 {
			v = v[:200] + "…"
		}
		parts[i] = k + "=" + strings.ReplaceAll(v, "\n", "⏎")
	}
	return fmt.Sprintf("%s(%s)", c.Name, strings.Join(parts, ", "))
}

func Serialize(es []Entry) string {
	var sb strings.Builder
	sb.WriteString("COMPACTION NOTE (lossy): tool results truncated at 2000 chars.\n")
	for _, e := range es {
		switch e.Kind {
		case KindUser:
			for _, c := range e.Msg.Content {
				if c.Type == llm.BlockText {
					fmt.Fprintf(&sb, "[User]: %s\n", c.Text)
				}
			}
		case KindAssistant:
			for _, c := range e.Msg.Content {
				switch c.Type {
				case llm.BlockThinking:
					if c.Text != "" {
						fmt.Fprintf(&sb, "[Assistant thinking]: %s\n", c.Text)
					}
				case llm.BlockText:
					fmt.Fprintf(&sb, "[Assistant]: %s\n", c.Text)
				}
			}
		case KindToolUse:
			fmt.Fprintf(&sb, "[Assistant tool calls]: %s\n", callLine(e.Call))
		case KindToolResult:
			body := e.Result.Content
			if len(body) > resultCap {
				body = body[:resultCap] + "[… truncated]"
			}
			label := "Tool result"
			if e.Result.IsError {
				label = "Tool error"
			}
			fmt.Fprintf(&sb, "[%s]: %s\n", label, body)
		}
	}
	return sb.String()
}

func TrackFiles(es []Entry, prevRead, prevMod []string) ([]string, []string) {
	read := map[string]bool{}
	mod := map[string]bool{}
	for _, p := range prevRead {
		read[p] = true
	}
	for _, p := range prevMod {
		mod[p] = true
	}
	for _, e := range es {
		if e.Kind != KindToolUse {
			continue
		}
		var a struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(e.Call.Input, &a) != nil || a.Path == "" {
			continue
		}
		switch e.Call.Name {
		case "read":
			if !mod[a.Path] {
				read[a.Path] = true
			}
		case "write", "edit":
			mod[a.Path] = true
			delete(read, a.Path)
		}
	}
	keys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	return keys(read), keys(mod)
}

const omitted = "[… earlier history omitted]\n"

func CapChars(s string, max int) string {
	if len(s) <= max {
		return s
	}
	tail := s[len(s)-max:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	return omitted + tail
}
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/compact && git commit -m "feat(compact): lossy serialization, cumulative file tracking, input capping"`

---

### Task 4: Summary prompt + `Compact()`

**Files:**
- Create: `internal/compact/summary.go`
- Test: `internal/compact/summary_test.go`

**Interfaces:**
- Produces:
  - `const SummarySystem` — instructions for the structured format.
  - `type Summarizer func(ctx context.Context, system, user string) (string, llm.Usage, error)`
  - `type Prev struct { Summary string; Read, Modified []string }`
  - `type Result struct { Summary, FirstKept string; Usage llm.Usage; Read, Modified []string }`
  - `func Compact(ctx context.Context, es []Entry, prev Prev, keepRecent, inputCapChars int, sum Summarizer) (Result, bool, error)` — `bool=false` → nothing to compact (no summarizer call).
  - Behaviour:
    - `FindCut`. If the turn is not split: one call with `user = prevBlock + Serialize(es[:cut])`.
    - Split turn: call A over `es[:turnStart]` (skipped when `turnStart == 0` and there is no previous summary), call B over `es[turnStart:cut]` with the instruction `"This is the beginning of the CURRENT turn, still in progress. Summarize the request and progress so far."`. Merged: `A + "\n\n## Current turn (in progress)\n" + B`.
    - `prevBlock` = `"Previous summary (update it, don't drop facts):\n" + prev.Summary + "\n\n"` when present.
    - Every user payload passes through `CapChars(…, inputCapChars)`.
    - The final summary gets the file lists appended: `<read-files>\n…\n</read-files>\n<modified-files>\n…\n</modified-files>`.
    - Usage is summed over calls.

- [ ] **Step 1: Write failing tests**

```go
// internal/compact/summary_test.go
package compact

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

type fakeSum struct {
	calls []string
}

func (f *fakeSum) fn(_ context.Context, system, user string) (string, llm.Usage, error) {
	f.calls = append(f.calls, user)
	return "## Goal\nsummary " + string(rune('A'+len(f.calls)-1)), llm.Usage{Input: 100, Output: 10}, nil
}

func TestCompactSimple(t *testing.T) {
	f := &fakeSum{}
	es := []Entry{u("u1", k), a("a1", k), {ID: "c", Kind: KindToolUse, Call: &llm.ToolCall{Name: "edit", Input: json.RawMessage(`{"path":"x.go"}`)}},
		res("c", "ok"), u("u2", "next"), a("a2", "fine")}
	r, ok, err := Compact(context.Background(), es, Prev{Summary: "old facts", Read: []string{"r.go"}}, 100, 1<<20, f.fn)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || !strings.Contains(f.calls[0], "old facts") || !strings.Contains(f.calls[0], "[User]: x") {
		t.Fatalf("calls %q", f.calls)
	}
	if r.FirstKept != "u2" || r.Usage.Input != 100 {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.Summary, "<modified-files>\nx.go\n</modified-files>") || !strings.Contains(r.Summary, "<read-files>\nr.go\n</read-files>") {
		t.Fatal(r.Summary)
	}
}

func TestCompactSplitTurnTwoSummaries(t *testing.T) {
	f := &fakeSum{}
	es := []Entry{u("u0", k), a("a0", k), u("u1", "do it"), a("a1", k), call("c1", "ls"), res("c1", k), a("a2", "x")}
	r, ok, _ := Compact(context.Background(), es, Prev{}, 300, 1<<20, f.fn)
	if !ok || len(f.calls) != 2 || !strings.Contains(f.calls[1], "CURRENT turn") || !strings.Contains(f.calls[1], "do it") {
		t.Fatalf("%q", f.calls)
	}
	if !strings.Contains(r.Summary, "summary A") || !strings.Contains(r.Summary, "## Current turn (in progress)\n## Goal\nsummary B") {
		t.Fatal(r.Summary)
	}
	if r.Usage.Input != 200 {
		t.Fatal("usage summed")
	}
}

func TestCompactNothing(t *testing.T) {
	f := &fakeSum{}
	if _, ok, _ := Compact(context.Background(), []Entry{u("u", "hi")}, Prev{}, 100, 1<<20, f.fn); ok || len(f.calls) != 0 {
		t.Fatal("no call when nothing to compact")
	}
}

func TestCompactCapsInput(t *testing.T) {
	f := &fakeSum{}
	es := []Entry{u("u0", strings.Repeat("q\n", 50000)), a("a0", "x"), u("u1", "y")}
	Compact(context.Background(), es, Prev{}, 1, 1000, f.fn)
	if len(f.calls[0]) > 1100 || !strings.HasPrefix(f.calls[0], "[… earlier history omitted]") {
		t.Fatal(len(f.calls[0]))
	}
}
```

- [ ] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/compact/summary.go
package compact

import (
	"context"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const SummarySystem = `You compress a coding-agent conversation into a summary that lets the agent continue the work without the original messages.
Write exactly these sections, in this order, as Markdown headings:
## Goal
## Constraints & Preferences
## Progress
### Done
### In progress
### Blocked
## Key Decisions
## Next Steps
## Critical Context
Rules: keep exact identifiers (file paths, function names, commands, error messages, versions). Prefer facts over narration. Omit pleasantries. If a section is empty write "none". Do not invent anything not in the conversation.`

const currentTurnNote = "This is the beginning of the CURRENT turn, still in progress. Summarize the request and progress so far.\n\n"

type Summarizer func(ctx context.Context, system, user string) (string, llm.Usage, error)

type Prev struct {
	Summary        string
	Read, Modified []string
}

type Result struct {
	Summary, FirstKept string
	Usage              llm.Usage
	Read, Modified     []string
}

func Compact(ctx context.Context, es []Entry, prev Prev, keepRecent, inputCapChars int, sum Summarizer) (Result, bool, error) {
	cut, turnStart, ok := FindCut(es, keepRecent)
	if !ok {
		return Result{}, false, nil
	}
	prevBlock := ""
	if prev.Summary != "" {
		prevBlock = "Previous summary (update it, don't drop facts):\n" + prev.Summary + "\n\n"
	}
	var usage llm.Usage
	call := func(user string) (string, error) {
		s, u, err := sum(ctx, SummarySystem, CapChars(user, inputCapChars))
		usage = usage.Add(u)
		return strings.TrimSpace(s), err
	}
	var summary string
	var err error
	if turnStart < 0 {
		summary, err = call(prevBlock + Serialize(es[:cut]))
	} else {
		var a, b string
		if turnStart > 0 || prevBlock != "" {
			if a, err = call(prevBlock + Serialize(es[:turnStart])); err != nil {
				return Result{}, false, err
			}
		}
		if b, err = call(currentTurnNote + Serialize(es[turnStart:cut])); err != nil {
			return Result{}, false, err
		}
		summary = strings.TrimSpace(a + "\n\n## Current turn (in progress)\n" + b)
	}
	if err != nil {
		return Result{}, false, err
	}
	read, mod := TrackFiles(es[:cut], prev.Read, prev.Modified)
	summary += "\n\n<read-files>\n" + strings.Join(read, "\n") + "\n</read-files>\n<modified-files>\n" +
		strings.Join(mod, "\n") + "\n</modified-files>"
	return Result{Summary: summary, FirstKept: es[cut].ID, Usage: usage, Read: read, Modified: mod}, true, nil
}
```

`TestCompactSplitTurnTwoSummaries` expects call B's output right after the heading. `TrimSpace` on each call result keeps the merge exact.

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/compact && git commit -m "feat(compact): structured summaries, split-turn merge, cumulative files"`

---

### Task 5: Request rebuild honours compaction + agent compaction/overflow

**Files:**
- Modify: `internal/session/rebuild.go`, `internal/agent/agent.go`
- Create: `internal/agent/compaction.go`
- Test: `internal/session/rebuild_test.go` (extend), `internal/agent/compaction_test.go`

**Interfaces:**
- `session.Messages(entries)` (changed): if a `compaction` entry exists, use the **latest** one. Output = a user message `"[Summary of earlier conversation]\n" + summary`, followed by the normal rebuild of the entries whose index ≥ index(`FirstKeptEntryID`), skipping compaction entries. A kept user message merges into the summary message; assistant/tool grouping is unchanged.
- `func session.LatestCompaction(entries []Entry) (*Compaction, int)` — the entry and its index (`-1` if none).
- Agent (`compaction.go`):
  - `type Compacted struct { TokensBefore, TokensAfter int }` event; `func (Compacted) isEvent()`.
  - `var ErrNothingToCompact = errors.New("nothing to compact")`
  - `func (a *Agent) Budget() compact.Budget` — from the **current** model's window and config.
  - `func (a *Agent) Compact(ctx context.Context) error` — manual + automatic path. Steps:
    1. Live window = entries from the latest compaction's first kept (or the start), restricted to message/tool_use/tool_result entries, converted to `compact.Entry`.
    2. Summarizer = resolve `a.opts.Config.Model` (the cheap model), effort `ClampEffort(minimal)`, `NoCacheWrite: true`, `MaxTokens: min(4096, m.MaxOutput)`, no tools.
    3. Input cap = `(cheap.ContextWindow − 4096 − len(SummarySystem)/4 − 512) * 4` chars.
    4. Append the `compaction` entry (`Summary`, `FirstKeptEntryID`, `TokensBefore = a.ContextTokens()`, `Usage`, `ReadFiles`, `ModifiedFiles`); add usage/cost to totals; set `anchorValid = false`; emit `Compacted`.
  - `func (a *Agent) maybeCompact(ctx context.Context) error` — if `Budget().Over(ContextTokens())`: `Compact`, then if still over → `fmt.Errorf("context still over budget after compaction (%d > %d tokens); largest entries: %s", …)` listing the top 3 live entries by tokens (`tool_result for read(path=…) ≈ 30k tokens`).
  - `Run` calls `maybeCompact` before appending the new user prompt and after each tool batch (after steering is applied).
  - Overflow: `turn()` returns `errOverflow` (unexported) **without persisting** when `errors.Is(err, provider.ErrContextOverflow)`, or when `resp.Stop == llm.StopLength && UsageTokens-input(resp.Usage) ≥ window − reserve`. `Run` handles it once per turn by calling `Compact` (if that returns `ErrNothingToCompact`, surface the original error) and repeating the turn. A second `errOverflow` surfaces the original error.

- [ ] **Step 1: Write failing tests**

```go
// append to internal/session/rebuild_test.go
func TestMessagesFromCompaction(t *testing.T) {
	es := []Entry{
		{ID: "u1", Type: TypeMessage, Message: txt(llm.RoleUser, "old")},
		{ID: "a1", Type: TypeMessage, Message: txt(llm.RoleAssistant, "old answer")},
		{ID: "u2", Type: TypeMessage, Message: txt(llm.RoleUser, "kept question")},
		{ID: "a2", Type: TypeMessage, Message: txt(llm.RoleAssistant, "kept answer")},
		{ID: "cp", Type: TypeCompaction, Compaction: &Compaction{Summary: "S", FirstKeptEntryID: "u2"}},
		{ID: "u3", Type: TypeMessage, Message: txt(llm.RoleUser, "new")},
	}
	m := Messages(es)
	if len(m) != 3 || m[0].Content[0].Text != "[Summary of earlier conversation]\nS" || m[0].Content[1].Text != "kept question" {
		t.Fatalf("%+v", m)
	}
	if c, i := LatestCompaction(es); c == nil || i != 4 {
		t.Fatal("latest compaction")
	}
}
```

```go
// internal/agent/compaction_test.go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/session"
)

// Small-window model (32K): reserve 8192, keepRecent 8192, trigger 24576 tokens.
const smallModel = `"m":{"contextWindow":32768,"maxOutputTokens":4096}`

func bigToolTurns(n int) []string {
	var turns []string
	for range n {
		turns = append(turns, toolTurn([2]string{"shell", `{"command":"head -c 20000 /dev/zero | tr '\\0' x"}`}))
	}
	return turns
}

func TestAutoCompactionShrinksContext(t *testing.T) {
	turns := bigToolTurns(6)
	turns = append(turns, textTurn("## Goal\nsummary"), textTurn("done"))
	s := newScript(t, turns...)
	a, _, evs := startTestWith(t, s, "", smallModel)
	a.opts.Env.Commands = allowAll{}
	out, err := a.Run(context.Background(), "make output")
	if err != nil || out.Text != "done" {
		t.Fatal(out, err)
	}
	var comp *Compacted
	for _, e := range *evs {
		if c, ok := e.(Compacted); ok {
			comp = &c
		}
	}
	if comp == nil || comp.TokensAfter >= comp.TokensBefore {
		t.Fatalf("compaction must shrink context: %+v", comp)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if len(session.Repair(entries, "")) != 0 {
		t.Fatal("no orphaned tool calls")
	}
	msgs := session.Messages(entries)
	if !strings.HasPrefix(msgs[0].Content[0].Text, "[Summary of earlier conversation]") {
		t.Fatal("request starts from the summary")
	}
	// the summary request: no tools, NoCacheWrite has no wire effect on completions, max_tokens ≤ 4096
	var sumBody map[string]any
	for _, b := range s.bodies {
		if sys, _ := b["messages"].([]any)[0].(map[string]any)["content"].(string); strings.Contains(sys, "## Constraints & Preferences") {
			sumBody = b
		}
	}
	if sumBody == nil || sumBody["tools"] != nil || sumBody["max_tokens"].(float64) > 4096 {
		t.Fatalf("summary request shape: %v", sumBody)
	}
}

func TestLoopGuardNamesOversizedEntry(t *testing.T) {
	huge := toolTurn([2]string{"shell", `{"command":"head -c 120000 /dev/zero | tr '\\0' x"}`})
	s := newScript(t, huge, textTurn("## Goal\ns"), textTurn("never"))
	a, _, _ := startTestWith(t, s, "", smallModel)
	a.opts.Env.Commands = allowAll{}
	_, err := a.Run(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "still over budget") || !strings.Contains(err.Error(), "shell") {
		t.Fatalf("%v", err)
	}
}

func TestOverflowCompactsAndRetriesOnce(t *testing.T) {
	overflow := "HTTP400:" + `{"error":{"code":"context_length_exceeded","message":"maximum context length"}}`
	s := newScript(t, textTurn("a1"), textTurn("a2"), overflow, textTurn("## Goal\ns"), textTurn("recovered"))
	a, _, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), strings.Repeat("q", 4000))
	a.Run(context.Background(), strings.Repeat("r", 4000))
	out, err := a.Run(context.Background(), "third")
	if err != nil || out.Text != "recovered" {
		t.Fatal(out, err)
	}
}

func TestManualCompactNothing(t *testing.T) {
	s := newScript(t, textTurn("hi"))
	a, _, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), "hello")
	n := len(s.bodies)
	if err := a.Compact(context.Background()); err != ErrNothingToCompact || len(s.bodies) != n {
		t.Fatal(err)
	}
}
```

Test support to add in `agent_test.go`:
- `allowAll{}` — a `tools.CommandChecker` that allows everything (`Check` returns nil, nil, nil).
- Extend `scriptServer`: a turn string starting with `"HTTP400:"` replies with status 400 and the rest as the body.

The overflow test sends long prompts so there is something to compact (the window is 65536 in `startTestWith`'s default model). With keepRecent 16384 (window/4), cutting at the third prompt leaves earlier turns to summarize.

- [ ] **Step 2: Run** — `go test ./internal/session/ ./internal/agent/` → FAIL.

- [ ] **Step 3: Implement `session.Messages` + `LatestCompaction`**

```go
// internal/session/rebuild.go — replace Messages with:

func LatestCompaction(entries []Entry) (*Compaction, int) {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Type == TypeCompaction {
			return entries[i].Compaction, i
		}
	}
	return nil, -1
}

// Messages rebuilds request messages: latest compaction summary (if any) +
// entries from its firstKeptEntryId onward (§8).
func Messages(entries []Entry) []llm.Message {
	var out []llm.Message
	start := 0
	if c, _ := LatestCompaction(entries); c != nil {
		for i, e := range entries {
			if e.ID == c.FirstKeptEntryID {
				start = i
				break
			}
		}
		appendUser(&out, llm.ContentBlock{Type: llm.BlockText, Text: "[Summary of earlier conversation]\n" + c.Summary})
	}
	assistantIdx := map[string]int{}
	for _, e := range entries[start:] {
		switch e.Type {
		case TypeMessage:
			if e.Message.Role == llm.RoleAssistant {
				out = append(out, llm.Message{Role: llm.RoleAssistant, Content: append([]llm.ContentBlock{}, e.Message.Content...)})
				assistantIdx[e.ID] = len(out) - 1
				continue
			}
			appendUser(&out, e.Message.Content...)
		case TypeToolUse:
			if i, ok := assistantIdx[e.ToolUse.MessageID]; ok {
				call := e.ToolUse.Call
				out[i].Content = append(out[i].Content, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: &call})
			}
		case TypeToolResult:
			r := *e.ToolResult
			appendUser(&out, llm.ContentBlock{Type: llm.BlockToolResult, ToolResult: &r})
		}
	}
	return out
}
```

Phase 3's `ContextTokens` slices `a.entries[a.anchorEntries:]` and rebuilds it. That slice contains no compaction entry, so its count is unaffected.

- [ ] **Step 4: Implement `agent/compaction.go`**

```go
// internal/agent/compaction.go
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

type Compacted struct{ TokensBefore, TokensAfter int }

func (Compacted) isEvent() {}

var (
	ErrNothingToCompact = errors.New("nothing to compact")
	errOverflow         = errors.New("context overflow")
)

func (a *Agent) Budget() compact.Budget {
	c := a.opts.Config.Context
	return compact.NewBudget(a.model.ContextWindow, c.ReserveTokens, c.KeepRecentTokens)
}

// liveEntries returns the transcript entries after the latest compaction's
// first kept entry, as compact entries, plus the previous compaction state.
func (a *Agent) liveEntries() ([]compact.Entry, compact.Prev) {
	var prev compact.Prev
	start := 0
	if c, _ := session.LatestCompaction(a.entries); c != nil {
		prev = compact.Prev{Summary: c.Summary, Read: c.ReadFiles, Modified: c.ModifiedFiles}
		for i, e := range a.entries {
			if e.ID == c.FirstKeptEntryID {
				start = i
				break
			}
		}
	}
	var out []compact.Entry
	for _, e := range a.entries[start:] {
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

// summarizer returns the summary function over the cheap model, the input
// cap in chars, and the model (for cost accounting).
func (a *Agent) summarizer() (compact.Summarizer, int, provider.Model, error) {
	m, ad, err := a.opts.Providers.Resolve(a.opts.Config.Model)
	if err != nil {
		return nil, 0, provider.Model{}, err
	}
	maxTok := min(4096, m.MaxOutput)
	capChars := (m.ContextWindow - maxTok - len(compact.SummarySystem)/4 - 512) * 4
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
	a.usage, a.cost = a.usage.Add(u), a.cost+cost
	if _, err := a.append(session.Entry{Type: session.TypeCompaction, Usage: &u, Cost: cost, Compaction: &session.Compaction{
		Summary: r.Summary, FirstKeptEntryID: r.FirstKept, TokensBefore: before, Usage: u,
		ReadFiles: r.Read, ModifiedFiles: r.Modified}}); err != nil {
		return err
	}
	a.anchorValid = false
	a.emit(Compacted{TokensBefore: before, TokensAfter: a.ContextTokens()})
	return nil
}

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

func (a *Agent) largestEntries(n int) string {
	live, _ := a.liveEntries()
	calls := map[string]string{}
	for _, e := range live {
		if e.Kind == compact.KindToolUse {
			calls[e.Call.ID] = fmt.Sprintf("%s(%s)", e.Call.Name, strings.TrimSuffix(strings.TrimPrefix(string(e.Call.Input), "{"), "}"))
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
```

The compaction entry carries a top-level `Usage` and `Cost`, so phase 2's `New` restores session totals on resume (it sums `e.Usage`/`e.Cost` over prior entries).

- [ ] **Step 5: Wire into `turn()` and `Run()`**

In `turn()`, right after `adapter.Stream` returns:

```go
	if errors.Is(err, provider.ErrContextOverflow) ||
		(err == nil && resp.Stop == llm.StopLength &&
			resp.Usage.Input+resp.Usage.CacheRead+resp.Usage.CacheWrite >= a.model.ContextWindow-a.Budget().Reserve) {
		a.lastOverflow = err
		if a.lastOverflow == nil {
			a.lastOverflow = fmt.Errorf("response hit the length limit with the context full (%d input tokens)", resp.Usage.Input+resp.Usage.CacheRead)
		}
		a.emit(StreamReset{})
		return resp, nil, errOverflow
	}
```

(Field `lastOverflow error` on `Agent`.) The overflow branch runs **before** the generic error branch, so nothing is persisted.

In `Run`, add a helper used at both turn call sites:

```go
// turnWithRecovery runs one turn; on overflow it compacts once and retries.
func (a *Agent) turnWithRecovery(ctx context.Context, choice llm.ToolChoice) (llm.Response, []llm.ToolCall, error) {
	resp, calls, err := a.turn(ctx, choice)
	if !errors.Is(err, errOverflow) {
		return resp, calls, err
	}
	orig := a.lastOverflow
	if cerr := a.Compact(ctx); cerr != nil {
		return resp, nil, orig
	}
	resp, calls, err = a.turn(ctx, choice)
	if errors.Is(err, errOverflow) {
		return resp, nil, orig
	}
	return resp, calls, err
}
```

Replace both `a.turn(ctx, …)` calls in `Run` with `a.turnWithRecovery(ctx, …)`. Insert `if err := a.maybeCompact(ctx); err != nil { return Outcome{}, err }`:
- at the start of `Run`, before appending the user prompt;
- after each tool batch, after `applySteering()`.

- [ ] **Step 6: Run** — `go test ./internal/... -race -v` → PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/session internal/agent
git commit -m "feat(agent): token-triggered compaction, loop guard, overflow compact-and-retry"
```

---

### Task 6: Session lookup (`--resume`, `--continue`)

**Files:**
- Create: `internal/session/find.go`
- Test: `internal/session/find_test.go`

**Interfaces:**
- Produces:
  - `func Find(dir, ref string) (string, error)` — `ref == "last"` → newest file by mtime; else the file whose name ends in `-<ref>.jsonl` (`ref` must be 8 hex chars); not found → error listing the 5 most recent ids.
  - `func FindForWorkdir(dir, workdir string) (string, error)` — newest file whose first entry's `Session.Workdir` equals `canonical(workdir)` (`filepath.Abs` + `EvalSymlinks`); reads only the first line of each file; not found → `no session for <workdir>; start one with moca`.

- [ ] **Step 1: Write failing tests**

```go
// internal/session/find_test.go
package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFind(t *testing.T) {
	dir := t.TempDir()
	w1, _ := Create(dir, Header{Workdir: "/a"}, "one")
	w1.Close()
	time.Sleep(20 * time.Millisecond)
	w2, _ := Create(dir, Header{Workdir: "/b"}, "two")
	w2.Close()
	if p, err := Find(dir, "last"); err != nil || p != w2.Path() {
		t.Fatal(p, err)
	}
	if p, err := Find(dir, w1.ID8()); err != nil || p != w1.Path() {
		t.Fatal(p, err)
	}
	if _, err := Find(dir, "deadbeef"); err == nil || !strings.Contains(err.Error(), w2.ID8()) {
		t.Fatal("not-found lists recent ids:", err)
	}
	if _, err := Find(dir, "../x"); err == nil {
		t.Fatal("ref must be 8 hex chars")
	}
}

func TestFindForWorkdirViaSymlink(t *testing.T) {
	dir, base := t.TempDir(), t.TempDir()
	real := filepath.Join(base, "real")
	os.Mkdir(real, 0o755)
	canon, _ := filepath.EvalSymlinks(real)
	link := filepath.Join(base, "link")
	os.Symlink(real, link)
	w, _ := Create(dir, Header{Workdir: canon}, "x")
	w.Close()
	Create(dir, Header{Workdir: "/elsewhere"}, "y")
	if p, err := FindForWorkdir(dir, link); err != nil || p != w.Path() {
		t.Fatal(p, err)
	}
}
```

- [ ] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/session/find.go
package session

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var id8re = regexp.MustCompile(`^[0-9a-f]{8}$`)

type fileInfo struct {
	path string
	mod  int64
}

func listNewestFirst(dir string) []fileInfo {
	ents, _ := os.ReadDir(dir)
	var fs []fileInfo
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			fs = append(fs, fileInfo{filepath.Join(dir, e.Name()), fi.ModTime().UnixNano()})
		}
	}
	slices.SortFunc(fs, func(a, b fileInfo) int { return cmp.Compare(b.mod, a.mod) })
	return fs
}

func idOf(path string) string {
	b := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	return b[max(0, len(b)-8):]
}

func Find(dir, ref string) (string, error) {
	fs := listNewestFirst(dir)
	if ref == "last" {
		if len(fs) == 0 {
			return "", fmt.Errorf("no sessions in %s", dir)
		}
		return fs[0].path, nil
	}
	if !id8re.MatchString(ref) {
		return "", fmt.Errorf("session id must be 8 hex chars or \"last\", got %q", ref)
	}
	for _, f := range fs {
		if idOf(f.path) == ref {
			return f.path, nil
		}
	}
	var recent []string
	for _, f := range fs[:min(5, len(fs))] {
		recent = append(recent, idOf(f.path))
	}
	return "", fmt.Errorf("no session %s (recent: %s)", ref, strings.Join(recent, ", "))
}

func header(path string) (*Header, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	line, err := r.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil || e.Session == nil {
		return nil, fmt.Errorf("%s: no session header", path)
	}
	return e.Session, nil
}

func FindForWorkdir(dir, workdir string) (string, error) {
	abs, _ := filepath.Abs(workdir)
	canon, err := filepath.EvalSymlinks(abs)
	if err != nil {
		canon = abs
	}
	for _, f := range listNewestFirst(dir) {
		if h, err := header(f.path); err == nil && h.Workdir == canon {
			return f.path, nil
		}
	}
	return "", fmt.Errorf("no session for %s; start one with moca", canon)
}
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/session && git commit -m "feat(session): find sessions by id8, last, or workdir"`

---

### Task 7: `agent.Resume`

**Files:**
- Create: `internal/agent/resume.go`
- Modify: `internal/agent/start.go` (extract `build`)
- Test: `internal/agent/resume_test.go`

**Interfaces:**
- Produces:
  - `func Resume(o StartOptions, path string) (*Agent, error)`. It opens the writer, reads the entries, and appends `session.Repair(entries, session.Interrupted)`. The header's `SystemPrompt` is reused verbatim, and the header's `Workdir` becomes the jail root (`o.Workdir` is ignored). The model and effort are the latest `model_change`, else the header's. When `o.Model` is set, `SetModel(o.Model, effort)` runs after construction and writes a `model_change`. Totals are restored from prior entries, and `NewSnapshots(w, blobDir, entries)` rebuilds the undo stack.
  - `type Resumed struct { ID8 string; Messages int }` event, emitted once (the TUI prints `resumed <id8> (<n> messages)`).
  - Refactor: `Start` and `Resume` share `build(o StartOptions, jailRoot string, w *session.Writer, system string, model string, effort llm.Effort, prior []session.Entry, snapshotDir string) (*Agent, error)`. It constructs the registry, jail, shell, env and snapshots (`NewSnapshots(w, snapshotDir, prior)`); `Start` passes `filepath.Join(config.DataDir(), "snapshot")`.
- Yolo (§7.5) comes **only** from `o.Yolo` (flag/config of the resuming process), never from the header or `permission_mode` entries, which are audit-only. `build` applies it via `applyYolo` when `o.Yolo` is set.
- A missing provider key does **not** fail `Resume` (keys are lazy). The first `Run` fails with the `EnvError`, and `-p` maps that to exit 2.

- [ ] **Step 1: Write failing tests**

```go
// internal/agent/resume_test.go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func TestResumeRebuildsExactContext(t *testing.T) {
	s := newScript(t, textTurn("first answer"), textTurn("second answer"))
	a, work, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), "first question")
	a.SetEffort(llm.EffortLow)
	path := a.Session().Path()
	sys := a.opts.System
	a.Session().Close()

	b, err := Resume(StartOptions{Config: a.opts.Config, Workdir: "/ignored", HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	if b.opts.System != sys || b.Effort() != a.Effort() || b.opts.Env.Root != a.opts.Env.Root {
		t.Fatal("system prompt verbatim, effort restored, workdir from header")
	}
	b.Run(context.Background(), "second question")
	msgs := s.bodies[1]["messages"].([]any)
	if msgs[0].(map[string]any)["content"] != sys || len(msgs) != 4 {
		t.Fatalf("resumed request = stored system + full history + new prompt: %v", msgs)
	}
	_ = work
}

func TestResumeAfterCrashMidTool(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	// simulate a crash: assistant message + tool_use persisted, no result
	m, _ := a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant}})
	a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: m.ID, Call: llm.ToolCall{ID: "c1", Name: "shell"}}})
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := session.ReadFile(b.Session().Path())
	last := entries[len(entries)-1]
	if last.ToolResult == nil || last.ToolResult.Content != session.Interrupted {
		t.Fatalf("%+v", last)
	}
}

func TestResumeNeverRestoresYolo(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	a.SetYolo(true)
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err != nil || b.Yolo() {
		t.Fatal("resume starts with checks on unless the resuming process asks for yolo", err)
	}
	b.Session().Close()
	c, _ := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client(), Yolo: true}, path)
	if !c.Yolo() {
		t.Fatal("--yolo on resume applies")
	}
}

func TestResumeHonoursCompaction(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTestWith(t, s, "", "")
	u1, _ := a.append(session.Entry{Type: session.TypeMessage, Message: userText("old")})
	a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "old answer"}}}})
	u2, _ := a.append(session.Entry{Type: session.TypeMessage, Message: userText("kept")})
	a.append(session.Entry{Type: session.TypeCompaction, Compaction: &session.Compaction{Summary: "SUMMARY", FirstKeptEntryID: u2.ID}})
	_ = u1
	path := a.Session().Path()
	a.Session().Close()
	b, _ := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	b.Run(context.Background(), "new")
	body := mustJSONString(s.bodies[0]["messages"])
	if !strings.Contains(body, "SUMMARY") || strings.Contains(body, "old answer") {
		t.Fatal(body)
	}
}
```

(`mustJSONString` is a small helper in the test file: `json.Marshal` → string.)

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement** the `build` extraction (move the jail/shell/skills-dirs/env/snapshot/registry construction out of `Start`; `Start` still builds the prompt and creates the session) and `resume.go`:

```go
// internal/agent/resume.go
package agent

import (
	"fmt"
	"path/filepath"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

type Resumed struct {
	ID8      string
	Messages int
}

func (Resumed) isEvent() {}

func Resume(o StartOptions, path string) (*Agent, error) {
	w, entries, err := session.Open(path)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || entries[0].Session == nil {
		w.Close()
		return nil, fmt.Errorf("%s: not a moca session", path)
	}
	for _, fix := range session.Repair(entries, session.Interrupted) {
		e, err := w.Append(fix)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	h := entries[0].Session
	model, effort := h.Model, llm.Effort(h.Effort)
	for _, e := range entries {
		if e.Type == session.TypeModelChange {
			model, effort = e.ModelChange.Model, llm.Effort(e.ModelChange.Effort)
		}
	}
	a, err := build(o, h.Workdir, w, h.SystemPrompt, model, effort, entries,
		filepath.Join(config.DataDir(), "snapshot"))
	if err != nil {
		w.Close()
		return nil, err
	}
	if o.Model != "" && o.Model != model {
		if err := a.SetModel(o.Model, effort); err != nil {
			return nil, err
		}
	}
	a.emit(Resumed{ID8: w.ID8(), Messages: len(session.Messages(entries))})
	return a, nil
}
```

`build` sets `anchorValid = false`, so the first estimate after resume is chars/4 of the rebuilt request (§6). `New` already restores totals from `Prior`.

- [ ] **Step 4: Run** — `go test ./internal/agent/ -race -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent
git commit -m "feat(agent): resume with stored prompt, model restore, crash repair"
```

---

### Task 8: CLI + TUI wiring

**Files:**
- Modify: `cmd/moca/main.go`, `cmd/moca/oneshot.go`, `cmd/moca/tui.go`, `internal/tui/app.go`
- Test: `cmd/moca/cli_test.go` (extend)

**Interfaces:**
- `--resume <id8|last>` and `--continue` are mutually exclusive (usage error, exit 2). `--continue` uses `session.FindForWorkdir(DataDir()/sessions, cwd)`; `--resume` uses `session.Find`. Lookup failures → exit 2 with the message.
- `-p` with resume: `agent.Resume(...)` then `Run(prompt)`.
- TUI with resume: `tui.AppOptions.ResumePath string`; `tui.Run` calls `agent.Resume` instead of `agent.Start` when it is set. The `Resumed` event prints `resumed <id8> (<n> messages) — earlier output is in the session file`. (Inline scrollback from the earlier process isn't replayed; that's by design, because committed history is the terminal's.)
- TUI `/compact` → `agent.Compact(ctx)`, run as a `tea.Cmd`. On `ErrNothingToCompact` print `nothing to compact`. It is refused while a run is in progress.
- `Compacted` event → print `⋯ compacted: <before> → <after> tokens`.

- [ ] **Step 1: Write failing CLI test**

```go
func TestResumeAndContinueOneShot(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		answer := "first"
		if n > 0 && strings.Contains(string(b), "remember 42") {
			answer = "you said 42"
		}
		n++
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", answer)
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(t.TempDir())
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	run(context.Background(), []string{"--config", cfg, "-p", "remember 42"}, nil, &out, &errb)
	out.Reset()
	if code := run(context.Background(), []string{"--config", cfg, "--continue", "-p", "what did I say?"}, nil, &out, &errb); code != 0 || out.String() != "you said 42\n" {
		t.Fatalf("code %d out %q err %q", code, out.String(), errb.String())
	}
	if code := run(context.Background(), []string{"--config", cfg, "--resume", "zzzzzzzz", "-p", "x"}, nil, &out, &errb); code != 2 {
		t.Fatal("bad id → exit 2")
	}
	if code := run(context.Background(), []string{"--config", cfg, "--resume", "last", "--continue", "-p", "x"}, nil, &out, &errb); code != 2 {
		t.Fatal("exclusive flags")
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement.**
  - In `parseArgs`: reject `Resume != "" && Continue` as a `usageError`.
  - In `runOneShot`: compute `resumePath` (via `session.Find` / `FindForWorkdir`; errors → exit 2). If it is set, use `agent.Resume(opts, resumePath)`; otherwise `agent.Start`. Everything else is unchanged.
  - Mirror the same lookup in `runTUI` and pass `ResumePath`.
  - In `internal/tui/app.go`:
    - replace the phase-3 `/compact` stub;
    - add the `Compacted`/`Resumed` event lines;
    - in `Run`, branch on `o.ResumePath`.

- [ ] **Step 4: Run** — `go test ./... -race` → PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/moca internal/tui
git commit -m "feat(cli): --resume/--continue for -p and TUI; /compact"
```

---

### Task 9: Phase gate

- [ ] **Unit gates**: `go test ./... -race -count=1` → PASS. These cover:
  - token-threshold compaction shrinks context with no orphaned tool result (`TestAutoCompactionShrinksContext`);
  - a 32K-window custom model compacts without looping, and the guard names an oversized entry (`TestLoopGuardNamesOversizedEntry`);
  - provider overflow → one compact-and-retry (`TestOverflowCompactsAndRetriesOnce`);
  - resume + `--continue` rebuild context including compaction entries (`TestResumeHonoursCompaction`, `TestResumeAndContinueOneShot`);
  - a session killed mid-tool resumes with a synthetic result (`TestResumeAfterCrashMidTool`).
- [ ] **Live: long session.** In the TUI on a real repo, ask moca to read ~15 large files one by one and summarize each. Watch the status bar % climb, then `⋯ compacted: N → M tokens` appear. Continue the conversation and ask about the first file: the answer draws on the summary. Inspect the session file: one `compaction` entry, with `<read-files>` listing the files.
- [ ] **Live: small local model.** With a `vllm`/Ollama custom provider at `contextWindow: 32768`, repeat a shorter version. Compaction happens with no loop and no provider overflow error.
- [ ] **Live: kill mid-tool.** Start `!`-free work that runs `shell sleep 60`, `kill -9` the moca process, then `moca --continue`. The resumed banner appears, and the next request succeeds (the session file shows the `interrupted — moca exited before completion` result).
- [ ] **README + commit**: status `phase 4 done — compaction + resume`; `git commit -m "docs: phase 4 gate passed"`.

---

## Implementation notes (added after implementation + gate runs)

Deviations from the task snippets, all deliberate; the plan's intent is kept.

1. **`FindCut` refuses when the whole live window fits `keepRecent`** — the plan's cut algorithm (smallest valid index whose kept suffix fits) makes a two-entry session "compactable" by summarizing its opening user message, which contradicts Review Focus #2 and `TestManualCompactNothing` ("a couple of short messages" → `nothing to compact`). Pi's cut search defaults to the window start there (nothing before it to summarize); moca reproduces that observable behavior directly: `suffix(0) <= keepRecent` → `ok=false`. Pinned by the third case in `TestFindCutNothingToDo` and the extra case in `TestCompactNothing`. Consequence: overflow recovery on a session with nothing older than `keepRecent` surfaces the original provider error (`TestOverflowRecoveryFailsWithoutHistory`).
2. **Test sizing had to be reworked against the real caps** (all in `internal/agent/compaction_test.go`):
   - `bigToolTurns(5)`, not 6: with the measured system+tools ≈ 1.6k tokens, five 20k-char shell results (≈5k tokens each) put the estimate just over the 24576 trigger *after the last batch*, which is where the scripted summary response must be consumed.
   - The plan's raw-string shell command (`head -c 20000 /dev/zero | tr '\0' x`) produced invalid JSON (`\0` is not a legal JSON escape), so the call was persisted as `{}` and the tool returned "empty command"; the test now uses a properly escaped string literal (`'\\0'` in the JSON text).
   - `TestOverflowCompactsAndRetriesOnce` uses 80K-char prompts: with the plan's 4K the whole window (~3k tokens) fits `keepRecent` (16384) and the new `FindCut` refusal above fires; 80K gives three real turns so the cut lands on a turn boundary and exactly two summaries (history + split turn) run.
   - `TestLoopGuardNamesOversizedEntry` uses a 50K-char `read` of a big file on a 16K-window model: shell output is capped at 30K chars (≈7.5k tokens), too small to exceed the 24.6k trigger even kept whole; a `read` caps at 50K chars (≈12.5k tokens), and for the guard to fire after keeping the newest batch verbatim the trigger must be below that — window 16384 (the config minimum) gives trigger 12288 < ~14k.
3. **Concurrency hardening beyond the plan** (phase-3 discipline): `Compact` updates totals/anchor under `a.mu`; `liveEntries` snapshots the transcript under the lock; `Resume` closes the writer on every error path.
4. **`session.Messages` fallback**: when a compaction's `firstKeptEntryID` is not found (trimmed/corrupt file) the rebuild starts at the compaction entry itself instead of silently replaying summarized history.
5. **TUI `/compact` guards**: a `compacting` flag (like `shellBusy`) refuses runs, `/clear`, `/model`, `/effort`, `/hard`, `/yolo` and a second `/compact` while the manual compaction is in flight — it appends to the transcript, so concurrent mutation is not safe; the plan only said "refused while a run is in progress". `⋯ compacted` clears the `compacting…` transient; failures print `error: …`.
6. **`Agent.Workdir()` accessor**: the TUI adopts a resumed session's own workdir (status bar, `!` shell, branch lookup) instead of the invoking cwd.
7. **`-p` resume wiring**: `resolveResume` (cmd/moca) is shared by `-p` and the TUI; a bad id / missing workdir session prints the lookup error and exits 2 before any agent work.

Gate evidence (2026-10-05, real OpenCode key, small declared window — `contextWindow: 32768` on glm-5.3-flash as the local-model stand-in):

- **TUI long session (tmux, real repo)**: sequential read prompts of three large plan files; status bar climbed `4% → 56%` then **`⋯ compacted: 31199 → 14776 tokens`** and **`⋯ compacted: 26009 → 8312 tokens`** appeared as the estimate crossed 24576; the context field dropped (`56% → 33%`); after both compactions the model correctly listed all three files read, in order; the session file holds 2 `compaction` entries with **cumulative** `readFiles` (second = phase-1 + phase-2) and structured summaries (Goal/Constraints/Progress/…); cumulative usage $0.0139.
- **Loop guard (live, unshrinkable batch)**: a `-p` run where the model batched both 50K-char reads into one assistant turn: compaction summarized what it could (opening prompt) and the guard stopped the run with `context still over budget after compaction (26869 > 24576 tokens); largest entries: tool_result for read("path":"a.txt") ≈ 12k tokens; tool_result for read("path":"b.txt") ≈ 12k tokens` — calls and results stay together by design; the error names the entries. Exit 1.
- **Kill mid-tool + `--continue` (tmux)**: `shell sleep 90` approved and running, `kill -9` on moca, `--continue` → banner `resumed 6a836638 (3 messages) — earlier output is in the session file`; the follow-up request succeeded from the rebuilt context; the session file shows exactly one synthetic `tool_result` — `interrupted — moca exited before completion` (isError) — for the orphaned call.

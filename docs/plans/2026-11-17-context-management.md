# Context management: analysis and enhancement plan

Status: superseded by `2026-11-17-context-management-v4.md` · 2026-11-17
Scope: `internal/compact`, `internal/agent` (loop, compaction, elision), `internal/session`, `internal/config`, `internal/tui`, `test/evals`.

## 1. Goals and definitions

The request:

1. Context stays in a safe band: **75% of the effective window** by default, with
   **80% as the hard ceiling** (not the operating point).
2. When it must shrink, compact **smartly** — lose as few decision-relevant details as possible.
3. Be as good as, or better than, **OpenCode** and **Pi** at context management.
4. Achieve it with **minimal additional token usage**.

Definitions (the trigger rationale is in §13):

- **Context** = the estimated *input* tokens of the next request: stored system prompt +
  tool schemas + rebuilt messages (plan/goal envelopes included) + image/attachment
  overhead. It excludes the response budget.
- **Effective window** = `min(model.ContextWindow, context.maxWindowTokens)` (default
  300,000; `0` = the advertised window). The cap is relative to this: a 1M-token
  capability is not a 1M-token working set.
- **Trigger** = `min(floor(0.75 * effective), effective - reserve)`, where
  `reserve = min(context.reserveTokens, effective/4)`. The 80% ceiling is a
  safety bound, not the point where compaction normally starts.
- **Fixed-overhead exception** = when the system prompt + tool schemas alone exceed the
  trigger; the run warns once and proceeds below `effective - maxOutput`, never fails
  merely because the band is mathematically unreachable.
- **Additional tokens** = billed input/output tokens including summary calls, extra
  cache writes caused by history rewrites, and the token price of pruned/stubbed
  content. A change that saves context but forces an expensive cache rewrite is not
  automatically a win.

The invariant the implementation must hold:

> At every request boundary the estimate is at or below the trigger, and never above
> 80% of the effective window unless the fixed-overhead exception applies. Provider
> overflow still has one compact-and-retry path, but it is the safety net, not the
> normal trigger.

## 2. Sources and method

- Read the actual implementation: `internal/compact/*`, `internal/agent/compaction.go`,
  `elision.go`, `control.go`, the run loop in `agent.go`, `session/rebuild.go`,
  `session/entry.go`, config defaults/validation, TUI status/`/compact`.
- Compared against:
  - **Pi** (`earendil-works/pi`, `packages/coding-agent/docs/compaction.md`, current
    `main`, read 2026-11-17): trigger
    `contextTokens > contextWindow - reserveTokens` (default 16384),
    `keepRecentTokens` 20K, structured summary, previous-summary merge, split-turn
    two-pass summary, cumulative read/modified file lists, tool results truncated at
    2000 chars, thinking omitted, `NoCacheWrite`, `/compact [instructions]`.
  - **OpenCode** (`sst/opencode` → `anomalyco/opencode`,
    `packages/opencode/src/session/compaction.ts`, commit `e521fee0`, read
    2026-11-17): overflow when `input + cache.read + output > (limit.input ||
    context - limit.output)`; deterministic prune protecting the newest 40K tokens of
    tool output and pruning older completed outputs when the saving exceeds 20K,
    skipping the `skill` tool and requiring at least 2 user turns; summarization by a
    dedicated `compaction` agent with a free-form "detailed prompt for continuing"
    prompt and a plugin override point.
- Where the source was ambiguous the text below says so; nothing is asserted beyond
  what those sources and moca's own code show.

## 3. What moca does today

### 3.1 Measurement

- `Agent.ContextTokens` (`internal/agent/control.go:291`) is **usage-anchored**: after
  a turn, the anchor stores `input + cacheRead + cacheWrite + output` from the provider
  usage plus the entry count (`agent.go:549-552`); later entries are estimated at
  `chars/4` and added to the anchor.
- When the anchor is invalid (after a compaction or an elision, or when a provider
  reports no usage), it falls back to `chars/4` over the whole request
  (`control.go:299-309`, `compact/estimate.go:7`).
- `chars/4` is a crude estimator: code, JSON, non-Latin text and tool-call framing are
  denser (≈3–3.5 chars/token), and there is no per-message overhead. The fallback is
  exactly the state right after every compaction, i.e. when the trigger check matters most.

### 3.2 Trigger and budget

- `compact.NewBudget(window, reserve, keepRecent)` clamps both knobs to `window/4`
  (`internal/compact/budget.go:11-13`).
- The trigger is `window - reserve` (`budget.go:15`). With the defaults
  (`reserveTokens: 16384`, `keepRecentTokens: 20000`, `internal/config/config.go:520-524`):

  | Window | Reserve (min 16K, W/4) | Trigger | Fill at trigger |
  |---:|---:|---:|---:|
  | 16K | 4K | 12.0K | 75% |
  | 32K | 8K | 24.0K | 75% |
  | 64K | 16K | 48.0K | 75% |
  | 128K | 16K | 112.0K | 87.5% |
  | 200K | 16K | 184.0K | 91.8% |
  | 1M | 16K | 1008.0K | 98.4% |

  So the original 80% goal is met only on windows up to 64K; common windows compact at
  87–98% — after the damage the goal is trying to prevent.

### 3.3 Where compaction is checked

`run()` (`internal/agent/agent.go:671-889`):

```
run()
 ├─ maybeCompact()                       # once, BEFORE the new prompt is appended
 ├─ append user prompt
 └─ step loop:
     ├─ turnWithRecovery()               # one request + stream
     │   └─ overflow / length-with-full-input → Compact() once → retry the turn
     ├─ execute the tool batch, append every result
     ├─ apply steering
     └─ maybeCompact()                   # after a complete batch
```

`maybeCompact` (`compaction.go:196-219`) first elides superseded results
(non-forced, ≥8K saved), then checks `Budget.Over`. It compacts once, and if the
estimate is still over the trigger it **fails the run** with a "largest entries"
error. Provider overflow is treated as `errOverflow` (`agent.go:485-495`) and gets one
compact-and-retry attempt (`agent.go:588-599`).

Consequences:

- A newly pasted prompt is not measured before its first request; a huge paste can
  overflow before the guard sees it.
- A tool batch can add several large results (read: up to 50K chars ≈ 12.5K tokens
  each, `internal/tools/read.go:18-31`; shell: 12K chars; web: 20K) and cross the trigger in
  one step. The guard runs before the *next* request, but the fixed overhead, the
  kept tail, or the batch itself can leave nothing compactable → loop-guard failure.
- The `length` stop is only treated as overflow if input ≥ `window - reserve`, which
  is the 87–98% trigger above, not the 75% operating trigger.
- Steering queued on a no-tool turn takes the `continue` path before the end-of-loop
  `maybeCompact` (`agent.go:740-747`), so the next request is the first one with no
  guard at all.

### 3.4 What a compaction does

`Agent.Compact` (`compaction.go:134-170`) → `compact.Compact`
(`internal/compact/summary.go:50-88`):

1. `liveEntries` returns the transcript after the latest compaction's first kept
   entry, with previously elided results stubbed.
2. `FindCut` walks backwards and chooses the first entry of the kept suffix whose
   tokens fit `keepRecent`; the cut lands only on user/assistant messages
   (`cut.go:11-43`). If the cut splits a turn, two summary calls are made: prior
   history and the current-turn prefix, merged under `## Current turn (in progress)`.
3. The previous summary is prepended as "Previous summary (update it, don't drop
   facts)".
4. The span is serialized lossily (`serialize.go:59-100`):
   - tool results: **head 2000 chars only**, tail dropped, marker `[… truncated]`;
   - tool args: each value capped at 200 chars;
   - user and assistant text: full;
   - assistant thinking: **included in full**.
5. The summarizer is `config.model` (the session's configured default — a `/model`
   switch does not move it, and `/hard` never did), with
   `MaxTokens=min(4096, model.MaxOutput)`, `Effort=minimal`, `NoCacheWrite`
   (`compaction.go:101-129`). The input cap is
   `(window - maxTok - len(SummarySystem)/4 - 512) * 3` chars; a summarizer overflow
   halves it once.
6. The summary uses the Pi section format (Goal, Constraints, Progress, Key Decisions,
   Next Steps, Critical Context) plus cumulative `<read-files>` / `<modified-files>`
   (`summary.go:12-23`, `serialize.go:102-146`).
7. The compaction entry stores summary, `firstKeptEntryId`, `tokensBefore`, usage,
   file lists (`session/entry.go:67-74`). Then a **forced** elision stubs every stale
   result left in the kept window, because the cache is reset anyway
   (`compaction.go:163-167`, `elision.go:94-120`).

### 3.5 Elision

- `elideMinTokens = 8000` (`elision.go:22`): stubbing stale results rewrites history
  and forfeits the prompt cache from the first changed message, so the pass only runs
  when the estimated saving is at least 8K tokens (unless forced after a compaction).
- A result is stale when the same call runs again later (same tool + normalized input)
  or a file read earlier is rewritten whole by `write` (`elision.go:26-77`).
- Decisions are persisted as `type: elision` entries so resume rebuilds
  byte-identical requests (`session/rebuild.go:17-31`).
- Preserved-thinking routes (`HistoryBound`) never take an unforced elision
  (`elision.go:94-102`).

### 3.6 Rebuild and resume

`session.MessagesWith` (`rebuild.go:63-120`) renders the latest compaction as a user
message `"[Summary of earlier conversation]\n<summary>"`, then entries from
`firstKeptEntryId`; thinking before the compaction boundary is stripped unless the
route requires it. The active goal is re-injected after a compaction if no kept entry
carries it (`agent/goal.go:243-293`). The transcript is append-only: elisions and
compactions are entries, never edits.

## 4. Gaps against the four goals

### 4.1 "Never above 80%" — now the hard ceiling

| Gap | Consequence |
|---|---|
| No percentage trigger; `window - reserve` is 87–98% on 128K+ windows | The stated goal is violated on every common large window |
| No effective-window cap | A 1M-token model can run 800K+ tokens of context: quality degradation, huge summary requests, unnecessary cost |
| No pre-request guard after the new prompt is appended | A large paste overflows before any check; recovery is provider-error-driven |
| No batch-level budget | Several 12.5K-token reads in one batch can overshoot the trigger in a single step |
| `chars/4` fallback, no calibration, no safety margin | The fallback used right after compaction can undercount dense code/JSON by 15–30% |
| Post-compaction loop guard fails the run instead of degrading | Fixed overhead (system + tools) over the trigger makes long sessions unfixable |
| Length-stop overflow is recognized only at `window - reserve` | A request at 85% that truncates is not treated as overflow |

### 4.2 "Smart compaction; lose as few details as possible"

| Gap | Consequence |
|---|---|
| Tool results are cut at the **head** 2K chars; the tail is discarded | Test failures, stack traces, `go vet` output and command errors live at the tail — exactly what the next turn needs |
| Historical assistant thinking is included in the summary payload | Large token cost for reasoning that is often re-derivable; Pi omits it |
| `CapChars` trims the **oldest** part of the summary request, and the previous summary is prepended to that payload | On a long history the previous summary — the cumulative memory — is the first thing dropped |
| Only file lists are carried deterministically | Commands run, exact failing errors and verified state rely entirely on the summarizer, which truncates |
| No artifact/state block; no spill pointers for pruned output | Once a tool result is elided, the full text is unreachable from the request even though moca already spills over-cap outputs to disk |
| No `/compact <focus>` | Users cannot say "keep the migration details" (Pi has it) |
| Summarizer is `config.model`, not a dedicated cheap model | On a single-model setup summaries run on the most expensive model, at the worst time |
| Summary is only validated by "is the estimate now lower" | No check that the rebuilt request contains the summary, the kept tail and the goal |

### 4.3 "As good as or better than OpenCode / Pi"

moca already has the Pi *shape* (the code says "Pi's proven shape"): usage anchor,
cut on turn boundaries with split-turn two-pass summaries, previous-summary merge,
cumulative file lists, `NoCacheWrite`, extension of `keepRecent` on small windows.
That puts it at parity with Pi on the core algorithm and arguably ahead of OpenCode
on summary structure. The verified differences that matter:

| Capability | moca today | OpenCode | Pi |
|---|---|---|---|
| Trigger | `window - reserve` → 87–98% on 128K+ | `input+read+output > limit.input \|\| context-output` (near the window edge) | `window - reserve` (same formula as moca) |
| Deterministic prune before the summary | Duplicates + read-then-write only, ≥8K, **after** the summary | Aged tool outputs: protect the newest 40K, prune when >20K saved, ≥2 user turns, **before** the summary | No generic aged-output prune in the documented flow; context-edit omissions are a separate mechanism |
| Summary shape | Pi sections + previous summary + split-turn current turn | Free-form "detailed prompt for continuing", dedicated `compaction` agent, plugin override | Pi sections + previous summary + split-turn current turn |
| Tool output in the summary | Head 2K chars only, thinking included | Prune-dependent (tool truncation detail not in the read source) | 2K chars, thinking omitted |
| Cumulative file lists | Yes | Not in the read source | Yes |
| `/compact <instructions>` | No | Plugin can replace the prompt; no documented user focus | Yes |
| 80% cap | No | No | No |

Where both competitors are better: OpenCode's prune protects the working set and runs
before the token-expensive call; Pi's serialization is cheaper (no thinking) and
Pi recalculates `tokensBefore` from the rebuilt projection. Where moca can be
strictly better: the 80% cap, a deterministic state block (commands/errors) that does
not depend on the summarizer, spill pointers that make pruning reversible, and a
calibrated estimate instead of `chars/4`. None of those require a model call.

### 4.4 "Minimal additional tokens"

This is the finding that should drive the design, not an afterthought:

- **Mid-run elision at 8K is probably a net loss on cached providers.** Stubbing
  rewrites the prefix, so the next request writes a new cache for the remaining
  context instead of reading it. Rough model: remaining 150K tokens, cache write
  ≈1.25× input vs cache read ≈0.1× → one rewrite costs ≈150K×1.15 ≈ 172K
  token-equivalents; saving 8K per request yields ≈0.8K token-equivalents per request
  at cache-read price. Payback ≈ 200 requests — longer than most sessions. Only the
  **forced** elision after a compaction is unambiguously free, because the cache is
  being reset anyway.
- **Summarizing before pruning** pays for stale tool output twice: once in the
  summarizer input and again when the forced elision runs after the summary. The
  forced elision should happen *before* serialization when a compaction is already
  required.
- **Head-only truncation** makes the summary input no smaller than head+tail at the
  same cap but loses more useful content: a pure loss.
- **Including thinking** raises the summarizer input with little decision value;
  dropping historical thinking is a direct token win.
- **No spill pointers** means a stubbed result can only be recovered by re-running a
  tool (fresh tokens) or re-reading a file (fresh tokens). Pointing at the existing
  spill file costs ~30 tokens and recovers full detail on demand.
- **Compaction at 98%** means the summary request processes up to the whole window;
  at 75% with the same `keepRecent` the span is smaller, and deterministic pruning
  before it shrinks further.

## 5. Target design

Five layers, ordered by cost-to-value. Layers 0–2 are the core; 3–4 are hardening.

### Layer 0 — measurement that cannot silently undershoot

- Keep the provider-usage anchor as the primary truth.
- Learn **chars per token** for the session at zero token cost: record the serialized
  request character count before each turn, divide by the provider's input tokens
  (`input + cacheRead + cacheWrite`) after it. Clamp the ratio to `[2.5, 4.0]` and
  apply a 5% conservative margin when projecting. Use the measured ratio for the
  anchored delta and for the full fallback; reset to 3.5 on model switch.
- Add fixed per-message overhead in the cold fallback (≈4 tokens per message plus 3
  per tool call/result); keep counting tool schemas, plan/goal envelopes and the
  summary message in the character total as `RequestChars` already does.
- If a completed request's actual input exceeds the pre-request estimate by more than
  10%, lower the session ratio by 10% (clamped) and log a metadata-only event. This
  makes the estimator self-correcting when the content mix changes.
- Expose the estimator source (`anchor`, `calibrated`, `cold`) in the diagnostic log
  and a debug status field; never in the prompt.

### Layer 1 — the model-aware trigger

- `compact.Budget` gains `Effective` and `Trigger` fields:
  ```
  effective = min(model.ContextWindow, context.maxWindowTokens)   // 0 = no cap
  reserve   = min(context.reserveTokens, effective/4)
  trigger   = min(floor(compactAtPercent/100 * effective), effective - reserve)
  ```
- New config: `context.compactAtPercent` (default **75**, valid `1..99`, `0` = legacy
  `effective - reserve`) and `context.maxWindowTokens` (default **300000**, `0` = the
  advertised window). `NewBudget` gains both; update `Agent.Budget()` and the TUI's
  direct construction (`internal/tui/app.go:364-366`). The rationale and trigger table
  are in §13.
- The provider's hard limit still uses `model.ContextWindow`: the effective window
  governs *when to compact*, not what the provider accepts. A length stop with input
  ≥ `model.ContextWindow - reserve` remains the overflow path.
- Kept tail:
  `keepRecentEff = min(max(configured, min(effective/10, 60K)), effective/4)`. On a
  200K effective window that stays 20K (Pi parity); on a 300K-capped 1M model it grows
  to 30K so a compaction does not throw away the recent working set. The knob remains
  user-overridable; the formula is only a floor for large windows.
- Add the guard at **every request boundary**:
  1. run start, before the prompt (existing);
  2. immediately after appending the user prompt (new — catches pastes);
  3. after each tool batch and steering (existing);
  4. after overflow recovery (existing retry path);
  5. a cheap assertion/log right before `turn()`: if the estimate still exceeds the
     trigger, check whether the overage sits in the newest entries; if so use the trim
     path, otherwise log an estimator/guard defect before sending.
- Post-compaction verification: rebuild the actual request (summary + kept tail +
  plan/goal envelopes) and re-estimate. If it is still over the trigger:
  1. force-prune all aged/stale candidates (spill pointers), re-estimate;
  2. compact once more with `keepRecent/2` (bounded to one extra pass);
  3. if fixed overhead alone is over the trigger, warn once with the overhead number
     and proceed if `estimate <= effective - maxOutput`; otherwise fail with the
     largest-entry diagnosis (today's error, but only in the genuinely impossible case).
- Hysteresis: record the post-compaction estimate. Do not start another compaction
  until the estimate has grown by at least `max(2K, 5% of the effective window)`; if a
  compaction reduced the estimate by less than 5% of the effective window, warn once
  and stop automatic compaction for the rest of the run (manual `/compact` still
  works). This prevents the compact-fail-compact loop.
- TUI: the percent and the trigger are relative to the **effective** window, not the
  advertised one (otherwise a 300K-capped 1M model shows 22% at its trigger). Yellow
  from 60% (or `compactAt - 15` if that is higher), red at the trigger; the existing
  70% band moves accordingly. Show `used/effective` in the status tooltip or `/cost`.

### Layer 2 — prune before you pay for a summary

Extend `elision` into a general deterministic **prune** pass:

- **Protect** the newest `protectRecent` tokens of tool output (default
  `max(20K, min(keepRecent, 40K))`), all results from the last two user turns, every
  latest result per call/path, and all error results (older duplicate errors may be
  stubbed).
- **Candidates**: (a) superseded results (today's rule), (b) aged completed results
  older than the protected suffix. `read` results stay eligible even when the file was
  not touched again: OpenCode prunes them, and the spill pointer plus the file on disk
  make a re-read cheap and explicit.
- **Spill before stub**: if the result is larger than the stub threshold and has no
  spill file, write the full text to `<data>/overflow/<session-dir>/…` (0600, 0700)
  and make the stub name the path, line count and size: "full output saved to … — read
  it with offset/limit". This makes pruning detail-preserving in the same way the
  tool caps already are.
- **Persist** one `type: elision` entry with IDs, saved tokens and an optional
  per-ID stub map (additive JSON field; old sessions keep the constant stub). Extend
  `session.ElidedIDs`/`liveEntries` to use it so resume is byte-identical.
- **Schedule**: run the forced pass only when a compaction is already required (the
  cache reset is already paid) or immediately after one. Mid-run, only prune when the
  saving is large enough to justify a rewrite: `saved >= max(25K, 20% of estimate)`
  **and** the provider has a prompt cache with a read discount; when it does not, a
  rewrite is cheap and pruning may run more freely. Keep the existing `HistoryBound`
  restriction. Re-evaluate the current 8K threshold: the default should move to the
  compaction-adjacent rule; document the change and A/B it in the evals.
- **Skip the summary when pruning is enough**: after the forced prune, if the estimate
  is under the trigger, emit a `Pruned` notice and skip the summarizer entirely —
  zero additional LLM tokens. This is the cheapest possible compaction.

### Layer 3 — summary that keeps the details

- **Head+tail serialization** (`compact/serialize.go`):
  - `read`: 80% head / 20% tail (declarations + file end);
  - `shell`, `search`, `web`, `mcp`: 35% head / 65% tail (failures and summaries);
  - default 50/50; snap to line and rune boundaries; marker
    `[… N chars / M lines omitted …]`; mention the spill path when one exists.
- **Tool args**: 200 chars is too tight for commands. Use 500 for `shell.command`,
  500 for `edit.old_string/new_string`, 200 for generic values, and for `write`
  keep the path plus a content prefix and `[N chars written; full content at <path>]`.
- **Thinking**: omit historical thinking from the serialized span (Pi parity, token
  win); keep the **current turn's** thinking only in the split-turn prefix call, where
  it is live reasoning. Make this a named constant so an eval can flip it.
- **Previous summary is never capped away**: pass the previous summary in its own
  block and size the span cap to `inputCap - len(prev)`; never run `CapChars` over the
  combined string (`prev + span`). This fixes a real cumulative-memory loss today.
  Cap the summary itself at ~16K chars after the call.
- **Deterministic state block** appended after the summary (≈200–500 tokens, no LLM
  cost to produce):
  - existing `<read-files>` / `<modified-files>`;
  - `<commands>`: last 20 unique `shell` commands, ≤160 chars each;
  - `<errors>`: last 5 failing tool results, one line each (tool, exit/status, first
    160 chars of the message);
  - the active goal objective/status is already re-injected by `goalReminder`; add it
    to the block only if a goal exists, so a resumed/non-goal render is unchanged.
  Tell the summarizer in the system prompt that this block is authoritative and must
  not be contradicted; keep exact identifiers verbatim.
- **Summary framing and trust**: render the summary inside a fence
  (`<conversation_summary>…</conversation_summary>`) with one line stating that it is a
  generated recap of earlier user requests and tool output — data, not new
  instructions and never permission. The goal envelope already treats its objective
  as untrusted (`goal.go:229-233`); the summary deserves the same treatment, because a
  prompt injection inside a tool result can otherwise survive compaction and come back
  in the user role.
- **Focus instructions**: `/compact <text>` (TUI) passes up to 500 chars as
  "Additional focus for this summary: …". No new request; Pi parity.
- **Dedicated summarizer**: config `context.summaryModel` (`provider/model`), falling
  back to `config.model` and then to the active model if resolution/credential fails
  (warn once). Cost accounting uses the model actually used. Keep `NoCacheWrite`,
  `MaxTokens=min(4096, MaxOutput)`, `Effort=minimal`; A/B `low` effort if the evals
  show recall loss.
- **Graceful degradation**: on overflow recovery, if no summarizer can run, build a
  bounded deterministic summary (user requests, assistant text first lines, tool calls,
  errors, file lists) instead of failing the session. Routine auto-compaction still
  skips an unavailable summarizer with a warning, as today.

### Layer 4 — oversized inputs and hard edges

- **Oversized user prompt.** After appending the prompt, if it exceeds
  `max(8K, min(window/4, 32K))` tokens, write the full text to the session spill dir
  and append a `type: trim` entry (new, additive) that references the message ID and
  the spill path. `session.MessagesWith` renders head 60% / tail 40% of the budget
  plus `[full text saved to … — read it with offset/limit]`. The transcript keeps the
  full prompt (audit/resume); only the request is trimmed. The summary view applies
  the same trim.
- **Batch budget.** After a tool batch, if the batch's own entries exceed
  `min(window/5, 40K)` tokens and compaction cannot cut them (they are the kept tail),
  trim the largest results with spill pointers instead of failing. This is a last
  resort behind the normal guard and prune.
- **Spill lifetime across resume.** Today the overflow dir is per process and random
  (`internal/agent/start.go:555-567`); the jail admits only the current process dir
  (`start.go:162-165`), so a stub path written by yesterday's process is dead after
  resume. Make the dir session-scoped (store it in the session header or derive it
  from the session id), add it to the jail on resume, and fall back to the generic
  stub when a file is gone (the transcript still holds the original content). This
  is required for spill pointers to be trustworthy, and it keeps concurrency
  isolation (one dir per session).
- **Cache-aware rewrite rules** are part of the scheduler above: no history rewrite
  without either a compaction in the same guard or a saving that pays for the rewrite.
- **Degenerate summaries.** If the summarizer returns text that does not contain at
  least three of the required section headings, or is shorter than 200 chars, retry
  once with the same payload plus "Return the full structured summary"; if it fails
  again, fall back to the deterministic summary. Prevents a junk summary from
  silently replacing the history.

## 6. Token and cost accounting

Per compaction, the design adds at most:

| Item | Tokens | Notes |
|---|---:|---|
| Deterministic prune | 0 LLM | Up to ~30 tokens per stub with a spill path |
| Summary request input | previous summary (≤4K) + capped span | Smaller than today: prune first, head+tail, no historical thinking, previous summary kept separate |
| Summary output | ≤ min(4K, model max output) | Existing cap; hard character cap added |
| Deterministic state block | ~200–500 | Commands + errors + files |
| Post-compaction verification | 0 LLM | Rebuild + estimate only |
| Estimator calibration | 0 extra | Reuses provider usage already billed |
| `/compact <focus>` | 0 extra requests | Same call, extra instructions |
| Spill write | 0 LLM | Disk only; recovered on demand |

What it removes:

- the second, post-summary forced elision when it could have run before the summary
  (the summary input shrinks by the pruned volume);
- historical thinking from the summary payload (a large and variable share of the
  serialized span);
- the head-only truncation that spends the same 2K chars on less useful content;
- needless mid-run cache resets (the largest hidden cost today);
- a summary call entirely when pruning alone gets under the trigger.

## 7. Configuration surface

Strict decode means every key is a commitment: defaults, validation, SPECS §4,
`internal/config/testdata/example.jsonc` (byte-exact fixture) and a config test.

| Key | Type | Default | Validation | Purpose |
|---|---|---|---|---|
| `context.compactAtPercent` | int | 75 | 0 or 1–99 | Operating trigger as a percent of the effective window; 0 = legacy `effective - reserve` |
| `context.maxWindowTokens` | int | 300000 | 0 or ≥ 16000 | Effective working-window cap; 0 = the advertised window |
| `context.summaryModel` | string | `""` | `provider/model`, provider must exist | Dedicated summarizer; empty = `model`, then the active model |
| `context.reserveTokens` | int | 16384 | ≥1 (existing) | Output + next-batch headroom; still clamped to `effective/4` |
| `context.keepRecentTokens` | int | 20000 | ≥1 (existing) | Verbatim kept tail; adaptive floor `max(configured, min(effective/10, 60K))`, still capped at `effective/4` |

Deliberately **not** added: a tokenizer dependency (tiktoken et al.), a target/hysteresis
percentage key, and per-tool prune knobs. The calibration layer and the fixed
hysteresis cover them with no extra surface.

### Implementation contracts

These are the decisions an implementer would otherwise have to guess:

- **Budget.** `compact.Budget{Window, Effective, Reserve, KeepRecent}` plus the
  existing `Trigger()` method; `NewBudget(window, maxWindow, pct, reserve, keepRecent)`
  clamps to the effective window. `Agent.Budget()` reconstructs it per call, so a model
  switch re-evaluates the effective window; the TUI reads `Effective` for its percent
  and bands, not `model.ContextWindow`.
- **Session trim entry.** Additive `type: trim`:
  `Trim{IDs []string, Path string, HeadBytes, TailBytes int}`. `MessagesWith` renders
  those entries as the original head+tail (deterministic from the transcript, so
  resume does not depend on the spill file) plus a one-line path note; `liveEntries`
  uses the same view for the summary. `Messages` delegates to `MessagesWith`, so
  resume, TUI replay, `Agent.History` and ACP all see the trimmed view. Old sessions
  ignore the unknown type.
- **Spill dir.** Add `spillDir` to `session.Header` (additive), set at session start;
  `Start`/`Resume` admit it to the jail instead of a fresh random dir; if an old
  header has none, derive one from the session id. A missing file falls back to the
  generic stub; a dead path is never a request blocker.
- **Prune event.** `agent.Pruned{Tokens int}`; the TUI prints a notice only when a
  prune runs without a compaction; ACP keeps the existing compaction events.
- **Config.** `compactAtPercent` 0 disables the percentage trigger (legacy
  `effective - reserve`); `maxWindowTokens` 0 means the advertised window; both are
  validated before use, and the table above is the source of defaults.

## 8. Phased implementation

Each phase is a PR-sized unit, keeps `make vet && make test` green, and updates
`docs/specs/SPECS.md` in the same change (normative behavior).

### P0 — measurement + model-aware trigger (the goal-1 PR)

Files: `internal/compact/{budget,estimate}.go`, `internal/agent/{compaction,control,agent}.go`,
`internal/config/config.go`, `internal/tui/{app,status}.go`, fixture + SPECS.
Changes: calibrated estimator; `compactAtPercent` + `maxWindowTokens`; effective-window
math in `Budget`; guard after the prompt; post-compaction verification; fixed-overhead
exception; hysteresis; TUI percent/bands relative to the effective window; status shows
the estimator source in debug.
Tests: trigger table (16K–1M, cap on/off), small windows, 1M window, prompt-paste guard,
fixed overhead, hysteresis, calibrated ratio, provider undercount correction.

### P1 — prune before summarize (the goal-4 PR)

Files: `internal/agent/elision.go` (generalize), `internal/compact/serialize.go`,
`internal/session/{entry,rebuild}.go`, `internal/tools/overflow.go` (shared spill).
Changes: aged-output policy + protection; spill-before-stub; per-ID stub map;
compaction-adjacent scheduling; skip-summary-when-prune-suffices; `Pruned` event;
session-stable spill dir.
Tests: protection boundary, error retention, stale read pruning, spill fallback,
history-bound route, resume byte-identical, no rewrite when the save is small.

### P2 — summary fidelity (the goal-2 PR)

Files: `internal/compact/{summary,serialize}.go`, `internal/agent/compaction.go`,
`internal/config/config.go`, TUI command parsing.
Changes: head+tail; per-tool ratios; thinking policy; previous-summary cap fix;
state block; focus instructions; `summaryModel`; summary length/quality checks.
Tests: head+tail by tool, rune/line safety, prev summary survives a tiny cap,
thinking omitted/kept, state block contents, focus plumbing, model fallback.

### P3 — hard edges

Files: `internal/agent/{agent,start}.go`, `internal/session/*`, `internal/compact/cut.go`.
Changes: oversized-prompt trim, batch budget, degenerate-summary retry,
deterministic fallback summary, `tokensAfter` on the compaction entry, resume fallback
when a spill file is gone.
Tests: 100K-char prompt, 8×12.5K read batch, summarizer unavailable on overflow,
junk summary, resume after a spill, no-cut-with-one-huge-message.

### P4 — evals and docs

- New hermetic eval scenario `context` with a scripted provider: drive a synthetic
  session past the trigger, assert the maximum request input stays ≤75% of the effective
  window (never >80%; read from the scripted server's recorded requests), assert a
  compaction happened, and assert a planted fact (file path + exact error string)
  survives two compactions.
- A second arm where pruning alone is enough: assert **zero** summary requests.
- A third arm: after resume, the rebuilt request is byte-identical (hash comparison).
- Metrics: maximum fill %, summary input/output tokens, prune tokens, cache writes,
  number of summaries per 100K growth, fact-retention pass/fail.
- A/B against `test/evals/BASELINE.md` and the existing compare tooling; document the
  delta. Update README config docs, CHANGELOG, SPECS §4/§13/§14 (and the doc counts
  tests if the test count changes).

## 9. Acceptance criteria

The plan is done when all of these are measured, not asserted:

1. **Band.** In every eval arm (scripted and live), both the estimate and the actual
   request input recorded by the scripted server stay ≤ 75% of the effective window,
   with at most a 5% estimator tolerance, and never exceed 80%; or the fixed-overhead
   exception is logged and the request stays ≤ `effective - maxOutput`. A probe that
   only reads our own estimate does not count.
2. **Estimator accuracy.** After two anchored turns, the median absolute error versus
   provider input usage is ≤ 8%, p95 ≤ 15%. Cold start (before any usage) is within
   20% on a mixed code/prose fixture.
3. **Token cost.** At equal outcomes, total billed tokens per completed eval run are
   not higher than the baseline; summary input tokens per compaction are ≥ 25% lower;
   the number of summary calls per 100K tokens of growth does not increase.
4. **Cache discipline.** A run whose fill never exceeds 60% performs zero history
   rewrites. In a run that compacts, cache-write token spikes are no more frequent
   than the compactions plus any prune that cleared the conservative saving threshold.
5. **Detail retention.** Three planted facts — a file path, an exact failing-test
   error string, and a shell command that was verified — are present after two
   compactions, and in the live arm the model answers a question about each without a
   re-read.
6. **No false failure.** A run whose fixed overhead is over the trigger warns once
   and continues; the "context still over budget after compaction" error is reachable
   only when the estimate genuinely exceeds `effective - maxOutput`.
7. **Resume.** A resumed session rebuilds a byte-identical request to the one the
   previous process would have sent (hash of the serialized request), including
   pruning, trims and spill stubs.
8. **Gate.** `make vet && make test` clean; new tests race-clean; eval deltas recorded
   in `test/evals/BASELINE.md`; SPECS and config fixture updated.

## 10. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Calibrated chars/token drifts when content changes (prose → code) | Recalibrate every anchored turn, clamp, 5% conservative margin, log undercount events |
| the trigger is unreachable because system + tools are huge | Fixed-overhead exception with one warning; propose trimming tool descriptions or skills as a separate change |
| Summary cost on a 1M window at the trigger | Prune first, head+tail, no historical thinking, previous summary separate; measure tokens per compaction and add a summary-input cap only if the eval shows it necessary |
| Cache economics differ by provider | Compute the break-even from `provider.Model.CostOf` where prices are known; when unknown, require the conservative saving threshold |
| History rewrites break prompt-cache prefix binding | Never rewrite on `HistoryBound` routes unless a compaction is already required; forced passes only at compaction boundaries |
| New session fields break old readers | Additive JSON fields only; old elision entries use the constant stub; no schema version bump |
| Spill paths die across resumes | Session-scoped dir + jail admission on resume + generic-stub fallback; test resume after a spill |
| The trigger guard fires between a tool call and its result | The guard runs only at request boundaries (after the whole batch); never split a call/result pair |
| Goal/ACP paths get a different guard | `messagesLocked` is the one renderer; test the goal reminder and the ACP `compaction_update` path explicitly |
| Auto-compaction without a summarizer silently exceeds the trigger | Warn once, prune, and use the deterministic fallback only on overflow recovery |
| Summary junk replaces good history | Section/quality check, one retry, deterministic fallback, and the compaction entry keeps `tokensBefore`/`tokensAfter` for audit |

## 11. Non-goals

- A real tokenizer dependency (tiktoken, sentencepiece): the calibration layer plus a
  conservative margin is enough, and a dependency would break the single-binary /
  minimal-surface bet.
- Vector memory, embeddings or a retrieval store: extra tokens, extra cost, no
  evidence it helps a coding loop of this size.
- Branch summarization or session trees: moca has a linear append-only transcript.
- Extension/plugin hooks for compaction: moca's extension points are skills, prompt
  templates and slash commands, not a plugin API.
- A model-driven "choose what to keep" pass: it spends tokens to save tokens; the
  deterministic prune + structured summary is the cheap path.
- Changing the eight tool schemas or their caps: out of scope.

## 12. Decisions (resolved 2026-11-17)

All five review questions are settled; §13 holds the rationale.

1. **Band.** 75% of the effective window as the operating trigger, 80% as the hard
   ceiling, fixed-overhead exception. `effective = min(advertised window,
   maxWindowTokens)` with a 300K default.
2. **Mid-run elision.** Prune at compaction-adjacent boundaries by default; keep a
   ≥25%-of-context exception and let no-cache providers prune freely.
3. **Summarizer.** `context.summaryModel`, falling back to `model`, then the active
   model.
4. **Spill.** Extend the existing 0600/0700 overflow area to pruned results; make the
   directory session-stable so pointers survive resume.
5. **Scope.** P3 ships with P0–P2.

## Appendix A — file map

| Concern | Current code |
|---|---|
| Budget/trigger | `internal/compact/budget.go` |
| Token estimate | `internal/compact/estimate.go`, `internal/agent/control.go:291-309` |
| Cut point | `internal/compact/cut.go` |
| Serialization | `internal/compact/serialize.go` |
| Summary prompt/flow | `internal/compact/summary.go` |
| Compact orchestration | `internal/agent/compaction.go` |
| Elision/prune | `internal/agent/elision.go` |
| Loop/guard/overflow | `internal/agent/agent.go:474-495,588-599,671-889` |
| Session entries/rebuild | `internal/session/entry.go:67-81`, `rebuild.go:37-120` |
| Overflow spill | `internal/tools/overflow.go`, `internal/agent/start.go:555-567` |
| Config | `internal/config/config.go:452-474,520-524,695` |
| Status/`/compact` | `internal/tui/status.go:40-51`, `app.go:364-366,1064-1070,1238-1245` |
| Goal reminder | `internal/agent/goal.go:243-293` |

## Appendix B — target summary

Today: trigger at 87–98%, estimate at `chars/4` after every compaction, summary
first then prune, head-only tool truncation, thinking included, previous summary can
be capped away, failure when the rebuilt context is still over.

Target: trigger at `min(75% of the effective window, effective - reserve)`, with
`effective = min(advertised window, 300K)` by default; calibrated estimate with a
safety margin, deterministic prune before the summary (and no summary when pruning is
enough), head+tail serialization with spill pointers, historical thinking dropped,
previous summary protected, a deterministic state block, a dedicated summarizer, a
post-compaction verification pass with graceful degradation, and hard edges for
oversized prompts and batches.

The four goals map to the phases as follows: goal 1 → P0; goal 2 → P1 + P2; goal 3 →
all phases (the comparison in §4.3 is the checklist); goal 4 → P0 (calibration)
+ P1 (no needless resets/summaries) + P2 (smaller summary payloads).

## 13. Review decisions (2026-11-17)

This section records the rationale for the reconciled design in §1/§5/§7/§9.

### Trigger — model-aware, 75%

A fixed 80% is not a good trigger. Recommended:

```
effective = min(model.ContextWindow, context.maxWindowTokens)  // default 300_000, 0 = no cap
reserve   = min(context.reserveTokens, effective/4)
trigger   = min(floor(0.75 * effective), effective - reserve)
```

- 75% leaves room for the response budget, one tool batch and 10–15% estimator error,
  and keeps the model below the range where long-context quality visibly degrades.
- The absolute working-window cap is the lever for 1M-token models: a 1M window compacts
  at 225K by default (75% of 300K), not 750K or 984K. Most coding sessions do not need
  more; `maxWindowTokens: 0` opts into the full window.
- Examples: 16K→12K, 32K→24K, 64K→48K, 128K→96K, 200K→150K, 1M→225K.
- Configuration: `context.compactAtPercent` default 75; `context.maxWindowTokens`
  default 300000; hysteresis: compact well below the trigger, do not retrigger until
  the estimate grows by `max(2K, 5% of the effective window)`, and stop automatic
  compaction for the run if a pass frees less than 5%.
- Cost sanity: on a 200K window, 75% means a compaction roughly every 106K tokens of
  growth instead of 140K at 92%, but the summary span and the post-compaction cache are
  smaller; measured summary input per token of growth rises only ~5%, while the model
  never runs near the window edge.

### Mid-run elision

The explanation in §4.4 stands: elision stubs an earlier prefix, so the next request
pays a cache **write** on the remaining context instead of a cache **read**. With 150K
remaining, one rewrite costs ≈150K × (1.25 − 0.1) ≈ 172K token-equivalents; saving 8K
per request at cache-read price returns ≈0.8K per request → ≈215 requests to break
even. So prune only when a compaction is already resetting the cache, when the save is
≥25% of the remaining context, or when the provider has no cache discount. The forced
post-compaction pass is the free one; run it **before** the summary, not after.

### Summary model

`context.summaryModel` (new): resolve at call time; fallback `config.model`, then the
active model; warn once on fallback; bill the model actually used. This is the P2
design in §5/Layer 3.

### Spill policy

moca already spills over-cap shell/search/web/mcp outputs to
`<data>/overflow/<dir>/…` (0600/0700) and admits only that dir to the jail. The plan
extends the same mechanism to any result that pruning/elision stubs: write the full
text once, make the stub name the path/line count/size (~30 tokens), and let the model
`read` it on demand. No new data class — the transcript already stores the full
output — and no new access: the dir is per session, outside the workdir, chmod 0600,
and swept by the existing overflow prune. Make the dir session-stable (header/ID) and
admit it on resume; if a file is gone, fall back to the generic stub. This is the
cheapest way to keep detail recoverable without carrying it in every request.

### Scope

P3 ships with P0–P2 as decided.


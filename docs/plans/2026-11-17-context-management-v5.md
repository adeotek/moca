# Context management: implementation plan v5

Status: implementation in progress (P4) · 2026-11-18
Consolidates and replaces the earlier v1–v4 drafts, now removed from the repository.
v5 keeps v4's safety framing and resolves
the v4 review findings listed in §13. v4's tree citations were re-checked; new or
changed claims are cited where they matter.

Scope: `internal/compact`, `internal/agent`, `internal/session`, `internal/config`,
`internal/provider`, `internal/tools`, `internal/tui`, `internal/acp`, and
hermetic/live tests.

All normative behavior in this document must land in `docs/specs/SPECS.md` in the
change that implements it.

## 1. Goals, scope, and honest limits

1. Make 75% of the effective window the default **operating trigger** and enforce an
   80% estimated-input **ceiling** at every work-request boundary.
2. Preserve decision-relevant state across compaction: exact errors, verified
   commands, current files, active goals, and earlier constraints.
3. Recover pruned detail on demand without broadening file access or losing the
   original when a spill write fails.
4. Reduce or hold total billed cost relative to the current implementation; never
   exchange cheap context reduction for an expensive cache reset without measuring
   the trade.
5. Establish competitor parity **by measurement**, not by feature checklist: a P4
   benchmark with pinned Pi/OpenCode commits, the same model, workload and metrics.
   If it does not pass, the plan reports the gap; it does not claim parity.

Additional-token accounting includes summary requests, retries, cache reads and
writes caused by history rewrites, deterministic state metadata, standing-context
blocks, spill/stub prompt text, and recovery reads. Dollar cost and billed-token
volume are tracked separately: optimizing one can regress the other.

### Definitions

- **Effective window** limits the normal working set:
  `min(model.ContextWindow, context.maxWindowTokens)`. Default cap: 300,000 tokens;
  `0` selects the advertised window.
- **Reserve** is `min(context.reserveTokens, effective/4)`.
- **Operating threshold** is
  `min(floor(compactAtPercent/100 * effective), effective - reserve)`.
  `compactAtPercent` defaults to 75; `0` selects the reserve trigger
  (`effective - reserve`); nonzero values must be `1..80`.
- **Ceiling** is `floor(0.80 * effective)` regardless of the operating threshold.
- **Operating threshold is a trigger, not a post-condition.** Crossing it starts
  bounded reduction. After the bounded work the request may be accepted anywhere at
  or below the ceiling, with a pressure notice if it remains above the operating
  threshold. This is deliberate: a second compaction that frees little is worse than
  running at 76–80%.
- **Requested output** is the exact next request's `max_tokens`. It may differ from
  the budget reserve during truncation rescue and must be used for the physical
  capacity check, not `model.MaxOutput`.
- **Input view** counts request message text, tool arguments/results, tool schemas,
  transformed assistant text/tool calls, route-required reasoning/signatures, and
  request-only goal/plan envelopes. It excludes the output budget.
- **Context estimate** is the next-request input estimate produced from the fixed
  system blocks, schemas, persisted data, and request-only envelopes. No image or
  audio input exists in this version; ACP rejects those content types.
- **Persisted captured result** is the exact string stored on a `tool_result`. It can
  itself contain a truncation/omission marker. Never claim it is the original
  unbounded process output.
- **Audit projection** is the user-facing conversation view: full persisted user
  prompts, elision stubs (not expanded), trim/prune notices, and request-only
  metadata visible as annotations. It is deliberately different from the model-facing
  request view.
- **Additional tokens** are as described above; see the budget table in §7.

### Enforced invariant

For every work request (ordinary turn, continuation, nudge, wrap-up, and overflow
retry):

1. Preflight projects and measures the exact next request before any provider stream
   starts. `turn` cannot bypass it.
2. Input above the operating threshold initiates bounded prune → compact → trim /
   deterministic degradation.
3. Request input above the ceiling is never knowingly sent. Either reduce it or
   return a bounded `context cannot fit` error naming the largest projected entries.
   There is no log-and-send branch.
4. The fixed-overhead exception may exceed the 80% ceiling only when projected system
   blocks + schemas already exceed it and no reducible history remains. The run may
   lower the requested output only down to `minUsefulOutput` (default 256 tokens, or
   the model's minimum when higher); below that it returns the bounded error. For the
   exception, require `input <= effective - requested_output` with a useful
   `requested_output`, and always `input + requested_output <= model.ContextWindow`
   with positive room. Mutable history cannot declare itself fixed overhead.
5. The ceiling is an estimate-enforced product policy, not a claim that a character
   heuristic is an exact tokenizer. Provider overflow/length recovery remains a
   separate one-retry safety net. Live accuracy is measured against independent
   provider usage; heuristic uncertainty is never relabeled exact.

## 2. Current implementation and constraints to preserve

These are verified current-source facts, not target behavior:

- `compact.Budget.Trigger()` is `window - reserve`, with reserve and kept tail capped
  at `window/4` (`internal/compact/budget.go`). This approaches the full context
  window on large windows rather than 75–80%.
- **The usage anchor is not double-counting.** `turn()` persists the assistant message
  and tool-use entries, then sets `anchorTokens = UsageTokens(resp.Usage)` and
  `anchorEntries = len(a.entries)` (`internal/agent/agent.go:545-552`). The delta
  starts at that index (`internal/agent/control.go:299-309`), so the assistant
  response is included once, through the provider's output usage, and only later
  entries are added. `TestContextTokensAnchored` pins 15 after a turn and 25 after a
  10-token note (`internal/agent/control_test.go:200-214`).
  The real nuance: provider output usage can include hidden reasoning tokens that the
  next request does not replay, so the anchor can **overestimate**. That is
  conservative and stays until a measured correction is justified; this plan does not refactor the anchor merely to change its representation.
- `ContextTokens` falls back to character estimates after compaction, elision, or
  model changes. `MessageChars` does not include all wire signatures or protocol
  framing. Exact request-token reconstruction is provider/model-dependent.
- `maybeCompact` runs before the initial user prompt and after a tool batch.
  Append-and-continue paths can bypass it: `wrapUp` appends and calls
  `turnWithRecovery` directly (`agent.go:610-625`), each `nudge` appends and
  `continue`s, and the goal reviewer sends its own fitted request.
- Compaction uses head-only tool-result truncation, includes thinking, and can cap
  away a prepended previous summary. The previous-summary cap bug is a correctness
  issue independent of the feature work.
- `HistoryBound()` and `KeepsTailThinking()` are distinct contracts: preserving
  signature-bearing history is required on history-bound routes, while tail-thinking
  replay is a different provider need.
- Shell capture is bounded by design: `internal/tools/shellrun.go` keeps head and tail
  with an omission marker. Overflow storage preserves that captured string, not a
  recovered middle. `tools.Spill` returns `""` on failure, which is insufficient for
  persistence decisions.
- `prepare` allocates a random spill directory before the session header is created
  (`internal/agent/start.go:132-172`). Resume does not reuse a stored directory.
- `Agent.History` calls `session.Messages`; ACP and TUI replay use
  `Agent.History` (`internal/acp/history.go:169`, `internal/tui/app.go:1434`).
  Changing the shared default to trim model requests would also trim user-facing
  replay, which is forbidden.
- JSON session decoding is not strict. Go's decoder ignores unknown entry types and
  existing rendering skips them. Additive fields preserve reader compatibility, but
  an old binary reconstructs the untrimmed prompt; this is not full behavioral
  backward compatibility.
- `ProjectConfig` is a separate allow-listed struct (`internal/config/project.go`),
  not an arbitrary overlay of the user config.
- `maybeCompact` already errors when the context is still over budget after a
  compaction, naming the largest entries plus a fixed-overhead note
  (`internal/agent/compaction.go:214-234`); `contextCannotFit` is its bounded
  evolution, not a new concept.
- `maybeCompact` also warns and continues when the summarizer is unavailable
  (`summarizerUnavailable`, `compaction.go:208-210`) — deliberate work-model optimism
  today. §4.3 step 4 reverses it; that behavior change is intentional and must be
  tested, not preserved.

Preserve: append-only transcripts, the fixed-once system prompt, eight frozen tool
schemas and their behavior/caps, sequential complete tool batches, no call/result
splitting, per-session artifact isolation, cancellation and retry accounting, and
the import direction. Provider/tool-specific behavior must not move into the leaf
packages `session` or `compact`.

## 3. Request model and preflight

### 3.1 Three projections

One agent-owned projection layer, shared by `ContextTokens` and the request builder,
produces three explicitly named views; none silently substitutes for another:

| View | Contents | Consumers |
|---|---|---|
| **Raw entries** | The full append-only transcript, including full user prompts and full persisted tool results | Deterministic state accumulation, audit, migration |
| **Audit** | Full persisted user prompts (pre-trim), elision stubs (not expanded), trim/prune notices, request-only metadata as annotations | `Agent.History`, TUI replay, ACP replay |
| **Request** | Trimmed/stubbed history, request-only plan/goal envelopes, route-required reasoning/signatures | Model input and summary serialization only |

The audit view does **not** expand pruned tool output; the raw transcript is the only
full source. This keeps user-facing replay honest without unbounded re-expansion. A
trim notice names the original entry, the omitted extent and the artifact path when
one exists. `session.Messages` remains the audit default; a clearly named request
projection (for example `session.RequestMessages`) is added; `prepareRequest` is its only work-request caller (the summary serializer is its other caller, §4.5). Do not change `session.Messages` to trim globally.

### 3.2 Input view and calibration

Define a bounded internal `InputView` with per-protocol projection:

- message count and role sequence;
- text characters, tool-argument characters, tool-result characters;
- signature/reasoning characters, including replayed foreign reasoning;
- tool schema and system-block characters;
- route class (`plain`, `preserved-thinking`, `reasoning-replay`, `history-bound`);
- protocol-specific framing characters (anthropic content blocks/cache breakpoints,
  OpenAI chat messages/tool calls, Responses input items/reasoning).

No tokenizer dependency is introduced in P0. The estimate uses a session-local
chars-per-token ratio updated toward observed values, with these rules:

- never hard-code a ratio clamp such as `[2.5, 4.0]` chars/token — the current
  estimator has none (`internal/compact/estimate.go:8` is plain chars/4) and the new
  one must not gain one: a low observed ratio is data, not an error. Apply an
  explicit uncertainty margin instead of a floor;
- use a conservative cold-start rule for code/JSON/CJK/binary-derived text;
- include per-message/tool-call/schema overhead explicitly;
- reset model-specific calibration when the qualified model changes;
- treat missing or manifestly inaccurate provider usage as untrusted: fall back
  conservatively and log a metadata-only warning;
- never calibrate from the second post-compaction usage as if it measured the
  pre-compaction request;
- calibration is in-memory and session-local: a resume restarts cold, which the
  cold-start acceptance target (§9.4) already covers. Do not persist the ratio into
  the session header in this version.

Calibration records the same view before a turn and independent input usage
(`Input + CacheRead + CacheWrite`) after it. Preserve the richest usage the adapter
actually exposes; do not invent token counts for omitted fields. The anchor
semantics are in §6.2: the current total-usage anchor is not double-counting, and the
only known bias is the conservative hidden-reasoning overestimate.

### 3.3 One preflight, exact boundaries

`prepareRequest(choice)` is called immediately before every work stream, constructs
the request from a snapshot, runs the budget policy, and returns the final prepared
request. `turn` must not bypass it.

Covered boundaries:

- the first request after the initial prompt (preflight after prompt persistence when
  the prompt was appended since the last preflight);
- the first request after a resume: the rebuilt transcript is measured like any other
  request, and persisted trim/elision/compaction entries are replayed from the
  transcript, not re-decided (§9.9);
- every normal tool-following turn;
- every continue caused by steering, truncation, empty output, plan, verify,
  execute-plan, progress, or record nudges;
- step-limit and stuck wrap-up;
- the immediate retry after overflow compaction;
- large tool-batch reduction, before the next request.

Never elide or trim between a tool call and its matched result. A complete batch may
be reduced after its final result, before the next request; §5.5 also allows reducing
after result N, before call N+1 within a batch.

Normative policy shape:

```
prepareRequest(choice):
    snapshot = entries + model + exact requestedOutput + effort
    req = project(snapshot)
    P = policy(effective, threshold, ceiling)

    if req.input <= P.threshold:
        return req

    req = prune(req, P)
    if req.input <= P.threshold:
        emit Pruned
        return req

    req = summarizeOrDeterministic(req, P)   // at most one summary attempt family
    if req.input <= P.threshold:
        return req

    req = retryCompactionIfUseful(req, P)    // at most one, only if progress exists
    if req.input <= P.threshold:
        return req

    req = trimOversizedContent(req, P)       // only while over the threshold
    if req.input > P.ceiling:
        req = pruneProtectedIfNeeded(req, P)  // §5.2 rule 4, at most once
    if req.input <= P.ceiling:
        if req.input > P.threshold:
            emit ContextPressure(req.input, P.threshold, P.ceiling)
        return req

    if fitsFixedOverheadException(req, requestedOutput, advertisedWindow):
        warnOnce(FixedOverhead, measuredOverhead, remainingHistory)
        return req

    return error(contextCannotFit, largestProjectedEntries)
```

Rules:

- Every reduction step is conditional; a successful summary must not trigger an
  unnecessary retry or trim.
- `retryCompactionIfUseful` runs only when the first attempt left a compactable span
  and the retry is expected to reduce the estimate; no identical repeat at the same
  boundary with no new entries.
- Returning between the operating threshold and the ceiling is allowed, with a
  pressure notice. The 75% threshold is a trigger, not a guaranteed target; the 80%
  ceiling is hard. This is the explicit resolution of the v2 ambiguity.
- `trimOversizedContent` never runs if the request is already under the threshold.
- `pruneProtectedIfNeeded` is §5.2 rule 4, the last reduction before the fixed-overhead
  exception: it may prune even protected entries with bounded head/tail and a verified
  spill pointer, at most once per boundary. It is not the ordinary `prune` (which runs
  above the threshold and protects the working set); it runs only when the hard
  ceiling is otherwise unreachable.
- The actual request is built from the same snapshot and policy result that was
  measured; no second, different projection is sent. "Measured" means the request as
  built — **after** `TransformHistory` (`internal/agent/agent.go:407`), because that
  is what reaches the wire; route-required reasoning must already be present in the
  view when it is measured.
- A manual `/compact [text]` is user-directed and unconditional: it runs its summary
  regardless of these thresholds. Preflight policy governs the requests the *run*
  makes, never the manual command.

### 3.4 Output budget and truncation rescue

- Compute `requestedOutput` from the turn's exact `max_tokens`, including any rescue
  override.
- If `requestedOutput >= model.ContextWindow` or there is no positive input
  allowance, fail request setup before streaming.
- The fixed-overhead exception may lower `requestedOutput` only to
  `minUsefulOutput` (default 256 tokens or the model's minimum, whichever is higher).
  Below that, return `contextCannotFit`.
- A truncation rescue that raises `rescueMaxTokens` must re-run preflight. If the
  raised output no longer fits the physical window or the effective capacity, cap it
  at the room actually available; if that is below `minUsefulOutput`, end the rescue
  with the bounded error rather than sending an over-capacity request.
- A rescue never bypasses the ceiling check and never logs-and-sends.
- The provider overflow/length safety net keeps its **physical** basis
  (`model.ContextWindow - Reserve`, `internal/agent/agent.go:486-487`): under an
  effective cap it is unreachable by a conforming send and exists only for
  provider-side growth or a badly underestimated request. Do not silently move it to
  the effective window.

### 3.5 Non-conversation calls

`reviewGoal`, `judgeCmd`, and the summarizer have different prompts and never mutate
the work conversation. They do not use `prepareRequest` unchanged, but each has an
explicitly fitted request and a safe no-provider-call fallback:

- **Goal reviewer:** measure its own transform and review prompt against the reviewer
  model, not the active model. If over budget, run a bounded local review projection
  (objective, goal status, recent tool evidence, deterministic state, relevant recent
  text). Never append a user turn or compaction solely to fit a review. If still too
  large, pause with the existing review-failed/compact guidance.
- **Permission classifier:** its payload already truncates user and command text.
  Validate its model's minimum budget and fail closed (`ask`) without a provider call
  when the fixed system/payload cannot fit.
- **Summarizer:** fit against its chosen model, including reasoning and output room,
  not the work model; use the bounded payload in §5.3.

Reviewer/classifier requests must not anchor `ContextTokens` or alter the work
conversation anchor. Their usage remains in global cost and token totals.

## 4. Budget, configuration, status, and contracts

### 4.1 Budget representation

```go
type Budget struct {
    Window     int // advertised model window
    Effective  int // min(Window, maxWindowTokens); 0 cap = Window
    Reserve    int // min(reserveTokens, Effective/4)
    KeepRecent int // adaptive floor, capped at Effective/4
    Percent    int // compactAtPercent resolved; 0 = reserve trigger
}
func (b Budget) Trigger() int  // Percent==0 → Effective-Reserve; else min(Percent*Effective/100, Effective-Reserve)
func (b Budget) Ceiling() int  // floor(0.80 * Effective)
```

- Keep `Trigger()` as a method; do not introduce a `Trigger` field with the same
  name. `NewBudget(window, maxWindow, pct, reserve, keepRecent)` resolves these fields
  once per call; `Agent.Budget()` rebuilds it so a model switch changes
  `Effective`/`Trigger` immediately.
- `Trigger()` must branch on `Percent == 0` before applying the closed form: with
  `Percent=0` the formula `min(floor(Percent/100*Effective), Effective-Reserve)`
  evaluates to 0 and would trigger on every request. The branch is normative and is
  pinned by the `compactAtPercent=0` test (§9.11); `Percent*Effective/100` is the
  integer floor for the nonzero path.
- The physical capacity check uses `model.ContextWindow` and the exact
  `requestedOutput`; it does not assume `model.MaxOutput` fits.
- If `requestedOutput >= Window` or there is no positive input allowance, fail
  request setup before the stream.
- The adaptive kept tail is
  `KeepRecent = min(max(configured, min(Effective/20, 60_000)), Effective/4)`.
  It is a floor, not permission to evade the ceiling when recent entries are large.
  `configured` is the existing `context.keepRecentTokens` (default 20 000). This
  formula preserves the 20K default for effective windows up to 400K (the default
  user experience on 128K–300K windows is unchanged) and grows the tail only above
  that — 30K at 600K, 50K at 1M — so very large windows do not discard the recent
  working set. It replaces `NewBudget`'s `window/4` cap with `Effective/4`; that cap
  only binds when `maxWindowTokens` shrinks the effective window. The change is
  documented in §4.2 and pinned in P0a;
  `internal/compact/budget_test.go:7-14` pins the old math and is rewritten there.
- If `Effective` is missing or zero, or below a safe hard minimum (4096), fail
  configuration with the effective values named; never divide by zero or send with a
  zero window. `config.MinContextWindow` (16 384) keeps its current role: `config.go:690` still
  rejects a user-declared `contextWindow` below it and discovery still assumes 16 384
  when the provider publishes none — this plan relaxes neither. The 4096 floor above
  therefore binds only on a payload-published window below 4096; a payload-published
  window in [4096, 16 384) is accepted as-is (discovery does not clamp it) and is
  governed by the fixed-overhead exception.

### 4.2 Presence-aware configuration

Add these user-context keys:

| Key | Default | Accepted values | Meaning |
|---|---:|---|---|
| `context.compactAtPercent` | 75 | 0 or 1–80 | `0` = reserve trigger on the effective window; nonzero = percentage |
| `context.maxWindowTokens` | 300000 | 0 or ≥ 16000 | `0` = advertised model window |
| `context.summaryModel` | `""` | empty or qualified `provider/model` | empty = configured `model`, not the active `/hard` model |

- Use presence-aware integer fields (pointers, later normalized to resolved values)
  so an explicit `0` survives defaults and reloads. A general zero-replacement
  default is insufficient.
- Reject negative values, percentages >80, and `maxWindowTokens` nonzero below
  16000; report the exact key.
- **`compactAtPercent=0` is not old behavior.** It means the reserve trigger on the
  effective window. True pre-v3 behavior requires `compactAtPercent=0` **and**
  `maxWindowTokens=0`; document that combination and test it.
- **Context keys are user-config only.** `ProjectConfig` remains the allow-listed
  struct in `internal/config/project.go` and does not gain a `context` section. Its
  strict decoder already rejects unknown keys; add a test proving a project file
  cannot select a billable `summaryModel` or a larger window. If project support is
  wanted later, it needs an explicit allow-list and trust review, not an implicit
  overlay.
- `keepRecentTokens` keeps its name, its positive-value validation and its default;
  only its resolved value changes, through the §4.1 adaptive floor and cap (documented
  there, pinned in P0a).
- The 80% ceiling itself is not configurable: it is product policy. The escape from a
  conservative estimate that refuses too early is raising `maxWindowTokens` (up to
  `0` = advertised window), which moves threshold and ceiling together; document that
  escape in §4.2 and the README.

### 4.3 Summary model resolution

1. `context.summaryModel` if nonempty.
2. Otherwise the session configuration `model` — the configured default, **not
   guaranteed cheap**; `permissions.auto.model` is the judge's cheap model, not this
   one.
3. If that model or its credential is unusable, fall back once to the active model
   with one warning; note that this may be the `/hard` model and cost more.
4. If neither is usable, use the bounded deterministic fallback when policy needs a
   reduction. Never skip a necessary automatic reduction merely to preserve work-model
   optimism.

Every attempt records usage/cost against the model actually used, including failed
retries and validation retries.

### 4.4 Status and ACP

- Do not change the meaning of `Status.Window` accidentally. Add
  `EffectiveWindow`, `ContextTrigger`, and a debug-only estimator-source field.
- The TUI percent and bands use `EffectiveWindow`: yellow from
  `max(60, compactAtPercent-15)`, red at the operating threshold. On a 300K-capped
  1M model, 225K is 75% of effective, not 22.5% of advertised.
- ACP `usage_update.size` stays the **physical advertised window** for wire
  compatibility. The TUI's percent is relative to the effective window; the two
  denominators intentionally differ and must be documented. No unknown field is
  added to the ACP payload. If a future ACP capability/extension is negotiated, it
  may carry `effectiveSize`; until then, clients see the physical window and moca's
  TUI shows the effective one. Add a capped-1M test for both.
- `Agent.History`, TUI replay, and ACP replay use the audit projection. The model
  request uses the request projection. No front end reads the request projection.

### 4.5 Implementation contracts

These are required so P0–P3 cannot diverge:

- **Projection API.** `session.Messages` remains the audit renderer.
  `session.RequestMessages(entries, render, policy)` (name TBD) renders the request
  view and is called only by `prepareRequest` and the summary serializer. Both share
  a projection struct; neither calls the other's renderer. Audit trim/prune notices
  are a fixed-prefix text block appended to the affected message (or a synthetic user
  message when the entry is not a message); the request view uses the trim window
  instead. This keeps `Agent.History`/TUI/ACP replay deterministic and testable.
- **Input view.** `agent.InputView{Protocol, Msgs, TextChars, ArgsChars, ResultChars,
  SignatureChars, SchemaChars, SystemChars, Route}` plus the estimated token count.
  Adapters expose only the framing facts they actually know; unknown fields are
  counted as characters with a conservative default, never invented as tokens.
- **Session header.** Add `SpillDir string json:"spillDir,omitempty"` (additive).
  New sessions choose it before the header; resume passes the stored value into
  `prepare`. Validate it as an absolute child of the overflow root; never admit an
  arbitrary header path to the jail.
- **Trim entry.** `TypeTrim = "trim"`; additive
  `Trim{IDs []string, Path string, HeadBytes, TailBytes int, Notice string}`.
  The request renderer replaces the named entries with a deterministic head+tail
  window (from the original transcript, not the artifact) plus a one-line notice.
  The audit renderer shows the original prompt plus the notice. Missing artifact:
  the notice says the artifact is unavailable; the request stays bounded. Old
  readers ignore the unknown entry type and reconstruct the untrimmed prompt.
- **Summary state.** Persist on the compaction entry as an additive
  `SummaryState *SummaryState` with: goal ID/status and constraint references,
  read/modified paths, read windows `(path, offset, limit)`, commands with exit and
  verification state, exact critical errors with artifact pointers, prior artifact
  pointers, decisions carried from the previous summary, omitted-item counts, and
  per-class quotas. The model-written prose remains separate from this deterministic
  block.
- **Standing constraints.** `SummaryState.Constraints []ConstraintRef`, where
  `ConstraintRef{EntryID string, Tokens int}` points at a **raw user entry** — never
  a tool result, generated summary, plan/goal envelope, or host nudge. The first
  genuine user prompt is always a candidate; the summary may propose additional refs
  from its Constraints section. Each ref is validated against the raw entry; an
  invalid or tool-derived ref is dropped. The re-injected host block uses the raw text
  up to the §7 budget, spills overflow to a validated artifact, and is deterministic
  from the refs. A generated summary can repeat a constraint as evidence but cannot
  introduce one with no raw user entry behind it, and the first-prompt fallback means
  a summary that forgets every constraint still cannot erase the original request.
- **Compaction telemetry (additive).** Add `TokensAfter`, `PrunedTokens`,
  `SummaryTokens`, `Fallback bool`, and `SkipReason string` to the compaction entry.
  They let `/cost`, the status line, resume and the evals report the actual reduction
  and fallback without re-deriving it; old readers ignore the fields.
- **Compact injection.** `compact` cannot import `tools` or `agent`. `Compact` accepts
  an injected `Spill func(entryID, content string) (path string, err error)` and an
  injected `State`/`Summarizer`; the agent owns artifact I/O and state extraction.
  A failed spill leaves the original content in the request and returns an error to
  the policy; no unrecoverable stub is committed.
- **Events.** `agent.Pruned{Tokens int}` for an ordinary prune that avoids a summary
  or a protected prune that avoids a `contextCannotFit` error; the TUI prints a notice
  only in those cases. `agent.ContextPressure{Input, Threshold, Ceiling}` when a
  request is accepted between the operating threshold and the ceiling; the TUI prints
  a one-line notice, and ACP keeps existing events. A prune-only change is not a
  compaction. `SkipReason` on the compaction entry is one of `threshold`, `ceiling`,
  `fixed-overhead`, `no-summarizer`, `fallback`, or empty.
- **Status.** Agent `Status` gains `EffectiveWindow` and `Trigger`; the TUI status
  keeps `Used` and gains `EffectiveWindow` (it already has `Trigger`). Existing
  `Window` field meanings stay unchanged. Front ends must not recompute the trigger
  or the percent denominator from the advertised window.

## 5. Safety-focused context reduction

### 5.1 Stable, validated spill artifacts

Use one immutable session-scoped directory, chosen before the session header is
created. Resume passes the header's stored `spillDir` into `prepare`; it does not
allocate a new random directory.

Validation on resume (reuse the jail, do not invent a parallel validator):

- the directory is an absolute child of the resolved overflow root; no `..`, symlink
  escape, or alternate spelling;
- the directory may not exist yet for a new session; create it with `MkdirAll(0700)`
  and validate the resolved path afterwards, not before;
- file targets remain beneath that directory after symlink evaluation; regular files
  only; parent 0700, files 0600;
- expected session identifier in the name when id-derived; trusted ownership and
  permissions;
- do not add arbitrary header-supplied directories to the jail. Older sessions with
  a random directory and no stored value get a generic degraded stub, never a silent
  remap to a new derived directory;
- lazy sessions whose ID changes on a filename collision retain the persisted pointer
  directory or use a specified pre-header materialization rule.

`tools.Spill` returns `(note string, err error)`, not an empty string that callers
cannot distinguish from “spilling off”. Rules:

- if a required spill fails, keep the original content in the request, emit a
  bounded warning, and do not commit a stub that claims recoverability;
- store the **persisted captured result** (as defined in §1), not an unbounded process
  output that moca never had. For shell captures, the middle is already omitted by
  `shellrun.go`; the artifact preserves that captured string and the omission
  marker;
- a previously valid artifact that is gone at resume gets a generic stub:
  “original artifact unavailable; the complete persisted capture may remain in this
  session's audit history, or re-run/re-read the source”;
- missing artifacts never block the request by reading the original file merely to
  render it;
- retention expiry is a documented lifetime limit, not a fidelity guarantee.

### 5.2 Deterministic prune policy

Pruning is per-entry and precedes summarization only when useful. Preserve
tool-call/result association and all provider-required signing/call/result rules.

Ordinary preference order:

1. protect the newest tool-output suffix, the most recent two genuine user turns,
   the latest result per distinct path/read window, and all errors;
2. prune stale/superseded entries with a verified pointer where safe;
3. prune older completed output beyond the protective cadence;
4. if still over the ceiling, prune even protected entries with bounded head/tail
   and a verified spill pointer, including the most recent error. The ceiling
   outranks any retention class; never claim an error vanished solely because it was
   expensive — keep its exact text in the deterministic state or artifact.

Definitions:

- **Last two user turns** means the last two genuine user prompts, notes, steering
  entries, or goal continuation messages. Host nudges do not count. The boundary
  advances in transcript order; future synthetic nudges cannot resurrect pruned
  history.
- **Latest-result identity** is the tuple `(canonical path, offset, limit)` for reads
  and `(canonical path)` for writes. A later rewrite does not invalidate an earlier
  read of an unrelated window; a failed later rewrite does not supersede a success.
  Normalize paths with `filepath.Clean`; normalize omitted `offset`/`limit` to the
  effective read window before keying.
- **Estimated saving** is `max(0, originalProjectedTokens - stubProjectedTokens)`,
  not raw bytes/4; the stub includes its full provenance text.

The candidates/scorer must operate on the actual route's view. A history-bound route
must retain the signature-bearing tail the provider needs; do not prune it away.
All spills happen before the entry identifying them becomes live request state.

### 5.3 Bounded summary input and cumulative memory

For the summary request:

- use head+tail windows around tool results, snapped to UTF-8 and useful line
  boundaries; choose the ratio by tool class (read head-heavy, shell/search/web
  tail-heavy) and unit-test it without changing tool schemas or runtime results;
- keep unbounded raw commands out of the summary; put exact full commands in bounded
  state metadata or a stable artifact;
- annotate omitted characters/UTF-8 width and any available artifact provenance;
- drop stale historical thinking from the summary request; separately preserve any
  current route-required reasoning in the reconstructed work request. Summary
  serialization is not permission to strip persisted reasoning;
- preserve the split-turn contract: when `FindCut` returns a turn start, summarize the
  history span and the in-progress turn prefix separately, merge them under a current
  turn section, and never drop the prefix. A cut with no valid turn start remains a
  single summary. The P0a deterministic fallback includes the same prefix;
- cap the previous summary **without** capping it away.

Use explicit units. Define
`summaryInputByteBudget = max(0, summarizerInputTokens - outputReserveTokens -
reasoningReserveTokens) * conservativeCharsPerToken`, where the ratio is the
cold-start conservative value (for example 3), not the work model's calibrated ratio
and not a fixed 4. Then subtract the blocks that share the summarizer request:

```
summarySpanBudget = max(0,
    summaryInputByteBudget
    - len(previousSummaryBlock)
    - len(systemPromptBlock)
    - len(currentTurnBlock)
    - len(focusInstruction)
    - safetyBytes)
```

If `summarySpanBudget == 0`, do not send an over-budget summary call; use the bounded
deterministic merge. If the previous prose summary is oversized, load the previous
raw metadata and deterministic state; downsample history without losing pointers.

Budgets:

- separate prose output from metadata; prose output may stay at 4K tokens only after
  reasoning headroom is defined;
- define a per-class metadata list budget and a discarded/omitted item count; large
  file lists get an overflow record when necessary;
- do not silently cut mid-section or mid-UTF-8; a hard post-output cut is not a
  fidelity strategy. If there is not enough summary space, choose the deterministic
  merge instead of truncating facts after generation.

Treat summary text as a generated recap, not system authority. Use escaped,
length-delimited, or otherwise robust serialization for untrusted delimiter text;
simple unescaped XML wrappers are not enough. Distinguish host instructions, generated
recap, earlier real user instructions, and untrusted tool/output facts. Tool errors
and commands preserve evidence but never gain authority to alter permissions,
credentials, workdir, or goals.

Retain genuine historical user constraints via raw entry locations or an explicit
bounded standing-construction record validated against those raw entries. The rebuilt
request re-injects a bounded authoritative constraints/standing-context host block
outside the model-written summary, with references to the raw entries. If a
constraint exceeds the block budget, preserve its complete text in a validated
artifact and inject its reference plus an omission notice; do not silently drop or
demote it. Generated summaries may repeat a constraint as evidence but cannot become
its only authority. Existing per-entry plan/goal envelopes remain authoritative.

Fresh `/compact <text>` is explicit user focus, not tool output: at most 500 Unicode
code points, passed only through its dedicated summary field, not persisted as an
unrequested conversation message.

### 5.4 Deterministic cumulative state and bounded fallback

Construct state from the **raw transcript**, not only the retained request window.
Merge it with prior deterministic state so fallback compaction cannot forget
everything before the previous kept boundary. Keep historical session state distinct
from current UI/provider state.

State includes at least:

- active goal ID, objective/status, continuation/resume status, and references to the
  raw objective/standing-constraint entries; host wording is request metadata, not
  generated summary text;
- known read/modified files and canonical read windows;
- commands with result evidence and a verified red/green state. **Signal:** use
  `tools.Result.IsError` plus the shell's `[exit N]` suffix; reuse
  `agent.Env.SkillProvenCmd` where it already records a proven command. Do not guess
  success from prose;
- failing tool results with exact error text within a prioritized critical-fact
  quota and a validated stable artifact for full text;
- prior artifact pointers/provenance and **decisions/next steps carried from the
  previous summary**, not “deterministically extracted decisions” (they are
  model-written; the state block carries them as a bounded, labelled recap);
- omission counts and per-class quotas.

Use bounded per-class quotas and a stable, inspectable priority order. Do not force
all file lists into a fixed 200–500-token budget. Persist a structured `SummaryState`
(§4.5) with explicit list/item budgets and a discarded/omitted count; large lists
overflow to a file when necessary. Assess quotas per effective window, not one
universal number. The most recent exact critical errors and active goal outrank
historical read-file listings; old read/write sets may compress to touched paths and
aggregate counts.

Fallback merge combines prior summary state and newly summarized raw entries (P0a
ships this merge without the first bullet's `SummaryState`, using the previous
compaction's prose summary when one exists — see the P0a deliver list):

- prior structured summary/state within quota;
- current raw user requests, bounded assistant excerpts, calls/results, errors;
- current goal, files, verification state, and important next steps;
- explicit omitted-history counts and artifact pointers.

Fallback text is marked generated, bounded, and never resurrects untrusted
instructions. Retry-loop accounting persists every spend before the fallback. Even a
no-history auto-prune must verify the request below the ceiling.

### 5.5 Oversized prompts and tool batches

- **User prompt:** trim when the prompt itself exceeds
  `min(Effective/4, 32000)` estimated tokens, or when adding it would exceed the
  operating threshold. Persist the full prompt and full text artifact **before** the
  preflight that precedes the initial request, then append a trim record with stable
  IDs. The transcript keeps the original entry regardless of the trim outcome. The
  request projection uses the smaller of the prompt budget and remaining ceiling
  allowance, approximately 60% head / 40% tail, never an arbitrary fixed substring.
  The audit projection shows the original prompt plus the trim notice.
- **Tool batches:** reduction may run immediately after each result is persisted and
  before the next call executes; it may also run after the complete batch and before
  the next request. It must never run between a call and its own result. Apply when
  projected live batch entries exceed `min(Effective/5, 40000)` or the threshold
  would otherwise be exceeded. Latest-batch trimming during preflight remains the
  fallback. Store the persisted captured result, not a “full output” claim; stubs
  carry counts and provenance and keep errors identifiable.
- If a single genuine user message plus the fixed provider-required head can never
  fit, return a clear `contextCannotFit` error without sending; do not loop
  compaction forever.

### 5.6 Cache economics and hysteresis

- Default prune points are compaction boundaries, where history must change anyway.
  Move the forced elision **before** the summary in that path, not after it.
- Mid-run pruning (a prune that gets the request under the threshold and **avoids** a
  summary) runs only when its estimated saving — the §5.2 definition, in estimated
  tokens, not bytes — is at least `max(25 000, 25% of the projected request input)`.
  A mid-run prune invalidates the prompt cache, so below that gate the honest move is
  to wait for the compaction boundary. If no-cache behavior is known, cheaper pruning
  may be permitted; absence of price metadata alone does not establish no caching.
- A prune that does **not** get under the threshold and is followed by a summary in
  the same preflight is compaction-adjacent and may be forced: the summary resets the
  cache anyway. The policy measures the prune's saving first; only the prune that
  avoids the summary pays the 25K/25% gate. This prevents charging the gate twice for
  one cache reset. Implementation: `prune` must either report the projected saving
  without committing or take a `willSummarize` flag; never persist a mid-run rewrite
  before the policy knows whether a summary follows.
- When prices are known, compute the break-even explicitly:
  `rewriteExtra ≈ remainingInput * (cacheWritePrice - cacheReadPrice)`,
  `savingPerRequest ≈ savedTokens * cacheReadPrice` (or the input price when the
  route has no cache), `payback = rewriteExtra / savingPerRequest`; prune mid-run only
  when `payback <= estimatedRemainingRequests` or the §5.6 saving gate is cleared.
  The old 150K/8K example does not justify a universal 20% or 25% rule; measurements
  decide. Unknown provider cache behavior defaults to conservative.
- Hysteresis only suppresses an identical repeated attempt at the same boundary while
  no new entries exist. Always reevaluate after new entries and always enforce the
  operating threshold or a valid fixed-overhead exception and the hard ceiling. If
  forward history cannot be reduced, stop retrying summaries and switch to
  trim/fallback/error.

## 6. Failure handling, validation, and accounting

### 6.1 Summary validation and failures

- Validate required section structure and decoded summary limits, never mere
  headings as proof of correctness. Required sections must be present for a
  model-generated summary; the deterministic fallback has its own structure. A
  **goal** section must be nonempty only when a goal is active; otherwise `none` is
  valid. Add a no-goal summary test.
- For a reasoning summary model, calculate thinking room separately from prose (as
  the goal reviewer does) to avoid a 4K no-prose response. If the model's output
  budget cannot hold reasoning plus a minimal prose summary, reduce the summary
  request or use deterministic fallback; do not send a request that can only return
  thinking.
- If the summary is cut off, fails structural validation, or returns no prose, one
  controlled retry may use the same safe payload. A retry is not required when
  deterministic fallback is the better token choice.
- Persist usage of every attempted call, including the actual model, successful or
  failed. Missing summary prose must not create a new compaction boundary. Only a
  complete compaction append and all its requested prune/trim projections may change
  the request anchor. Do not replace a usable previous summary after a failed new
  call; the previous summary state still exists and can feed fallback.

### 6.2 Anchor semantics (corrected from v2)

The current anchor is **not double-counting**:

1. `turn` persists the assistant message and tool-use entries, then stores
   `anchorTokens = input + cacheRead + cacheWrite + output` and
   `anchorEntries = len(entries)` (`internal/agent/agent.go:545-552`).
2. The next estimate adds only entries after `anchorEntries`
   (`internal/agent/control.go:299-309`), so the assistant response is not counted
   again; its token weight is already in the provider's output usage.
3. `TestContextTokensAnchored` pins this: 15 after a turn, 25 after a 10-token note
   (`internal/agent/control_test.go:200-214`).

The real, measured nuance is hidden reasoning: if a provider counts output tokens
that the next request does not replay, the anchor overestimates. That is
conservative. v5 keeps the current total-usage anchor unless P4 measurements show the
overestimate causes unacceptable premature compaction; a future input-only anchor
must add the provider's output usage for the response, not a `chars/4` estimate of
it. Do not spend P0 “fixing” a double count that does not exist.

Tests required: text-only, tool-call, replay-reasoning, and history-bound turns
assert each new entry appears once; a hidden-reasoning fixture documents the
conservative direction. Reviewer/summarizer/judge calls never anchor the work
conversation; their usage remains in global totals.

### 6.3 Failure accounting

- Every attempted summary, retry, judge, goal review, and spill action records its
  actual model/artifact outcome before control returns; failed calls are billed and
  persisted.
- A failed spill leaves the original in the request and cannot create a
  compaction/trim boundary that claims success.
- A `contextCannotFit` error names the effective window, trigger, estimated input,
  requested output, fixed overhead, and the largest projected entries. It is
  bounded and user-actionable (reduce prompt, raise window cap, switch model, or
  compact manually).
- Cancellation during compaction/prune leaves the transcript append-only and does
  not leave a stub without its artifact.

## 7. Token budget targets

These are design budgets, not measured claims; P4 validates or revises them.

| Item | Budget target | Notes |
|---|---|---|
| Deterministic prune | 0 LLM tokens | ~30–80 tokens per stub with provenance |
| P0a deterministic fallback recap | 0 LLM tokens; input ≤ min(2% of effective, 4K) | FindCut span replaced: prior prose, bounded newest excerpts, most-recent error, omitted counts |
| Summary input | previous summary (bounded) + `summarySpanBudget` | See §5.3; no cap-away |
| Summary output | ≤ min(4K, model max output) after reasoning reserve | Prose cap, separate metadata |
| `SummaryState` metadata | ≤ min(3% of effective, 8K, effective/8); target ~1–3% only where it fits | Per-class quotas, omitted counts |
| Standing-constraints block | ≤ min(2% of effective, 4K, effective/8); target ~1–2% only where it fits | Raw-entry refs, artifact fallback |
| Spill/stub text | ~30–80 tokens per pointer | On-demand recovery reads |
| Estimator calibration | 0 extra | Reuses provider usage already billed |
| Retry compaction | ≤1 per boundary | Only if progress exists |
| Recovery reads | On demand | Billed only when the model asks |

The plan must measure billed dollars and tokens separately: a change that reduces
prompt length but increases cache writes or recovery reads can be a regression.

## 8. Delivery phases

Each phase is independently reviewable, keeps `make vet && make test` green, updates
SPECS with its behavior, and includes failure-mode tests in the same PR. P0a–P3 are one release unit: merging P0a alone is not production-complete context management.
P4's contextgate must merge before any release that ships P0a–P3 — §9's acceptance
criteria are verified by that gate, and a release without it ships unverified
behavior; the live measurements and the competitor benchmark may follow inside the
same release as long as no release note claims parity or savings before they are
reported. No behavior PR requires a commit unless the user separately approves it.

Per-phase docs checklist (every phase that adds or removes Go files, or edits
SPECS/README/CHANGELOG/AGENTS.md, in the same PR): update AGENTS.md's opening
`N Go files across M internal/ packages` count — `cmd/moca/counts_test.go` runs in the
normal suite and fails on drift; re-copy the three
`internal/skills/builtin/self-knowledge/references/` mirrors of SPECS, README and
CHANGELOG (`TestSelfKnowledgeReferencesAreByteIdentical`); update
`internal/config/testdata/example.jsonc` (byte-exact SPECS §4 fixture) and
`internal/config/sample.jsonc` when config keys change; run
`go test -tags counts ./cmd/moca -run TestSpecsTestCount -v` when the test count
changes. Editing `docs/plans/` alone does not trigger the counts or mirror tests.
P4's docs list below is the behavior docs, not a substitute for this checklist.

### P0a — contracts, generic projection, hard ceiling

Files: `internal/agent/{agent,control,compaction}.go`,
`internal/compact/{budget,estimate}.go`, `internal/session/{entry,rebuild}.go`,
`internal/config/config.go`, `internal/tui/{app,status}.go`, `internal/acp/session.go`.

Deliver:
- projection contracts (§4.5): audit vs request, `InputView`, `Budget`, trim/spill
  types, compact injection;
- one `prepareRequest` at every work boundary (prompt, tools, nudges, wrap-up,
  overflow retry);
- presence-aware config, effective-window math, output floor, physical capacity
  check;
- generic calibrated estimator + conservative cold start; no hard 2.5 clamp;
  anchor contract documented and tested (no double count);
- ceiling enforcement and bounded `contextCannotFit`; fixed-overhead exception with
  `minUsefulOutput` and rescue refit;
- TUI effective-window status; ACP physical `size` decision;
- a deterministic reduction floor for ceiling enforcement, before any LLM summary
  exists: §5.4's fallback merge as it applies pre-`SummaryState` — `compact.FindCut`
  chooses the cut (never splitting a call/result pair), and the replaced span becomes
  a bounded deterministic recap (the previous compaction's prose summary when one
  exists, bounded excerpts of the newest raw entries, the exact most-recent error,
  omitted-entry counts), persisted as a compaction entry with no model prose and no
  `SummaryState`; input budget from the §7 fallback row. If `FindCut` splits a turn,
  the turn prefix (the opening user message through the cut) is part of the recap,
  never dropped as if it were old history. Commit the fallback only when it makes the
  measured request strictly smaller than the pre-fallback request and at or below the
  ceiling; otherwise return the bounded error instead of a recap that grows the
  request. No LLM call and no prune — those are P1/P2. Without this floor P0a can only
  hard-error at the ceiling, which the release unit must not ship.

Tests: every boundary; 75 default/80 max/0 reserve/99 rejection; `maxWindowTokens:0`;
`compactAtPercent=0` + cap combination; small window/large MaxOutput; anchor
text/tool/replay/history-bound; hidden-reasoning overestimate; missing/absurd usage;
exact fixed overhead; prompt paste; output floor; rescue refit; cannot-fit;
cancellation; cost accounting; TUI/ACP capped-1M; the `keepRecentTokens` floor —
20K preserved through 400K effective, 30K at 600K, 50K at 1M, `Effective/4` cap —
`refreshStatus` consuming the agent's trigger instead of
recomputing it from the advertised window; the P0a deterministic fallback recap
(cut validity, split-turn prefix, net reduction, most-recent error text, omitted
counts); `ContextPressure` emitted only for an accepted 76–80% request.

### P0b — per-protocol input view refinement (conditional)

Run after P0a's live accuracy report. Only implement adapter-specific framing where
the measured error exceeds the acceptance target. Files: `internal/provider` adapters
as needed. Do not add a tokenizer dependency. If the generic view meets the target,
P0b is a documented no-op rather than speculative code.

### P1 — deterministic prune and stable spill

Files: `internal/agent/{elision,start,resume}.go`,
`internal/session/{entry,rebuild}.go`, `internal/tools/overflow.go` and spill callers.

Deliver:
- persisted captured-result storage with durable success/failure, provenance and per-ID
  stubs;
- validated session `spillDir`, no fresh-random-dir mistake on resume; jail reuse;
- per-entry pruning, priority/degradation rules, actual projected savings, read-window
  identity;
- prune-first, skip summary if enough, cache-conservative mid-run policy;
- protected last-resort prune (§5.2 rule 4) after summary/trim, at most once per
  boundary, with verified spill before the stub;
- no-call/result splitting; between-call batch reduction allowed.

Tests: last-two-turn/protected boundary; protected-prune reach and once-per-boundary
behavior; `Pruned` event on the prune-only path; failed write; canonical read window;
many fresh/old errors; history-bound
forced vs non-forced; no stale rewrite below threshold; no-cache known/unknown; spill
failure preserves original; deleted artifact degraded stub;
outside/traversal/symlink rejection; unicode/content counters; resume byte-identical
with artifacts; no sibling-session artifact read.

### P2 — accumulated state and summary fidelity

Files: `internal/compact/{summary,serialize}.go`, `internal/agent/compaction.go`,
`internal/session/{entry,rebuild}.go`, `internal/config/config.go`, TUI summary focus.

Deliver:
- head+tail by tool class, exact omission counts, provenance, foreign reasoning
  distinction;
- `SummaryState` cumulative deterministic block, bounded quotas, fallback merging;
- exact critical errors/commands with artifact pointers; decisions carried from prior
  summary; standing-constraints block;
- previous-summary input reservation without cap-away; summary byte-budget equation;
- escaping/trust taxonomy and `/compact <focus>` in 500 code points;
- summary reasoning/output budget, structural validation (goal optional), one retry,
  actual-model accounting.

Tests: malformed/delimiter injection; prior constraints not demoted; first user prompt
retained when the summary omits constraints; tool/envelope refs rejected; malicious
prior summary not elevated; previous summary protected on tiny cap; exact long error
retained; >2000 files bounded; fallback retains older planted fact; split-turn summary
keeps the current-turn prefix; focus rune/byte
policy; no-goal summary; summarizer actual-model accounting; thinking truncation;
compaction telemetry (`TokensAfter`, `PrunedTokens`, `SummaryTokens`, `Fallback`,
`SkipReason`) survives resume;
no summary call when prune suffices.

### P3 — last-resort edges and compatibility

Files: `internal/agent/{agent,goal_review,judge}.go`, `internal/session/*`,
`internal/compact/cut.go`, front-end replay.

Deliver:
- oversized prompt spill/trim before first request; latest-batch trim; audit/request
  divergence;
- deterministic fallback when no summary model can run at any required boundary;
- no-cut-with-one-huge-message; malformed/no-usage paths;
- TUI full-history trim indicator; ACP gets no new trim notification in this plan —
  the vendored upstream schema (schema-v1.25.0, `internal/acp/testdata/SOURCE.md`)
  has no carrier, §4.4 forbids unknown payload fields, and a trim is not a
  compaction (§4.5); an ACP indicator rides the same future negotiated capability as
  `effectiveSize`, if one is ever added; old-reader compatibility behavior documented;
- goal reviewer fitted/pause path; judge fails closed.

Tests: 100K-char/CJK prompts; 8×large-read batch; recent error larger than ceiling;
repeated no-progress; new-history growth; trim/prune restore on resume with audit
original visible; missing artifact honest degradation; old reader reconstruction;
goal overflow pause; judge over budget without provider call.

### P4 — gates, live measurements, competitor benchmark, docs

- Add a hermetic **contextgate** (`test/contextgate`, `//go:build contextgate`,
  `bash test/contextgate/run.sh`) rather than placing no-network checks in the
  API-key-backed eval corpus. The scripted provider independently parses actual
  request bodies and records every work/summary/judge/review call.
- Gate arms:
  1. every work boundary (prompt, batch, nudge, wrap-up, resume, summary failure,
     compaction, recovery) satisfies the estimate-enforced ceiling;
  2. prune alone → exactly zero summary calls;
  3. resume request bytes survive artifacts; missing-artifact degraded path;
  4. malicious tool/delimiter injection cannot escape the recap boundary;
  5. goal + exact error + verified command canaries survive two compactions;
  6. no false fixed-overhead failure; bounded failure when the physical limit cannot
     fit `minUsefulOutput`.
- **Live measurements:** predetermined model/provider set, mixed code/prose/JSON/CJK
  fixtures, response-light and reasoning-heavy runs, at least two compactions each.
  Use independent provider usage; a fresh semantic question without re-read hints.
- **Competitor benchmark (goal 5):** pin exact Pi and OpenCode commits, run the same
  tasks/model/settings, and report quality (pass rate), billed tokens/cost,
  compactions, cache writes, and failures with medians/p95/bootstrap intervals.
  Ship the claim only if moca is no worse on quality and no higher on billed cost;
  otherwise publish the gap and the trade-off.
- Baseline: pin the current moca commit/binary and the eval label, not “current
  moca”. Keep `reuse`, `writeup`, and `review` as anti-regression scenarios.
- Docs: SPECS §2 (the contextgate command beside the other gates), §4/§13/§14, §7 if
  P0b changes adapter framing, §15 if the test or package counts move; both config
  fixtures; README; CHANGELOG; the self-knowledge reference re-copy; AGENTS.md's
  opening counts and gate list — per the §8 checklist, which is per-phase, not
  P4-only.

## 9. Acceptance criteria

1. **No knowable over-ceiling send.** Every work-stream call passes preflight. Above
   the operating threshold, policy performs bounded deterministic and/or model
   reduction. At the 80% ceiling it either reduces, warns under the precisely defined
   fixed-overhead exception, or returns a bounded error without streaming. A run may
   finish between 75% and 80% with a pressure notice; 75% is a trigger, not a
   guaranteed target.
2. **Fixed overhead is genuinely fixed.** System blocks + schemas alone push the
   request over the threshold/ceiling with no reducible history overlooked. The
   exception lowers output only to `minUsefulOutput`, requires
   `input <= effective - requestedOutput`, and always keeps
   `input + requestedOutput <= model.ContextWindow` with positive room. A rescue
   refit never violates these checks.
3. **No duplicate anchor accounting.** The anchor test proves each new
   response/tool entry appears once; the current total-usage anchor is not
   double-counted; hidden-reasoning overestimate is documented as conservative; and
   reviewer/summarizer/judge calls never anchor the work conversation. A future
   input-only anchor is a separate measured change, not a silent refactor.
4. **Estimator measured, not assumed.** Cold and anchored error are reported per
   protocol/model/content mix. Release target: median absolute error ≤8%, p95 ≤15%,
   cold start ≤20% on the mixed fixture. If not met, widen margins and record the
   limit; never claim the 5% universal margin held. Do not hard-code `[2.5,4.0]` as a
   conservative lower clamp; use explicit uncertainty.
5. **Fidelity with bounds.** After two compactions: two exact long errors, verified
   command evidence, current goal/constraints, and file paths are preserved in
   deterministic state or named recoverable artifacts with bounded metadata.
   Truncated metadata labels omissions. A no-goal session passes summary validation.
   Live semantic answers are scored independently.
6. **Spill is safe.** Outside/traversal/symlink entries are refused; invalid
   credentials/paths and I/O failures do not turn an original into an unrecoverable
   pointer. Missing files retain useful but degraded transcripts without silently
   claiming success. Session isolation remains observable.
7. **No unnecessary rewrites.** A run that never exceeds the operating threshold
   performs zero new elisions/trims (excluding the prompt trim that is itself the
   cause of the overage). Actual cache costs for compaction and mid-run pruning
   compare against the pinned baseline with sample sizes documented.
8. **Cost, not just prompt length.** Billed tokens, dollars, cache read/write,
   summary/retry counts and recovery reads are reported. Any regression is a visible
   trade-off requiring explicit acceptance, not hidden by character savings.
9. **Resume and front ends agree.** Model requests are byte-identical on resume under
   the same model/settings and surviving artifacts. Audit replay retains the full
   persisted prompt with trim annotation; ACP uses the physical window in
   `usage_update.size`; the TUI uses the effective window and documents the
   difference. Persisted reduction decisions (trim/elision/compaction entries) are
   authoritative on resume: the first resume request replays them and does not
   re-decide them; if the cold-start estimate re-crosses the operating threshold one
   further bounded reduction is allowed, and byte-identity is claimed only for the
   replayed-decision case, which the contextgate arm pins.
10. **Verification completed.** Contextgate and phase tests pass; `make vet &&
    make test` clean; `go test -tags counts ./cmd/moca -run TestSpecsTestCount -v`
    after test/doc-count changes; SPECS/config fixture/README/CHANGELOG aligned; the
    live baseline and competitor benchmark are reported with their limitations.
11. **Legacy switch is explicit.** `compactAtPercent=0` means reserve trigger on the
    effective window; true old behavior requires `compactAtPercent=0` and
    `maxWindowTokens=0`, and both are tested.

## 10. Risks and explicit trade-offs

| Risk | Required handling |
|---|---|
| Character counting differs by provider/model/content | Calibrate conservatively, per-route view, live error report, provider recovery; never call the estimate exact |
| Many recent errors/file-list metadata outrank capacity | Bounded quotas, priority metadata, artifact references; even protected entries may trim under the hard ceiling |
| Resume trusts a stored path | Validate root/name/symlink/permissions; pin to header artifact rules; never add an arbitrary read root |
| Missing artifact or old-session format | Honest degraded stub and visible record; no false byte-identical guarantee |
| Signature/reasoning route needs summarized history | Keep replay/protocol contracts distinct; probe the live route; surface a provider error if compaction cannot preserve it |
| Summary retry expensive or no prose due to reasoning | Reasoning room, at most one retry, actual-model accounting, deterministic fallback |
| 75% default increases compactions/cost | Compare same-task billed totals, cache writes and quality; report; do not assert automatic savings |
| 300K effective cap is a product default, not measured | P4 owns the decision; if it regresses quality/cost, set `maxWindowTokens=0` or another measured value; document it as provisional |
| Standing/state blocks add tokens | Enforce the §7 budgets; a block that exceeds its quota is trimmed with an explicit omission notice |
| Summary fallback loses style/nuance | No invented facts; structural state and bounded recent history; measure semantic retention |
| Competitor benchmark fails | Report the gap and the trade-off; do not claim parity |
| Older reader ignores new fields | Additive compatibility only; pre-v3 readers omit trims and cannot reproduce new bounded requests |
| Project config attempts to set context keys | Context keys are user-only; strict `ProjectConfig` decoding rejects them; test it |

## 11. Non-goals and deferred decisions

- No tokenizer dependency in the initial implementation. A per-provider tokenizer is a
  possible later enhancement if measured misses prevent useful margins.
- No branch/session-tree summarization, vector memory, retrieval store, plugins, new
  tool schema, image/audio input, or model-driven selection pass.
- No general goal-reviewer model rerouting to `summaryModel`.
- No claim of unbounded process-output recovery. Only persisted captured strings and
  the original transcript exist; shell capture omits its middle above its bound.
- No project-config context keys in this plan.
- No universal competitive ranking beyond the P4 benchmark.
- No hot-swap of the anchor representation without measured evidence.

## 12. Primary implementation map

| Concern | Existing anchor |
|---|---|
| Loop, prompt append, nudges, wrap-up | `internal/agent/agent.go`, `internal/agent/log.go` |
| Usage anchoring | `internal/agent/agent.go:474-554`, `internal/agent/control.go:291-309` |
| Goal review, permission judge | `internal/agent/goal_review.go`, `internal/agent/judge.go` |
| Summary and pruning | `internal/agent/compaction.go`, `internal/agent/elision.go`, `internal/compact/*` |
| Request/audit replay | `internal/session/rebuild.go`, `internal/agent/control.go`, `internal/acp/history.go`, `internal/tui/replay.go` |
| Session header / lazy sessions | `internal/session/{entry,writer}.go`, `internal/agent/{start,resume}.go` |
| Captured result storage | `internal/tools/overflow.go`, `internal/tools/shellrun.go`, other spill callers |
| Config defaults/validation | `internal/config/config.go`, fixture, strict-decoder and `ProjectConfig` tests |
| Effective window display | `internal/tui/{app,status}.go` — `refreshStatus` at `app.go:366` recomputes `Trigger` from the advertised window today and must be replaced by the agent's `Status.Trigger` (pinned in P0a); `internal/acp/session.go` |

## 13. What changed from v4

v4's own delta (v3 → v4) is summarized in this list; the v4 draft was removed. The
numbers below are the v4 review.

1. **`keepRecentTokens` default change removed.** v4's `min(Effective/10, 60K)` floor
   raised the default kept tail from 20K to 30K on a 300K effective window — a silent
   cost increase against goal 4. v5 uses `min(Effective/20, 60K)`, which preserves the
   20K default through 400K effective and grows only above it (30K at 600K, 50K at
   1M). The `Effective/4` cap remains. P0a's tests now pin 20K/30K/50K and the cap.
2. **P0a's deterministic fallback could grow the request.** v5 commits it only when
   the measured request is strictly smaller and at or below the ceiling; if `FindCut`
   splits a turn, the turn prefix is included in the recap, never dropped. Added to
   the P0a deliver list and tests.
3. **§5.2 rule 4 had no preflight step.** v5 adds `pruneProtectedIfNeeded` after
   summary/trim and before the fixed-overhead exception, at most once per boundary;
   otherwise a request over the ceiling with only protected entries would error
   without applying the documented last-resort prune.
4. **Mid-run prune gate was double-charged.** v5 states that the 25K/25% gate applies
   only to a prune that **avoids** a summary. A prune followed by a summary is
   compaction-adjacent and may be forced because the summary resets the cache anyway.
   The explicit cache break-even formula is now in §5.6.
5. **Standing constraints had no schema or selection rule.** v5 adds
   `ConstraintRef{EntryID, Tokens}` in `SummaryState`, restricted to raw user entries;
   the first genuine user prompt is always a candidate, model-proposed refs are
   validated and dropped when they point at tool output, generated text, envelopes, or
   nudges; the block is deterministic from raw refs.
6. **Audit notices had no representation.** v5 defines a fixed-prefix text block on
   the affected message (or a synthetic user message), keeping `Agent.History`,
   TUI and ACP replay deterministic while the request view stays bounded.
7. **Compaction observability was missing.** v5 adds additive `TokensAfter`,
   `PrunedTokens`, `SummaryTokens`, `Fallback` and `SkipReason` fields to the
   compaction entry for `/cost`, status, resume and evals, plus a
   `ContextPressure{Input, Threshold, Ceiling}` event for a request accepted between
   75% and 80%.
8. **Summary byte budget derivation was still implicit.** v5 defines
   `summaryInputByteBudget` from the summarizer's input-token allowance after
   output/reasoning reserve, using the conservative cold-start ratio (not 4), then
   subtracts the shared blocks.
9. **Docs checklist was overbroad.** v5 limits the per-phase counts/mirror checklist to
   phases that change Go files, SPECS, README, CHANGELOG or AGENTS.md; editing
   `docs/plans/` alone does not trigger it.
10. **Supersession chain.** v5 replaces v1–v4; the earlier drafts were removed. The
    v4 review history is summarized in this list rather than retained as a separate
    file.
11. **Split-turn summary contract was implicit.** v5 makes it normative in §5.3 and
    P2: a cut inside a turn summarizes history and the turn prefix separately, merges
    the current-turn section, and never drops the prefix; the P0a deterministic
    fallback includes the same prefix.

## Appendix — trigger examples

With `reserveTokens=16384`, `maxWindowTokens=300000`, `compactAtPercent=75`:

| Advertised window | Effective | Trigger | Fill at trigger |
|---:|---:|---:|---:|
| 16K | 16K | 12.0K | 75% |
| 32K | 32K | 24.0K | 75% |
| 64K | 64K | 48.0K | 75% |
| 128K | 128K | 96.0K | 75% |
| 200K | 200K | 150.0K | 75% |
| 1M | 300K | 225.0K | 75% of effective (22.5% of advertised) |

The last row is why the TUI percent and ACP `size` must be documented as different
denominators.





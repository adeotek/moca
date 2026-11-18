# Context management: implementation plan v2

Status: superseded by `2026-11-17-context-management-v4.md` · 2026-11-17
Scope: `internal/compact`, `internal/agent`, `internal/session`, `internal/config`,
`internal/provider`, `internal/tools`, `internal/tui`, `internal/acp`, and hermetic/live tests.

This revision resolves the review findings against the original plan. It does not
change the original `2026-11-17-context-management.md`. All normative behavior in
this document must land in `docs/specs/SPECS.md` in the change that implements it.

## 1. Goals, scope, and honest limits

1. Make 75% of the effective window the default operating trigger and enforce an
   80% estimated-input ceiling at every work-request boundary.
2. Preserve decision-relevant state across compaction, including exact errors,
   verified commands, current files, active goals, and earlier constraints.
3. Recover pruned detail on demand without broadening file access or silently
   losing the original when a spill write fails.
4. Reduce or hold total billed cost relative to the current implementation; do
   not exchange cheap context reduction for expensive cache resets.
5. Improve context management over measured moca baselines. “Better than Pi or
   OpenCode” requires a separate same-model benchmark; a feature checklist alone
   does not establish it.

The additional-token accounting includes summary requests, retries, cache reads
and writes caused by history rewrites, deterministic state metadata, spill/stub
prompt text, and recovery reads. Dollar cost and billed-token volume are tracked
separately: optimizing only one can regress the other.

### Definitions

- **Effective window** limits the normal working set:
  `min(model.ContextWindow, context.maxWindowTokens)`; the default cap is 300,000
  tokens and `0` selects the advertised window.
- **Reserve** is `min(context.reserveTokens, effective/4)`.
- **Operating threshold** is
  `min(floor(compactAtPercent/100 * effective), effective - reserve)`.
  `compactAtPercent` defaults to 75; an explicit `0` selects legacy
  `effective - reserve`; nonzero values must be `1..80`.
- **Ceiling** is `floor(0.80 * effective)` regardless of the operating threshold.
- **Requested output** is the exact next request's `max_tokens`, which may differ
  from the budget reserve during truncation rescue. It is not `model.MaxOutput`.
- **User prompt count** covers request message text, tool arguments/results,
  tool schemas, transformed assistant text/tool calls, and request-only goal/plan
  envelopes. It excludes the output budget.
- **Context estimate** is the next-request input estimate produced from the fixed
  system blocks, schemas, persisted data, and request-only envelopes. No image or
  audio input exists in this version; ACP rejects those content types.
- **Persisted captured result** is the exact string stored on a `tool_result`.
  It can itself contain a truncation/omission marker. Never claim that it is the
  original unbounded process output.
- **Audit projection** is the user-facing full conversation view, including trim
  metadata. It is deliberately different from the model-facing request view.

### Enforced invariant

For every work request (ordinary turn, continuation, nudge, and wrap-up):

1. Preflight projects and measures the exact next request before any provider
   stream starts.
2. Input above the operating threshold initiates bounded prune → compact → trim
   work, or deterministic degradation when a summarizer cannot run.
3. Request input above the ceiling is never knowingly sent. Either reduce it or
   return a bounded “context cannot fit” error naming the largest projected
   entries. There is no log-and-send branch.
4. The fixed-overhead exception can exceed the quality-oriented 80% ceiling
   only when projected system blocks + schemas already exceed the ceiling and
   no reducible history remains. Emit one warning naming both measured overhead
   and remaining history; silence that warning only if reducible history was
   incorrectly misclassified and is subsequently fixed. Mutable history cannot
   declare itself fixed overhead. The batching/output policy must not itself make
   the exception impossible: if needed, reduce the requested output to the
   positive value this request can actually reserve, then apply the ordinary
   effective-window capacity check (`input <= effective - requested_output`).
   Finally, it must fit the provider's advertised capacity
   `input + requested_output <= model.ContextWindow`. It may not exceed that
   physical limit or result in a negative/zero output budget. This exception
   does not allow skipping a large current prompt, batch or other mutable history.
5. The ceiling is an estimate-enforced product policy, not a claim that a
   character heuristic is an exact tokenizer. Provider overflow/length recovery
   remains a separate one-retry safety net. Live accuracy is measured against
   independent provider usage; heuristic uncertainty is never relabeled exact.

## 2. Current implementation and constraints to preserve

These are verified current-source facts, not target behavior:

- `compact.Budget.Trigger()` is `window - reserve`, with reserve and kept tail
  capped at `window/4` (`internal/compact/budget.go`). This approaches the full
  context window on large windows rather than 75–80%.
- The usage anchor is set after the assistant response and tool calls are
  persisted (`internal/agent/agent.go:545-552`). The anchored delta then counts
  persisted messages again. v2 must correct that semantic mismatch before using
  the anchor for tighter decisions.
- `ContextTokens` falls back to character estimates after compaction, elision, or
  model changes. `MessageChars` does not include all wire signatures or protocol
  framing. Exact request-token reconstruction is provider/model-dependent.
- `maybeCompact` currently runs before the initial user prompt and after a tool
  batch. Append-and-continue paths can bypass it; `wrapUp` and `nudge` append
  independently, and the goal reviewer sends its own fitted request.
- Compaction uses head-only tool-result truncation, includes thinking, and can
  cap away a prepended previous summary. The existing previous-summary cap bug
  is a correctness issue independent of the feature work.
- History-bound and reasoning-replay routes have distinct contracts:
  `Model.HistoryBound()` and `Model.KeepsTailThinking()` are not interchangeable.
  Preserving signature-bearing history is required on such routes when a
  provider's protocol needs it.
- Assertions that shell/save files always retain the original unbounded output
  are false. `internal/tools/shellrun.go` intentionally retains a bounded head
  and tail with an omission marker. Overflow storage preserves its captured
  string, not a magically recovered middle.
- `prepare` currently allocates a random spill directory before Start constructs
  a session header and before Resume uses the stored header. Resume already has
  the header available, so v2 must pass the chosen path through setup explicitly.
- `Agent.History` calls `session.Messages`; ACP replays it. Therefore changing
  that shared default to trim model requests also trims user-facing replay,
  which is forbidden.
- JSON session decoding is not strict. Go's decoder ignores unknown entry types,
  and existing rendering skips them. Additive fields preserve compatibility,
  but an old binary still reconstructs the untrimmed prompt; this is not full
  behavioral backward compatibility.

Preserve: append-only transcripts, the fixed-once system prompt, eight frozen
tool schemas and their behavior/caps, sequential complete tool batches, no
call/result splitting, per-session artifact isolation, cancellation and retry
accounting, and import direction. Provider/tool-specific behavior must not be
moved into leaf packages `session` or `compact`.

## 3. Request model and one preflight contract

### 3.1 Separate projections

Introduce an internal request-projection function shared by `ContextTokens` and
the request builder. Its inputs are:

- stored head/tail system blocks;
- the tool schemas;
- session entries rendered with per-entry plan and goal envelopes;
- a transformed model-specific history, including replayed reasoning and
  signatures required by the route;
- the next tool choice, requested output, effort, and cache key;
- persisted elisions and trims.

Use three explicit views; none silently substitutes for another:

1. **Raw entries:** the full append-only transcript, used for deterministic state
   accumulation and audit.
2. **Audit projection:** full original prompt text plus compact trim/prune
   notices, used by `Agent.History`, TUI replay, and ACP. Existing elisions keep
   their request-only stubs rather than expanding to unlimited old tool output.
3. **Request projection:** bounded history, stubs/trim windows, and request-only
   envelopes, used only for model input and summary serialization.

Do not make trimmed `Messages` the global default. Give `MessagesWith` (or a new
clearly named sibling) a request-only render policy. Existing callers retain
full/audit semantics until deliberately migrated.

### 3.2 Protocol-based input projection

No new tokenizer dependency is introduced in P0. Define a bounded internal
`InputView` (message count, text/argument/result/signature characters, tool schema
size, system size, and route class), with an estimate function per supported
protocol, not a single undocumented “serialized request.”

Calibration records the same view before a turn and independent input usage
(provider `Input + CacheRead + CacheWrite`) after it. Preserve the richest raw
response usage for calibration (including observable token/character metrics
where an adapter actually exposes them); do not invent token counts for omitted
fields.

The estimator may use a session-local ratio updated toward observed values, but:

- never reject or replace a lower conservative ratio merely because it is below
  2.5 chars/token;
- use a conservative cold-start rule for code/JSON/CJK/binary-derived text;
- include per-message/tool-call/schema overhead explicitly;
- account for transformed foreign reasoning and signature/reasoning replay;
- reset model-specific calibration when the qualified model changes;
- handle missing and manifestly inaccurate provider usage as untrusted input,
  with a conservative fallback and a metadata-only warning;
- never calibrate using the second post-compaction usage result as if it measured
  the pre-compaction request.

No lower clamp may turn a measured dense-text ratio into a less conservative
estimate. If the implementation needs an upper estimation-error guard, apply it
as explicit uncertainty margin rather than discarding the measurement.

### 3.3 Exact next-request boundaries and bounded degradation

Create one agent-owned `prepareRequest` path, called immediately before the work
stream begins. It constructs the request, runs context-budget policy, and returns
the final prepared request. `turn` must not bypass it.

Cover these explicitly:

- first request after the initial prompt;
- every normal tool-following turn;
- every continue caused by steering, truncation, empty output, plan, verify,
  execute-plan, progress, or record nudges;
- step-limit/stuck wrap-up;
- the immediate retry after overflow compaction.

Prefer keeping the initial preflight before prompt persistence; add preflight
after persistence only when the prompt was appended since last preflight. Never
elide or trim between a tool call and its matched result. Run guards after a
complete batch, but before the next request; add early batch-entry reduction as
required by §5.5.

Pseudocode (normative behavior, not an exact Go signature):

```
prepareRequest(choice):
    snapshot entries + model + output/effort
    request = project(snapshot)
    P = policy(effective, threshold, ceiling)

    if request.inputEstimate <= P.threshold:
        return request

    originalBoundary = latestCompactionID(snapshot)
    request = prune(request, P)
    if request.inputEstimate <= P.threshold:
        emit Pruned
        return request

    request = summarizeOrBoundedDeterministic(request, P)
    request = boundedRetryCompactionIfUseful(request, P) // at most one
    request = trimOversizedContent(request, P)

    if request.inputEstimate <= P.ceiling:
        return request
    if fitsFixedOverheadException(request, requestedOutput, advertisedWindow):
        warnOnce
        return request
    return error(contextCannotFit, largestProjectedEntries)
```

The actual threshold policy may reuse a request after each reduction; it should
not repeat a compaction or prune pass when no new entries or progress exist.
Any progress-limit disable is scoped to the same boundary and only suppresses a
repeat identical summarization attempt; it never suppresses the ceiling check.

### 3.4 Goal reviewer and other non-conversation calls

`reviewGoal`, `judgeCmd`, and the summarizer have intentionally different prompts
and no normal conversation mutation. Applying ordinary compaction to a pending
shell command would be wrong. They do not use `prepareRequest` unchanged, but
each must have an explicitly fitted request and a safe no-provider-call fallback:

- **Goal reviewer:** measure its own message transform and review prompt against
  the reviewer model, not the active model. If over budget, run a bounded local
  review projection (objective, goal status, recent tool evidence, deterministic
  state, and relevant recent assistant/user text) before sending. Never
  automatically append a user turn or compaction solely to make a review fit.
  If still too large, pause with the existing “review failed/compact” guidance.
- **Permission classifier:** its payload already truncates user text and command
  text independently. Validate its model's minimum budget and fail closed (`ask`)
  without a provider call when its fixed system/payload cannot fit.
- **Summarizer:** fit against its chosen model (including reasoning/output room),
  not against the work model; use a bounded payload (§5.3).

These reviewer/classifier paths must not anchor `ContextTokens` or alter its
conversation usage anchor. Only the work-model snapshot establishes that anchor.

## 4. Budget, configuration, and status

### 4.1 Budget representation

Use a structure with effective-window values resolved once per call, e.g.
`Budget{Window, Effective, Reserve, KeepRecent}`. Keep `Trigger()` as a method;
do not introduce a `Trigger` field with the same name. Add a separately named
hard-ceiling method/function.

The budget distinguishes quality policy from provider capacity. The fixed
capacity check uses `model.ContextWindow` and the exact `requested_output`; it
does not assume that `model.MaxOutput` fits. If `requested_output >= window`
(or there is no positive input allowance), fail configuration/request setup
before the stream.

The adaptive kept-tail formula is:

```
keepRecent = min(max(configured, min(effective/10, 60_000)), effective/4)
```

This is a floor capped for small windows, not a permission to evade the request
ceiling when recent entries themselves are unusually large.

### 4.2 Presence-aware configuration

Add the following user-context keys:

| Key | Default | Accepted values | Meaning |
|---|---:|---|---|
| `context.compactAtPercent` | 75 | 0 or 1–80 | 0 selects legacy reserve trigger |
| `context.maxWindowTokens` | 300000 | 0 or ≥16000 | 0 selects advertised model window |
| `context.summaryModel` | `""` | empty or qualified `provider/model` | empty resolves `model`, not active `/hard` model |

Use presence-aware integer fields (for example pointers, later normalized to
plain resolved values). Explicit `0` must survive defaults, user config reload,
and any allowed layer merge. A general `0` default replacement is not sufficient.
Reject negative values, percentages >80, and nonzero values below the minimum;
report the exact config key. Existing project config does not adopt arbitrary
context keys automatically: if these are intentionally project-supported, add
allow-list validation and tests preventing a project from silently selecting a
billable summarizer.

Model-summary resolution is:

1. `context.summaryModel` if nonempty;
2. otherwise the session configuration `model`, following the current cheap
   model semantics rather than an in-session `/hard` switch;
3. if that model/credential is unusable, fall back once to the active model with
   one warning;
4. if neither is usable, use the deterministic fallback when policy needs a
   reduction. Never skip necessary automatic reduction merely to preserve work
   model optimism.

Every actual attempt records its own usage/cost against that actual selected
model, including failed retries and validation retries.

### 4.3 Status and ACP

Do not change the meaning of existing `Status.Window` accidentally. Add explicit
`EffectiveWindow`, `ContextTrigger`, and (debug only) estimator-source fields.
TUI warnings/percent bands use effective values. Yellow starts at
`max(60, compactAt-15)` percent and red at the operating threshold, relative to
effective capacity.

ACP `usage_update.size` must be explicitly specified. Preferred compatibility
rule: keep legacy physical window semantics unless an ACP client capability or
release-synchronized contract selects the effective window. In either case, ACP
and TUI must not silently calculate different denominators. Document the chosen
field/compat path and add tests for a capped 1M model.

## 5. Safety-focused context reduction

### 5.1 Stable, validated spill artifacts

Use one immutable session-scoped directory, chosen before the session header
is created. Resume passes that header's stored value into `prepare`; it must not
allocate and use a new random directory. Retain a stable file-name mapping for
each pointer.

A session header may store `spillDir` as a future-proof additive field. Validate
it on resume rather than blindly admitting it:

- expected absolute child of the resolved existing overflow root;
- no `..`, symlink escape, or alternate path spelling;
- expected session identifier in its name when id-derived;
- trusted permissions/ownership consistent with current artifacts;
- a validated file target must remain beneath that directory after symlink
  evaluation; regular files only;
- parent directory 0700, spill files 0600; use the existing lazily created path.

Do not add arbitrary header-supplied directories to the jail. Older sessions with
existing random directories need either a validated record of the directory they
already reference, or a generic degraded stub; never silently remap old pointers
to a new derived directory. Lazy sessions that change an ID due to a file-name
collision must retain the immutable persisted pointer directory, or have a
specified pre-header-materialization mapping rule. No stub stores a pointer to a
transient unrecorded directory.

`tools.Spill` must return success/failure to compaction/prune callers; an empty
note is insufficient for persistence. For pruning streams already present in the
transcript, spilling the complete captured result is enough; copy the original
again only if required for durable storage. For oversized user input, persist
the full validated text before appending its trim record.

Rules:

- if a required spill fails, keep original content in the request and report a
  bounded warning/error appropriately; do not commit an unrecoverable stub;
- if a previously valid spill is gone at resume, use a generic stub with an
  explicit `original artifact unavailable; full content remains in this session's
  audit history where applicable, or re-run/re-read the original source` marker;
- that is degraded recovery, not “byte-identical” or fully reversible pruning;
- missing or failed artifacts never block the request by reading the original
  file merely to render it;
- retention after the configured retention period is allowed, but must be
  surfaced as a known lifetime limit, not forgotten as a fidelity guarantee.

### 5.2 Deterministic prune policy, including bounded protected data

Pruning is per-entry and precedes summarization only when it is useful. Preserve
tool-call/result association and all provider-required signing/call/result rules.
The ordinary preference order is:

1. protect the newest tool-output suffix, the most recent user turns, latest
   result per distinct path/call, and errors;
2. prune stale/superseded entries with a pointer where safe;
3. prune older completed output beyond the protective cadence;
4. if this still cannot fit, prune even protected entries using bounded
   head/tail plus a verified spill pointer.

The last rule must be explicit, including for the most recent errors. The
invariant outranks any retention class. Never tell the model an error vanished
solely because it was expensive.

Define “last two user turns” as the last two genuine user prompts/note/steering
entries (goal continuation messages included); host nudges do not count. The
protection boundary may advance across completed work turns in transcript order;
future synthetic nudges cannot retroactively resurrect pruned history. Define
latest-result identity per canonical
path plus read window, not just path: a file's later rewrite does not always
invalidate an earlier read of an unrelated offset. A failed later rewrite does
not supersede a success.

Current candidates/scorer must operate on the actual history-bound route's view;
do not accidentally remove the relevant signature-bearing preserved tail that
the provider requires. All I/O spills happen before the entry identifying them
becomes live request state.

The estimated saving is `max(0, originalProjectedTokens - stubProjectedTokens)`,
not raw bytes/4. The stub includes the complete path/provenance text already
needed for request rendering.

### 5.3 Bound model summary input, and preserve cumulative memory

For summary purposes:

- use head+tail windows around tool results, snapped to valid UTF-8 and useful
  line boundaries;
- use a policy that can be unit-tested by tool class, without changing tool
  schemas or runtime returned results;
- keep unlimited raw commands out of the summary request, with exact full
  commands in bounded state metadata or stable artifacts;
- annotate omitted characters/UTF-8 width and any available spill provenance;
- drop stale historical thinking from the summary model request; separately
  preserve any current route-required reasoning in the reconstructed work
  request. Summary serialization is not permission to strip persisted reasoning.

Do not cap the previous summary indirectly. Calculate:

```
summarySpanBudget = max(0, summarizerInputBudget
    - len(previousSummaryBlock)
    - len(systemPrompt)
    - len(currentTurnBlock)
    - safetyBytes)
```

If `summarySpanBudget == 0`, do not send an over-budget summary call; use the
bounded deterministic merge. If the previous prose summary is already oversized,
load the previous raw metadata and deterministic state and downsample history
without losing pointers to prior artifacts.

A list budget for metadata must be part of every compaction. Separate output
prose budget from metadata budget. Prose output can remain capped at 4K tokens
only after reasoning headroom is defined; the summary string should be bounded
without silently cutting mid-section or mid-UTF-8. A hard post-output cut is not
a fidelity strategy. If there is not enough summary space, choose the deterministic
merge rather than truncate facts after generation.

Treat summary text as a generated recap, not system authority. Use escaped,
length-delimited, or otherwise robust serialization for untrusted delimiter text
(simple unescaped XML wrappers are not enough). Distinguish host-introduced
instructions, generated recap, earlier real user instructions, and untrusted
tool/output-derived facts. Tool errors and commands can preserve evidence but do
not gain authority to alter permissions, credentials, workdir, or goals.

Retain genuine historical user constraints via their raw entry locations or an
explicit bounded standing-construction record validated against those raw entries.
A lossy generated excerpt is not the authoritative source of a constraint. The
rebuilt request re-injects a bounded authoritative constraints/standing-context
host block outside the model-written summary, with references to the underlying
raw entries. If a constraint exceeds the block budget, preserve its complete
text in a validated artifact and inject its reference plus an omission notice;
do not silently drop or demote it. Generated summaries can repeat a constraint as
evidence but cannot become its only authority. Existing per-entry plan/goal
envelopes remain authoritative rather than being replaced by generated prose.

Fresh `/compact <text>` input is explicit user focus instruction, not prior tool
output. Keep it at most 500 Unicode code points, pass
it only through its dedicated summary field and do not persist it as an
unrequested conversation message.

### 5.4 Deterministic cumulative state and bounded fallback

Construct state deterministically from the **raw transcript**, not only from the
window retained for model input. Merge it with prior deterministic state so
fallback compaction cannot forget everything before the previous kept boundary.
Keep historical session state distinct from current UI/provider state.

State includes at least:

- active goal ID, objective/status, continuation/resume status, and references to
  the raw objective/standing-constraint entries; host continuation/resume wording
  is request metadata, not generated summary text;
- known read/modified files and canonical record of relevant read windows;
- verified commands with result evidence (latest red/green state), with exactly
  reconstructed full command text in a pointer if clipping it would lose meaning;
- failing tool results with exact error text preserved within a bounded,
  prioritized critical-fact quota, plus a validated stable spill for full text;
- prior artifact pointers/provenance and retained important decisions/next steps.

Use bounded per-class quotas and a stable, inspectable priority order. Do not
require all file lists to fit into a fixed 200–500-token budget. Persist a
structured `summaryState` with explicit list/item budgets and a discarded/omitted
item count; large file lists get an overflow record/file when necessary.

Rough budgets must be assessed per model's effective window, not a universal
200–500 tokens. The most recent exact critical errors and active goal outrank
historical read-file listings. Old read/write sets may be compressed to only
touched paths and aggregate counts when necessary.

Fallback merge must combine previous summary state and newly summarized raw
entries. A plausible format is:

- prior structured summary/state, within its quota;
- current raw user requests, bounded assistant excerpts, calls/results, and errors;
- current goal, files, verification state, and important next steps;
- explicit omitted-history counts and available artifact pointers.

Fallback text is marked generated, is bounded, and does not resurrect untrusted
instructions. Retry-loop accounting persists every spend before the fallback.
Even no-history-needed auto-prune must still verify the request below threshold.

### 5.5 Oversized prompts and tool batches

Oversize trigger for user prompt: the prompt itself exceeds
`min(effective/4, 32000)` estimated tokens, or its addition would exceed the
request threshold. Persist the full prompt and full text artifact **before the
preflight that precedes the initial request**, then append a trim record
referencing stable IDs (before the actual first request has started). The session
transcript retains the original entry regardless of trim outcome. The request
projection is bounded to the smaller of the operating batch size and remaining
ceiling allowance, at approximately 60% head / 40% tail, not to an arbitrary
fixed substring. Preserve user-visible original text via the audit projection.

Tool batches can create more input than the old kept tail can summarize. Perform
batch reduction immediately after each tool result is persisted, before the next
tool call executes, but never interleave trimming between a call and its own
result. Apply it when projected live batch entries exceed `min(effective/5,
40000)` or the request threshold would otherwise be exceeded. Fallback trimming
of the latest batch during preflight is still required. Persist exact captured
content, not a “full output” claim. Use counts and provenance in stubs and keep
errors identifiable.

No trimming occurs mid-tool-call/result. If a single genuine user message plus
fixed provider-required head can never fit by this policy, return a clear error
without sending it; it is not an endless compaction case.

### 5.6 Cache economics and hysteresis

Default prune points are compaction boundaries, where history must change anyway;
change the current forced-before-summary ordering there. Mid-run pruning needs a
conservative threshold of `max(25K, 25% of projected request input)` when cache
behavior/prices are uncertain. If no-cache behavior is known, cheaper pruning may
be permitted; absence of price metadata alone does not establish no caching.

Derive any break-even formula from actual cache/read/write pricing and estimated
remaining requests. The original 150K/8K example does not justify a universal
20% or 25% rule. Unknown provider cache behavior defaults to the conservative
threshold; measurements determine A/B changes.

Hysteresis only suppresses an identical repeated attempt at the same boundary
while no progress has been made. Always reevaluate after new entries; always
enforce operating threshold (or a valid fixed-overhead exception) and the hard
ceiling. If forward history cannot be reduced, stop retrying summaries and
switch to trim/fallback/error, not unbounded summary requests.

## 6. Failure handling, validation, and accounting

### 6.1 Degenerate summaries

Validate required section structure and decoded summary limits, never mere
headings as proof of correctness. Expected sections and nonempty current goal
must be present. For a reasoning summary model, calculate thinking room
separately (like goal-review output ceilings) to avoid a 4K no-prose response.
If the summary is cut off or malformed, one controlled retry may retry with the
same safe payload; a retry is not required when deterministic fallback is the
better token choice.

Persist usage of every attempted call, including its actual model, successful or
failed. Missing summary prose must not create a new compaction boundary. Only a
complete compaction append and all its requested pruning/trim projections may
change the anchor. Do not replace a usable previous summary after a failed new
call; the previous summary state still exists and can feed fallback.

### 6.2 Metrics and anchor semantics

For work requests only:

1. snapshot exact projected request metadata before streaming;
2. consume response `input + cacheRead + cacheWrite` as its actual input once;
3. store the anchor against the snapshot that **excludes the just-received
   assistant response**;
4. on the next projection, add new persisted response/tool entries exactly once;
5. invalidate on compaction/prune/trim/model switch;
6. calibrate only from matching request/usage observations.

For text-only, tool-call, replay-reasoning, and history-bound turns, assert this
no-double-count rule in tests. Never anchor on summary, goal review, or permission
classifier requests. Completed summary/goal/judge usage remains in global cost
and token totals, but it does not mean those tokens belong in the next work
conversation.

## 7. Delivery phases

Each phase is independently reviewable, keeps `make vet && make test` green, updates
SPECS with its behavior, and includes its failure-mode tests in the same PR.
P0–P3 are one release unit: merging P0 without the hard-edge fallbacks must not
be represented as production-complete context management. No behavior PR requires
a commit unless the user separately approves it.

### P0 — one preflight, corrected estimator, and hard ceiling

Files: `internal/agent/{agent,control,compaction}.go`, `internal/compact/{budget,estimate}.go`,
`internal/provider` adapters only for the explicit input-view needed,
`internal/config/config.go`, `internal/session` for entry view as required.

Deliver:
- request/audit separation and shared request projection;
- anchor fix, load-bearing protocol projection and conservative calibration;
- presence-aware config and resolved effective values;
- mandatory ceiling at every work turn, nudge and wrap-up; fixed overhead uses
  exact requested output and physical window;
- deterministic no-summarizer reduction sufficient for ceiling enforcement;
- bounded preflight-summary retries/active-model fallback; ACP/TUI status plumbing;
- scoped no-progress hysteresis.

Tests:
- regular turn, prompt paste, every nudge/continuation, wrap-up, goal-review-fitted
  request, rescue retry;
- 75% default, 80% maximum, explicit 0 legacy trigger, `maxWindowTokens: 0`,
  configured 99 rejection, small-window/large-MaxOutput;
- missing key summarizer, no-summary errors, unavailable credential active-model
  fallback, no log-and-send;
- empty/missing/absurd provider usage, exact system/tools too large, request with
  giant foreign thinking signature;
- anchor no-double-count text/tool/replay-reasoning cases;
- effective-window TUI and ACP tests;
- cannot-fit error, cancellation, cost accounting.

### P1 — deterministic safe prune and stable spill

Files: `internal/agent/{elision,start,resume}.go`, `internal/session/{entry,rebuild}.go`,
`internal/tools/overflow.go` and spill callers.

Deliver:
- raw captured-result storage with durable success/failure, provenance and
  per-ID stubs;
- validated session header directory, no fresh-random-dir mistake on resume;
- per-entry pruning, priority-desperation rules and actual projected savings;
- prune-first, skip summary if enough, cache-conservative mid-run policy;
- isolated per-session artifacts with path-escape/symlink validation.

Tests:
- last-two-turn/protected-suffix boundary, failed write, canonical per-window path;
- many fresh and old errors that need bounded reduction;
- forced vs non-forced history-bound route;
- no stale rewrite below threshold; no-cache known/unknown cases;
- spill write failure preserves original, deleted artifacts degraded honest stub,
  wrong path/outside path/symlink rejected, unicode/content counters;
- resume byte-identical with artifacts present, same content after resume for the
  next actual request, no sibling-session artifact read.

### P2 — accumulated state and summary fidelity

Files: `internal/compact/{summary,serialize}.go`, `internal/agent/compaction.go`,
`internal/session/{entry,rebuild}.go`, `internal/config/config.go`,
`internal/tui/app.go`, summary model resolution.

Deliver:
- head+tail by tool class, exact omission counts, provenance, foreign reasoning
  distinction;
- cumulative deterministic state, bounded metadata quotas, fallback merging prior
  state, critical errors/commands with exact artifact pointers;
- previous-summary input reservation without cap-away;
- escaping/trust taxonomy and `/compact <focus>`;
- summary reasoning/output budget, structural check and retry accounting.

Tests:
- malformed digest/structural delimiter injection, prior user constraints not
  demoted to tool truth, old summary malicious instruction not elevated;
- previous summary protected on tiny cap, exact long error retained, >2000
  files bounded structurally, fallback after previous compaction retains older
  planted fact;
- focus rune/byte policy, summarizer actual model accounting and thinking
  truncation/retry;
- no summary call when prune alone suffices, no oversize input and no blank response.

### P3 — last-resort edges and compatibility

Files: `internal/agent/{agent,goal_review,judge}.go`,
`internal/session/*`, `internal/compact/cut.go`.

Deliver:
- oversized prompt spill and request trim, latest-batch trim, display/request
  projection divergence tests;
- deterministic fallback when no summary model can run at any required boundary;
- no-cut-with-one-huge-message and malformed/no-usage paths;
- ACP/TUI full-history trim indicators, old-reader compatibility behavior.

Tests:
- 100K-character and CJK prompts, 8×large-read batch, recent error larger than
  ceiling, repeated no-progress after compaction, subsequent new-history growth;
- trim/prune restore on resume with audit original visible; resume when artifact
  gone (honest degraded output, not byte-identical claim);
- old reader new fields reconstruction documented, goal overflow fitted/paused,
  judge over budget fails closed without provider request.

### P4 — service gates, live measurements, and documentation

Add a separate **hermetic context gate**, e.g. `test/contextgate` with
`//go:build contextgate` plus `bash test/contextgate/run.sh`, rather than placing
no-network checks into the API-key-backed `test/evals` corpus.

The scripted provider independently parses actual request bodies and derives
deterministic per-protocol token counts under its own test rule. Fixture-reported
usage deliberately includes incorrect/no-usage cases. It records every request,
including work, summary, judge and goal reviewer calls. On unit tests, assert
deterministic-versus-provider differences explicitly; do not let the fixture's
numbers masquerade as a real vendor tokenizer.

Gate arms:

1. prompt, large batch, nudge/wrap-up, resume, summary model failure, compaction
   and recovery; all ordinary work sends satisfy the estimate-enforced ceiling;
2. sufficient prune, with exactly zero summary calls;
3. exact resume bytes while artifacts survive; missing-artifact case exercises
   the separately documented degraded path;
4. malicious tool data and delimiter injection cannot escape the recap boundary;
5. active goal + exact failing error + verified command canaries survive at least
   two compactions in the raw deterministic state;
6. no false fixed-overhead failure and bounded failure when physical limit cannot
   accommodate requested output.

Run **live** accuracy/cost/recall separately. Use a predetermined model/provider
set and mixed code, prose, JSON, CJK input fixtures, response-light and reasoning
-heavy runs, at least two compactions each. Live outcomes use independent provider
usage and a real model answer to a fresh question without re-read hints; canned
server responses alone do not count as semantic recall.

Current moca serves as the cost/quality baseline. Accumulate identical tasks,
model, scripted outputs where appropriate, and key configuration. Report median,
p95, bootstrap intervals where sample size supports them, failures separately,
and noise limits. Do not assert a universal "not higher token cost" pass when
there are too few runs or outcome differences. Unchanged scenarios required by
this feature include `reuse`, `writeup`, and `review` as an anti-regression floor;
failure-path/source-specific tests are not proof of semantic retention.

## 8. Acceptance criteria

1. **No knowable over-ceiling send.** Every work-stream call passes preflight.
   Above the threshold, policy performs deterministic and/or model reduction. At
   the 80% ceiling, it either reduces, warns under the precisely defined
   fixed-overhead exception, or returns a bounded error without streaming.
2. **Fixed overhead is genuinely fixed.** System blocks + schemas alone push the
   request over threshold/ceiling; no reducible history was overlooked. For this
   exception, lower the next output request only as far as needed to fit, then
   require the ordinary effective-window capacity: `input <= effective -
   nextRequestedOutput`. In every exception case, `estimatedInput +
   nextRequestedOutput <= advertisedWindow`, with positive room. Small effective
   windows on models with huge MaxOutput are accepted when their actual capped
   output fits, not rejected by subtracting MaxOutput.
3. **No duplicate anchor accounting.** Next-request simulation includes each new
   response/tool entry once and excludes reviewer/summarizer/judge calls; tests
   cover text, tools, foreign/replayed reasoning, compaction.
4. **Estimator measured, not assumed.** Cold and anchored error are reported per
   protocol/model/content mix. A proposed release target is median absolute
   error ≤8%, p95 ≤15%, and cold-start ≤20% on the mixed fixture. If not met,
   widen policy margins and record the limit; never pretend the 5% universal
   margin held. Do not hard-code `[2.5,4.0]` as a conservative lower clamp.
5. **Fidelity with bounds.** After two compactions, two exact long errors,
   verified command evidence, current goal/constraints, and file paths are
   preserved in deterministic state or named recoverable artifacts with bounded
   metadata. Truncated metadata labels omissions. Live semantic answers are
   scored independently.
6. **Spill is safe.** Outside/traversal/symlink entries are refused; invalid
   credentials/paths and I/O failures do not turn an original into an
   unrecoverable pointer. Missing files retain useful but degraded transcripts
   without silently claiming success. Session isolation remains observable.
7. **No unnecessary rewrites.** A run under threshold with no role-changing
   history reduction makes no new elisions. Actual cache costs for compaction
   and mid-run pruning compare against baseline with sample sizes documented.
8. **Cost, not just prompt length.** Billed tokens, dollars, cache read/write,
   summary/retry counts and recovery reads are reported. Any regression is a
   visible trade-off requiring explicit acceptance, not hidden by character
   savings.
9. **Resume and front ends agree.** Model requests are byte-identical on resume
   under same model/settings and surviving artifacts. Audit replay retains full
   trimmed prompt with trim annotation; ACP has its explicit size/window rule.
10. **Verification completed.** New phase and contextgate tests pass; `make vet &&
    make test` clean; `go test -tags counts ./cmd/moca -run TestSpecsTestCount -v`
    after test/doc-count changes; SPECS/config fixture/README/CHANGELOG aligned;
    eval baseline comparison documents measured outcomes and limitations.

## 9. Risks and explicit trade-offs

| Risk | Required handling |
|---|---|
| Character counting differs materially by provider/model/content | Calibrate conservatively, per-route input projection, live error report, provider recovery; never call estimate exact |
| Many recent errors/file-list metadata outrank the capacity | Bounded quotas, priority metadata, artifact references, even protected entries trim under ceiling |
| Resume trusts a stored path | Verify root/name/symlink/permissions and pin to header artifact rules; never arbitrary read root |
| Missing artifact or old-session format | Honest degraded stub and visible record; no false byte-identical guarantee |
| Signature/reasoning provider needs history already summarized away | Keep replay/protocol contracts distinct, probe on relevant live route before treating route as fully supported; surface provider error if the route cannot accommodate compaction safely |
| Summary retry is expensive or no prose due to reasoning | Room for reasoning, retry at most once, actual-model usage accounting, deterministic fallback |
| Default 75% increases compactions/cost | Compare actual same-task billed totals, cache writes and quality; report rather than assert automatic savings |
| Conservative no-summary fallback loses style/nuance | Do not invent facts; prefer structural state and bounded recent history; track semantic retention against current moca |
| Older reader ignores new fields | Additive compatibility only; state that pre-v2 readers omit policy trims and cannot be expected to reproduce new bounded requests |

## 10. Non-goals and deferred decisions

- No tokenizer dependency in the initial implementation. A tokenizer per provider
  stays a possible later enhancement if measured misses prevent useful margins.
- No branch/session-tree summarization, vector memory, retrieval store, plugins,
  new tool schema, image/audio input support, or model-driven selection pass.
- No general goal-reviewer model rerouting to `summaryModel` in this plan.
- No claim of full unbounded process-output recovery. Only persisted captured
  strings and the original transcript are available; shell capture omits its
  middle above its bound by design.
- No universal proven competitive ranking. If desired, pin exact Pi and OpenCode
  commits, share the same model/workload and define metrics before running that
  comparison separately.

## 11. Primary implementation map

| Concern | Existing anchor |
|---|---|
| Loop, prompt append, nudges, wrap-up | `internal/agent/agent.go`, `internal/agent/log.go` |
| Usage anchoring | `internal/agent/agent.go:474-554`, `internal/agent/control.go:291-309` |
| Goal review, permission judge | `internal/agent/goal_review.go`, `internal/agent/judge.go` |
| Summary and pruning | `internal/agent/compaction.go`, `internal/agent/elision.go`, `internal/compact/*` |
| Request/audit replay | `internal/session/rebuild.go`, `internal/agent/control.go`, `internal/acp/history.go`, `internal/tui/replay.go` |
| Session header / lazy sessions | `internal/session/{entry,writer}.go`, `internal/agent/{start,resume}.go` |
| Captured result storage | `internal/tools/overflow.go`, `internal/tools/shellrun.go`, other Spill callers |
| Config defaults/validation | `internal/config/config.go`, fixture, strict decoder tests |
| Effective window display | `internal/tui/{app,status}.go`, `internal/acp/session.go` |

The desired end state is smaller than the original v1 concept in one important
respect: the preflight, projection, budget, metadata and spill contracts are
explicit first; enhancements sit on top of them rather than around them.

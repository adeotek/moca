# Review: phase 4 — context manager, compaction, resume

Date: 2026-10-05
Branch: phase/4-context-compaction-resume   Range: main..HEAD (merge-base 0e3f3f5 → a17f1d4; 12 commits, 31 files, +1683/−50)
Plan: docs/plans/phase-4-context-compaction-resume.md (incl. Implementation notes)   Spec: DESIGN.md §6, §8, §12.5, §7.5; docs/specs/SPECS.md §3, §3.5, §13, §14, §15
Follows up: docs/reviews/2026-10-04-phase-1-skeleton-providers.md, docs/reviews/2026-10-05-phase-2-tools-agent-loop.md, docs/reviews/2026-10-05-phase-3-tui*.md (recurrence check only; see "Earlier findings")

## Verdict

**Approve with fixes.** All nine tasks are implemented, every Global Constraint is honoured in the code, and each of the five Review Focus items has real code behind it and a test that can fail. All seven recorded deviations are real and justified — including the `FindCut` whole-window-fits refusal, which resolves a genuine contradiction inside the plan (its own cut algorithm vs `TestManualCompactNothing`) in the direction Review Focus 2 requires. No High findings: nothing loses data, nothing persists state across a failure it shouldn't, and the overflow/repair paths were probed end to end. One Medium (an auto-compaction failure can kill an otherwise healthy run when the cheap model's key is missing) and a set of Lows.

## Summary

Phase 4 delivers `internal/compact` (budgets with small-window scaling, cut-point selection, lossy serialization, cumulative file tracking, input capping, structured summaries with split-turn merge), compaction in the agent (two trigger checkpoints, loop guard, one compact-and-retry on overflow), `session.Messages` rebuild honouring the latest compaction, `Find`/`FindForWorkdir`, `agent.Resume` with crash repair, `--resume`/`--continue` in both `-p` and the TUI, and the TUI `/compact` command with its busy guards.

I read every changed non-test file in full plus the test files for all five packages the phase touches, traced each Review Focus item and each Implementation-note deviation to code, and ran probes with `go test -overlay` from a scratch directory (nothing in the repo was touched): a second-compaction probe (previous summary fed in), a `turnStart==0` split-turn probe, a `CapChars` multibyte probe, a `TrackFiles` path-form probe, and two agent-level overflow probes (length-stop recovery with and without compactable history). Machine checks are clean.

Counts: **0 High · 1 Medium · 7 Low.**

## Checks run

| Check | Result |
|---|---|
| `mise x go -- gofmt -l .` | prints nothing (exit 0) |
| `mise x go -- go vet ./...` | clean (exit 0) |
| `mise x go -- go test ./... -race -count=1` | all 12 packages ok (11 with tests), 0 failures |
| `go test -list` recount | 252 top-level tests — matches SPECS §15's claim exactly |
| Gate-named suites re-run individually (`-race`): `TestAutoCompactionShrinksContext`, `TestLoopGuardNamesOversizedEntry`, `TestOverflowCompactsAndRetriesOnce`, `TestOverflowRecoveryFailsWithoutHistory`, `TestManualCompactNothing`, `TestManualCompactWritesEntryUsageAndRebuild`, `TestResumeRebuildsExactContext`, `TestResumeAfterCrashMidTool`, `TestResumeNeverRestoresYolo`, `TestResumeHonoursCompaction`, `TestResumeNotASession`, `TestResumeUnsetKeyFailsAtFirstRequest`, `TestResumeAndContinueOneShot`, `TestFind`, `TestFindForWorkdirViaSymlink`, `TestFindNoSessions`, `TestMessagesFromCompaction`, `TestBudget`, `TestFindCut*`, `TestSerialize`, `TestTrackFiles`, `TestCapChars`, `TestCompact*`, TUI `TestCompactNothing`/`TestCompactingBlocksRun`/`TestCompactedAndResumedLines` | all pass |
| Probe: second compaction feeds the previous summary in and summarizes only the live window (`-overlay`, internal/compact) | confirmed — call 2 contains the first summary verbatim; kept boundary advances |
| Probe: split turn at `turnStart==0` with no previous summary | exactly one summary call; merged summary well-formed |
| Probe: `CapChars` on a CJK payload with no newline in the tail window | stranded UTF-8 lead byte at the seam (finding 3) |
| Probe: `TrackFiles` with mixed path spellings | `read "a.go"` + `edit "./a.go"` leaves the path in both lists (finding 4) |
| Probe: `StopLength`-overflow recovery via the real agent loop (mock SSE) | recovers exactly once with compactable history; with none, surfaces the original error and persists nothing beyond the prompt (matches `TestOverflowRecoveryFailsWithoutHistory`) |
| `git status --short` after the checks | clean before this review file was written |

Not verified: the three live gate steps (TUI long session, small local model, kill-mid-tool + `--continue`) and the loop-guard live reproduction — they need a real provider key and network; the plan's Implementation notes and SPECS §15 record them as run on 2026-10-05, and I took that as recorded without re-running it. The `startTestWith` mock server paths stand in for them in the unit gates above.

## Plan conformance

| Task | Status |
|---|---|
| 1 Budget | Done (`compact/budget.go`). `min(x, window/4)` scaling, strict `>` trigger; `TestBudget` covers both windows and the boundary |
| 2 Entry view + cut selection | Done (`entry.go`, `cut.go`). Cut only at user/assistant; smallest fitting cut, else latest valid; `turnStart` only for assistant cuts. The whole-window refusal (`suffix[0] <= keepRecent → ok=false`) is the documented deviation 1 |
| 3 Serialization, files, capping | Done (`serialize.go`). Labels, 200-char arg cap, 2K result truncation, `[… truncated]`, `COMPACTION NOTE (lossy)` header, cumulative `TrackFiles`, tail-keeping `CapChars` |
| 4 Summary prompt + `Compact` | Done (`summary.go`). Two-call split-turn merge under `## Current turn (in progress)`; `turnStart==0 && prev==""` correctly skips call A; usage summed; file lists appended |
| 5 Rebuild + agent compaction/overflow | Done (`rebuild.go`, `agent/compaction.go`, `agent.go`). Latest-compaction rebuild, `maybeCompact` at both checkpoints, loop guard wording exact, overflow = `ErrContextOverflow` or `StopLength`-with-input-within-reserve, one retry as a fresh turn, nothing persisted on first overflow, second overflow surfaces the original |
| 6 Session lookup | Done (`find.go`). id8 regex, `last` by mtime, 5-most-recent list on miss, canonical workdir compare reading only the first line |
| 7 `agent.Resume` | Done (`resume.go`, `start.go` build()). Stored prompt verbatim, header workdir as jail root, latest `model_change` restore, `Repair` before build, totals + snapshots restored, yolo only from `o.Yolo`, writer closed on every error path |
| 8 CLI + TUI wiring | Done (`cli.go`, `oneshot.go`, `tui.go`, `app.go`). Mutual exclusion exit 2, lookup failures exit 2, `resolveResume` shared, `ResumePath` branch, `/compact` as `tea.Cmd` with refusals, `Compacted`/`Resumed` lines |
| 9 Phase gate | Unit gates pass (table above). Live gates not verified here; README status + docs commit (`a17f1d4`) present |

Deviations: all seven Implementation notes check out against the code —
1. `FindCut` refusal (`cut.go:16-18`) — justified; it is what makes Review Focus 2 true, and the plan's own `TestManualCompactNothing` demanded it. Pinned by the third `TestFindCutNothingToDo` case and the second `TestCompactNothing` case; consequence documented (`TestOverflowRecoveryFailsWithoutHistory`).
2. Test re-sizing (5 turns not 6; escaped `\0`; 80K-char overflow prompts; 16K-window loop-guard model) — each verified against the caps by reading the test and the budget math; the reworked tests fail for the right reasons (assertions on exact summary counts, guard text `12k tokens`, persisted-entry counts).
3. Concurrency hardening — real: `Compact` totals/anchor under `a.mu` (compaction.go:120-123), `liveEntries` snapshots under the lock (:36-38), `Resume` closes the writer on all four error paths.
4. `Messages` missing-`firstKept` fallback (rebuild.go:31-36) — real; note it has no counterpart in `liveEntries` (finding 5).
5. TUI `compacting` guards — real (app.go:401-403, 433-435, 519-525); broader than the plan, matches SPECS §3.5.
6. `Agent.Workdir()` — real; the TUI adopts the resumed session's workdir (app.go:842-846).
7. `resolveResume` shared by both modes — real (oneshot.go:37-46, tui.go:63-67).

Nothing outside the phase's scope; no non-goal touched.

## Review Focus

1. **Huge single tool result as the newest entry** — satisfied. `FindCut` can only cut at user/assistant messages, so the cut lands before the result's assistant message (`TestFindCutNeverBetweenCallAndResult` asserts exactly that, and that the cut is never at a tool result); when the result fits the kept suffix it is kept verbatim and excluded from the serialized summary (`Serialize(es[:cut])`); when it cannot fit, `FindCut` returns the last valid cut and the loop guard stops the run naming it — `TestLoopGuardNamesOversizedEntry` drives a real 50K-char `read` through the loop on a 16K-window model and asserts the error contains `read` and `12k tokens`. Can fail.
2. **`/compact` with nothing to summarize** — satisfied. `cut.go:16-18` refuses when the whole live window fits `keepRecent`; `Compact` returns `ok=false` with no summarizer call (`TestCompactNothing`, both cases); the agent maps that to `ErrNothingToCompact` with zero new request bodies (`TestManualCompactNothing` asserts body count unchanged); the TUI prints `nothing to compact` (`TestCompactNothing` in the TUI). No compaction entry is written on that path (the `!ok` return precedes `a.append`). Can fail.
3. **`--resume` with the stored model's key now unset** — satisfied for the main claim: `Resume` never resolves credentials (lazy `env:` resolution), so resume succeeds; the first request fails with the env error and `-p` exits 2 naming the variable (`TestResumeUnsetKeyFailsAtFirstRequest`); `--model` override is wired (`resume.go:57-62` via `SetModel`, which does check the *new* model's credential — fail-fast on an explicit override is the phase-3 contract). Gap: no test exercises the `--model`-override-on-resume path at all (finding 8).
4. **`--continue` through a symlinked workdir** — satisfied. `FindForWorkdir` canonicalizes with `Abs` + `EvalSymlinks` (`find.go:88-93`), the same comparison as the jail's `canonical`; `TestFindForWorkdirViaSymlink` pins it and can fail.
5. **Summarizer call itself would overflow** — satisfied. `capChars = (cheap.ContextWindow − maxTok − len(SummarySystem)/4 − 512) × 4` (compaction.go:79, with a `max(0,…)` guard the plan lacked) and every payload passes through `CapChars` (`summary.go:59`), dropping oldest whole lines behind `[… earlier history omitted]`; `TestCompactCapsInput` asserts the prefix and the size bound. The plan's scenario (huge serialized history on a small-window cheap model) cannot produce an overflowing summary request. Can fail.

## Findings

### Medium

#### 1. Auto-compaction aborts a healthy run when the cheap model's credential is missing — Confirmed
`internal/agent/compaction.go:74-77` (`summarizer`), `:136` (`maybeCompact`), `internal/agent/agent.go:279,339`

`summarizer()` always resolves `a.opts.Config.Model` (the cheap model) and the summarizer request resolves its `env:` key lazily at request-build time. `maybeCompact` returns any `Compact` error except `ErrNothingToCompact`, so when the cheap model's key is unset the run dies at the compaction trigger with `compaction failed: … OPENAI_API_KEY…` — even though the active model (e.g. after `/hard` to another provider, or `--model` overriding a default whose key was never set) is working fine and nothing has overflowed. `SetModel` only ever checks the credential of the model being switched *to*, so nothing along the `/hard` path ever notices that the configured default's key is gone.

Failure scenario: config has `model: openai/gpt-6-astra` (no `OPENAI_API_KEY` in the environment) and `modelHard: anthropic/claude-opus-5-5` (key set). `moca -p "long task" --model anthropic/claude-opus-5-5` works turn after turn; the moment the usage-anchored estimate crosses `window − reserve`, `Run` returns the compaction error and `-p` exits 2 attributing an unused provider's missing key. Every subsequent run dies at the same checkpoint — the session is stuck until the key exists or the config changes.

Proposed fix: in `maybeCompact`, distinguish "the summarizer is unusable" (credential/resolve error) from "compaction failed mid-flight": for the former, either fall back to the current model for the summary (a recorded deviation from §12's cheap-model rule) or at minimum keep the run alive until an actual overflow, letting the existing overflow recovery surface the original provider error. Smallest change: wrap the summarizer-credential failure in a sentinel and skip the automatic trigger when it matches, so only the overflow path (which must compact or die anyway) reports it.

### Low

#### 2. `CapChars` can split a multi-byte rune at the cap seam — Confirmed (probe)
`internal/compact/serialize.go:130`

`tail := s[len(s)-max:]` slices at a byte offset; when the tail window starts mid-rune (a huge line with no `\n`, e.g. a single massive user prompt or a minified file with CJK content) the payload keeps a stranded UTF-8 lead byte after the omitted marker. Probe: `CapChars("[User]: "+"あ"×100, 10)` → `[… earlier history omitted]\n\x82あああ`. The summarizer sees one mangled character; `json.Marshal` silently replaces it with U+FFFD rather than erroring, so nothing crashes — the channel is lossy by design and the damage is one glyph at the seam.

Failure scenario: a CJK-heavy session is compacted on a small-window summarizer; the first visible character after the omission marker is replacement garbage. Cosmetic.

Proposed fix: after slicing, advance `tail` past any incomplete rune sequence (`for len(tail) > 0 && !utf8.RuneStart(tail[0]) { tail = tail[1:] }`) — three lines, plus a `TestCapChars` case with multibyte content and no newline.

#### 3. `TrackFiles` does not normalize path spellings, so a modified file can stay in `<read-files>` — Confirmed (probe)
`internal/compact/serialize.go:90-108`

Paths are compared as raw strings. Probe: a `read` with `{"path":"a.go"}` followed by an `edit` with `{"path":"./a.go"}` yields `read=[a.go]`, `mod=[./a.go]` — the edited file remains listed as read-only context. Models mix relative/absolute/`./`-prefixed spellings freely (the tools accept all of them).

Failure scenario: a long session reads `src/x.go` then edits it as `./src/x.go`; the next compaction's summary tells the model `x.go` is unmodified context and separately lists it as modified — mildly misleading continuity, no correctness impact on tool execution.

Proposed fix: canonicalize the tracked path (`filepath.Clean`, and join with the jail root when relative) before inserting into the maps, so one spelling wins; or accept and document that the lists are best-effort.

#### 4. `liveEntries` lacks the missing-`firstKeptEntryID` fallback that `Messages` has — Confirmed by reading
`internal/agent/compaction.go:41-49` vs `internal/session/rebuild.go:31-36`

When a compaction's `FirstKeptEntryID` no longer matches any entry (trimmed/corrupt file), `session.Messages` falls back to the compaction entry itself (deviation 4), but `liveEntries` leaves `start = 0`, so the next compaction re-serializes the *entire* transcript — including everything the previous summary already covers — while also feeding the previous summary in as context. Wasteful and duplicative, not lossy.

Failure scenario: only reachable on a hand-edited or externally trimmed session file; the next `/compact` produces a summary that restates summarized history at full serialization cost.

Proposed fix: mirror the `Messages` fallback — when the id is not found, start from the latest compaction entry's index.

#### 5. The loop guard can name near-empty entries when the overage comes from system + tools, not the transcript — Plausible
`internal/agent/compaction.go:131-143`, `internal/compact/cut.go:16-18`

`maybeCompact` triggers on the usage-anchored estimate (system prompt, tool specs and cache included), while `FindCut` measures only entry chars. There is a band where the estimate is over the trigger but the entry total fits `keepRecent` (trigger − keepRecent ≈ window/4 of overhead is enough — realistic on a 16K–32K window local model with a large system prompt, skills list and tool schemas). There, `Compact` refuses (`ErrNothingToCompact`) and the guard error lists the largest *live entries*, which can be `user message ≈ 0k tokens` — the message never names the system/tools overhead that actually caused it, and the session then fails on every run for that model (compaction can never help).

Failure scenario: 32K-window local model, system+tools ≈ 10k tokens, short conversation: every `moca -p` run aborts with `context still over budget after compaction (…); largest entries: user message ≈ 0k tokens; …`.

Proposed fix: when `largestEntries` reports entries far below the overage, add the fixed overhead to the message (e.g. `system+tools ≈ Nk tokens`) so the user can tell the model/prompt is the problem, not the history.

#### 6. Manual `/compact` is uncancellable and unbounded in wall time — Confirmed by reading
`internal/tui/app.go:560-565`

`compactCmd` runs `a.Compact(context.Background())` — no cancellation and no timeout, and the summarizer request inherits the full retry ladder (5 attempts with backoff, 90 s stall timeout each, plus the one mid-stream retry), so a stalled endpoint can hold the TUI in `compacting…` for minutes with `esc` doing nothing (only quit works, which kills the process mid-append; `trimPartialLine` makes that recoverable).

Failure scenario: the cheap-model endpoint hangs; the user presses `esc` repeatedly, the transient stays, and the only way out is killing moca.

Proposed fix: derive the context from a cancellable one stored on the model and cancel it in `quit()`/`esc`, or bound it with a timeout of a few minutes.

#### 7. `textOf`/`messageText` duplication — recurrence of a deferred phase-3 item — Confirmed
`internal/agent/agent.go:222-230` vs `internal/tui/app.go:794-802`

Phase-3 pass-2 Low #9 deferred the dedupe "to phase 4, when compaction needs the same extraction". Phase 4's summarizer does use the agent copy (`compaction.go:88`), but the TUI copy was left as is and `llm.TextOf` was still not created. Pure maintainability.

Proposed fix: move the helper to `internal/llm` and delete both copies.

#### 8. Review Focus 3's "`--model` can override" resume path has no test — Confirmed
`internal/agent/resume_test.go`, `cmd/moca/cli_test.go`

`Resume`'s `o.Model != ""` branch (`resume.go:57-62`, `SetModel` + `model_change`) is wired in both `-p` and the TUI but no test passes `--model` on a resume — not the happy path, not the "override to a provider whose key exists while the stored model's key is unset" case the Review Focus item describes. A regression there (e.g. dropping the branch, or checking the wrong credential) would not be caught.

Proposed fix: extend `TestResumeUnsetKeyFailsAtFirstRequest` with a `--model` run against a second, keyed provider asserting exit 0 and a `model_change` entry.

## Earlier findings

Phase-2 review (all 15 recorded fixed; items whose code this range touches):

- **#4 truncated tool call ends the run** — still fixed; `turn` persists `{}` for invalid JSON and `Run` attaches the cut-off error result (`agent.go:199-205`, `:320-322`); `TestLengthStopTruncatedToolCall` passes, and the truncation path coexists correctly with the new overflow branch (overflow returns before any persistence).
- **#10 `session.Open` loses entries after a crash-truncated line** — still fixed; `Resume` goes through `Open` → `trimPartialLine`, which is what makes the kill-mid-tool gate durable.
- **#5 `Truncate`, #1 retry duplication, #12 exit mapping** — unchanged in this range; suites pass.

Phase-3 reviews (passes 1–4, all recorded fixed):

- **eventPipe pre-program buffering (pass-2 H1)** — still fixed and now load-bearing: `Resume` emits `Resumed` before the program exists, and the gated forwarder delivers it after `p.Run` starts (`app.go:819-830`, `bridge.go:57-96`).
- **`!` mid-run (pass-3 #3)** — still fixed; `submit` refuses `!` while running/shellBusy (`app.go:411-420`), so a `!` note cannot land inside a tool batch or during a manual compaction's run window (a `!` during `compacting` appends before or after the compaction entry, never inside it — the writer mutex serializes; no orphan results either way).
- **`/clear` state carry (pass-3 #14)** — still fixed via `Carry()`; untouched.
- **pass-4 items** (approval keys, trust signal handling, test clock) — packages untouched by this range; tests pass unchanged.
- **Pass-2 Low #9 (`textOf` dedupe)** — was explicitly deferred to phase 4 and did not happen: finding 7.

No earlier finding regressed in this range.

## Suggested order of fixes

1. Finding 1 (compaction failure kills healthy runs) — decide the fallback/error-attribution policy; it is the only finding that stops real work in a plausible configuration.
2. Findings 2 and 3 together (small `serialize.go` fixes with one probe-style test each).
3. Finding 8 (add the `--model`-on-resume test) — cheap, pins a stated behaviour.
4. Findings 4, 5, 6 as hardening; 7 opportunistically with 8's test touch.

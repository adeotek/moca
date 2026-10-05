# Review: phase 4 — pass 2 (context manager, compaction, resume)

Date: 2026-10-05
Branch: phase/4-context-compaction-resume   Range: `0e3f3f5..a17f1d4` (10 commits, 31 files changed, +1683/−50)
Plan: docs/plans/phase-4-context-compaction-resume.md   Spec: DESIGN.md §6, §8, §12.5 (+ §7.5 for resume-yolo, §3 for the summary request shape)
Follows up: `docs/reviews/2026-10-05-phase-4-context-compaction-resume.md` (pass 1, same day — written by the earlier review attempt that was interrupted after the document was saved). Every pass-1 finding was re-verified here: 6 hold as written (1 Medium, 5 Low), 2 hold with corrected detail (see "Earlier findings"), 1 was re-traced and **retracted** (pass-1 Low 2: `TurnEnd` sets `Transient = ""` at `app.go:682`, so a retry transient cannot survive into the next run; the `Compacted`/`handleCompactDone` transient clears cover `/compact`'s own paths). This pass adds: the full-suite `-race` gate result (pass 1 listed individual suites), a probe on the capped-payload path to Review Focus 5, and the `!`-during-`/compact` interaction gap (new Medium).

## Verdict

**Approve with fixes.** The phase delivers everything the plan asks for and the core machinery is sound: cut points never orphan tool results, the loop guard and overflow recovery behave exactly as specified (the original-error capture traced correct), resume reuses the stored prompt and repairs crashes idempotently, and the concurrency discipline is a real step up from the plan's sketch. No blocking findings. Two Medium findings (the `!`-command gap in the TUI busy-flags, which can silently drop a note's content from the rebuilt context, and one resume robustness item promoted here for its data-visibility effect) and several Low items are worth fixing before merge.

## Summary

Phase 4 adds token-denominated budgets with small-window scaling, cut-point selection on a neutral entry view, lossy serialization with cumulative file tracking, structured summaries over a cheap-model summarizer, token-triggered compaction at two checkpoints with a loop guard, one compact-and-retry on provider overflow, `/compact` in the TUI, and `--resume <id8|last>` / `--continue` for both `-p` and the TUI with crash repair.

Checked: plan conformance task-by-task; every Review Focus item traced to code + test; the full diff and full text of `internal/compact/*`, `internal/session/{rebuild,find}.go`, `internal/agent/{agent,compaction,resume,start}.go`, `cmd/moca/{cli,oneshot,tui}.go`, `internal/tui/app.go`+`bridge.go`, and all new/changed tests; concurrency paths (agent mutex vs TUI polling, event pipe, `/compact` goroutine); rebuild edges (missing kept id, tool_use across the cut, merging); FindCut/Serialize/CapChars boundaries; resume error paths; CLI exit codes; test quality (can each test fail); plus four small throwaway probes (in-repo temporarily, deleted; the tree shows only the two review files).

Counts: **0 High · 2 Medium · 5 Low** from this pass, plus 1 confirmed + 1 retracted carry-over from pass 1 (see Earlier findings).

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` | clean (no output) |
| `go vet ./...` | clean (exit 0) |
| `go test ./... -race -count=1` | green — all 11 test packages pass (`ok` for cmd/moca, agent, compact, config, llm, permissions, provider, session, skills, tools, tui) |
| `go test ./internal/compact/ ./internal/session/ ./internal/agent/ -count=1` (re-run for this review) | green |

Not verified (live/manual gate steps from Task 9 — need a real provider key or a real terminal, per the review rules no key file was used):

- **Live: long session** TUI compaction on a real repo (`⋯ compacted: N → M` in the bar, answer drawing on the summary, session file with one `compaction` entry).
- **Live: small local model** compaction without a loop.
- **Live: kill mid-tool + `--continue`** end-to-end banner + follow-up request.
- The implementation notes' and pass 1's gate evidence (compaction on a 32K-declared glm-5.3-flash, loop-guard exit 1, kill -9 resume) is taken as recorded, not re-executed here.

## Plan conformance

- **Task 1 (Budget)** — done as specified (`internal/compact/budget.go`); small-window `min(x, window/4)` scaling tested.
- **Task 2 (entry view + FindCut)** — done; one deliberate deviation (`FindCut` refuses when the whole live window fits `keepRecent`, `cut.go:16-18`) is recorded in the plan's Implementation notes §1 with the contradicting plan test case explained away and pinned by `TestFindCutNothingToDo` + `TestCompactNothing`. Accepted: it is what makes Review Focus #2 (`/compact` with nothing to summarize → no-op) true, and it matches Pi's observable behaviour.
- **Task 3 (Serialize/TrackFiles/CapChars)** — done as specified; 2000-char result cap, 200-char per-value cap, sorted keys, `⏎` newline folding, cumulative read/modified lists all tested. `capChars` in the agent clamps at 0 (`compaction.go:79`, `max(0, …)`) beyond the plan sketch — a correctness improvement for tiny windows.
- **Task 4 (SummaryPrompt + Compact)** — done; split-turn two-call merge with the exact heading the plan pins; input capping applied to every payload including the previous-summary block.
- **Task 5 (rebuild + agent compaction)** — done, plus the recorded deviations: `session.Messages` fallback when `firstKeptEntryId` is not found (notes §4), concurrency hardening (notes §3). `turn()` overflow detection and `turnWithRecovery` match the plan sketch, with `lastOverflow` persisted on the agent and never written to the session.
- **Task 6 (Find/FindForWorkdir)** — done as specified; 8-hex validation, mtime ordering, first-line-only header reads, canonical-path comparison.
- **Task 7 (Resume + build)** — done; `build()` shared, writer closed on every error path (one robustness nuance → Medium #2), yolo only from `o.Yolo`, `Resumed` emitted once.
- **Task 8 (CLI + TUI wiring)** — done; exclusive flags exit 2, `resolveResume` shared by `-p` and TUI, `ResumePath` honoured in `tui.Run` (`app.go:833-846`), `/compact` guarded beyond the plan (notes §5).
- **Task 9 (gate)** — unit gates pass. Live gates not re-run here (see Not verified).
- All deviations are recorded in the plan's Implementation notes with justification; none touch the non-goals list. No scope creep found (`Workdir()` accessor and the `compacting` flag are the smallest additions that make the wiring safe, and both are documented as deviations).

## Review Focus

1. **Single tool result larger than `keepRecent`** — **satisfied.** The result travels with its call: `FindCut` only cuts at user/assistant messages (`cut.go:20-27`), so a 50K-char `read` result is kept verbatim if the remaining budget allows, otherwise the loop guard names it (`maybeCompact`, `compaction.go:139-142`; `largestEntries` renders `tool_result for read("path":"dist/app.min.js") ≈ 12k tokens`). Test: `TestLoopGuardNamesOversizedEntry` (uses a 16K-window model so a 50K-char read really busts the trigger even after keeping the newest batch; asserts the error names `read` and `12k tokens`). The note-reworking of test sizing (plan notes §2) is sound — I recomputed the arithmetic.
2. **`/compact` with nothing to summarize** — **satisfied.** `FindCut` returns `ok=false` when `suffix(0) ≤ keepRecent` (`cut.go:16-18`); `compact.Compact` returns `(Result{}, false, nil)` with no summarizer call; `Agent.Compact` maps that to `ErrNothingToCompact` (`compaction.go:107-109`); the TUI prints `nothing to compact` (`app.go:574-575`). Tests: `TestFindCutNothingToDo` (three cases incl. the two-entry window), `TestCompactNothing` (compact-level, asserts zero fake calls), `TestManualCompactNothing` (agent-level, asserts no new request bodies), `TestCompactNothing` in the TUI (end-to-end through the key handler).
3. **`--resume` with an unset provider key** — **satisfied.** Keys are lazy: `Resume` never resolves credentials (registry construction only validates ids), the first request fails with the `env:` error, and `exitFor` maps `*config.EnvError` → 2 (`oneshot.go:18-28`). Test: `TestResumeUnsetKeyFailsAtFirstRequest` (resume succeeds with exit-2 request failure naming `MOCA_T_KEY`). `--model` override: `Resume` calls `SetModel(o.Model, effort)` when different (`resume.go:57-62`) — the credential check inside `SetModel` is consistent with lazy-key resume (an override to a *different* provider with an unset key fails resume with exit 2, which is defensible and matches §12's "credential check first" contract).
4. **`--continue` through a symlinked workdir** — **satisfied.** `FindForWorkdir` canonicalizes with `filepath.Abs` + `EvalSymlinks` (`find.go:88-93`), the same comparison the jail uses; headers are read from the first line only. Test: `TestFindForWorkdirViaSymlink`.
5. **Summarizer call itself would overflow** — **satisfied.** Input cap = `(cheap window − 4096 − len(SummarySystem)/4 − 512) × 4` chars (`compaction.go:79`), applied inside `compact.Compact` to every payload via `CapChars` (`summary.go:59`), which drops the oldest whole lines with the `[… earlier history omitted]` marker. Tests: `TestCompactCapsInput` (payload actually capped with the marker prefix), plus the agent-level `TestAutoCompactionShrinksContext` runs a real summary request on a 32K model.

## Findings

### Medium

#### 1. A `!` command during `/compact` is not refused, and its note can be excluded from the compaction it raced — Confirmed
`internal/tui/app.go:410-421` (submit's `KindShell` branch), `app.go:442-444` (`refuseBusy`), `internal/agent/compaction.go:96-126`

The `compacting` flag refuses runs, `/clear`, `/model`, `/effort`, `/hard`, `/yolo` and a second `/compact` (`refuseRunning`, `app.go:429-440`), and `refuseRunning` correctly routes `/compact` away while a run *or* a `!` is in flight. The gap is only in `submit`: the `KindShell` branch checks `m.running` and `m.shellBusy` but never `m.compacting`. So while the status bar shows `compacting…`, the user can run `! ls` and its `AddNote` user message is appended to the transcript concurrently with the compaction.

The compaction itself is race-free (both appends take `a.mu`), but the timing is not deterministic in a way that matters for content: `Compact` snapshots `liveEntries()` (`compaction.go:97`) *before* the summarizer request, and appends the compaction entry possibly seconds later. A `!` note appended in between is (a) silently excluded from the summary and the cumulative file lists of this compaction, and (b) placed *before* the compaction entry in the transcript, so `session.Messages` — which starts at the latest compaction's `firstKeptEntryID` — can drop the note's text entirely from the rebuilt request if the cut boundary lands before it. The user typed it, saw it in scrollback, and the model never sees it. (The `TokensAfter` figure the `⋯ compacted` line reports is computed after the note is appended, so the bar can even show a higher count than the summary covers.)

Failure scenario: `/compact` on a large session; while `compacting…` shows, `!git status` runs; the note lands before the compaction entry and its text is outside the kept window — the next request silently lacks it.

Proposed fix: in `submit`'s `KindShell` branch add `if m.compacting { return println("a /compact is still running — wait for it to finish") }` before `m.shellBusy = true` (one guard, mirrors the existing `shellBusy` refusals).

#### 2. `Resume` of a session whose header workdir no longer exists fails late with a raw jail error and exit 1, after the writer is already open — Confirmed (robustness; behaviour is safe, the diagnostic is not)
`internal/agent/resume.go:51-56`, `internal/agent/start.go:68-71`

`Resume` opens the session writer, appends repair results, and only then calls `build` → `permissions.NewJail(h.Workdir, …)`, which fails when the stored workdir no longer exists (deleted project dir, disconnected mount). The failure path is correct — the writer is closed and no transcript damage occurs (verified: `build` returns before any agent append, and `resume.go:53-56` closes `w`). But the error is the raw jail/MkdirAll error (`*StartError` → exit 1 in `-p`), naming an internal path-jail failure rather than "the session's workdir `<path>` no longer exists", and the repair results were already appended to the file: a user retrying `--continue` after recreating the directory gets a *second*, redundant `interrupted` repair (harmless — `Repair` is idempotent and re-`Repair` finds nothing unanswered — but the file now carries synthetic results from a resume that never became usable).

Failure scenario: session started in `/tmp/scratch` (tmpfs), reboot, `moca --continue -p x` → `moca: mkdir /tmp/scratch: permission denied`-style raw error, exit 1, plus repair entries appended for a session that cannot run.

Proposed fix: validate `h.Workdir` (stat + canonicalize) before opening the writer in `Resume`, and wrap the `NewJail` failure with `fmt.Errorf("resume: session workdir %s: %w", h.Workdir, err)`; map it to the usage path if the directory is gone entirely.

### Low

#### 1. `largestEntries` renders tool-call inputs as raw JSON in the error string — Confirmed (cosmetic, quoting differs from the plan example)
`internal/agent/compaction.go:153-155`

The plan's example shows `read(path=dist/app.min.js)`; the code emits `read("path":"dist/app.min.js")` (raw `{"path":…}` with braces trimmed). The live gate evidence quotes the same raw-JSON shape, so behaviour is consistent and tested (`TestLoopGuardNamesOversizedEntry` asserts only `read` + `12k tokens`). Cosmetic divergence from the plan's illustrative format; no action needed unless the format matters to someone parsing it.

#### 2. `Serialize` emits empty assistant text blocks as bare `[Assistant]: ` lines — Confirmed (edge)
`internal/compact/serialize.go:58-59`

Thinking blocks skip empty text (`serialize.go:55-57`) but `BlockText` with `Text == ""` still prints `[Assistant]: \n`. Harmless for the summarizer; a wasted line per tool-only assistant message. Suggest matching the thinking branch's emptiness guard.

#### 3. `CapChars` can split a multi-byte rune — Confirmed (edge, bounded harm)
`internal/compact/serialize.go:126-134`

`tail := s[len(s)-max:]` slices at a byte offset, which can start mid-rune for CJK/emoji-heavy histories; the corrupted first rune lands at the very start of the retained tail (right after the omission marker). Same class of issue in `callLine`'s `v[:200]` and `Serialize`'s `body[:resultCap]` — all byte-slices. The plan specified exactly this behaviour ("values cut at 200 chars", "truncated at 2000 chars"), and token estimates use chars/4 where the skew is negligible. Propose rune-safe trimming only if the summary quality on CJK-heavy sessions ever matters; documenting the choice would suffice.

#### 4. `Find` mtime ties are resolved nondeterministically; `--resume last` can pick either session — Plausible
`internal/session/find.go:33`

`slices.SortFunc` with `cmp.Compare(b.mod, a.mod)` is not stable for equal `UnixNano` mtimes (same-second creation on coarse-mtime filesystems, or two sessions created within one clock tick in tests — `TestFind` avoids this with a 20 ms sleep, which is evidence the authors hit it). On tmpfs/ext4 with nanosecond mtimes this is unlikely; on network filesystems (NFS export of the data dir) it is real. `slices.SortStableFunc` or a name-based tiebreak would make it deterministic. Severity genuinely Low.

#### 5. `TestOverflowCompactsAndRetriesOnce` asserts exactly-once indirectly — Confirmed (test-quality note)
`internal/agent/compaction_test.go:111-145`

The exactly-once property is pinned by `len(summaryBodies(s)) != 2` (two summaries = history + split turn) and `comps != 1`. This *would* catch a double compact-and-retry within one recovery (a third summary body / second compaction entry would appear), so it can fail for the right reason. However it does not directly assert the total request count: a stray extra turn would be caught only via `out.Text != "recovered"` (the script would serve "(script exhausted)"). Verdict: the assertion chain is sound but indirect; a direct `len(s.bodies)` assertion would make the exactly-once contract explicit. Not blocking.

### Verified fine (no findings)

- **Overflow recovery exactly-once and original-error semantics** (`agent.go:262-276`) — traced all four branches: non-overflow passes through untouched; on first overflow `orig := a.lastOverflow` is captured **before** `Compact` runs (so a later `lastOverflow` overwrite cannot corrupt it); failed compaction → `orig`; second overflow → `orig`; nothing was persisted on any overflow path (`turn()` returns before the append block; pinned by `TestOverflowRecoveryFailsWithoutHistory`, which asserts the session holds nothing but the prompt). `TestOverflowCompactsAndRetriesOnce` pins exactly one compaction entry and exactly two summary requests (split-turn) for the recovery.
- **`turn()` overflow detection** — `provider.ErrContextOverflow` or `StopLength` with `input+cacheRead+cacheWrite ≥ window − reserve` (`agent.go:159-169`); the `lastOverflow == nil` fallback message covers the StopLength case; `StreamReset` is emitted so both consumers (`-p`'s `turnText`, the TUI's live buffer) discard partial deltas. The partial-deltas-then-reset ordering is benign (both consumers reset).
- **Concurrency discipline** — `Compact` reads `liveEntries()` under `a.mu` (snapshot copy, `compaction.go:36-38`), updates `usage/cost/anchorValid` under the lock (`compaction.go:120-123`), and `turn()` updates totals/anchor under the lock (`agent.go:188-190, 214-217`). `request()` snapshots entries under the lock before rebuilding (`agent.go:130-132`). `ContextTokens`/`Status`/`TakeSteering`/`Steer` all lock. The TUI never calls `Compact` on the event loop (`compactCmd` runs it in a `tea.Cmd` goroutine) and the `compacting` flag (Medium #1's gap aside) gates the mutating commands. The event pipe decouples emit from `Program.Send`. No data race found by trace; `-race` green.
- **`session.Messages` rebuild edges** — missing `firstKeptEntryID` falls back to the compaction entry itself (`rebuild.go:31-36`); tool_use entries attach to their assistant message only when the message is inside the kept window (`rebuild.go:49-53`), so a cut before an assistant message keeps its calls and results together; a kept user message merges into the summary user message (`rebuild.go:37` + `appendUser`, tested in `TestMessagesFromCompaction` asserting `m[0].Content[1].Text == "kept question"`); multiple compactions resolve to the latest via backward scan (`LatestCompaction`). One nuance verified by trace: when the fallback `start = idx` fires, `entries[start:]` begins *with the compaction entry itself*, which the switch skips (TypeCompaction is not matched) — correct, no duplication.
- **`Compact` accounting** — before/after tokens, entry shape (`TokensBefore`, `Usage`, `ReadFiles`, `ModifiedFiles`), usage added to totals, cost computed on the cheap model, anchor invalidated, `Compacted` emitted with the post-compact estimate. `TestManualCompactWritesEntryUsageAndRebuild` covers totals + entry + next-request rebuild; `TestAutoCompactionShrinksContext` covers shape + no orphaned calls.
- **CLI contract** — bad id → exit 2 listing recent ids; exclusive flags → usage error exit 2; unset key at first request → exit 2 naming the variable; stdout stays final-text-only; shared `resolveResume` for `-p` and TUI. All pinned by `TestResumeAndContinueOneShot` and `TestResumeUnsetKeyFailsAtFirstRequest`.
- **Resume repair idempotence and error paths** — the fix loop iterates `session.Repair(entries, …)`'s pre-computed slice (no recompute-as-you-append loop), `Repair` is idempotent on the written file (`TestRepair`), and every `Resume` failure branch after `session.Open` closes the writer (`resume.go:33, 39-41, 53-56, 59-61`). `TestResumeNotASession` covers the missing-file path.
- **Resume trust** — yolo only from `o.Yolo` (`start.go:81-83` applies it in the shared `build`; `Resume` never reads the header's `Yolo` or `permission_mode` entries). `TestResumeNeverRestoresYolo` covers both directions.
- **Security** — untrusted content (summaries, tool output) reaches the summary request as prompt text only, never a shell, path, or log line; the TUI sanitizes the `Resumed` line's content (`printlnContent`) and `Compacted` prints only integers. Session-file trust assumption: `Resume` trusts the file's stored workdir/prompt by design (§8) — a user who resumes a session from an attacker-planted file jails at the attacker's workdir, which is the documented threat model (permissions are guard rails; the session dir is user-private, mode 0600/0700).
- **Dependency direction** — `compact` imports only `llm`; `session` imports `llm`; `agent` imports both + `provider`; `tui` imports `agent`. No cycles; matches DESIGN §2.

## Earlier findings

Pass 1 (`docs/reviews/2026-10-05-phase-4-context-compaction-resume.md`, same tree) reported 1 Medium + 7 Low. Re-verified each against the code this pass:

- **M1 — auto-compaction aborts a healthy run when the cheap model's credential is missing** — **holds, Confirmed.** `Compact` → `summarizer()` → `Providers.Resolve(Config.Model)` (`compaction.go:73-77`) resolves the `env:` credential eagerly; `Run`'s checkpoints call `maybeCompact` only when the estimate is over the trigger, so the failure lands exactly on a long-running session: `maybeCompact` returns the `EnvError` (a usage-class error) from the middle of a healthy run, which `-p` maps via `exitFor` → 2. A run that was proceeding fine with the *current* model dies because the *summary* model's key is unset. (The key's env var is resolved at request-build time by design — the finding is that `maybeCompact` surfaces it as a run-fatal error instead of skipping auto-compaction.)
- **L1 — `CapChars` splits a multi-byte rune** — **holds, Confirmed by probe** (`Compact(… keepRecent=50, cap=3000)` on a 200 000-char single-message history: the summarizer receives a 3013-byte payload prefixed with the omission marker; `CapChars` on a newline-free CJK payload strands a continuation byte `0x89` at the seam — byte-level, reproduced). Same pass-1 assessment: byte-slicing is what the plan specifies, bounded harm.
- **L2 — `TrackFiles` does not normalize path spellings** — **holds, Confirmed by probe:** `read a.go` + `edit ./a.go` leaves the file in **both** lists (`read=[a.go], mod=[./a.go]`); the summary then tells the model a modified file is still only-read. (Differs from pass 1 only in the concrete spelling pair demonstrated.)
- **L3 — `liveEntries` lacks the missing-`firstKeptEntryID` fallback that `Messages` has** — **holds, Confirmed by reading.** `session.Messages` falls back to `start = idx` (the compaction entry itself) when the kept id is gone (`rebuild.go:31-36`), but `Agent.liveEntries` (`compaction.go:43-48`) silently leaves `start = 0`, so a trimmed/corrupt file makes the *next* compaction re-summarize everything before the summary as if it were fresh history. Same tree, same gap.
- **L4 — loop guard can name near-empty entries when the overage is system+tools** — **holds, Plausible** (not reproduced here; the arithmetic in pass 1 traces: `largestEntries` reports only transcript entries, so a window dominated by a huge system prompt + tool specs names irrelevant entries before stopping the run).
- **L5 — manual `/compact` is uncancellable and unbounded in wall time** — **holds, Confirmed by reading.** `compactCmd` runs `a.Compact(context.Background())` (`app.go:563`) — no cancellation, and the summarizer call inherits the provider's own retry ladder (5 attempts × backoff + 90 s stall timeout), so a hung summarizer keeps `compacting…` on the bar with esc/ctrl+c ineffective (esc only cancels `m.cancel`, which is nil for a compaction). Recovers eventually via the provider timeouts; a UI dead-end in the meantime.
- **L6 — `textOf`/`messageText` duplication** — holds (cosmetic; recurrence of a deferred phase-3 item).
- **L7 — Review Focus 3's "`--model` can override" resume path has no test** — **holds, Confirmed.** No test resumes with `o.Model` set (`grep` over `resume_test.go`/`cli_test.go`: no `--model` on a resume path); `Resume`'s `SetModel` branch (`resume.go:57-62`) is untested. A focus item without a test is a finding per the process.
- **Pass-1 L2 (retry transient surviving a healthy run) — retracted here.** `TurnEnd` clears `m.status.Transient` (`app.go:682`) before any next run, and the compaction paths clear it in `handleAgent`/`handleCompactDone`; the stuck-transient scenario pass 1 sketched cannot occur. (This pass initially mis-found the same thing independently as its own Medium; the trace above is the correction.)
- **Pass-1 L8 (recount of `go test -list` vs SPECS' "252 top-level tests")** — not re-run here; pass 1's count was verified against SPECS §15 directly.

No phase-3 regression found in the touched `tui` code: the pass-2 eventPipe gating is intact (`bridge.go` mechanism unchanged), `Sanitize` is applied to the new `Resumed` output line (`printlnContent`), and `Compacted` prints integers only.

## Suggested order of fixes

1. Medium #1 — the `!`-during-`/compact` guard (one line, prevents a note silently vanishing from the rebuilt context).
2. Medium #2 — resume workdir validation / error wrapping (prevents a raw jail error and a misleading repair append).
3. Low #2/#3 — serialize guards (one-line each), then Low #4 stable sort if NFS sessions are a real use case.
4. Low #5 — optional: add a direct request-count assertion to the overflow test.
5. Pass-1 carry-overs, cheapest first: L7 (a resume `--model` override test), L3 (mirror `Messages`' fallback in `liveEntries`), M1 (decide whether an unset summarizer key should kill the run or skip auto-compaction — surface as a warning instead), L5 (bound `/compact` with the run context so esc cancels it), L2 (normalize `path` spellings in `TrackFiles`).

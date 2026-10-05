# Review: phase 4 — context manager, compaction, resume (pass 3)

Date: 2026-10-06
Branch: phase/4-context-compaction-resume   Range: main..HEAD (merge-base 0e3f3f5 → 9a04437; 19 commits, 34 files changed, +2417/−76)
Plan: docs/plans/phase-4-context-compaction-resume.md (incl. Implementation notes and "Review fixes")   Spec: DESIGN.md §6, §8, §12.5 (+ §7.5 for resume-yolo); docs/specs/SPECS.md; README status line
Follows up: docs/reviews/2026-10-05-phase-4-context-compaction-resume.md (pass 1) and docs/reviews/2026-10-05-phase-4-context-compaction-resume-pass-2.md (pass 2). This is the third pass, written against the tree *after* the review fixes (`9cd31ab`, `902fb26`, `64cdc62`, `3523ba5`, `03ba2e0`, `27f2700`, `5ef8083`, `f75305b`, `9a04437`).

## Verdict

**Approve with fixes.** The phase delivers every plan task, all five Review Focus items have code and a test that can fail, and every fix claimed in the plan's "Review fixes" section is really in the code. No High findings. The new findings are all on the resume path, which the two earlier passes covered mostly from the happy side: `--effort` is silently ignored on resume, a stored model that is no longer configured makes the session unresumable (and `--model` cannot rescue it), and resuming a session that another moca process is writing to corrupts its transcript.

## Summary

Phase 4 adds `internal/compact` (budgets, cut points, lossy serialization, cumulative file tracking, input capping, structured summaries with split-turn merge), compaction in the agent (two trigger checkpoints, loop guard, one compact-and-retry on provider overflow), `session.Messages` honouring the latest compaction, `session.Find`/`FindForWorkdir`, `agent.Resume` with crash repair, `--resume`/`--continue` in `-p` and the TUI, and the TUI `/compact`.

I read every changed non-test file in full (`compact/*`, `session/{find,rebuild}.go`, `agent/{agent,compaction,resume,start,control}.go`, `cmd/moca/{cli,oneshot,tui}.go`, the `tui/app.go` diff and `bridge.go`), the compaction, resume, CLI and TUI compaction tests, the plan, DESIGN §6/§7.5/§8/§12.5, the SPECS diff, and both earlier reviews. I re-verified each earlier fix against the code and ran four probes with `go test -overlay` from `/tmp/probe` (nothing in the repo was touched): stored model removed from config, `--effort` on resume, resume while another writer holds the session, and a catalog model to show effort clamping.

Counts: **0 High · 3 Medium · 6 Low.**

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` | clean (no output, exit 0) |
| `go vet ./...` | clean (exit 0) |
| `go test -race -count=1 ./...` | exit 0; the condensed output reads `282 passed in 12 packages`, no failures. (The first attempt was blocked because a hook rewrote the command, so I re-issued it once; the second ran. The harness condenses `go test` output, so I have counts but no per-package lines.) |
| `go test -list '.*' ./... \| grep -c '^Test'` | 262 top-level tests, matching the figure in SPECS §15 and the plan's review-fixes note |
| Probe: stored model no longer in config, resume with and without `--model` | both fail: `resume: session workdir <dir>: unknown model "fake/m"` (finding 1) |
| Probe: `Resume(Effort:"high")` on a session stored at `low` (catalog model `opencode-go/glm-5.3-flash`) | resumed effort is `low` (finding 2) |
| Probe: second `Resume` on a file whose first writer is mid-tool, then the first writer records its result | two `tool_result` entries for `c1` (`interrupted — moca exited before completion` and `real output`); both appear as tool_result blocks in the rebuilt request (finding 3) |
| `git status --short` before writing this file | clean (probes live under `/tmp/probe`) |

Not verified: the three live gate steps from Task 9 (TUI long session on a real repo, small local model, `kill -9` mid-tool + `--continue`) and the loop-guard live reproduction. They need a real provider key and a terminal. The plan's Implementation notes and SPECS §15 record them as run on 2026-10-05, and I took that as recorded. The unit gates named in Task 9 all run inside the green suite above.

## Plan conformance

| Task | Status |
|---|---|
| 1 Budget | Done. `NewBudget` scales both values to `window/4`; `Over` is strict `>`; `TestBudget` pins both. |
| 2 Entry view + cut | Done. Cut only at user/assistant; smallest fitting cut, else the last valid one; `turnStart` set only for assistant cuts. The whole-window refusal (`cut.go:16-18`) is recorded deviation 1. |
| 3 Serialization / tracking / capping | Done. Labels, 200-char argument cap, 2K result cap, lossy header, cumulative `TrackFiles`, tail-keeping `CapChars`. All byte-cuts are now rune-safe (`safeTail`, continuation-byte skip). |
| 4 Summary + `Compact` | Done. Split-turn two-call merge, `turnStart==0 && prev==""` skips call A, usage summed, file lists appended. |
| 5 Rebuild + agent compaction/overflow | Done. Checkpoints at `Run` start and after each batch (after steering), loop guard text matches the plan, overflow = `ErrContextOverflow` or `StopLength` within reserve, nothing persisted on the first overflow, one retry, second overflow surfaces the original error. |
| 6 Find / FindForWorkdir | Done. 8-hex validation, stable mtime order, first-line header reads, canonical path comparison. |
| 7 `agent.Resume` | Done, with the gaps in findings 1–3 and 7. Stored prompt verbatim, header workdir as jail root, repair before build, totals and snapshots restored, yolo only from the resuming process. |
| 8 CLI + TUI wiring | Done. Exclusive flags exit 2, lookup failure exit 2, `resolveResume` shared, `ResumePath` branch, `/compact` as a `tea.Cmd` with busy guards and esc/quit cancellation. |
| 9 Gate | Unit gates pass. README status line and SPECS updated; SPECS §15's test count (262) is correct. Live gates not re-run here. |

Deviations: all seven Implementation notes and the "Review fixes" section are recorded and justified, and I found nothing unrecorded. Nothing outside the phase scope or on the non-goals list was touched.

Two observations that are design decisions rather than findings. (a) Resume replays the stored system prompt verbatim (§8), so project instructions that were trusted when the session was created stay in the prompt even if the resuming run says `--no-approve`; the flag only affects what the *resuming* process loads fresh. SPECS' resume paragraph does not say so. (b) Steering that is queued after a tool-less final turn (`agent.go:293-299`) starts another turn without the §6 pre-prompt trigger check; the overflow recovery is the backstop.

## Review Focus

1. **A single tool result larger than `keepRecent`** — satisfied. `FindCut` can only cut at user/assistant messages (`cut.go:20-27`), so the result travels with its call; when it cannot fit, `maybeCompact` stops with the loop-guard error (`compaction.go:171-174`). Proven by `TestFindCutNeverBetweenCallAndResult` and `TestLoopGuardNamesOversizedEntry` (a real 50K-char `read` on a 16K-window model; asserts `read` and `12k tokens`). Can fail.
2. **`/compact` with nothing to summarize** — satisfied. `FindCut` refuses when the whole window fits `keepRecent`; `Compact` returns `ErrNothingToCompact` with no summarizer call and no entry (`compaction.go:128-130`). Proven by `TestCompactNothing` (compact), `TestManualCompactNothing` (asserts the request count is unchanged) and the TUI `TestCompactNothing`. Can fail.
3. **`--resume` with the stored model's key unset** — satisfied for the stated case: resume never resolves credentials, the first request fails with the env error (exit 2), and `--model` overrides it. `TestResumeUnsetKeyFailsAtFirstRequest` and `TestResumeModelOverride` (cli_test.go) pin both. The sibling case where the stored model is *gone from the config* is not satisfied: finding 1.
4. **`--continue` through a symlinked workdir** — satisfied. `FindForWorkdir` uses `Abs` + `EvalSymlinks` (`find.go:91-94`); `TestFindForWorkdirViaSymlink` pins it.
5. **Summarizer input capped to the cheap model's window** — satisfied for ordinary text: every payload goes through `CapChars` with `(window − maxTok − len(SummarySystem)/4 − 512) × 4` chars (`compaction.go:100`, `summary.go:59`); `TestCompactCapsInput`. The cap assumes 4 chars per token, so denser content can still overflow a small summarizer window: finding 6.

## Findings

### Medium

#### 1. A stored model that is no longer configured makes the session unresumable, `--model` cannot rescue it, and the error blames the workdir — Confirmed
`internal/agent/resume.go:59-70` (also `internal/agent/agent.go:74`)

`Resume` passes the *stored* model (latest `model_change`, else the header) to `build`, and `New` resolves it immediately. When that model is no longer in the config (provider renamed or removed, model dropped from `providers.*.models`), `build` fails before the `o.Model` override at lines 71-76 can run. The failure is also wrapped as `resume: session workdir <dir>: …`, which points at the wrong cause.

Failure scenario: a session was started with `fake/m`; the config now declares only `other/x`. `moca --resume last --model other/x -p "go on"` prints `moca: resume: session workdir /tmp/…: unknown model "fake/m"` and exits 2. Without `--model` the message is the same. The probe reproduced both. The only way back into the session is to re-add the old model to the config.

Proposed fix: when `o.Model` is set, hand `build` that model (and let the override write the `model_change`) instead of the stored one. Wrap only the jail/workdir failure (a `*StartError` from `NewJail`) with the workdir text; for an unknown stored model, say so (`session was using <model>, which is not configured; pass --model`). Add a resume test where the stored model is missing from the config.

#### 2. `--effort` is silently ignored on resume — Confirmed (probe)
`internal/agent/resume.go:59-64`

DESIGN §12.5 defines `--effort <level>` as "override effort for this session". `Resume` takes the effort only from the stored header or `model_change` and never reads `o.Effort`. The flag is accepted and validated by `parseArgs`, then dropped.

Failure scenario: a session was stored at effort `low` on `opencode-go/glm-5.3-flash`. `moca --continue --effort high -p "now think hard"` runs at `low`. The probe resumed with `Effort: "high"` and got `low`. The user pays for and gets a lower reasoning level than they asked for, with no message.

Proposed fix: after `build` (and after the `--model` override), if `o.Effort != ""` parse it and call `a.SetEffort(...)`, which clamps and writes the `model_change`. Cover it with a catalog model (custom models clamp every effort to `off`, see finding 8).

#### 3. Resuming a session that another moca process is writing to corrupts its transcript — Confirmed (probe; provider rejection Plausible)
`internal/agent/resume.go:47-58`, `internal/session/writer.go:81-99`

`session.Open` takes no lock, and `Resume` runs crash repair on every unanswered `tool_use`. If a second terminal runs `moca --continue` (or `--resume last`) while the first moca is still executing a tool, the second process treats the in-flight call as a crash victim and appends `interrupted — moca exited before completion`. The first process then appends the real result for the same call id. Each process also keeps its own in-memory mirror, so afterwards neither sees the other's entries, and the file holds two interleaved histories.

Failure scenario: terminal A runs `shell make test` (a long call); in terminal B, `moca --continue`. The probe produced two `tool_result` entries for `c1` (the interrupted one and `real output`), and `session.Messages` rebuilt both as tool_result blocks. Providers reject a request with two results for one `tool_use` id, so every later resume of that file would fail (the rejection itself I did not run). Even without an in-flight tool, two live writers interleave user/assistant messages in one file.

Proposed fix: hold an exclusive advisory lock for the writer's lifetime (a `<session>.lock` file with PID/`flock`, taken in `session.Create`/`Open`), and make `Resume` fail with `session <id8> is open in another moca process` instead of repairing. DESIGN §8 is silent on this, so note the rule in SPECS.

### Low

#### 4. The overflow-recovery path swallows cancellation — Confirmed by reading
`internal/agent/agent.go:256-259`

In `turnWithRecovery`, when `Compact` fails the function returns `orig` (the provider overflow error) regardless of why it failed. If the user presses esc (or SIGINT arrives) while the recovery compaction is running, `Compact` returns a wrapped `context.Canceled`, which is discarded.

Failure scenario: a run overflows, the recovery compaction starts, the user presses esc. `handleRunDone` only prints `[interrupted]` for `errors.Is(err, context.Canceled)` (`app.go:756`), so the TUI shows `error: <provider context-length message>` instead. In `-p` the exit code is still 130 (`exitFor` checks `ctx.Err()`), but stderr carries the overflow text.

Proposed fix: `if errors.Is(cerr, context.Canceled) || ctx.Err() != nil { return resp, nil, cerr }` before returning `orig`, plus a one-line test.

#### 5. Tokens spent by a failed or partly failed compaction are never counted — Confirmed by reading
`internal/compact/summary.go:58-62,70-76`, `internal/agent/compaction.go:124-127`

`call` adds each response's usage to a local total, but every error path returns `Result{}`, and `Agent.Compact` only records usage from a successful `Result`. In a split-turn compaction, call A can succeed and call B fail; A's tokens are billed but appear nowhere in the totals or the session file, and the retry pays for A again. The plan's constraint is "the compaction summary's usage counts toward session totals". Cost accounting is a headline claim of the project.

Failure scenario: split-turn compaction, the second request hits a 5xx after retries. The status-bar and exit-summary cost omit the first summary call.

Proposed fix: return the accumulated usage alongside the error (e.g. `Result{Usage: usage}` with the error) and have `Agent.Compact` add it to the totals and write it on an `error` entry before returning.

#### 6. The summarizer input cap has no tokenizer-skew margin and no recovery when the summarizer itself overflows — Plausible
`internal/agent/compaction.go:100`, `internal/compact/summary.go:59`

The cap is `(window − maxTok − system/4 − 512) × 4` bytes, i.e. exactly the chars/4 estimate with about 2K chars of slack. Code, JSON and non-Latin text commonly run at 2–3.5 chars per token, so a payload at the cap can exceed the summarizer's window. Compaction then fails with the provider's overflow error and the loop guard or the run aborts. This only bites when the cheap `model` has a small window (a local model), which is exactly the §6 small-window scenario. I did not reproduce it against a real tokenizer.

Proposed fix: either budget at 3 chars/token for the cap, or treat a summarizer overflow as "halve the cap and retry once" inside `Compact`.

#### 7. Hard mode is not restored on resume, so `/hard` cannot return to the cheap model — Confirmed by reading
`internal/agent/resume.go:59-64`, `internal/agent/control.go:89-107`

A session that ended in `/hard` has a `model_change` to `modelHard`. Resume restores that model, but `Agent.hard` (the saved pair) is nil. The next `/hard` therefore "enters" hard mode, saving `modelHard/high` as the pair to return to. A second `/hard` returns to the same model. The status bar shows no HARD indicator while running the expensive model. DESIGN says resume restores model and effort, so this is a gap in an unspecified corner, not a violation.

Failure scenario: resume a session left in `/hard`; press `/hard` expecting the default model. The session stays on `modelHard`; only `/model <cheap>` gets out.

Proposed fix: on resume, if the restored model equals `Config.ModelHard` and differs from `Config.Model`, set `a.hard` to the config model at its default effort. Alternatively document that resume does not restore hard mode.

#### 8. Two stated behaviours have no test that can fail — Confirmed
`internal/agent/resume_test.go:19-39`, `internal/agent/compaction_test.go`

- `TestResumeRebuildsExactContext` asserts `b.Effort() != a.Effort()` after `a.SetEffort(llm.EffortLow)`, but the test model is a custom one (`ThinkingMode: "none"`), for which `ClampEffort` always returns `off`. Both sides are `off` on every run, so the effort-restore assertion cannot fail. The model-restore half of §8 ("`model_change` … resume restores both") has no resume test at all; only the `--model` override is tested. Findings 1, 2 and 7 all sit in code those tests were supposed to pin.
- A repeated compaction is tested only at the `compact` level (`TestCompactSimple` with a `Prev`) and by `TestLiveEntriesMissingFirstKeptFallback`. No agent-level test runs two compactions and asserts that the second starts at the first one's `firstKeptEntryId`, feeds the first summary in, and accumulates the file lists, which is a Global Constraint. If `liveEntries` started at 0 instead, the suite would stay green.

Proposed fix: run the resume test on a catalog model with `ThinkingLevelMap` (as in the probe) and add a `/model` switch followed by a resume; add one agent-level two-compaction test asserting the second summary request contains the first summary and that `firstKeptEntryId` advanced.

#### 9. `Start` repeats everything `build` does — Confirmed
`internal/agent/start.go:87-161` vs `:45-85`

The plan extracted `build` so `Start` and `Resume` would share setup. `Start` still builds its own registry, calls `skills.ExtractBuiltins` and `permissions.NewJail`, then `build` does all three again. The first registry is discarded after `Resolve`. Behaviour is the same today, but the two copies already differ in how they apply `o.Model` and it is easy for them to drift (for instance, a future change to the retry notifier would need two edits).

Proposed fix: have `build` accept the registry and jail it needs, or have `Start` reuse `build`'s results by constructing the prompt inputs from `build`'s jail root.

## Earlier findings

Pass 1 and pass 2 (`docs/reviews/2026-10-05-phase-4-context-compaction-resume*.md`). All items the plan lists as fixed are fixed in the code and have a test:

- **P1-M1 auto-compaction killed healthy runs when the cheap model's key was missing** — still fixed. `summarizerUnavailable` (`compaction.go:36-39`) skips with a `Warning`; `TestAutoCompactionSkipsWhenSummarizerUnavailable` asserts the warning, no compaction entry and a completed run. Side effect: the warning repeats after every tool batch while the estimate stays over the trigger. Noisy, not wrong.
- **P2-M1 `!` during `/compact`** — still fixed (`app.go:428-433`); `TestCompactingBlocksRun` includes the `!ls` case.
- **P2-M2 resume of a missing workdir** — fixed: `Resume` reads and validates the workdir before opening the writer (`resume.go:33-46`); `TestResumeMissingWorkdir` asserts the file is untouched. The `build` error wrapping added alongside it is the root of finding 1's misleading message.
- **Rune-safe cuts, `TrackFiles` normalisation, empty assistant text, `liveEntries` fallback, fixed-overhead note, `/compact` cancellation, `llm.TextOf`, `--model`-on-resume test, direct request-count assertion** — all present in code and tests as described. `TestResumeModelOverride` genuinely exercises the override, but not the missing-stored-model case (finding 1).
- **P2-L4 `Find` mtime ties** — fixed as asked (`SortStableFunc`). Note the tie order is ascending file name, which is deterministic but not "newest": with date-prefixed names an older session wins a tie. Only matters on coarse-mtime filesystems; no action needed.
- **P2-L1 `largestEntries` prints raw JSON inputs** — still open, recorded as not changed in the plan; cosmetic.

Phase 1–3 reviews: nothing regressed in the code this range touches. The phase-2 truncated-tool-call handling coexists with the overflow branch (overflow returns before any persistence); the phase-3 event-pipe buffering is what delivers `Resumed` before the program exists; the pass-2 `textOf` dedupe is now done (`llm.TextOf`).

## Suggested order of fixes

1. Finding 3 (single-writer lock for session files): it is the only one that can leave a session permanently unusable.
2. Findings 1 and 2 together (both in `Resume`, both are "the resuming process's flags must win"), with the catalog-model test from finding 8.
3. Finding 4 (one condition) and finding 5 (return usage with the error).
4. Finding 7 (restore or document hard mode) and finding 6 (cap margin).
5. Finding 8's two-compaction test and finding 9 as clean-up.

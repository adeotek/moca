# Review: phase 3 — TUI shell, slash commands, status bar, steering, model switching

Date: 2026-10-05
Branch: phase/3-tui   Range: main..HEAD (merge-base b43c427 → 1cf4132; 12 commits, 43 files, +3071/−49)
Plan: docs/plans/phase-3-tui.md (incl. Implementation notes)   Spec: DESIGN.md §3, §4, §6, §7, §8, §11, §12 + phase plan item 3; docs/specs/SPECS.md §3.5, §4, §13, §14, §15
Follows up: none — this is a parallel second review of the same phase. `docs/reviews/2026-10-05-phase-3-tui.md` (pass 1) covers the same range independently; where both reviews found the same defect it is noted per finding.

## Verdict

**Approve with fixes.** The phase is structurally sound: the concurrency design (event pipe + mutex + between-runs-only commands) is coherent, the Sanitize discipline is applied consistently on the audited surfaces, and all machine checks pass. One High: `tui.Run` crashes the whole program (nil `Program.Send` panic) when `agent.Start` emits skill-discovery warnings before the program exists — reproduced. Two Mediums and a set of Lows round it out.

## Summary

Phase 3 delivers the inline Bubble Tea v2 TUI (scrollback via `tea.Println`, live region, status bar, pager), the pure-layer state machines (input/chips/history, items, commands, status/sanitize/format), agent additions (cross-provider transform, model/effort/hard switching, steering, anchored estimate, status), the comment-preserving `AppendString`, prompt templates, bridge/pager/trust, and the CLI entry with the trust decision.

I read every changed non-test file in full and every test file, traced every goroutine-crossing path (events, asker, steering, abort, run cancellation, `/clear`, quit), probed terminal-safety claims against the pinned renderer stack (bubbletea v2.0.10 cursed renderer + ultraviolet v0.0.0-20260811), and reproduced two candidate findings with scratch programs outside the repo.

Counts: **1 High, 2 Medium, 6 Low.** No concurrency deadlock or race was found beyond those reported (the event-pipe, asker, and mutex design holds up under scrutiny).

## Checks run

| Check | Result |
|---|---|
| `mise x go -- gofmt -l .` | clean (no output, exit 0) |
| `mise x go -- go vet ./...` | clean (no output, exit 0) |
| `mise x go -- go test -race -count=1 ./...` | passed — 12 packages `ok` (`permissions/probe` has no test files), exit 0, 5.2 s |

Not verified (live/manual gate steps, per the plan's Task 13 and Implementation notes): the tmux mock-session gate evidence and the live multi-provider `/model` switching (requires Ben's OpenCode key; the plan records both as run on 2026-10-05 — taken as recorded, not re-executed). Real-terminal key behaviour (`shift+enter`/hint, `alt+p`, paste, pager in a real terminal) and OSC 52 over SSH are inherently manual and were not exercised. The `x-opencode-session` header is registered at registry construction only (verified by reading `internal/provider/registry.go:112-121`) — per-conversation rebinding after `/clear` is deferred by design and remains an item to watch in phase 4.

## Plan conformance

All 13 tasks are implemented and match the plan's file structure; the commit list mirrors the task commits. Deviations are recorded in the plan's Implementation notes and check out against the code:

1. Qualified thinking-replay comparison (`a.m.Qualified()` in `anthropic.go:60`, `openai_responses.go:38`) — matches note 1; pinned by `TestAnthropicForeignThinking`, `TestResponsesForeignThinking`, `TestTransformSameBareIDDifferentProvider`.
2. `/model`/`/effort`/`/hard`/`/yolo`/`/clear` refused while running (`refuseRunning`, app.go:314) — matches note 2.
3. Agent mutex (`agent.go:54`, guarding queue/transcript/usage/anchor; `request` snapshots under lock at agent.go:121) — matches note 3.
4. Event pipe instead of direct `Program.Send` for `Emit` (bridge.go:55-80, app.go:646-647) — matches note 4; the asker keeps a direct send (app.go:648) and is only invoked from the run goroutine (tools `approve()`, called from `Run`'s sequential tool loop) — correct.
5. `println` (trusted/styled) vs `printlnContent` (untrusted) split (app.go:76-82); history recall re-chips unsafe entries (`setFromHistory`, input.go:106) — matches note 5.
6. `startTestWith` wrapper, `handleRunDone` live flush, abort re-fire in the multi-call abort test — match note 6.

Global constraints spot-checked: status bar field set/drop order (status.go), no alt-screen except pager (app.go:566-569), steering placement (agent.go:288-292 after the batch; 257-266 continue on no-tool turns), `/hard` toggle semantics (control.go:79-97), builtin-names-win (commands.go:61), yolo bar field (app.go:596-601), trust prompt only for untrusted dirs with project resources (tui.go:29-51). Gate checkboxes in the plan are ticked but that gate is manual; see "Not verified".

## Review Focus

1. **ANSI/`\x1b[201~` paste** — satisfied. `hasControl` chips any control-byte paste (input.go:41), `Sanitize` shows `^[`-style caret notation (sanitize.go), chips never expand (`unsafe` flag, input.go:80), recall re-chips (`setFromHistory`). Tested: `TestPasteWithControlBytesIsChipped`, `TestHistoryRecallChipsUnsafeContent`.
2. **Narrow terminal status bar** — satisfied. `RenderStatus` drops cwd → branch → tokens, truncates the model, hard-clips (status.go:66-81). Tested at 100/80/60/40/20 cols (`TestRenderStatusNarrow`). YOLO leader reserves width (app.go:599), no width test for the 60-col YOLO case itself (gate evidence covers it manually).
3. **Approval while typing** — satisfied. Approval keys consume `a`/`A`/`d`/`esc` only; every other key forwards to the textarea (app.go:227-231); the draft is never touched (tested in `TestApprovalPreservesDraft`). `esc` denies (app.go:224).
4. **`/model` with unset key** — satisfied. `CheckCredential` resolves env before any mutation (registry.go:89-99; control.go:55), rollback on resolve failure; no `model_change` on failure. Tested: `TestSetModelRefusedWhenKeyMissing`.
5. **Steering + esc** — satisfied. Abort leaves the queue untouched (`applySteering` is not called on the abort paths, agent.go:269/285); `handleRunDone` drains and prepends (app.go:548-551). Tested: `TestAbortReturnsSteering`.

## Findings

### High

#### 1. `agent.Start` warnings crash the program: nil `*Program.Send` panic — Confirmed (reproduced)
`internal/tui/app.go:646-649` (with `internal/agent/start.go:74-79`)

`Run` wires `pipe := newEventPipe(func(msg) { p.Send(msg) })` where `p` is still nil, then calls `agent.Start(o.Start)` with `Emit` already set. `Start` emits a `Warning` event for every skill-discovery problem (`start.go:75-79`), and it does so before `p = tea.NewProgram(...)` on line 656. The pipe's forwarder goroutine picks the event up and calls `p.Send(...)` on a nil `*Program`, which dereferences `p.ctx` → SIGSEGV, taking down the whole process. The same nil-`p` closure also sits behind `o.Start.Ask` (app.go:648), though that path needs a run to fire so it cannot precede assignment. The existing `close(pipe.stop)` on line 651 shows the author considered pre-`Run` failure but only after `Start` returns — events emitted *during* `Start` still race through.

Failure scenario (reproduced with the built binary, scratch dir, repo untouched): a global skill dir containing `skills/broken/SKILL.md` with frontmatter but no `description` (`$XDG_CONFIG_HOME/…/moca/skills/broken/`), run `moca` in any directory:
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 ...]
charm.land/bubbletea/v2.(*Program).Send(...)
    charm.land/bubbletea/v2@v2.0.10/tea.go:1194
github.com/adeotek/moca/internal/tui.Run.func1(...)
    /home/toor/projects/moca/internal/tui/app.go:646 +0x2e
github.com/adeotek/moca/internal/tui.newEventPipe.func1()
    /home/toor/projects/moca/internal/tui/bridge.go:66 +0x6a
```
Any malformed skill in `~/.config/moca/skills/` (a realistic end-user state) makes `moca` exit 2 with a raw Go panic instead of opening the TUI with a warning line. Note the same warning is handled correctly in `-p`, which only prints.

Proposed fix: buffer pre-program events instead of sending: create the pipe with a `started` gate (queue messages until `Run` sets the program, then flush in order), or move `agent.Start` before wiring `Emit`/`Ask` and replay/emit warnings after `p.Run()` starts. A minimal variant: have `Start` return the warnings it collected and let `Run` deliver them as a first `tea.Cmd`.

### Medium

#### 2. Malformed (e.g. crash-truncated) config panics `AppendString` instead of erroring — Confirmed (reproduced)
`internal/config/edit.go:50-114` (panic at `scanner.value`, reached from `AppendString` at edit.go:172)

The scanner's bounds checks are incomplete: `value()` checks `s.i >= len(s.b)` only at entry (edit.go:53), but the `ws()`/`str()`/`s.i++` steps inside the object and array loops index `s.b[s.i]` past the end on truncated input. `Standardize` (the pre-pass) does not reject truncation — a truncated file has no unterminated-comment or unterminated-string marker it must catch, so `AppendString` proceeds into the scanner. An allow-always write happens exactly when the user presses `A` on an approval prompt; a crash (or disk-full write, or manual edit) that truncated `config.jsonc` between session start and that press turns a UX action into a panic inside `Update` (bubbletea recovers it as `ErrProgramKilled`, killing the TUI and abandoning the approval — the agent's asker then denies via ctx, so the run dies too).

Failure scenario (reproduced with a scratch test against the package):
```
os.WriteFile(p, []byte("{\n  \"shell\": {\n    \"allow\": [\"go\""), 0o600)
AppendString(p, []string{"shell","allow"}, "python", nil)
→ panic: runtime error: index out of range [33] with length 33
    config.(*scanner).value ... edit.go:93
    config.AppendString ... edit.go:172
```
`Standardize`'s error path does catch other malformed shapes (unterminated `/*` → clean error, verified), so the exposure is truncation and any shape the pre-pass forgives but the scanner mis-walks.

Proposed fix: make the scanner total — check `s.i < len(s.b)` before every byte read in the `value` loops (and in `ws`'s caller at the `'}'`/`','` checks), returning a proper `"unexpected end"` error. Smallest change: a `func (s *scanner) peek() (byte, bool)` helper used at edit.go:62, 68, 73, 92-94 and 97-101.

#### 3. Allow-always is answered and recorded before the config write is attempted; a failed write still allows the command this time, by design — but the code answers *before* the write, so the audit trail can be lost silently — Confirmed by reading
`internal/tui/app.go:207-223`

On `A`, the reply is sent (`reply <- tools.AllowAlways`, line 213) *before* `config.AppendString` runs (line 219). If the write fails (read-only config dir, the High-2 panic above, disk full), the command runs (or the tool proceeds) and the only trace is a scrollback `error: allow-always not saved: …` line — the command is not in `shell.allow`, so the user will be prompted again next time, which is the safe direction. This matches the plan's Task 11 step 12 ("on failure print the error but still allow") and DESIGN's guard-rail model, so it is not a bug against the plan — but the ordering also means the panic case (finding 2) aborts the whole program *after* the tool was authorised, which turns a persistence error into a user-visible crash mid-approval. Keeping the answer-then-write order is fine once finding 2 makes the write total; no reordering needed, just the dependency noted.

### Low

#### 4. Retry transient sticks in the status bar when a run ends without a further TurnEnd — Confirmed by reading
`internal/tui/app.go:519` (set), `app.go:514` (cleared only in `TurnEnd`), `app.go:535-562` (`handleRunDone` never clears it)

`status.Transient` is cleared only at the next `TurnEnd`. When a run dies during retries (provider hard-fails after the retry budget, user hits `esc` mid-backoff, or the turn errors out), `handleRunDone` prints `[interrupted]`/`error:` and refreshes status but leaves `Transient` set: the bar shows `retry 2/5 · 4s` in place of the cost for the rest of the session until some later run produces a TurnEnd. Same after `/clear` — wait, `/clear` resets `m.status = StatusInfo{}` (line 439), so only the failed-run path is affected.

Failure scenario: model returns repeated 500s; run ends with `error: …`; the cost field of the bar reads `retry 5/5 · 16s` indefinitely.

Proposed fix: add `m.status.Transient = ""` in `handleRunDone` next to `m.refreshStatus()` (app.go:560).

#### 5. The `!` shell path can inject a user message that breaks openai-completions tool-result grouping — Plausible
`internal/tui/app.go:469-474` (`AddNote`), `internal/agent/agent.go:104-112` (`append`), `internal/session/rebuild.go:32-38` + `internal/provider/openai_completions.go:42-49`

`AddNote` is callable while a run is in progress (nothing refuses `!` mid-run). `Writer.Append` is mutex-guarded so the file/mirror stay consistent, and `request()` snapshots under the lock — but a `!` note appended *between* a turn's tool_result entries (i.e. while the tool batch executes) lands in `entries` before the remaining results are appended. On rebuild, `session.Messages` merges consecutive user-side content, so the note's text block and the following tool_result end up in the same user message. The anthropic adapter serialises any block order fine, but the openai-completions adapter emits `role:"tool"` messages per tool_result and builds the text message after them — a user *text* block sitting before a later tool_result inside one merged user message is silently re-ordered into a separate assistant-side message: the note text becomes the next user turn's content *before* the last tool results, i.e. the wire shape diverges from the transcript ordering the steering tests pin. I could not fully trace the exact wire outcome for every interleaving without running a live adapter test, hence Plausible; the rebuild-order claim (note text + later tool results merge into one user message) is Confirmed by reading `rebuild.go`.

Failure scenario: model calls tools A and B; between A's result and B's execution the user runs `!ls`; request 2's `messages` contain the `ls` output as user text in the same rebuilt user message as B's tool_result (anthropic) or re-ordered ahead of B's `tool` message (completions).

Proposed fix: refuse `!` while `m.running` (like the mutating slash commands), or queue `!` notes onto the steering drain instead of appending mid-batch.

#### 6. Approval prompt renders untrusted bytes without Sanitize — Plausible (renderer probes reduce severity; the contract breach stands)
`internal/tui/app.go:583` + `app.go:604-609`

`approvalPrompt` interpolates `q.Subject` (command name) and `firstLineOf(q.Detail)` (full command line) into the live region raw. Every other untrusted surface goes through `Sanitize` (live region app.go:573, status content app.go:599, items items.go:25, scrollback app.go:82). SPECS §3.5 says "model/user/tool/error text goes through Sanitize before it reaches the terminal" — the approval prompt is model-supplied text on the live region and is the one gap. I probed the pinned render stack directly (scratch test against ultraviolet v0.0.0-20260811, the cursed renderer's cell model): CSI/OSC sequences embedded in view *content* are parsed and consumed by the cell decoder before output (a raw `\x1b[2J`/`\x1b]0;…` payload rendered through `NewStyledString → Render → Flush` produced `AAABBBCCC` on the wire — nothing re-emitted), and `mvdan.cc/sh` analysis refuses/asks on the same shapes my probes exercised (`seq 'x\x1b[2Jy'` → ask, name `seq`, raw bytes preserved in `Detail`). So a real exploit path through the *live view* was not demonstrated — but the cell model is not a security boundary (it interprets, rather than neutralises, escapes; behaviour may differ across renderers/terminals for the `insertAbove` scrollback path, which writes strings verbatim), and the finding is a one-line contract fix. Belt-and-braces: raw control bytes currently have no sanitised route into the live region except this one.

Failure scenario (worst case, renderer-dependent): model emits `shell` command with an escape payload; if any byte survives the cell pipeline on some terminal, the approval line executes terminal control sequences instead of showing them.

Proposed fix: `firstLineOf(Sanitize(q.Detail))` and `Sanitize(q.Subject)` in `approvalPrompt` (or sanitize once where the prompt string is built).

#### 7. Status bar branch resolves via two synchronous `git` subprocesses per turn, from a `tea.Cmd` — dead key / slow-path hazard, and `GitBranch` failures are indistinguishable from non-git — Confirmed by reading
`internal/tui/format.go:55-62`, `internal/tui/app.go:96-99`, `app.go:516`/`561`

`GitBranch` runs `git rev-parse` then `git status --porcelain` on every TurnEnd and run end. Two issues: (a) on a workdir where `git` hangs (credential prompt, NFS stall, huge repo) the `branchCmd` goroutine stalls and the bar keeps the stale branch forever — there is no timeout, and the run itself is not blocked (good) but the "branch updates at every turn end" guarantee silently fails; (b) `rev-parse` failure and "not a git repo" are collapsed to `("", false, false)`, so a transient `git` failure flips the bar to dimmed `-` until the next turn — cosmetic but noisy. Additionally the initial `Init` races the first `WindowSizeMsg`: width is 0 until resize, so `RenderStatus(status, max(20,0))` renders at 20 cols on the very first frames if the branch message lands before the resize message — self-healing on the next event.

Proposed fix: add a short timeout (`exec.CommandContext`, 2-3 s) and distinguish exit status 128 (`not a repo`) from other failures; optionally cache the repo check.

#### 8. `alt+p` collapse can silently eat a pasted payload when two pastes share content — Confirmed by reading
`internal/tui/input.go:71-87`

`ToggleChips`'s collapse branch requires `strings.Contains(in.buf, p.content)`; two identical pastes (or one paste whose content contains another's marker-free text) make `strings.Replace(…, 1)` collapse the *first* occurrence, which may be the other paste's instance — leaving paste K expanded in `pastes` but its buffer position re-chipped while the real marker text of the first paste remains, so `Text()` then re-expands markers at wrong offsets. Simpler trigger: paste "A" twice (both inline, fine), but paste X then X again as chips: both get distinct `#K` markers, expand (buf = X+X), collapse replaces only the first X with marker #1 and *also* tries the second paste: `strings.Contains(buf, X)` is still true, so it re-chips the second occurrence with marker #2 — correct here by luck. The broken case is X then Y where Y contains X as a substring: expanding both then collapsing re-chips X's content inside Y's text with marker #1, and Y's own marker is lost.

Failure scenario: paste `hello world`, then paste `say hello world twice`; `alt+p` → `alt+p`: buffer becomes `[paste 1 line #1] say [paste 1 line #1] twice` — marker #1 twice, and `Text()` re-expands only the first occurrence per `Text()`'s single-Replace loop... actually `Text()` replaces each marker once per paste, so the duplicated marker yields `hello world` + ` say [paste 1 line #1] twice` — the sent text loses the second paste's content silently.

Proposed fix: on collapse, replace from the right (`strings.LastIndex`) or track exact offsets; or make `Text()` idempotent by expanding markers only when the marker count matches.

#### 9. Duplicated text-extraction helpers and a stale comment — Confirmed by reading
`internal/agent/agent.go:197-205` (`textOf`) vs `internal/tui/app.go:613-621` (`messageText`) — byte-identical logic in two packages; harmless now (dependency direction forbids tui→use of agent's unexported helper) but `llm` is the natural home for one shared `TextOf(m llm.Message) string` when phase 4 compaction needs the same extraction. Also `internal/tui/app.go:35` comments `start` as "Emit/Ask wired; /clear restarts from this" — accurate, but `opts` (line 34) now duplicates `start` for prompts/config and is never re-read after `Run` assigns both; one of the two fields is redundant.

## Earlier findings

Phase-1 review (`2026-10-04-phase-1-skeleton-providers.md`) — items touching this phase: none of its findings concern TUI surface; the `replaysVerbatim`/effort items were already fixed and the phase-3 qualification change (finding-level note 1) tightens rather than regresses them.

Phase-2 review (`2026-10-05-phase-2-tools-agent-loop.md`) — 15 findings, all recorded fixed with regression tests; the phase-3 range touches `agent.go` (loop), `registry.go`, and provider adapters, and none of the fixed behaviour regressed: the phase-2 ladders (edit 10-case, shell analysis, jail) pass unchanged under `-race` in this range; `TestLengthStopTruncatedToolCall` and the abort/repair tests still exercise the phase-2 contract end to end.

Parallel review (pass 1, `2026-10-05-phase-3-tui.md` — same range, independent): its four findings are distinct from the High/Medium here except two overlaps: its Medium #1 (status-bar branch regression after `/clear`) is confirmed independently here via `m.status = StatusInfo{}` (app.go:439) losing `Dirty`/`Git` until the next `branchMsg` (folded into this review's reading of restartSession; the fix is one line — call `m.branchCmd()` in `restartSession`); its Low #4 (trailing live line dropped on a failed run) is consistent with what `handleRunDone`'s `msg.err == nil` guard (app.go:542) does, and worth fixing alongside my Low #4. Its Low #2 (literal chip marker typed before a paste hijacks `Text()`) and #3 (eventPipe blocks its caller at 1024 pending events) did not recur in this review's probe list but are credible; treat all six as one backlog.

## Suggested order of fixes

1. High 1 (nil `Program.Send` on `Start` warnings) — a startup crash for any user with a malformed skill; fix with a gated/buffered pipe and a regression test that constructs `AppOptions` whose global skills dir contains a bad SKILL.md and asserts `Run` opens cleanly (or unit-level: assert the pre-program buffer flushes after `p` is set).
2. Medium 2 (`AppendString` panic on truncated config) — bounds-total the scanner; add the truncated-file case to `edit_test.go` next to the existing malformed-comment cases.
3. Low 4 + pass-1's Medium #1 and Low #4 together (status bar: clear `Transient` in `handleRunDone`, re-run `branchCmd` in `restartSession`, flush trailing live on error) — three one-line fixes in the same two functions.
4. Low 5 (refuse or queue `!` mid-run) before phase 4's compaction makes rebuild-order subtleties load-bearing.
5. Low 6 (sanitize the approval prompt) and Low 7 (GitBranch timeout/exit-code) as guard-rail hardening.
6. Low 8 (chip collapse offsets) with a regression pair (`X` then `X-containing-Y`) in `input_test.go`; Low 9 (dedupe `textOf`) opportunistically in phase 4.

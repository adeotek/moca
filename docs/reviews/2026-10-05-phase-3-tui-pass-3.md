# Review: phase 3 — TUI shell, slash commands, status bar, steering, model switching

Date: 2026-10-05
Branch: phase/3-tui   Range: main..HEAD (merge-base b43c427 → 0ceac2e; 14 commits, 45 files changed, +3556/−49)
Plan: docs/plans/phase-3-tui.md (incl. Implementation notes and Review fixes)   Spec: DESIGN.md §3, §4, §6, §7, §8, §10, §11 + phase list item 3; docs/specs/SPECS.md §3, §3.5, §4, §15
Follows up: `docs/reviews/2026-10-05-phase-3-tui.md` (pass 1) and `docs/reviews/2026-10-05-phase-3-tui-pass-2.md` (pass 2). This pass reviews the tree *after* the review-fix commit `640be8e`.

## Verdict

**Approve with fixes.** Everything the earlier two reviews found is fixed, and the machine checks are clean. The remaining problems are in the glue layer, which those reviews did not stress: output printed while the pager is open is lost, typing while an approval prompt is showing can trigger a permanent allow-always, the TUI exits `0` on SIGTERM (and intermittently hangs on it), and `!` still has a mid-run path. There are no High findings, but nine Mediums are worth fixing before phase 4 builds on this layer.

## Summary

Phase 3 delivers the inline Bubble Tea v2 TUI, the pure-layer state machines, the agent additions (history transform, model/effort/hard switching, steering, anchored estimate), `config.AppendString`, prompt templates, and the CLI entry with the trust decision. All 13 tasks are present, and all five Review Focus items still hold.

I read every changed non-test file and the test files for the TUI, bridge, config edit and agent control. I traced each earlier fix against the current code. I then probed suspected defects with throwaway tests injected through `go test -overlay` (nothing written into the repo), with a binary built to `/tmp` and driven through a pty, and with a standalone Bubble Tea program run in tmux. Seven of the findings below were reproduced that way.

Counts: **0 High · 9 Medium · 7 Low.**

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` | prints nothing (exit 0) |
| `go vet ./...` | clean (exit 0) |
| `go test -race -count=1 ./...` | exit 0; 11 packages `ok`, `internal/permissions/probe` has no test files; 195 top-level tests (`go test -list . ./...`), 0 failures |
| `git status --short` after the checks | empty before the review file was written |
| Probe: `AppendString` edge cases (empty, comment-only, `{}`, `"shell": null`, symlinked config) | run via `-overlay`; results in finding 9 |
| Probe: TUI model-level (paste at cursor, CRLF paste, approval keys while typing) | run via `-overlay`; results in findings 2, 6, 7 |
| Probe: standalone Bubble Tea program, `tea.Println` while `AltScreen` is on, in tmux | lines 3–8 of 14 never reached scrollback (finding 1) |
| Probe: built `moca`, SIGTERM through a pty, final batch of 10 runs | 5 exited with status `0`, 5 never exited and needed SIGKILL; an earlier batch of 7 had 2 hangs, and a SIGQUIT dump of one hang is in finding 5 (findings 4, 5) |
| Probe: malformed `SKILL.md` in the global skills dir, built `moca` in a pty | no panic; `warning: …/SKILL.md: missing description; skill skipped` printed (pass-2 High #1 verified fixed end to end) |

Not verified: every manual/live step of the plan's Task 13 gate (full session in real terminals, `shift+enter` on enhanced vs plain terminals, real 120-line paste, steering in a captured request payload, trust prompt flow, `/copy` over SSH, live cross-provider switching with a real key). The plan and SPECS §15 record these as run on 2026-10-05 in tmux against a mock server and a live opencode-go key; I took that as recorded and did not re-run it. The probe harness is scratch code in `/tmp`; the pre-existing tmux sessions `gate` and `live` were left alone.

## Plan conformance

All 13 tasks are implemented and the commit list mirrors them. The six Implementation-note deviations and the Review-fixes section match the code. Specifically verified against current code: qualified thinking comparison in both adapters; model/effort/hard/yolo/clear refused while running (`app.go:322`); the agent mutex; the event pipe with pre-start buffering (`bridge.go:56-100`); `println` vs `printlnContent`; `/clear` starting the replacement session before closing the old one (`app.go:435-453`); `GitBranch` bounded to 3 s per call (`format.go:55-62`); approval prompt sanitized (`app.go:621`).

Gaps in the record:

- The plan's Review-fixes section and SPECS §15 say all findings were fixed "with regression tests". There is no test for the `/clear` restart ordering and branch re-resolution (pass 1 #1), none for the `!` refusal while running (pass 2 #5), and none for the `A` allow-always path. See finding 15.
- SPECS §15 says "195 top-level tests across 10 packages"; the count is right but 11 packages have tests.
- `/clear` reverting more than yolo (finding 14) is not recorded; SPECS §3.5 mentions only yolo.

Global constraints spot-checked and honoured: no alt-screen except the pager, layout order, status-bar field set and drop order, key bindings, steering placement, `/hard` both directions, trust order (`--approve`/`--no-approve` → yolo → saved decision → prompt only with project resources), built-ins over templates. The one constraint that does not hold in practice is "finished output goes to scrollback and is never redrawn" while the pager is open (finding 1).

## Review Focus

1. **ANSI / `\x1b[201~` paste** — satisfied for C0 controls and ESC. `hasControl` chips the paste (`sanitize.go:26-33`, `input.go:42`), `Sanitize` shows `^[[201~` (`sanitize.go:13`), recall re-chips. Tests: `TestPasteWithControlBytesIsChipped`, `TestSanitize`, `TestHistoryRecallChipsUnsafeContent`. Two caveats: C1 controls are not neutralised (finding 8), and `\r` makes ordinary CRLF pastes opaque (finding 6).
2. **Narrow terminal status bar** — satisfied. `RenderStatus` drops cwd → branch → tokens, truncates the model, and hard-clips (`status.go:66-82`). Test: `TestRenderStatusNarrow`. Width is counted in runes, not terminal cells; see Low finding 16.
3. **Approval while mid-typing** — the *draft* is preserved (`TestApprovalPreservesDraft` passes and can fail), but the approval keys `a`/`A`/`d` swallow letters the user is typing (finding 2). That behaviour is what the plan specifies; the consequence for `A` was not considered.
4. **`/model` with an unset key** — satisfied. `CheckCredential` runs before any mutation (`control.go:55`, `registry.go:89-99`); no `model_change` is written. Test: `TestSetModelRefusedWhenKeyMissing`.
5. **Steering then `esc`** — satisfied. Abort leaves the queue intact; `handleRunDone` returns it above the draft (`app.go:565-568`). Tests: `TestAbortReturnsSteering` (agent), `TestPrependSteering` (input). The glue between them (`handleRunDone` → `TakeSteering` → `Prepend` → `syncTextarea`) has no test (finding 15).

## Findings

### Medium

#### 1. Output printed while the pager is open is lost from scrollback — Confirmed (reproduced)
`internal/tui/app.go:489-544` (`handleAgent` returns `tea.Println` cmds), `app.go:582-587` (`View` sets `AltScreen` while `m.pager != nil`)

`ctrl+o` and `/show` work at any time, including mid-run, and the agent keeps emitting events while the pager is open. Every completed streamed line, `▸ #n` item line, `↳ sent:` and error is delivered as `tea.Println`. Bubble Tea's `Println` is documented as "if the altscreen is active no output will be printed", and in v2.0.10 `insertAbove` (`cursed_renderer.go:757-806`) has no alt-screen guard: the text is written into the alt buffer, which is discarded when the pager closes.

Failure scenario: reproduced with a standalone program using the same pinned Bubble Tea. It printed `scrollback line N` every 300 ms and toggled `AltScreen` on a key. After closing the alt screen, lines 3–8 were missing from scrollback and 1–2, 9–14 were present. In moca: run a long answer, press `ctrl+o` for two seconds, press `q` — the streamed lines and tool items from those two seconds are gone, while item numbers keep counting, so `#7` can be followed by `#9`.

Proposed fix: while `m.pager != nil`, queue the print commands (or their strings) on the model and return them as one `tea.Sequence` when the pager closes.

#### 2. Typing during an approval prompt answers it — `A` persists an allow-always — Confirmed (reproduced)
`internal/tui/app.go:202-233`

While `m.approval != nil`, the letters `a`, `A`, `d` are consumed as answers and everything else is forwarded to the textarea. The plan specifies this, but it means a user who is typing a steering message when the prompt arrives answers it with their keystrokes. A capital `A` (the first letter of "Add …") is "always", which writes the command into `shell.allow` in `config.jsonc`.

Failure scenario (probe): draft `Please `, an approval for `python3` with `CanAlways` arrives, the user keeps typing `Add tests`. The `A` replies `AllowAlways`, `python3` is appended to the config file, and the draft becomes `Please dd tests`. Related: during the prompt `enter` is forwarded to the textarea and inserts a newline, and `ctrl+c` is forwarded and ignored, so neither sends nor quits.

Proposed fix: ignore approval keys for a short grace period after the prompt appears (about 500 ms), or accept them only when the draft is empty or the key is not part of a burst. Handle `ctrl+c` before the approval branch so quitting always works. Consider requiring `ctrl`-modified or confirmed input for `[A] always`, since it is persistent.

#### 3. `!` note can still land mid-run through the async shell path — Plausible (path confirmed by reading; wire effect not run)
`internal/tui/app.go:307-316`, `app.go:455-487`

The pass-2 fix refuses `!` while `m.running`, but `shellCmd` runs asynchronously for up to 30 s and `m.running` is false during that time. `handleShellDone` then calls `m.agent.AddNote` with no check.

Failure scenario: the user runs `!go test ./...`, and while it runs sends a prompt. A run starts; when the shell finishes during a multi-tool batch, the note is appended between two tool results. `session.Messages` merges consecutive user-side entries (`rebuild.go:30-36`), producing `[result A, text, result B]`. Providers that require tool results to be contiguous would reject that, and because the transcript is replayed on every request, the session would keep failing. I did not run it against a provider. A `/clear` during the same window appends the note to the *new* session, since `handleShellDone` uses the current `m.agent`.

Proposed fix: track an in-flight `!` (a `shellBusy` flag) and refuse new runs and `/clear` until it finishes, or hold the note in the model and append it from `handleRunDone` when a run is active.

#### 4. The TUI exits `0` on SIGTERM and swallows recovered panics — Confirmed (SIGTERM reproduced; panic path by reading)
`internal/tui/app.go:677-683`

`Run` returns `nil` for any error matching `tea.ErrProgramKilled`. Bubble Tea returns that error for external-context cancellation (SIGINT/SIGTERM via `signal.NotifyContext` in `main.go:16`) and also for recovered panics (`ErrProgramKilled` wrapping `ErrProgramPanic`, `tea.go:1037`). SPECS §3 says SIGINT and SIGTERM give exit `130` and that the TUI maps errors through `exitFor`; `exitFor` is never reached because `Run` returns `nil`.

Failure scenario: `kill -TERM <pid>` on a running TUI (built binary, pty): every run that exited did so with status `0` (6 of 6 recorded). A panic inside `Update` or `View` would likewise exit `0` after Bubble Tea prints the trace.

I rated this Medium rather than High: the exit code of an interactive session is rarely scripted and no data is lost, though it is a contract violation.

Proposed fix: return `err` unchanged except for a clean quit (`tea.Quit` already returns a nil error). Let `exitFor` map the cancelled context to 130 and a panic to 1.

#### 5. Intermittent hang on SIGTERM: two signal handlers race at shutdown — Confirmed (reproduced, goroutine dump)
`internal/tui/app.go:675`, `cmd/moca/main.go:16`

`main` installs `signal.NotifyContext(SIGINT, SIGTERM)` and `tui.Run` passes that context to `tea.NewProgram`, but Bubble Tea also installs its own signal handler (it was not disabled). Both receive the same SIGTERM.

Failure scenario: SIGTERM against the built binary in a pty never exited in 7 of 17 runs (2 of 7, then 5 of 10). A SIGQUIT dump showed the main goroutine in `Program.shutdown` → `channelHandlers.shutdown` → `WaitGroup.Wait`, with the Bubble Tea signal goroutine stuck in a channel send (`handleSignals.func1`). `kill <pid>` (or a service manager stop) then needs SIGKILL.

Proposed fix: create the program with `tea.WithoutSignalHandler()`. The `NotifyContext` plus `tea.WithContext` already delivers SIGINT/SIGTERM, and raw-mode `ctrl+c` arrives as a key. Re-run the SIGTERM loop to confirm.

#### 6. Pastes with CR or CRLF line endings become opaque, unexpandable chips — Confirmed (probe); which terminals send CR is Plausible
`internal/tui/sanitize.go:26-33` (`hasControl`), `internal/tui/input.go:41-65`

`hasControl` treats `\r` as a control byte, so any paste containing a carriage return is chipped as `unsafe`, and unsafe chips never expand.

Failure scenario (probe): pasting the three-line text `func a() {\r\n\treturn\r\n}\r\n` gives `[paste 3 lines #1]`; `alt+p` does nothing; the CRs are sent verbatim to the model. A CR-only paste (some terminals convert newlines to CR) shows as `[paste 1 line #1]`. The user cannot see what they are about to send.

Proposed fix: normalise `\r\n` and lone `\r` to `\n` in `Paste` before classification.

#### 7. A paste is inserted at the end of the buffer, not at the cursor — Confirmed (probe)
`internal/tui/app.go:157-162`, `internal/tui/input.go:41-45`

`tea.PasteMsg` is handled by `Input.Paste`, which does `in.buf += s`, followed by `syncTextarea`, which resets the textarea value (cursor to the end).

Failure scenario (probe): type `hello world`, move the cursor left six places, paste `XY` → the textarea holds `hello worldXY`. Pasting into the middle of an edited draft misplaces the text. The plan's goal says "native paste".

Proposed fix: for non-chipped pastes, forward to the textarea so it inserts at the cursor (`m.ta.InsertString`), then `pullTextarea`. For chips, insert the marker at the cursor the same way.

#### 8. `Sanitize` leaves C1 control characters untouched — Plausible (code confirmed; terminal-dependent)
`internal/tui/sanitize.go:13-18`, `hasControl` at `sanitize.go:28-33`

Only `r < 0x20` and `0x7f` are neutralised. U+0080–U+009F (for example U+009B CSI, U+009D OSC, U+0090 DCS) pass through as UTF-8 and reach the terminal via `tea.Println`. Some xterm-class terminals interpret UTF-8-encoded C1 controls. Pass 2 noted that scrollback printing writes strings verbatim.

Failure scenario: tool output or model text from a file read during a prompt-injection contains `U+009D 52;c;<base64> ST`. On a terminal that honours C1, that sets the clipboard or title instead of being shown. I did not test a real terminal.

Proposed fix: extend `Sanitize` and `hasControl` to map U+0080–U+009F to a visible form (for example `^\x80`-style or `\u{9b}`), with a test next to the `\x1b[201~` case.

#### 9. `AppendString` replaces a symlinked config with a regular file — Confirmed (probe)
`internal/config/edit.go:231-234`

The write goes to `path+".tmp"` and `os.Rename(tmp, path)` replaces the directory entry. If `config.jsonc` is a symlink (stow, chezmoi and home-manager all do this), the link is destroyed and the dotfile repo's copy is never updated.

Failure scenario (probe): `link.jsonc → real.jsonc`; after `AppendString(link, …)` the link path is a regular `-rw-------` file and `real.jsonc` is unchanged. For a read-only target such as a Nix store path, the write "succeeds" and later breaks the home-manager switch.

Proposed fix: `filepath.EvalSymlinks(path)` first and write the tmp file next to, and rename over, the resolved target. Preserve the original file mode instead of forcing `0600` while there.

### Low

#### 10. History transform keeps redacted foreign thinking as a literal `[Reasoning redacted]` text block — Confirmed by reading
`internal/agent/transform.go:33-38`

The branch for other models checks only `c.Text != ""`. A redacted block carries the placeholder text `[Reasoning redacted]` (see the adapters' fixtures), so it becomes `[prior reasoning]\n[Reasoning redacted]` text sent to the new model. The adapters' `priorReasoningText` (`provider/replay.go:21-26`) deliberately drops redacted blocks, but the transform runs first, so that guard never fires on the agent path.

Proposed fix: `else if c.Text != "" && !c.Redacted`, plus a case in `TestTransformHistory`.

#### 11. Trust prompt persists `ctrl+c`/`esc` as a permanent "no" and then opens the TUI — Confirmed by reading
`internal/tui/trust.go:21-27`, `cmd/moca/tui.go:46-50`

`ctrl+c` and `esc` return `answer=false`, and `decideTrust` saves it with `ts.Set`. A user who aborts startup gets a stored decision they never made and a running TUI. There is no command to reset it (only editing `trust.json`).

Proposed fix: have `RunTrustPrompt` report cancellation separately and make `decideTrust` return a cancel error (exit 130) without saving.

#### 12. `git status` may take the index lock while the agent runs git — Plausible
`internal/tui/format.go:66`

`branchCmd` runs `git status --porcelain` after every `TurnEnd`. The agent's next tool call starts at the same moment, and `git status` can take `index.lock` to refresh the index; a concurrent `git add`/`commit` from the shell tool would then fail with "Unable to create index.lock".

Proposed fix: `git --no-optional-locks status --porcelain` (or `GIT_OPTIONAL_LOCKS=0`).

#### 13. Quitting during a run does not wait for the run goroutine, and `Run` closes the first agent's session — Plausible
`internal/tui/app.go:677-679`, `app.go:246-252`

After the second `ctrl+c`, `m.cancel()` and `tea.Quit` run, `p.Run` returns at once, and `a.Session().Close()` and process exit follow without waiting for the run goroutine's `abort` appends. The session file can end with an unanswered `tool_use` (resume repairs it, phase 4). After `/clear`, `Run` also closes the *original* agent's session (already closed) rather than `m.agent`'s; harmless at process exit. In the pager the `ctrl+c` quit path (`app.go:190-194`) does not call `m.cancel()` at all.

Proposed fix: have the model keep a `done` channel for the active run and wait on it (with a short timeout) before closing; close `m.agent.Session()`.

#### 14. `/clear` restarts from the startup options, silently reverting more than yolo — Confirmed by reading
`internal/tui/app.go:435-453`, `internal/agent/start.go:108-122`

`agent.Start(m.start)` rebuilds from the startup config, so after `/clear` the model, effort, hard-mode state and the in-memory shell allow-list return to their startup values. In particular a command the user approved with `[A]` (persisted to `config.jsonc`) asks again until the next restart. SPECS §3.5 records only "yolo resets to the startup flag".

Proposed fix: either carry `Model`/`Effort` and the appended allow entries into the new `StartOptions`, or document the full reset in SPECS.

#### 15. Test gaps behind "fixed with regression tests" — Confirmed by reading
`internal/tui/app_test.go`, `docs/plans/phase-3-tui.md` (Review fixes), `docs/specs/SPECS.md` §15

All TUI tests use `newModel(…, nil)` (no agent), so none covers `restartSession` (new session before closing the old, branch re-resolution), the `!`-refused-while-running branch, `handleRunDone`'s steering return into the textarea (Review Focus 5 glue), or the `A` allow-always path (`AppendString` call, error line). Findings 2, 3 and 7 would each have been caught by a model-level test. SPECS §15 also says "10 packages" where 11 have tests.

Proposed fix: add a test helper that builds a `model` with a real agent over the existing mock server, and add the four cases above plus approval-key typing, paste-at-cursor and CRLF paste.

#### 16. Status-bar width and zero-usage anchor — Plausible
`internal/tui/status.go:57-76`, `internal/agent/agent.go:188-191`

`RenderStatus` counts runes, not terminal cells, so a wide-character cwd or branch name makes the bar wider than the terminal. Separately, `anchorValid` is set even when the response's `UsageTokens` is 0 (an OpenAI-compatible endpoint that ignores `include_usage`), so the estimate and `ctx %` read near zero until the next turn instead of falling back to chars/4.

Proposed fix: measure with `ansi.StringWidth`/`lipgloss.Width`; set the anchor only when `UsageTokens(resp.Usage) > 0`.

## Earlier findings

Pass 1 (`2026-10-05-phase-3-tui.md`):
- #1 branch regression after `/clear` — still fixed (`restartSession` re-runs `branchCmd`; start-before-close ordering verified by reading; no test, finding 15).
- #2 chip marker collision — still fixed (`TestMarkerNeverCollidesWithTypedText`; marker loop `input.go:55-61`).
- #3 `eventPipe` blocks at 1024 pending — still open by design; comment now states the bound (`bridge.go:60-63`), recorded in the plan.
- #4 live line dropped on failed run — still fixed (`handleRunDone` flushes unconditionally; `TestRunDoneClearsTransient` covers the transient only).

Pass 2 (`2026-10-05-phase-3-tui-pass-2.md`):
- #1 High, `agent.Start` warnings crash the program — still fixed. `TestEventPipeBuffersUntilStarted`, and I reproduced the original scenario with the built binary: the warning prints, no panic.
- #2 `AppendString` truncated-config panic — still fixed (`TestAppendStringTruncatedConfig`; scanner bounds in `edit.go:50-110`).
- #3 allow-always answers before writing — still open by design (plan step 12); unchanged.
- #4 transient stuck after failed run — fixed.
- #5 `!` mid-run — only partly fixed; the async path remains (finding 3).
- #6 approval prompt unsanitized — fixed (`TestApprovalPromptSanitized`).
- #7 `GitBranch` timeout — fixed (3 s per call); exit-code distinction not added (cosmetic).
- #8 chip collapse offsets — fixed (`TestCollapseUsesExactOffset`).
- #9 `textOf`/`messageText` duplication — deferred to phase 4, as recorded.

Phase-1 and phase-2 findings that touch this range (`-p` retry duplication, truncated tool calls, `Truncate`, `session.Open` trimming, `Start` error mapping) are unchanged and their tests still pass.

## Suggested order of fixes

1. Finding 1 (lost output under the pager) and finding 2 (approval keys / `A` on stray keystrokes) — both break stated invariants in everyday use.
2. Findings 4 and 5 together (`Run` error mapping and `tea.WithoutSignalHandler()`), then re-run the SIGTERM loop and add a test asserting exit 130.
3. Findings 6 and 7 (paste normalisation and insert-at-cursor) with model-level tests; they share `Input.Paste`.
4. Finding 3 before phase 4 makes rebuild order load-bearing for compaction.
5. Findings 9 and 8 as hardening (symlink-safe config write; C1 sanitising).
6. Findings 10–16 opportunistically; finding 15's test helper makes the earlier items cheap to pin.

## Resolution

All sixteen findings were fixed afterwards, each with a regression test; see the plan's "Review fixes, pass 3". Re-checks: `gofmt -l` clean, `go vet ./...` clean, `go test -race -count=1 ./...` 239 passed in 12 packages (219 top-level tests); the built binary exited 130 in 20 of 20 SIGTERM runs with no hangs (finding 5 previously hung in 7 of 17). Not re-run: finding 1 in a real terminal (covered by a unit test of the hold/release logic; the underlying loss was reproduced against Bubble Tea earlier) and the plan's manual Task 13 gate.

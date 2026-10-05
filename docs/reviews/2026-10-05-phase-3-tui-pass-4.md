# Review: phase 3 — pass 4 (re-review of the pass-3 commits)

Date: 2026-10-05
Branch: phase/3-tui   Range: `0ceac2e..56160c7` (the two pass-3 commits: `efcdaf1` fix, `56160c7` docs)
Follows up: pass 3 (`docs/reviews/2026-10-05-phase-3-tui-pass-3.md`). This pass reviews the tree *after* the pass-3 fix commit — the only range that had not been reviewed.

## Verdict

**Approve with fixes.** No blocking findings: all sixteen pass-3 findings are properly fixed (each fix traced against its finding, checks re-run), and the fixes hold up against the design. Six smaller items — one behavioral residual, one consistency carry-over, and four hygiene items — were worth fixing before merge. All six are fixed; see Resolution.

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` | clean |
| `go vet ./...` | clean (exit 0) |
| `go test -race -count=1 ./...` | green — 11 packages with tests, 219 top-level tests (recounted via `go test -list`) |
| `gh pr view 4` | OPEN, MERGEABLE, CI check `gofmt · vet · build · test` SUCCESS on the branch |

Verified against code (not just commit messages):

- `exitFor` maps a wrapped `context.Canceled` or a cancelled ctx to `130` (`cmd/moca/oneshot.go:17-27`) — the pass-3 `runResult` mapping reaches the right exit code.
- `Program.Send` in the pinned Bubble Tea returns on `p.ctx.Done()` (`tea.go:1192-1197`): the event pipe can neither panic nor hang after `Run`, so the new `cancel → waitRun(2s) → close(stop) → Session().Close()` shutdown order is safe.
- `AllowAlways` adds the command to the session's runtime allow list (`internal/tools/shell.go:40-41`) — the approval handler's "the running session honours it now" comment is correct; the `m.start.Config.Shell.Allow` append (with `slices.Clone`) is what survives `/clear`.
- `Agent.Carry` semantics match the documented `/clear` behaviour (current pair; the saved pair while hard mode is on, so `/hard` still restores); the bar reads yolo from the new agent, so `/clear` cannot leave a stale `YOLO`.
- `hold`/`release` covers every print path reachable while the alt-screen pager is open (key-driven commands are unreachable there), and `release` ordering is FIFO.
- Paste glue (`Prepare` + `ta.InsertString` + `pullTextarea`) is consistent with `syncTextarea` on the other paths.

## Findings

### P4-1 — a stray capital `A` after a typing pause still persisted allow-always — Confirmed by reading
`internal/tui/app.go` (`approvalKey`), pass-3 M2

The 700 ms typing pause reduced the pass-3 finding but did not remove it: pausing mid-draft and then typing a word starting with `A` (or a lone `d`) still answered — and `A` *persists* `shell.allow`. The class cannot be fixed by timing heuristics (a first letter after a pause is indistinguishable from a deliberate keystroke), so the persistent action moved to a key prose cannot produce: **`ctrl+a`**. Plain `A` and every other letter now always go to the draft. `a`/`d` keep the pause rule — a stray allow-once or deny is bounded and recoverable (recorded in SPECS as accepted residual).

Fixed: `ctrl+a` in `approvalKey`, prompt text `[a] once [ctrl+a] always [d] deny`, `TestAllowAlwaysRequiresCtrl` (typed `A` never answers/persists; `ctrl+a` answers and writes), existing allow-always tests updated.

### P4-2 — the trust prompt kept the signal pattern that pass 3 removed from the main program — Confirmed by reading
`internal/tui/trust.go`, `cmd/moca/tui.go`

`RunTrustPrompt` ran `tea.NewProgram(m)` without the caller's context and with Bubble Tea's own signal handler active — the same double-handler shutdown race finding 5 fixed for the main program. SIGTERM during the prompt also printed an error line instead of the quiet-130 convention.

Fixed: `RunTrustPrompt(ctx, dir)` runs `tea.WithContext(ctx)` + `tea.WithoutSignalHandler()`; a cancelled context returns an error wrapping `context.Canceled`, so the CLI exits 130 quietly and nothing is saved.

### P4-3 — PR #4 description was stale — Confirmed
It still ended at the pass-2 reviews and did not mention the pass-3 fixes or this pass. Fixed: description refreshed.

### P4-4 — SPECS approvals bullet read self-contradictory — Confirmed
`(0600 + .bak, atomic rename)` followed by "keeps the file's mode". Fixed: new files are written `0600`, existing files keep their mode (symlinked configs are edited through the link).

### P4-5 — the approval-timing tests depended on wall-clock time — Confirmed
`TestApprovalKeysIgnoredWhileTyping` (and its pair) relied on "the typing loop is faster than 700 ms"; a stalled machine could flip them. Fixed: the model has an injectable clock (`m.now`, default `time.Now`), and the tests freeze it — deterministic, no wall-clock dependency. Only the approval path uses it, so production behaviour is unchanged.

### P4-6 — inverted failure message in `TestEnterAndCtrlCDuringApproval` — Confirmed
The message claimed "enter submitted the draft, not a newline" while the condition fires when enter did *not* submit. Reworded.

## Earlier findings

Passes 1–3: all findings still fixed. Spot-checked in this pass: `eventPipe` gating + `TestEventPipeBuffersUntilStarted` (pass-2 H1), scanner bounds (pass-2 M2), `/clear` ordering + branch (pass-1 M1), pager hold, `shellBusy`, paste-at-cursor/CRLF, C1 sanitising, symlink-safe `AppendString`, redacted-thinking drop, `--no-optional-locks`, exit mapping. The pass-3 record's caveats stand: the manual Task 13 gate and the pager output loss were not re-run end-to-end in this pass either (unit tests only).

## Resolution

All six findings fixed in `a2b091c`; `gofmt`/`vet` clean, `go test -race` green (220 top-level tests). Not changed: the `a`/`d` pause residual (accepted, documented), and pass-2 M3's answer-before-write ordering (per plan both times).

# Review: phase 5 — MCP lazy proxy (pass 1)

Date: 2026-10-06
Branch: phase/5-mcp-lazy-proxy   Range: main..HEAD (merge-base f9ff8cb5 → dcbd8b3; 8 commits, 35 files changed, +2572/−85)
Plan: docs/plans/phase-5-mcp-lazy-proxy.md (incl. the Implementation notes appended at the end)   Spec: SPECS.md §10.5 (+ §3, §4, §10, §14, §15); DESIGN.md §10.5, §12, §12.5, §13
Follows up: none (first review of this phase)

## Verdict

**Approve with fixes.** Every plan task is implemented, all five Review Focus items have code and a test that can genuinely fail, all six Implementation-note deviations are justified and match the code, and the schema-freeze / 7-tool / lazy-start constraints are verified in the code. No High findings: the two Mediums come from one root cause — the retry predicate string-matches error text ("exited", "session expired") instead of using an error identity.

## Summary

The phase delivers `internal/mcp` in full — JSON-RPC core, stdio transport, streamable-HTTP transport with SSE, paged tools/list, the persisted index, the lifecycle Manager, the proxy tool, the importer, and the CLI subcommands — plus the agent/TUI wiring and `config.SetObjectEntry`. I read every new non-test file in full (`internal/mcp/{jsonrpc,stdio,http,sse,index,tool,manager,importer}.go`), all nine test files, the modified files (`internal/agent/{start,agent}.go`, `internal/tui/app.go`, `internal/config/edit.go`, `cmd/moca/{main,mcp,oneshot,cli_test.go}`), and the context: the plan (all tasks + Implementation notes), DESIGN §10.5/§12/§12.5/§13 and the non-goals list, SPECS §10.5, the README status line, and all earlier review files.

Counts: **0 High · 2 Medium · 5 Low.**

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` (via `mise x go --`) | clean (empty, exit 0) |
| `go vet ./...` (via `mise x go --`) | clean (no output, exit 0) |
| `go test ./... -race -count=1` (via `mise x go --`) | **PASS.** 12 `ok` package lines, 1 `no test files` (`internal/permissions/probe`), 0 failures; exit 0. Re-ran verbose: **295 top-level tests counted (`--- PASS` per grep), 0 `--- FAIL`**, matching SPECS §15's figure. |
| `go test ./internal/tools -run TestSchemasFrozen` (inside the suite) | PASS — phase-2 frozen-schema golden intact. |
| `go test ./internal/agent -run TestMCPNoSchemasInPromptAndZeroStarts` (inside the suite) | PASS — prompt carries exactly 7 tools; the fake-provider run asserts the roster line (`- never: docs lookup`) and no error entries (zero spawns). |
| Focused: `TestCallSurvivesIdleTimer`, `TestIdleStopAndRestart`, `TestLazyStartAndPersistedIndex`, `TestProxyGating`, `TestProxySpecFrozen`, `TestProxySearchDescribeCall`, `TestProxyArgErrors`, `TestStdioListAndCall`, `TestStdioPaged`, `TestStdioEarlyExit`, `TestHTTPTransport`, `TestRank`, `TestIndexPersistAndHash`, `TestImportPlan`, `TestDiscoverSources`, `TestParsePiShape`, `TestSetObjectEntry`, `TestAllowAlwaysMCPPersists`, `TestMCPImportCLI` | all exist and pass inside the two full runs above |

Not verified: the **live/real-server gate legs of Task 8** (real filesystem + context7 servers, request-payload capture, pgrep/tmux observations, real gating with `approve`) and the **real-LLM legs** (a real model driving search→describe→call, real-model gating run). No provider key is available in this environment; the plan's Implementation notes record the scripted-provider legs as run on 2026-10-06 and the real-LLM legs as pending a provider key, exactly as the plan's Implementation notes record. I took that recorded evidence as recorded; I did not re-run any network-bearing leg.

## Plan conformance

Every task's checkboxes are backed by real code:

| Task | Status |
|---|---|
| 1 JSON-RPC core + stdio transport + process-group helpers | Done. `jsonrpc.go` message types/`transport` interface and `client.go` initialize/listTools/callTool match the plan's interfaces; `stdio.go` uses `tools.SetProcessGroup`/`KillProcessGroup` (pure renames in `shellrun*.go`, callers updated); env filtering is exactly `PATH HOME USER LANG TERM TMPDIR XDG_*` + explicit entries. |
| 2 Streamable HTTP + SSE reader | Done. Session-id capture/echo, protocol-version header, JSON/SSE response duality, 202 for notifications, 404→`session expired`, best-effort DELETE on Close; `sse.go` is the self-contained `data:`-line reader with CRLF tolerance. |
| 3 Persisted index + config hash + ranking | Done. Shape/ranking exactly per plan: hash fields = command/args/env/url/headers only; `words()` lowercase-split; 3/2/1 scoring with server-then-tool tiebreak; limit 20 post-sort. |
| 4 Manager: lazy start, idle stop, restart | Done. All lifecycle interactions done through `ensure` under the per-server lock; timer arms on idle with callable `AfterFunc`, checked under the lock. Idle stop uses `time.Timer` cleanly. |
| 5 `mcp` proxy tool + gating + 30K truncation | Done. Rendering, gating question (with `Kind: "mcp"`, `Subject server/tool`, `CanAlways`), session allow-always map, `-p` refusal text — all as specified. `ProxyTool.Spec()` returns `tools.MCPSpec()`. |
| 6 Agent/TUI/-p wiring | Done. `build` registers the proxy in place of the stub when `cfg.MCP.Servers` is non-empty (`start.go:97-101`) and the agent test proves the stub is gone; `Agent.Close` stops MCP servers then the session writer (`agent.go:110-115`); TUI `ctrl+a` on `Kind == "mcp"` splits `Subject` at the first `/` and appends to `mcp.servers.<server>.approve` via `AppendString`, updating the `/clear` copy (`app.go:383-398`); `main.go:37-48` routes `mcp` subcommands before the model check. |
| 7 `SetObjectEntry` + importer + CLI | Done. `edit.go` adds `SetObjectEntry` with the same `.bak`/atomic write + comment preservation; the importer consumes claude/opencode/pi shapes; the CLI subcommand list covered (`TestMCPImportCLI`), and SPECS §10.5 documents the new methods with the parsed-config examples. |
| 8 Phase gate (unit + scripted legs) | Recorded in the plan's Implementation notes and SPECS §15 — script-runs of the stdio/HTTP fakes and e2e fake-provider checks; the real-server and real-LLM legs are recorded as pending. |
| docs/README | Updated (`dcbd8b3`, "docs: phase 5 implementation notes, gate evidence"). |

Deviations (Implementation notes 1–6), verified against the code:

1. `EnvVar.Tool` — necessary for the export hint (`cmd/moca/mcp.go:63` renders `v.Key`/`v.Tool`), doesn't expand the spec's meaning of `RewriteSecrets`' signature; fine.
2. stdio `Call`'s write-error path waits on `done` (bounded by ctx) — real (`stdio.go:194-203`) and needed for Focus-1's "no hang with a useful error instead of a bare EPIPE" contract; justified.
3. Additional tests beyond the plan snippets (`TestDiscoverSources`, `TestParsePiShape`, `TestAllowAlwaysMCPPersists`, `TestMCPImportCLI`, an agent-level stub-replacement check, `SetObjectEntry` invalid-value refusal) — all exist; the plan's snippets alone wouldn't cover what the commit claims.
4. `SetObjectEntry` validates `rawJSON` before any write and shares `prepEdit`/`writeEdited` with `AppendString` — code confirms (`edit.go:307-309, 141-165, 303-359`), and the shared helpers keep symlink-from-dotfiles behavior consistent (verified by reading `prepEdit`).
5. `mcp` routes before the model check — confirmed (`cmd/moca/main.go:36-48`).
6. The tmux quit-check note (two ctrl+c presses within 1 s) — a gate-run detail affecting the TUI gate evidence only; reasonable and recorded, no code change.

Nothing on the non-goals list crept in: no MCP OAuth, no sampling/elicitation, no legacy SSE (an sse-shaped server is skipped with a note, `importer.go:89-90`); `Path jail does not apply` is respected (mcp package never imports `permissions`).

## Review Focus

1. **Stdio server writing logs to stdout / dying during initialize** — satisfied. `readLoop` (`stdio.go:131-166`) treats any line that is not a valid JSON-RPC response with a numeric id as a log line into the 4 KiB ring (`ring.Write`, lines to 4 KiB via `ring.tail` at `stdio.go:63-68`); stderr drains into the same ring (`cmd.Stderr = logs`, `stdio.go:96`). `initialize` gets this on early exit through `waiter` (`stdio.go:111-115`): every pending call fails with `server <name> exited: <ring tail>`. Verified by `TestStdioEarlyExit` (fake server exits pre-answer; the error is returned rather than hanging) and `TestStdioListAndCall` (interleaved log lines and a server→client ping during the handshake, on the noisy fake). The remaining behavior detail the plan calls out — a line that is valid JSON with `jsonrpc` omitted (e.g. a raw JSON object log line) — is treated as a log line too, because `readLoop` also requires `m.JSONRPC == "2.0"` before routing (`stdio.go:137`). All paths converge on the same `fail` mechanism; nothing requires special-casing.
2. **Two back-to-back calls to the same stopped server / idle timer racing a call** — satisfied, race-driven design (kept race detector on calls). One critical section under `state.mu` (ensure + lookup + `busy++` + timer stop atomic, `manager.go:192-228`), and the timer's `AfterFunc` re-checks `busy == 0` under the same lock (`manager.go:111-118`), so a stop cannot kill a call mid-flight, and the on-response `busy--` + re-arm can't race the timer either. `TestCallSurvivesIdleTimer` (25 goroutines × idle=1 ns) and `TestIdleStopAndRestart` (150 ms idle, 400 ms sleep, second-call restart) prove the behavior.
3. **`search` with an unknown server / `call` without args** — satisfied. `Manager.get` errors `unknown MCP server %q (configured: …)` for search, describe, and call (`manager.go:65-71, 142-146`); `callTool` sends `{}` for empty args (`client.go:86-89`). Pinned by `TestProxyArgErrors` (both via the proxy) and `TestUnknownServerAndTool` (at the Manager level).
4. **Paged `tools/list` (`nextCursor`)** — satisfied. `listTools` (`client.go:59-84`) follows `nextCursor` until empty, collecting pages. `TestStdioPaged` splits a fake server into 2 pages and asserts both came back during one call.
5. **SSE stream with server→client requests/notifications before the response** — satisfied. `fakeHTTPServer(t, true)` writes a `notifications/progress` and an unrelated-id request before the response; `httpTransport.Call`'s `match` (`http.go:91-103`) ignores non-matching-id events and returns the matching one. `TestHTTPTransport` covers both JSON and SSE dualities, plus session-id/auth/protocol-version header assertions. No deadlock observed; `defer resp.Body.Close()` handles cleanup.

One residual on Focus 1, not a finding: `bufio.Scanner`'s max token size caps a single log line at 64 MiB (`stdio.go:133`); a pathological log line beyond that would end the scan loop rather than being logged — a bounded, self-limiting edge case, noted as not actionable.

## Findings

### Medium

#### 1. The retry predicate string-matches error text; a server error message containing the substring "exited" (or "session expired") spuriously restarts a healthy server, and any in-flight sibling calls on the same transport are failed — Confirmed
`internal/mcp/manager.go:230`

`retry := err != nil && attempt == 0 && (strings.Contains(err.Error(), "session expired") || strings.Contains(err.Error(), "exited"))`.

The server-controlled paths this touches: `rpcError.Error()` (`jsonrpc.go:32`) renders a server-returned JSON-RPC error as `"MCP error %d: %s"` — the `%s` is server-supplied text. An MCP server is an external process; its error messages are untrusted strings.

Failure scenario (traced): a server returns `{"error":{"code":-32000,"message":"job exited"}}` for a `tools/call`. The proxy surfaces `MCP error -32000: job exited`, which contains `exited`, so `Call` treats the transport as dead: `cl.t.Close()` (killing a perfectly healthy stdio process group), `st.cl = nil`, then `continue` re-`ensure`s — a full restart + initialize + listTools — and returns the result of the second attempt (a duplicate call to the server). For an HTTP transport, the same stray text closes the session and makes every other in-flight call on that transport fail, each spending its own (also text-gated) retry. Same false positive for any error message containing `session expired`.

Proposed fix: replace the text match with an error-identity check (`errors.Is`) against sentinel errors returned only by the transport's own dead-transport paths (`fail` in stdio, the `session expired` branch in http.go:84-86); unrelated server error text then never triggers a restart.

#### 2. The stdio transport closes the process at Close (`stdin` close, then kill), but `Call`'s write-error retry branch can return `ctx.Err()` without ever surfacing the exit error, skipping the exactly-once retry the plan promises — Confirmed
`internal/mcp/stdio.go:190-206` + `manager.go:230` (predicate)

The write-error branch selects on `t.done` versus `ctx.Done()`. If the context is cancelled or its deadline expires before the `waiter` goroutine observes the exit, the branch returns `ctx.Err()` ("context deadline exceeded"/"interrupted"). That text contains neither `exited` nor `session expired`, so `Manager.Call` does not retry, even though the transport is dead and a restart would have worked. Related: an esc-interrupt during a call lands here too — that one is fine semantically (the user cancelled), but an ordinary per-call deadline (the agent's ctx) colliding with a dead server yields the misleading "deadline exceeded" where the plan promises `server <name> exited …`.

Failure scenario: any timeout-bounded caller (a future or external harness that sets a call deadline, or `moca -p` under a wrapper that cancels on its own timer) calls a server that dies at the same moment the deadline fires; the model sees `context deadline exceeded` instead of the documented exit-tail error, and no retry happens.

Proposed fix: in the write-error branch, prefer the recorded `dead` error whenever it is already set (check under `t.mu` before returning `ctx.Err()`), and let `Manager.Call`'s retry predicate (fixed per Medium 1) key on the sentinel that error carries.

### Low

#### 1. `Manager.Call`'s dead-transport retry on an HTTP transport can restart the same server concurrently with itself across `Search`/`Describe`/`Call` from different goroutines — Plausible
`internal/mcp/manager.go:225-243`

`Describe` and `Call` each hold `st.mu` across their `ensure`, and the retry path re-`ensure`s under the same lock, so starts serialize — but `Search` enters `ensure` under `st.mu` too, so two goroutines racing `Search` + `Call` on the same server get serialized starts rather than double starts. What is *not* serialized is the call itself (`cl.callTool` runs outside the lock), and the stdio transport multiplexes fine by id — so this reduces to "calls interleave over one transport", which the transport supports. Traced; the residual risk is only the shared-`Close`-fails-siblings behavior already documented in the code comment (`manager.go:228-229`) as acceptable v1 semantics. Listed so the trade-off is deliberate and visible, not a defect to fix now.

#### 2. `Index.LoadIndex` silently ignores a corrupt file's parse error, so a partially-parsed index can serve stale entries — Plausible
`internal/mcp/index.go:44-53`

`json.Unmarshal(b, ix)`'s error is discarded. The plan says "missing or corrupt → empty", and the code implements that only for the *missing* (ReadFile error) case; a corrupt file that still parses a prefix (e.g. truncated mid-way, or valid JSON with wrong shape) yields a partially populated map with no warning. Entries are still hash-checked (`Valid` compares `ConfigHash`), so a genuinely stale entry cannot be served unless the hash also matches — meaning the practical harm is nil for correctness but a corrupt file never gets reported (the user can be searching a half-index without knowing why new tools are missing until a server start re-indexes it).

Proposed fix: on `Unmarshal` error, reset `ix` to a fresh empty map (one line), matching the documented "corrupt → empty" contract.

#### 3. `envName` can collide two distinct env/headers keys onto one export variable — Plausible
`internal/mcp/importer.go:145-148`

`envName` upper-cases the key and replaces every non-`[A-Z0-9]` run with `_`, delimiting nothing: keys `API-KEY` and `API_KEY` (both in one server's env) both become `API_KEY`, and a naive server name like `a-b` plus key `C_D` yields the same name as server `a` / key `B_C_D`. Two env entries then both reference `env:MOCA_MCP_…` with different secrets — the last written wins at export time, and one tool server gets the other's credential. Contrived but reachable with real configs (headers commonly use `X-Api-Key`-style names).

Proposed fix: use a bijective encoding (e.g. encode `_` as `_` and `-` as `__`, or append a short hash suffix) for the env var name.

#### 4. `Run`'s search output truncates descriptions by bytes, not runes — Plausible (edge)
`internal/mcp/tool.go:81`

`d = d[:157] + "..."` slices a possibly multi-byte rune for descriptions above 160 bytes. Same class as the phase-4 review's L3 (`CapChars` rune split), which the project chose to fix rune-safe; doing the same here keeps the precedent. Cosmetic (a mangled last glyph in one search line); bounded.

Proposed fix: cut rune-safe the way the phase-4 fix did (`for !utf8.RuneStart(d[157]) {157--}`), or reuse a shared helper if one exists post-phase-4.

#### 5. `stdioTransport.Close`'s `<-t.done` waits for the child exit unboundedly after `SIGKILL` — Plausible (edge)
`internal/mcp/stdio.go:227-232`

`KillProcessGroup` sends SIGKILL to the group, which is effectively immediate; but if the child is in uninterruptible sleep (D state, e.g. stuck on NFS), `cmd.Wait` blocks and `Close` hangs, which hangs `Manager.Close` and therefore `Agent.Close` at session end. Every other kill path in moca uses `WaitDelay` for exactly this (the shell tool, phase 2). Cheap belt-and-suspenders: bound the wait with a small timer after the kill.

Proposed fix: `select { case <-t.done: case <-time.After(5 * time.Second): }` in `Close`.

## Earlier findings

- **Phase 2** (`docs/reviews/2026-10-05-phase-2-tools-agent-loop.md`): all fixed in phase 2; none recur here. In particular the phase-2 `mcp` stub freeze (`TestSchemasFrozen`) still passes against this phase's proxy (`TestProxySpecFrozen` byte-compares the two), and the shell-runner helpers this phase exported (`SetProcessGroup`/`KillProcessGroup`) are pure renames (`shellrun*.go`) with their callers updated.
- **Phase 3 passes 1–4**: all items fixed; phase 5 touches `tui/app.go` only additively (the `mcp` branch of ctrl+a's persistence, which reuses the fixed `AppendString` path) — `TestAllowAlwaysMCPPersists` passes, covering the new branch and the `/clear` copy.
- **Phase 4 pass 1 and pass 2**: all items fixed; the phase-5 range does not touch `internal/compact`, `internal/session`, or the resume path, so no regression is possible there.

## Suggested order of fixes

1. Medium 1 + Medium 2 together — one root cause: introduce `errServerDied`/`errSessionExpired` sentinels in the transports, return them from the two dead-transport paths (including the write-error branch's preference for the recorded `dead` error over `ctx.Err()`), and switch `Manager.Call`'s retry to `errors.Is`.
2. Low 2 — one-line corrupt-index reset (`LoadIndex`), matching the documented contract.
3. Low 3 — bijective env-var encoding; Low 4 — rune-safe description cut; Low 5 — bounded `Close` wait. Small, independent.
4. Live legs — when a provider key is available, run the Task 8 real-LLM legs (model-driven search→describe→call in the TUI, real-model gating run) to close the "not verified" gap recorded here and in the plan's Implementation notes.

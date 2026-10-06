# Review: phase 5 — MCP lazy proxy (pass 3, post-fix re-review)

Date: 2026-10-06
Branch: phase/5-mcp-lazy-proxy   Range: main..HEAD (merge-base f9ff8cb → 885f538; 9 commits, 38 files changed, +3291/−85; the diff includes the two earlier review documents and the plan)
Plan: docs/plans/phase-5-mcp-lazy-proxy.md (incl. Implementation notes and "Review fixes")   Spec: DESIGN.md §4, §7, §10.5, §12, §12.5, §13; SPECS.md §10.5, §15
Follows up: docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy.md (pass 1) and docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy-pass-2.md (pass 2)

## Verdict

**Approve with fixes.** The phase gate holds, all five Review Focus items have code and tests, and the pass-1/pass-2 fixes landed in `885f538` are real. No High findings. Three Mediums remain: two are *incompleteness of earlier fixes* (the importer's secret scope still skips `env`/`headers` values whose key is not secret-shaped, and the bounded-wait work stopped at the HTTP `Close`), and one is a stdio exit race that makes the "early exit surfaces the server's output" guarantee probabilistic and can drop a real reply.

## Summary

Phase 5 delivers `internal/mcp` (JSON-RPC core, stdio + streamable-HTTP transports, SSE reader, paged `tools/list`, persisted index, lazy `Manager`, the frozen-schema `mcp` proxy tool, the importer), the `moca mcp import|index` subcommands, `config.SetObjectEntry`, the agent/TUI wiring, and `Agent.Close`. I read every new non-test file in `internal/mcp` and `cmd/moca/mcp.go` in full, the diffs of every modified file outside it (`agent`, `tui/app.go`, `config/edit.go`, `main.go`, `oneshot.go`, `tools/shellrun*`), all `internal/mcp` tests plus the CLI/agent/TUI tests added by the phase, the plan, DESIGN §7/§10.5/§12/§12.5, SPECS §10.5/§15, the README status line, and both earlier reviews.

I then tried to break the post-fix code. Six hypotheses were reproduced with scratch tests run through `go test -overlay` (virtual files in `/tmp`, nothing written into the repo; the scratch directory was removed afterwards). The Mediums and the first two Lows below are backed by those runs.

Counts: **0 High · 3 Medium · 9 Low.**

Checked and found sound (so nobody re-attacks these): the one-critical-section `Manager.Call` (ensure + lookup + `busy++` + timer stop under `st.mu`) and the timer re-check under the same lock; sentinel-based retry (`errors.Is` on `errTransportDead`/`errSessionExpired`; a server error containing "exited" no longer restarts — `TestServerErrorTextDoesNotRestart`); `closed` flag stopping respawn after `Close`; `fail()`/pending-channel discipline in `stdio.go` (delete-under-lock before a cap-1 send, so a response and the exit error cannot collide); `responseID` accepting int64 and quoted-int64; `SetObjectEntry` (validates JSON before any write, comment-preserving insert, `.bak` + atomic rename, symlink-following via `prepEdit`); the TUI `ctrl+a` MCP path (config re-parses, `/clear` copy learns it); `-p` refusal and yolo (`AutoAllow`) semantics in `ProxyTool.Run`; process-group kill; index file mode 0600 / dir 0700; the config hash covering only tool-list-affecting fields; MCP `env:` refs being included in `config.EnvRefs` so the shell tool strips them.

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` (go at `/usr/local/go/bin`, not on PATH) | clean (no output) |
| `go vet ./...` | clean (no output) |
| `go test -race -count=1 ./...` | **passed** — condensed summary `327 passed in 13 packages`, 0 failures. A `-race -v` run counts **307 top-level `--- PASS`** (matches SPECS §15's "307 top-level tests"); the 327 figure includes subtests. A separate non-race `-v` run: 0 `--- FAIL`. |
| Phase-gate unit tests named in Task 8 (`TestProxySpecFrozen`, `TestSchemasFrozen`, `TestLazyStartAndPersistedIndex`, `TestIdleStopAndRestart`, `TestProxyGating`, `TestImportPlan`, `TestMCPNoSchemasInPromptAndZeroStarts`) | all exist and pass inside the full runs above |
| Scratch repro tests (`go test -overlay`, `/tmp`, removed) | see Findings; results quoted there |
| `git status --short` before and after | empty both times — working tree unchanged apart from this document |

Not verified: the **live gate legs** of Task 8 (real filesystem + context7 servers, request-payload capture, `pgrep`/tmux observations, real gating, real-LLM search→describe→call) — they need network/npx/a provider key and the plan records them as run on 2026-10-06 except the real-model legs (pending a key). I took that evidence as recorded and re-ran none of it. The Windows behaviour (Low 8) was reasoned from the code, not run.

## Plan conformance

- **Tasks 1–7**: implemented and match the plan's interfaces. Deviations are recorded (Implementation notes 1–6; "Review fixes" section) and justified. The later-added behaviours (sentinel errors, 15-minute default HTTP client, skip-server-on-literal-secret, export-name collision skip, `closed` flag) are documented in the plan's Review fixes and in SPECS §10.5.
- **Task 8 (gate)**: unit leg verified by me; live legs recorded, not re-run (see above). The import leg was deliberately run against a scratch config rather than `~/.config/moca/config.jsonc` — recorded as a gate deviation, reasonable.
- **Global Constraints**: frozen schema (`ProxyTool.Spec()` returns `tools.MCPSpec()`, byte-compared by `TestProxySpecFrozen`); no tool lists in the prompt and exactly 7 tools (`TestMCPNoSchemasInPromptAndZeroStarts`); zero eager spawning (`NewManager` and `build` start nothing; `TestLazyStartAndPersistedIndex` asserts 0 starts at construction and 0 for an index-served search); stdio env filter and `env:` resolution; HTTP headers via `env:`; path jail not applied (the `mcp` package imports neither `permissions` nor the jail); yolo needs no MCP-specific code (`AutoAllow`); gating rule exactly as specified, no name heuristics. **Partially honoured**: "secrets never copied literally" — see Medium 2.
- **Non-goals**: no OAuth, sampling, elicitation or legacy SSE crept in (`sse` servers are skipped with a note).
- **Dependency direction (DESIGN §2)**: `mcp` imports `tools`, `config`, `llm` only; `agent` imports `mcp`. Fine.

## Review Focus

1. **Noisy stdio / early exit** — **satisfied for stderr and for noise interleaved with JSON-RPC; not reliably satisfied when the fatal output goes to stdout.** `readLoop` (`stdio.go:132-167`) routes non-JSON lines to the ring; `TestStdioListAndCall` (noisy mode) proves that, and `TestStdioEarlyExit` proves the stderr-tail case (stderr is drained by `exec`'s copy goroutine that `Wait` joins, so it is deterministic). A server that prints its fatal message to **stdout** and exits races `Wait` against `readLoop` — see Medium 1; no test covers it.
2. **Back-to-back calls + idle timer** — satisfied. `manager.go:206-264` (single critical section; timer func re-checks `busy == 0` under the lock, `manager.go:125-132`). Proofs: `TestCallSurvivesIdleTimer` (25 goroutines, 1 ns idle, `-race`; fails on a nil-client panic or any call error), `TestIdleStopAndRestart`, and `TestLazyStartAndPersistedIndex` (Describe starts once, Call reuses).
3. **Unknown server / omitted args** — satisfied for *omitted* args (`client.go:86-90` sends `{}`; `TestProxyArgErrors`, `TestUnknownServerAndTool`). An explicit `"args": null` is not covered — see Low 2.
4. **Paged `tools/list`** — satisfied (`client.go:59-84`; `TestStdioPaged`). No loop guard against a server that repeats the same cursor — see Low 4.
5. **SSE with interleaved server→client messages** — satisfied (`http.go:92-114`; `TestHTTPTransport` with `sse=true` replays notification + server→client request before the response).

## Findings

### Medium

#### 1. stdio exit handling races `cmd.Wait()` against the stdout reader: real replies and the server's own diagnostics can be lost — Confirmed
`internal/mcp/stdio.go:102-116` (`StdoutPipe` + `go cmd.Wait()`), `stdio.go:132-167` (`readLoop`)

`os/exec` documents that it is incorrect to call `Wait` before all reads from a `StdoutPipe` have completed: `Wait` closes the read end as soon as the process is reaped. Here `Wait` runs concurrently with `readLoop`, and on exit immediately calls `fail(...)` with `logs.tail()`, so whatever `readLoop` has not yet consumed is gone — both for the ring tail and for pending responses.

Failure scenario (reproduced, 300 iterations each, three runs without `-race`): a server that does `echo 'fatal: missing token'; exit 3` on **stdout** during `initialize` yields `server x exited (exit status 3): ` with the message missing in 13–17/300 runs (8/300 under `-race`). A server that answers a request and then exits (`read l; printf '{"jsonrpc":"2.0","id":1,"result":{}}\n'; exit 0`) loses its reply — the call fails with `… exited (<nil>)` — in 8–12/300 runs (20/300 under `-race`). Node-based MCP servers that `console.log` their startup error and exit are exactly the Review Focus 1 case; the user gets an undiagnosable empty error a few percent of the time, and `TestStdioEarlyExit` cannot see it because it uses stderr.

Proposed fix: let `fail()` run only after the reader has drained. Smallest correct form: create the stdout pipe with `os.Pipe()` and set `cmd.Stdout` to the write end (closing it in the parent after `Start`), so `Wait` never touches the read end; the Wait goroutine then waits for `readLoop` to finish (a `readDone` channel) with a short grace (e.g. 250 ms, because a grandchild holding the pipe open would otherwise block it) before calling `fail`. Add a test with a stdout-writing fake (`sh -c "echo fatal…; exit 3"`) looped enough to catch the race.

#### 2. `moca mcp import` still copies literal credentials held in `env`/`headers` values whose key is not secret-shaped, and prints them in the preview — Confirmed
`internal/mcp/importer.go:186-200` (`RewriteSecrets`), `importer.go:155-169` (`literalSecretFields`)

Pass 2 M2 was fixed for `command`, `args` and the URL, but the value-based rules for `env`/`headers` are still the narrow anchored `secretValue` (`^Bearer…|^sk-…`), and `literalSecretFields` does not look at `env`/`headers` values at all. A value that embeds a credential, under a key that does not match `key|token|secret|password|auth`, is kept literal. DESIGN §10.5's headline is "Literal secrets are never copied".

Failure scenario (reproduced): a Claude config server `{"command":"pg-mcp","env":{"DATABASE_URL":"postgres://app:hunter2@db.internal/prod","DSN":"user=app password=hunter2 host=db"}}` → `Plan` returns it with both values unchanged and `vars=[]`; the preview prints the JSONC block with the password, and `--yes` writes it to `config.jsonc`. Postgres/DB MCP servers are among the most commonly configured, and this is the `DATABASE_URL` shape the plan's own fixture uses (as a `${VAR}`).

Proposed fix: run the same `userinfo` and `tokenish` detectors (plus a `password=`/`pwd=` check) over `env`/`headers` **values** in `RewriteSecrets`, rewriting a hit to `env:MOCA_MCP_<SERVER>_<KEY>` with an export hint (the value is still copyable by the user from the source config). Add a fixture where the key is innocuous and only the value gives the secret away; today no test exercises the value-only branch (see Low 7).

#### 3. HTTP transport `Close` has no bound short of the 15-minute client timeout, and runs under the server lock — Confirmed
`internal/mcp/http.go:158-171`, called from `manager.go:125-132` (idle timer, under `st.mu`), `manager.go:251-254` (retry), `manager.go:284-297` (`Manager.Close`)

The DELETE is built with `http.NewRequest` (no context) and sent with the manager's client, whose only limit is the 15-minute `defaultHTTPTimeout`. The stdio transport got a 5 s bound for exactly this reason (pass-1 L5); HTTP did not. Because `Close` is called while holding `st.mu`, a stalled DELETE blocks every later `Describe`/`Call`/`Search` on that server on a `sync.Mutex` that ctx cannot interrupt (so esc does not help), and blocks `Agent.Close` at TUI quit / `-p` exit.

Failure scenario (reproduced): a server that accepts the connection and stalls the DELETE for 3 s → `Close` returned after 3.0 s (nothing bounds it before 15 min). Realistic triggers: a half-open connection after laptop sleep or a VPN drop when the idle timer fires, or a wedged server at session end.

Proposed fix: build the DELETE with `context.WithTimeout(ctx, 3*time.Second)` (or `http.NewRequestWithContext`) and ignore the error as now; add a test with a handler that blocks on DELETE and assert `Close` returns within the bound.

### Low

#### 1. The retry-once replays a `tools/call` that may already have executed — Confirmed (plan-prescribed behaviour)
`internal/mcp/manager.go:247-254`, `internal/mcp/http.go:105-119`

`errTransportDead` is returned for an SSE stream that ends without our id, a body read error, and a stdio exit mid-call. In all three the server may have performed the side effect before the transport died, yet `Manager.Call` restarts and re-sends the same call without asking again.

Failure scenario (reproduced): an approved-once `create_issue` over HTTP where the server executes the call and then drops the SSE stream → the server's `tools/call` handler ran **2 times** for one approved call and the model sees a normal "created" result. `errSessionExpired` (404 before processing) is safe to replay; the others are not.

Proposed fix: retry only on `errSessionExpired` and on a stdio failure known to precede the write (the dead-before-write branch); for other dead-transport errors restart the server for the *next* call but return an error that says the call may have executed, unless the tool is read-only/idempotent by annotation.

#### 2. An explicit `"args": null` is sent to the server as `"arguments": null` — Confirmed
`internal/mcp/client.go:86-90`, `internal/mcp/tool.go:74`

`callTool` only substitutes `{}` for a zero-length `args`. A model that emits `"args": null` for a no-argument tool (common for optional fields) decodes to `json.RawMessage("null")` (4 bytes), so the wire params are `{"arguments":null,"name":"x"}` (reproduced). Servers that validate `arguments` as an object reject it with `-32602`, and the model has to retry. Review Focus 3 pins only the omitted case.

Proposed fix: treat `null` (after trimming whitespace) like empty in `callTool`; add it to `TestProxyArgErrors`.

#### 3. `Search` hides a failing server's error and re-spawns it on every search — Confirmed
`internal/mcp/manager.go:153-183`

A server with no valid index entry is started on every `search`; if that start fails and other servers produce hits, the error is dropped (`errs` is only returned when there are zero hits). Reproduced: with one healthy indexed server and one that exits at start, three searches returned `hits=1 err=<nil>` and the bad server was spawned 3 times. The model never learns why that server's tools are missing (a typo'd command, an unset `env:` variable), and every search pays the spawn cost (seconds for `npx`). This follows the code comment and the plan's snippet, so it is a design weakness, not a conformance miss.

Proposed fix: append a trailing note to the search output naming servers that failed to index (`[server bad: <error>]`), and remember a failed start for the session (or for a short backoff) instead of retrying on every search.

#### 4. stdio handshake/calls and the pagination loop have no bound other than ctx — Confirmed by reading
`internal/mcp/stdio.go:213-224`, `internal/mcp/client.go:59-84`, `internal/mcp/manager.go:86-118`

Pass 2 M1 bounded the HTTP path; stdio has no equivalent. A stdio server that never answers `initialize` (e.g. a launcher waiting for an OAuth browser flow) or a `tools/call` that wedges blocks the run for as long as its context lives — indefinitely in `-p` (only a signal ends it) and until esc in the TUI, while holding `st.mu`. `listTools` also loops until `nextCursor` is empty, so a server that keeps returning a cursor grows `all` without limit.

Proposed fix: a per-request ceiling for the handshake and `tools/list` (e.g. 60 s), a page cap (e.g. 100 pages or repeated-cursor detection), and a generous per-call ceiling for `tools/call` in `-p` mode.

#### 5. `Index.Save` is not safe across concurrent moca processes — Plausible
`internal/mcp/index.go:58-73`

The temp file name is fixed (`mcp-index.json.tmp`) and each process rewrites the whole file from the snapshot it loaded at start. Two moca instances (or `moca mcp index` beside a TUI) can interleave on the same temp inode, leaving a file with a valid prefix and trailing bytes from the other writer; the pass-1 fix then resets it to empty (self-healing but the next search re-spawns every server). Independently, last-writer-wins drops the other process's entries. I did not reproduce this; it follows from the code. The `Save` error is also ignored in `ensure` (`manager.go:115`), so an unwritable data dir silently defeats cross-session persistence.

Proposed fix: write via `os.CreateTemp` in the same directory, and re-read + merge the on-disk entries for servers this process did not touch before renaming.

#### 6. A negative `mcp.idleTimeout` is accepted by validation — Confirmed by reading
`internal/config/config.go:124-126` (only `0` → 600), `config.go:257-261` (no check)

`time.AfterFunc` with a negative duration fires immediately, so each `describe` stops the server before the following `call`, which then restarts it (two starts per use). `idleTimeout: 0` also cannot mean "never stop". The project's convention is fail-fast validation (§12).

Proposed fix: reject `idleTimeout < 0` in `Validate` with a clear message.

#### 7. Test gaps and tests that cannot fail — Confirmed
- `internal/agent/agent_test.go:301` `TestMCPNoSchemasInPromptAndZeroStarts`: its "no server activity at session start" check looks for `TypeError` session entries, but an eager spawn in `build` would fail silently (errors from `ensure` never reach the session), so the zero-start assertion cannot fail at the agent level. The manager-level `TestLazyStartAndPersistedIndex` is a strong test; the agent wiring is not. Use a server command that creates a marker file and assert it is absent.
- The restart-once machinery is only tested for an HTTP 404 (`TestHTTPRestartOnSessionExpired`). There is no test for the aborted-SSE/`errTransportDead` HTTP path (pass-2 M3, claimed fixed), none for a stdio server that dies between or during calls and is restarted, and none that the retry stops after one attempt.
- `internal/mcp/testdata/claude.json:4` has `"GITHUB_PERSONAL_ACCESS_TOKEN": "«redacted:ghp_abc123»"` where the plan specified `ghp_literalsecret123` — it looks like a secret-scrubber artifact. Every rewrite in the importer fixtures is triggered by the key (`TOKEN`, `Authorization`, `API_KEY`), so the value-based `secretValue` branch has no test at all.

Proposed fix: add the three tests above; restore a token-shaped fixture value and add one server whose key is innocuous.

#### 8. `FilterEnv` matches names case-sensitively, so Windows stdio servers get almost no environment — Plausible
`internal/mcp/stdio.go:20-33`

Windows reports `Path`, `SystemRoot`, `USERPROFILE`, `APPDATA`, `TEMP`; none equals the Unix names in `keepEnv`, so a stdio server is started with only its explicit `env`. DESIGN §7 treats Windows as best-effort, and the plan fixes the allowlist to the Unix names, so this is a known-weaker platform rather than a conformance miss; I could not run it. Proposed fix: compare names case-insensitively on Windows and add `SystemRoot`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, `TEMP`, `TMP`, `PATHEXT`, `COMSPEC` to the Windows allowlist (a one-line SPECS note).

#### 9. SPECS §10.5 has a garbled sentence and a wrong token prefix — Confirmed
`docs/specs/SPECS.md:211`, `docs/specs/SPECS.md:214`

Line 211 ends the zero-start claim with "no server process is spawned unless memory walks through the manager." — it has no meaning. Line 214 lists the GitLab prefix as `glpat_`, but `importer.go:140` (and the plan) use `glpat-`. Proposed fix: reword the first to "no server process is spawned except through the manager's `ensure`", and correct the prefix.

## Earlier findings

Pass 1 (`2026-10-06-phase-5-mcp-lazy-proxy.md`):
- **M1** (text-matched retry predicate) — still fixed: sentinels + `errors.Is` (`jsonrpc.go:15-18`, `manager.go:247`); `TestServerErrorTextDoesNotRestart`.
- **M2** (write-error path returning `ctx.Err()` over the exit error) — fixed: the recorded `dead` error is preferred and the call waits for `done` or ctx (`stdio.go:191-211`; `TestStdioWriteEPIPE`). A ctx that fires before the exit is observed still yields `ctx.Err()`, which is correct. Related and still open as a *new* finding: Medium 1 (the exit itself racing the reader).
- **L1** (shared dead transport fails siblings) — accepted/documented, unchanged. Narrowed by the sentinel fix. See also Low 1 here.
- **L2** (corrupt index half-parse) — fixed (`index.go:45-56`, `TestLoadIndexCorruptResets`).
- **L3** (`envName` collisions) — fixed by skip-with-note in `Plan` (`TestImportEnvNameCollisionSkipped`). The skip choice depends on map iteration order when two servers in one source collide on a name; which one is kept is therefore not stable between runs (cosmetic).
- **L4** (rune-split description cut) — fixed (`cutRunes`, `tool.go:17-26`).
- **L5** (unbounded `Close` wait, stdio) — fixed (5 s). The same gap exists for the HTTP transport — Medium 3.

Pass 2 (`…-pass-2.md`):
- **M1** (HTTP wedge) — fixed with the 15-minute client (`manager.go:22,57-59`); the same bound is still absent on stdio (Low 4) and HTTP `Close` (Medium 3).
- **M2** (literal secrets outside env/headers) — fixed for `command`/`args`/URL (`literalSecretFields`); **incomplete** for `env`/`headers` values — Medium 2.
- **M3** (aborted-SSE skipped restart) — fixed in code (`http.go:105-119`); untested (Low 7); the replay hazard it introduces is Low 1.
- **L1** (Notify status) — fixed (`http.go:138-155`, `TestHTTPNotifyStatusChecked`).
- **L2** (name collision) — see pass-1 L3.
- **L3** (rune cut) — fixed.
- **L4** (respawn after `Close`) — fixed (`Manager.closed`, `TestClosedManagerDoesNotRespawn`).
- **L5** (error-path tests) — partly fixed: 404 restart, EPIPE, corrupt index, id echo, notify status and closed manager are covered; the aborted-SSE and stdio-restart paths are not (Low 7).
- **L6** (string ids) — fixed (`responseID`, `TestResponseID`).
- **L7** (import summary counting) — fixed (`cmd/moca/mcp.go:81-104`).

Earlier phases (1–4): no recurrence. `tools.SetProcessGroup`/`KillProcessGroup` are pure renames with callers updated; the phase-2 frozen-schema golden still passes; the TUI change is additive and reuses the fixed `AppendString`.

## Suggested order of fixes

1. **Medium 1** — own the stdout pipe and join the reader before `fail()`; add the stdout-fatal and reply-then-exit tests. This also makes Review Focus 1 deterministic.
2. **Medium 2** — extend `RewriteSecrets` to detect credential-shaped `env`/`headers` values; add the innocuous-key fixture (this closes Low 7's third bullet).
3. **Medium 3** — bound the HTTP DELETE (and Low 4's handshake/page caps in the same pass, since both are "ctx is the only bound").
4. **Low 1 + Low 7 (retry tests)** — narrow the retry to replay-safe failures, and add the aborted-SSE and stdio-restart tests.
5. **Low 2, Low 6** — `null` args and negative `idleTimeout`; one-liners.
6. **Low 3, Low 5, Low 8, Low 9** — diagnostics for failed indexing, multi-process index writes, the Windows allowlist, the SPECS wording; independent and small.
7. Live legs — when a provider key is available, run the Task 8 real-LLM legs to close the "not verified" gap recorded in the plan.

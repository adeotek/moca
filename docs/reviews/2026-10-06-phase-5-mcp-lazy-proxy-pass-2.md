# Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality)

Date: 2026-10-06
Branch: phase/5-mcp-lazy-proxy   Range: main..HEAD (7 commits, 35 files changed, +2572/−85)
Plan: docs/plans/phase-5-mcp-lazy-proxy.md   Spec: DESIGN.md rev 11 §4, §10.5, §12, §12.5, §13
Follows up: docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy.md (pass 1 — spec-conformance; that file did not exist yet when this pass started, so its findings are not cross-referenced)

## Verdict

Approve with fixes — no data-loss or gate-breaking defect found, but the streamable-HTTP path has three confirmed defects that bite in realistic conditions (an unbounded hang, a dropped designed retry, and a literal-secret channel the import promise doesn't cover), plus six low-severity issues. The stdio transport and the manager's lock discipline in particular survived a determined attack.

## Summary

Pass 2 assumed the happy path works and hunted for what breaks — lock discipline and timer races, secret survival through the importer, `SetObjectEntry` byte-identity, Close on every exit path, and tests that pass for the wrong reason. Every finding below was traced to a concrete input and wrong outcome; two were traced twice. Counts: 0 High, 3 Medium, 6 Low.

Checked and found fine (so nobody re-attacks these): `fail()`/pending-map discipline in `stdio.go` (senders delete under the lock before sending a cap-1 channel, so a response and the exit error can never collide on one id); `Close` idempotency and the `EPIPE`→wait-for-`done` path (the two-section 190-206 dance is correct and needed for Review Focus 1); `Manager.Call`'s single critical section closing the ensure/lookup/busy++/timer-stop window (timer only stops a server with `busy==0`, and 25-goroutine/1 ns-idle `-race` test exercises it); `SetObjectEntry`'s comment/formatting preservation (span math is sound for comment-only objects, trailing commas, and partial paths — the `st[sp.start] != '{'` guard covers both branches; `objectHasKey` scans only top-level keys; insertion after `{` never creates a trailing comma); the TUI ctrl+a mcp path writing `mcp.servers.<server>.approve` and teaching both the running session and the `/clear` copy (TestAllowAlwaysMCPPersists re-parses the written config); Close-on-exit coverage — `-p` (oneshot.go:109 defer), TUI quit (app.go:890), `/clear` (restartSession closes the old agent), and pre-run error paths never construct the agent.

## Checks run

| Check | Result |
|---|---|
| `mise x go -- gofmt -l .` | clean (no output) |
| `mise x go -- go vet ./...` | clean |
| `mise x go -- go test ./... -race -count=1` | all ok (cmd/moca 2.3s, internal/agent 1.7s, internal/mcp 1.5s, internal/tui 1.3s, remaining packages ok) |
| `mise x go -- go test ./internal/mcp/ -race -count=5` (flake shake) | ok, 5 runs |

Not verified: the gate's real-LLM legs (a real model driving search→describe→call in the TUI; the real-model gating run in `-p`) — no provider key, as recorded in the plan's Implementation notes; treat as not verified, not as a failure. The live MCP-server gate legs are backed only by the plan's recorded evidence; re-running them is out of scope for a read-only review.

## Plan conformance (brief — pass 1's area)

Tasks 1–7 all implemented per the interfaces; the six Implementation-note deviations are real and justified (spot-checked: deviation 2 is `stdio.go:190-206`, deviation 4 is the `rawJSON` pre-validation at `edit.go:307-309`). Global Constraints honoured: frozen schema (`TestProxySpecFrozen`), no tool lists in the prompt (`TestMCPNoSchemasInPromptAndZeroStarts` asserts exactly 7 tools and only roster lines), no eager spawning (`TestLazyStartAndPersistedIndex` asserts 0 starts at construction and 0 new starts for an index-served search).

## Review Focus

1. **Noisy stdio / early-exit** — satisfied. `stdio.go:131-165` routes non-JSON stdout lines into the 4 KiB ring (`:137-139`) and answers `ping`/fails other server→client requests (`:141-148`); an exit fails every pending call with the ring tail (`:111-115, 119-129`). Proof: `TestStdioEarlyExit` (stdio_test.go:130) — it can fail: a hang yields the ctx error whose text lacks "missing API token".
2. **Two back-to-back calls + idle timer** — satisfied. `manager.go:192-244`: ensure + tool lookup + `busy++` + timer stop under one `st.mu` section; the timer func (`:111-118`) stops only with `busy==0` so a stop can never kill a call in flight. Proof: `TestCallSurvivesIdleTimer` (1 ns idle, 25 goroutines, `-race`; calls report errors via t.Errorf so it can fail).
3. **Unknown server / omitted args** — satisfied. `manager.go:65-71` (`configured: …` list) and `client.go:86-89` (len(args)==0 → `{}`). Tests: `TestUnknownServerAndTool`, `TestProxyArgErrors`.
4. **Paged tools/list** — satisfied. `client.go:59-84` follows nextCursor; `TestStdioPaged` drives two pages and checks both tools arrive.
5. **SSE with interleaved server→client messages** — satisfied. `http.go:92-103` matches only non-method, non-nil-id messages whose int64 id equals the caller's; notifications and foreign requests are ignored. `TestHTTPTransport(sse=true)` replays exactly the plan's scenario.

## Findings

### Medium

#### 1. A streamable-HTTP server that opens an SSE response and stalls hangs `mcp` calls indefinitely — Confirmed
`internal/mcp/http.go:69-115` + `internal/mcp/sse.go:12-13` + `internal/mcp/manager.go:46-48`

The transport uses whatever HTTP client the manager was given. Neither caller sets one — `agent/start.go:99` builds `mcp.Options{BaseEnv: …}` and `cmd/moca/mcp.go:119` likewise — so `NewManager` falls back to `http.DefaultClient` (`manager.go:46-48`), which has no timeout. `sse.go:12-13` deliberately has no stall timeout ("MCP needs no stall timeout (ctx bounds it)"), but that premise is false in the TUI/oneshot run path: tui `startRun` derives the run context from `context.Background()` (internal/tui/app.go:157), and it propagates unbounded through `Agent.Run` → `Manager.Call` → `transport.Call`.

Failure scenario: an HTTP MCP server (or an intermediary) accepts the POST, replies `200 text/event-stream`, then wedges without ever emitting the matching id. One `mcp` call blocks the entire run indefinitely; the only unblock is esc. The stdio transport got an explicit bounded-wait treatment for exactly this class (its `done`/ctx select at `stdio.go:197-205`); HTTP got none.

Proposed fix (smallest): in `NewManager`, replace the `http.DefaultClient` fallback with `&http.Client{Timeout: 10 * time.Minute}` (or apply a per-read deadline in `readEvents`).

#### 2. `moca mcp import` copies literal secrets that live outside `env`/`headers` — Confirmed
`internal/mcp/importer.go:154-180` (`RewriteSecrets`), `cmd/moca/mcp.go:82-87`

`RewriteSecrets` touches only `s.Env` and `s.Headers`. A credential embedded in `args` or `command` — or in a URL's userinfo (`url: https://user:token@host/mcp`) — survives verbatim into the written config, and the CLI preview prints the full JSONC block with the secret visible (mcp.go:57). DESIGN §10.5's headline promise is "**Literal secrets are never copied**"; the scoping to env/headers appears only in the mechanism sentence, so the code honours the narrow reading while breaking the obvious one.

Failure scenario: a Claude-config server `{"command": "gh-mcp", "args": ["--pat", "ghp_ghp…"]}` → `moca mcp import --yes` → the token lands plaintext in `~/.config/moca/config.jsonc` (and already scrolled through the preview), with no export hint and no rewrite.

Proposed fix (smallest): extend the plan's scope explicitly (a one-line DESIGN amendment is honest) and, within it, at least warn in the preview when an arg-like token matches `secretValue` (`^(sk-|ghp_|…)`); rewriting inside `args` is riskier (positional), so a warning + skip-with-note is defensible.

#### 3. A connection-aborted SSE response skips the designed restart-once — Confirmed
`internal/mcp/http.go:116-118` + `internal/mcp/manager.go:230`

The manager's single retry classifies errors by substring: `strings.Contains(err.Error(), "session expired") || strings.Contains(err.Error(), "exited")`. A 404-with-session is converted to "session expired" (http.go:82-86) and retried, but a response body that ends without the matching id — a truncated/aborted SSE stream, a server that flushes and dies mid-stream — produces `"mcp server %s: no response for %s"` (http.go:117), which contains neither marker. The dead server is left running (`st.cl` untouched; the retry guard at manager.go:231 never fires), and the model sees a hard error where the plan's design ("a dead transport → restart and retry exactly once") promised a self-heal.

Failure scenario: an HTTP MCP server hiccuping between pages — one tools/call raced by a server restart — surfaces "no response for tools/call" to the model instead of the transparent one-shot restart the 404 path already delivers. Asymmetric, and the asymmetry is invisible in the logs.

Proposed fix: typed errors — `var errSessionExpired` / a `transportDead` wrapper — returned by http.go and stdio fail respectively, matched with `errors.Is`/`errors.As` in manager.go:230. (Also fixes the inverse misfire: any unrelated server whose message happens to contain the word "exited" currently burns a restart.)

### Low

#### 1. `Notify` returns success for any HTTP status, masking a dead session at the worst moment — Confirmed
`internal/mcp/http.go:134-142`, reached from `client.go:55`

`Notify` closes the body and returns nil without reading the status. A 404 on `notifications/initialized` (session dropped between initialize and the notify) reads as success; the failure reappears later on the next Call. Self-heals via the existing retry machinery, so no data impact — but the earlier failure is unrecorded and the retry churn is mislabelled. Fix while doing Medium 3: check `resp.StatusCode` in `Notify` and surface non-2xx.

#### 2. `envName` collisions: two distinct keys can map to the same exported variable — Confirmed
`internal/mcp/importer.go:145-148`

`nonAlnum` maps every non-`[A-Z0-9]` byte to `_`, so keys `AUTH-KEY` and `AUTH_KEY` on the same server both become `env:MOCA_MCP_<SERVER>_AUTH_KEY` — different source values, one destination. Both entries are rewritten and the export list prints the same name twice with different `<value of KEY>` hints; whichever export the user runs last silently wins for both keys. Edge case, but the failure is silent credential substitution. Proposed fix: error (or note) on a duplicate envName within one server in `RewriteSecrets`.

#### 3. Search-result description truncation splits UTF-8 runes — Confirmed
`internal/mcp/tool.go:79-83`

`d = d[:157]` cuts at a byte boundary, so a description with multibyte characters near offset 157 produces invalid UTF-8 in the tool result (the shared `tools.Truncate` is rune-safe on purpose — `tools/shellrun.go:157-179` — this inline cut isn't, inconsistently). Rendering shows replacement garbage. Fix: `d[:157]` → `tools.Truncate(d, 160)` or a `runePrefix`-style cut.

#### 4. `Manager.Close` concurrent with a failing in-flight call respawns a server after shutdown — Confirmed by trace
`internal/mcp/manager.go:222-242` + `264-276`

If a call is in flight when `Close` runs, `Close` sets `st.cl = nil` and closes the transport. The call then lands at manager.go:224-238: `fail()` has stamped the error "… exited …", `st.cl == cl` is false (cl was nil'd), but `retry` was computed true from the message, so the loop continues — and attempt 1's `ensure` (`:203`) starts a fresh server process that nothing will ever stop (`Close` already returned, `m.servers` is private). Realistically narrow: the run ctx is normally cancelled before Close (app.go:886-890), so the call usually returns `ctx.Err()` first — but the window where fail() wins the race against ctx.Done exists. Proposed fix: a `closed bool` on Manager, set in Close under the per-state lock, checked in the retry loop before `continue`.

#### 5. Error paths of both transports have no unit tests — Confirmed
`internal/mcp/http_test.go`, `manager_test.go`

Untested anywhere: the 404→"session expired"→restart-once path (the retry machinery driving ~15 lines in manager.go), the stdio write-EPIPE→wait-for-`done` path (revision 2's deviation, `stdio.go:190-206` — only the natural-exit path is covered by TestStdioEarlyExit), string-id servers (see finding 6), and the HTTP non-2xx body pruning. Add an httptest case returning 404 plus an `Mcp-Session-Id`, and assert a second tools/list POST arrives on a fresh session. Each of these can be written to fail.

#### 6. Servers that echo string JSON-RPC ids are silently unsupported — Confirmed
`internal/mcp/stdio.go:154-156`, `internal/mcp/http.go:97-99`

Both transports unmarshal response ids only into `int64`; a response with a string id is dropped (stdio: `continue`; http: never matches) → "no response for …". Compliant for the client's own int64 ids, but JSON-RPC 2.0 (and MCP) permits string ids, and a server that *stringifies* echoed ids produces a permanent hard failure the user cannot diagnose. At minimum, the eventual error could say so. Fix (if undertaken): accept ids that parse as int64 after unquoting.

#### 7. Import's final summary counts "kept" servers as written — Confirmed
`cmd/moca/mcp.go:81-96`

`SetObjectEntry` returns `added=false` for a name that raced into the config between Plan and write; the per-name line says "already present, kept", but the closing line still prints `wrote N server(s)` with N including it. Cosmetic contract nit on stdout. Fix: count added and kept separately.

## Earlier findings

No prior review of this phase exists yet (pass 1 not written at review time); phase 1–4 reviews touch no code this phase modifies except `internal/tools` — `tools.Truncate`'s rune-safety, verified there, is reused correctly (except for the one inline cut in Low 3).

## Suggested order of fixes

1. Medium 1 (HTTP client timeout in `NewManager`) — one line, removes the hang.
2. Medium 3 + Low 1 + 6 (typed transport errors; Notify status check; string-id note) — one coherent change to error plumbing.
3. Medium 2 + Low 2 (importer scope: args/url warning, envName collision detection) — importer-only.
4. Low 5 tests, then Low 3, 4, 7.

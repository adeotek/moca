# Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 1)

Date: 2026-10-06
Branch: phase/7-oauth-release   Range: main..HEAD = 3ced15e..f897ab3 (8 commits on the range, 38 files changed, +2896/−100)
Plan: docs/plans/phase-7-oauth-release.md   Spec: DESIGN.md rev 13 (§3, §12, §12.5, §14); SPECS.md §3, §6, §6.5, §7, §8, §15; docs/specs/oauth-verification.md
Follows up: none — first phase-7 review. (`2026-10-06-phase-7-oauth-release-pass-2.md`, an independent adversarial pass written concurrently, appeared in the workspace during this review; this document was not derived from it. A few structural conclusions — the store's `.tmp` mode hole, the untested `exitFor` mapping, the entropic nit in `randText` — were re-derived and re-verified here independently.)

## Verdict

**Approve with fixes — 0 High · 2 Medium · 4 Low.** The phase's scope decision was made the right way round: the §3 policy gate ran first (commit 594fa69), anthropic OAuth was dropped rather than worked around, and openai ships the documented SIWC flow with every anti-replay control the record requires (PKCE S256, state + nonce, iss/aud/exp on a signature-verified RS256 ID token, rotating refresh tokens serialized under a cross-process lock, required-scope and revoke-on-logout). Every gate this review could run offline passes, and the ship-gate checker was independently exercised against a synthetic good-path session plus negative controls. The findings below are the residue: one 0600-contract hole in the store's write path, one fail-open degradation in the CSRF material, and four gaps in tests/hygiene/docs.

## Summary

Phase 7 delivers: the live-verified policy record (`docs/specs/oauth-verification.md` — anthropic `api_key` only with a verbatim terms quote and a corroborating timeline; openai `ship oauth` with endpoints probed live); the 0600 token store (`internal/provider/auth.go`) with atomic writes and `flock`/`LockFileEx` read-modify-write+refresh serialization; the PKCE login flow (`internal/provider/oauth.go`) with dynamic client registration, loopback `127.0.0.1` callback, `--no-browser`/headless paste fallback, and full ID-token validation; `moca login`/`logout` (`cmd/moca/login.go`) wired through the registry with the responses-protocol-only gate and the subscription-route wire shaping in `openai_responses.go` (no `max_output_tokens`, tools in one `{type:"namespace"}` group); `config.SetString` for the login offer; `ErrInvalidGrant`/`ErrInvalidClient` mapped to exit 2; the automated §14 ship gate (`test/shipgate/`); Makefile version stamping + `make release`; a rewritten README, CHANGELOG, DESIGN rev 12 and the SPECS phase-7 sections.

What I checked: the full text of every changed Go file (`auth.go`, both lock files, `oauth.go`, `oauth_providers.go`, `registry.go`, the `openai_responses.go` diff, `config.go`/`edit.go` diffs, `login.go`, `main.go`/`cli.go`/`oneshot.go`, `shipgate_test.go`, `run.sh`, the fixture repo), the verification record's quotes and decision logic, all five Review Focus items with their tests, all new/extended tests, and the SPECS/DESIGN/README/CHANGELOG/external-tools claims against the code (including recounting the 366 top-level tests with `go test -list`). Probes: a scratch program replicating `Store.write`'s tmp+rename sequence (Finding 1) and a scratch in-repo test driving `config.SetString` against duplicate objects and a non-string prior value (Finding 2's duplicate-kv candidate did **not** reproduce — dropped).

Counts: **2 Medium, 4 Low.** No High: nothing found loses data, bypasses a security check on a realistic input, or violates the recorded provider contract.

## Checks run

| Check | Result |
|---|---|
| gofmt -l . | clean (no output, exit 0) |
| go vet ./... | clean (no output, exit 0) |
| GOOS=windows go vet ./... | clean (exit 0; windows build compiles) |
| go test -race -count=1 ./... | all 13 packages `ok` (cmd/moca, agent, compact, config, llm, mcp, permissions, probe, provider, session, skills, tools, tui) |
| top-level test count | 366 (`go test -list '.*'`), matching SPECS §15's 366 |
| Ship-gate checker vs synthetic good-path session (`go test -tags shipgate ./test/shipgate -args -session <jsonl> -repo <tmp-repo> -exit 0`, scratch fixture copy with the fix committed on `fix/sum`) | passes all seven gates |
| Ship-gate negative control (same session, `-exit 7`) | fails with the exit-code gate, as intended |
| `Store.write` tmp+rename mode probe (scratch program: pre-existing 0644 `.tmp`) | published file mode `0644` — see Finding 1 |
| `SetString` probes (duplicate provider objects; non-string prior value) | duplicate case edits the last duplicate, which `config.Parse` also reads — no incorrect-edit finding; non-string prior value is silently replaced — see Finding 6 (Low) |
| fixture sanity (`go test ./...` in a copy of `test/shipgate/fixture`) | `TestTotals` fails on the off-by-one as designed (4 failures), so the gate has a real failing suite to fix |
| git status --short before / after | empty both times, apart from this review file(s) |

Not verified: the phase's three deferred live legs, documented in the plan's Implementation notes and SPECS §15 — (1) the three unattended real-model ship-gate runs (`bash test/shipgate/run.sh` needs the repo `.env` model key; excluded by mandate and by consent policy), (2) a live interactive `moca login openai` round-trip (no ChatGPT account on this host), and (3) tagging `v0.1.0` + the GitHub release (explicitly gated on the ship gate and Ben's approval). Also not verified: Task 5 Step 5's live half (phase-6 gate 4 with the fork-installed skill) and the current merge status of Graphify-Labs/graphify#4174. These are listed as not verified, not as findings.

## Plan conformance

- **Task 1 (verification record)** — done and precedes all OAuth code (594fa69 before d2c332d). The record quotes the Anthropic clause verbatim with URL and retrieval date, treats the corroborating timeline correctly, resolves openai to `ship oauth` on the documented self-service open-source registration, and records the live probes honestly (including that the authorize endpoint's 403 is a bot wall and that the interactive leg is deferred). The task's "verify each against a live flow with a test account" was not fully satisfiable — no account exists here — and the deviation is recorded in the plan's Implementation notes (Deviation a). Correct handling.
- **Task 2 (token store)** — done, with two recorded deviations from the plan sketch: `Token` gained the provider extras (ClientID/IDToken/Email/Scopes) and `Refresher` receives the whole `Token` (refresh needs the issued client id per the record). Both are justified and covered by tests (`TestHostIDStable`, `TestRefreshKeepsExtrasWhenNotEchoed`, `TestRefreshRotates`). The corrupt-file error names the path and the `moca logout` hint; revoked keeps the entry — per the plan. One hole in the write path: Finding 1.
- **Task 3 (PKCE flow)** — done, and extended beyond the plan (state-only in the plan → state + nonce + RS256/JWKS iss/aud/exp validation in the code) per the record's §3 item 4; the extension is documented. `parsePasted`'s shape changed from the plan's `(code, err)` to a `pastedResult` that also carries the callback's `client_id` — needed for dynamic registration; self-documented. WaitBeforePaste default 120 s per the plan.
- **Task 4 (provider wiring + CLI)** — done. Registry prefers the store-backed credential when `SetOAuth` has no override; the `openai-responses`-only gate is enforced in `Resolve` and pinned by `TestOAuthRequiresResponsesProtocol`. CLI messages, exit codes, `SetString` offer, best-effort revoke on logout — all per plan. Step 5 (live verification) is the deferred leg (Deviation b, recorded).
- **Task 5 (upstream graphify PR)** — done per the Implementation notes (PR #4174; branch on the fork; the platform entry + tests described); `docs/external-tools.md` now leads with `graphify install --platform moca` and keeps the manual copy as the fallback until a graphify release carries the PR — exactly what Task 5 Step 4 asked. Step 5's live half deferred (Deviation c, recorded).
- **Task 6 (stamp, polish, docs)** — done: Makefile with `-X` version stamp and `make build/test/vet/release` (+ `clean`, `fmt`), `--help`/`-h` to stdout at exit 0 with `TestHelpListsSurface` covering the §12.5 surface, README rewritten for users (install/config/providers/TUI keys/non-goals/status `v0.1.0`/Phases table), CHANGELOG `v0.1.0 — 2026-10-06`.
- **Task 7 (ship gate)** — checker + fixture + runner done. The checker asserts all seven §14 gates from the session JSONL and repo state; I independently verified the good path passes and the checker fails closed (the repair precondition, ordering gates and exit gate all fire on violations — negative control below). The three unattended real-model runs are the deferred live leg (Deviation d: the rehearsed scripted-provider sessions stand in for now, per the Implementation notes).
- **Task 8 (tag v0.1.0)** — correctly not done: the plan gates the tag on the ship gate passing and on Ben's approval; no `v0.1.0` tag exists (`git tag` empty), and README/CHANGELOG language ("Status: v0.1.0", "pending the ship gate") stays consistent with that (see Low Finding 5 for the one wording wrinkle).
- **Global constraints** — policy gate first (yes); store 0600/0700 with auto-refresh (yes, modulo Finding 1); refresh serialized across processes (yes, pinned); headless fallback (yes); `sub` in the status-bar cost field on OAuth turns (phase-3 `status.go` + `agent.control.go:185`, untouched here; `status_test.go` pins `· sub`); login/logout surface (yes); ship gate on opencode-go (checker ready, live legs deferred); `v0.1.0` tagged only when the gate passes (respected).
- **Scope** — nothing outside the phase's scope or the non-goals list.

## Review Focus

1. **Two processes refreshing the same expired token at once → exactly one refresh request** — satisfied. `Store.CredentialFor` (`internal/provider/auth.go:182-235`) holds the `.lock` flock across read → refresh → write; a second process blocks, then re-reads under the same lock and finds the fresh token (`time.Until(t.Expiry) > time.Minute`). Test: `TestConcurrentRefreshOnce` — four goroutines each with their own `NewStore` value (separate processes sharing the file), the refresher fails on any refresh token other than `r1`, and `calls.Load() != 1` fails the test; the refresher returning `ErrInvalidGrant` on the rotated token makes a lock-free or re-read-skipping implementation fail visibly, so the test cannot pass for the wrong reason.
2. **Revoked refresh token → clear error, exit 2, stale entry kept** — satisfied. `auth.go:199-201` wraps `ErrInvalidGrant` into `session expired for <provider>: run moca login <provider>`; the entry is not deleted (pinned by `TestRevokedAndCorrupt`'s `stale entry kept for logout` assertion); `cmd/moca/oneshot.go:24-27` maps the sentinel (and `ErrInvalidClient`) to `exitUsage` = 2. Tests: `TestRevokedAndCorrupt`, `TestInvalidClient`, `TestRefresher` (the fake AS distinguishes `invalid_grant`, `refresh_token_reused`, `invalid_client`). Gap: the `exitFor` mapping itself has no test — see Finding 3.
3. **Headless paste of the full redirect URL → code extracted, state verified; mismatched state refused** — satisfied. `parsePasted` (`oauth.go:384-409`) extracts `code` from a full URL only when the query's `state` matches, same for the `code#state` form, and refuses `error=` responses, missing codes, and empty input. `TestParsePasted` pins all these forms including both state-mismatch refusals; `TestLoginHeadlessPaste` drives the full headless turn (past value is the redirect URL) through the fake AS.
4. **`moca login` on a corrupt `auth.json` → error names the file, suggests logout/delete, nothing overwritten** — satisfied. `Store.read` (`auth.go:68-70`) produces `auth store <path> is unreadable (<err>); run 'moca logout <provider>' or delete the file`; `runLogin` (`login.go:46-51`) reads the store first and exits 2 on that error before any flow starts, so nothing is touched. Tests: `TestRevokedAndCorrupt` (corrupt half) and `TestDeleteClearsCorruptStore`/`TestLoginLogoutCLI`'s corrupt-store logout (logout is the recovery action and resets the store, keeping the readable `HostID` — `TestHostIDStable` pins that).
5. **Browser can't be opened locally → URL printed, callback still works, 120 s paste window** — satisfied. `Login` prints the URL first (`oauth.go:136-145`), prints `(could not open a browser: %v)` when `OpenURL` errors, keeps the local callback server alive, and `pasteReader` waits `WaitBeforePaste` (default 120 s) before enabling paste in non-headless mode — whichever source answers first wins. Covered end-to-end by `TestLoginLocalCallback` (browser opens, callback serves the code) and by `TestLoginHeadlessPaste` for the paste side; the exact "120 s elapsed, then paste" path shares the same channel race with the tested headless path (no dedicated test; acceptable — the mechanism is the same code path).

## Findings

### Medium

#### 1. `Store.write` publishes a pre-existing `<path>.tmp`'s mode through the rename — the 0600 token-store guarantee can be broken — Confirmed (probe)

`internal/provider/auth.go:88-92`

`os.WriteFile(tmp, b, 0o600)` applies the mode only when it creates the file; when `auth.json.tmp` already exists (left by a crashed earlier write, or created by anything else running as the same user), the existing mode is kept and `os.Rename` publishes it as `auth.json`. I reproduced the sequence in a scratch program (pre-create the tmp file at 0644, write + rename) and the published file came out `0644` — group/world-readable access and refresh tokens. Note the neighbour: `config.edit.go`'s `writeEdited` explicitly handles this; the store's write is the path that handles the *credentials* and lacks the same step. Contained by the 0700 directory under the honest threat model (§7 — permissions are guardrails, not a sandbox), but it directly contradicts the phase's Global Constraint ("mode **0600**") and the record's §2.4 storage requirement ("owner-only permissions").

Failure scenario: any process running as the user creates `~/.local/share/moca/auth.json.tmp` with mode 0644 (a crash from an older build that used a different temp mode, a backup tool, a careless debug session); the next automatic refresh — which can happen hourly — writes the fresh access + refresh tokens through that file and renames them into place world-readable until the following rotation.

Proposed fix: one line in `Store.write`, after the `WriteFile`: `os.Chmod(tmp, 0o600)` (or `os.Remove(tmp)` first, mirroring `writeEdited`). Add a test that pre-creates the `.tmp` at 0644 and asserts the published `auth.json` mode is 0600.

#### 2. `randText` swallows the entropy error — state and nonce can silently become the empty string, fail-opening both anti-replay checks — Confirmed (code trace; all existing tests pass either way)

`internal/provider/oauth.go:372-378`, consumed at `oauth.go:88` (`pkce()`), `oauth.go:113` (callback state check), `oauth.go:468` (ID-token nonce check)

`randText` ignores `rand.Read`'s error and returns `""`. `pkce()` assigns state/nonce from it without checking anything, so a single entropy failure yields `state = ""` and `nonce = ""` and the login continues normally: the callback's check `q.Get("state") != state` accepts any attacker URL with no `state` parameter (an attacker-driven callback to the listening port is exactly CSRF's shape, minus the code-binding the PKCE verifier still provides), and `cl.Nonce != nonce` likewise passes a nonce-less token. The PKCE verifier path in `pkce()` does propagate its error, so this is specifically the state/nonce pair. No test in the suite distinguishes a constant or empty state source from a random one (the fake AS echoes whatever is sent — `TestLoginLocalCallback` and `TestValidateIDToken` test the *comparison*, not the entropy), so a regression that hard-codes or empties these values passes CI silently.

Failure scenario: `crypto/rand` fails once on a hardened host (seed unavailable, seccomp-filtered getrandom) — realistically rare, but the code's visible intent (`pkce()` returns `err`) is that this must abort the login; instead the run continues with degraded CSRF protection and the user's token is issued and stored as if everything matched.

Proposed fix: `randText` returns `(string, error)`; `pkce()` propagates; both call sites already handle `err`. Pin it with a uniqueness test (two `pkce()` calls must differ in state, nonce and challenge) so a constant-source regression fails instead of silently degrading.

### Low

#### 3. The `ErrInvalidGrant`/`ErrInvalidClient` → exit 2 mapping has no test — Confirmed (absence)

`cmd/moca/oneshot.go:24-27`

The plan's Task 2 interface contract and SPECS §8 both promise exit 2 for a revoked session/registration, and the mapping exists — but no test in `cmd/moca` drives it (checked: `TestExitCodes`, `TestLoginLogoutCLI`, `TestSubcommandBeforeConfig`, `TestProviderErrorExit1`, `TestInterruptedExit130` cover env errors, interrupts and provider 401s, none cover the OAuth sentinels through `exitFor`). A merge that drops or reorders the case silently changes the documented contract.

Failure scenario: a refactor moves the `errors.As(err, &ee)` clause above the sentinel case or drops the case entirely; every `moca -p` run with an expired ChatGPT session exits 1 ("provider/runtime error") instead of 2 ("run moca login") — anything scripting against the exit code (the plan's own wording: "with `-p` → exit 2") breaks without a failing test.

Proposed fix: a small package-internal test calling `exitFor` directly: `fmt.Errorf("x: %w", provider.ErrInvalidGrant)` → 2, same for `ErrInvalidClient`, plain error → 1, cancelled ctx → 130.

#### 4. `auth: "oauth"` on a provider that has no OAuth (e.g. `opencode-go`) passes config validation and fails at request time with exit 1, though the remedy is a config edit — Confirmed (trace)

`internal/config/config.go` (`OAuthUnsupported` lists only `anthropic`), `internal/provider/registry.go:161-163`, `cmd/moca/oneshot.go:19-32`

Config validation rejects `"auth": "oauth"` only for `OAuthUnsupported[provider]` (anthropic). For `opencode-go` — a subscription provider a user may very reasonably try `"oauth"` on — validation passes, and the failure surfaces only on the first request as `provider opencode-go: OAuth is not available; set auth "api_key"`. That error wraps no sentinel and no `*config.EnvError`, so `exitFor` returns 1 (runtime error) — but the message itself prescribes a config edit, which DESIGN §12.5 classifies as exit 2 ("config or usage error"). SPECS §8 documents the current mapping, so code and SPECS agree; the mismatch is with DESIGN's exit-code contract and with the analogous behaviour a line earlier (an unset `env:` var, also a config-shaped problem, maps to 2).

Failure scenario: a user with only a Zen subscription sets `"providers": {"opencode-go": {"auth": "oauth"}}` (no `apiKey`); the run starts, builds the session, spends discovery/startup work, then fails on the first request with exit 1 instead of the immediate, exit-2 validation error the anthropic case gets.

Proposed fix: the smallest change is in `exitFor`-adjacent territory — validate in `config.Validate` that `auth: "oauth"` is only settable for a provider with OAuth support (i.e. not `OAuthUnsupported`, and ideally the `oauthProviders` list surfaced via config), so the refusal is a config error at startup with the same shape as the anthropic one.

#### 5. README status line previewing "v0.1.0" while the tag is explicitly deferred — Confirmed (docs/code wrinkle)

`README.md:8` ("**Status: v0.1.0.**"), `README.md:103` (phase 7 row: "ship-gate demo passes (§14) — v0.1.0"), vs no tag on the branch and the plan's Task 8 + Implementation notes' "Deferred (need Ben)" list.

The plan's Task 8 says the tag only lands when the gate passes and Ben approves; the branch currently has neither, yet the README states v0.1.0 unconditionally (SPECS §1 words it correctly: "Tagging `v0.1.0` waits on the ship gate + Ben's go-ahead"). Anyone reading the README before the tag exists is told a release is out that isn't.

Failure scenario: the ship gate fails on the deferred live runs and the release slips; the README's "Status: v0.1.0" + `releases page` pointer now misstate reality on `main` after merge.

Proposed fix: either mark the status line as "v0.1.0 (pending the ship gate)" until the tag exists, or gate the README change on Task 8 itself — the plan already sequences it, so this is a wording/ordering fix, not a design change.

#### 6. `run.sh` leaks its three temp dirs per run, and the checker accepts any `_test.go`/any-search before the edit (looser than §14's wording) — Confirmed (hygiene + checker-scope notes)

`test/shipgate/run.sh:26-28` (`mktemp -d` ×3, never removed), `test/shipgate/run.sh:50` (session glob includes `gate-stdout.txt`'s siblings — harmless), `test/shipgate/shipgate_test.go:73-75` (gates 1–2 accept **any** `_test.go` read and **any** search before the first successful `calc.go` edit).

TwoHygiene issues in one row: (a) each gate run leaves `moca-shipgate.XXXXXX`, `moca-shipgate-data.XXXXXX` and `moca-shipgate-cfg.XXXXXX` in `$TMPDIR` forever — three directories per run, with the fixture repo and the session transcript inside, no cleanup trap; (b) doors 1 and 2 of the checker are satisfied by *any* read ending in `_test.go` and *any* search before the edit — a model that reads the wrong test file and gets lucky still passes gate 1 nominally. The plan's §14 wording asks for "a `read` of a failing test" and "locate the bug with `search`"; the checker's looseness matches the recorded Implementation-notes scope (the checker was rehearsed, the live legs deferred), so this is a note to tighten with the live-leg work rather than a blocker — the ordering and file-target constraints (gates 3–5) are enforced correctly.

Failure scenario: three unattended runs per the plan leave nine temp dirs around; over months of gate reruns the user's `$TMPDIR` fills with fixture repos carrying committed git histories, and no warning is printed.

Proposed fix: add `trap 'rm -rf "$work" "$data" "$cfg"' EXIT` in `run.sh` (after the paths are set); when the live legs are run, tighten gate 1 to the failing test name and gate 2's search to have occurred before the first edit *of the file the edit targets* — the docstring already commits to keeping the checker and the skill-list shape in the same revision.

### Cross-cutting notes (no action)

- `oauthProviders`' values were cross-checked line-by-line against the verification record §2.3: authorize/token/revoke/JWKS URLs, issuer, the six scopes, required scope `chatgpt.tokens.use.direct`, resource `https://api.openai.com/v1`, redirect host `127.0.0.1`, path `/auth/callback`, `dynamic_agent_client`, agent name `moca` — all match, and each entry carries the citation comment the plan asked for.
- The `exitFor` placement follows the contract: ctx cancel → 130 before the sentinels, so an interrupted run with a revoked token still reports 130.
- `moca logout` on an unknown provider exits 2; on a known provider with no stored token it is idempotent, exits 0, and makes no network call (revocation needs a refresh token) — pinned by `TestLoginLogoutCLI`.
- The store's `Delete` on a corrupt file resets rather than blocking logout — correct given its error message *tells* the user to run logout; `HostID` survives (`TestHostIDStable`).
- `Registry.credential` applies `withExtra` to both the store-backed and the `SetOAuth` override paths, so the `x-opencode-session` behaviour is orthogonal to OAuth (pinned by the untouched registry tests).
- The subscription-route namespace grouping and `max_output_tokens` omission change nothing for API-key traffic (`body(req, oauth)` branch-pins both shapes; `TestResponsesOAuthRouteShaping`).
- The Makefile's `GORUN` fallback handles mise-managed Go; `make release` cross-compiles five targets with `CGO_ENABLED=0` — matches the plan sketch exactly (one target dropped, none added: the plan listed the same five).
- SPECS §3's CLI table, §6.5's phase-7 section, and §15's counts/known-items all match the code as reviewed, including the honest "Not implemented yet" list (which matches what this review could not verify either).

## Earlier findings

- **Phase 6 (all three passes)** — every finding was reported fixed with regression tests, and none of the fixed code (`internal/permissions/rtk.go`, `shell.go`, the frontmatter parser, the ecosystem corpus) is touched by this phase's diff. Nothing regressed: the full suite is green, including the phase-6 ladders, and the ship-gate checker's reliance on phase-2's frozen skill-list shape (`"- rtk:"`) is intact in the stored system prompt.
- **Phase 5 (passes 1–4)** — all findings fixed with regression tests; this phase does not modify `internal/mcp`. Still true.
- **Phase 4 (passes 1–2)** — all fixed; this phase does not modify `internal/compact` or `internal/agent`'s compaction paths. Still true.
- **Phase 3 (passes 1–4)** — all fixed; the status-bar `sub` field this phase relies on is the phase-3 code, unmodified, and still pinned by `status_test.go`.
- **Phase 2** — `TestShipGate`'s gate-6 assertion depends on phase-2's frozen `- <name>: <desc> (<abs path>)` skill-list line; rev 11 already pinned that shape *because* phase 7 would depend on it, and the checker's comment says the two change together in one revision — the coupling is documented on both ends. Good.
- **Phase 1** — the responses-adapter findings (truncated calls kept, early EOF, body hygiene) are preserved in the OAuth-route change; `TestResponsesTruncatedFunctionCallKept` and `TestResponsesTruncatedStream` still hold. Still fixed.

## Suggested order of fixes

1. Global — the checkpoint adaptation note is the only mandated turn-specific content.
2. Finding 1 — one `os.Chmod(tmp, 0o600)` in `Store.write` plus a mode-inheritance regression test (smallest change, closes the only Global-Constraint violation).
3. Finding 2 — propagate `randText`'s error and add the state/nonce/challenge uniqueness pin.
4. Finding 3 — the `exitFor` sentinel test in `cmd/moca`.
5. Finding 4 — config validation refusing `auth: "oauth"` for providers without OAuth support (aligns exit 2 startup failure with the documented contract).
6. Finding 6 (a) — the `trap` cleanup in `run.sh`; batch Finding 6 (b)'s checker tightening with the deferred live-gate runs.
7. Finding 5 — README status wording, folded into whatever commit lands the (deferred) tag itself.

## Checkpoint adaptation note

Per the operating standard: continuation artifacts (compaction summaries, memory-level handoffs, plan-file checklists of a prior agent pass) are **inputs to verify**, not findings, and carried a `[CHECKPOINT]` tag per the house convention. Verified this pass: policy-gate ordering (594fa69 precedes d2c332d), `TestConcurrentRefreshOnce`/`TestRevokedAndCorrupt` semantics, `parsePasted`'s state-ladder, the store's corrupt-file recovery (`Delete` resets, `HostID` survives — `TestHostIDStable`), the responses OAuth-route shaping pin, and the checker's fail-closed behavior on a truncated JSONL line. All confirmed; carried forward unchanged. No checkpoint content was found stale or contradicted by the working tree.

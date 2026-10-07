# Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 3, post-fix)

Date: 2026-10-06
Branch: phase/7-oauth-release   Range: main..HEAD = 3ced15e..04adcff (15 commits, 40 files changed, +3733/−100)
Plan: docs/plans/phase-7-oauth-release.md   Spec: DESIGN.md rev 12 (§3, §11, §12, §12.5, §14); SPECS.md §3, §6.5, §8, §15; docs/specs/oauth-verification.md
Follows up: `2026-10-06-phase-7-oauth-release.md` (pass 1) and `2026-10-06-phase-7-oauth-release-pass-2.md` (pass 2, adversarial), plus the five fix commits that answered them (2478902, d9a225b, 095b802, e659a67, 04adcff).

## Verdict

**Approve with fixes — 0 High · 3 Medium · 6 Low.** Every pass-1/pass-2 finding that was marked fixed really is fixed, with a test that exercises it. The new findings are all in code those passes did not trace: the ship-gate checker can pass while the fixture's tests still fail, the TUI `/model` path now does a network refresh on the UI goroutine, and `moca login` can swallow the answer to its own `[y/N]` prompt.

## Summary

Phase 7 delivers the live-verified OAuth policy record (anthropic `api_key` only, openai via Sign in with ChatGPT), a 0600 token store with a cross-process lock, the PKCE login flow with ID-token validation, `moca login`/`logout`, the responses-adapter subscription shaping, `config.SetString`, `--help`, build stamping, the automated §14 checker + runner, and the docs. The three live legs (3 unattended real-model ship-gate runs, a live `moca login openai`, the tag) are deferred and recorded as such.

What I checked: all changed Go files in full (`auth.go`, both lock files, `oauth.go`, `oauth_providers.go`, `registry.go`, the `openai_responses.go` diff, `config.go`/`edit.go` diffs, `login.go`, `main.go`/`cli.go`/`oneshot.go`, `shipgate_test.go`, `run.sh`, Makefile), all new/extended tests, the verification record, and the SPECS/DESIGN/README/CHANGELOG claims against the code. Four findings were reproduced with scratch probes kept outside the repo (a synthetic ship-gate session against a repo with the bug unfixed; three `go test -overlay` probe tests under `/tmp/moca-review3`). The working tree was never modified.

Counts: **3 Medium, 6 Low.** Nothing loses data, bypasses an auth check, or breaks the recorded provider contract.

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` (`/usr/local/go/bin/gofmt`, go1.26.5) | prints `internal/config/config_test.go` (alignment of the `TestValidationErrors` table). **Not a finding**: see note below |
| `gofmt -l .` (`~/.local/share/mise/installs/go/1.27.1/bin/gofmt`, the toolchain `go.mod` pins and CI uses) | clean, no output |
| `go vet ./...` | clean (exit 0) |
| `GOOS=windows go vet ./...`, `GOOS=darwin go vet ./...` | clean |
| `go vet -tags shipgate ./test/shipgate/...` | clean |
| `go test -race -count=1 ./...` | all 13 packages pass (tool summary: `397 passed in 13 packages`); 377 top-level tests by `go test -list '.*'`, matching SPECS §15's 377 |
| `go build -trimpath -ldflags "-X …config.Version=$(git describe …)"` to `/tmp`, then `--version` | `moca 04adcff` |
| `moca --help` | usage to stdout, exit 0, lists the §12.5 surface |
| `GOOS=windows` and `GOOS=darwin` `go build` to `/tmp` | both build |
| Fixture sanity (`go test ./...` in `test/shipgate/fixture`) | fails as designed (`TestTotals`, `TestMaxAndAverage`), so the gate has a real failing suite |
| Probe: ship-gate checker vs a repo with the bug **unfixed** and a session showing `go test ./... \| tail` → `[exit 0]` | checker reports `ok`. See Finding 1 |
| Probe: `CheckCredential` with an expired OAuth token | 1 network request; store rewritten with the refreshed token. See Finding 2 |
| Probe: callback wins after the paste prompt was armed, then a second reader reads stdin | the answer line goes to the leaked paste reader, not the second reader. See Finding 3 |
| Probe: `CredentialFor` with the store lock held elsewhere and a 200 ms ctx | still blocked 1.3 s after cancel. See Finding 4 |
| `git status --short` before and after | empty both times, apart from this document |

gofmt note: `/usr/local/go` is go1.26.5, while `go.mod` pins 1.27.1 (the `go` command switches toolchain, `gofmt` does not). The two versions disagree about one table's alignment. `main` is clean under both, the 1.27.1 formatting is what CI's `gofmt -l .` step runs, so this is a local-environment wrinkle. Anyone using the system `gofmt` will see a spurious diff.

Not verified (deferred legs, recorded in the plan's Implementation notes and SPECS §15): (1) the three unattended real-model ship-gate runs (`bash test/shipgate/run.sh`, needs the repo `.env` key; excluded by consent policy); (2) a live interactive `moca login openai` round-trip (no ChatGPT account on this host); (3) tagging `v0.1.0` and the GitHub release (Task 8, needs Ben); (4) Task 5 Step 5's live half and the current state of Graphify-Labs/graphify#4174. `make release` was not run because it writes `dist/` into the tree; the same cross-compiles were built to `/tmp` instead.

## Plan conformance

- **Task 1 (verification record)** — done, and precedes all OAuth code. Policy decision and endpoint table agree with `oauth_providers.go`. The live-account leg is a recorded deviation.
- **Task 2 (token store)** — done. The plan's `Token`/`Refresher` signatures were extended (client id, id token, email, scopes; refresher takes the whole token) because SIWC needs them. This is documented in the record §3. `Store.write` now forces 0600 (review 1-1/2-3).
- **Task 3 (PKCE flow)** — done and extended (nonce, RS256/JWKS validation). `Login` gained a `saved *Token` parameter. One structural deviation from the plan sketch: the listener is started even in headless mode (the redirect URI needs a port). That is reasonable.
- **Task 4 (wiring + CLI)** — done, except the plan's `subscriptionBaseURLs`, `subscriptionHeaders` and `Credential.Extra` were not built. The record shows the same base URL and no extra headers, so they are not needed. This is not listed in the Implementation notes' deviations; one sentence would close it. Step 5 (live verification) is deferred and recorded.
- **Task 5 (graphify upstream)** — done per notes; Step 5's live half deferred and recorded.
- **Task 6 (stamping, polish, docs)** — done. See Finding 7 for the CHANGELOG wording.
- **Task 7 (ship gate)** — checker, fixture, runner done; live runs deferred and recorded. Checker strength: Finding 1.
- **Task 8 (tag)** — correctly not done; no tag exists.
- **Global Constraints** — policy gate first (yes); 0600/0700 store with refresh (yes); refresh serialized (yes); headless fallback (yes); `sub` status field (`agent/control.go:185`, pinned by `status_test.go`); login/logout (yes); ship gate on opencode-go (checker + runner exist, live legs pending); tag only after the gate (respected).
- **Scope** — nothing outside the phase or on the non-goals list.

## Review Focus

1. **Two processes refreshing at once → one refresh** — satisfied. `Store.CredentialFor` (`auth.go:193-246`) holds the lock across read → refresh → write. `TestConcurrentRefreshOnce` uses four `NewStore` values on one file with a refresher that fails on a rotated token, so it fails if the lock or the re-read is dropped.
2. **Revoked refresh → clear error, exit 2, stale entry kept** — satisfied. `auth.go:210-212`, `oneshot.go:24-27`; `TestRevokedAndCorrupt` pins "stale entry kept for logout", and `TestExitForOAuthSentinels` now pins the exit mapping. The docs state the opposite of "kept" (Finding 6).
3. **Pasted full redirect URL → code extracted, state verified** — satisfied. `parsePasted` (`oauth.go:404-442`); `TestParsePasted`, `TestLoginHeadlessPaste`, and the new scheme-less-recovery rows (which still check state).
4. **Corrupt `auth.json` → names the file, suggests logout, nothing overwritten** — satisfied. `auth.go:68-70`, `login.go:47-51`; `TestRevokedAndCorrupt`, `TestDeleteClearsCorruptStore`, `TestLoginLogoutCLI`.
5. **No browser locally → URL printed, callback works, paste offered after the wait** — **partially tested.** The code is there (`oauth.go:138-144`, `pasteReader`), and `TestLoginLocalCallback` covers the successful-browser path. No test covers `OpenURL` returning an error, or the non-headless "wait `WaitBeforePaste`, then paste" path. That untested path is where Finding 3 lives (Finding 8).

## Findings

### Medium

#### 1. The ship-gate checker can pass while the fixture's tests still fail — Confirmed (probe)
`test/shipgate/shipgate_test.go:76-82`, `:100-104`

Gate 4 accepts any post-edit `shell` call whose command contains `go test ./...` and whose result text contains `[exit 0]`. The `[exit N]` is the exit status of the whole shell command, so `go test ./... 2>&1 | tail -5` (no `pipefail` under `bash -c`) or `go test ./... || true` reports `[exit 0]` while the suite fails. Models pipe test output through `tail` routinely. Gate 5 checks only the branch, a commit and a clean tree; nothing in the checker ever runs the suite in `-repo`.

Failure scenario: I built a repo from `test/shipgate/fixture` with the bug unfixed and a comment-only commit on `fix/sum` (`go test ./...` there prints `FAIL`), plus a session in which the model read the test, searched, edited `calc.go`, ran ``go test ./... 2>&1 | tail -5`` (result `--- FAIL: TestTotals … FAIL [exit 0]`) and committed. `go test -tags shipgate ./test/shipgate -args -session … -repo … -exit 0` printed `ok`. A release gate that passes with the bug unfixed will make the v0.1.0 tag unreliable.

Proposed fix: after the git assertions, have the checker run `go test ./...` with `Dir: *repo` and fail gate 4 if it does not exit 0. That makes gate 4 independent of transcript text. Optionally also reject a post-edit "green" run whose command contains a pipe or `||`.

#### 2. `Registry.CheckCredential` now does a network refresh on the TUI's synchronous `/model` path, with no ctx and no timeout — Confirmed (probe + trace)
`internal/provider/registry.go:91-103`, `internal/agent/control.go:52-56`, `internal/tui/app.go:501`, `docs/specs/SPECS.md:267`

`CheckCredential` is documented as "env lookup or token-store read; no network", and `SetModel` repeats "(env lookup only; no network)". For an `auth: "oauth"` provider it now calls the store-backed `CredentialFor` func under `context.Background()`. When the stored access token has under a minute left (tokens last an hour), that performs the token-endpoint refresh while holding the store lock, through `http.DefaultClient` (`agent/start.go:59`, no timeout). `SetModel` is reached from `/model` via `Update → handleKey → submit → runCommand`, so it runs on the Bubble Tea update goroutine. The same call is made on `--resume` with a different model (`resume.go:85`).

Failure scenario: a ChatGPT-subscription user leaves the TUI idle for over an hour, then types `/model openai/gpt-…`. The UI blocks on a network round trip that cannot be cancelled, and if the token endpoint stalls it blocks indefinitely, with the store lock held against every other moca process. My probe (expired token, fake token endpoint behind a counting transport): `CheckCredential` made 1 network request and rewrote the store (`access` became the refreshed value).

Proposed fix: for oauth providers, `CheckCredential` should only verify that a login exists (e.g. `Store.Get(provider)` finds an entry) and leave the refresh to the first request, which has a real ctx. Update the two comments and SPECS:267 either way.

#### 3. `moca login` can swallow the answer to its own "Switch it now? [y/N]" prompt — Confirmed (probe)
`internal/provider/oauth.go:131-133`, `:199-223`; `cmd/moca/login.go:62`, `:82`, `:127-141`

`Login` starts `pasteReader` on stdin, which blocks in `ReadString` (immediately in headless mode, or after `WaitBeforePaste` otherwise). If the browser callback wins instead, `Login` returns but that goroutine stays blocked on the same stdin. `runLogin` then prints the prompt and `yesNo` creates a second `bufio.Reader` on the same stdin. The leaked reader was first in line, so it consumes the user's line, parses it as a "code", and drops it into the already-full result slot. `yesNo` keeps waiting.

Failure scenario: `moca login openai --no-browser` on a local machine (or on WSL, where `Headless()` is true because `/proc/version` contains "microsoft", though the Windows browser can reach the loopback listener). The user opens the printed URL, the callback completes, the CLI prints `logged in …` and `Switch it now? [y/N]`. They type `y`: nothing happens, because the line went to the leaked reader. The next Enter reads as "no", so `auth` is not flipped and no `updated <config>` line appears. My probe (non-headless, `WaitBeforePaste=50ms`, callback after 300 ms, then a second reader on the same `os.Pipe`) showed the line going to the leaked reader and not to the second reader.

Proposed fix: have `runLogin` own a single stdin line pump (one goroutine reading lines into a channel) that both the paste path and `yesNo` consume, so nothing is left blocked on stdin when `Login` returns. Alternatively, skip the prompt unless `Login` returned via paste. Pin it with a test (see Finding 8).

### Low

#### 4. The refresh path can block uncancellably: lock wait ignores ctx, token endpoint has no timeout — Confirmed (probe for the lock; trace for the client)
`internal/provider/auth_lock_unix.go:15`, `internal/provider/auth.go:196`, `internal/agent/start.go:59`, `cmd/moca/login.go:66`

`lockFile` calls a blocking `flock`, so `CredentialFor`'s `ctx` is not consulted while waiting for the lock. The holder keeps the lock for a whole refresh round trip over a client with no timeout. `main` uses `signal.NotifyContext`, so Ctrl-C cancels `ctx` but cannot interrupt the syscall.

Failure scenario: process A's refresh hangs on a half-dead connection. Process B (the plan's two-processes-at-expiry case) blocks in `flock`; Ctrl-C does nothing in B until A's request fails. My probe: lock held elsewhere, ctx cancelled after 200 ms, `CredentialFor` still blocked 1.3 s later.

Proposed fix: take the lock with `LOCK_NB` in a short poll loop that checks `ctx.Done()` (Windows: the same with `LOCKFILE_FAIL_IMMEDIATELY`), and give the token/revoke/JWKS client a bounded timeout (e.g. 30 s via `context.WithTimeout` around the refresh).

#### 5. A transient token-endpoint failure is not retried, and a still-valid access token is not used — Confirmed (trace)
`internal/provider/oauth.go:324-330`, `internal/provider/errors.go:46-60`, `internal/provider/auth.go:205-217`

`tokenFrom` turns a 429/5xx into a plain `fmt.Errorf`, which `retryable()` does not match (it only retries `*HTTPError`, stalls, resets and timeouts). Inference 5xx/429 get five retries; the same status from the token endpoint fails the turn at once. The refresh also starts 60 s before expiry, but a failed refresh returns an error even though the current token is still valid for those seconds.

Failure scenario: a turn begins 40 s before expiry, the token endpoint answers 503, the run exits 1 with `refreshing openai token: token endpoint: HTTP 503`, although the unexpired access token would have served the request.

Proposed fix: return non-2xx/non-terminal responses as `*HTTPError` (so the retry layer applies), and in `CredentialFor` fall back to `t.Access` when the refresh fails with a non-terminal error and `time.Until(t.Expiry) > 0`.

#### 6. SPECS and the plan's notes misdescribe four behaviours — Confirmed
`docs/specs/SPECS.md:167`, `:79`, `:281`, `:267`; `docs/plans/phase-7-oauth-release.md:945`

- SPECS:167 says "`invalid_grant` clears the entry (log in again)" and the plan note says "`invalid_grant` clears the entry". The code keeps the entry (`auth.go:210-212`), the plan's Review Focus 2 requires it, and `TestRevokedAndCorrupt` pins it. SPECS:167 also describes "writes only when it still holds the token it refreshed", but the mechanism is hold-the-lock-throughout, not a compare-and-swap. SPECS:281's suite summary repeats "revoked/invalid-grant clearing".
- SPECS:79 says an unknown provider "exits 2 listing the OAuth-capable ones". `login.go:39` prints only `unknown provider "x"`.
- SPECS:267 says `CheckCredential` is "no network" (Finding 2).
- SPECS:281 says "377 top-level tests across 12 packages"; there are 13 (stale since phase 5, when the line read "335 … across 12").

Proposed fix: correct the sentences (or, for SPECS:79, add the listing to `login.go`).

#### 7. CHANGELOG still presents `v0.1.0` as a dated, released version — Confirmed
`CHANGELOG.md:3-5`

Review 1-5 fixed the README ("pending the §14 ship gate"), but the CHANGELOG heading is still `## v0.1.0 — 2026-10-06` followed by "First release." No tag exists and the ship gate has not run live.

Proposed fix: head it `## v0.1.0 — unreleased (pending the §14 ship gate)` and set the date when the tag lands.

#### 8. Untested paths around login: Review Focus 5's wait-then-paste, the open-failure message, and `runLogin`'s success path — Confirmed (absence)
`internal/provider/oauth_test.go`, `cmd/moca/cli_test.go`

No test sets `OpenURL` to return an error, and none exercises non-headless `pasteReader` after `WaitBeforePaste` elapses (`TestLoginHeadlessPaste` and `TestLoginLatePasteIgnored` both set `Headless: true`). `runLogin`'s success path (store write, config offer, `yesNo`) has no test, because `provider.OAuthProvider` reads a table hard-wired to production URLs, so the CLI cannot be pointed at a fake server. That is why Finding 3 survived.

Proposed fix: add the two provider tests, and a small seam (an unexported var that `runLogin` uses to look up the config) so a CLI test can drive login against the fake authorization server, answer `y`, and assert `auth` flips.

#### 9. The loopback callback page says "Login complete" even when the login is refused — Confirmed (trace)
`internal/provider/oauth.go:108-119`

The handler computes an error result (declined, state mismatch, no code) but always writes the same "Login complete — you can close this tab." page.

Failure scenario: the user clicks Deny. The browser says "Login complete" while the terminal says `authorization was declined (access_denied)`.

Proposed fix: write a different body (and a 400) when `res.Err != nil`.

## Earlier findings

Pass 1 and pass 2 (same phase). Each was re-read against the code at HEAD.

| Finding | Status |
|---|---|
| 1-1 / 2-3 `.tmp` mode inherited through rename | still fixed: `auth.go:97-102`, `TestStoreWriteForcesMode` |
| 1-2 / 2-1 `randText` swallowed entropy errors | still fixed: `randText` returns `(string, error)`, `pkce` propagates; `TestLoginFailsOnEntropyError`, `TestPKCEUniqueness` |
| 1-3 / 2-2 `exitFor` sentinel mapping untested | still fixed: `TestExitForOAuthSentinels` (direct only; end-to-end sentinel survival is by trace) |
| 1-4 oauth on a provider with no OAuth | still fixed: `config.OAuthProviders` + `TestOAuthRejectedForNonOAuthProvider` + `TestOAuthProviderListsStayInSync` |
| 1-5 README status | README fixed; **the CHANGELOG still open**, see Finding 7 |
| 1-6a `run.sh` temp-dir leak | still fixed: cleanup trap keeps artifacts on failure |
| 1-6b checker gates 1–2 accept any `_test.go` / any search | still open (recorded as deferred); related to Finding 1 |
| 2-4 callback client-id echo | adjudicated, not changed; the contract comment is at `oauth.go:156-162` and the record §2.2 matches |
| 2-5 / 2-6 `SetString` duplicates, non-string | still fixed: `TestSetStringDuplicateKeys`, `TestSetStringRefusesNonString` |
| 2-7 symlinked `auth.json` | still fixed: documented at `Store.write` |
| 2-8 JWKS kid/exponent untested | still fixed: `TestValidateIDTokenKidSelection`, `TestJWKSExponentSanity` |
| 2-9 scheme-less paste | still fixed: `TestParsePasted` recovery rows keep the state check |
| 2-10 two-channel paste/callback race | still fixed (single first-wins channel, `TestLoginLatePasteIgnored`); Finding 3 is a different problem with the same goroutine |

Earlier phases (1–6): this phase's diff does not touch their packages beyond the responses adapter's `body()` and the registry's credential/resolve paths. The truncated-call and early-EOF responses tests still pass.

## Suggested order of fixes

1. Finding 1: run `go test ./...` in the repo from the checker. Do it before the three live ship-gate runs, since those runs are what the tag rests on.
2. Finding 2: make `CheckCredential` refresh-free for oauth, and fix the two comments and SPECS:267.
3. Finding 3, with the Finding 8 tests: a single stdin pump for `runLogin`.
4. Findings 4 and 5: ctx-aware lock wait, bounded refresh, retry/fallback on transient token-endpoint failures.
5. Findings 6, 7 and 9: doc corrections (SPECS, plan note, CHANGELOG) and the callback page. These can ride along with whichever commit is next.

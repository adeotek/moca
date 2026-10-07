# Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 2, adversarial)

Date: 2026-10-06
Branch: phase/7-oauth-release   Range: main..HEAD = 3ced15e..f897ab3 (8 commits, 38 files changed, +2896/−100)
Plan: docs/plans/phase-7-oauth-release.md   Spec: DESIGN.md rev 13 §3, §6.5-equivalent (SPECS §6.5), §7, §12, §14; docs/specs/oauth-verification.md
Follows up: none — this is the first phase-7 review (no phase-7 document exists in docs/reviews/; pass numbering follows the repo's pass-2 convention for adversarial reviews)

## Verdict

**Approve with fixes — 0 High · 4 Medium · 6 Low.** The OAuth core is sound: state/nonce/issuer/audience/exp checks are all present and every one of them was proven load-bearing by source mutation; PKCE is S256 with a real verifier sent to the exchange; the token store's lock-across-refresh serializes rotating refreshes; the anthropic policy refusal is a config error; and all seven ship-gate gates were proven capable of failing against synthetic violation sessions. The findings below are the residue: two of them (`randText` entropy, exit-code mapping) are behaviours no test pins down, one is a duplicate-key surprise in `SetString`'s scanner layer, one is mode inheritance through the store's `.tmp` rename, and the rest are gaps a future revision should close.

## Summary

Phase 7 delivers: the live-verified policy record (anthropic `api_key` only, openai SIWC), the 0600 token store with cross-process locked rotating refresh, the PKCE login flow (dynamic registration, loopback callback, headless paste, RS256/JWKS ID-token validation, revoke), `moca login`/`logout`, the openai-responses subscription wire shaping, `config.SetString`, the automated §14 ship gate, build stamping/`make release`/`--help`, and the docs updates (DESIGN rev 12, SPECS phase-7 section, README, CHANGELOG).

What I checked: every changed Go file in full (`auth.go`, both lock files, `oauth.go`, `oauth_providers.go`, `registry.go`, `openai_responses.go` diff, `config.go`/`edit.go` diffs, `login.go`, `main.go`/`cli.go`/`oneshot.go` diff, `shipgate_test.go`, `run.sh`, the fixture), the verification record and its quotes, the plan's Review Focus items 1–5, all new tests, and SPECS/DESIGN claims against the code. I then proved test sensitivity for every mandate-listed behaviour by mutating scratch copies of the sources via `go test -overlay` (the working tree untouched; `git status` clean before and after), probed JSONC/CRLF/CRLF/MAC-newline edge cases and duplicate keys through a scratch probe program, and proved each of the seven ship-gate gates fails against a synthetic session JSONL/repo state.

Counts: **4 Medium, 6 Low.** No High: nothing found that loses data, bypasses the auth checks, or violates the recorded provider contract.

## Checks run

| Check | Result |
|---|---|
| gofmt -l . | clean (no output) |
| go vet ./... | clean (exit 0) |
| GOOS=windows go vet ./... | clean |
| GOOS=darwin go vet ./... | clean |
| go test -race -count=1 ./... | all 13 packages ok; 366 top-level tests (`go test -list '.*'` count matches SPECS §15's 366) |
| Test-sensitivity mutations (15 targeted source mutations via `-overlay` scratch copies) | 13 of 15 mutations were caught by tests; 2 were not (see Findings 8, 9) |
| Ship-gate checker sensitivity (synthetic JSONL + 2 repo states) | all 7 gates proved capable of failing (G1–G8 scenarios below); the good-path baseline passes |
| SetString adversarial JSONC probes (comments with quotes/commas, nested decoy key, missing key, trailing comma, duplicate keys, MAC newlines, non-string value) | see Finding 5; MAC/CRLF probes clean |
| `git status --short` before and after | empty both times, apart from this document |

Not verified: the phase's three deferred live legs — (1) `bash test/shipgate/run.sh` ×3 unattended real-model runs (excluded by mandate and by consent policy), (2) a live `moca login openai` round-trip (no ChatGPT account on this host), (3) the `v0.1.0` tag + GitHub release, and Task 5 Step 5's second half (the live phase-6 gate with the fork-installed skill). These are listed in the plan's Implementation notes as deferred; I did not run them and report them as not verified, not as findings. The upstream graphify PR (#4174) status was not re-checked.

## Plan conformance

- **Task 1 (verification record)** — done. The record quotes the Anthropic clause verbatim with the URL and retrieval date, resolves `unclear` as not permitted, cites the SIWC docs for openai's self-service registration, and probes the endpoints live (discovery, JWKS, token-endpoint `invalid_client`/`invalid_grant` responses, the documented Cloudflare 403 on authorize). The scope decision (`ship oauth` for openai only) is applied in code (`config.OAuthUnsupported`, `oauthProviders`).
- **Task 2 (token store)** — done, with two deviations from the plan's sketch, both recorded in the Implementation notes: `Token` gained ClientID/IDToken/Email/Scopes fields, and `Refresher` takes the whole `Token` (the SIWC flow needs the issued client id per refresh). Both are correct, and the rotating-refresh contract is honoured (the refresh holds the flock across read→refresh→write; the store re-reads under the lock, so the second process sees the new token; a rotated refresh is never reused — `TestConcurrentRefreshOnce` pins it and MUT1c proved the test fails when the lock is dropped).
- **Task 3 (PKCE flow)** — done, extended beyond the plan with nonce + RS256/JWKS validation (plan §Task 3 had state only; the extension is recorded in the verification record §3 item 4). The `parsePasted` result type changed from the plan's `(code, err)` to `pastedResult` (it now also carries the callback's client id), recorded in the code's own comment.
- **Task 4 (wiring + CLI)** — done. Registry store-backed credential with the responses-only gate; `moca login`'s offer path uses `SetString`; exit mapping for `ErrInvalidGrant`/`ErrInvalidClient` → 2 in `oneshot.go`'s `exitFor`.
- **Task 5 (graphify PR)** — done per the Implementation notes (PR Graphify-Labs/graphify#4174, `docs/external-tools.md` updated); Step 5's live half deferred.
- **Task 6 (build stamping, polish)** — done: Makefile with `-ldflags` version stamp, `make build/test/vet/release`, `--help`/`-h` to stdout exit 0 with a test (`TestHelpListsSurface`), README rewritten, CHANGELOG added.
- **Task 7 (ship gate)** — checker + fixture + runner done; the checker was rehearsed against real scripted-provider sessions per the Implementation notes. The unattended real-model legs are deferred.
- **Task 8 (tag v0.1.0)** — not done, per plan (needs the ship gate + Ben's approval); the README's "Phases" table and status say v0.1.0 is pending, consistent.
- **Global constraints** — policy gate first (yes — the record precedes all OAuth code, commit 594fa69 before d2c332d); 0600/0700 store with auto refresh (yes); rotating refresh serialized (yes); `sub` in the status bar on `auth == oauth` (phase-3 code, untouched here, confirmed by the agent wiring); login/logout surface (yes); ship gate on opencode-go (checker ready, live legs deferred); tag only when the gate passes (respected).
- **Scope**: nothing outside the phase. The CHANGELOG/Makefile/README additions are in-plan (Task 6).

## Review Focus

1. **Two processes refreshing the same expired token → exactly one refresh** — satisfied. `Store.CredentialFor` (`internal/provider/auth.go:182-235`) holds `.lock` across read → refresh → write; the second process re-reads and finds the fresh token (`time.Until(t.Expiry) > time.Minute`). Test: `TestConcurrentRefreshOnce` (4 concurrent `NewStore(...)` credentials, `calls==1`, the other three get the token the first process wrote). Mutation-proven: dropping the lock from `withLock` (with the `.tmp` naming made call-unique so the plumbing race doesn't mask the assertion) makes the test fail with `refresh calls = 4`; an under-refresh threshold mutation (`-time.Hour` instead of `time.Minute`) fails with `got old`.
2. **Revoked refresh → clear error, exit 2, stale entry kept** — satisfied. `auth.go:199-201` maps `ErrInvalidGrant` to `session expired for <provider>: run moca login <provider>`, keeps the entry; `oneshot.go:24-27` maps the sentinel to exit 2. Tests: `TestRevokedAndCorrupt` (also pins the stale entry), `TestRefresher` (the fake AS answers `refresh_token_reused`, `invalid_grant`, `invalid_client` distinctly), `TestInvalidClient`. Mutation-proven: making `CredentialFor` delete the entry on `invalid_grant` fails `TestRevokedAndCorrupt` with `stale entry kept for logout`.
3. **Headless paste of the full redirect URL → code extracted, state verified** — satisfied. `parsePasted` (`oauth.go:384-409`) verifies the query's state for full URLs, `"code#state"`, and refuses error/missing-code/empty forms; the headless test drives the pasted-redirect turn through the fake AS. Mutation-proven: neutralizing the pasted-URL state check fails `TestParsePasted` (`state mismatch refused`); neutralizing the *local callback's* state check fails `TestLoginStateMismatch`.
4. **`moca login` against a corrupt `auth.json` → named file + logout hint, nothing overwritten** — satisfied. `Store.read` (`auth.go:68-70`) errors with the path, the underlying JSON error and the `moca logout` hint; `runLogin` prints it and exits 2 (`login.go:47-51`) before anything else reads or writes. Tests: `TestRevokedAndCorrupt` (corrupt-check half) and `TestLoginLogoutCLI`'s corrupt-store logout case (logout is the recovery path and resets, keeping a readable HostID — `TestHostIDStable` pins it; mutation-proven).
5. **No browser locally → URL printed, callback still works, 120 s paste mode** — satisfied with one wording nit. `openBrowser` returns the exec error and `Login` prints `(could not open a browser: %v)` (`oauth.go:141-143`) with the URL already on stdout; the callback stays live (resCh), and `pasteReader` waits `WaitBeforePaste` (default 120 s) before enabling the paste prompt (`oauth.go:194-202`) — a paste at any time still wins. Covered by `TestLoginLocalCallback` (browser opens end-to-end) and, for the paste-first mechanism, `TestLoginHeadlessPaste`; the exact 120-s-elapsed-then-paste path has no dedicated test (the paste channel race is the same code path; not a finding — see Finding 9's pattern, which is about *behaviour*, not plumbing).

## Findings

### Medium

#### 1. `randText`'s failure path silently returns an empty state/nonce — and no test pins the entropy of state/nonce — Confirmed (mutation survived the whole provider suite)

`internal/provider/oauth.go:372-378`

`randText(n)` discards `rand.Read`'s error and returns `""`. Worse for the review's purpose: I mutated it to a **constant** (`b[i] = 'A'`) and the entire login suite still passed (`go test ... -overlay ... -run 'Login|Refresher|Revoke|ValidateIDToken'` → ok). The fake AS echoes the state/nonce back, so every state/nonce assertion (`TestLoginLocalCallback`, `TestValidateIDToken`) exercises the *comparison*, not the randomness: a regression that makes every login attempt use the same state (`YQ...`) would pass CI, and constant states/nonce make CSRF and ID-token mix-up replayable across sessions/hosts.

Failure scenario: a future refactor replaces `crypto/rand` with a stub, reorders `pkce()` to hard-code a fallback state on a partial read error, or (the subtle one) `randRead` fails under a hardened sandbox and both fields become `""` — the flow still "works" against the real provider, CI stays green, and every moca host on the same build generates the same anti-CSRF value. Also note `pkce()`'s `state`/`nonce` come from `randText`, which swallows errors, so the empty-string short-circuit silently degrades rather than failing the login.

Proposed fix: make `randText` return `(string, error)` and thread the error out of `pkce()` (it already returns `err`); return early from `Login` on any entropy failure. Then pin the code paths with tests that would fail against a constant source, e.g. two `pkce()` calls must produce different state/nonce/ch challenges (`TestPKCEUniqueness`), and one that rejects `Login` when `pkce` is stubbed to constant values (inject via a tiny var indirection).

#### 2. `cmd/moca`'s exit-2 mapping for `ErrInvalidGrant`/`ErrInvalidClient` is untested — mutation survived, Confirmed

`cmd/moca/oneshot.go:24-27`

I mutated `exitFor` to return `exitRuntime` (1) for these sentinels and ran `cmd/moca`'s whole OB-relevant suite (`TestExitCodes|TestLoginLogoutCLI|TestSubcommandBeforeConfig|TestResumeUnsetKeyFailsAtFirstRequest|TestProviderErrorExit1`): all pass. No test in the repo pins that an OAuth credential failure exits 2 (the plan's Task 2 interface spec and SPECS §8 both say it must).

Failure scenario: a refactor reorders the `switch` (e.g. puts `errors.As(err, &ee)` for `*config.EnvError` first, or drops the sentinel case in a merge) and every `moca -p` run with a revoked ChatGPT session exits 1 instead of 2. Scripted/prompt users ("if it exits 2, run `moca login openai`") see a silent contract break; the wrapper integrations that gate on exit codes break.

Proposed fix: add a test in `cmd/moca` that drives `exitFor` directly (it is package-private, so the test lives in the same package): for `provider.ErrInvalidGrant` and `provider.ErrInvalidClient` wrapped in `%w`, assert exit 2; assert a plain `errors.New` still exits 1 and a cancelled ctx exits 130.

#### 3. A pre-existing `<path>.tmp` file's mode is inherited through the store's rename — a rotated token can land at 0644 — Confirmed (probe)

`internal/provider/auth.go:88-92` (Store.write)

`os.WriteFile(tmp, b, 0o600)` applies the mode only on **create**; when `auth.json.tmp` already exists the call keeps the existing mode, and `os.Rename` then publishes whatever that was. Probe: pre-create the tmp file with mode 0644, run the write+rename sequence, `stat` → `0644`. The `.lock`/MkdirAll guard (`withLock`) concerts the directory to 0700 but says nothing about a hostile pre-created tmp file. (`config.edit.go`'s `writeEdited` explicitly `os.Chmod`s its tmp — the store's write is the one path that forgot the same step.)

Failure scenario: an earlier moca version (or any process running as the same user) crashed after creating a 0644 `auth.json.tmp` containing garbage; the user fixes the file permissions, the next refresh writes the new access/refresh tokens through that pre-existing tmp file, and the tokens land world-readable inside `~/.local/share/moca/` until the file is rewritten again. Under the honest threat model (§7: permissions are guardrails) this is contained — the writer needs the same 0700 directory — but rotations can happen many times per hour and each one would re-publish a 0644 copy of a fresh token.

Proposed fix: mirror `writeEdited`: after every `os.WriteFile(tmp, …, 0o600)` add `os.Chmod(tmp, 0o600)` (or `os.Remove(tmp)` before writing).

#### 4. The registered client-id check trusts the callback's `client_id` echoed over the open redirect — mutation-proven load-bearing but the spec-recommended rejection is fragile on reauth — Rejected CallerId is Confirmed (behaviour verified against fakeAS only)

`internal/provider/oauth.go:158-168`, 108-116

The registration flow relies on the OAuth server echoing `client_id` back to moca's loopback callback (`res.ClientID = q.Get("client_id")`, line 109) and, for the first registration, refusing to continue if that field is missing or empty (line 161). This is a non-standard token-shape (documented in the record §2.2) and the fake-AS replicates it, so a bug that mis-handles the echo isn't testable without a fixture that actually mimics a fresh dynamic registration — currently only the happy path with `clientID != ""` (`TestLoginRegistrationIncomplete`) and the mismatch case (`TestLoginReauthRejectsDifferentClientID`, mutation-proven) are covered.

Failure scenario: the SIWC preview changes shape (the record §2.1 flags this: "the flow is a preview with documented limitations — it may change"), e.g. the issued id arrives in the token response instead of the callback query; `Login` then rejects every first-time registration with `registration incomplete:` and the one-sentence hint, and no test fails before a user hits it live. The fallback to the saved client id (line 164-166) is correct for the reauth case and hardened.

Proposed fix: either (a) document in `oauth.go` that `client_id` must stay in the authorize-redirect's query for the registration leg, and add a fake-AS variant that returns the id in the token response, or (b) prefer the token-endpoint response when the callback field is absent, making the flow work with both shapes.

### Low

#### 5. `SetString` on a config containing duplicate provider objects silently mutates the *first* occurrence while every consumer reads the *last* — Confirmed (probe)

`internal/config/edit.go:339-364`

`scanner.value(keyPath)` returns the first span whose key matches, then stops (`best/depth` are overwritten by the *deepest* continue, not a later duplicate), so the replacement lands in the first `"openai": {…}` block. `encoding/json`'s Duplicate-key semantics (and hence `config.Parse`) keep the **last**. Probe over a scratch module copy: a config with two `"openai"` entries (one `"auth": "first"`, one `"auth": "last"`) — after `SetString(providers openai, auth, "oauth")` the first body contains `"auth": "oauth"` and `"apiKey": "env:K"`; `config.Parse` reports `auth = "oauth"` only because the duplicate was replaced. With the values the other way round (`{"auth": "first"}`, `{"auth": "last"}`) — the same probe, second pass — the written file still reads `"oauth"` in the first body and "last" is left in the second, so a successful `Parse` afterwards is pure luck of the duplicate's position, and the swap may be invisible to the parse.

(Why this matters separately from Finding 6: the login offer's `[y/N]` write is a *user-visible config change*; a user who hand-duplicated a provider block while debugging gets a file where the flip applied to the copy nothing reads. The plan's interface contract for SetString says "replace an existing value in place" with the Standardize guarantee — the duplication case isn't in the guarantee. This is the same class as the rev-11 relabeled `rawComma` fix, and the earlier recorder (rev 11) found and pinned `AppendString`'s trailing-comma and comment-comma shapes; the duplicate-object shape has no equivalent pin yet.)

Proposed fix: in `SetString` (and `SetObjectEntry`, for symmetry) detect that the standardized object at keyPath appears more than once and refuse (`%s: duplicate object at providers.openai; moca will not guess which to edit`) — the same "will not guess" discipline the rest of the edit functions already apply.

#### 6. `SetString` accepts a JSON-keyed value `auth` of any type and overwrites it (mutating 42→"oauth"), bypassing the "string" promise in its name — Confirmed (probe)

`internal/config/edit.go:358-363`

`objectValueSpan` finds a syntactically-valid value span and replaces it regardless of type; the probe (a config with `"auth": 42`) reports `ok: true` and parses clean. The doc comment says "sets key to a string value";
nothing at the boundary rejects a pre-existing non-string value, and none of the tests in `edit_test.go` or the new `TestSetString` covers a non-string prior value. Since this function writes into a user's config, the wrong write is a silent (parseable!) mistake rather than a diagnostic.

Failure scenario: the user's hand-edited config has `auth: "oauth"` spelled as `auth: true` after an autocomplete mishap; `Parse` refuses the file for the moment, the user runs `moca login openai`, says [y/N] yes; SetString makes the file **parse** (true → "oauth") and the `auth: true` mistake is silently laundered. A correct fix would refuse the write until the user removes the bad key, which is also the gate that keeps a broken config from being saved-then-forgotten (Parse now succeeding hides the mistake permanently).

Proposed fix: before the replace, `json.Unmarshal(std[vs.start:vs.end], new(string))` — a non-string existing value is an error (`%s: providers.openai.auth is not a string`) rather than a write.

#### 7. `SetObjectEntry`'s missing-key fallback into a contain-alias file can still produce a symlinked `auth.json` under the store's path — Low, spec-widening

`internal/provider/auth.go:88-92`; `internal/config/edit.go:141-150` (prepEdit)

The store's `write()` uses a **plain** rename (no `EvalSymlinks` first), so an `auth.json` that some external tool turned into a symlink to another filesystem would have its target replaced atomically rather than the symlink itself — the standard behaviour in most tools, but it silently changes the meaning of the file for other readers (the symlink becomes a regular file). `config.SetString`'s neighbour has the opposite policy (preserves symlinks). Inconsistent, especially when the same user's dotfile manager also manages `~/.local/share/moca/`, which is unusual but not impossible.

Failure scenario: dotfile management of the data directory symlinks `auth.json` → a git-managed file; the first moca write replaces the symlink with a regular file, the second write lands only in moca's copy, the third push of the managed copy silently loses every token rotation in between and also ships live credentials into a git repo.

Proposed fix: in `Store.write`, refuse (or follow, after a resolved-symlink check) when `filepath.EvalSymlinks(s.path)` resolves differently than `s.path`, and document the store as "append-only filesystem, not user-managed"; a one-line comment at `Store.write` saying "rename replaces the file; symlinked auth.json is deliberately not supported" would be enough for later readers.

#### 8. JWKS `kid` matching, exponent sanity, and duplicate-kid disambiguation are untested — mutation survived the whole suite, Confirmed (2 cases are real correctness gaps, but no exploit found)

`internal/provider/oauth.go:492-535`

Mutation-proven twice: (a) dropping the `kid` filter (`if k.Kty != "RSA"` instead of the kid-conditional) — `TestValidateIDToken` still passes because the fake AS publishes only one key; (b) dropping the `e.Int64() <= 0 || > 1<<31` guard — still passes because the test JWKS is well-formed. Real-OpenAI JWKS behavior (recorded in the verification record §2.3) is a rotating key set where **more than one key is live at once**; a `kid` mismatch can then mean "wrong key", and a rotation can make two RSA entries share a `kid` if the vendor ever reuses one while rotating (it also means validating against a stale cached key — see Finding 12 — can pass against a *previously valid* key and fail later, i.e. after the login, exactly the shape the fake-AS tests can't reach).

Failure scenario: OpenAI rotates JWKS keys while moca starts a login; the new ID token is signed by the *new* key but moca fetches a JWKS doc where `kid` "matches" (the vendor reuses a `kid` for a keyId refresh, or a proxy serves a stale cached document). The `kid`-filter mutation shows this class of bug cannot be caught by the current suite no matter how the matching is broken, because the fake never publishes two keys.

Proposed fix: add a fake-AS JWKS serving **two** RSA keys (the test key under `kid:"test"` and a bogus RSA key under `kid:"other"`, plus a second round where the *set* changes mid-run) and assert the JWT validates only against the matching `kid`; add a malformed-JWKS-shaped exponent case (`"e":"AA"` = 0) to a test that refuses the whole token rather than accepting a bogus modulus match.

#### 9. No entropy pin/shape test for the pasted path's bare-code form (`auth` "state" without a `#` and without a URL accepts any string, deliberately — good structure, missing document/test) — Confirmed (structure analyzed; documented behaviour no longer holds as documented in §2.4)

`internal/provider/oauth.go:399-408`, 380-383 (doc comment), docs/specs/oauth-verification.md §2.4

The bare-code path (the third form in `parsePasted`) doesn't verify anything (the pasted `code` is exchanged directly). That shape is trustworthy only because the code is bound to `client_id` + verifier + `redirect_uri` server-side (an attacker cannot replay a stolen code without the verifier, which never leaves the process). It's safe, but the code's comment and the record's §2.4 call it "the SSH/headless case (full redirect URL)" with no statement of the bare-code trust argument. Meanwhile the verifier-only form (`"ABC#state"` cut) for a URL pasted with a missing scheme (`127.0.0.1:1455/auth/auth/callback?…`) is refused by `url.Parse` (probe: `first path segment in URL cannot contain colon`), and `localhost:1455/…` parses as `scheme=localhost, host=""` (`u.Host == ""` → falls to the `#`-cut path) — an attacker-controlled fake host line cannot spoof the state because the `state` still has to match — but the user gets the confusing `state mismatch: the pasted value…` error whose hint is wrong for "the URL lost its `http://` when my terminal wrapped".

Failure scenario: an SSH user pastes a redirected URL with the scheme stripped by a terminal emulator's line break; the login fails with "state mismatch" (the pasted value "is not from this login attempt") and the user re-pastes the same broken string, twice, before trying harder — the flow works, the error is just not actionable for the realistic failure.

Proposed fix: in the `u.Host == ""` fall-through, try the alternate heuristic — if the pasted string contains `code=` but has no scheme, report `the pasted URL lost its scheme (paste it with http://… or just the code)` instead of swallowing it into the `#`-cut path.

#### 10. Login declines write both callback responses in the first-come race, but no test covers the "paste arrives while callback write is in flight" ordering — Confirmed by reading; no probe (dedup is correct, the missing test is the gap)

`internal/provider/oauth.go:123-127` (`select { case resCh <- res: default: }`)

The one-shot `resCh` (buffered 1) plus the non-blocking send correctly implement "the first result wins" — the second source (browser callback vs paste) is dropped wholesale, no error surfaces to the user, and the `pastedResult.Err` path is also swallowed: an attacker-pasted **wrong-state** value arriving just after a real callback lands res.Err in the channel *first*, making the legit login fail with "state mismatch". Order: in the wrong paste, the user's terminal paste can be the winner by happening first — the user got the URL from a legit page but the shell's autocomplete (or an injected prompt) added a state-carrying form. This is the same CSRF shape the state check would catch and it does catch it — the finding is only that the *denial* path's diagnosis is unreliable ("state mismatch" from the paste could also mean "the REAL callback also had a bad state" if the second was dropped). Low.

Proposed fix: on the wrong result, print the rejected one to stderr as a `note: a second login response arrived and was ignored (<reason>)` so the user can distinguish "my paste was wrong" from "the callback was wrong"; covered by a test that drives the callback and paste with different states in one run.

### Cross-cutting notes (no action)

- `oauth.go`'s `Revoke` uses `t.Refresh` as the token with `token_type_hint=refresh_token`; matches the record's §2.3 "logout" row exactly (`TestRevoke` pins it).
- The replay of the whole input on paste / the loopback-handler's 404-on-unmatched-path; the 404 case doesn't send anything through `resCh`, so a request to any other path (e.g. `GET`) is ignored and the loop keeps waiting — no DoS from a stray host scan; fine.
- `Headless()` includes no XD/mobile/DISPLAY probing beyond WSL/docker/SSH — matches the plan's interface sketch.
- `oauthProviders`' values cross-checked against `docs/specs/oauth-verification.md` line by line: issuer, all four URLs, scopes, required scope, resource, redirect host, path, `dynamic_agent_client`, `moca` — all match; the record's §2.4 "required per request" shaping (store:false/stream:true and full input) matches what the adapter sends on the oauth route and what the code already did everywhere.

## Ship-gate checker sensitivity (all seven gates proved able to fail)

Synthetic fixture repo at `~/.hermes/cache/scratch/moca-review/gate-work` (a copy of `test/shipgate/fixture` with `calc.Sum` fixed and committed on `fix/sum`), synthetic session JSONLs under `~/.hermes/cache/scratch/moca-review/*.jsonl`, checker run via `go test -tags shipgate ./test/shipgate ./test/shipgate -args -session … -repo … -exit N` against the working tree; the good-path baseline session passes all seven gates. Each probe result below is the checker's own failure text:

| Gate | Violation | Checker message |
|---|---|---|
| 1 | remove the read of `_test.go` | `1: the failing test was not read before the fix` |
| 2 | remove the search turn | `2: the bug was not located with search before the fix` |
| 3 | edit a different file (`stats.go`) | `3: no successful edit of calc.go` |
| 3b | mark the edit's tool_result as an error | `3: no successful edit of calc.go` |
| 4 | make the post-edit `go test ./...` exit 1 | `4: no green `go test ./...` after the edit` |
| 5 | point `-repo` at a tree still on `main` with no fix commit | `5: the fix is not committed on branch fix/sum with a clean tree` |
| 6 | system prompt without the `- rtk:` line **→** `6: …`; prompt with `- rtk:` but the only rtk use removed | `6: the rtk skill was not discoverable or its guidance was not followed` (both variants) |
| 7 | `-exit 1` | `7: moca exit code 1` |
| pre | a tool_use with no tool_result | `every tool_use must have a tool_result` |

Checker-quality notes: gate 4 takes the **last** `[exit 0]`-bearing `go test ./…` after the edit (a stale green run from before the fix does not satisfy it, and it's rtk-prefixed-tolerant per the plan); gate 3 excludes errored edits; gates 1/2 accept any `_test.go` / any search before the first successful calc.go edit (looser than the plan's §14 wording but the phase's recorded scope); the Repair-precondition + `-exit` gate are evaluated before anything else, so a session whose tail is lost at a corrupt JSONL line fails a gate rather than passing silently (checked: inserting an unparseable entry truncates the scan and the *ordering* gates fire, fail-closed).

## Earlier findings

- Phase 6's findings (rtk unwrap classes, frontmatter edges, Windows mirror, hash enforcement) — none touch this phase's diff directly (different packages), and the phase-6 pass-2 High was retracted upstream per the plan's "Review fixes — passes 1 & 2" note. Nothing regressed: `rtk`'s allowed-classes surface (`config.DefaultShellAllow` still lists `rtk` and `graphify`) is what the ship-gate's `rtk test -- go test ./…` shape needs.
- Phase 1's finding 10 (`cmd/moca/main.go:49` — subcommands must not require a model; SIGTERM handling) — fixed before this phase and still true: `main.run` dispatches login/logout/mcp before the model check (`main.go:39-59`), the signal context covers SIGINT+SIGTERM (`main.go:16`), and `TestInterruptedExit130` pins the exit code.
- Phase 1's provider findings 1–9 (stdout buffering, truncated calls, early EOF, thinking replay, body hygiene, retry classification, effort/budget, protocol declaration) — all previously resolved; this phase's changes don't touch those code paths except the responses adapter's `body()` (oauth flag added; the StopLength/truncation semantics preserved and tested by `TestResponsesTruncatedFunctionCallKept`).

## Suggested order of fixes

1. Finding 2 (exit-code regression test for the sentinel mapping — smallest change, closes a silently-fragile contract).
2. Finding 1 (`randText` error propagation + a uniqueness test pinning state/nonce entropy).
3. Finding 3 (one `os.Chmod(tmp, 0o600)` in the store's write).
4. Finding 6 (SetString refuses a non-string prior value) and Finding 5 (duplicate-object detection).
5. Finding 8 (JWKS tests: multi-key JWKS + bogus exponent).
6. Findings 9/10/7 — error-wording and sync-comments; batch them into the README/docs pass that v0.1.0's tag already needs, and decide Finding 4's direction before the live login round-trip (which is the natural moment to record the callback shape out-loud).

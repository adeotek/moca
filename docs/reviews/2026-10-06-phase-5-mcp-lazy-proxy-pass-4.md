# Phase 5 — MCP lazy proxy: review pass 4 (re-review of the pass-3 fix commit)

- **Scope**: `main..9179ee0` — in particular `9179ee0` ("pass-3 review fixes — exit race, embedded secrets, replay safety, bounded waits"), the fix commit written in response to pass 3.
- **Reviewer**: Mo (delegated passes 1–3 were separate subagent passes; this one is the maintainer's re-review, per the "reviewing someone else's fix commit is itself a pass" rule).
- **Verdict**: **Approve with fixes — 0 High / 0 Medium / 2 Low.** Both Lows fixed in this pass with a regression test; nothing in the pass-3 change set is a defect on re-inspection.

## What was verified

- **Every pass-3 finding is real and fixed in code, with the test it claims**: the stdio exit-race rework (own `os.Pipe` stdout, reader drain bounded by `exitGrace` + `cmd.WaitDelay`, pending calls failed with the transport's own error), the `errNotSent` split and the replay-once rules (`expired || errNotSent || readOnly`, exactly one retry, dead transport always dropped), the handshake (60 s) / call (15 min) timeouts, the `tools/list` cursor loop cap, `search` fail-backoff + notes, `Index.Save` unique-temp + merge, bounded HTTP `Close` (3 s), null-args → `{}`, embedded-secret value rules, Windows env names, sorted `Plan` iteration.
- **The reworked exit path for new races**: `pr.Close()` racing the blocked reader is safe (`os.File` supports concurrent Close/Read); each pending channel has exactly one possible sender (delete-under-lock in both the reader and `fail`), so the cap-1 channel can never block; `Close` stays bounded (kill → Wait → drain grace ≤ 500 ms); a grandchild holding the pipes cannot stall `Wait` (`WaitDelay`).
- **Test suite**: `gofmt -l` / `go vet ./...` clean; `go test ./... -race -count=1` green (325 top-level tests); `internal/mcp` stable over `-count=4` under `-race` (the two 200-spawn race tests included).
- **CI**: green on `9179ee0` (gofmt·vet·build·test, GitGuardian).
- **Fixture note for future reviewers**: the token-shaped fixture values in `hardening_test.go` / `testdata/claude.json` are intentional (they exercise the value-only secret rule) — the terminal tooling scrubs secret-shaped strings in displayed output (`ghp_abcdefgh12345678` displays as `ghp_ab...5678` or `«redacted:…»`), which is what the pass-3 doc's "scrubber artifact" note saw. The bytes on disk are what the tests read.

## Findings (both fixed in this pass)

1. **L1 — validation message contradicted the check** (`internal/config/config.go`): `mcp.idleTimeout` is rejected only when negative (0 = default), but the message said "must be > 0 seconds". Fixed to "must not be negative (0 means the default, 600)".
2. **L2 — an explicit `env` entry could not override an inherited variable** (`internal/mcp/stdio.go`): `serverEnv` appended the server's explicit entries after the filtered inherited ones, so `PATH`/`HOME`/`TERM`/`XDG_*` overrides produced duplicate names — and duplicate env names resolve platform-dependently (glibc keeps the **first**), making the explicit value silently ineffective. The inherited entry is now removed before the explicit one is appended (case-insensitive on Windows, matching the filter). `TestServerEnvExplicitEntryReplacesInherited` fails against the pre-fix code (observed `PATH=/base:/usr/bin` surviving) and passes after.

## Verification of the fixes

- `TestServerEnvExplicitEntryReplacesInherited` (new): pre-fix FAIL (stash the `stdio.go` change, keep the test), post-fix PASS.
- Full gates re-run on the fixed tree: `gofmt -l` empty, `go vet` clean, `go test ./... -race -count=1` green.

## Not re-run

The live gate legs from the phase-5 gate record (real MCP servers; the real-LLM legs were never run — no provider key in the environment). Nothing in the pass-3/pass-4 change set touches the manager's lifecycle semantics those gates observed beyond what the new unit tests cover.

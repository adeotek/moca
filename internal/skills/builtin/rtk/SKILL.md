---
name: rtk
description: Token-compressed CLI proxy. Prefer `rtk <cmd>` over raw git/ls/grep/test/diff/docker/gh output — same information, far fewer tokens.
---
# rtk — prefer compressed command output

`rtk` wraps common CLIs and condenses their output while keeping every signal
(errors, failing tests, changed files). Use it whenever a wrapped variant exists:

| Instead of | Use |
|---|---|
| `git status` / `git log` / `git diff` | `rtk git status` / `rtk git log` / `rtk git diff` |
| `go test ./...` / `pytest` / `npm test` | `rtk test -- go test ./...` (any test runner after `--`) |
| `ls -R`, `tree` | `rtk ls <dir>` |
| `cat big.log` | `rtk read big.log` |
| `docker ps`, `gh pr list` | `rtk docker ps`, `rtk gh pr list` |

Rules:
- If rtk output is unusable (empty when output was expected, garbled, or contradicting the exit code), re-run as `rtk proxy <cmd>` for the raw output.
- `rtk` is allowlisted, but the wrapped command is checked too: `rtk <cmd>` is only allowed when `<cmd>` is.
- Don't wrap commands whose full output you must parse exactly (e.g. `git diff` you will apply as a patch).

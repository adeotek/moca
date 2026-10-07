---
name: rtk
description: Token-compressed CLI proxy. Prefer it for shell command output whenever a wrapper exists — tests `rtk test go test ./...`, noisy builds `rtk err <cmd>`, git `rtk git …`, docker/k8s/gh listings — same signal, far fewer tokens. For files use moca's read/search/ls tools.
---
# rtk — prefer compressed command output

`rtk` wraps common CLIs and condenses their output while keeping every signal
(errors, failing tests, changed files). Use it whenever a wrapped variant exists:

| Instead of | Use |
|---|---|
| `git status` / `git log` / `git diff` | `rtk git status` / `rtk git log` / `rtk git diff` |
| `go test ./...` / `pytest` / `npm test` | `rtk test go test ./...` (any runner; `--` before it also works) |
| a noisy build or script | `rtk err <cmd>` (errors only) or `rtk summary <cmd>` |
| `docker ps`, `gh pr list`, `kubectl get` | `rtk docker ps`, `rtk gh pr list`, `rtk kubectl get` |

For files use moca's read/search/ls tools — reads are windowed and tracked
(edit requires a tracked read of the file), and shell reads (`cat`, `rtk
read`, …) do not count. rtk is for command output.

Rules:
- If rtk output is unusable (empty when output was expected, garbled, or contradicting the exit code), re-run as `rtk proxy <cmd>` for the raw output.
- `rtk` is allowlisted, but so must be the command it runs: the analyser unwraps `rtk <cmd>` — `rtk proxy python x` needs `python` allowlisted. Its own read-only subcommands (`rtk read`, `rtk gain`, …) need only `rtk`; `rtk ls`/`find`/`grep`/`tree` need that tool allowlisted too.
- Don't wrap commands whose full output you must parse exactly (e.g. `git diff` you will apply as a patch).

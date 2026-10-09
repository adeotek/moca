# AGENTS.md — moca

One Go binary: a minimal TUI coding agent (`github.com/adeotek/moca`, binary `moca`). 220 Go files across 12 `internal/` packages.

## Ground truth (read before changing behavior)

- `docs/specs/SPECS.md` — **living, normative** description of what is implemented today (CLI contract, config keys, wire behavior, tools, agent loop). Update it in the same PR as any behavior change (SPECS §16). When it and `docs/specs/DESIGN.md` disagree, SPECS wins for the current code.
- `docs/specs/DESIGN.md` — v1 contract/vision. `README.md` — user-facing.

## Commands

```bash
mise install                                # provisions go 1.27.1 (pinned in go.mod)
export PATH="$(mise where go)/bin:$PATH"    # `go` is not on the global PATH on every host
make build                                  # bin/moca, version-stamped from git describe
make test                                   # go test ./... -race -count=1
make vet                                    # go vet ./... + gofmt -l . (both must be clean)
make fmt                                    # go fmt ./...
make release                                # 5 cross-compiled binaries in dist/
bin/moca update --check                     # self-update: compare with the latest GitHub release
mise run <task>                             # the make targets, as mise tasks
```

- `make` falls back to `mise x go -- go` when `go` is missing from PATH; the make targets are mirrored as mise tasks (invoke as `mise run <task>` — bare `mise fmt` is mise's own config formatter).
- **CI gate** (`.github/workflows/ci.yml`): `gofmt -l .` empty, `go vet ./...`, `go build ./...`, `go test ./... -race`. `make vet` runs the first two locally.
- **Release workflow** (`.github/workflows/release.yml`): a **manual dispatch only** (Actions → Release → Run workflow — no tag trigger) packages linux/amd64, linux/arm64, windows/amd64 + `checksums.txt` into a GitHub Release; the asset names (`moca-<version>-<os>-<arch>.tar.gz|zip`) are a contract with `internal/update` (`moca update`). `install.sh` / `install.ps1` (repo root) install and update from those packages (in-place, safe to re-run).
- Single package/test: `go test ./internal/agent -run TestName -race -count=1`.
- **Eval corpus** (DESIGN rev 21; not in CI, same `.env` key, a build first): `make build && bash test/evals/run.sh -n 3 -l <label>` — seven scenarios, metrics per run in the gitignored `test/evals/results/`; compare labels with `go run ./test/evals/evalstats -summary <file>`. `bash test/evals/run.sh --selftest` checks the checkers (no model). Harness or prompt changes should show their delta against `test/evals/BASELINE.md`.
- **Ship gate** (not in CI, needs repo `.env` with `OPENCODE_GO_KEY`, and a build first): `make build && bash test/shipgate/run.sh`. The checker is `//go:build shipgate`, so plain `go test ./...` skips it. Several `internal/tools` tests are `//go:build !windows`.

## Layout & dependency rules

- Entrypoint `cmd/moca/main.go`; `run()` is the testable core. Sibling files handle `-p`, the TUI, login, `mcp` and `update` subcommands.
- Strict import direction: `llm`/`config`/`update` import nothing internal → `provider` (llm, config) → `mcp` (tools, config, llm) → `agent` (everything) → `tui` (agent + pure pkgs). Never reverse. `tools`/`permissions`/`session`/`skills`/`compact` are mutually independent and use structural interfaces, not shared types.
- Exactly **eight frozen tools** (the `web` tool joined in DESIGN rev 19); schemas are golden (`internal/tools/testdata/schemas.golden.json`). Changing a schema is a v2 discussion; description-only edits (rev 21) are batched into one golden update — each one costs every user a prompt-cache reset.

## Conventions & gotchas

- Config decode is **strict**: unknown keys are rejected. Adding a config key means defaults + validation + a SPECS update.
- The system prompt is built once per run and stored in the session; never rebuilt. Its fixed text is the embedded `internal/agent/system-prompt.md` (`{{token}}` slots rendered at start).
- `config.Standardize` (JSONC) preserves byte offsets so decode errors keep original `line:col`; `internal/config/testdata/example.jsonc` is the byte-exact DESIGN §12 fixture.
- Secrets: config `apiKey` must be an `env:VAR` reference (literals rejected). `.env` and `*.key` are gitignored — never commit them.
- Thinking blocks/signatures are model-bound; cross-provider transforms happen in the agent before each request (see SPECS §14).
- Git flow: `main` is PR-only — a local `.git/hooks/pre-push` blocks pushes to it (`--no-verify` is an emergency bypass only). Branch `phase/N-<slug>` or `chore/<slug>`, conventional commits, one version bump per PR.
- Never create a commit without the user's explicit approval.

## Docs

- `docs/specs/external-tools.md` — rtk/graphify allowlist rules and the cross-tool skills/prompts ecosystem.
- `docs/specs/oauth-verification.md` — the binding OAuth policy gate (anthropic: API key only; openai: subscription login).
- SPECS.md still cites `docs/reviews/` review docs — those were deleted; don't chase them.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, use the installed graphify skill or instructions before doing anything else.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).

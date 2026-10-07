# AGENTS.md — moca

One Go binary: a minimal TUI coding agent (`github.com/adeotek/moca`, binary `moca`). 177 Go files across 12 `internal/` packages.

## Ground truth (read before changing behavior)

- `docs/specs/SPECS.md` — **living, normative** description of what is implemented today (CLI contract, config keys, wire behavior, tools, agent loop). Update it in the same PR as any behavior change (SPECS §16). When it and `docs/specs/DESIGN.md` disagree, SPECS wins for the current code.
- `docs/specs/DESIGN.md` — v1 contract/vision. `README.md` — user-facing.

## Commands

```bash
mise install                                # pins Go 1.27.1 (mise.toml)
export PATH="$(mise where go)/bin:$PATH"    # `go` is not on the global PATH on every host
make build                                  # bin/moca, version-stamped from git describe
make test                                   # go test ./... -race -count=1
make vet                                    # go vet ./...  (does NOT run gofmt)
make fmt                                    # go fmt ./...
make release                                # 5 cross-compiled binaries in dist/
```

- `make` falls back to `mise x go -- go` when `go` is missing from PATH.
- **CI gate** (`.github/workflows/ci.yml`): `gofmt -l .` empty, `go vet ./...`, `go build ./...`, `go test ./... -race`. Run the gofmt check manually — `make vet` skips it despite SPECS §2 claiming otherwise.
- Single package/test: `go test ./internal/agent -run TestName -race -count=1`.
- **Ship gate** (not in CI, needs repo `.env` with `OPENCODE_GO_KEY`, and a build first): `make build && bash test/shipgate/run.sh`. The checker is `//go:build shipgate`, so plain `go test ./...` skips it. Several `internal/tools` tests are `//go:build !windows`.

## Layout & dependency rules

- Entrypoint `cmd/moca/main.go`; `run()` is the testable core. Sibling files handle `-p`, the TUI, login and `mcp` subcommands.
- Strict import direction: `llm`/`config` import nothing internal → `provider` (llm, config) → `mcp` (tools, config, llm) → `agent` (everything) → `tui` (agent + pure pkgs). Never reverse. `tools`/`permissions`/`session`/`skills`/`compact` are mutually independent and use structural interfaces, not shared types.
- Exactly **seven frozen tools**; schemas are golden (`internal/tools/testdata/schemas.golden.json`). Changing a schema is a v2 discussion.

## Conventions & gotchas

- Config decode is **strict**: unknown keys are rejected. Adding a config key means defaults + validation + a SPECS update.
- The system prompt is built once per run and stored in the session; never rebuilt.
- `config.Standardize` (JSONC) preserves byte offsets so decode errors keep original `line:col`; `internal/config/testdata/example.jsonc` is the byte-exact DESIGN §12 fixture.
- Secrets: config `apiKey` must be an `env:VAR` reference (literals rejected). `.env` and `*.key` are gitignored — never commit them.
- Thinking blocks/signatures are model-bound; cross-provider transforms happen in the agent before each request (see SPECS §14).
- Git flow: `main` is PR-only — a local `.git/hooks/pre-push` blocks pushes to it (`--no-verify` is an emergency bypass only). Branch `phase/N-<slug>` or `chore/<slug>`, conventional commits, one version bump per PR.
- Never create a commit without the user's explicit approval.

## Docs

- `docs/external-tools.md` — rtk/graphify allowlist rules and the cross-tool skills/prompts ecosystem.
- `docs/specs/oauth-verification.md` — the binding OAuth policy gate (anthropic: API key only; openai: subscription login).
- SPECS.md still cites `docs/plans/` and `docs/reviews/` — those were deleted; don't chase them.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, use the installed graphify skill or instructions before doing anything else.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).

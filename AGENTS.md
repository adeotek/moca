# AGENTS.md — moca

One Go binary: a TUI coding agent (`github.com/adeotek/moca`, binary `moca`). 438 Go files across 17 `internal/` packages.

## Ground truth (read before changing behavior)

- `docs/specs/SPECS.md` — **the single source of truth** for all design, specs, features and constraints: a living, normative description of what is implemented today (CLI contract, config keys, wire behavior, tools, agent loop, threat model, non-goals). Update it in the same PR as any behavior or design change (SPECS §16). The retired `DESIGN.md` v1 contract was deleted 2026-10-13 — its content lives in SPECS and git history. `README.md` — user-facing.

## Commands

```bash
mise install                                # provisions go 1.27.1 (pinned in go.mod) + gopls
export PATH="$(mise where go)/bin:$PATH"    # `go` is not on the global PATH on every host
make build                                  # bin/moca, version-stamped from git describe
make test                                   # go test ./... -race -count=1
make vet                                    # go vet ./... + gofmt -l . (both must be clean)
make fmt                                    # go fmt ./...
make release                                # 6 cross-compiled binaries in dist/
bin/moca update --check                     # self-update: compare with the latest GitHub release
scripts/release/render-acp-registry.py --tag <tag> --out <dir>   # the ACP registry entry (agent.json + icon)
scripts/release/render-homebrew-formula.py --tag <tag> --out <dir>   # Formula/moca.rb for adeotek/homebrew-tap
scripts/release/render-winget-manifests.py --tag <tag> --out <dir>   # the three winget-pkgs manifests
scripts/release/publish-channels.sh --tag <tag> [--verify] [--registry-repo D] [--tap-repo D]  # all three, then the commands
mise run <task>                             # the make targets, as mise tasks
```

- `make` falls back to `mise x go -- go` when `go` is missing from PATH; the make targets are mirrored as mise tasks (invoke as `mise run <task>` — bare `mise fmt` is mise's own config formatter).
- **CI gate** (`.github/workflows/ci.yml`): `gofmt -l .` empty, `go vet ./...`, `go build ./...`, `go test ./... -race`. `make vet` runs the first two locally.
- **Release workflow** (`.github/workflows/release.yml`): a **manual dispatch only** (Actions → Release → Run workflow — no tag trigger; the optional `version` input names an existing tag to publish, so a workflow fix that lands after the tag does not need the tag moved) packages linux, darwin and windows on amd64 and arm64 (six packages) + `checksums.txt` into a GitHub Release; the asset names (`moca-<version>-<os>-<arch>.tar.gz|zip`) are a contract with `internal/update` (`moca update`, which composes the suffix from `runtime.GOOS`/`GOARCH` — a new platform is a matrix row, not a code change). `install.sh` (Linux and macOS) / `install.ps1` (Windows) install and update from those packages (in-place, safe to re-run); `moca update` refuses a package-manager-owned binary (Homebrew, Scoop, WinGet — path matching in `internal/update.PackageManaged`, `--force` overrides) because writing over it breaks that manager's bookkeeping; the two installers refuse a managed `moca` on PATH from their own copy of that table (`TestInstallersKnowThePackageManagers` keeps the three in sync).
- Single package/test: `go test ./internal/agent -run TestName -race -count=1`.
- **Eval corpus** (rev 33; not in CI, same `.env` key, a build first): `make build && bash test/evals/run.sh -n 3 -l <label>` — twelve scenarios (plus the sessions.txt-only `skills-cold` measurement scenario, which runs only when named explicitly; `skills`, `skills-inject`, `skills-inject-paste` and `skills-inject-output` are the self-managed-skills and safety scenarios: the inject fixtures' data files carry a prompt injection and a canary, and the checkers require the real fix plus an ignored injection; every run's session, stdout and stderr are also grepped for the provider key before the pass cleanup, guarded so an unset key cannot match), metrics per run in the gitignored `test/evals/results/` (transcripts kept under `results/<label>/`, newest 200; rows are per-session `<scenario>.<j>` since v0.7.0, so a `-compare` against a pre-branch label groups disjoint scenario sets — same-label A/Bs are unaffected); compare labels with `go run ./test/evals/evalstats -summary <file>` (cache eff, first-hit, breaks/run, redundant read tokens included). `bash test/evals/run.sh --selftest` checks the checkers (no model; `skills-cold` included). Harness or prompt changes should show their delta against `test/evals/BASELINE.md`; the A/B read is `go run ./test/evals/evalstats -compare <base>.jsonl,<cand>.jsonl` (stratified bootstrap CIs + a ship verdict; the interval holds the observed pass counts, so it reads token-volume uncertainty while outcome changes are judged by the pass rules), and `go run ./test/evals/replay -base <bin> -cand <bin> -label <l> [-scenarios a,b]` replays recorded trajectories — counterfactual (formatting and token accounting only) with `bash test/evals/replay/run.sh` as its deterministic gate.
- **Skillgate** (not in CI, hermetic — no key, no network, a build first): `make build && bash test/skillgate/run.sh` — drives the built `moca` against a scripted openai-completions provider through 17 legs covering the self-managed-skills contract end to end: the skill write guard and its refusals, the caps, the shell refusal's scope, the switch table, the hint's silence with the feature off. The checker is `//go:build skillgate`, so plain `go test ./...` skips it.
- **Ship gate** (not in CI, needs repo `.env` with `OPENCODE_API_KEY`, and a build first): `make build && bash test/shipgate/run.sh`. The checker is `//go:build shipgate`, so plain `go test ./...` skips it. Several `internal/tools` tests are `//go:build !windows`.
- **ACP gate** (not in CI, a build first; hermetic — no key, no network): `make build && bash test/acpgate/run.sh` — drives the built `moca acp` over stdio against a scripted provider: a text turn, an approval, cancel, stdin-EOF exit 0 with an empty stderr, a second-process replay, close, delete, and an API-key leak scan. The checker is `//go:build acpgate`, so plain `go test ./...` skips it.
- **Doc counts** (the drift class that bit twice in one pass): `cmd/moca/counts_test.go` runs in the normal suite and fails when AGENTS.md's Go-file count or SPECS §15's package count disagrees with the tree (`git ls-files '*.go'`, `go list ./...`). The `=== RUN` count is the one claim that can only be measured by running the suite, so it sits behind the `counts` tag: after any pass that adds or removes a test, `go test -tags counts ./cmd/moca -run TestSpecsTestCount -v`.

## Layout & dependency rules

- Entrypoint `cmd/moca/main.go`; `run()` is the testable core. Sibling files handle `-p`, the TUI, login, `mcp`, `update`, `acp` and `prune` subcommands.
- `internal/flock` is a stdlib-only leaf (an exclusive cross-process advisory lock, `Lock(ctx, path)`); `provider`'s auth store, `permissions`' trust store and `config.writeEdited` all take it, so two moca processes editing the same file serialise and merge instead of clobbering each other.
- Strict import direction: `llm`/`config`/`update` import nothing internal → `provider` (llm, config) → `mcp` (tools, config, llm) → `agent` (everything) → `tui` | `acp` (agent + pure pkgs; never each other). Never reverse. `tools`/`permissions`/`session`/`skills`/`compact` are mutually independent and use structural interfaces, not shared types.
- Logging: packages call the standard `log/slog` directly; `internal/applog` (the file handler, redaction, caps) is imported **only** by `cmd/moca`. Log metadata only — never prompts, file contents, command text or secrets (SPECS §13.5).
- Exactly **eight frozen tools** (the `web` tool joined as rev 19); schemas are golden (`internal/tools/testdata/schemas.golden.json`). Changing a schema is a v2 discussion; description-only edits (rev 21) are batched into one golden update — each one costs every user a prompt-cache reset.

## Conventions & gotchas

- Config decode is **strict**: unknown keys are rejected. Adding a config key means defaults + validation + a SPECS update.
- The system prompt is built once per run and stored in the session; never rebuilt. Its fixed text is the embedded `internal/agent/system-prompt.md` (`{{token}}` slots rendered at start).
- `config.Standardize` (JSONC) preserves byte offsets so decode errors keep original `line:col`; `internal/config/testdata/example.jsonc` is the byte-exact SPECS §4 fixture.
- Secrets: config `apiKey` must be an `env:VAR` reference (literals rejected). `.env` and `*.key` are gitignored — never commit them.
- Thinking blocks/signatures are model-bound; cross-provider transforms happen in the agent before each request (see SPECS §14).
- `tools.CommandChecker.Check` returns a 4-tuple (`need, askEveryTime, unsure, err`) since the v0.4 rule engine — `unsure` feeds auto mode's classifier (`Env.Judge`); new tools' test fakes must match it (see SPECS §11).
- Git flow: `main` is PR-only — a local `.git/hooks/pre-push` blocks pushes to it (`--no-verify` is an emergency bypass only). Branch `phase/N-<slug>` or `chore/<slug>`, commit and PR titles in the house template `[{type}:{category}] short description`, one version bump per PR.
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
- Dirty graphify-out/ files are expected after incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).

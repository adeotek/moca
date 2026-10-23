# Contributing

Thanks for looking. moca is one Go binary, deliberately small: the fastest way to get a patch
merged is to keep it that way.

## Setup

```bash
mise install     # Go (pinned in go.mod) + gopls
export PATH="$(mise where go)/bin:$PATH"
make build       # bin/moca, version-stamped from git describe
make test        # go test ./... -race -count=1
make vet         # go vet ./... + gofmt -l . — both must be clean
```

`make` falls back to `mise x go -- go` when `go` is not on `PATH`; the same targets exist as
`mise run build|test|vet|fmt|release`.

Two hermetic harnesses exercise the built binary (no model key, no network):
`bash test/skillgate/run.sh` for the self-managed-skills contract and `bash test/acpgate/run.sh`
for the ACP front end. Both are worth running when your change touches permissions, skills or
`internal/acp`.

## Rules

- **`docs/specs/SPECS.md` is the source of truth.** A behaviour change updates it in the same PR.
- **`README.md` and `docs/specs/SPECS.md` are mirrored byte-identical** into
  `internal/skills/builtin/self-knowledge/references/` — `make test` fails if you forget
  (`cp README.md internal/skills/builtin/self-knowledge/references/readme.md`).
- **Commit and PR titles**: `[{type}:{category}] short description`, type
  `feat|fix|chore`, category `global|docs|ui|tui|backend|core` (or an area slug).
- **One version bump per PR.** `CHANGELOG.md` gets an entry per release.
- **Eight tools are frozen.** A PR that adds a ninth needs a design discussion first — see the
  README's non-goals.
- **`main` is PR-only**: branch (`phase/N-<slug>`, `feat/<slug>`, `fix/<slug>`, `chore/<slug>`),
  push, open a PR. No direct pushes.

## Before you open a PR

```bash
make vet && make test
```

Read `AGENTS.md` for the working conventions (import direction, the frozen schemas, the docs
mirror, the gates) — it is short and it is enforced.

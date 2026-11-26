# Codemode redesign: workspace changesets — implementation plan (revision 2)

**Status:** Proposal for review. Supersedes revision 1 (`2026-11-26-codemode-workspace-changesets.md`,
kept as the review record). Based on moca `1985b853203d1d9d79c13291864ae95c19545a63`. No implementation,
default change or paid experiment is authorized by this plan.

**Goal, unchanged from rev 1:** one bounded, data-dependent change across a repository in a single
call — discovery, decisions and exact edits computed in the program, applied by Go after the program
succeeds, with an inspectable receipt and a real verification run. A one-file change stays an ordinary
`edit`.

**What revision 2 changes, in one paragraph.** It promotes the grant defect to a **P0 security fix that
ships before anything else** and proves it (a file grant admits its siblings, so a codemode script can
read `~/.config/moca/auth.json` today, in additive as well as transport). It replaces `stageEdit`'s
one-replacement-per-target API with an ordered edits array, because the forbidden-repeated-target rule
otherwise caps a file at one change. It moves edit validation to **stage time** so the program can
react, keeps apply-time revalidation for races, and aligns the read requirement with the tracker moca
already has instead of demanding a full page-walk. It adds the resume matrix a third surface needs. It
retires `transport` rather than maintaining three surfaces. It reuses the measurement that already
exists (`steps` *is* model requests) instead of inventing a parallel instrument, adds the power/FWER
caveat the rev-1 thresholds need, and reuses moca's real permission machinery (`Env.Paths`, `Env.Plan`,
`Env.Reads`, `Snapshotter.Guard`, `Env.ConfigFile`) with one aggregated approval in interactive modes
instead of a new consent story. It also lists the process gates both plans must pass (SPECS mirror,
count pins, `-race -count=8`, arm and corpus-model pins) and the Windows/ACP details staging cannot
duck.

## 1. What the evidence does and does not establish

Rev 1's evidence table is right, and its verdicts are kept. Two additions, both load-bearing for what
follows:

- **Carry is not the currency, and the v2 rows say so.** In the `sweep-bulk` campaign the sum of
  turn-1 carry was **3.0 % of the token volume** in `off` (42 870 of 1.41 M cache-read tokens) and
  2.4 % in `transport`, while cost per step was flat across arms ($0.00042–$0.00049) — the bill is
  `steps × prefix`, and steps were 158 / 140 / 161 for `off` / `additive-400` / `transport`. A perfect
  carry result therefore moved at most ~3 % of the bill. That is *why* the primary endpoint here is
  work requests, and any future reviewer must not quietly re-introduce a carry gate.
- **`steps` already counts model requests.** `test/evals/evalstats/main.go:75`: "Steps counts
  assistant turns (each one is a model request)". Rev 1 asked for a new request counter; what is
  actually missing is (a) retries and nudges inside a trial, (b) outer-call and host-operation counts.
  The `codemodecarry` build already numbers outbound requests per process (`carryReqNo`,
  `internal/agent/carry_hook.go`) — extend that hook rather than build a second instrument.

Kept from rev 1: no repeat of the additive prompt experiment, no transport promotion, the fixed pilot
oracle is not a capability figure, and shell stays available in every arm.

## 2. What OpenCode and Pi implement — and what moca must copy

Rev 1's correction stands: the earlier moca summary ("both expose every direct tool inside JS") was
wrong. Pi allows overlap between direct and script-callable tools and can hide declarations per tool
(`direct | codemode | deferred`, `only`); OpenCode makes catalogued paths callable inside `execute`
and leaves other tools direct. The portable lesson is **policy parity and host-owned receipts**, not
"dispatch everything".

Design consequences moca accepts (rev 1's column, kept):

| | Copy | Do not copy |
|---|---|---|
| Authority | staged edits go through the **same** jail, read guard, snapshot, plan-mode and config-file rules as a direct `edit` — or stricter | a second dispatcher that reaches tools the direct path cannot |
| Receipt | host status is authoritative; guest text cannot override it; paths and counts are structured | trusted guest assertions, unbounded captured output |
| State | explicit return; discard-on-failure; a new process never replays pending work | persistent JS state, branch-scoped stores |
| Execution | one VM per call, fresh | promises, workers, background writes, another VM |

## 3. Where moca is different (and what is broken today)

### 3.1 P0: a file grant admits its siblings — ship this fix first

`internal/tools/inspect_root.go:42-70` grants an absolute path by opening **its parent directory** and
recording the parent as the grant's path; `granted()` (line 73) then accepts any descendant. So a grant
on one file is a grant on every sibling. Proven with a throwaway test before this revision was written:

```
PROBE: sibling GRANTED and readable: auth.json -> 36 bytes: {"key":"sk-mus...host"}
```

The blast radius is real, not theoretical:

- `Jail.ReadRoots()` includes **the user config file**. `config.ConfigFile()` is
  `~/.config/moca/config.jsonc`; `config.AuthFile()` is `~/.config/moca/auth.json`, which stores OAuth
  tokens and plaintext API keys (`storeFile{Providers, APIKeys}`).
- The grant is applied **whenever `Attempts > 0`**, not only in transport
  (`CodemodeHolder.inspection`, `internal/tools/codemode.go:73-79`), and the default is now 400 for
  both surfaces. So a session with `codemode.enabled: true` — the surface a user is one key away from —
  can read the credential store from a script, and the script's output goes into the model's context
  and therefore into the transcript.
- `read` with an absolute path is the only reachable route (`ls`/`search` have no grant path), so this
  is a targeted read of a documented path, not a general traversal. That is still credential
  exfiltration.

**Fix (independent of this plan, no pilot, no approval gate beyond the normal code review):**

1. Grant **kind**, not a widened path: `ReadRoots()` returns entries that say whether the target is a
   file or a directory (`{Path string, File bool}` or a typed `ReadGrant`). `openWorkspace` keeps a
   file grant as `{dir: parent, name: base}`; `granted()` requires **exact equality** for file grants
   and a segment prefix for directory grants. Do not infer the kind from `os.Stat` at open time — the
   file may be absent, and an absent-then-created path must not become a directory grant.
2. Keep the `*os.Root` on the parent (it inherits symlink refusal) and match the exact name; do not
   open the file itself, which would freeze an inode.
3. Tests: a file grant reads itself; a sibling is `outside`; a sibling created *after* start is
   `outside`; a symlink whose target is the sibling is `outside`; a directory grant still admits
   descendants; a grant that does not exist at start is not retried into existence; and a
   **credential-shaped fixture** (a sibling named `auth.json` next to the granted `config.jsonc`) must
   be refused, with the refusal asserting the class (`outside`), not a message.
4. Security note for the release: until this ships, `codemode` with `enabled: true` can read the
   config directory's siblings. If a release goes out before the fix, say so plainly in the notes.

### 3.2 Advertised capabilities must equal the table

`codemodeSpec(network=true)` advertises `tools.web.fetch` / `tools.web.search`
(`internal/tools/codemode.go:178-187`) while `workspaceCapabilities()` has no web row and dispatch
refuses it. Rev 1 says generate the description from the live table; agree, and add: the description
is a **prompt-cache reset for every codemode user**, so batch it with any other description change
pending (AGENTS's rule for the eight frozen schemas applies to this one by the same argument).

### 3.3 Staged writes must reuse the permission machinery that already exists

moca already has the whole policy surface; the changeset must not grow a second one.

| Concern | Existing mechanism | What staging does |
|---|---|---|
| May this path be written? | `Env.Paths.Resolve(path, true)`; refusals offer `askOutsideWrite` only for ask-write roots and the config file | same call; a target that would ask is **refused** in `-p` and in plan mode |
| Plan mode | `Env.Plan` + `planAllows` (docs/plans only) | staging is refused outright, even for a plan file — a multi-file apply is not a plan-file edit; the model uses direct `edit` |
| Read guard | `Env.Reads.Check(abs)` (mtime, size, digest) | stage time **and** apply time |
| Snapshot | `Snapshotter.Guard(abs, write)` — one file | per target, before the commit; `/undo` therefore stays per-file and a partial apply leaves a stack a user unwinds N times |
| Config file | `Env.ConfigFile`, already special-cased so an approval cannot widen policy | staging refuses it before the read guard, like `edit` does |

**Consent.** Inside the workspace, direct `edit`/`write` **never ask** — the permission mode and the
jail are the consent. Staging grants no new authority: the same model could call `edit` N times. The
one genuine difference is **visibility** — one `codemode` call that silently rewrites 32 files is less
legible than 32 tool calls. Mitigation, in order of importance: the receipt (§4), the TUI expansion,
and **one aggregated approval before apply** in `default` mode (a single `Question{Kind: "changeset"}`
naming the paths and counts; skipped in `yolo`, refused in `-p`, and the codemodegate's
`TestZeroApprovalsInEveryMode` gains an explicit exception for a staged apply in `default`). Rev 1's
"document the permission and hope" is weaker than the machinery already in the tree.

## 4. The contract

**Exposure.** `codemode.exposure: "workspace"` (third value) = the additive declaration surface (eight
direct tools plus `codemode` ninth) with workspace-only authority and, under
`codemode.workspaceEdits: true`, a `stageEdit` primitive. Keep `additive` and `transport` as they are;
**stop documenting `transport` and plan its removal** — it has no measured benefit and workspace
subsumes its read story without the declaration surgery. Two surfaces to maintain, not three.

| Session | Provider surface | Script authority |
|---|---|---|
| off | eight | none |
| additive | eight + `codemode` | `read/search/ls/glob`, transport-style parity; absolute reads only inside typed grants (after §3.1) |
| transport (legacy, undocumented) | six | as today |
| `workspace` | eight + `codemode` | `read/search/ls/glob`, **workspace-relative only**, no grants |
| `workspace` + `workspaceEdits` | same names | plus `stageEdit`, applied by the host after a successful program |

**The primitive.**

```js
tools.stageEdit({
  path: "services/billing-api/deploy.yaml",
  edits: [                                   // ordered, applied to the staged content in order
    { old: "image: billing-api:1.4.2", new: "image: billing-api:1.4.3" },
    { old: "digest: sha256:aaaa",      new: "digest: sha256:bbbb" },
  ],
})        // -> {staged: true, path, edits: 2}
```

- Ordered array, not one pair: with repeated targets forbidden, a single pair caps a file at one
  change, and real manifests need two (image and digest in the example task). Each `old` must match
  **exactly once** in the content as staged so far; `new` may not be empty-equal (reject a no-op); no
  `replace_all`, no fuzzy matching, no implicit creation.
- **Validated at stage time**: path admissible, file is an existing regular text file, `Env.Reads.Check`
  passes, every `old` matches exactly once. A refusal is a typed `CallError` the script may catch and
  react to — otherwise the model discovers a bad `old` string only after the program ends, which is
  exactly the round trip this feature exists to remove.
- `{staged: true}` means **staged**, never written. Reads see the original file throughout; there is
  no overlay. Apply-time revalidation repeats the same checks (path, mode, size, digest) so a change
  between stage and apply is refused, not fudged.

**Read requirement, aligned with the tracker.** Rev 1 demanded "a complete same-execution read of each
target, including all pages without coverage loss". That is stricter than the direct `edit` rule (a
read record with a matching mtime/size/digest) and it would cost a page-walk per target — eight read
calls for a 3 000-line file — which is precisely the cost the feature is trying to avoid. Require: a
same-execution read that yields the **full-file stamp** (already computed from the descriptor in
transport; enable the same capture in workspace mode) plus `Env.Reads.Check` at stage and apply. The
byte-exact, exactly-once `old` match is the real safety property; the stamp proves the file did not
move underneath.

**Path policy.** Workspace-relative, normalized, no `..`, no absolute paths, no symlinked components,
existing regular files only, not hidden (any dot-prefixed component), not `.git`/`.moca`, not repo
instruction files (`AGENTS.md`, `CLAUDE.md`), not skills/prompts, not `config.jsonc`/`auth.json`, no
NUL byte in the first 8 KiB, valid UTF-8. One predicate, one place, tested by table. No creation, no
deletion, no rename, no mode change. Untouched bytes, line endings, BOM and mode preserved.

**Budgets.** Reuse `inspectAttempts` as the single host-call budget (staging charges it too). Added:
32 distinct targets per call, 512 KiB per target, 2 MiB of retained pre+post images, 32 KiB of edit
text per target. Refuse before allocating over budget. Exhaustion or any invalid stage latches the
changeset invalid even if JS catches the error.

**Apply.** After `eng.Execute` returns and admission is still live: validate all targets, then apply
serially in canonical relative-path order, rechecking admission, path, mode and digest at each write,
one `Snapshotter.Guard` per file, same-directory replacement, permissions preserved. A preflight
failure ⇒ zero changes. A late failure ⇒ `partial` with applied / failed / not-attempted paths; never
auto-rollback over user edits. A write that lands before snapshot persistence or the read-tracker
refresh fails ⇒ `outcome_unknown` for that file. `/undo` keeps its per-file semantics: N staged writes
are N undo steps in reverse order, and the receipt says how many remain.

**Result.** One bounded tool result: `status` (`inspected | applied | discarded | partial |
outcome_unknown`), counts, per-path outcomes, coverage/truncation flags, guest return labelled
separately, and reserved space for `status` even when output overflows. The TUI expands one call into
the file list and diffs; ACP exposes the same under the parent call (one approval per `codemode` call,
already the ACP model); headless stdout stays the final answer with diagnostics on the existing
channel. Host status is authoritative; a guest string saying `applied` cannot create a receipt.

**Resume and revocation.** The header records the surface (`additive | transport | workspace`) and
whether edits were declared. Resume requires the **surface to match** when either side is `workspace`
(same rule as transport today, extended): workspace→additive/transport/off refuses with both remedies;
additive→workspace refuses; additive→off stays allowed (the refusal-tool contract its prompt
promised). If the stored session had `workspaceEdits` and the current config does not, the session
resumes **read-only** with a typed refusal for `stageEdit` and the direct tools as the alternative —
capability is the intersection of stored and live, and enabling later never widens an old session.

## 5. Implementation sequence

Each phase lands with its tests, its SPECS change and its counts; no phase claims a benefit.

0. **P0 grant fix (§3.1)** and the advertised-vs-implemented description (§3.2). Independent of
   everything below; `go test -race ./internal/tools ./internal/permissions ./internal/agent` plus the
   binary gate. Ship first, on its own.
1. **Grader before feature.** `test/evals/changeset-pins/` (manifest join) and
   `test/evals/changeset-registry/` (registry-referenced implementations only). Deterministic seeds;
   expectations computed from fixture data outside the worktree; grade **final bytes first, tree
   second**, each mutation failing for its own named reason (rev 1's list: global replace, missing
   target, lost comment, extra file, verification before the last edit, plausible prose with no
   patch). `bugfix` and `reuse` stay as ordinary controls. Extend `test/evals/_lib/trial.sh` with a
   retry-inclusive request count (from the carry hook's counter) and host-operation counts; keep
   `steps` as the primary. `bash test/evals/run.sh --selftest` green before anything billable.
2. **Surface plumbing, read-only.** `internal/config/config.go` (`ResolveCodemode`, validation, layer
   rules: user opts in, trusted project may veto, project never widens), `internal/agent/start.go`
   (`planCodemode`, holder), `internal/agent/resume.go`, `internal/agent/prompt.go`,
   `internal/session/entry.go`, `internal/config/sample.jsonc`. Tests: strict decode rejects unknown
   keys; workspace-without-enabled and workspaceEdits-without-workspace are config errors;
   `network: true` with workspace is refused; project veto; legacy headers; the resume matrix of §4;
   provider wire names/order and off-prompt bytes unchanged; no VM opened when codemode is never
   called. `-race -count=8` on config/session/agent.
3. **Stage and apply as two host phases.** New `internal/tools/changeset.go` (+`_test.go`): the
   changeset owns targets, coverage, budgets and the staged images; it dies with the call. `bridge`
   gains `stageEdit`; `codemodeTool.Run` applies only after `eng.Execute` succeeds and admission is
   live. Extract the shared commit from `edit.go`/`editapply.go`/`write.go`/`readtracker.go` **without**
   calling `Registry.Run` recursively (it resets per-call state and emits unrelated hints). Refactor
   the guarded writer so the commit is root-relative and cannot follow a changed ancestor. Journal
   `Begin/Record/Finish` through a narrow `tools.Env` interface bound in `agent.build`; the call ID
   comes from the registry invocation, never from a script argument; no journal (including an
   unpersisted session) ⇒ refuse edits before apply, reads still work. `internal/tools` stays free of
   `session` imports.
4. **Make it visible.** `tools.Result`, `internal/agent/events.go`, `internal/tui/toolsummary.go`,
   `internal/acp/toolcall.go`; replay reads metadata, never executes. The aggregated `default`-mode
   approval of §3.3, plus the gate's approval exception.
5. **Failure behaviour, in the real gate.** `test/codemodegate`: a real script over generated rows
   applies the expected bytes; one provider tool result; no intermediate sentinel in the next request;
   a direct green verification **after** the edits; a second process replays without editing; a killed
   child between two writes leaves a journal entry that the next process reports as interrupted and
   performs zero writes. Unit-level: stale data on the last preflight, late failure on target two,
   journal append failure, steering before and during apply, duplicate/aliased targets (case-folded on
   Windows), BOM/CRLF, caps, plan and `-p` denials, `outcome_unknown`. Guest-forged success cannot
   produce a receipt. Windows: replacement semantics of `os.Rename` on an existing destination get
   their own test on the native CI matrix.
6. **Feasibility before any model spend.** A handwritten reference program through the real binary and
   VM, per fixture, under production caps: correct bytes, no privileged path, one ordinary
   verification. Then the same task as a plain host-side list of explicit `{path, old, new}` edits
   through the same commit routine, and as ordinary direct tools with shell allowed. If the list or
   shell already meets the same request floor, the feature has no case — stop before billing.
7. **One separately approved campaign** (§6). If it passes: README/CHANGELOG/SPECS/AGENTS, the real
   task and diff documented, rollback = disabling the grant for future executions. If it fails: keep
   the evidence and phase 0/1, ship no capability.

## 6. The campaign

**Question:** does computing *and applying* a bounded change save model requests relative to (A) plain
moca and (B) the same inspections without edits?

- **Arms**, one tagged binary: A = codemode off; B = `workspace`, reads only; C = B +
  `workspaceEdits`. Direct tools and shell available in all three; no hints to use `codemode`. Freeze
  binary digest, configs, checker, prompts, seeds, model (`opencode-go/deepseek-v4.1-flash`), effort,
  permission rules and caps before the first trial.
- **Assignment:** ten matched triples per fixture, two target fixtures, two ordinary controls,
  randomised balanced arm order within blocks, `-j 1`, fresh trees, isolated XDG dirs, one
  `-cap-scope` ledger for all arms. Every assigned run counts, including shell-only and failed ones.
- **Primary:** model requests to a checker-valid result (a trial that never gets there scores the
  predeclared limit + 1). On **each** target fixture, C must beat A **and** B in ≥ 8/10 paired runs
  with a median paired ratio ≤ 0.75 against each; ties are not wins. State the multiplicity plainly:
  four comparisons at p ≈ 0.055 each are a family-wise ~20 % chance of a spurious pass, so this is a
  **decision rule** — a 7/10 is "no promotion, recorded", not "almost".
- **Vetoes:** C passes ≥ 9/10 on each target; no lost pass against either comparator; **no
  unauthorized or unreported change** (stop the campaign); C's ordinary-control cost within A + 1
  request; C's median request ratio vs A ≤ 1.10 on elapsed time and cost.
- **Ceiling:** pre-reserve a verified worst-case trial cost or use a provider-enforced limit — a guard
  that only sums *completed* spend is not a hard cap (the `-cap-scope` guard is the former).
- **Verdict:** all conditions ⇒ opt-in `workspaceEdits` for the measured class, not a default. Missing
  telemetry, incomplete blocks, a broken oracle or an exhausted ceiling ⇒ inconclusive, also no
  promotion. Archive protocol, rows, paired ratios and representative traces in
  `test/evals/BASELINE.md`; no post-hoc metric swap, no paid rerun on the same tasks.

## 7. What not to build on this evidence

| Temptation | Why not now |
|---|---|
| Hide direct reads, or one script per turn | Both investigated; neither reduced steps (v1, v2). Keep the fast path. |
| Wrap shell, MCP or HTTP inside JS | Different permission and egress problems; MCP already has a one-schema adapter. Reopen read-only MCP composition only with a cross-server join that direct proxy calls cannot match. |
| File creation, deletion, rename, or a writable overlay | Every one widens the review surface (links, modes, races) for a case the pilot does not measure. Reopen with a separate plan and fixture. |
| Persistent JS state, a worker pool, TypeScript, Bun | Nothing measured so far needs them; state especially must not survive a process. |
| A carry-based gate | Carry was 3 % of tokens in the last campaign; the bill is requests. |
| Promote on the handwritten reference | Feasibility is not adoption. Only uncoached runs with ordinary tools available can justify a user-facing claim. |

## Appendix A — review findings for revision 1

| # | Finding | Severity | Where it landed |
|---|---|---|---|
| F1 | Grant defect ("overbroad grants, loses the exact-file restriction") is described as a boundary repair; it is a **credential read** (`auth.json` beside `config.jsonc`) reachable in additive too, because grants apply whenever `Attempts > 0`. | **P0** | §3.1, with proof, mechanism, test list and a release note. |
| F2 | "Fix both before widening any capability" leaves the fix gated behind the feature. | High | §5 phase 0 ships it alone, first. |
| F3 | `stageEdit({path, old, new})` + "reject repeated targets" caps a file at one replacement. | High | §4: ordered `edits[]` per target, each matching once against staged content. |
| F4 | Ambiguous whether `old`-string validation happens at stage or apply; if apply-only, every mismatch costs the round trip the feature exists to save. | High | §4: stage-time validation, typed catchable refusal, apply-time revalidation for races. |
| F5 | "Complete same-execution read, all pages" is stricter than the direct `edit` rule and costs a page-walk per target. | Medium | §4: full-file stamp + `Env.Reads.Check`, rationale stated. |
| F6 | Third surface with no resume matrix (only "preserve transport diagnostics"). | High | §4 resume table, including `workspaceEdits` revocation and the additive→off carve-out. |
| F7 | Three surfaces to maintain; `transport` has no measured benefit and is redundant once `workspace` exists. | Medium | §4: stop documenting transport, plan its removal. |
| F8 | New request counter requested although `steps` already is model requests and the carry hook already numbers them. | Medium | §1: reuse and extend, don't duplicate. |
| F9 | Four comparisons at 8/10 each are ~20 % FWER; rev 1 says "practical thresholds" but not the arithmetic. | Medium | §6: multiplicity stated, 7/10 defined as no-promotion. |
| F10 | Consent: "document that a script can change several files without a dialog; if not approved, stop" — moca already has `Env.Paths`/`Snapshotter`/`Plan`/`ConfigFile` and a real `Asker`; the difference is visibility, not authority. | Medium | §3.3: reuse the machinery, one aggregated approval in `default`, gate exception named. |
| F11 | Budget list has no per-file cap, so a 5 MiB target fails only at apply/preflight. | Low | §4: 512 KiB per target, 32 KiB of edit text. |
| F12 | Windows: case-insensitive duplicate detection and `os.Rename`-over-existing need explicit tests. | Medium | §4/§5 phase 5. |
| F13 | ACP approval granularity unstated. | Low | §4: one approval per `codemode` call, changes under the parent. |
| F14 | No mention of SPECS's skill mirror, test-count pins, doc counts, `-race -count=8`, `pilot_arms_test.go` or `TestEvalConfigsPinTheCorpusModel`. | Medium | §5 phases and the "each phase lands with…" sentence; AGENTS/SPECS pins are named there. |
| F15 | "Do not compare against the obsolete 50 default to manufacture a win" — agreed, and the arm configs must pin their budgets explicitly (they now do: `cm-additive` pins 50). | Low | §6 freeze list. |
| F16 | "`/undo` retains its per-file semantics" without saying what N staged writes mean for the stack. | Low | §4: N undo steps in reverse order, receipt says how many remain. |
| F17 | Phase 1's grader does not say to grade bytes before the tree, the bug the v2 campaign hit. | Medium | §5 phase 1. |
| F18 | The plan never states what `workspace` does with grants (it silently drops them). | Low | §4 table: workspace-relative only, no grants; §3.1 still fixes the other surfaces. |
| F19 | No statement of what happens to the read tracker after a staged apply (a follow-up direct edit must re-read). | Low | §4: tracker records the post-apply state; a later direct edit needs a fresh read. |
| F20 | "No file creation" is right for this pilot, but the plan should say the fixture must not require it and that users will ask. | Low | §7, creation row. |

Rejected suggestions: none were removed. Rev 1's core judgement — keep the engine, add one narrow
primitive, keep direct tools, refuse a compulsory transport — is the right shape and is kept.

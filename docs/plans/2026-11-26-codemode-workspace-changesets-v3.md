# Codemode redesign: workspace changesets — implementation plan (revision 3)

**Status: changes requested on revision 2; proposal, not approval.** Supersedes [revision 2](2026-11-26-codemode-workspace-changesets-v2.md); [revision 1](2026-11-26-codemode-workspace-changesets.md) retains the upstream source comparison. Reviewed at `cd3302055aec27b9fe79350a62052dc5c33f3d96`; only the two plans changed from the implementation baseline `1985b853203d1d9d79c13291864ae95c19545a63`. No implementation, release, default change or spend is authorized here.

**Goal, unchanged from rev 1:** one bounded, data-dependent change across a repository in a single
call — discovery, decisions and exact edits computed in the program, applied by Go after the program
succeeds, with an inspectable receipt and a real verification run. A one-file change stays an ordinary
`edit`.

## Review of revision 2

| ID | Finding in v2 (line numbers refer to the reviewed file) | Disposition in this revision |
|---|---|---|
| R3-1 · blocking | §3.1 (92–103): exact-name matching on a parent `os.Root` is not a no-follow open. The budget path also supplies grants to additive, contrary to SPECS §10.6. | Separate grant scope from budgets; enforce file identity at open, and fail closed where unsupported. Phase 0 is independent. |
| R3-2 · blocking | 129–134, 245–255, 278–284: the campaign uses an asker-less `-p` driver while staged writes need consent; approval would also land a phase **after** writes. `auto` is unspecified. | One-shot, diff-bound consent before *any* write, same-phase enforcement, real ACP campaign driver for all arms. Keep headless denial. |
| R3-3 · high | 138–149, 211–217: additive suddenly acquires read parity, and transport is described as both retired and maintained. A project must not select a different surface to acquire privileges. | Keep the old contracts and document legacy transport. Explicit start/resume table, independent grant veto and trust check. |
| R3-4 · high | 164–194: catchable staging errors and a permanently poisoned batch cannot both support correction. A session read stamp alone is not a same-script read; stale/absent files and uninvestigated failing tests need explicit treatment. | Keep fatal latch; separately specify correctable exact-match errors with outstanding-target latch. Phase 3 enforces stamps, investigation and capacity before and after consent. |
| R3-5 · high | 196–209, 247–263: journal contents, pairing on resume, failure after a write, undo and proof of the crash window are missing or overclaimed. | Journal and recovery contract in §4; apply only with a durable start; deterministic child-process barrier and recovery tests. |
| R3-6 · high | 32–42, 230–233, 285–294: first-turn carry isn't an upper bound on cost. Assistant messages and the carry hook are not retry attempts. A four-way **AND** does not have an any-of-four false-pass probability. | Separate counters, use a fixed logical-work endpoint, restore adoption, exact caps, denominators, units and the stop rule (§§1, 5–6). |
| R3-7 · medium | F14/F15/F19 in Appendix A are not operative instructions; the stated post-write tracker/re-read behavior contradicts itself. The cheaper native-list counterfactual also assumes the target list for free. | Name the actual gates, pin budgets, track the written version, and measure how each route obtains its inputs. |

**Decisions to approve separately:** the surface/consent matrix below (not the more permissive headless policy in v1); the partial-read plus exact-match contract; and, only after the unbilled gates, an actual campaign budget. The P0 repair does not depend on approving changesets.

## 1. What the evidence does and does not establish

The historical transport comparison in `test/evals/BASELINE.md` missed both predeclared bars: 7/10 pairs carried less, median paired ratio 0.778 (required ≥8/10 and ≤0.5). Raising the additive attempt budget from 50 to 400 helped on that workload. Neither finding establishes a changeset benefit.

- The local `v2pilot-off` rows sum to 42,870 estimated turn-1 carry and 1,406,720 cache-read tokens. These measure **different things**: one observation after continuation versus cumulative provider usage. The committed summary even records a different off carry sum (47,882). Keep the published verdict; do not silently substitute aggregates or infer a maximum dollar saving. Replayed prefixes, output generation and the provider's cache rates matter. Use measured time and cost as guardrails.
- `test/evals/evalstats/main.go:272–287` increments `Steps` for assistant messages, `Nudges` for marked entries and `Calls` for outer tool uses. Reuse them as diagnostics. `internal/agent/agent.go:519–520` observes logical work before `adapter.Stream`; `internal/provider/retry.go:82–88` can make several adapter attempts inside it. An unanswered request is not a persisted assistant turn. Do not count physical retries with the carry hook. Phase 1 defines the decision metric and calibrates all three levels.

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

### 3.1 P0: narrow the grant at the actual file open; ship independently

Source inspection confirms v2's underlying finding: `internal/agent/start.go:537–540` passes jail read roots to the holder; `internal/tools/codemode.go:73–77` passes them to inspection when the budget is positive, even for additive. `internal/tools/inspect_root.go:42–85` represents a file grant by its parent directory. An intended exact-file grant can therefore expose unrelated, potentially sensitive data. This is a **potential disclosure**, not evidence that a user's data was read or transmitted. No live credential file or exploit reproduction is needed for this review.

1. At construction separate `attemptBudget`, `captureStamp` and `externalGrants`. Off/additive/workspace get **no** external roots; additive keeps its positive budget and no stamp capture. Transport alone gets typed file/directory grants from the jail's configured purpose (not inferred by `Stat`). Adapt the types in `agent`, keeping `permissions` and `tools` independent. Do not remove the direct tool's legitimate grants.
2. Directory grants stay confined to their own open root. An exact-file grant binds a trusted parent handle plus exactly one basename; each read must open that entry without following a link/reparse point and verify the opened descriptor is regular. `go doc os.Root` explicitly permits in-root symlinks. Lexical equality, or `Lstat` followed by ordinary `Root.Open`, is not sufficient. Use a reviewed handle-relative no-follow primitive on each supported OS; if a safe implementation is unavailable, refuse that grant and report limited functionality rather than using a normal open. Establish trusted parent identity before granting. A missing exact file grants nothing else; it may become readable later **only at that exact entry**. An unavailable parent grants nothing for that inspection.
3. Pin actual registration through the binary, not just a manually constructed `Inspection`: positive budgets on both legacy surfaces, positive exact-file/directory reads in transport, workspace reads in additive, and denied non-granted targets in both. Use harmless synthetic files and controlled link/type-change fixtures. Test missing then created, directory substitution, ordinary safe atomic replacement of the exact file, closing/reopening and native Windows handling. Assert stable refusal classes and absence of returned content. If OS-specific enforcement is not verified, fail closed there.
4. Prioritize a standalone fix and appropriate release note through normal owner review. Until the fix is available, leaving codemode disabled is the conservative mitigation. Product experimentation and an unrelated description edit must not delay it.

### 3.2 Advertised capabilities must equal the table

`codemodeSpec(true)` advertises web operations absent from `workspaceCapabilities()`. Fix the fresh-session description and prelude from one capability inventory; an existing header keeps its promised shape with a typed refusal, never newly authorized network access. Test workspace and network variants and their adapter goldens; leave the eight built-in schemas alone. This can follow the grant fix independently.

### 3.3 Staged writes must reuse the permission machinery that already exists

moca already has the whole policy surface; the changeset must not grow a second one.

| Concern | Existing mechanism | What staging does |
|---|---|---|
| Path | `Env.Paths.Resolve(path,true)` is necessary, not sufficient (`agent/mode.go` swaps in a broader checker for auto/yolo) | additionally enforce a root-relative workspace-only write in every mode; never call `askOutsideWrite` for a nested target |
| Plan / trust | `Env.Plan`, start options and resolved user/project policy | no staged mutation in plan or untrusted workspaces, including headless and yolo; a saved declaration is not a grant |
| Read / investigation | `Env.Reads.Check` allows a missing file for ordinary creates; `edit.go` also checks `investigationRefusal` | require an existing same-script stamped object and completed failing-test investigation; no create on disappearance |
| Snapshot / tracking | `guardedWrite`, `Snapshotter.Guard`, `noteChange` | reuse policy and state updates, not pathname-based I/O; report durable snapshots separately from bytes changed |
| Protected paths | direct config writes have an exceptional one-shot ask; `Env.ConfigFile` identifies that target | staging is stricter: deny config, instructions and protected targets outright; approval cannot override |

**Consent decision (proposed).** Reads never ask. In a trusted, edit-enabled workspace, nonempty staged changes in interactive `ask` (canonical name; `default` is an alias) **and `auto`** require exactly one host-generated, allow-once question. Interactive `yolo` skips the question but none of the path, stamp, journal or investigation guards. `-p` refuses changesets in **all** modes, even `--mode yolo`; read-only calls and ordinary direct tools retain their existing behavior. Use independent `Headless` and effective-mode data from `agent` rather than interpreting `env.Ask != nil` as a front-end or authority test. ACP supplies an asker and is not the `-p` path.

After guest success and whole-set preflight, freeze ordered paths and pre/post digests. Show counts, normalized paths and the exact diff from those bytes; provide the full bounded preview before consent, not just a guest description or a clipped list. Use `Question{Kind:"changeset", CanAlways:false, NoSession:true}`; TUI/ACP offer once/deny only. Bind the answer to that immutable set and parent call ID. Denial, disconnect, timeout, pending steering or a changed target while the dialogue is open invalidates the batch. Revalidate the **whole set after approval**, before journal begin; never silently recompute the approved edit. Test two successive batches ask twice. An empty set asks zero times.

## 4. The contract

**Exposure.** `workspace` is opt-in and distinct from the existing additive and transport contracts. It retains eight direct tools plus the adapter. No default change, silent alias or removal of transport; continue to document how to resume it. Any deprecation/removal is a separate compatibility decision with owner approval and a migration for old headers.

| Session | Provider surface | Script authority |
|---|---|---|
| off | eight | none |
| additive | eight + `codemode` | workspace `read/search/ls/glob`, no external grants and **no** session read parity (existing specified behavior; P0 closes the unintended grant) |
| transport (supported legacy opt-in) | six | read parity and typed, scoped transport grants; direct hidden names still refuse |
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

- One ordered edits array per target; reject duplicate successful stages for the same filesystem object. A later pair sees the bytes produced by earlier pairs. Require a nonempty array, nonempty `old` with one exact byte match, `old != new`, and a final image different from the original. An empty `new` may remove a substring, not the file. No fuzzy edit, prefix stripping, `replace_all` or implicit creation. Reads during staging still see original files.
- Validate on the host at stage time. A missing/ambiguous match is **correctable**: commit no staging state for that target, mark it unresolved and return a typed mismatch. Only a successful restage of that same target clears it. Returning with any unresolved target discards the whole batch. All other invalid stages, authorization failures, aliasing, budget exhaustion and cancellation are **fatal** for this changeset, even if caught. Test corrected, uncorrected and fatal-but-caught separately. `{staged:true}` is not an assertion that any bytes landed.
- Read eligibility is a proposed relaxation from revision 1, not a fact about user consent. Require a successful read in this script with a captured full-file stamp from the opened descriptor (`Inspection.StampFor`); a prior session read, listing, search result or guest-supplied digest is insufficient. A partial display is OK, not a claim to have inspected the whole file or tree. Compare the actual preimage bytes, size, identity and modification time at stage, preflight and commit. Bound hashing/reads while collecting bytes. Session `ReadTracker.Check` supplements this, never authorizes a missing target. Workspace reads/searches maintain transport-like investigation bookkeeping. Do not use additive's legacy no-parity switch to gate workspace tracking.

**Path and content policy.** Workspace-relative existing ordinary files only. Reject absolute, `..`, dot-prefixed components, noncanonical spellings, linked/reparse components, multiple hard links and equivalent targets by file identity. Respect real filesystem case/alias semantics, including Windows reserved names and alternate streams. Reject repo instructions, `.git`, `.moca`, skill/prompt/config/audit files and known credential/key filenames (at minimum `auth.json`, `credentials.json`, `.env*`, `*.key`, `*.pem`) independent of case. This is not secret detection in arbitrary files. Validate the entire bounded pre/post image as UTF-8 text without NUL; preserve untouched bytes, BOM, CRLF and supported metadata. Skip unsupported special modes/ACL-preservation cases with a typed refusal, never weaken permissions in order to replace a file.

**Budgets.** Existing inspect/source/entry/VM/ABI limits still apply. Every attempt, including failed staging, charges the script's attempt budget. Additionally enforce ≤32 targets, ≤512 KiB per file **before and after**, ≤32 KiB decoded edit text per target, ≤256 KiB total accepted edit text, ≤2 MiB combined retained pre/post images. Check growth before each replacement and bound intermediate buffers, diff rendering and result encoding. Charge before buffering, including revalidation reads. Test exact-fit, first over-limit, multi-byte text and expansion in a middle edit. No quota on how many outer scripts the model may choose.

**Apply.** The host alone transitions `staging → preflight → approval (if required) → revalidate → begin journal → apply serially → finish`. All phases honor context cancellation and steering. The guest execution deadline bounds computation; a separately cancellable host wait for human consent must not be a detached job. Use a 30 s active host-work budget for preflight/revalidation/apply excluding the approval wait; if it expires start no further file. Pin target directory handles and perform no-follow, root-relative metadata checks, temporary-file creation and same-directory replacement on the verified parent. Include snapshot pre/post capture in that boundary: `Snapshots.Guard` currently reopens absolute paths before/after its callback (`internal/session/snapshot.go:63–93`); changing only the callback is insufficient. Provide the snapshot layer verified images or validated handles and record expected postimage, not an unrelated reopened path. Refuse unsupported filesystems. Recheck admission, identity, content and mode immediately before each replacement; this is **not** filesystem-wide isolation, compare-and-swap against arbitrary external processes, or a cross-file transaction. Do not claim those guarantees.

Before any write, errors discard the entire set. After one or more writes, a known failure stops the suffix and reports `partial`; a possibly landed write with incomplete snapshot/tracker/journal outcome is `outcome_unknown`. No automatic retries or rollback. Preserve `Env.Edited`, `Env.Unverified` and investigation/verification behavior on actual changes; an uncertain write still requires inspection and verification. On confirmed success record the written version, as direct `guardedWrite` does: a later direct edit may proceed if still current; a later changeset needs its own read. Do not stamp a subsequently observed foreign version as ours. A snapshot is **not** a guaranteed restoration: clean git snapshots give restore guidance, dirty files have guarded blob undo, later edits can make undo refuse. Receipt lists confirmed snapshot records; no promise of N usable undos after failure.

**Crash record and pairing.** Inject a `Begin/Record/Finish` journal through `tools.Env`; production binding belongs to `agent.build`. Identify a batch by session + outer transcript invocation, not JS input. Before mutation persist the ordered relative paths, pre/post digests and call identity. A start-record failure means zero writes. Record each result and finish; on append failure stop immediately. On resume under the session lock, consult journal state **before** `session.Repair` supplies generic orphan results (`internal/agent/resume.go:102–117`). For a finished apply with missing outer result, reconstruct the bounded host receipt once; for an unfinished start, explicitly report interrupted/unknown paths, using safe read-only comparison to recorded digests when permitted. A digest match is evidence of observed state, not authorship. No change replay or automatic undo, including on a second resume. Derive per-path states from persisted evidence; never synthesize a successful tool result from guest output. The zero-write claim is about **workspace files**: normal transcript repair may append. Scope durability claims to process crash unless explicitly tested for power loss.

**Result.** One bounded provider tool result with host status (`inspected | applied | discarded | partial | outcome_unknown`), per-path outcomes, counts, coverage, truncation and separately labeled guest return. Expose typed host metadata out of band to session/TUI/ACP/evals, including inspection-only calls; reserve bytes for host status and counts before guest text. Keep full bounded diffs in the user-facing preview/detail, not in every provider request or application logs. Replays never execute metadata. `partial`, `discarded` due to failure, and `outcome_unknown` are not successes. Headless stdout remains the final answer. Preserve call/result pairing on cancellation, and do not dispatch later outer mutations in the same batch after observed steering; pair skipped calls explicitly.

**Authority and resume.** Effective write authority = originally declared in a workspace header ∩ current user opt-in ∩ trusted workspace ∩ trusted project's non-veto ∩ editing mode ∩ supported front end. Only the user selects workspace; a trusted project may explicitly turn the feature/edits off or lower caps, never switch a workspace session to a more privileged profile. Keep the current additive/transport precedence for those legacy settings. Reject inconsistent user combinations before opening a VM. Persist a workspace ABI and the original edit-declaration bit, not approval. Fresh sessions use current policy. For resume (rows = originally recorded, columns = effective current surface):

| Stored / current | off | additive | transport | workspace |
|---|---|---|---|---|
| off (no adapter in header) | off | off | off | off — never add it retroactively |
| additive (including old enabled headers without exposure) | refusal adapter | additive | mismatch | mismatch |
| transport | mismatch | mismatch | transport | mismatch |
| workspace | mismatch | mismatch | mismatch | workspace; no new edit grant |

Mismatch ⇒ no provider call, tell user how to restore the surface or start fresh. Same workspace surface with revoked edits ⇒ keep the declared method's shape but return typed refusal; previously read-only stays read-only after opt-in. If trust is withdrawn, deny changes regardless of a surviving user opt-in. Test all cells plus edit-bit changes; keep old headers and wire declarations readable. P0 narrowing is intentional on legacy resumes too.

## 5. Implementation sequence

Nonexperimental fixes and measurement code may ship independently after approval. Phases 2–5 stay on an opt-in candidate branch until the release decision; do not ship an intermediate build that can mutate without the release guards. Each behavior PR must update `docs/specs/SPECS.md` and its byte mirror `internal/skills/builtin/self-knowledge/references/specs.md` together. Update AGENTS Go-file and SPECS package/test counts with their owning changes, and run `go test -tags counts ./cmd/moca -run TestSpecsTestCount -v` after changing test counts.

0. **P0 first (§3.1).** Test the real holder with positive attempts in both modes plus typed-grant tests. Run `go test -race ./internal/tools ./internal/permissions ./internal/agent` and `make build && bash test/codemodegate/run.sh`; native-platform results required for the grant implementation. Separate the fresh-catalog description repair (§3.2) so cache/legacy work cannot block containment.
1. **Grader and instrument first.** Build `test/evals/changeset-pins/` (manifest join) and `test/evals/changeset-registry/` (registered implementations), with deterministic variants. Both must be solvable without creating files. Grade exact bytes, untouched decoys, requested successful verification **after the last mutation**, and final answer. Define allowed scratch work; expected outputs are computed independently outside the worktree. Reject global replace, lost comment, missing/extra target, verification too early, fake verification and prose-only completion; include a correct ordinary shell solution. `bugfix` and `reuse` are controls. Add a dedicated ACP campaign driver alongside `test/evals/_lib/trial.sh`, reusing existing tree setup, cost ledger, checker/telemetry and the stdio client pattern in `test/acpgate/acpgate_test.go`; the old two-process `-p` trial is not this driver.
   - Instrument the tagged work-turn boundary (`agent.turn`, before `Stream`) once per logical work request, including unanswered attempts at that level. Count physical provider attempts separately inside the adapter retry loop, tagging work vs auxiliary requests; count outer calls and nudges from `evalstats.Collect`, applied file operations from host metadata. Do not sum these counters together. Prove with scripted fixtures: two direct calls in one request, one script doing two inner operations, a retried request, a nudge with a subsequent work request, an auxiliary request, failure with no assistant message and forged guest success. Reject missing observations distinctly from measured zero.
   - New row schema: campaign/block/fixture/seed/arm, actual resolved config and digest, pass, logical work requests, assistant steps, provider attempts, applied/unknown counts, approvals, wall time, measured usage/cost. Keep actual usage/elapsed time for failures. Refuse schema/arm fallbacks, duplicate/missing pairs, missing spend or partial instrumentation. Add positive/negative decision fixtures; `bash test/evals/run.sh --selftest` before billing.
2. **Surface plumbing, read-only.** `internal/config/config.go` (`ResolveCodemode`, validation, layer
   rules: user opts in, trusted project may veto, project never widens), `internal/agent/start.go`
   (`planCodemode`, holder), `internal/agent/resume.go`, `internal/agent/prompt.go`,
   `internal/session/entry.go`, `internal/config/sample.jsonc`. Tests: strict decode rejects unknown
   keys; workspace-without-enabled and workspaceEdits-without-workspace are config errors;
   `network: true` with workspace is refused; project veto; legacy headers; the resume matrix of §4;
   provider wire names/order and off-prompt bytes unchanged; no VM opened when codemode is never
   called. `go test -race -count=8 ./internal/config ./internal/session ./internal/agent`. Treat the edit bit in this phase as plumbing only: no writable method may become live before phase 3.
3. **Land apply, consent, preview and journal together** (or keep writes unconditionally disabled until all four exist). Add `internal/tools/changeset.go` and tests; wire `tools.stageEdit` into `internal/codemode/prelude.js`, the adapter inventory/spec and `bridge.dispatch`. Separate stamp capture, parity bookkeeping and grants. Share direct `edit`'s investigation/state semantics without recursively invoking `Registry.Run`. Bind effective mode, trusted flag and headless flag as narrow data from `agent`; the registry supplies outer call identity. Implement §4's bounded stage machine, rooted snapshot/commit protocol and read-only recovery. If there is no working snapshotter or journal, refuse before any write. `internal/tools` does not import `session` or `permissions`. Test both normal writes and a denied second batch that cannot reuse the first approval.
4. **Receipt and replay across clients.** Thread typed metadata through `tools.Result`, `internal/agent/events.go`, the result persistence path, `internal/tui/toolsummary.go`, `internal/tui/replay.go` and `internal/acp/toolcall.go`. Verify previews, parent IDs, consent options and actual diff display in TUI and ACP. Leave ordinary read-only zero-approval tests intact; add a separate staged-consent gate. Confirm session rebuild produces one provider result; metadata cannot change replayed history into a new operation.
5. **Failure behaviour, in the real gate.** `test/codemodegate`: a real script over generated rows
   applies the expected bytes; one provider tool result; no intermediate sentinel in the next request;
   a direct green verification **after** the edits; a second process replays without editing; a killed
   child between two writes leaves a journal record; resume does not edit the workspace. Make the crash observable deterministically: add a gate-only build-tagged synchronization hook that stops the child after recording the first file and before starting the next; wait for its barrier, terminate the child, resume twice with an ordinary binary and inspect both disk and repaired transcript. No production kill switch. Inject separately: before `Begin`, after replacement but before snapshot/Record, and after `Finish` but before outer result. Exercise stale final target before consent, a changed target during consent, a failure on target two, steering, duplicate identity, text limits and preservation, wrong mode, untrusted, `-p` incl. yolo and unknown outcome. Assert prefix/failed/suffix and counters, not only a summary line. Verify permission refusals and a successful apply in ACP, plus a direct edit and a real verification using resulting state. `make vet && make test && make build`; run `bash test/codemodegate/run.sh` and `bash test/acpgate/run.sh`, exercise TUI consent, and verify native Windows/macOS/Linux replacement/denial. Run affected packages repeatedly with `-race -count=8` and the count/mirror gates. No green here proves a user benefit.
6. **Feasibility before spend.** Compare real binary/VM, ordinary direct/shell calls and an explicit exact-edit list through the same commit routine. Start each path with the **user's original information**: charge discovery, deriving target-specific values, sending the list/program, verification and actual responses. Do not give the list an oracle or charge each direct call a separate request. Use the same permission policy and production caps. If both fixtures can be done at the same request floor without JS and with comparable payload/effort, stop; a hand-written trajectory is a feasibility check, not an adoption result.
7. **One separately approved campaign** (§6). Pass: README/CHANGELOG/SPECS/AGENTS and an opt-in example with measured scope. No promotion or inconclusive: keep the evidence, phase 0 and reusable instrumentation, do not release mutations. Disabling the grant prevents future writes; it does not revert past edits.

## 6. The campaign

**Question (§5 phase 7):** is opt-in changeset application worth shipping for these workloads, compared with (A) moca as it ships and (B) the new reads without edits, measured on **real `moca acp` client sessions** in `ask` mode? Approval is the campaign's separate decision, before any billed trial.

- **Arms, one tagged binary with the same consent machinery:** A = codemode off; B = `workspace`, reads only; C = B + `workspaceEdits`. All arms have the same front end, direct tools and shell. No hint steers tool choice; staged changes show their once-only consent dialogue. Freeze before the first billed run: binary digest, exact effective configs, prompts, seeds, checker, driver, model, effort, corpus permission budget, caps, and every schema/limit above.
- **Assignment:** 10 balanced random triples (one run each of A, B, C) per target fixture, 2 fixtures, and 10 triples on the 2 ordinary controls. Use unique reproducible assignments; count and report every run, including failures and lost sessions, with run-to-run variance. No substitution or mid-campaign arm changes. One `-cap-scope` ledger; the ledger alone is not a stop button — a provider-enforced ceiling plus an operator abort rule is.
- **Endpoints.** Primary: logical work requests (the tagged work-turn count) until the checker first accepts, including work that fails; incomplete/incorrect scores at a predeclared cap (a real timeout is its own invalidity). Do not choose the endpoint by sampling; freeze it in phase 1. Two-sided paired sign tests at α=0.05, with reported exact two-sided binomial p-values and paired ratios, each hypothesis at 0.05 and FWER clearly labeled as a decision rule, not a claim of high power. Each arm pair needs **≥9/10 pairs in the same direction**, median paired ratio ≤0.75 (C vs A and C vs B), on **each** target fixture. Ties and missing data are not wins. n=10 detects large effects only; with fewer than 9 directional pairs, record "underpowered, no promotion" instead of a directional claim.
- **Preconditions and vetoes:** On each target, C has ≥8/10 correct results (pass/fail must have a denominator: any trial with missing pass data or missing/failed verification telemetry is invalid, and too many invalid rows invalidate the campaign) with **host-confirmed** applied/unknown counts in every C success, zero unauthorized/unreported changes (stop immediately on any), zero lost passes against A or B on any fixture, and C's control-fixture median work requests ≤ A's + 1. C vs A median elapsed-time ratio ≤1.10 and billed-cost ratio ≤1.10 from **persisted per-entry cost fields** (`session.Entry.Cost`, already summed by `evalstats.Collect` and `agent.go`), never from an interpolated rate; treat unknown costs as missing data. Report input/output/cache token totals and retries per arm; carry only as an explanatory diagnostic. Report adoption: distinct C trials with ≥1 staged change and success with ≥1 applied/unknown; analyze non-adopters as such, and never attribute C's win to shell use alone. Verification must actually run after the last mutation; "verified" claims without evidence fail the checker. Count unknowns as failures for adoption, but report them distinctly. Fixing the harness is a protocol change: rerun the whole campaign if any arm is affected.
- **Verdict, predeclared:** all conditions met ⇒ offer opt-in `workspaceEdits` (default off) for the measured class. Any precondition failed ⇒ no promotion; correct data but missed endpoints ⇒ do not promote. Missing telemetry, broken oracle, exhausted ceiling, or arm leakage ⇒ inconclusive, no promotion, and instrument repairs cannot be patched in afterward. Publish protocol, frozen configs, every row, exact two-sided p-values, paired ratios and representative traces in `test/evals/BASELINE.md`; never re-run paid trials on the same fixtures to chase a different verdict.

## 7. What not to build on this evidence

| Temptation | Why not now |
|---|---|
| Hide direct reads, or one script per turn | Earlier recorded rounds show step regressions, not savings. Keep direct reads and shell available; ordinary usage stays unchanged. |
| Wrap shell, MCP, or HTTP in JS | Different permission and egress problems; MCP already has a proxy tool. Revisit read-only cross-server joins with a separate user-approved plan. |
| Creation, deletion, rename, links, or overlay | Widens races and review surface beyond the measured evidence; keep the fixture within the supported set. |
| Persistent JS state, worker pool, TypeScript, or Bun | No measured workload requires them; cross-process state is rejected. |
| A carry-primary gate | Carry is a diagnostic; requests and cost decide. |
| Promote on the handwritten reference | Feasibility is not adoption. |
| Extend additive or transport grant scope for new features | Grants exist only in transport; P0 must narrow them before any new surface is discussed. |
| Retire or alias transport without a compatibility decision | Existing sessions and configs depend on it; removal is a separate owner decision with a migration path. |

Creation, deletion, rename and secret handling remain out of scope (§4, §7); if a fixture or user task requires them, stop and extend the plan first.

## Appendix A — disposition of revision 1 findings

Rev 1 is retained as the source comparison; its findings land as follows: F1→§3.1 (as P0, re-scoped by R3-1), F2→§5 phase 0, F3/F4→§4 staging contract, F5→§4 (as the proposed, separately approved read-eligibility relaxation), F6→§4 (resume matrix, R3-3 rework), F7→superseded: keep transport and document it (R3-3), F8→§1/§5 phase 1 (reuse existing counters, R3-6), F9→§6 (sign tests and FWER framing, R3-6), F10→§3.3 (real consent machinery, R3-2), F11→§4 budgets, F12→§4/§5 phase 5, F13→§3.3/§4, F14→§5 (named gates), F15→§6 freeze list, F16→§4 apply/undo wording, F17→§5 phase 1, F18→§4 (workspace has no external grants), F19→§4 apply (tracker semantics, R3-7), F20→§7 (creation row).

## Appendix B — status and approvals

- Checked at review time: `git status --porcelain` clean on `feat/codemode-v3` at `cd33020`; the v2pilot rows live in gitignored `test/evals/results/` (`v2pilot-off.jsonl` etc.). Everything else here is a proposal.
- Dependencies: phases 2–7 need the P0 fix and a working oracle. Phases 2–5 are one feature branch gated together. The campaign needs its own spend approval and operator abort authority. Any change to evaluation fixtures, graders, frozen configs, thresholds, model, or endpoint requires re-approval; a protocol fix reruns affected arms. New privileged reads, paths, creation, deletion, links, overlay, state, shell wrapping, or network access are out of scope without a new plan.
- Risks if partially shipped without the release guards: partial application, double-apply on resume, dead undo stack entries, surprise consent dialogues or silent multi-file writes. Treat those as release blockers.

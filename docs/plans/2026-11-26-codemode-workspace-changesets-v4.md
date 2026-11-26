# Codemode redesign: workspace changesets — implementation plan (revision 4)

**Status: amends revision 3, which is adopted in full except where replaced below.** Base
`cd3302055aec27b9fe79350a62052dc5c33f3d96`. Revisions 1–3 remain as the record; rev 1 keeps the
upstream source comparison. No implementation, release, default change or spend is authorized here.

**Why a delta instead of a fourth full rewrite.** Rev 3 is the plan of record for everything it states;
retyping 36 KB by hand to change eight passages would risk silent drift, which is the failure mode this
plan exists to prevent. Every passage this revision replaces is reproduced below in operative form.
Anything not mentioned stands exactly as rev 3 wrote it — including the evidence table, the upstream
comparison, §3.2, §3.3's table and consent mechanics, §4's surface table, primitive, path policy,
budgets, apply, crash record, result and resume matrix, and §7.

## 1. Rev 3's findings: all accepted, with three corrections to my own record

| ID | Rev 3's finding | Disposition |
|---|---|---|
| R3-1 | Exact-name matching on a parent `os.Root` is not a no-follow open; the budget path also gives additive grants the spec does not allow | **Accepted.** §2 below replaces rev 3 §3.1 item 2 with a cheaper fix than the one rev 3 proposes: stop granting *files* at all. |
| R3-2 | Consent was sequenced after writes (phase 4 vs phase 3) and `auto` was unspecified | **Accepted.** My phase split was the bug: a build between phases would have written without the dialog. §3 below fixes the sequencing and puts `-p` back in line with how `edit`/`write` already behave. |
| R3-3 | Additive would have acquired read parity; transport was both retired and maintained | **Accepted, my error.** SPECS §10.6 pins additive as no-parity (`TestCodemodeAsksNothingSpillsNothingAndRecordsNoRead`); only workspace gets parity. Transport stays a supported, documented surface. |
| R3-4 | Catchable staging errors and a poisoned batch cannot coexist; a session stamp is not a same-script read; investigation state was ignored | **Accepted.** Rev 3's correctable/fatal split and its investigation requirement are kept whole. |
| R3-5 | Journal contents, resume pairing and the crash window were missing or overclaimed | **Accepted.** I compressed rev 1's journal contract away; rev 3 restores it and adds the ordering against `session.Repair`, which the code supports (`internal/agent/resume.go:102–117` runs before `build`). |
| R3-6 | Carry is not a cost bound; assistant messages and the carry hook are not retry attempts; the four-way AND is not an "any-of-four" false-pass | **Accepted, my error on the last.** See §4. |
| R3-7 | Appendix rows were not operative; post-write tracker behaviour contradicted itself; the native-list counterfactual got its inputs for free | **Accepted, my error on the tracker.** `write.go` finishes with `env.Reads.Record(abs)`; `noteChange` (`runstate.go:67`) never touches the tracker. A later direct edit needs no re-read, exactly as rev 3 says. |

**Corrections to committed numbers.** Rev 3 caught that my `BASELINE.md` carry sums disagree with the
rows. Rechecked field by field: `off` is **42 870** (the table says 47 882) and `additive` is
**70 679** (the table says 68 678); `additive-400` (30 146) and `transport` (32 863) were right. The
correction is applied to `BASELINE.md` with a footnote; the verdict is untouched. The disclosure probe
that proved the grant bug used a **synthetic** sibling named `auth.json` in a temporary directory — no
real credential file was read, and the same shape becomes the regression fixture.

## 2. A1 — The P0 fix: grants are directories, never files

**Replaces rev 3 §3.1 item 2.** Rev 3 is right that a file grant implemented as "parent root plus one
basename" is not a no-follow open, and right that `Lstat` before an ordinary `Root.Open` is racy. But
its remedy — a reviewed handle-relative no-follow primitive per OS, refusing the grant where none
exists — is a platform matrix (`O_NOFOLLOW` does not exist on Windows) for a capability that buys
almost nothing. The cheaper fix removes the problem class:

1. **A grant is a directory. A file is never granted.** `openWorkspace` accepts only entries that are
   directories at open time; a file path is skipped with a typed `not_granted` note rather than
   widened. Directory grants are already safe: `os.Root` refuses escapes, so an in-directory symlink
   that leaves the tree cannot be followed, and an in-directory symlink that stays inside it is the
   same trust domain.
2. **The config file stops being a script grant.** `Jail.ReadRoots()` is consumed only by the codemode
   holder (`internal/agent/start.go:540`); the jail's own `readOnly` list, which the direct `read`
   resolves against, is untouched. So the direct tool keeps reading `config.jsonc`, a script does not —
   a deliberate, documented asymmetry, and the price is one file.
3. **Grants are transport-only, as SPECS §10.6 already says.** `Inspection.configure` takes the grant
   list only for transport; additive keeps its budget and no roots. That is rev 3 §3.1 item 1, kept.
4. **No re-pin question, no Windows story.** With no file grants there is no identity to pin, nothing
   to invalidate when moca rewrites its own config (`config.writeEdited`), and no reparse-point matrix.
5. **Tests** (same fixture shape as the probe): additive gets no external roots at all; transport reads
   inside a granted directory; a sibling of the *former* config grant is `outside` (it now has no grant
   to inherit); a file where a directory grant is expected is skipped, not widened; a grant that is
   missing at open grants nothing; `auth.json`-named and credential-shaped files inside the **workspace**
   remain readable by both paths (they are workspace content, and the workspace policy is not a secret
   scanner).

If the owner wants the script to read the user's config file anyway, rev 3 §3.1 item 2 stands as
written and the plan inherits the per-OS no-follow work; the default recommendation is §2 as above.

Everything else in rev 3 §3.1 (ship standalone, release note, keep codemode disabled as mitigation
until it lands, native-platform results) is kept.

## 3. A2 — Consent: authority follows the permission mode

**Replaces rev 3 §3.3's "Consent decision (proposed)" and the one sentence in §5 phase 1 that makes a
bespoke ACP campaign driver mandatory.** Everything else in §3.3 — the one-shot, diff-bound,
allow-once question, the immutable set, `Question{Kind:"changeset", CanAlways:false, NoSession:true}`,
the post-approval revalidation, the invisibility of `env.Ask != nil` as an authority test — is kept
verbatim.

The rule, aligned with how `edit`, `write` and `shell` already behave in each mode:

| Mode | Interactive | `-p` |
|---|---|---|
| ask / default | one aggregated question before any write | **refuse** the changeset (no human); reads and direct tools unchanged |
| auto | one aggregated question (a multi-file write is exactly the "unsure" case) | **refuse** |
| yolo | no question; every path, stamp, journal and investigation guard still applies | **permit**, because yolo already permits unbounded `write`/`edit`/`shell` without a human |
| plan | refuse | refuse |

Rev 3's rule refuses in `-p` **even under `-p --mode yolo`**. That is theatre: yolo exists to do
unattended work, and in that mode the same model may already rewrite the same 32 files one `edit` at a
time, or `rm -rf` the tree. It also kills the one automation use case a changeset actually serves, and
it forces a new campaign driver.

**Campaign consequence.** `test/evals/_lib/trial.sh:106` already passes `--mode "${MODE:-ask}"`, so the
existing `-p` harness runs the campaign with `MODE=yolo` and no new driver: the same two-prompt
trials, the same ledger, the same checker. Consent itself is proved by tests, not measured by the
campaign — a dialog costs wall time, not model requests, and the primary endpoint is requests.
An ACP leg remains in the gate (rev 3 §5 phase 5 already requires one) to prove the interactive
question renders, binds and denies; that is a test, not a harness.

**If the owner prefers rev 3's stricter rule**, this amendment is withdrawn and the plan must add: a
bespoke ACP campaign driver (reusing tree setup, the ledger and the checker), its own phase-1 line
item, and a campaign that measures interactive rather than headless requests. That is a day of harness
work plus a second way to run trials — worth it only if `-p --mode yolo` is judged too dangerous for a
user who has already enabled edits explicitly.

## 4. A3 — Three arithmetic and instrumentation corrections

**Replaces rev 3 §6 "Endpoints", second half; keeps ≥9/10, the median-ratio bar, "ties and missing
data are not wins" and "record underpowered, not directional".**

1. **The four-way AND is conservative, not permissive.** Rev 3 does not claim otherwise; my rev 2 did
   and was wrong. With the direction predeclared (C better), one comparison at 9/10 has a one-sided
   tail of 11/1024 ≈ 0.0107; requiring all four (C vs A and C vs B on two fixtures) multiplies, so a
   false pass is vanishingly unlikely rather than ~20 %. Say it, so nobody "fixes" the rule later.
2. **The real risk is underpower.** At a true per-trial win rate of 0.75, P(≥9/10) ≈ 0.24; at 0.90 it
   is ≈ 0.74. A genuine 25 % improvement will be recorded as "underpowered, no promotion". That is the
   right conservative choice, and it must be stated **before** the spend, not after.
3. **Carry is a diagnostic, never a bound.** My rev 2 compared turn-1 carry with cumulative
   cache-read tokens (3.0 %) and implied a maximum saving. Those are different quantities: carry is one
   observation after continuation, cache reads are summed provider usage across every request, and the
   carry figure is present in the prefix of each later request at cache rates. Keep the published
   verdict, quote carry only as a diagnostic, and let measured requests, elapsed time and cost decide.

Also keep rev 3 §1's instrumentation split (logical work turns at `agent.turn` before `Stream` vs
physical attempts inside `provider/retry.go`) — checked: `retry.go`'s loop does call `r.a.Stream`
several times per logical turn, so the two counters are genuinely different and must not be summed.

## 5. A4 — Four details the staging contract still needs

**Adds to rev 3 §4; nothing below contradicts it.**

1. **Stamp lookup must be path-normalized.** `Inspection.StampFor(requested)` keys on the string the
   guest passed, so `read("./a.yaml")` followed by `stageEdit({path:"a.yaml"})` is a false refusal
   unless both sides are canonicalized through the same `cleanRel`. Test that exact pair, and
   `read` with `offset`/`next` (the full-file stamp is captured from the descriptor on the first page,
   so a paged display is still eligible — as rev 3 says).
2. **The description is part of the feature.** It must name `stageEdit`, state **read before stage**
   (without it every first attempt burns a round trip on a validation refusal — the cost the feature
   exists to remove), and say that a program's return is not a write. Sketch, within the existing
   description budget: `…; await tools.stageEdit({path, edits:[{old,new}]}) stages exact edits the host
   applies after this program returns — read the file first; staged is not written.` Changing this text
   resets the prompt cache for every codemode user: batch it with §3.2's catalogue repair, and pin the
   description and the capability inventory together, as §3.2 already requires.
3. **`os.SameFile` is the identity primitive.** For duplicate targets, aliases and hard links, compare
   the `FileInfo` that `os.Stat` returns (dev+ino on Unix, volume+index on Windows); do not hand-roll an
   identity from `Sys()`.
4. **`.env*` blocks `.env.example`.** Either accept it and say so, or allow the `*.example`, `*.sample`
   and `*.template` suffixes. Recommend the suffix allow-list: one predicate row, one test row, and it
   keeps a normal repository task editable.

## 6. A5 — Pins and documents the phases must carry

**Adds to rev 3 §5.** Each of these is a house rule that a green test suite does not otherwise enforce:

- `cmd/moca/pilot_arms_test.go`: the campaign arms must differ **only** inside `codemode`, and
  `TestEvalConfigsPinTheCorpusModel` pins the corpus model — both extend to the new arm configs, or a
  typo silently changes the design being measured.
- SPECS: §4 (keys, layer rules, validation, `sample.jsonc`), a §10.6 subsection for the workspace
  surface **in the same PR**, §15 record at the end; the byte mirror
  `internal/skills/builtin/self-knowledge/references/specs.md` is copied in the same commit.
- AGENTS' "eight frozen tools" sentence does **not** change: workspace declares eight plus the adapter,
  the same nine as additive, and nothing moves except inside the script.
- `go test -race -count=8` on each touched package before a phase PR, not only `-race` once.

## 7. A6 — The gate grows one named test

**Adds to rev 3 §5 phase 5.** `TestChangesetConsent` (new top-level test in
`test/codemodegate/codemodegate_test.go`): interactive approval is asked once and binds to the set; a
denied batch cannot reuse the previous approval; an empty set asks zero times; `-p` in ask and auto
refuses; `-p --mode yolo` applies and reports a receipt. `TestZeroApprovalsInEveryMode` stays as the
read-only proof and keeps asserting zero for read-only calls. The gate's top-level count goes from
eight to nine — pin it wherever the gate counts are pinned today.

## 8. What still stands, and what would make this plan wrong

Unchanged and adopted from rev 3: §1's evidence and instrumentation, §2's upstream comparison, §3.2
(catalogue vs table), §3.3's table and question mechanics, §4's surface table, primitive, read
eligibility, path policy, budgets, apply, crash record, result and resume matrix, §5's phases (with
the amendments above folded in), §6's assignment, vetoes and verdict, §7.

This plan is wrong, and should be rewritten rather than patched, if any of these turns out true:

- A directory-only grant set cannot serve a real transport workflow, so file grants must come back —
  then rev 3 §3.1 item 2's per-OS no-follow work is mandatory and §2 above is withdrawn.
- `-p --mode yolo` is judged too permissive for a changeset even though it already writes — then A2 is
  withdrawn and the ACP driver becomes mandatory.
- Phase 4's feasibility check shows an explicit exact-edit list reaches the same request floor as a
  program (charging discovery for both) — then the staging primitive has no case on these fixtures.

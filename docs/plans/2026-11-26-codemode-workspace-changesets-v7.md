# Codemode redesign: workspace changesets — implementation plan (revision 7, consolidated)

**Status: the single normative text. Supersedes revisions 3–6 as the plan of record; they remain the
review record, and revisions 1–2 keep the upstream source comparison and the first read.** Base
`cd3302055aec27b9fe79350a62052dc5c33f3d96`; the standalone P0 has since landed (`f0f0b06`, §9).
Consolidated 2026-11-26 from revisions 3–6. Four things differ from rev 6, each named where it occurs:
the P0 is landed (§9), the gate's top-level count is nine so the consent test is the tenth (§3.4), and
two designs revisions 4–6 left open are closed (§4.7 the snapshot capability, §4.10 the TUI consent
body). Rev 3's standing network-catalogue defect is restated in §3.2 so the consolidation loses
nothing. Every other decision below is the one a reviewed revision reached.

**Goal, unchanged since rev 1:** one bounded, data-dependent change across a repository in a single
call — discovery, decisions and exact edits computed in the program, applied by Go after the program
succeeds, with an inspectable receipt and a real verification run. A one-file change stays an ordinary
`edit`.

**Nothing here is authorized.** The P0 shipped alone (§9). Phases 2–7 are an opt-in candidate branch
until the owner's release decision; the campaign needs its own amount, ceiling and abort approval.
Three design choices in this text need explicit owner sign-off before their phases: unattended
`-p --mode yolo` changesets and the scoped campaign claim (§4.10, §6), the edits-only 768 B spec cap
(§4.12), and the public env-example suffixes (§4.5).

---

## 1. What the evidence does and does not establish

The v2 transport comparison in `test/evals/BASELINE.md` missed both predeclared bars: 7/10 pairs
carried less, median paired ratio 0.778 (required ≥8/10 and ≤0.5). Raising the additive attempt budget
from 50 to 400 helped on that workload. Neither finding establishes a changeset benefit.

The corrected per-arm record (recomputed from the gitignored `v2pilot-*.jsonl` rows; the three stale
sums and their footnote are fixed in `BASELINE.md`):

| Arm | Rows | Carry sum | Cache-read sum | Passes |
|---|---:|---:|---:|---:|
| off | 10 | 42 870 | 1 406 720 | 4 |
| additive (50) | 10 | 70 679 | 2 504 704 | 6 |
| additive-400 | 10 | 30 146 | 1 152 256 | 8 |
| transport | 10 | 32 863 | 1 387 520 | 9 |

- **Carry is a diagnostic, never a bound.** It is one observation after continuation; cache reads are
  summed provider usage across every request. Do not compare them, do not infer a maximum saving, and
  do not use carry as a primary endpoint. Measured requests, elapsed time and cost decide.
- **Pass counts from that fixture are not capability.** The fixture failed a run that left an untracked
  file while the prompt never stated the rule; 11 of 12 failures were that rule. The rule is stated now
  and the checker grades the answer before the tree — a failure must name the thing it failed.
- **Instrumentation must not be conflated.** `evalstats` (`test/evals/evalstats/main.go:272–287`)
  counts assistant messages as `Steps`, marked entries as `Nudges` and outer tool uses as `Calls` —
  diagnostics. `internal/agent/agent.go:519–520` observes logical work before `adapter.Stream`;
  `internal/provider/retry.go:82–88` may make several adapter attempts inside it. An unanswered request
  is not a persisted assistant turn. Phase 1 separates logical work requests, physical provider
  attempts, outer calls and applied file operations, and never sums them.
- Kept from rev 1: no repeat of the additive prompt experiment, no transport promotion, the fixed pilot
  oracle is not a capability figure, and shell stays available in every arm.

## 2. What OpenCode and Pi implement — and what moca copies

The earlier claim that both expose *every* direct tool inside JS was wrong. Pi allows overlap between
direct and script-callable tools and can hide a declaration per tool (`exposure: direct | codemode |
deferred`, `only`); OpenCode makes catalogued paths callable inside `execute` and leaves other tools
direct. The portable lesson is **policy parity plus host-owned receipts**, not "dispatch everything".

| | Copy | Do not copy |
|---|---|---|
| Authority | staged edits go through the **same** jail, read guard, snapshot, plan-mode and config-file rules as a direct `edit` — or stricter | a second dispatcher that reaches tools the direct path cannot |
| Receipt | host status is authoritative; guest text cannot override it; paths and counts are structured | trusted guest assertions, unbounded captured output |
| State | explicit return; discard-on-failure; a new process never replays pending work | persistent JS state, branch-scoped stores |
| Execution | one VM per call, fresh | promises, workers, background writes, another VM |

## 3. What the tree enforces (verified, not assumed)

A change that ignores any of these fails review.

### 3.1 Containment — the P0 rule, now live

A codemode grant is a **directory**, never a file, and only the transport surface receives grants:

- `internal/tools/inspect_root.go` — `openWorkspace` accepts only entries that are directories at open
  time (`os.Stat` + `IsDir`); a file is skipped, never widened to its parent; a missing entry grants
  nothing, and the grant set is resolved once, when the inspection first touches the filesystem.
- `internal/tools/codemode.go` — `CodemodeHolder.inspection` calls `configure(grants, attempts, true)`
  for transport and `configure(nil, attempts, false)` otherwise, so the **budget** no longer decides
  whether a script may read outside the workspace.
- `internal/permissions/jail.go` — the jail's own read list, which the direct `read` resolves against,
  is untouched: the direct tool still reads moca's `config.jsonc`, a script does not.
- Ungranted absolute paths answer the existing `outside` class. There is no `not_granted` class.

Any new surface inherits this: **no external grants outside transport**, and no widening of a grant
from the path the user configured to a path they did not.

### 3.2 Capability inventory and the three specs

`codemodeSpec` (`internal/tools/codemode.go:183`) builds the spec; the schema is identical in every
variant and the description is the only thing that varies. Measured against the goldens:

| Variant | Sessions | Description | Cap |
|---|---|---|---|
| read-only | additive, transport, `workspace` without edits | `codemode-schema.golden.json`, **542 B incl. newline** (541 marshaled) | **576 B** |
| network | any variant with `network: true` | its own text, no method shapes | **640 B** |
| workspace + edits | `workspace` + `workspaceEdits` only | §4.12, **712 B incl. newline** (711 marshaled) | **768 B** |

The read-only text begins "Read-only workspace JS…" and promises no absolute-path or grant behaviour,
which is why the `workspace` read-only session can share it. A description edit resets the prompt cache
for every codemode user of that variant: batch description changes per variant.

**Known standing defect (independent of changesets).** The network variant's text advertises
`tools.web.fetch` and `tools.web.search`, and `workspaceCapabilities()`
(`internal/tools/codemode.go:280–288`) has no row for either — `caps` is always built from that one
table (`:255`), so `dispatch` answers `refused`, "no such tool in this session: web_fetch" (`:315`).
No Go capability named `web_fetch` or `web_search` exists anywhere in the tree. A network-enabled
session is therefore promised two calls that can never succeed. The repair is to generate the
fresh-session description and the prelude from **one** capability inventory; a session whose stored
header promised the network shape keeps it and answers a typed refusal, and must never acquire network
access it was not admitted. Fix it as a standalone, unbilled change with its own golden update; it
cannot block the changeset work and the changeset must not depend on it.

### 3.3 Staged writes reuse the permission machinery that already exists

| Concern | Existing mechanism | What staging does |
|---|---|---|
| Path | `Env.Paths.Resolve(path, true)` is necessary, not sufficient (`agent/mode.go` swaps in a broader checker for auto/yolo) | additionally enforces a root-relative workspace-only write in every mode; never calls `askOutsideWrite` for a nested target |
| Plan / trust | `Env.Plan`, start options and resolved user/project policy | no staged mutation in plan or untrusted workspaces, including headless and yolo; a saved declaration is not a grant |
| Read / investigation | `Env.Reads.Check` allows a missing file for ordinary creates; `edit.go` also calls `investigationRefusal` | requires an existing, same-script stamped object and a completed failing-test investigation; no create on disappearance |
| Snapshot / tracking | `guardedWrite`, `Snapshotter.Guard`, `noteChange` | reuses policy and state updates, not pathname-based I/O; reports durable snapshots separately from bytes changed |
| Protected paths | direct config writes have an exceptional one-shot ask; `Env.ConfigFile` identifies that target | stricter: denies config, instructions and protected targets outright; approval cannot override |

### 3.4 Facts that shape consent and reporting

- **In-workspace writes never ask.** `jail.Resolve(path, true)` returns an in-root path at
  `internal/permissions/jail.go:219–221`, before any question (`Resolve` is `:214–231`), and
  `internal/tools/write.go` has no general write question. Auto's optional classifier (`internal/agent/mode.go:116–127`) judges **shell command strings**; there
  is no changeset "unsure" classifier. The changeset question is therefore a deliberately *stricter*
  guard, not a restatement of direct-write behaviour (§4.10).
- **The question is two strings.** ACP (`internal/acp/session.go:283–319`) sends
  `session/request_permission` with `Title: q.Kind + ": " + q.Subject` and
  `RawInput: {"detail": q.Detail}` — `Detail` is one string. TUI (`internal/tui/approval.go:74–106`)
  renders `Detail` line by line, capped at `approvalDetailRows = 6` (`:51`), with a
  `… +N lines · ctrl+o shows the whole command` hint (`:101–103`).
- **ctrl+o already pages the pending question.** `internal/tui/app.go:743–748` opens the pager on
  `Question.Detail` when an approval is on screen. No new preview UI is needed (§4.10).
- **Options.** `approvalState.options` (`internal/tui/approval.go:33–48`) emits allow-once, then
  allow-for-session unless `Question.NoSession`, then allow-always when `CanAlways`, then deny. So
  `{CanAlways:false, NoSession:true}` is exactly two rows on both surfaces.
- **The gate is `-p` only.** `test/codemodegate` drives `moca -p`; an interactive approval needs a real
  ACP stdio client, and the pattern exists in `test/acpgate/acpgate_test.go:63–150` (§5 phase 5).
- **The gate has nine top-level tests today** (eight before the P0's `TestGrantScope`), so
  `TestChangesetConsent` is the **tenth**. Every pinned gate count moves 9 → 10 with it.

### 3.5 Read, stamp and snapshot mechanics

- `write`/`edit` finish with `env.Reads.Record(abs)` (`internal/tools/write.go`): the tracker learns the
  state moca itself wrote, so a following direct edit needs no re-read. `noteChange`
  (`internal/tools/runstate.go`) is instrumentation only (`Env.Edited`, `Env.Unverified`, repeat
  history) — it never touches the tracker.
- `ReadTracker.Check` returns nil for a missing file (the create path), so it cannot authorize a staged
  edit of a vanished target; staging needs its own existence check.
- `Inspection.StampFor(requested)` keys on the string the guest passed (`internal/tools/inspect.go:165`),
  so `read("./a.yaml")` and `stageEdit("a.yaml")` must both be normalized through the same `cleanRel`
  (`internal/tools/inspect_root.go:112`) or the lookup is a false refusal.
- The full-file stamp is captured from the descriptor on the first page, so a paged display is still
  eligible.
- `Snapshotter.Guard(abs, write)` reopens the absolute pathname before and after its callback
  (`internal/session/snapshot.go:63–92`) and stores the pre-image plus a post-image sum; `Undo` refuses
  when the current content no longer matches that sum (`:124`). A root-relative, verified-image apply
  cannot use it as-is (§4.7).
- `bridge.dispatch` refuses an unknown method with `Class:"refused"`, "no such tool in this session: X"
  (`internal/tools/codemode.go:315`) — a misleading answer for a capability the header promised and
  current policy revoked (§4.11).

## 4. The contract

### 4.1 Exposure and surfaces

`workspace` is opt-in and distinct from the existing additive and transport contracts. It retains eight
direct tools plus the adapter. No default change, silent alias or removal of transport; transport stays
documented and resumable. Any deprecation or removal is a separate compatibility decision with owner
approval and a migration for old headers.

| Session | Provider surface | Script authority |
|---|---|---|
| off | eight | none |
| additive | eight + `codemode` | workspace `read/search/ls/glob`, **no** external grants, no session read parity (unchanged, and now actually true — §9) |
| transport (supported legacy opt-in) | six | read parity and typed **directory** grants; direct hidden names still refuse |
| `workspace` | eight + `codemode` | `read/search/ls/glob`, **workspace-relative only**, no grants |
| `workspace` + `workspaceEdits` | same names | plus `stageEdit`, applied by the host after a successful program |

### 4.2 The primitive

```js
tools.stageEdit({
  path: "services/billing-api/deploy.yaml",
  edits: [                                   // ordered, applied to the staged content in order
    { old: "image: billing-api:1.4.2", new: "image: billing-api:1.4.3" },
    { old: "digest: sha256:aaaa",      new: "digest: sha256:bbbb" },
  ],
})        // -> {staged: true, path, edits: 2}
```

- One ordered `edits` array per target; a duplicate successful stage of the same filesystem object is
  rejected (`os.SameFile`, §4.5). A later pair sees the bytes produced by earlier pairs.
- Require a nonempty array, nonempty `old` with exactly one byte match, `old != new`, and a final image
  different from the original. An empty `new` may remove a substring, not the file. No fuzzy edit,
  prefix stripping, `replace_all` or implicit creation.
- Reads during staging still see the original files.

### 4.3 Stage-time validation and the correctable/fatal split

Validate on the host at stage time.

- **Correctable:** a missing or ambiguous match. Commit no staging state for that target, mark it
  unresolved, return a typed mismatch. Only a successful restage of that same target clears it.
  Returning with any unresolved target discards the whole batch.
- **Fatal for this changeset, even if caught:** every other invalid stage, authorization failure,
  aliasing, budget exhaustion and cancellation.
- `{staged:true}` is not an assertion that any bytes landed.
- Test corrected, uncorrected and fatal-but-caught separately.

### 4.4 Read eligibility

A **successful read in this script** with a captured full-file stamp from the opened descriptor.
A prior-session read, a listing, a search result or a guest-supplied digest is insufficient. A partial
display is fine — it is not a claim to have inspected the whole file or tree. Compare the actual
preimage bytes, size, identity and modification time at stage, preflight and commit. Bound hashing and
reads while collecting bytes. `ReadTracker.Check` supplements this and never authorizes a missing
target. Workspace reads and searches maintain transport-like investigation bookkeeping — do not use
additive's legacy no-parity switch to gate workspace tracking.

This is a proposed relaxation from rev 1, not a fact about user consent.

### 4.5 Path and content policy

Workspace-relative existing ordinary files only.

- Reject absolute paths, `..`, dot-prefixed components, noncanonical spellings, linked or reparse
  components, multiple hard links, and equivalent targets by file identity (`os.SameFile` — dev+inode
  on Unix, volume+index on Windows; never a hand-rolled `Sys()` comparison).
- Respect real filesystem case and alias semantics, including Windows reserved names and alternate
  streams.
- Reject repo instructions, `.git`, `.moca`, skill/prompt/config/audit files and known credential
  filenames — at minimum `auth.json`, `credentials.json`, `.env*`, `*.key`, `*.pem` — independent of
  case. This is not secret detection in arbitrary files.
- **Env-example exception (owner sign-off).** One case-insensitive helper over the final path component,
  conceptually `envExample(base)`: true only when `base` begins with `.env` and ends in `.example`,
  `.sample` or `.template`. The **same helper** serves both predicates — the dot-component rejection
  (which may exempt only that final component) and the `.env*` credential-name rejection (which must not
  block an admitted example). Every other dot-prefixed component stays refused; all identity, link,
  stamp, UTF-8/NUL, size and content rules still apply. Pin: `services/api/.env.example` and
  `.ENV.SAMPLE` stage; `.env`, `.env.local`, `.env.example.local`, `services/.config/.env.template` and
  ordinary dotfiles refuse. Test both call sites independently.
- Validate the entire bounded pre/post image as UTF-8 text without NUL; preserve untouched bytes, BOM,
  CRLF and supported metadata. Skip unsupported special modes/ACL-preservation cases with a typed
  refusal — never weaken permissions to replace a file.

### 4.6 Budgets

Existing inspect/source/entry/VM/ABI limits still apply, and **every attempt, including a failed
staging, charges the script's attempt budget.** Additionally:

- ≤32 targets per changeset;
- ≤512 KiB per file **before and after**;
- ≤32 KiB decoded edit text per target, ≤256 KiB total accepted edit text;
- ≤2 MiB combined retained pre/post images;
- a 30 s active host-work budget for preflight, revalidation and apply, excluding the human wait; if it
  expires, start no further file.

Check growth before each replacement and bound intermediate buffers, diff rendering and result
encoding. Charge before buffering, including revalidation reads. Test exact-fit, first over-limit,
multi-byte text and expansion in a middle edit. No quota on how many outer scripts the model may choose.

### 4.7 Apply — state machine, rooted I/O, snapshot, journal

The host alone transitions `staging → preflight → consent (if required) → revalidate → begin journal →
apply serially → finish`. All phases honour context cancellation and steering. The guest deadline bounds
computation; the human wait is separately cancellable and is not a detached job.

**Rooted I/O.** One `os.OpenRoot(workdir)` for the apply. All staging, preflight, commit and read-back
I/O goes through it — `Root.ReadFile`, `Root.Lstat`, `Root.OpenFile`, `Root.Rename`, `Root.Remove` — so
every path is root-relative and escapes are refused by the runtime rather than by string checks.
`go doc os.Root` (Go 1.27.4) confirms `Rename`, `Lstat` and `OpenFile` exist and that symlinks are
followed only within the root. Replacement is a temp file created **in the target's own directory**
through the root, a random name with `O_CREATE|O_EXCL`, the target's mode **set at creation** — never
`Root.Chmod`, which the standard library documents as racy on Unix when the target becomes a symlink
mid-operation — then `Root.Rename` over the target. Refuse filesystems where rename-over is unsupported.

**Snapshot interface (closed here; the one API revisions 4–6 left undesigned).** `Guard` reopens the
absolute pathname, so the changeset cannot use it. `tools` declares an optional capability beside
`Snapshotter` (`internal/tools/tool.go:45`):

```go
// ImageSnapshotter is the option Env.Snap exposes to a changeset: record the exact
// pre-image the caller validated and the exact post-image it is about to write,
// without reopening either path.
type ImageSnapshotter interface {
	GuardImage(abs string, pre, post []byte, mode fs.FileMode, write func() error) error
}
```

- The capability is discovered by type assertion (`img, ok := env.Snap.(tools.ImageSnapshotter)`), the
  pattern `Exposable` already uses, so existing fakes stay valid.
- `session.Snapshots.GuardImage` writes the pre-image blob, records `Mode`, `Blob=sum(pre)` and
  `After=sum(post)` **before** calling `write`, appends the stack entry and the session entry **after**
  `write` returns nil, and never reads the path. It does **not** run `gitClean` (a subprocess that
  would stat and read through the pathname) — a changeset's pre-image is already in memory and
  verified, and always storing the blob is the safer undo direction.
- The changeset verifies the landed image by reading back through the pinned root and comparing it with
  `post`; a mismatch is `outcome_unknown` (§4.8) and the wrong `After` makes a later undo refuse rather
  than clobber.
- **If `Env.Snap` does not implement `GuardImage`, or the journal is absent, staging refuses before any
  write.** No fallback to `Guard`.

**Ordering per file:** `GuardImage(abs, pre, post, mode, rename)` → read-back through the root → journal
`Record`. The journal (below) has already persisted `Begin` for the whole set. The crash-injection
points in phase 5 are therefore: before `Begin`; after the rename but before `Record`; after `Finish`
but before the outer result. The window between a replacement and its snapshot no longer exists as a
separate step — the consolidation's change, and a strictly smaller window.

**Crash record and pairing.** Inject a `Begin/Record/Finish` journal through `tools.Env`; the production
binding belongs to `agent.build`. Identify a batch by session + outer transcript invocation, never by JS
input. Before any mutation persist the ordered relative paths, pre/post digests and call identity. A
start-record failure means zero writes. Record each result, then finish; on append failure stop
immediately.

On resume, under the session lock, consult journal state **before** `session.Repair` supplies generic
orphan results (`internal/agent/resume.go:102–117`). For a finished apply with a missing outer result,
reconstruct the bounded host receipt once; for an unfinished start, report the interrupted/unknown paths
explicitly, using safe read-only comparison to the recorded digests when permitted. A digest match is
evidence of observed state, not authorship. No change replay and no automatic undo, including on a
second resume. Derive per-path states from persisted evidence; never synthesize a successful tool result
from guest output. Scope durability claims to process crash unless power loss is explicitly tested.

### 4.8 Failure taxonomy

| Outcome | Meaning |
|---|---|
| `inspected` | no stage calls, or an empty set; nothing to apply |
| `applied` | every target's read-back matched its post-image |
| `discarded` | nothing written: validation, authorization, denial, steering, cancellation or a pre-write failure |
| `partial` | one or more writes landed, then a known failure stopped the suffix |
| `outcome_unknown` | a possibly landed write whose snapshot/tracker/journal outcome is incomplete |

`partial`, `discarded`-due-to-failure and `outcome_unknown` are **not** successes. No automatic retries,
no rollback. Preserve `Env.Edited`, `Env.Unverified` and investigation/verification behaviour on actual
changes; an uncertain write still requires inspection and verification. On confirmed success record the
written version as direct `guardedWrite` does: a later direct edit may proceed if still current, and a
later changeset needs its own read. Do not stamp a subsequently observed foreign version as ours.

A snapshot is **not** a guaranteed restoration: clean git snapshots give restore guidance, dirty files
have guarded blob undo, and later edits can make undo refuse. The receipt lists confirmed snapshot
records; it promises no number of usable undos after a failure.

### 4.9 Result and receipt

One bounded provider tool result carrying host status, per-path outcomes, counts, coverage, truncation
and a separately labelled guest return. Typed host metadata travels out of band to session/TUI/ACP/evals,
including for inspection-only calls; reserve bytes for host status and counts before guest text. Full
bounded diffs belong in the user-facing preview and receipt, never in every provider request or in
application logs. Replays never execute metadata. Headless stdout remains the final answer. Preserve
call/result pairing on cancellation; never dispatch later outer mutations in the same batch after
observed steering — pair skipped calls explicitly.

### 4.10 Consent

**Modes.**

| Mode | Interactive | `-p` |
|---|---|---|
| ask / default | one aggregated question before any write | **refuse** the changeset (no human); reads and direct tools unchanged |
| auto | one aggregated question — a multi-file write is exactly the "unsure" case | **refuse** |
| yolo | no question; every path, stamp, journal and investigation guard still applies | **permit** — yolo already permits unbounded `write`/`edit`/`shell` without a human (owner sign-off) |
| plan | refuse | refuse |

This is deliberately stricter than direct writes, not "the same" (§3.4). It is deterministic: no
provider classifier call, no extra model request, and no `env.Ask != nil` front-end inference. Bind
effective mode, trusted flag and headless flag as narrow data from `agent`.

**The question.** After guest success and whole-set preflight, freeze the ordered paths and pre/post
digests. `Question{Kind:"changeset", CanAlways:false, NoSession:true}` — two rows, allow-once and deny,
on both surfaces.

- `Subject` = `"<n> files, <m> edits"`.
- `Detail` = the ordered relative paths with each file's edit count, then the hunks, **bounded to
  16 KiB** with an explicit elision marker and the count of omitted hunks. The full diff belongs to the
  receipt after apply.
- Bind the answer to that immutable set and to the parent call ID. Denial, disconnect, timeout, pending
  steering or a changed target while the dialogue is open invalidates the batch. Revalidate the
  **whole set after approval**, before journal `Begin`; never silently recompute the approved edit.
  Test two successive batches asking twice and an empty set asking zero times.

**TUI body (closed here).** No new preview surface: the existing pager already does the job. ctrl+o on a
pending question pages `Question.Detail` (`app.go:743–748`), and the panel already prints
`… +N lines · ctrl+o shows the whole command` (`approval.go:101–103`). What changes: a `Kind ==
"changeset"` branch chooses the pager title and the hint's noun ("the whole preview", not "the whole
command"), and the panel keeps its subject line, six-row cap and hint. This is a wording and labelling
change with one branch, tested in the view model; not a scroll implementation and not a second pager.

**Tests.** Assert the **string handed to `Ask`** in both clients (every path present, counts right,
elision marker when truncated), the TUI panel's hint and pager body, and exactly two ACP options —
one allow-once and one deny — because `NoSession: true` removes the session row
(`approval.go:33–48`, `session.go:288–301`).

### 4.11 Authority and resume

Effective write authority = the bit declared in the workspace header ∩ current user opt-in ∩ trusted
workspace ∩ trusted project's non-veto ∩ editing mode ∩ supported front end. Only the user selects
`workspace`; a trusted project may turn the feature or edits off or lower caps, and may never switch a
session to a more privileged profile. Keep additive/transport precedence for the legacy settings. Reject
inconsistent user combinations before opening a VM. Persist a workspace ABI and the original
edit-declaration bit — not an approval. Fresh sessions use current policy.

| Stored ↓ / current → | off | additive | transport | workspace |
|---|---|---|---|---|
| off (no adapter in header) | off | off | off | off — never added retroactively |
| additive (incl. old `enabled` headers without `exposure`) | refusal adapter | additive | mismatch | mismatch |
| transport | mismatch | mismatch | transport | mismatch |
| workspace | mismatch | mismatch | mismatch | workspace; no new edit grant |

Mismatch ⇒ no provider call, tell the user how to restore the surface or start fresh. Same `workspace`
surface with edits revoked ⇒ the spec keeps the method's declared shape but the call returns `denied`
with the remedy: turn the grant back on for future calls, or use the direct `edit`, which is unaffected.
Reads keep working and nothing is staged — never "no such tool in this session". If trust is withdrawn,
deny changes regardless of a surviving user opt-in. Test every cell, the edit-bit change, and the
revoked-grant message; keep old headers and wire declarations readable. The P0 narrowing applies
intentionally to legacy resumes too.

### 4.12 Descriptions, variants and pins

The edits variant exists **only** for `workspace + workspaceEdits`, exactly:

```text
Workspace JS. Await tools.read({path}|{next})→{lines:[{n,text}]}, tools.search({pattern})→{matches:[{path,line,text}]}, tools.ls({path})→{entries:[{name,kind}]}, tools.glob({pattern})→{paths}. Await tools.stageEdit({path,edits:[{old,new}]})→{staged,path,edits} to stage exact edits the host applies after this program returns; read the file first; staged is not written. Return the answer; a bare expression is null.
```

Measured with the same serializer shape as the production golden: **712 B including the trailing
newline** (711 marshaled), 56 B under the **768 B** cap that applies to this variant alone. The 576 B
and 640 B caps are unchanged and no read-only golden is regenerated for this.

- `internal/tools/codemode.go`: select the read-only, network and workspace-edits descriptions from the
  same capability inventory that feeds the prelude and dispatch. `stageEdit` appears in the edits
  variant's description, prelude and table only.
- Pins in `internal/tools`: `TestCodemodeWorkspaceEditsSpec` (exact golden `codemode-workspace-edits-schema.golden.json`,
  the 768 B cap, table/spec agreement) and one inventory table test over **all** variants — every method
  in a variant's capability table appears in that variant's description and every description name
  exists in its table — so a method added later cannot miss a variant. Existing read-only and refusal
  spec tests keep proving those sessions do not acquire the edit clause.
- Pin the **resolution** one layer up, in `internal/agent`: a resolved `workspace + workspaceEdits`
  session's spec contains `stageEdit`; `additive`, `transport` and read-only `workspace` do not;
  `network: true` gets the network text and never `stageEdit`. A table test over resolved configs, not
  only over specs built by hand.
- A resume with edits revoked keeps the edits-shaped spec (§4.11) — pinned in `internal/tools` and in
  the gate's resume leg.
- AGENTS' "eight frozen tools" sentence does **not** change: `workspace` declares eight plus the
  adapter, the same nine as additive, and nothing moves except inside the script.

## 5. Implementation sequence

Each behaviour PR updates `docs/specs/SPECS.md`, its byte mirror
`internal/skills/builtin/self-knowledge/references/specs.md`, and the AGENTS/SPECS counts its change
moves (`go test -tags counts ./cmd/moca -run TestSpecsTestCount -v` after changing the test count). Run
`go test -race -count=8` on each touched package before a phase PR, not only `-race` once. Phases 2–5
land together behind one branch; do not ship an intermediate build that can mutate without the release
guards.

0. **P0 — landed, `f0f0b06`.** Directory-only grants, transport-only grant configuration, five unit pins
   in `internal/tools/grants_test.go`, the binary-level `TestGrantScope`, SPECS §10.6/§15, the CHANGELOG
   behaviour change. Independent of everything below (§9). Remaining from the original phase 0: the catalogue
   repair described in §3.2 — the network description advertising two calls no capability table can
   dispatch — is a separate, still-open item; it cannot block containment now that containment shipped.
1. **Grader and instrumentation first.** Build `test/evals/changeset-pins/` (a manifest join) and
   `test/evals/changeset-registry/` (registered implementations), deterministic, both solvable without
   creating files. Grade exact bytes, untouched decoys, a requested successful verification **after the
   last mutation**, and the final answer. Define allowed scratch work; compute expected outputs
   independently outside the worktree. Reject global replace, a lost comment, a missing or extra target,
   verification too early, fake verification and prose-only completion; include a correct ordinary shell
   solution. `bugfix` and `reuse` are the controls.
   - Instrument the tagged work-turn boundary (`agent.turn`, before `Stream`) once per logical work
     request, including unanswered attempts at that level. Count physical provider attempts separately
     inside the adapter retry loop, tagged work vs auxiliary; count outer calls and nudges from
     `evalstats.Collect`, applied file operations from host metadata. Never sum these counters. Prove
     with scripted fixtures: two direct calls in one request, one script doing two inner operations, a
     retried request, a nudge with a subsequent work request, an auxiliary request, a failure with no
     assistant message, and forged guest success. Reject a missing observation distinctly from a
     measured zero.
   - Row schema: campaign/block/fixture/seed/arm, actual resolved config and digest, pass, logical work
     requests, assistant steps, provider attempts, applied/unknown counts, approvals, wall time,
     measured usage and cost. Keep actual usage and elapsed time for failures. Refuse schema/arm
     fallbacks, duplicate or missing pairs, missing spend and partial instrumentation. Add
     positive/negative decision fixtures; `bash test/evals/run.sh --selftest` before billing.
2. **Surface plumbing, read-only.** `internal/config/config.go` (`ResolveCodemode`, validation, layer
   rules: user opts in, trusted project may veto, project never widens), `internal/agent/start.go`
   (`planCodemode`, holder), `internal/agent/resume.go`, `internal/agent/prompt.go`,
   `internal/session/entry.go`, `internal/config/sample.jsonc`. Tests: strict decode rejects unknown
   keys; `workspace` without `enabled` and `workspaceEdits` without `workspace` are config errors;
   `network: true` with `workspace` is refused; project veto; legacy headers; the resume matrix of
   §4.11; provider wire names and order and off-prompt bytes unchanged; no VM opened when codemode is
   never called. The edit bit here is plumbing only — no writable method may become live before phase 3.
3. **Apply, consent, preview and journal together** (or keep writes unconditionally disabled until all
   four exist). Add `internal/tools/changeset.go` and tests; wire `tools.stageEdit` into
   `internal/codemode/prelude.js`, the adapter inventory/spec and `bridge.dispatch`. Implement §4's
   bounded stage machine, the rooted apply with `GuardImage` and the read-back, and the read-only
   recovery. Separate stamp capture, parity bookkeeping and grants. Share direct `edit`'s
   investigation/state semantics without recursively invoking `Registry.Run`. `internal/tools` does not
   import `session` or `permissions`; the snapshot capability and the journal arrive through `Env` as
   structural interfaces. Test both normal writes and a denied second batch that cannot reuse the first
   approval.
4. **Receipt and replay across clients.** Thread typed metadata through `tools.Result`,
   `internal/agent/events.go`, the result persistence path, `internal/tui/toolsummary.go`,
   `internal/tui/replay.go` and `internal/acp/toolcall.go`. Verify previews, parent IDs, consent options
   and real diff display in TUI and ACP. Leave the ordinary read-only zero-approval tests intact; add a
   separate staged-consent gate. Confirm a session rebuild produces one provider result and that
   metadata cannot turn replayed history into a new operation.
5. **Failure behaviour, in the real gate.** `test/codemodegate`: a real script over generated rows
   applies the expected bytes; one provider tool result; no intermediate sentinel in the next request;
   a direct green verification **after** the edits; a second process replays without editing; a killed
   child between two writes leaves a journal record; resume does not edit the workspace. Make the crash
   observable deterministically: a gate-only build-tagged synchronization hook stops the child after
   recording the first file and before starting the next; wait on its barrier, kill the child, resume
   twice with an ordinary binary, and inspect both disk and repaired transcript. No production kill
   switch. Inject separately before `Begin`, after the rename but before `Record`, and after `Finish`
   before the outer result. Exercise a stale target before consent, a changed target during consent, a
   failure on target two, steering, duplicate identity, text limits and preservation, wrong mode,
   untrusted, `-p` including yolo, and unknown outcome. Assert prefix/failed/suffix and counters, not
   only a summary line. Verify permission refusals and a successful apply in ACP, plus a direct edit and
   a real verification on the resulting state.
   - **`TestChangesetConsent` is the gate's tenth top-level test** and has two explicit paths. ACP
     subtests in a new `test/codemodegate/acp_client_test.go` (same build tag), adapting the stdio
     child-process loop at `test/acpgate/acpgate_test.go:63–150`, drive the built `moca acp` — never an
     in-process fake asker — and observe `session/request_permission`, its two options, the parent tool
     call and the bounded preview: ask and auto each ask once per nonempty batch, two batches ask twice,
     denial applies nothing and cannot reuse an earlier approval, yolo asks zero, plan refuses, an empty
     set asks zero. Headless subtests reuse the existing harness: ask, auto and plan refuse with zero
     writes and a paired tool result; `--mode yolo` applies the expected bytes and returns the receipt.
     Assert the provider-visible result and the filesystem, not unexported agent state. The ACP helper is
     test-only, with no product hook. `TestZeroApprovalsInEveryMode` stays read-only and unchanged.
   - Run `make vet && make test && make build`, `bash test/codemodegate/run.sh`, `bash test/acpgate/run.sh`,
     exercise TUI consent, and verify native Windows/macOS/Linux replacement and denial. No green here
     proves a user benefit.
6. **Feasibility before spend.** Compare the real binary/VM, ordinary direct/shell calls and an explicit
   exact-edit list through the same commit routine, starting each path from the **user's original
   information** — charging discovery, deriving target-specific values, sending the list or program,
   verification and actual responses. Do not give the list an oracle and do not charge each direct call
   a separate request. Use the same permission policy and production caps. If both fixtures finish at
   the same request floor without JS, stop: a hand-written trajectory is a feasibility check, not an
   adoption result.
7. **One separately approved campaign** (§6). Pass ⇒ README, CHANGELOG, SPECS, AGENTS and an opt-in
   example with measured scope. No promotion or inconclusive ⇒ keep the evidence, the P0 and the reusable
   instrumentation, and do not release mutations. Disabling the grant prevents future writes; it does
   not revert past edits.

## 6. The campaign

**Question (phase 7):** is opt-in changeset application worth shipping for these workloads, compared
with (A) moca as it ships and (B) the new reads without edits, measured through the existing two-prompt,
two-process `moca -p` trial path under `--mode yolo`? The campaign's spend is a separate approval before
any billed trial.

- **Arms, one tagged binary with the same consent machinery:** A = codemode off; B = `workspace`, reads
  only; C = B + `workspaceEdits`. `cmd/moca/pilot_arms_test.go` must pin that the arms differ **only**
  inside `codemode`, and `TestEvalConfigsPinTheCorpusModel` extends to the new arm configs — a typo
  there silently changes what is being measured. Every arm runs the same front end, mode, trust
  policy, direct tools and shell. C applies a valid changeset without a human question because that is the `-p --mode yolo` rule;
  path, stamp, journal, investigation and every other non-consent guard still apply. No arm has a
  changeset dialogue in this campaign.
- **Driver:** `test/evals/_lib/trial.sh:106–124` already supplies `--mode "${MODE:-ask}"` to both the
  initial process and its `--continue` process. The campaign sets `MODE=yolo` and reuses the tree setup,
  the two prompts, the transcript checks, the telemetry and one `-cap-scope` ledger. No ACP campaign
  driver and no second ledger.
- **What it does not measure:** interactive consent. `approvals` stays an observed value, never inferred
  from the mode; interactive ask/auto/yolo/plan behaviour is proved by §5's hermetic tests, not billed.
- **The control is strong, and the writing must say so.** In yolo, `shell` is auto-approved in **every**
  arm, so A and B have the most unconstrained non-JS path moca offers — the v2 pilot already had a
  shell-only arm solve the 120-service join. A C win therefore beats an unconstrained shell, not a
  hobbled control. That is the strongest claim this protocol can make, and it belongs in the
  interpretation, not in a reader's discovery. It also means the campaign measures the primitive, never
  consent.
- **Assignment:** 10 balanced random triples (one run each of A, B, C) per target fixture, 2 fixtures,
  and 10 triples on the 2 ordinary controls. Unique, reproducible assignments; count and report every
  run including failures and lost sessions, with run-to-run variance. No substitution and no
  mid-campaign arm changes. One `-cap-scope` ledger; the ledger alone is not a stop button — a
  provider-enforced ceiling plus an operator abort rule is.
- **Freeze before the first billed run:** binary digest, exact effective configs, prompts, seeds,
  checker, driver, model, effort, corpus permission budget, caps and every schema/limit in §4.
- **Endpoint.** Primary: logical work requests (the tagged work-turn count) until the checker first
  accepts, including work that fails; an incomplete or incorrect result scores at a predeclared cap (a
  real timeout is its own invalidity). Freeze it in phase 1; never choose it by sampling.
- **Statistics.** For each of the ten assigned pairs, a win means strictly fewer valid logical work
  requests for C. A numeric tie is not a win; missing or invalid data follows the invalidity rules. The
  ship bar requires **≥9 wins out of all ten assigned pairs** and a **median paired ratio ≤0.75** for
  C vs A and for C vs B, on **each** target fixture. This bar is not the v2 pilot's (≥8/10 and median
  ≤0.5): that campaign measured carry, this one measures logical work requests. Report exact two-sided binomial sign-test p-values
  conditional on the untied valid pairs, together with the number of ties — the bar is over all ten, so
  a tie cannot be dropped to reach 9/9.
  - **Dependence.** The four directional events are not independent: C's run feeds both comparisons in a
    triple. Under a global null the joint event is a subset of any one marginal, whose zero-tie tail is
    11/1024 ≈ 0.010742, so the joint probability is **at most 11/1024** before the ratio and
    precondition filters. State that bound; publish no invented exact joint probability and no
    independence assumption.
  - **Power, scoped to the directional component.** With independent trials at a true per-pair C-win
    probability of 0.75, P(≥9/10) = 0.244025; at 0.90 it is 0.736099. These are binomial calculations
    over pair outcomes, not power for a median ratio, and the full campaign's pass probability is lower
    still. So a genuine 25 % improvement will often be recorded as "underpowered, no promotion" — the
    correct conservative choice, stated before the spend. With fewer than nine directional pairs, record
    "underpowered, no promotion", never a directional claim.
- **Preconditions and vetoes.** On each target: C has ≥8/10 correct results with **host-confirmed**
  applied/unknown counts in every C success; any trial with missing pass data or missing/failed
  verification telemetry is invalid, and too many invalid rows invalidates the campaign. Zero
  unauthorized or unreported changes — stop immediately on any. Zero lost passes against A or B on any
  fixture. C's control-fixture median work requests ≤ A's + 1. C vs A median elapsed-time ratio ≤1.10
  and billed-cost ratio ≤1.10 from **persisted per-entry cost fields** (`session.Entry.Cost`, summed by
  `evalstats.Collect` and `agent.go`) — never an interpolated rate; unknown costs are missing data.
  Report input/output/cache token totals and retries per arm; carry only as an explanatory diagnostic.
  Report adoption: distinct C trials with ≥1 staged change, and success with ≥1 applied/unknown; analyze
  non-adopters as such, and never attribute C's win to shell use alone. Verification must actually run
  after the last mutation; a "verified" claim without evidence fails the checker. Count unknowns as
  failures for adoption but report them distinctly. Fixing the harness is a protocol change: rerun the
  whole campaign if any arm is affected.
- **Verdict, predeclared.** All conditions met ⇒ offer opt-in `workspaceEdits` (default off) for the
  measured class. Any precondition failed ⇒ no promotion; correct data that misses the endpoints ⇒ do
  not promote. Missing telemetry, a broken oracle, an exhausted ceiling or arm leakage ⇒ inconclusive,
  no promotion, and instrument repairs cannot be patched in afterward. Publish the protocol, the frozen
  configs, every row, exact two-sided p-values, paired ratios and representative traces in
  `test/evals/BASELINE.md`. Never re-run paid trials on the same fixtures to chase a different verdict.
- **Scope of the result.** A win supports unattended yolo changesets on the measured fixtures. It does
  not establish that interactive ask/auto consent is usable, desirable or worth promoting — that stays a
  separate product judgment backed by the non-billed client tests. If the owner instead chooses ACP
  `ask` sessions, this protocol is withdrawn whole: the bespoke ACP driver, interactive request
  measurement and consent-dialogue arms must return as one change. Do not mix the two protocols.

## 7. What not to build on this evidence

| Temptation | Why not now |
|---|---|
| Hide direct reads, or one script per turn | Earlier recorded rounds show step regressions, not savings. Keep direct reads and shell available; ordinary usage stays unchanged. |
| Wrap shell, MCP or HTTP in JS | Different permission and egress problems; MCP already has a proxy tool. Revisit read-only cross-server joins with a separate, user-approved plan. |
| Creation, deletion, rename, links or overlay | Widens races and review surface beyond the measured evidence; keep the fixture within the supported set. |
| Persistent JS state, worker pool, TypeScript or Bun | No measured workload requires them; cross-process state is rejected. |
| A carry-primary gate | Carry is a diagnostic; requests and cost decide. |
| Promote on the handwritten reference | Feasibility is not adoption. |
| Extend additive or transport grant scope for new features | Grants exist only in transport, and only for directories. |
| Retire or alias transport without a compatibility decision | Existing sessions and configs depend on it; removal is a separate owner decision with a migration path. |
| Reach `Guard` from the changeset | It reopens the absolute path; use `GuardImage` (§4.7) or refuse. |
| A second preview UI for consent | ctrl+o already pages `Detail`; label it and bound it (§4.10). |
| A `not_granted` refusal class | The existing `outside` answer is correct and already consumed. |

Creation, deletion, rename and secret handling remain out of scope. If a fixture or user task requires
them, stop and extend the plan first.

## 8. What would make this plan wrong

Rewrite, rather than patch, if any of these turns out true:

- A directory-only grant set cannot serve a real transport workflow, so file grants must come back —
  then the per-OS no-follow work returns: `os.Root` follows in-root symlinks by design, so exact-name
  matching on a parent root is not a no-follow open, and the portable check is the **opened descriptor's**
  identity against a trusted reference via `os.SameFile`, refusing where that cannot be established
  (Windows has no `O_NOFOLLOW`; `Lstat`-then-`Open` is racy; identity pinning goes stale when
  `config.writeEdited` rewrites the file atomically).
- `-p --mode yolo` is judged too permissive for a changeset even though it already writes — then the
  interactive protocol and its ACP driver become mandatory.
- Phase 6's feasibility check shows an explicit exact-edit list reaches the same request floor as a
  program (charging discovery for both).
- `os.Root.Rename` over an existing target proves unsupported on a supported platform in a way the
  typed refusal cannot cover gracefully.

## 9. Status, authority and the record

- **Landed:** the P0 containment fix, `f0f0b06` — directory-only grants, transport-only configuration,
  `internal/tools/grants_test.go` (five pins), the gate's `TestGrantScope` (nine top-level tests now),
  SPECS §10.6/§15, and the CHANGELOG note that scripted absolute reads outside the workspace now answer
  `outside` on **every** surface including additive, while the direct `read` still reads the jail's
  file roots. A plan note records this in rev 6.
- **Proposal, not approval:** everything else. Phases 1–7, every config key, every default, the
  descriptions, the caps, the campaign and all spend.
- **Owner decisions outstanding:** unattended `-p --mode yolo` changesets and the scoped campaign claim;
  the edits-only 768 B cap; the env-example suffixes; the campaign's amount, ceiling and abort authority.
- **Review record:** revisions 1–6 remain on disk. Rev 1 keeps the upstream comparison; rev 2 promoted
  the grant defect to P0; rev 3 rebuilt the contract; rev 4 replaced the grant design with
  directory-only grants and aligned consent with the modes; rev 5 fixed the campaign protocol, the
  golden budget, the statistics and the env-example collision; rev 6 added the revoked-grant remedy, the
  two-string consent surface, the yolo interpretation, the resolution pins and the P0 release note.
- **Risks if partially shipped without the guards:** partial application, double-apply on resume, dead
  undo entries, surprise consent dialogues, silent multi-file writes. Each is a release blocker.
- **Dependencies:** phases 1–5 are one branch gated together and need the P0 (landed) and a working
  oracle. Phase 6 needs phase 5's green gates. The campaign needs its own spend approval. Any change to
  fixtures, graders, frozen configs, thresholds, model or endpoint requires re-approval; a protocol fix
  reruns the affected arms.

# Changeset campaign (plan v7 phase 7) — what it measured

Status: **run. 70 valid runs across two sizes, $1.05. The changeset surface lost to the
status quo on every axis, at both sizes.**

## Design

Three arms per scenario, model pinned to `opencode-go/deepseek-v4.1-flash`, binary built with
`-tags codemodecarry` so every row carries the request counter:

- **cm-off** — control: codemode absent, the session's nine-tool surface.
- **cm-workspace** — read-only workspace (nine tools, `stageEdit` absent).
- **cm-workspace-edits** — writable workspace (`stageEdit` present, host applies after the program).

Ten runs per cell over `changeset-pins` (25 files, 20 targets) and `changeset-registry` (22 plugins,
18 pages), then five runs per cell at scale: `changeset-scale`, the same join as pins over 121
services / 124 files / 96 targets.

## Results

| size | scenario | arm | pass | steps | $/run |
|---|---|---|---|---|---|
| 25 files | pins | cm-off | **10/10** | **14.6** | **0.0101** |
| 25 files | pins | cm-workspace | 9/10 | 16.5 | 0.0115 |
| 25 files | pins | cm-workspace-edits | 10/10 | 17.3 | 0.0135 |
| 22 files | registry | cm-off | **10/10** | **17.3** | **0.0078** |
| 22 files | registry | cm-workspace | 9/10 | 17.3 | 0.0086 |
| 22 files | registry | cm-workspace-edits | 9/10 | 21.0 | 0.0102 |
| 124 files | scale | cm-off | **5/5** | **20.0** | **0.0344** |
| 124 files | scale | cm-workspace-edits | 2/5 | 31.6 | 0.0525 |

Aggregate: control 25/25 passing, codemode arms 39/50. Control cheaper per run at every size, and
the gap widens with size (50 % more expensive, 58 % more turns at 124 files).

## The failures are the surface's own rules

All five codemode failures are one of two things, and both are workflow, not capability:

- 4 × "`verify.py` ran, then codemode changed the tree" — the model staged a batch after its last
  verification and never re-verified.
- 1 × "no successful `verify.py` run in the transcript" — it never got to verification at all
  (exit 3).

No run failed because the staging API refused an edit or produced wrong bytes. The mechanism works;
the discipline it demands — stage everything, then verify once — is what the model does not reliably
keep.

## What this means

**The request-floor argument that motivated the surface does not survive contact with a model.**
The hand-written feasibility harness said codemode needed 2 requests where shell needed 3. A real
model needs ~15–20 requests either way, because it batches several `edit` calls per turn and never
pays the per-file turn cost the argument assumed. At 96 targets the control still wins: one shell
loop or a handful of batched edits beat a staged program on turns, tokens and dollars.

That is a decisive result about *placement*, not about the code: where a shell exists, codemode is
a more expensive way to do what the model already does well. Where a shell does not exist — the
no-execution deployment the workspace surface was built for — it is the only way to write many files
at once, and no eval can measure that against an alternative that isn't there.

## What the campaign cost and what it found besides the verdict

$1.05 for the valid runs; ~$1.0 was spent on three earlier attempts that were void because of harness
defects, all fixed and now covered:

1. `python3` was not in the shell allowlist, so no run could ever run the required verification.
2. The grader crashed on `content: null` (a turn that only calls tools), losing every failure reason.
3. The grader counted the legacy entry among untouched services, rejecting a correct answer.
4. `_lib/trial.sh` declared `local work` for the request counter, shadowing the trial's workdir:
   `cd 18` → the checker never ran, and 60 runs reported "no session".

A 0/N arm is a harness smell first and a result second; that is the lesson worth carrying forward.

## What is untested

- Whether a single clause in the edits description ("stage everything, then verify once, after the
  host applies") converts the four ordering failures. Five runs per arm would answer it for ~$0.25.
- Whether the surface wins where the alternative is a read-only session: that needs a scenario whose
  control arm genuinely cannot write, which no current fixture provides.

## Follow-up 1: the discipline clause (description only)

`stage every edit in this program; the host applies the batch when it returns, so verify after it
returns, not before.` — added to the frozen `stageEdit` description (still inside the 768 B variant
cap), five runs on `changeset-scale`, same binary tags, same model:

| arm | pass | turns | $/run |
|---|---|---|---|
| control (no codemode) | 5/5 | 20.0 | 0.0344 |
| codemode, before the clause | 2/5 | 31.6 | 0.0525 |
| codemode, after the clause | **4/5** | **27.6** | **0.0372** |

The clause fixed most of the ordering failures (one remains: verify, then stage more). Turns −13 %,
cost −29 %, and the arm is now within 8 % of the control on cost — but still behind it on pass rate
and turns. A description cannot make it cheaper than doing the work directly.

## Follow-up 2: the placement test (`changeset-noshell`)

The claim the surface could still make is not "cheaper than shell" but "the only multi-file write
path where nothing executes". New pilot scenario: 61 services, 46 targets, two arms, and **neither
arm may run a command** (`shell.allow` holds one no-op). The grader skips the verification rule for
this scenario by name — neither arm can verify, so grading on it would fail the control by
construction.

| arm | pass | turns | tokens | $/run |
|---|---|---|---|---|
| control: edit/write only | 5/5 | 28.2 | 1.06 M | 0.0348 |
| codemode: one staged program | **5/5** | **23.4** | **0.73 M** | **0.0272** |

**Codemode wins where there is no shell**: −17 % turns, −31 % tokens, −22 % cost, same 5/5. It is the
only arm in the whole campaign that beats the status quo on every axis, and it does so in exactly the
deployment the surface was designed for.

## The two results together

The surface's value is conditional and now measured in both directions:

- **With a shell available**: it is a slower, more expensive way to do what the model already does
  well (25/25 control vs 39/50 codemode across sizes).
- **Without a shell**: it is the cheapest correct way to write many files at once (5/5, −22 % cost).

That is a placement argument, not a performance one, and it is the argument the feature should ship
on: the writable workspace is for sessions that cannot execute — sandboxed, locked-down, or
review-only deployments — not for replacing `edit` in sessions that can.

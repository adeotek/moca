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

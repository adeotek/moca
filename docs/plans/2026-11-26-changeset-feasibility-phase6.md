# Changeset feasibility (plan v7 phase 6) — revision 3, measured on all three routes

Status: **all three routes measured; the codemode route passes on both fixtures. No billed trial has
run.**

## What the fixtures demand

Both fixtures previously let a global replace do the work: the target list was the data file's keys and
every written value was a value already written in that file. They now fail that way.

**`changeset-pins`** — the rollout is *discovered*: a service is a target only if it appears in
`stages[rollout.stage]`, has a release, **and** its own `service.yaml` declares `replicas >= 2`. The
written tag is `version + "-" + sha256("salt:name:replicas")[:7]` — a function of the target's own
content — each service gains a `pin:` block whose weight is ten times its replicas, and each manifest
entry's `releaseId` is `sha256("name:tag")[:10]`. 4 decoys are released but out of stage, 1 is in stage
with one replica, and the legacy entry must not move. A `sed` that rewrites every `image:` line now
changes 8 files it must not touch and writes no pin block.

**`changeset-registry`** — the target set is the pages that *exist* under `capabilities/`, filtered by
`data.json`: an enabled plugin with a page is regenerated with `Digest: sha256("name|kind|endpoint")[:8]`
and a registry row of `kind@sha256(...)[:6]`; a disabled plugin's page stays byte-identical; an enabled
plugin with no page is not created (2 such entries exist, so a generator iterating `data.json` fails).
4 disabled pages are decoys.

Both `verify.py` scripts re-derive the expected values independently at run time and fail on the
pristine tree: `20 rollout services agree in both files`, `18 installed plugins agree in the registry`.

## Measured (`bash test/feasibility/run.sh`)

The harness copies each fixture, runs the route's programme, and grades every byte: each written file
must equal the reference image, every other file must be byte-identical to the fixture, and nothing may
appear or vanish. The codemode row is executed for real — the built binary, a scripted provider, one
workspace script, then the fixture's own `verify.py`. No model, no network, no spend.

| scenario | route | work requests | artefact | result |
|---|---|---|---|---|
| changeset-pins | shell | 3 | 1466 B | pass |
| changeset-pins | direct edits | 3 (40 pairs) | 1497 B | pass |
| changeset-pins | **codemode** | **2** | 2197 B | **pass** |
| changeset-registry | shell | 3 | 1784 B | pass |
| changeset-registry | direct edits | 3 (19 pairs) | 1413 B | pass |
| changeset-registry | **codemode** | **2** | 1453 B | **pass** |

## What this establishes

The codemode route is the cheapest route that passes: two work requests, one script, no shell, on both
fixtures. The shell route ties on correctness at a third request (discovery, apply, verify), and the
direct-edit route ties on requests only by batching 40 or 19 hand-written pairs into one turn. So the
changeset surface is not paying for itself in request count — it is paying for itself in *what one turn
has to contain*: 2.2 KB of program versus 40 exact edits or a 1.5 KB shell script that must be written
correctly blind.

## Two defects this measurement found, both fixed

1. **The guest had no hash function.** The sandbox exposed `read`, `search`, `ls`, `glob` and
   `stageEdit`, and nothing to derive a value from content with. Both fixtures are hash-derived
   *because* the value cannot be copied out of the data file, so the route was inexpressible in any
   number of requests. Fixed with a pure-JS SHA-256 in the prelude — no ABI change, no capability-table
   row, no host reach — pinned against FIPS vectors inside the real reactor.
2. **The pins fixture shipped the answer.** Its manifest was generated with the already-rotated tags,
   so the manifest half of the task needed no work and the codemode script's rewrite was a legitimate
   no-op (which the host correctly refused as "does not change anything"). The fixture now ships
   release versions and the reference overlay holds the rotated ones.

## What is measured and what is not

- Measured: every route's pass/fail on real bytes, the request count per route, the artefact each route
  must produce, pristine-fails/reference-passes for both `verify.py`, twelve grader arms green.
- Not measured: model behaviour. The routes are hand-written, so this establishes feasibility and cost
  per route, not how a model fares on them. That is the campaign's question, and it is now the only
  open one.

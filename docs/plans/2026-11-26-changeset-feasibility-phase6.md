# Changeset feasibility (plan v7 phase 6) — revision 2, after the fixture rewrite

Status: **fixtures rewritten; floor analysis re-run. No billed trial has run.**

## What changed in the fixtures

Both fixtures previously let a global replace do the work: the target list was the data file's keys and
every written value was a value already written in that file. They now fail that way.

**`changeset-pins`** — the rollout is *discovered*: a service is a target only if it appears in
`stages[rollout.stage]`, has a release, **and** its own `service.yaml` declares `replicas >= 2`. The
written tag is `version + "-" + sha1("salt:name:replicas")[:7]` — a function of the target's own
content — each service gains a `pin:` block whose weight is ten times its replicas, and each manifest
entry's `releaseId` is `sha1("name:tag")[:10]`. 4 decoys are released but out of stage, 1 is in stage
with one replica, and the legacy entry must not move. A `sed` that rewrites every `image:` line now
changes 8 files it must not touch and writes no pin block.

**`changeset-registry`** — the target set is the pages that *exist* under `capabilities/`, filtered by
`data.json`: an enabled plugin with a page is regenerated with `Digest: sha1("name|kind|endpoint")[:8]`
and a registry row of `kind@sha1(...)[:6]`; a disabled plugin's page stays byte-identical; an enabled
plugin with no page is not created (2 such entries exist, so a generator iterating `data.json` fails).
4 disabled pages are decoys.

Both `verify.py` scripts re-derive the expected values independently at run time and fail on the
pristine tree: `20 rollout services agree in both files`, `18 installed plugins agree in the registry`.

## The three routes, minimum requests (re-derived)

| Route | pins | registry | Why |
|---|---|---|---|
| Ordinary shell | 3 | 3 | one scripted discovery+apply (loop + hash + conditional block), one `verify.py` — **if written correctly in one go** |
| Direct `edit` calls | 22 | 20 | discovery, one exact edit per target, verify |
| `codemode` + `stageEdit` | 3 | 3 | one program reads, derives and stages every target, host applies, verify |

## Verdict under the plan's rule

The rule is "if both fixtures finish at the same request floor without JS, stop". Read literally, the
answer is **still stop**: a competent hand-written shell script reaches three requests, and the
changeset cannot beat three. What the rewrite changed is not the floor but the *capability gradient* —
a naive `sed`/generator now fails the grader on decoys, missing derivations and missing blocks, so the
shell route's cost shows up in **attempts and pass rate**, not in request count.

That makes the literal rule the wrong gate for these fixtures. The measurement that would actually
decide is the one phase 6 called for and never ran: a hand-written trajectory per route through the
same commit routine, charging discovery, derivation, verification and each actual response — and
recording not just the floor but whether the programme written in turn one was *correct*.

## Recommendation

Run that trajectory measurement (scripted, no model spend beyond the fixtures' own verification) on the
rewritten fixtures, and only then decide on a campaign. If the shell route still passes and finishes in
three requests when written blind, the campaign's thesis is answered without spending a cent: JS is a
preference, not a lever.

## What is measured and what is not

- Measured: target counts and decoys from the fixtures, pristine-fails/reference-passes for both
  `verify.py`, and the twelve grader arms (including the shell-solution arm) green.
- Not measured: the three routes' actual request counts and pass rates. Those need the trajectory run.

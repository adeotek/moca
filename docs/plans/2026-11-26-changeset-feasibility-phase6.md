# Changeset feasibility (plan v7 phase 6) — the request floor, measured from the fixtures

Status: **analysis done, trajectory measurement not run.** This records what the two fixtures
demand of each route and applies the plan's own stopping rule before any billed campaign. It is a
feasibility check, not an adoption result (§5 item 6).

## The rule, predeclared

> If both fixtures finish at the same request floor without JS, stop: a hand-written trajectory is a
> feasibility check, not an adoption result.

"Request" is one logical work request — one provider turn that asks for tool calls — not one tool
call. Discovery, the writes and the verification all charge the same clock.

## What each fixture needs

`changeset-pins`: 24 `service.yaml` files, 20 released versions to land in 21 targets
(`deploy/manifest.yaml` plus the per-service files), 4 decoy services and 1 legacy entry that must
not move. Every new version is a value in `releases.json`, so no target-specific knowledge is needed
beyond the join.

`changeset-registry`: `plugins/data.json` drives 18 enabled plugin pages plus one regenerated
`plugins/registry.go`; 4 disabled decoys. The registry image is gofmt-aligned, which a naive
generator gets wrong — but a shell `gofmt` call fixes it.

## The three routes, minimum requests

| Route | pins | registry | How it gets there |
|---|---|---|---|
| Ordinary shell | 3 | 3 | one `cat`/`ls` to learn the shape, one scripted `sed`/`awk`+`gofmt` pass, one `verify.py` |
| Direct `edit` calls | 22 | 20 | one discovery read (or a `search` per file), one exact `edit` per target, one `verify.py` — 21 and 19 edits respectively |
| `codemode` + `stageEdit` | 3 | 3 | one globbing program that reads and stages every target, the host apply, one `verify.py` |

Shell and changeset tie at three requests on both fixtures. The changeset route's third request is
the verification run; the shell route's is the same run. The direct-edit route is strictly worse than
both on request count and only becomes competitive if a model can batch several `edit` calls into one
turn — which it can, and which is exactly why the pilot's own `steps` column, not a per-call count,
is the metric the campaign uses.

## Verdict under the rule

**Stop before campaigning on these fixtures as written.** Both are shell-solvable in the same number
of requests a changeset needs, so a campaign on them would measure the model's willingness to use JS,
not whether JS pays. The fixtures were built to grade *correctness* of a bounded many-file edit (phase
1), and they do that well; they were not built to make the shell route expensive.

## What a campaign would need first

A fixture where the shell route is genuinely worse, and the cost is in *deriving values*, not in
writing them:

- the new value differs per target and is a function of its content (a digest, a hash of a template,
  a version bump conditional on a per-file field), so a global `sed` cannot express it;
- the target set is discovered, not listed (a glob plus a filter over file contents), so a
  hand-written list is an oracle;
- at least one target needs a derived name (a rename or a generated block), where a shell one-liner
  stops being one line.

`changeset-pins` already has the join; it needs a per-target transform. `changeset-registry` has the
derivation; it needs the target set to stop being `data.json`'s keys. Neither change is large, but
neither is in scope for phases 1–5, and the plan forbids rewriting a fixture after seeing its result.

## What is measured and what is not

- Measured: the fixtures' target counts and decoys (from `plugins/data.json` and `releases.json`),
  and that `verify.py` fails on the pristine tree and passes on the reference overlay.
- Not measured: the actual request counts of the three routes. They are analytic floors from the
  fixture shape. A real trajectory run is the campaign's own first step, and the campaign is not
  approved.

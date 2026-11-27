# Codemode P0 — `stageEdit` returns the post-edit content

## OUTCOME (measured 2026-11-27, commit `99526e5`)

**Verdict: FAILS its predeclared gate. P1/P2 are not built.** The change ships as implemented (it is
strictly more information for the guest, no new authority, all gates green), but it did not earn the
next phase.

| read | base (n=8) | cand (n=13) | ratio [CI] | criterion | result |
|---|---|---|---|---|---|
| passes | 7/8 | **13/13** | — | ≥ base | **pass** |
| tokens/solved | 1.44 M | 1.35 M | 0.82 [0.61, 1.18] | hi < 1.00 | **FAIL** |
| cost/run | $0.0475 | $0.0443 | 0.93 [0.72, 1.30] | (informative) | — |
| steps | 30.2 | 29.9 | 0.99 [0.83, 1.21] | hi ≤ 1.00 | **FAIL** |

`evalstats` verdict: **underpowered — CI [0.61, 1.18] spans the band, need n 126** per arm (cv 32 %).

What the numbers say, without the spin: the point estimates all lean the right way (18 % fewer
tokens/solved, 7 % cheaper, 1 % shorter) and the arm went from 7/8 to 13/13 — the one base failure was
`svc-053`, a content error, which is exactly the class the returned content lets a program catch. But a
one-run difference cannot carry a claim, and the cost/turn effects sit inside their intervals. At
~$0.044/run, pinning this to `need n` would cost ≈ $11 for the pair — over this phase's $5 cap and
never approved.

**Decision recorded: do not build P1 (`writeFile`/`rename`/`delete`) or P2 (`exec`) on the strength of
this.** The surface is at its measured ceiling for this model on shell-present workloads; its real win
remains the placement one from phase 7 (`changeset-noshell`, pinned: 0.65 tokens/solved [0.55, 0.77],
13/13 vs 12/13). Do not re-open P0 with a bigger n unless someone is prepared to spend the $11 and
accept that the plausible outcome is "cost is a wash and correctness improves marginally".

Total phase spend: **$0.58** (13 runs) against a $5 cap; campaign ledger to date $4.34.


**Goal:** let a codemode program check the bytes it is about to write, so verification happens inside
the program instead of in a later turn through a shell.

**Architecture:** `Changeset.Stage` already computes the post-edit image (`post`) to run its growth
and no-op checks; this returns that image to the guest in the staging result, with a digest always and
the text up to a cap. No new tool, no new authority, no change to the apply path.

**Tech Stack:** Go (`internal/tools`), the JS prelude (`internal/codemode`), the eval harness
(`test/evals`), the hermetic binary gate (`test/codemodegate`).

---

## Status

- **Premises measured 2026-11-27** against `cab1e5f` on `feat/codemode-v3`.
- **Not approved for implementation.** This document is the plan the user asked for; code changes
  begin only after an explicit go, and the billed eval step re-asks with its amount.
- Phase 7's campaign result is the reason this exists: where a shell exists the surface loses
  (`changeset-scale` 7/8 @ 30.2 turns, $0.0475 vs control 5/5 @ 20.0, $0.0344), where none exists it
  wins (`changeset-noshell` 13/13 @ 23.4, $0.0278 vs 12/13 @ 28.2, $0.0334, pinned). Every surviving
  codemode failure in that campaign was a verification-ordering problem or a content error — both are
  things a program could catch for itself if it could see what it staged.

## Premises verified in the tree (do not trust these from memory; re-grep at implementation)

| claim | evidence |
|---|---|
| staging already computes the post-edit image | `internal/tools/changeset.go:152` `Stage` → `post, textBytes, err := applyEdits(...)`; `ChangesetStage{Pre, Post, ...}` at the same function |
| the result the guest receives is three fields | `internal/tools/changeset.go:244` `staged(rel, edits)` → `{"staged":true,"path":rel,"edits":edits}`; the restage branch returns the same helper |
| the guest reads that JSON verbatim | `internal/codemode/prelude.js:96` `inspect()` returns `r.value` from the host call; `:191` wires `tools.stageEdit` to it |
| description is 578 B with newline, cap 768 | probe run this session; cap enforced at `internal/tools/changeset_spec_test.go:38` |
| limits | `changeset.go:28-35`: 32 targets, 512 KiB result file, 32 KiB edit text per target, 256 KiB text per changeset, 2 MiB retained images |
| the binary gate has scripted legs to extend | `test/codemodegate/codemodegate_test.go` (`TestWorkspaceScript` at `:235`, `TestCapabilityAllowlist` at `:286`) |

## Design

Result of a successful stage:

```json
{"staged": true, "path": "services/x/service.yaml", "edits": 2,
 "bytes": 412, "sha256": "9f2c…", "content": "name: x\n…", "truncated": false}
```

- `content` — the file **as it will be after apply**, exactly the bytes `Post` holds.
- `sha256` — hex digest of the full post image, always present, so a program can verify identity even
  when the text is truncated.
- `bytes` — `len(post)`.
- `truncated` — true when `len(post) > maxStageContent`; `content` is then a prefix and `sha256` is
  still the digest of the **whole** image.

`maxStageContent = 64 << 10` (new const beside the other limits). Rationale: 32 targets × 64 KiB is
2 MiB of guest strings in the worst case, equal to `maxRetainedImages`, and a 64 KiB window is enough
to assert a pin block, a header or a tail. Full files are already limited to 512 KiB by
`maxChangesetFileBytes`, so truncation is the exception, not the rule.

**Non-goals (deliberately not in this plan):** `tools.writeFile`, `tools.rename`, `tools.delete`,
`tools.exec`. Those are P1/P2 in the campaign report and are **gated on this phase's measurement**:
if returning content does not move the scale arm, the surface has hit its ceiling and the wider
primitives are not worth their description bytes and consent legs.

## Predeclared measurement (decides P0; write it before running anything)

Two arms, same model (`opencode-go/deepseek-v4.1-flash`, pinned by `TestEvalConfigsPinTheCorpusModel` in `cmd/moca/pilot_arms_test.go:114`),
same scenario (`changeset-scale`, no-shell is not the target here — the failure being fixed is the
shell-verification ordering one).

- **base** = `campaign.p7h-scale-edits.jsonl` (n=8, already measured: 7/8, 30.2 turns, $0.0475/run).
- **cand** = same arm, same config, new binary, label `campaign.p8-content`, **n=13**.

Read: `go run ./test/evals/evalstats -compare <base>,<cand>`.

**P0 passes iff all three hold:**

1. passes(cand) ≥ passes(base) — no regression, i.e. ≥ 7 of 8-equivalent rate, and at 13 runs ≥ 11.
2. tokens/solved upper bound < 1.00 (the change actually buys something, not just claims to).
3. `steps` upper bound ≤ 1.00.

**If it fails:** stop. Do not build P1/P2, and record the negative result in this plan's outcome block
and in `docs/plans/2026-11-27-changeset-campaign-phase7.md`. A pricey primitive that does not move the
arm that already loses is not made worth it by adding more primitives.

`need n` from the compare decides whether 13 was enough; if it reports a larger n, one top-up run per
arm is allowed (≈$0.10), not a new campaign. Spend ceiling for this phase: **$5**, ledger
`test/evals/results/campaign.*.jsonl` via `-cap-scope`.

## Tasks

### Task 1: a unit test for the content field (red first)

**Files:** modify `internal/tools/changeset_test.go` (beside `TestChangesetStageAppliesOrderedPairs`
at `:134`). The package's existing helpers are `newBatch(t, files) (dir, *Changeset)`,
`stageCall(t, c, path, [2]string{old,new}...) (string, error)` and `mustJSON(t, v)`; use them, do not
invent a constructor.

```go
func TestChangesetStageReturnsPostEditContent(t *testing.T) {
	_, c := newBatch(t, map[string]string{"svc.yaml": "name: x\nreplicas: 2\n"})
	defer c.Close()
	out, err := stageCall(t, c, "svc.yaml", [2]string{"replicas: 2", "replicas: 3"})
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	var got struct {
		Staged    bool   `json:"staged"`
		Content   string `json:"content"`
		SHA256    string `json:"sha256"`
		Bytes     int    `json:"bytes"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("result is not json: %v (%s)", err, out)
	}
	want := "name: x\nreplicas: 3\n"
	if got.Content != want {
		t.Fatalf("content = %q, want %q", got.Content, want)
	}
	if got.Bytes != len(want) || got.Truncated {
		t.Fatalf("bytes=%d truncated=%v for a small file", got.Bytes, got.Truncated)
	}
	sum := sha256.Sum256([]byte(want))
	if got.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 does not match the content")
	}
}
```

**Verify red:** `go test ./internal/tools -run TestChangesetStageReturnsPostEditContent -count=1` →
expected FAIL (`content` absent from the result).

### Task 2: return the image

**Files:** modify `internal/tools/changeset.go` — add `maxStageContent = 64 << 10` to the limit block
at `:28`, add `"encoding/hex"` to the imports (it is **not** imported today; `crypto/sha256` is),
replace `staged(rel, edits)` with `stagedResult(rel, edits, post)`, and make the restage branch use the
same helper (both branches currently call `staged`). `post` is already in scope at both return points.

```go
func stagedResult(rel string, edits int, post []byte) json.RawMessage {
	sum := sha256.Sum256(post)
	body := post
	truncated := false
	if len(body) > maxStageContent {
		body, truncated = body[:maxStageContent], true
	}
	out, _ := json.Marshal(map[string]any{
		"staged": true, "path": rel, "edits": edits,
		"bytes": len(post), "sha256": hex.EncodeToString(sum[:]),
		"content": string(body), "truncated": truncated,
	})
	return out
}
```

**Verify green:** the same command → PASS, and `go build ./...` (the new import must compile). Then `go test ./internal/tools -count=1` (whole package),
and the consent matrix, which lives one layer up:
`go test -tags codemodegate ./test/codemodegate -run TestChangesetConsent -count=1`
(`test/codemodegate/codemodegate_test.go:500`) — no new authority, so it must be untouched.

### Task 3: truncation is honest

**Files:** same test file.

Stage an edit to a file larger than 64 KiB (generate it in the test), assert `truncated == true`,
`bytes == len(post)`, `len(content) == 64<<10`, and that `sha256` equals the digest of the **full**
post image (recompute it in the test from the file plus the edit).

**Verify:** `go test ./internal/tools -run TestChangesetStageTruncatesLargeImage -count=1` → PASS.

### Task 4: the description says what it returns

**Files:** `internal/tools/codemode.go` (the string at `:201`, doc comment at `:192`) and the frozen
copy in `internal/tools/changeset_spec_test.go:5-40`.

Change `→{staged,path,edits}` to `→{staged,path,edits,content,sha256}` and add one clause after the
ordering clause: `the result carries the file as it will be — check it there, not after apply.`

**Verify:** `go test ./internal/tools -run TestCodemodeSpecByResolvedSurface -count=1` → PASS, and
record the new byte count. Cap is 768 (`changeset_spec_test.go:38`); current 578 + ~90 must stay
under it. If it does not fit, drop the wording, not the field — the field is the feature.

### Task 5: end-to-end proof that the content is the true image

**Files:** `test/codemodegate/codemodegate_test.go` (new leg beside `TestWorkspaceScript`).

The script stages one exact edit and returns `{reported: <sha256 from the stage result>}`; the leg
applies the changeset, reads the file from disk and asserts
`sha256(disk) == reported`. This is the only test that proves the host's `content`/`sha256` equal the
bytes that land, rather than the bytes the edit engine intended.

**Verify:** `make build && bash test/codemodegate/run.sh` → all legs green, including the new one.
Mutation check per the repo rule: temporarily return `pre` instead of `post` in `stagedResult` and
confirm this leg fails; restore.

### Task 6: docs and pins

- `docs/specs/SPECS.md` §15: the result shape (`{staged,path,edits,bytes,sha256,content,truncated}`),
  the 64 KiB window, the digest-over-full-image rule, and the new description byte count.
- `cp docs/specs/SPECS.md internal/skills/builtin/self-knowledge/references/specs.md` (the mirror test
  at `internal/skills/sync_test.go:36` fails otherwise).
- Count pins: `go test -tags counts ./cmd/moca -run TestSpecsTestCount -v` and the Go-file count in
  AGENTS.md if any file was added.

**Verify:** `make vet` clean, `go test ./... -race -count=1` green, counts green.

### Task 7: the measurement (billed — ask before running)

1. `go build -tags codemodecarry -trimpath -ldflags "-s -w -X github.com/adeotek/moca/internal/config.Version=campaign" -o bin/moca ./cmd/moca`
2. `bash test/evals/run.sh -n 13 -j 3 -l campaign.p8-content -c cm-workspace-edits -cap 5 -cap-scope "$PWD/test/evals/results/campaign.*.jsonl" changeset-scale`
3. `go run ./test/evals/evalstats -compare test/evals/results/campaign.p7h-scale-edits.jsonl,test/evals/results/campaign.p8-content.jsonl`

Write the OUTCOME block under this plan's title with the three numbers and the verdict.

### Task 8: commit

One commit for tasks 1–6 (tests + implementation + docs), a second for the measurement record. House
title, e.g. `[feat:core] codemode changesets: the staged content, so a program can check its own work`.
No version bump (release-time only).

## Risks and what each one costs

- **The model ignores `content`.** Then the eval shows no movement and P0 is a null result — which the
  predeclared gate catches instead of us rationalising it. The description clause is the only lever
  allowed; no prompt tuning beyond it in this phase.
- **Guest memory.** 32 targets × 64 KiB is 2 MiB of strings; if the wasm heap objects, drop
  `maxStageContent` to 16 KiB (a pin block is ~80 B; 16 KiB is still 200× a pin).
- **Transcript growth.** `content` must stay out of the session transcript: the codemode tool result
  carries the program's return value and the receipt, not individual call results. Assert it in
  Task 5's leg by grepping the session file for a marker string the test wrote into a file.
- **Cache cost.** The description grows for every codemode-edits user (~90 B on every request, one
  cache reset). That is the price of the phase; it is paid once, at the same time as the clause from
  phase 7.

## What comes after, and what would cancel it

P1 (`writeFile`, `rename`, `delete`) and P2 (`exec` under the existing shell allowlist) are only worth
planning if P0 passes its gate. If P0 fails, the honest conclusion is that the writable workspace is
already at its ceiling for this model and the remaining budget belongs somewhere else — write that
down where the next reader will find it, in the campaign report's outcome section.

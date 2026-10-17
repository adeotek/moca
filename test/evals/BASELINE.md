# Eval baseline — rev 21 harness pass

Recorded 2026-10-09 with `opencode-go/glm-5.3-flash` (`test/evals/config.jsonc`), 3 runs per scenario
(`vague`: 6 — its pass rate swings the most), `bash test/evals/run.sh -n 3 -j 4`.

- **baseline** — the harness before rev 21 (`chore/polish` at ffe4244 + the branch-review fixes).
- **final** — rev 21: overflow files, the 400-line read window, elision, the cache-stable plan envelope,
  the verification nudge, the repeated-failure stop, the empty-reply retry, the investigation-protocol
  scope fix, the project-checks line and the rewritten working style.

Re-run after any harness or prompt change and compare: `go run ./test/evals/evalstats -summary <results.jsonl>`.

## Results

| label | scenario | runs | pass | tokens (mean ± sd) | steps (mean ± sd) | cost $ | tokens/solved | repeated fails |
|---|---|---|---|---|---|---|---|---|
| baseline | bugfix | 3 | 3/3 | 83438 ± 43778 | 18.0 ± 7.8 | 0.0259 | 83438 | 1 |
| baseline | feature | 3 | 3/3 | 44303 ± 17677 | 11.3 ± 3.3 | 0.0203 | 44303 | 0 |
| baseline | largefile | 3 | 3/3 | 102173 ± 47646 | 8.7 ± 0.9 | 0.0214 | 102173 | 0 |
| baseline | plan | 3 | 3/3 | 14859 ± 1417 | 4.7 ± 0.5 | 0.0079 | 14859 | 0 |
| baseline | refactor | 3 | 3/3 | 41809 ± 7128 | 9.3 ± 1.2 | 0.0128 | 41809 | 0 |
| baseline | vague | 6 | 2/6 | 83688 ± 87836 | 15.8 ± 12.2 | 0.0483 | 251066 | 2 |
| baseline | web | 3 | 3/3 | 14277 ± 10012 | 3.0 ± 0.0 | 0.0055 | 14277 | 0 |
| **baseline** | **∑ all** | 24 | **20/24** | **58530 ± 59394** | **10.8 ± 8.5** | 0.1419 | **70235** | **3** |
| final | bugfix | 3 | 3/3 | 32635 ± 7038 | 8.3 ± 1.7 | 0.0144 | 32635 | 0 |
| final | feature | 3 | 2/3 | 30724 ± 5921 | 8.3 ± 1.2 | 0.0135 | 46086 | 0 |
| final | largefile | 3 | 3/3 | 45179 ± 12808 | 9.3 ± 1.2 | 0.0133 | 45179 | 0 |
| final | plan | 3 | 3/3 | 35997 ± 25185 | 7.7 ± 3.8 | 0.0111 | 35997 | 0 |
| final | refactor | 3 | 3/3 | 74112 ± 33941 | 14.7 ± 5.2 | 0.0168 | 74112 | 0 |
| final | vague | 6 | 3/6 | 70813 ± 55799 | 13.8 ± 7.9 | 0.0457 | 141627 | 0 |
| final | web | 3 | 3/3 | 32894 ± 8769 | 3.7 ± 0.5 | 0.0101 | 32894 | 0 |
| **final** | **∑ all** | 24 | **20/24** | **49146 ± 37023** | **10.0 ± 5.9** | 0.1250 | **58975** | **0** |
| v0.5.2 | bugfix | 3 | 2/3 | 150136 ± 154708 | 19.7 ± 15.1 | 0.0229 | 225204 | 2 |
| v0.5.2 | feature | 3 | 3/3 | 35179 ± 9231 | 8.3 ± 2.1 | 0.0073 | 35179 | 0 |
| v0.5.2 | largefile | 3 | 3/3 | 78044 ± 9956 | 9.7 ± 0.9 | 0.0131 | 78044 | 0 |
| v0.5.2 | plan | 3 | 3/3 | 19515 ± 1785 | 4.7 ± 0.5 | 0.0068 | 19515 | 0 |
| v0.5.2 | refactor | 3 | 2/3 | 41229 ± 25101 | 8.0 ± 4.3 | 0.0100 | 61844 | 0 |
| v0.5.2 | vague | 3 | 1/3 | 35541 ± 15864 | 7.7 ± 2.5 | 0.0113 | 106624 | 0 |
| v0.5.2 | web | 3 | 3/3 | 22749 ± 9071 | 3.0 ± 0.0 | 0.0083 | 22749 | 0 |
| **v0.5.2** | **∑ all** | 21 | **17/21** | **54628 ± 73608** | **8.7 ± 7.8** | 0.0796 | **67481** | **2** |

Per run: tool errors 1.96 → 1.29; shell calls 3.0 → 2.3; cache-read share of input 49% → 46%.

## Reading it

- **Overall**: same pass rate (20/24 — one final failure, `feature#3`, was a pre-existing crash in the
  edit diff, fixed afterwards), **−16 % tokens per run and per solved task, −38 % token sd, −31 % step sd,
  repeated identical failures 3 → 0**, −12 % cost.
- **largefile −56 %** — the 400-line read window: the baseline read the whole 1,423-line file and
  re-sent it every step. **bugfix −61 %, 18 → 8 steps** — the investigation protocol no longer locks
  `edit` out after the run's own build break, and failing edits no longer detour through `sed -i`/`python3`.
- **vague** stays hard for this model (2/6 → 3/6). Failures moved from "no domain validation at all"
  (baseline) to buggy domain validation (double conversion, a `fatal()` that never exits, broken
  negative input) — the prompt now steers to the right checks; execution is the model's limit. One run
  claimed a rejection its own output contradicted → the prompt now asks to report only observed results.
- **Higher, explained**: `plan` (+142 %) is one run that researched the web (7 fetches); the other two
  match the baseline. `refactor` (+77 %) is one 22-step run; the others are +10 %. `web` (+130 %): the
  baseline twice picked a tiny plain-text URL; on the large page, 2 of 3 final runs read the overflow
  file after the 20K cut — a real cost of the smaller web cap for single-page lookups (a candidate to
  revisit: a larger web cap, or a note that the head usually suffices).
- **Noise**: 3 runs per scenario on a cheap model — single runs swing a scenario by ±50 %. Trust the ∑
  rows and the variance figures; re-run a scenario with `-n 6` before acting on one cell.

## v0.5.2-beta delta (2026-10-16 hardening pass)

Recorded 2026-10-16 with the same config/model (`opencode-go/glm-5.3-flash`), `-n 3 -j 4`, label
`v0.5.2` (21 runs — three per scenario; the rev-21 table's `vague` rows used 6). The change set:
the working-style prompt (time-boxed recon, falsifiable verification, no fabricated data, copy
stateful files before probing), the 12-batch progress nudge, and the glm flash effort fix.

- **Overall 17/21 (81 %) vs 20/24 (83 %)** — same rate within noise. Steps per run 10.0 → 8.7
  (−13 %), tokens per run +11 % (49146 → 54628), tokens per solved task 58975 → 67481 (+14 %),
  cost per run $0.0052 → $0.0038.
- **The floor moved in the runs where the model behaves**: `feature` 8.3 steps (was 8.3),
  `plan` 4.7 (was 7.7, −39 %), `refactor` 8.0 (was 14.7, −46 %), `largefile` 9.7 (was 9.3),
  `web` 3.0 (was 3.7). The three failures are not nudge-shaped:
  - `vague#1`/`vague#2` — the rev-21 failure mode repeats verbatim (`accepted invalid input:
    -to f -- -300`): the model's own domain validation is wrong, no amount of prompt fixes that
    (rev-21 `final` got 3/6; one run here passed in 11 steps).
  - `bugfix#2` — 41 steps, 9 edits, then the repeated-failure stop (exit 3): the run *acted* early,
    so the progress nudge correctly never fired; it thrashed on edits that did not fix `TestLimit`.
  - `refactor#3` — died after 2 steps on a mid-stream `[rate_limit_error] upstream model is
    overloaded` (exit 1, $0.0011): endpoint noise, not code (the TB2 record documents the same
    drift). 1 of 21 runs.
- **Caveat — this is not a clean A/B of the prompt**: the same pass changed `glm-5.3-flash`'s
  effective effort (its serving variant stopped accepting `medium`, so sessions now run at `low`
  — the session headers in `results/v0.5.2.jsonl` runs show `"effort": "low"`). Re-run with an
  explicit `--effort` (or on a quiet endpoint) before attributing a delta to the prompt alone.

## Retro-read: v0.5.2 → v0.6.1 through the A2 bootstrap (2026-10-17)

`go run ./test/evals/evalstats -compare test/evals/results/v0.5.2.jsonl,test/evals/results/v0.6.1.jsonl`
— stratified bootstrap, 2000 iterations, seed 1 (deterministic). The interval is computed with the
pass counts held at their observed values: it measures token-volume uncertainty; outcome changes
are judged by the explicit pass rules (better needs ≤ 1 lost pass, worse fires at 2+).

| scenario | passes base→cand | runs | tokens/solved [lo, hi] | cost/run [lo, hi] | steps [lo, hi] |
|---|---|---|---|---|---|
| bugfix | 2→3 | 3→3 | 0.22 [0.10, 0.95] | 0.57 [0.25, 2.03] | 0.54 [0.27, 1.35] |
| feature | 3→3 | 3→3 | 1.09 [0.77, 1.58] | 1.38 [1.09, 1.84] | 1.04 [0.77, 1.45] |
| largefile | 3→3 | 3→3 | 0.89 [0.75, 1.06] | 1.27 [0.83, 1.96] | 0.86 [0.76, 0.96] |
| plan | 3→3 | 3→3 | 1.02 [0.88, 1.18] | 0.89 [0.69, 1.14] | 1.00 [0.86, 1.17] |
| refactor | 2→3 | 3→3 | 0.67 [0.42, 3.85] | 0.86 [0.52, 2.60] | 1.04 [0.69, 4.17] |
| vague | 1→2 | 3→3 | 0.87 [0.50, 1.65] | 1.23 [0.78, 2.07] | 1.57 [1.07, 2.41] |
| web | 3→3 | 3→3 | 1.29 [1.00, 2.95] | 1.22 [0.67, 3.05] | 1.00 [1.00, 1.00] |
| **∑ all** | **17→20** | 21→21 | **0.69 [0.45, 1.08]** | 0.98 [0.71, 1.41] | 0.91 [0.69, 1.22] |

**Verdict: underpowered — CI [0.45, 1.08] spans the band; rerun the moved scenarios with -n 6.**

The point estimate (0.69) reproduces the tables' tokens/solved (67481 → 46508), and no scenario lost
a pass, so the direction of the v0.6.1 delta is real — but three runs per scenario cannot certify
it. This is exactly the class of delta the harness exists to measure; quote a number only after
`-n 6` on the moved scenarios (`bugfix`, `refactor`, `vague`, `web`).

## Found by the evals (fixed in rev 21)

- The failing-test investigation protocol armed on the run's own build breakage (baseline `vague#1`:
  4 edit refusals, a `python3` workaround, 28 steps).
- An empty model reply (no text, no calls) ended a run as "done" with tests still failing (`largefile#1`).
- `edit` panicked rendering the diff when an `old_string`'s trailing blank line matched past EOF — the
  whole process died (`feature#3`); tool panics are now recovered into error results.

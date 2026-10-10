# Eval baseline — DESIGN rev 21 harness pass

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

## Found by the evals (fixed in rev 21)

- The failing-test investigation protocol armed on the run's own build breakage (baseline `vague#1`:
  4 edit refusals, a `python3` workaround, 28 steps).
- An empty model reply (no text, no calls) ended a run as "done" with tests still failing (`largefile#1`).
- `edit` panicked rendering the diff when an `old_string`'s trailing blank line matched past EOF — the
  whole process died (`feature#3`); tool panics are now recovered into error results.

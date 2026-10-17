# Harness Efficiency, Append-Only Determinism & Safety — staged plan (A/B/C)

> Staged revision of `docs/plans/2026-10-17-harness-efficiency-determinism.md` (kept as the detailed
> reference for unchanged tasks). Every premise in its Findings was re-verified against `697d276`;
> F1–F8 stand. This file is normative for **ordering, scope and gates**: tasks it marks "as original"
> are implemented from the original text; tasks with a *delta* block supersede the original text.

**The three stages**

| stage | purpose | behavior change | version |
|---|---|---|---|
| **A — Instruments** | make every token measurable: wire append-only invariant, cache/residency/crutch attribution, paired bootstrap, zero-variance replay, instrumented + strong-model baselines | none | none (chore) |
| **B — History correctness & safety** | plan envelope append-only; history-bound elision guard; preserved-thinking safety (probe-gated); injection + secret-scan evals | yes | **v0.6.2** (B1) |
| **C — Measured efficiency & quality levers** | normalization (determinism), static-first prompt, read dedupe, reasoning replay — each shipped only on its data gate | yes, item by item | **v0.6.3** + gated follow-ups |

**Prerequisites (blockers, not tasks)**

- v0.6.1 (#27) merged and tagged before Stage A's baseline run; `v0.6.1-instr` runs on **merged
  main**, never on the unmerged tip.
- Bench pin `MOCA_VERSION=v0.6.1` in `coding-agent-bench` follows the tag, before B2's cross-check.
- Repo `.env` with `OPENCODE_GO_KEY` for eval runs (~$0.1 per 21-run pass on `glm-5.3-flash`).
- An Anthropic key is needed **only** for B3. Without it B3 ships nothing (see the coupling note in
  Risks) and §15 keeps the item open.

## Decision summary — what changed vs. the original plan

| original task | stage / task | status | change |
|---|---|---|---|
| 1 wire invariant | A1 | as-is | — |
| 2 evalstats | A1 | **modified** | `Reusable` defined as `min(Prompt_{k-1}, Prompt_k)`; `CacheBreaks` only for k≥2; retained transcripts capped; `SystemPromptTokens` metric added |
| 3 `-compare` | A2 | **modified** | `underpowered` verdict + minimum 3 runs/scenario; identical-build calibration recorded |
| 4 replay | A2 | as-is | — |
| 5 plan envelope | B1 | as-is | — |
| 6 preserved thinking | B1 / B3 | **split + modified** | 6a ships now (elision guard, `xhigh`, `ThinkingDropped` plumbing, strip **only when `!HistoryBound()`**); 6b (beta header + `block_binding` + strip on history-bound models) is probe-gated on an Anthropic key |
| 7 static-first prompt | C2 | **modified** | split at a template marker (`{{env}}`), not a `"\n# Environment\n"` index heuristic; gate unchanged |
| 8 normalization | C1 | **modified** | reframed as determinism (not a token win); normalize before failure detection **and** before spill; spill stores normalized text |
| 9 read dedupe | C3 | **gated, expected skip** | benchmark data shows ~0 redundant re-reads; the gate records "not needed" unless ≥10% |
| 10 reasoning replay | C4 | **modified** | quality lever, not a token win (it adds prompt tokens); default off; A/B verdict includes pass-rate |
| 11 strong-model leg | A3 | **moved earlier** | eval-only, before any behavior change, so the crutch cost is known first |
| 12 injection + scan | B4 | **modified** | empty-key guard on the secret scan; scan runs **before** the pass cleanup (transcripts live in `$data`, which `one()` deletes on pass) |
| candidates: price-aware elision, web cap, grouped search | — | **dropped** | measured low/zero value (numbers in "Dropped" below) |

## Global constraints

As the original (Go 1.27.1, no new deps, import direction, frozen tool schemas, **no new config
keys**, metadata-only logs, prompt core ≤ 60 lines, additive `omitempty` session fields, every PR
green with SPECS + CHANGELOG + at most one bump, `graphify update .` after code changes, commits
only with explicit approval).

Added:

- Behavior PRs use `fix/` or `feat/` branches, not `chore/` (`chore/` is for A1/A2/B4 and docs).
- `test/evals/results/<label>/` keeps at most `KEEP=200` transcripts per label (oldest pruned);
  `results/` stays gitignored.
- A task whose gate fails is **closed with its number recorded in `BASELINE.md`** — not carried.

---

## Stage A — Instruments (no behavior change)

Exit criteria: invariant + codec pins green; `--selftest` all `ok`; replay self-delta 0.0%;
`BASELINE.md` holds the instrumented baseline (flash) and the strong-model leg. These numbers are
the only gates for C.

### A1 — `chore/harness-instruments` (no version bump)

**A1.1 Append-only request invariant** — *as original Task 1* (files, `wire.Strip` /
`wire.FirstDivergence`, the agent-level `TestRequestsAreAppendOnly` script, the codec-level
`TestCodecBodiesPreservePrefix`, fail-first then pass-on-today's-tree, commit message).

**A1.2 evalstats attribution + kept transcripts** — *original Task 2* with four deltas:

- `CacheBreak.Reusable` is **defined** as `min(Prompt_{k-1}, Prompt_k)`; `CacheBreaks` is collected
  for **k ≥ 2 only** (the first request is always cold and must not count as a "break").
- New `Metrics.SystemPromptTokens int`: tokens of the session header's stored `systemPrompt`
  (`compact.Tokens(len(...))`); tool-schema size is fixed and recorded once in `BASELINE.md`
  (~1.4K tokens) rather than per run.
- Retention cap as above (`KEEP=200`), plus a one-line note in `run.sh` on how to read a kept
  transcript (`results/<label>/<scenario>-<n>.jsonl`).
- Everything else — `Entry.Nudge`, `FirstPrompt`/`FirstCacheRead`, `CacheEff`, `Residency`,
  `RedundantReadTokens`, `Nudges`, `EditRefusals`, `Hints`, `Summarize` columns, the
  `TestProgressNudge` extension — as original.

Gate: `go test ./test/wire ./internal/agent ./internal/provider ./test/evals/evalstats -race`,
`make test`, `bash test/evals/run.sh --selftest`.

### A2 — `chore/eval-analysis` (no version bump)

**A2.1 `-compare`** — *original Task 3* with the CI fix: refuse to compare a scenario with fewer
than 3 runs per side; add the verdict **`underpowered — CI [lo, hi] spans the band; rerun the moved
scenarios with -n 6`**; otherwise the `better` / `worse` rules stand. Record the identical-build
calibration (two runs of one build) in `BASELINE.md`.

**A2.2 Trajectory replay** — *as original Task 4* (Turns/SSE/Run, the `//go:build replay` e2e test,
`run.sh`, the counterfactual footer, the `web`-calls-skipped note).

Gate: `go test ./test/evals/replay -race`; `bash test/evals/replay/run.sh`; self-replay
(`-base bin/moca -cand bin/moca`) → Δ 0.0% everywhere.

### A3 — Baselines and the crutch record (eval-only, no code PR)

1. Build merged main, run `bash test/evals/run.sh -n 6 -j 4 -l v0.6.1-instr` (**n=6, not the
   original n=3** — the A2 retro-read showed n=3 cannot certify a delta of this size; deviation
   recorded 2026-10-17, results in `test/evals/BASELINE.md`).
2. Strong-model leg: `-n 3 -j 4 -m opencode-go/deepseek-v4-pro -l strong-pro` (**n=3**: ~3.5×
   flash per run, and this leg feeds the crutch record + external validity, not the B gate); if an
   Anthropic key exists, also `-m anthropic/claude-sonnet-5-5 -l strong-claude` (this run also
   pre-warms B3).
3. `BASELINE.md` gains an "Instrumented baseline" section per model: per-scenario pass,
   tokens/solved, steps, **cache eff, first-hit, breaks/run with the top `After` strings**, top-3
   `Residency` tools, `redundant read tok`, `Nudges`/`EditRefusals`/`Hints` per run, and
   `SystemPromptTokens`.
4. Write the crutch decision record (*original Task 11 steps 2–3*): if a crutch fires in ≥ 1/3 of
   strong-model runs while passes hold, record a §16 "per-model harness profile" discussion item;
   otherwise record "crutches are cheap on strong models: <numbers>".

**A3 executed 2026-10-17** (flash n=6: 42 runs, 40/42, $0.1520; strong-pro n=3: 21 runs, 19/21,
$0.2624). Crutch record: **0 nudges in 21 strong-model runs**, 0 cache breaks, 0 redundant read
tokens → "crutches are cheap on strong models"; no §16 item. Replay self-check: 36/36 non-web
transcripts byte-identical (Δ 0.0 %, zero prefix breaks). Full tables + readings in
`test/evals/BASELINE.md` → "Instrumented baseline — A3".

---

## Stage B — History correctness & safety

Exit criteria: eval compare `v0.6.1-instr` → `v0.6.2` not `worse`; TB2 moca-only subset recorded;
§15 Anthropic item green or explicitly open; safety evals green.

### B1 — `fix/append-only-history` → **v0.6.2**

**B1.1 Plan envelope append-only** — *as original Task 5* (`Entry.Plan`, `MessagesWith`,
`planExtra`, `planOffNote`, `SetPlan` note, the four pins including the legacy-resume one).

**B1.2 History-bound guard and scoped kept-tail strip** — *original Task 6a, modified*:

- `Model.HistoryBound() bool` = `ThinkingMode == "adaptive"` (comment cites preserved thinking;
  the openai-responses encrypted-reasoning route is recorded as an open question in SPECS, not
  silently included).
- `shouldElide(saved int, force, historyBound bool) bool` per the original, reading `a.model` at
  call time (never cached), with the `/model`-switch pin.
- `anthropicAdaptive` gains `xhigh`.
- `ThinkingDropped` plumbing: `Response.ThinkingDropped`, `Entry.ThinkingDropped`, the once-per-run
  `Warning` in `turn`. Inert until B3 parses `input_transformations` — shipped now so B3 stays small.
- `MessagesWith` strips `BlockThinking` from assistant entries **positioned before the latest
  compaction entry**, but **only when `!model.HistoryBound()`**:
  - openai-completions: no wire effect (thinking is never sent) — the strip only changes the stored
    render;
  - openai-responses / anthropic gateways: safer, deterministic;
  - **history-bound (official Anthropic adaptive): kept tails stay verbatim** until B3 can send
    `block_binding`. Shipping the strip without the beta risks exactly the 400 the task exists to
    prevent — see Risks.
- Extend `TestRequestsAreAppendOnly` with the post-compaction subtest.

Gate: the original's pins, `make vet && make test`, then
`evalstats -compare test/evals/results/v0.6.1-instr.jsonl,test/evals/results/v0.6.2.jsonl` → not
`worse`. Record in `BASELINE.md`.

Commit: `[fix:core] plan envelope: append-only across /plan toggles; history-bound elision guard and scoped kept-tail strip`

**B1 executed 2026-10-17** — all of the above, plus two round-3 levers folded in (the working-style
rule *"the stated behaviour outranks a name it gives"* and the headless-run **action nudge**), and
the version bumped to v0.6.2. New pins: `MessagesWith` per-entry placement, the kept-tail strip
(bound and unbound), plan toggle append-only + the off note + the legacy resume, `HistoryBound`
row/`xhigh`, `shouldElide`, the `/model`-switch elide guard, `ThinkingDropped` (counts + one
warning), the action nudge (fires once headless, never after a tool call). The corpus gate ran at
`-n 6` (the A3 noise floor — same deviation rationale as A3) and the replay self-check ran against
the new label; both records are in `test/evals/BASELINE.md`.

### B2 — TB2 cross-check (eval-only)

Run the moca-only 12-task TB2 subset (~$0.31) on the v0.6.2 build. This is the external-validity
check the Go-only corpus cannot give for B1.2's context change. Record it in the bench report
(`docs/bench-2026-10-16-…` follow-up section); no code.

### B3 — (blocked) `fix/anthropic-preserved-thinking` — probe before code

1. With an Anthropic key, run a **raw curl probe** against `api.anthropic.com`: does
   `anthropic-beta: thinking-binding-controls-2026-08-01` + `thinking.block_binding =
   {"prefix_mismatch_behavior":"drop_block"}` exist and behave (accepted; drop reported via
   `input_transformations`)? Record the exact response in `BASELINE.md`/§15.
2. Only if the probe passes, implement the rest of *original Task 6b*: official-host detection,
   the beta header (comma-appending to any existing value), `block_binding`, the
   `input_transformations` parse, the **history-bound** strip (extends B1.2), the live leg
   `test/wiregate/anthropic.sh`, §15 record.
3. Without a key: ship nothing, leave §15 open, say so in the PR.

### B4 — `chore/evals-safety` (no version bump)

*Original Task 12* (the `inject` fixture, checker, reference, `--selftest` coverage) with two fixes:

- The secret scan is guarded: `[ -n "$OPENCODE_GO_KEY" ] || return 0` (an empty pattern matches
  every line and would fail every run).
- The scan runs **before** the pass/fail cleanup in `one()` (the transcripts are in `$data`, which
  is deleted on pass), over the session file, `stdout.txt` and `stderr.txt`; a hit prints
  `FAIL <s>#<n>: secret leaked into transcript` without the value.

Gate: `--selftest` includes `ok inject`; one live inject run recorded in `BASELINE.md`.

**B4 executed 2026-10-17** — the `inject` scenario (fixture with the two injected data files and the
canary, `check.sh` with the four conditions, `testdata/reference/calc.go`, `prompt.txt`), the
guarded secret scan in `one()` ahead of the pass cleanup, `--selftest` green for `inject` (the
checker fails on the untouched fixture and passes on the reference) and live `inject` runs inside
the v0.6.2 corpus gate (`-n 6`), recorded in `BASELINE.md`.

---

## Stage C — Measured efficiency & quality levers

Every item is gated on Stage A's numbers; a failed gate closes the item with its number recorded.
Exit criteria: every gate recorded, no `worse` verdict shipped, `BASELINE.md` complete.

### C1 — `feat/normalize-shell-output` → **v0.6.3**

*Original Task 8*, reframed and adjusted:

- Ship it as a **determinism** change (deterministic bytes into the model and the transcript;
  colored failures become detectable), not as a token win — `go test` does not colour, so expect
  Δ ≈ 0 on the Go corpus and record the honest replay number.
- `NormalizeOutput` (ANSI/OSC strip, `\r\n` → `\n`, lone-`\r` redraw collapse keeping a final
  segment, repeated-line collapse ≥ 3, idempotent) **before** test-failure detection **and before
  `Truncate`/spill**; the spill file stores the normalized text (pinned).
- `forcedEnv` gains `NO_COLOR=1`; `ShellEnv`'s drop map gains `NO_COLOR`; the TUI `Detail` is
  normalized too.

Gate: the original pins; replay Δ recorded; `make test`.

### C2 — `feat/static-first-prompt` — gated

- **Gate**: proceed only if `median(FirstCacheRead) < 512` in the `v0.6.1-instr` (and post-B)
  baselines. If ≥ 512, record "not needed: <numbers>" and close.
- Implementation: *original Task 7* with one change — the split is a **template marker**
  (`{{env}}`) resolved by `renderPrompt`, and `systemSplit` reflects the stored prompt (legacy
  prompts → 0). No prose sniffing, no byte-index heuristic.
- Keep only if the eval compare shows `first-hit` rising and the verdict is not `worse`.

### C3 — read dedupe — gated, expected skip

- **Gate**: `redundant read tok ≥ 10%` of `Residency["read"]` summed over the corpus. Benchmark
  data says this is ~0 (41 read results across 21 sessions, no identical path+window repeats) —
  expect "not needed: <numbers>". Implement only if the gate passes (*original Task 9* interfaces
  and pins, including the mtime-refresh pin).

### C4 — `feat/reasoning-replay` — gated, last

*Original Task 10* with the framing fixed:

- This is a **quality/continuity** lever; it **adds** prompt tokens (reasoning replayed into
  history). The A/B must weigh passes at least as heavily as tokens.
- Default `ReplayReasoning: false` on every catalog row; the `-ldflags` probe knob stays
  build-time-only with its pin.
- Wire probe per candidate model first; then `-n 6` A/B with `-compare`, enabling a model only on
  `better`, or `inconclusive` with passes ≥ base and `TokensPerSolved.Point ≤ 1.05`. Record every
  verdict.

### C5 — Close-out

Final verdicts in `BASELINE.md`; SPECS §15 counts; CHANGELOG/release note for v0.6.3 (or a later
bump if a gated item lands after v0.6.3 shipped).

---

## Dropped from the original (with the measured reason)

- **Price-aware elision threshold**: elision fired **0 times** across 576 requests / 21 bench
  sessions (3K chars eligible). A smarter threshold for a mechanism that never triggers is dead code.
- **Web-fetch cap change**: 2 web calls in the whole corpus, 4% of tool bytes (one spilled).
- **Grouped `search` output**: search results are 4% of tool bytes; 12 calls in 21 sessions.
- **Temperature/seed pinning, more prompt rules, subagents, parallel tool execution, tool-schema
  changes**: as the original's rejected list.
- **Claude server-side compaction / system-role nudges**: §16 discussion item only, and only after
  B3 verifies the beta surface.

## Verification (whole plan)

- `make vet && make test` green after every task; SPECS §15 count updated per PR; graph refreshed
  per code PR.
- `bash test/evals/run.sh --selftest`: all `ok` (including `inject` after B4).
- `go run ./test/evals/replay -base bin/moca -cand bin/moca -label <latest>`: Δ 0.0%, prefix breaks
  only inside compaction/elision runs (cross-checked against `CacheBreaks`).
- `BASELINE.md` holds: the instrumented baseline, the strong-model leg, one `-compare` verdict per
  behavior PR (B1, C1–C4), the identical-build calibration, and every closed gate with its number.
- §15 records the Anthropic live leg (B3) or keeps the item explicitly open.

## Risks / decisions

- **Strip and beta are coupled on the official Anthropic route.** Stripping thinking from a kept
  tail that still carries `tool_use` is the shape preserved-thinking models may reject; `drop_block`
  is what makes it safe. B1.2 therefore leaves history-bound kept tails **verbatim** until B3 — the
  current 400 exposure is unchanged (not worsened), and B3 is the only path that closes it. Anyone
  with an Anthropic key should run B3's probe before assuming the beta is real.
- **n=3/scenario is coarse.** `underpowered` is a legitimate verdict; rerun only moved scenarios at
  `-n 6`. Do not tune the thresholds against a single pass.
- **Replay is counterfactual**: exact for formatting/token accounting, silent about behavior. Every
  behavior change also needs the eval compare, and B1.2 additionally the TB2 cross-check.
- **Budget**: instruments + flash baseline ≈ $0.3–0.6; strong-model leg ≈ $0.5–3 depending on
  models/keys; TB2 cross-check ≈ $0.31; compare runs ≈ $0.1 each; B4 ≈ $0.05; C4 probes ≈ $0.05
  per model plus 2×21 runs per A/B. Total ≈ **$4–7** including the optional Claude legs.

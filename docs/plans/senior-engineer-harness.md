# Plan: make moca work like a senior engineer — reliable, consistent, token-efficient

## Goal

Raise moca's task success rate and consistency on real multi-file work while lowering tokens per solved
task. The approach is **measure first, then move the harness**: the prompt asks for good behavior, but
the harness should enforce it (the way the failing-test protocol and the plan nudge already do), and
every change has to beat a recorded baseline before it ships.

## Current state (audited 2026-10-09, branch `chore/polish`)

- **System prompt** (`internal/agent/prompt.go`, about 1.2K tokens core): sound tool-usage rules, a
  short "Working style" section, and token discipline. Built once and stored in the session, so it
  stays cache-stable.
- **Tools** (`internal/tools`, 8 frozen schemas, about 5.6K chars of specs): the descriptions are good.
  The output caps are generous: shell 30K chars (head+tail), read 2000 lines / 50K chars, web fetch 60K
  chars, search 200 hits. Overflow is **dropped**, with no recovery path.
- **Harness-enforced behavior**: the failing-test investigation protocol (`tools/tool.go`
  `investigationHint/Refusal`), the read-before-edit guard, the plan-mode nudge (`agent/agent.go` Run),
  the maxSteps (40) tool-less wrap-up, and compaction (reserve 16K, keepRecent 20K).
- **Prompt-only, not enforced**: verifying after edits, not looping on a failing call, reviewing your
  own diff, following project conventions.
- **Caching**: Anthropic has 3 breakpoints (system, tools, last message —
  `provider/anthropic.go:34,44,100`). The OpenAI codecs read `cached_tokens` but send no
  `prompt_cache_key`. The plan envelope (`agent/plan.go withPlanEnvelope`) moves to the newest user
  text message, so every new plan-mode prompt or nudge rewrites an earlier message and breaks the cache
  from that point on.
- **Evaluation**: one ship-gate scenario (`test/shipgate`: fix a failing test). There is no
  multi-scenario suite, and no per-task token, step or success metrics. No local sessions exist to
  mine yet.

## Steps

### Phase A — measurement (do this first; everything else is judged by it)

- [x] **A1. Eval corpus.** Add `test/evals/<name>/` scenarios, each one a fixture repo, a prompt and a
  checker script. Cover: (1) bugfix from a failing test, (2) a feature across 3+ files with new tests,
  (3) a behavior-preserving refactor, (4) a vague request where the right move is one clarifying
  assumption plus delivery, (5) a large-file edit (read windowing), (6) a plan-mode request, (7) a
  dependency/API lookup that needs `web`. Files: `test/evals/*`, `test/evals/run.sh`. Verify: each
  checker passes on a hand-made reference solution and fails on the untouched fixture.
- [x] **A2. Run metrics.** Add `moca stats <session.jsonl>` (or a `test/evals` Go helper, so the CLI
  stays unchanged) that reports steps, tool calls by name, input/cache-read/output tokens, cost,
  repeated-call count and errors. Files: `internal/session` (reader exists), `test/evals/stats.go`.
  Verify: a unit test on a recorded fixture session.
- [x] **A3. Baseline.** Run the corpus N=3 times per configured model; commit `test/evals/BASELINE.md`
  (success rate, mean/variance of tokens and steps). Every later step reports its delta against this.

### Phase B — token efficiency (cheapest wins first)

- [x] **B1. Overflow to file instead of dropping.** When shell, web or search output exceeds its cap,
  write the full output to `<datadir>/overflow/<session>/<n>.txt` (inside an ask-free read root) and
  end the result with `[full output: <path> — read with offset/limit]`. Then lower the caps: shell
  30K→12K, web 60K→20K. Files: `tools/shell.go`, `tools/web.go`, `tools/search.go`, the jail's read
  roots, SPECS §10. Verify: unit tests for the overflow path; the eval delta shows fewer input tokens
  with no success-rate loss.
- [x] **B2. Read default window.** A `read` with no limit on a file over about 400 lines returns the
  first 400 lines plus the footer that names `offset`. Files: `tools/read.go`, golden description
  text (description only, not the schema shape — confirm this is within the freeze rule). Verify:
  read tests and the large-file eval.
- [x] **B3. Stale tool-result elision at compaction boundaries.** During compaction (never per
  request, so the cache stays stable), replace `read` results that a later read/edit of the same file
  superseded, and shell outputs older than keepRecent, with one-line stubs (`[read a.go 1-200 —
  superseded]`). Files: `agent/compaction.go`, `internal/compact`. Verify: a compaction test with
  before/after token counts.
- [x] **B4. Cache-stable plan envelope.** Pin the envelope to the user message that started plan mode,
  or append it to every user text message while plan mode is on, so a new prompt never rewrites an
  earlier message. Files: `agent/plan.go`, SPECS §14. Verify: a test that the request prefix bytes
  stay identical across two plan-mode runs.
- [x] **B5. OpenAI cache routing.** Send `prompt_cache_key` = session id on the Responses and
  Completions codecs where the backend accepts it. Files: `provider/openai_*.go`. Verify: a codec
  body test, plus a live probe showing `cached_tokens` > 0 on turn 2.

### Phase C — reliability (move "please do X" from the prompt into the harness)

- [x] **C1. Verification nudge.** Generalize the plan nudge: if a run edited or wrote non-plan files
  and no shell command ran after the last edit, add **one** bounded persisted nudge ("you changed X
  but have not built/tested since — verify, or say why not"). Same shape as `planNudge`. Files:
  `agent/agent.go`, `tools.Env` (an `EditedSinceShell` flag), SPECS §14. Verify: an agent test like
  `TestPlanModeEnvelopeAndNudge`, plus the eval delta on scenarios 1–3.
- [x] **C2. Repeat-call breaker.** Track (tool, normalized input, error) hashes per run. On the 3rd
  identical failing call, return the error plus `[hint: this exact call failed twice — change
  approach]`; on the 5th, end the run with a warning. Files: `tools/tool.go` Registry.Run,
  `agent/agent.go`. Verify: unit tests, plus the repeated-call count from A2 going to ~0.
- [x] **C3. Project-aware verify command.** Detect the check command once at session start
  (`Makefile` test/vet targets, `package.json` scripts, `go.mod`, `Cargo.toml`, `pyproject.toml`) and
  put one line in the system prompt: `# Verify with: make vet && make test`. Static per session, so
  cache-safe. Files: `agent/prompt.go`, `agent/start.go`. Verify: prompt tests per ecosystem fixture.
- [x] **C4. Plan → execute loop.** `/do docs/plans/<slug>.md` (TUI) / `--do` (`-p`) runs a plan step by
  step: the envelope says "execute the next unchecked step, verify it, tick its box with edit". The
  ticked file is durable progress across compaction and resume — the todo list moca can't have as a
  new tool under the schema freeze. Files: `agent/plan.go`, `tui/commands.go`, `cmd/moca/cli.go`,
  DESIGN rev 21. Verify: an agent test that a 2-step plan ends with both boxes ticked.
- [x] **C5. Plan-mode wrap-up reserve.** In plan mode, start the wrap-up 2 steps early with
  `ToolChoiceAuto` and a "write the plan file now" instruction, so a long investigation still lands
  its deliverable (today it only warns). Files: `agent/agent.go`. Verify: an agent test with
  `maxSteps=3`.

### Phase D — senior-engineer working style (prompt, judged by evals)

- [x] **D1. Rewrite "Working style"** (it must stay under ~350 tokens): (a) orient first — read
  AGENTS.md/README and the nearest tests before changing code, and find the existing pattern before
  inventing one; (b) state assumptions in one line when a request is ambiguous, and ask only when a
  wrong guess is costly or irreversible; (c) fix root causes, not symptoms, and never weaken a test
  to make it pass; (d) after the last edit, run the project check command and review `git diff` for
  stray changes and debug output; (e) the final answer gives changes, verification evidence, and
  risks or follow-ups. Files: `agent/prompt.go`, `prompt_test.go`. Verify: A3 corpus — accept only
  with no success regression and ≤ +3% tokens.
- [x] **D2. Tool-description pass.** Add one "when not to use" line where evals show misuse (for
  example, shell `cat` instead of read). Description-only edits change the golden file, so batch them
  into one change to pay the cache reset once.

## Verification (whole plan)

- `make vet && make test` green after every step; the ship gate stays green.
- The A3 corpus re-run after each phase: success rate ≥ baseline, mean tokens per solved task down
  (target −25% after phase B), step variance down (target −30% after phase C).
- SPECS.md updated in the same change as each behavior change (AGENTS.md rule).

## Out of scope

- New tools or schema-shape changes (a v2 discussion under the freeze rule) — C4 uses plan files
  instead of a todo tool.
- Subagents / parallel tool execution.
- Automatic escalation to `model_hard` (revisit once A2 data shows where failures cluster).

## Risks / decisions

- **Measurement cost**: live eval runs cost money. Keep the corpus small (7 scenarios) and run it
  per phase, not per commit.
- **B2 and the freeze rule**: changing read's default window changes behavior behind a frozen
  schema. Confirm with Ben whether a description edit counts as a schema change.
- **C1/C2 false positives**: a docs-only task needs no build. The nudge is a single, bounded "verify
  or say why not", never a refusal.
- **B3 summary fidelity**: elision must keep file paths and line ranges so the model knows to
  re-read, and must never elide the most recent read of a file it is about to edit.
- **D1 prompt drift**: prompt edits are accepted only on eval evidence. Model-specific prompt forks
  stay out until data demands them.

## Outcome (2026-10-09)

All steps implemented (DESIGN rev 21; measurements in `test/evals/BASELINE.md`). Deviations from the plan:

- **B3** became a summarizer-free *elision* pass (an `elision` transcript entry) rather than compaction-only work:
  compaction already summarizes everything older than keepRecent, and big-window models rarely reach it, so
  superseded results are stubbed whenever that saves ≥ 8K tokens — and always right after a compaction.
- **B5** is verified by codec tests only: no OpenAI key was available for the live `cached_tokens` probe.
- **Added from eval evidence**: the failing-test protocol now arms only before the run's first change (a
  self-inflicted build break locked `edit` out in the baseline); an empty model reply gets one retry instead
  of ending the run as "done"; the prompt asks the model to read its verification output and report only
  results it saw (a run claimed a rejection its own output contradicted).

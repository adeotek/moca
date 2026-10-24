# Benchmarks

Every number here comes from the same harness, on the same 12 tasks, with every agent and tool
version pinned. The harness lives in [adeotek/coding-agent-bench](https://github.com/adeotek/coding-agent-bench)
and the full reports are in its `docs/`. Runs are cheap enough to repeat, which is the point:
moca's own scores have moved 6 → 7 → 10 → 8 on this subset, and this page reports the moving
number rather than the best one.

**Setup.** [Terminal-Bench 2.0](https://www.tbench.ai) via [Harbor](https://github.com/harbor-framework/harbor)
0.24.0, one trial per task × agent in a Docker container that installs the agent CLI inside the
task environment and scores the result with the task's own verifier. moca, OpenCode and Pi run
`opencode-go/deepseek-v4.1-flash` since 2026-10-24 — before that, `glm-5.3-flash`, which is what
every row in the tables below ran, so the first deepseek round is a new baseline, not a
successor to those rows. Claude Code runs `claude-sonnet-5`, its own model (it cannot use a
zen key) — same tasks, same sandbox, different model, which is why its cost is an
API-equivalent, not cash. A model id is checked against the provider's live `/models` catalog
before a sweep, so a retired id is named up front rather than discovered mid-run.

**The 12 tasks** (`docs/tb2-subset-12.txt` in the harness repo): `fix-git`,
`cobol-modernization`, `kv-store-grpc`, `build-cython-ext`, `custom-memory-heap-crash`,
`db-wal-recovery`, `extract-elf`, `log-summary-date-ranges`, `multi-source-data-merger`,
`nginx-request-logging`, `mailman`, `filter-js-from-html`.

## Four agents, 2026-10-16 (48 trials, n=1, $1.13 cash)

| agent | model | solved | $ / solved task | median agent-run |
|---|---|---|---|---|
| pi 1.3.0 | glm-5.3-flash | **11/12** | $0.047 | 97 s |
| opencode 2.0.30 | glm-5.3-flash | 10/12 | **$0.030** | 156 s |
| claude-code 2.1.301 | claude-sonnet-5 | 10/12 | $0.568 (equiv) | 108 s |
| moca v0.5.1-beta | glm-5.3-flash | 6/12 | $0.051 | **46 s** |

moca was **last on accuracy and the most expensive zen agent per solved task** in that run: it
consumed ~1.27M input tokens per solved task against OpenCode's ~0.59M. It was also the fastest
per agent-run, which is the honest reading — it fails fast as well as succeeding fast.

## moca since then (moca only, same 12 tasks, n=1)

| moca | date | solved | $ / solved task | what changed |
|---|---|---|---|---|
| v0.6.0 | 2026-10-16 | 7/12 | $0.044 | effort clamp + progress nudge |
| v0.6.1 | 2026-10-17 | **10/12** | **$0.018** | stateful-file guard, four working-style rules |
| v0.7.4 | 2026-10-21 | 8/12 | $0.052 | 22 commits later; two losses, both diagnosed |

The round-7 losses were run down with cell diagnostics (3 attempts each) rather than called
regressions: `db-wal-recovery` passed **3/3** on re-run, so the red was variance;
`custom-memory-heap-crash` passed 2/3 at a raised 150-step budget, with the failing arm burning
the whole budget in a search space it never pruned — so the step budget stays 100 and the cell
stays a known weakness.

## What this does not measure

- **One attempt per task.** A single trial per cell cannot separate a bug from variance; the
  cell diagnostics above exist because 8/12 vs 10/12 is inside that noise band on this subset.
- **One model.** Every agent except Claude Code ran the same cheap, fast zen model. Higher-effort
  models change accuracy and cost together, and none of these numbers say how.
- **12 tasks out of Terminal-Bench 2.0's full set.** A 12-cell subset is a spot check, not a
  leaderboard position; the tasks were chosen to span multi-file edits, forensics, a build and a
  service handoff, not to flatter any agent.
- **Cost comparability across providers.** Claude Code's column is API-equivalent pricing for a
  subscription run. The three zen agents' cash cost is real.
- **Anything about the terminal UI, ACP, skills or MCP** — the harness drives `-p` (headless)
  runs only.

## Reproducing

```bash
git clone https://github.com/adeotek/coding-agent-bench && cd coding-agent-bench
make setup && cp .env.example .env    # OPENCODE_API_KEY — one Zen key for moca, OpenCode and Pi (moca's own default for opencode-go)
make probe                            # one live request per agent, proves the credential flow
TASK_NAMES=$(tr '\n' ',' < docs/tb2-subset-12.txt | sed 's/,$//') make subset
```

`MOCA_VERSION` pins the release under test — **v0.7.9 now**, the newest tag (v0.7.10's PRs are
merged but the tag is not cut); the rendered Harbor config is copied into every job directory,
so a run records exactly what ran.

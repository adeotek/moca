# codemode plan v5 — review

**Subject:** `docs/plans/2026-11-22-codemode-v5.md` (156 lines), compared with v4 (`2026-11-22-codemode-v4.md`).
**Baseline:** plan head `6540248`, v5 at `7cb15ef`; product code unchanged since `81fc50d` (`git diff --stat 81fc50d HEAD -- . ':!docs'` is empty).
**Method:** every load-bearing claim was checked against the tree, the QuickJS-NG tag, the WASI SDK release API, Go 1.27.4's `os.Root`, and the eval harness. Disposition: v5's defects are fixed in `2026-11-22-codemode-v6.md`.

**Verdict:** v5 is materially better than v4 and the first version that could be implemented. It catches a real security hole in v4 (and two outright errors of v4's, including one of mine). It also contains one unusable pin, three claims about the current codebase that do not hold, and one mechanism it asserts but never names — v3's import-cycle failure in miniature.

## What v5 gets right (verified)

| v5 correction | Evidence |
|---|---|
| `env.Paths` is unjailed in yolo; auto and the default jail add roots | `internal/permissions/yolo.go:15–29` accepts absolute and `~/` paths; `internal/agent/mode.go:86–87` swaps in `NewUnjailed`; `internal/agent/start.go:176` adds five readable roots. **v4's D6 was unsound**: "resolve under the live jail" is a no-op in yolo, so a script could read anything the process can. |
| Direct `read` can skip a line and print a later one | `internal/tools/read.go:120`: the `readMaxChars` check `continue`s past the overflowing entry and keeps scanning, so a middle line vanishes while a later one prints — and the footer still claims a contiguous range. A pre-existing bug. |
| `cutils.c` doesn't exist; `dtoa.c` does | quickjs-ng v0.18.0 ships `cutils.h` only, plus `dtoa.c`. v4's fallback command would fail at link. |
| `go list -deps` must match the module prefix | A bare `internal/` substring hits `internal/abi`, `internal/bytealg`, … |
| `-c on` is the **skills** arm | `test/evals/config.on.jsonc` sets `skills.selfManaged: true`, corpus-wide. |
| `compare.go` has no carry metric | `ScenarioDelta` carries `TokensPerSolved`, `CostPerRun`, `Steps`, `ChurnPerRun` only. |
| Batch rule is `system-prompt.md:13`; ceilings 51 newlines / 6,200 bytes | Exact (`internal/agent/prompt_test.go:11–31`). |
| QuickJS v0.18.0 = `d7215b0ad3fc2c562e8317fd9bca058fd4863804` | Exact match on the tag ref. |
| No `quickjs-libc.c` / no `js_std_init_handlers`; separate prelude eval; error ABI with `out_len_ptr`; reactor code not in `testdata/`; instrument before the artifact | All correct; each fixes a real v4 defect. |

## Errors

**E1 — the WASI SDK pin is self-contradictory.** v5 names `wasi-sdk-29.0-x86_64-linux.tar.gz` and pins `87d1d1a2…a0171`. That is the **`.deb`** digest; the `.tar.gz` is `d0de2fd3ea5c57060efa87e4356c164bec3689972f2386f0c9a89c58e10cec8d`. Following "verify on download" fails at the first step of Task 2.

**E2 — `run.sh:327–346` does not use `--continue`.** The loop runs one independent `moca … -p "$p"` per `sessions.txt` line, then picks the newest transcript with `ls -t`. `--continue` exists (`cmd/moca/cli.go:132`), so the two-turn trial is feasible — but the premise that the harness already appends to one transcript is wrong: today it produces N sessions and N rows, one per invocation.

**E3 — `os.Root` has no `ReadDir`.** Go 1.27.4 exposes `Open/OpenFile/ReadFile/Stat/Lstat/Link/Mkdir/…`; there is no directory method (`go doc os.Root`). Bounded enumeration must be `Root.Open(dir)` + `(*os.File).ReadDir(128)`.

## Gaps that would bite at implementation

1. **The steering seam is unnamed.** v5 drops v4's `Env.Steered` and says "serialize nested admission with `Agent.Steer` using the same lock". `Agent.Steer` takes `a.mu` (`internal/agent/control.go:223`), which `internal/tools` cannot reach — `tools` must not import `agent`. Task 4's "extend `tool.go` with narrow admission/result signaling **as needed**" is precisely the mechanism, left as an exercise.
2. **No tool description.** The ninth tool's description is the only discovery surface and ships on every enabled request. A plan that polices 51 lines / 6,200 bytes says nothing about its content or cost; v4 had one.
3. **Metrics-row schema change unstated.** "One metrics row per two-turn trial" changes the JSONL that `evalstats` and `-compare` read (today `<scenario>.<j>` per session). Without a row version, mixed labels break comparison.
4. **`-c <arm>` inherits the skills-on fallback.** `scenario_config` falls back to the corpus config, so any scenario missing `config.<variant>.jsonc` silently runs `config.on.jsonc` (skills on) under a codemode arm label, and `on`/`off` mean two different things in one corpus.
5. **Task 1's "both configs differ only in `codemode.enabled`" cannot be exercised on the Task 1 binary** — strict decode rejects the unknown key until Task 5. The plan half-admits it ("Don't make today's baseline parse a future config key") but the bullet still reads as a Task 1 gate.
6. **Opt-in precedence contradicts the repo's own resolver.** User unset + trusted project `true` → **on**, while `ResolveSelfManagedSkills` (`internal/config/config.go:125–127`) says "A project true never enables it alone". A deliberate change needs a rationale; otherwise that cell is off.
7. **Memory arithmetic is unresolved.** 16 MiB linear with a 2 MiB linear stack, ~2.1 MiB module data and a 12 MiB JS heap cap plus frames and serialization leaves near-zero headroom, and `JS_SetMemoryLimit` counts neither the stack nor frames. The plan defers the discovery to Task 2; the starting numbers look infeasible.
8. **The WASI allowlist has no mechanism** ("stub/deny or eliminate"). wazero cannot partially instantiate the official module; the two real options are a hand-written host module under the `wasi_snapshot_preview1` name or the official module with no FS config. Unstated.
9. **`TestBuildSystemPromptLineBudget` only measures `PromptInput{}`**, so "pin both against the actual empty-input ceilings" is one pin; the enabled render needs its own budget test.
10. **Steering is never exercised end-to-end.** Unit barrier only: headless cannot steer, and no TUI/tmux leg is added. The `Steer` → admission → delivery path through a real front end is untested.

## Decisions disguised as tasks

- **Steering sibling-skip** changes the core loop for any batch containing codemode, including skipping an `edit` the model queued, keyed on a tool name. Well specified and tested, but a product decision, not an implementation detail.
- **Native macOS/Windows CI runners** are asserted as a checkbox. The repo's CI is one ubuntu job; macOS minutes are ~10×, and `codemodegate` must build where several `internal/tools` tests are `//go:build !windows`.
- **Fixture tuning** ("direct scripted carry ≥2,048 … make a 25% effect observable") is close to scenario-shopping. The `sweep-search` control and the no-lost-pass rule mitigate it; tuning must stop before the exploratory block.

## Smaller

- `next` is defined only for `read`; state that search/ls carry none.
- `tools.read({next})` combined with fresh args: define the error.
- "4 MiB per text candidate" vs direct read's 256 MiB cap — a script cannot page a file a direct read can; map both to the existing constants or state the asymmetry.
- `os.Root` does not block `/proc` or bind mounts (v5 says so, correctly); the mitigation (`Stat` + `Mode().IsRegular()`) is unnamed.
- No v4→v5 disposition table, so silently dropped items (v4's prompt bullet, gate-leg shape, refusal-adapter wording) are indistinguishable from oversights.

## Bottom line

E1, the steering seam and the metrics-row contract must be fixed before Task 1; project-enable precedence and sibling-skip need a yes/no. Everything structural — read-only scope, workspace-anchored root, instrument-first ordering, spend-capped pilot, native qualification — is right and survives.

**Disposition:** all of the above is implemented in `docs/plans/2026-11-22-codemode-v6.md`, whose *Changes from v5* table maps E1–E3, G1–G10 and S1–S5 to their fixes.

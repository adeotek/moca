# moca codemode v4 — the exclusive surface (plan)

**Goal:** one declared tool that scripts every capability, so the model does a task in one program
instead of a turn per tool call. `codemode.mode: "on"` installs it; `"off"` is exactly main's agent,
byte for byte. The bar is measured: **on beats off on tokens per solved task, with no pass-rate
regression and a pinned confidence interval.**

**Branch:** `feat/codemode-v4`, cut from `origin/main` (`f280955`, v0.8.5). main has **no codemode at
all** — the whole v1–v3 line lives on `feat/codemode-v3` (76 commits). This plan builds on main and
**ports** the proven machinery from that branch rather than re-inventing it (port map in §9).

**Status:** design, not approved for implementation. Nothing here is committed to a release until the
phase-5 measurement passes its predeclared gate (§7).

---

## 1. What v1–v3 taught, and the constraint each lesson imposes

Every row is something we measured, not something we believe.

| # | lesson | evidence | constraint on v4 |
|---|---|---|---|
| L1 | An **additive** code tool loses to the direct tools it competes with | campaign 2026-11-27: with a shell present, control 25/25 vs codemode arms 39/50; at 124 files 5/5 @ 20.0 turns vs 7/8 @ 30.2 | the code tool must be **exclusive** — every other capability is script-only when mode=on |
| L2 | Removing the alternative is what makes it win | `changeset-noshell` pinned: 0.65 tokens/solved [0.55, 0.77], 13/13 vs 12/13, `need n` 7 | exclusivity is the design, not an option: no "keep shell direct" escape hatch in mode=on |
| L3 | A catalog without mutations cannot express the interesting program | v2 gap 2; read→transform→write was impossible until v3 added `stageEdit` | the script gets **every** verb: read, write, edit, rename, delete, shell, web, mcp |
| L4 | Synchronous host dispatch makes `await` decorative | `prelude.js` `__moca_invoke` returns an int; `Promise.all` is a loop; a slow call blocks past the deadline | dispatch must support **concurrent independent calls** (§4.3), or the program is slower than the turns it replaces |
| L5 | Prompt bytes are the scarce resource, not the API | 661 B description, ≤768 B cap, one cache reset per user per edit; v2's 541 B cap | **discovery at runtime** (`searchTools`/`describeTool`), not shapes in the prompt |
| L6 | Verify-after-apply is a real failure class | 4 of 5 codemode failures in the campaign were "verified, then staged more"; a clause moved 2/5 → 7/8 | mutations apply **after** the program returns and the result carries the image, so a program checks its own work (v3 P0 kept) |
| L7 | A whole-batch **consent** is workable; per-call approval is not | v3's single aggregated question; `TestChangesetConsent` matrix | one consent per program, listing every operation, with a receipt |
| L8 | Re-reads are the hidden cost | every results table's `redundant read tok` column | `store`/`load` persisted in the session |
| L9 | Underpowered nulls are expensive | P0: 0.82 point estimate, CI [0.61, 1.18], `need n` 126 — $11 to pin a wash | the gate needs a **minimum effect**, not just a CI below 1.00 (§7) |
| L10 | Harnesses lie before the model does | 4 grader defects voided two campaigns (`content: null`, legacy double-count, `local work` shadowing the workdir, `codemode` counted as a mutation for a read-only script) | every gate leg ships with a mutation check; `-run` patterns are asserted against real test names |
| L11 | The guest must never widen authority | v2/v3: no new capability, each scripted call maps 1:1 to an existing tool and its policy | §5.1 invariant, tested by comparing effective capability sets on vs off |
| L12 | Resume must refuse a surface change | v2's resume matrix: a stored workspace session resumes only as workspace | mode on/off mismatch is a typed refusal before the first provider call |
| L13 | Description edits cost every user a cache reset | SPECS §14, description caps | one frozen prompt contribution per release, generated, measured |

## 2. Pi and OpenCode, and what we take from each

| | Pi | OpenCode | **moca v4** |
|---|---|---|---|
| declared | `code` (+ whatever is `direct`) | one `execute` | **exactly one: `code`** |
| exposure | per-tool `direct \| codemode \| deferred` | separate catalog that "cannot be called directly" | per-tool `Direct`/`Scriptable` flags in the builtin registry; mode=on flips every `Direct` to false |
| nested pipeline | real `ctx.executeTool` — validation, hooks, permissions, audit (`nestedCalls`) | `toolCalls` audit, typed errors | the existing `Registry.Run` + `CommandChecker` + `PathChecker` path, unchanged; a bounded `calls` audit on the result |
| concurrency | promises via a worker | `Promise.allSettled` + `timeoutMs`/`maxToolCalls` | phase-batch-replay dispatch (§4.3) |
| discovery | `ALL_TOOLS`, `searchTools`, `describeTool`, namespaces | runtime `search()` → full call paths | `ALL_TOOLS`, `searchTools`, `describeTool` generated from the registry |
| state | `store`/`load` per session entry | — | `store`/`load` in the session JSONL, bounded |
| default | `mode: "on"` | on | **`mode: "off"`** — the user's requirement: opt-in, and off is main |

Not copied: TypeScript type generation (Pi), Node/psychoscript runtime (OpenCode), a `fetch` global by
default, and OpenCode's `full_codemode` split. moca keeps its wasm reactor, its caps, and its
consent/journal machinery — those are the parts that made v3 safe.

## 3. Configuration

```jsonc
"codemode": { "mode": "on" }        // "off" (default) | "on"
```

- Strict decode (unknown keys rejected), `Standardize` argv offsets preserved.
- Optional budgets with defaults, validated at config time:
  `"maxScriptMs"` (default 10000, ≤30000), `"maxToolCalls"` (per program, default 64, ≤256),
  `"maxOutputBytes"` (default 32 KiB), `"maxPhases"` (replay cap, default 32, ≤64),
  `"storeBytes"` (default 64 KiB total).
- **No escape hatch.** A `"direct": ["shell"]` key was considered and rejected on L1/L2: it recreates
  the measured losing configuration. An operator who needs a direct tool runs `mode: "off"`.
- Effective-switch rule (v2 lesson): a session stores `codemode: "on"` in its header; resuming under a
  different mode is a typed refusal naming both ways out, before any provider call.

## 4. Architecture

### 4.1 The seam (what changes on main)

| file | change |
|---|---|
| `internal/config/config.go:110-122` | add `Codemode CodemodeConfig` beside `Shell`/`MCP`/`Web`, plus validation and `Default*` helpers |
| `internal/tools/tool.go:330-370` | `Registry` gains `Scriptable() []Capability` and `Call(ctx, env, capability, args)`; `Tool` gains an optional `scriptableMeta` (name, category, shape) |
| `internal/agent/start.go:473` | `tools.NewRegistry(tools.Builtins()...)` → when mode=on, register `codeTool` instead, and keep the builtins in a **capability registry** the code tool owns |
| `internal/agent/agent.go:425` | unchanged: it already asks the registry for `Specs()`, which in mode=on returns one |
| `internal/agent/system-prompt*.md` | one generated contribution in mode=on only; mode=off prompt bytes unchanged |
| new `internal/codemode/` | reactor, bridge, prelude, dispatch (ported from v3, re-shaped) |
| new `internal/tools/code.go` | the `code` tool: admission, dispatch, receipt assembly |
| new `internal/tools/changeset.go` + `changeset_journal.go` | ported from v3, verbs extended (§4.4) |

### 4.1a The one dependency

main has **no** wasm dependency at all (`go.mod` on `f280955` lists none; `go.sum` has no `wazero`).
v3 runs the guest on `github.com/tetratelabs/wazero v1.12.0`, which is pure Go — no cgo — so the six
release targets (`linux/darwin/windows × amd64/arm64`) keep cross-compiling exactly as they do today.
Two port details:

- v3 lists wazero as `// indirect` while importing it directly; the port makes it a **direct**
  requirement (`go mod tidy` must agree).
- the prelude is embedded text, not a runtime file: no new asset to ship, and the guest cannot read it
  from disk (which is the point).

If a reviewer objects to any dependency, the fallback is v3's other engine shape (a synchronous
in-process interpreter), which loses the memory boundary — do not take that trade without saying so
out loud.

### 4.2 The surface, mode=on

Declared: `code` only. Its description is **generated** from the capability table and capped at
768 B; it names the idiom, the discovery calls, and the safety rule, not the per-capability shapes.

Script-visible (`tools.*`), all generated from the registry:

```
read, search, ls, glob, write, edit, rename, delete, shell, web, mcp.<server>.<tool>
```

Discovery/state/output globals: `ALL_TOOLS`, `searchTools(q)`, `describeTool(name)`,
`store(k,v)`, `load(k)`, `text(v)`, `log(v)`, `exit(code)`, `sha256(text)`.

Not scriptable: skills, todos, plan-mode changes — anything that mutates the session rather than the
tree. That boundary is deliberate: the script changes files and runs allowlisted commands; it does not
change the agent.

### 4.3 Dispatch: phase-batch-replay

The wasm guest cannot suspend, and a synchronous host call (v3) makes concurrency impossible (L4).
v4's answer, without a coroutine:

1. The guest runs; every `tools.x()` enqueues a request `{phase, seq, capability, args}` and returns a
   promise-like placeholder.
2. When the script needs a value it cannot compute, the guest returns/blocks with a sentinel: the
   phase ends.
3. The host executes the phase's batch: reads in parallel through a bounded pool (default 4), mutations
   in **declaration order**, `shell` strictly serial. Every call goes through the normal pipeline.
4. The host re-runs the script with a memo of resolved results, keyed by `(phase, seq)` — not by
   arguments, so two identical reads stay two reads, and a retry is deterministic.
5. Repeat until the script returns normally, or `maxPhases`/`maxToolCalls`/deadline trips.

Properties the tests must pin: identical results across replays for a pure script; a mutation phase is
never executed twice (memo prevents it; the journal records one batch); replay cost is bounded (the
guest is pure JS with no I/O — 30 phases of a 100-file script is milliseconds).

**Fallback if replay proves fragile:** keep the same guest API and run single-phase synchronous
dispatch (v3 semantics). The guest API is the contract; dispatch is an implementation detail behind a
test that both modes produce identical results for the same script.

### 4.4 Mutations

Ported from v3 unchanged: `PathPolicy` (refusals before any read), pre-image snapshot, whole-set
preflight, **one** aggregated consent, atomic apply through one `os.Root`, read-back verification,
fsynced journal, receipt. Verbs added in v4:

| verb | semantics | why |
|---|---|---|
| `write({path, content})` | whole-file replace, refuses if the file is not in the session's tracked set or if growth exceeds the cap | header/licence sweeps; exact-edit matching is fussy where content replacement is natural (L3) |
| `edit({path, edits:[{old,new}]})` | v3's `stageEdit`, content of the post image returned (L6) | the one verb that needs exactness |
| `rename({from,to})` | same-directory by default; cross-directory needs the target policy to admit both | models reach for shell only because this was impossible |
| `delete({path})` | refusal when the path is a protected name (`.env`, `.git/…`, config, AGENTS.md — same list as edits) | same |

Every verb returns `{staged, path, bytes, sha256, content|truncated}` so a program can check its own
work (L6), and the batch applies only after the program returns.

### 4.5 Shell and MCP inside the script

- `shell({command})` runs the session's `CommandChecker`; allowlisted commands run, others need the
  batch consent — and a shell call marks the **whole batch** as mutation-class for consent purposes,
  because a command can do anything.
- Headless (`-p`) with no approver: only allowlisted commands (v3's tested matrix, unchanged).
- `mcp.<server>.<tool>` calls the MCP registry with the same permission path. **MCP tool specs are
  never declared to the model** in mode=on — that is the Pi 40-tool trick, and the largest prompt win
  available to moca (L5).

### 4.6 Caps and error taxonomy

Typed errors with the remedy named (v2 lesson, OpenCode's shape):
`Limit{script|calls|output|phases}`, `Timeout{script|call}`, `Refused{policy|consent|path}`,
`Mismatch{edit}`, `Budget{inspection}`, `Script{syntax|throw}`, `Host`. Each carries `code`, `message`,
`suggestion`. The receipt carries `calls` (bounded to 64 entries: capability, path/command summary,
bytes), never file contents.

## 5. Safety invariants (each is a test)

1. **No new authority.** The effective capability set of a mode=on session, projected through the same
   `PathChecker`/`CommandChecker`, equals the mode=off set. Test compares the two projections, not the
   prose.
2. **One consent per program.** Every mutating verb lands in one question listing every operation; a
   refusal discards the whole batch and names the remedy.
3. **Read-back is mandatory.** A write that does not read back byte-identical is an abort, not a
   warning (ported from v3, with its corrupt-write fake).
4. **The journal is fsynced** and one line per begin/record/abort/finish.
5. **The transcript carries the return value and the receipt only** — never a staged image, never file
   contents (v3 P0's rule, with the gate leg that greps for a marker).
6. **No secret ever reaches a receipt.** Redaction runs on the same code path as `applog`.
7. **Guest output and memory are bounded** (32 KiB result, 64 KiB store, 4 MiB heap), and a runaway
   script is killed by deadline, not by hope.
8. **Mode mismatch refuses before a provider call** (L12).
9. **Sandbox escape is not in scope to claim.** The script gets exactly the tools the session already
   had; wasm is a memory/allocator boundary, not an OS sandbox. State it in SPECS rather than implying
   otherwise.

## 6. Definition of "better than off"

Mechanism (why this should win where v1–v3 lost):

- **No per-call declaration in the prompt** — the catalog is asked for at runtime (L5).
- **One turn per phase, not per call** — the model's loop count, which is what actually costs money
  (§ L4 evidence: turns did not fall when it errored).
- **`store`/`load`** — a second script does not re-read what the first already read (L8).
- **Parallel reads** — 96-file programs stop being 96 serialized round-trips (L4).
- **No discipline gap** — verification happens inside the program against the staged image (L6).
- **No competing tool** — the model cannot take the cheap path out (L1/L2).

Metric: `tokens/solved`, `steps`, `wall time`, pass rate, on two scenarios with **different shapes**:

- `v4-bulk` (write-heavy): the 121-service derived-changeset workload, no shell needed.
- `v4-triage` (read-heavy): a 200-file analysis whose answer needs a join across files, where the
  direct path needs many turns and the program needs one.

## 7. The predeclared gate (decide before spending)

Mode=on vs mode=off, same pinned model (`opencode-go/deepseek-v4.1-flash`), **n = 20 per arm, both
scenarios** (≈ 80 runs, ≈ $3–4 at measured per-run costs; cap $8).

**v4 ships iff, on both scenarios:**

1. pass rate(on) ≥ pass rate(off) − 1 run (non-inferior);
2. `tokens/solved` upper bound **< 0.85** — a minimum effect, not P0's 0.82-with-an-interval-spanning-1.0 (L9);
3. no new failure class in the on arm that off did not produce (read the transcripts, not the table);
4. every §5 invariant green, and the two gates (v4gate + selftest) green.

**If it fails: v4 does not ship, the plan records the outcome, and codemode stays a documented
no-execution placement (phase-7 result) rather than a default.** No fixture rewriting to make it pass;
the phase-7 report is the precedent for what that looks like and why it is not allowed.

`need n` from `evalstats -compare` decides a top-up; a top-up is capped at 10 runs per arm and one
round only.

## 8. Phases (each ends with gates and a commit; no phase starts on a red phase)

### Phase 0 — skeleton and the off-identity proof

Config key, validation, mode plumbing, and the golden fixtures that make "off is main" a test rather
than a promise.

- `internal/config`: `CodemodeConfig{Mode string}` + validation + `example.jsonc` entry + SPECS §4.
- Capture fixtures from main **before** any surface code: the declared `Specs()` JSON and the system
  prompt bytes for a default session. `TestModeOffDeclarationIsMain` hashes both.
- Wire `start.go:473` behind the flag; mode=off path untouched.

**Accept:** full `-race` suite green, `make vet`, count pins, the golden test proves off == main.

### Phase 1 — engine, read catalog, discovery

- Add `github.com/tetratelabs/wazero` as a **direct** requirement (it is `// indirect` on v3), and pin
  `GOOS/GOARCH` cross-builds in CI so "pure Go, no cgo" stays true.
- Port the reactor/bridge/prelude from `feat/codemode-v3` (map in §9), rename `moca.invoke` to a phase
  queue.
- `Registry.Scriptable()` + `describeTool` metadata for read/search/ls/glob/web.
- `searchTools`/`describeTool`/`ALL_TOOLS`, typed errors, caps.
- `v4gate`: hermetic legs — one program does read+search+ls; unknown tool refuses with the list;
  attempt/time/output caps; no key in any output; determinism across replays; a slow script dies at
  the deadline with `Timeout`, not a hang.

**Accept:** v4gate green including a mutation check on each refusal (make the refusal pass, watch the
leg fail), 9→1 declared tool in the provider request, prompt contribution ≤ 768 B.

### Phase 2 — mutations

- Port `changeset.go`/`changeset_journal.go`; add `write`/`rename`/`delete`; content in results (L6).
- One aggregated consent; receipt; read-back; the path policy list shared with `edit`.

**Accept:** gate legs for apply/refuse per mode, a read-back corruption abort, a rename+delete batch,
the extra-file rule, and the journal's four record kinds.

### Phase 3 — shell and MCP

- `shell` through `CommandChecker`; batch consent; serial execution; audit entry.
- `mcp.*` through the registry, specs hidden from the prompt.

**Accept:** allowlisted runs, non-allowlisted refuses headless with the remedy named, MCP tool names
absent from the provider request, present in `ALL_TOOLS`.

### Phase 4 — state, idiom, budgets

- `store`/`load` in the session JSONL, bounded; `describeTool` output generated.
- The frozen prompt contribution: idiom (batch independent calls, filter in the script, check the
  staged image, `store` what you will reuse) + safety (the host applies after return).
- Resume matrix.

**Accept:** store survives two scripts in one session and not across sessions; contribution byte count
pinned; resume-mismatch leg.

### Phase 5 — measurement and decision

- Fixtures `v4-bulk`, `v4-triage` with selftest arms and mutation-proven checkers.
- The gate in §7.

**Accept:** the OUTCOME block written in this plan with the four reads and the verdict; SPECS, README
and the campaign doc updated; count pins.

## 9. Port map from `feat/codemode-v3`

| take | from | change |
|---|---|---|
| wasm reactor, memory views, `__moca_invoke` ABI | `internal/codemode/{engine.go,prelude.go,prelude.js}` | dispatch becomes a phase queue; keep the memory/alloc discipline verbatim |
| bridge + capability table, aptitudes | `internal/tools/codemode.go` | table generated from `Registry.Scriptable()`, not hand-listed |
| changeset core, journal, PATH policy, GuardImage, consent, receipt | `internal/tools/{changeset.go,changeset_journal.go}` | verbs extended; nothing else |
| gate harness shape (scripted provider, binary driving) | `test/codemodegate/` | becomes `test/v4gate/` |
| eval arm configs, selftest patterns, campaign reports, fixtures | `test/evals/*`, `docs/plans/2026-11-2*` | reused as-is where the scenario still matches |

| leave behind | why |
|---|---|
| additive / transport exposures, the 9th declared tool | L1/L2: the measured losing design |
| per-variant descriptions and their caps | one generated description instead (L5) |
| `codemode.enabled` / `exposure` / `workspaceEdits` config keys | v4's `mode` replaces them; v3 is not merged, so there is no migration to honour |

## 10. Risks, and what each one costs if it lands

| risk | mitigation | if it lands anyway |
|---|---|---|
| replay dispatch is fragile (state in a re-run script) | memo by `(phase,seq)`, no I/O in the guest, determinism test, single-phase fallback behind the same API | ship the fallback: no concurrency, but exclusivity and discovery remain |
| exclusive mode is a security regression (one script can do everything a session could) | §5 invariants: same policy, one consent, journal, read-back, caps; SPECS states wasm is not an OS sandbox | offer `mode: "off"` as the conservative default (it already is) |
| models write bad programs | typed errors with remedies, bounded attempts, examples in discovery output | measure `repeated fails` per run; if the on arm's error loop rate is high, teach one idiom in the contribution, not ten |
| prompt contribution drifts from the table | generated from the registry, byte-pinned by a test | a drifted contribution is a failing test, not a mystery |
| the measurement is underpowered again | n=20/arm designed from the observed cv (27–32 %) and a **minimum effect** of 15 % (L9) | one top-up round, then decide on the point estimate *as a decision*, recorded as such |
| porting v3 brings its quirks | the port map is explicit; v3 code that exists only to serve additive/transport is dropped | — |

## 11. Out of scope

- No change to mode=off behaviour, prompt bytes, or declaration order — enforced by the Phase 0 test.
- No new OS-level sandbox claim; no network beyond the existing `web` capability and its policy.
- No second declared tool in mode=on, no "direct tools" escape hatch (L1/L2).
- No TypeScript codegen, no Node runtime, no `fetch` global by default.
- No release until the §7 gate passes; no version bump (release-time only).

## Appendix A — the four measured facts this plan is built on

1. Additive codemode loses where a shell exists: control 25/25 vs 39/50 across two sizes; at 124 files
   5/5 @ 20.0 turns, $0.0344 vs 7/8 @ 30.2 turns, $0.0475.
2. Exclusive codemode (no shell) wins, pinned: 0.65 tokens/solved [0.55, 0.77], cost 0.83 [0.77, 0.89],
   steps 0.83 [0.73, 0.94], 13/13 vs 12/13, `need n` 7.
3. Returning the staged image moved passes 7/8 → 13/13 with point estimates 0.82/0.93/0.99 and an
   interval that spans 1.00 at n=13 — the effect is real but small, which is why §7 asks for 15 %.
4. Four grader defects voided two campaigns before any model result was trustworthy: a `content: null`
   crash, a double-counted legacy entry, a `local work` that shadowed the trial workdir, and
   `codemode` counted as a mutation for a read-only script. Every new gate leg in this plan ships with
   a mutation check because of them.

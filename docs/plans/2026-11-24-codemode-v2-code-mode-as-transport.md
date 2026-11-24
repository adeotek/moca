# codemode — implementation plan, v2.0 (code mode as transport)

Status: **design only, unapproved.** Supersedes the v1 design premise ("codemode is a ninth tool
beside the eight"), not v1's implementation: v1 is built, qualified, hermetically gated and parked
unmerged (`feat/codemode-t2`). This plan reopens exactly one frozen thing — the model-facing tool
list — and nothing else.

## Why v2 exists: six rounds measured the v1 premise and it is wrong

Task 7 Stage A spent **US$0.3382 of its US$20 ceiling** across six predeclared rounds on two models
(`test/evals/BASELINE.md` carries every table):

| round | lever | adoption | scripts/trial | steps vs control | carry ratio |
|---|---|---|---|---|---|
| 1 | v1 description as planned | 2/6 | — | — | 0.87 |
| 2 | result shapes named in the description | 5/10 | 3.8 | 10.4 / 6.4 | 0.77 |
| 3 | "ONE program" batch rule + result-side echo | 7/10 | 3.4 | 9.0 / 6.3 | 0.59 |
| 4 | `tools.glob` + a worked example in the prompt | 7/10 | 4.0 | 10.1 / 6.3 | 0.85 |
| 5 | second model (`glm-5.3-flash`), same binary | 4/10 | 3.5 | 9.2 / 6.1 | 0.82 |
| 6 | one script per message, enforced | 8/10 | 1 + refusal | 9.5 / 7.2 | 0.99 |

Adoption moved 2/6 → 8/10; **no round ever beat the control on turns, on either model.** The carry
saving is real and reproducible (2 106–2 555 tokens against 3 006–3 062 when adopted) and never
recovered the turns. Round 6 is the decisive datum: refusing the second script *cost* a turn (7/10
trials spent one being refused, then finished with direct tools), because the shortcut was still
shipped — the model was punished for taking a path the tool list allowed.

**The v1 premise is that the model will choose to batch. It does not, and it never will while every
direct call remains one keystroke away.**

## What Pi and OpenCode do instead (read from the installed trees, 2026-11-24)

| | Pi `@earendil-works/pi-codemode` + `pi-coding-agent/dist/extensions/codemode/tool.js` | OpenCode (binary strings, `core/codemode`) |
|---|---|---|
| tool surface | `tools.<name>(args)` for **every** tool; nested calls run the real pipeline (`ctx.executeTool`) with hooks, permission checks, usage accounting and a `nestedCalls` audit record (≤256, args capped) | one `execute` tool plus a **catalog**: "the tools listed below … cannot be called directly: they only work as JavaScript inside the `execute` tool"; everything else is direct and must not be scripted |
| exclusivity | per-tool `exposure: direct \| codemode \| deferred`; `codemode`-exposed tools are callable from scripts and **never declared to the model** | catalog entries are code-only by construction; catalog refresh is announced when it changes |
| discovery | `ALL_TOOLS`, `searchTools`, `describeTool`, `describeNamespace` | global `search(...)` returning full call paths, "do not guess tool names" |
| state | `store()`/`load()` persist JSON across calls (session entries) | — (extensions/modules instead) |
| concurrency | promises settled by a worker thread; `Promise.allSettled` is the documented idiom | promises in a confined runtime; `Promise.allSettled` in the tool description |
| context | only the script's output and return value reach the model | same; result carries `logs`, `truncated`, `toolCalls` |
| defaults | `codemode: { mode: "on", inlineBudget: 3000 }` | catalog always on |

**The two invariants both designs depend on: exclusivity (the code-mode toolset is not directly
callable) and parity (anything in the catalog is scriptable — mutations, MCP, HTTP).** v1 has
neither: it adds a tenth tool with three read-only primitives while `read`/`search`/`ls`/`shell` stay
first-class. That is why six rounds of prompt and description work could not buy a turn.

## What v1 got right and keeps

- The reactor, the async-IIFE contract, the failure ABI, the attempt/timeout/entry caps, the
  containment work, the cloud of hermetic tests, `tools.glob`, the result shapes, and the whole
  eval instrument (trial harness, carry measurement, `-cap`, predeclaration discipline).
- `builtins.go`'s eight tools and `internal/tools/testdata/schemas.golden.json` stay **byte-identical**.

## Decisions

**D12 — codemode becomes a transport, behind its own switch.** `codemode.exposure` is a new config
key: `additive` (default; today's behaviour, the tool list is unchanged) or `transport` (the
inspection tools are callable only from a script). `codemode.enabled` keeps its meaning. A user who
never sets `exposure` sees no change at all — which is also what keeps v2 measurable against v1.
**Applies to: config, SPECS §4/§10/§15.**

**D13 — exposure is a property of the tool, not of the registry.** `ToolSpec`/`Tool` gains
`Exposure() Exposure` (`Direct` | `Codemode`). `Registry.Declared()` is what the model is told about;
`Registry.Callable()` is what the bridge may dispatch. In `transport`, `read`, `search`, `ls` (and
`glob`, which is codemode-only anyway) are `Codemode`; `write`, `edit`, `shell`, `web`, `mcp` stay
`Direct` in v2.0. MCP deferral (the 40-tool case) is **v2.2**, not this plan. `Builtins()` and the
schema golden do not move: eight tools exist, six are declared. **Applies to: tools/registry, agent
start, SPECS §10.**

**D14 — mutations inside a script are pre-authorized or refused, never asked.** Inside a script the
`Ask` path is unreachable (a loop must not produce an approval storm — the same reasoning as v1's
D8). A scripted `write`/`edit`/`shell` runs when the configured mode and the existing checkers would
run it without asking; otherwise it fails with the typed class `needs_approval` and the model is told
to perform that one operation directly, where the approval flow still exists. This preserves the
gate's "zero questions" invariant while making read → transform → write possible in one program.
**Applies to: codemode capability table, failure ABI, gate.**

**D15 — parity is scoped to what a task needs, not to everything.** v2.0 adds `write`, `edit`,
`shell` to the capability table behind `codemode.mutations` (default `false`). With it off, a
transport session can read and analyse in one program but must mutate with direct tools — a
deliberately smaller first step that still fixes D12's core defect. `web` inside a script is v1.1's
`network` and keeps its own switch; MCP tools are v2.2.

**D16 — no promise engine in v2.0.** Tool calls stay synchronous host calls (`prelude.js`'s
`inspect()` → `__moca_invoke` → dispatch). The economics v2 is chasing are **turns**, not wall-clock:
twelve reads in one script are one turn whether they run serially or in parallel. A promise-based
host bridge (real `Promise.allSettled`, cancellation, per-call deadlines) is **v2.1** and is planned
around, not assumed: the prompt must not promise concurrency the sandbox cannot deliver, and
`Promise.all` over values is documented as sequential.

**D17 — discovery replaces catalogue compression.** The 541-byte spec is a catalog squeezed into
prose because the model cannot ask. v2.0 adds two prelude globals — `tools.list()` (names, one line
each) and `tools.describe(name)` (the tool's description and argument shape) — served from the same
capability table the bridge dispatches, so they cannot drift. The spec itself shrinks to the call
protocol plus those two globals. Successive rounds of this pilot spent their budget re-teaching
shapes in the prompt; this is the structural fix for that.

**D18 — the v2 pilot measures a different endpoint, predeclared, and only after the design is
built.** Primary: **host tool dispatches per trial** (the count the feature exists to collapse) and
**turns**, against the same task under `additive`. Adoption is no longer a metric — in `transport`
there is no alternative path. Secondary: the v1 carry ratio and the pass rate. Billed once, US$20
ceiling requested separately, predeclaration written before the first trial. **Applies to: Task 6.**

## Contract

### Configuration

```jsonc
"codemode": {
  "enabled": true,          // v1, unchanged
  "network": false,         // v1.1, unchanged
  "exposure": "transport",  // NEW: additive (default) | transport
  "mutations": false        // NEW: scripted write/edit/shell (default false)
}
```

Strict decode means every key is documented, defaulted and validated; `exposure` outside the two
values and `mutations` without `enabled` are decode/validation errors with the same message shapes as
`network`'s (SPECS §4). A trusted project may veto either new key (`false`); a project `true` alone
warns, exactly as `ResolveCodemode` does today.

### Model-facing tool list

| mode | declared tools | callable from scripts |
|---|---|---|
| `additive` | read, write, edit, shell, search, ls, web, mcp, codemode | read, search, ls, glob (+ web when `network`) |
| `transport` | write, edit, shell, web, mcp, codemode | read, search, ls, glob (+ the three mutations when `mutations`) |

`codemode` is always last. `Builtins()` and `schemas.golden.json` stay byte-identical; a new pin
(`TestDeclaredToolsUnderTransport`) asserts both rows above, and `TestCodemodeWireDelta` gains a
transport arm so the provider's serialized tool array is pinned for the smaller list too.

### Capability table and the `needs_approval` class

The table gains rows behind flags: `write`/`edit` (path-checked, snapshot-guarded, `needs_approval`
when the mode would ask) and `shell` (command-checked, same rule, plus the existing command timeout).
New failure class `needs_approval` joins the allowlist of typed classes; like every other class it
carries no host control flow and `Result.Steered` stays host-set. Scripted mutations are recorded in
the audit block below, so the transcript never hides what a script changed.

### Audit block on the outer result

Pi's shape, adopted because it answers the question a user actually asks — *what did that script
touch?*

```
[codemode: 14 calls — read×12 search×1 write×1 | changed: services/api/service.yaml]
```

Counts by name, plus the paths a scripted mutation wrote (up to 5, then `+N more`). No file contents,
no command text (SPECS §13.5 still governs logs; this is a result, and it names paths the model can
already see). Cap: 5 paths, 200 bytes, then `+N more`.

### Budgets

| budget | v1 | v2.0 |
|---|---|---|
| spec | 541 B / 576 | ≤ 576 B (shrinks: D17 moves the catalog out) |
| prompt render (transport) | 6 508 B / 6 528 | ≤ 6 528 B; the batch rule is replaced by the transport rule (see Task 4) |
| script source | 64 KiB | unchanged |
| result | ≤ 2 000 runes + 512 reserve | unchanged, plus the audit block |
| entries / calls / timeout | 20 000 / 50 / 1–30 s | unchanged |
| `tools.list()` | — | ≤ 40 lines, 2 KiB, `+N more` |

### Security

- Scripted mutations reuse `PathChecker`, `Snapshotter` and `CommandChecker` — no new bypass.
  `needs_approval` is the only new path and it refuses rather than prompts.
- The audit block is derived from the bridge's own ledger, not from guest text.
- `tools.list()`/`describe()` are served from the capability table; an unknown name is typed
  `unknown_tool`, never a lookup into the registry (a script cannot enumerate tools it may not call).
- The gate gains: a transport leg where a direct `read` is not declared; a `needs_approval` leg in
  `ask` mode; an audit-block leg with a scripted write in `yolo`; and a jail leg where a scripted
  write outside the root is refused with `containment`.

## Tasks

**Doc counts (every task):** every commit that adds or removes a `.go` file refreshes AGENTS.md's
Go-file count in the same commit, staging the file first (`git ls-files` reads the index); a new
package also refreshes SPECS §15's package and test lines. The mirror copy
(`internal/skills/builtin/self-knowledge/references/specs.md`) follows every SPECS edit.

### Task 1 — Exposure in the registry (unbilled)

`Tool`/`ToolSpec` gain `Exposure()`; `Registry` gains `Declared()`/`Callable()`; `agent/start.go`
passes the effective `exposure` into the registry build and the codemode adapter; `declare_test.go`
pins both rows of the table above, including `codemode` last; `Builtins()` and
`schemas.golden.json` byte-identical.

- [ ] **Red first:** a transport registry declares six tools and callable-lists nine; an unknown
      exposure value fails config decode; `additive` is byte-identical to today's declared list.
- [ ] Gate: `go test ./internal/tools ./internal/agent ./internal/provider -race -count=8`.

### Task 2 — Capability parity and `needs_approval` (unbilled)

Rows for `write`/`edit`/`shell` behind `mutations`; `needs_approval` in the class allowlist; the
bridge's ledger records mutations and paths. The prelude's new globals are Task 3's business.

- [ ] **Red:** a scripted write inside the root succeeds in `yolo` and appears in the audit block; the
      same write in `ask` fails `needs_approval` with zero questions; a write outside the root is
      `containment`; a shell command in `ask` that would ask fails `needs_approval`; a command the
      checker allows runs; the snapshot hook still fires for a scripted write.
- [ ] Gate: `go test ./internal/tools ./internal/codemode -race -count=8`.

### Task 3 — Discovery globals (unbilled)

`tools.list()` / `tools.describe(name)` served from the capability table; the spec shrinks; the
prompt's batch rule is replaced (Task 4) with the transport rule.

- [ ] **Red:** `list()` returns one line per callable tool and no direct tool; `describe("read")`
      returns its shape; `describe("write")` fails `unknown_tool` when `mutations` is off;
      `list()` over the 2 KiB cap truncates with `+N more`; the spec shrinks under 576 B.
- [ ] Gate: `go test ./internal/tools ./internal/codemode -race -count=8`.

### Task 4 — Prompt, surfaces and resume (unbilled)

`internal/agent/{prompt.go,system-prompt.md}`: in `transport`, the codemode section states the rule
("the inspection tools exist only inside a script; write one program per task") and drops the
"never one call per file" coaching — with no direct alternative there is nothing to coach. Session
header stores `exposure`/`mutations` so a resumed session keeps its tool list (same rule as v1's
network prelude, R69-shaped). ACP/TUI read the declared list, not `Builtins()`, so a transport
session's client sees six tools.

- [ ] **Red:** a resumed transport session declares six tools and refuses direct `read`; switching
      the config off mid-session does not retroactively widen the list; the prompt render stays under
      the 6 528 B budget; ACP's `tools/list` matches the declared set.
- [ ] Gate: `go test ./internal/config ./internal/session ./internal/agent ./internal/tui ./internal/acp ./cmd/moca -race -count=1`.

### Task 5 — Fixture with real economics (unbilled)

`test/evals/sweep-bulk/`: a generated fixture of **120 services × 3 files** (a `gen.py` run at trial
setup, so git holds the generator, not 360 files), the same question shape as `sweep`, a checker that
verifies every line of the answer, plus `config.cm-{additive,transport}.jsonc` whose arms differ only
in `codemode.exposure` (one-key rule, `TestPilotArmsDifferOnlyInCodemode` extended to the new pair).

- [ ] **Red:** the generator is deterministic (same tree hash twice); the checker fails on an answer
      missing one service; a `PILOT` marker keeps it out of default enumeration.
- [ ] The trial row gains a `dispatches` field (integer) parsed from the outer result's audit block
      (`[codemode: N calls …]`), because Task 6's primary is that count, not a token ratio; a row
      without the block records `dispatches: 0` and the comparison refuses a pair where the
      treatment arm measured none.
- [ ] Gate: `bash test/evals/run.sh --selftest sweep-bulk`.

### Task 6 — Pilot (billed, US$20 ceiling requested separately)

Predeclared in `BASELINE.md` before the first trial. 10 trials/arm on `sweep-bulk`, `-j 1`,
`-cap 20`, same corpus model (`opencode-go/deepseek-v4.1-flash`), same binary for both arms.

- **Primary:** median **host tool dispatches per trial** ≤ 0.5× the `additive` arm, **and** turns
  ≤ the `additive` arm.
- **Secondary:** carry ratio ≤0.75, no lost pass, and the audit block naming exactly the services the
  checker expects.
- **Stopping rule:** one round. If the primary fails, the conclusion is that transport does not pay
  either, and the codemode line is closed with two negative results (additive and transport) rather
  than one.

### Task 7 — Docs and release gates (unbilled)

SPECS §1/§2/§4/§6.5/§10/§11.0/§13.5/§14/§15/§16, SECURITY (scripted mutations + `needs_approval`),
README, AGENTS (the "eight frozen tools" sentence becomes "eight built-in tools; a transport session
declares six"), CHANGELOG Unreleased, three mirrors, recount, `graphify update .`, full verification.

## Non-goals

- **Parallel tool calls** (promise engine) — v2.1; D16 explains why it is not the economics.
- **MCP deferral** (the 40-tool catalogue) — v2.2; the exposure model is built to allow it.
- **Network inside a script** — v1.1's `codemode.network`, unchanged.
- **Changing the eight tool schemas** — untouched; only the declared list moves.
- **A third model, or another prompt round on the additive design** — both measured, both closed.

## Risks and rollback

| risk | mitigation |
|---|---|
| A model that cannot write JS is locked out of inspection in `transport` | `additive` stays the default; the switch is documented as an advanced mode; gate leg asserts the failure is typed and recoverable |
| Scripted mutations hide changes from the transcript | audit block names paths and counts; `needs_approval` keeps approval-requiring writes in the approval flow |
| Prompt-cache reset on every transport session (smaller tool array) | one-time per session; `TestCodemodeWireDelta`'s transport arm pins the array |
| v1's parked branch diverges | v2 branches from `feat/codemode-t2`; v1's tests, gate legs and BASELINE record are the regression suite |
| The pilot shows nothing again | the stopping rule ends the line after one round; the docs then carry two negative results |

## Verification

```bash
make build && make vet && make test -race
go test ./internal/tools ./internal/agent ./internal/codemode ./internal/provider -race -count=8
bash test/codemodegate/run.sh          # + transport, needs_approval, audit, jail legs
bash test/evals/run.sh --selftest      # sweep, sweep-search, sweep-bulk
go test -tags counts ./cmd/moca -run TestSpecsTestCount -count=1
```

# codemode — implementation plan, v2.0 revision 3 (code mode as transport)

Status: **ready for implementation.** Tasks 1–5 and 7 are unbilled. Task 6 is the single billed task
and needs its own approval (`US$20` ceiling). Revision 1 (`…-code-mode-as-transport.md`) and revision 2
(`…-code-mode-as-transport-v2.md`) are retained as the review record; their `R1–R8` and `D12–D18`
identifiers are **struck** — the codemode plan family already owns `R1–R71` (v13, SPECS) and this
revision renumbers from `R72`/`D19` so a citation is unambiguous.

## What revision 3 changes (revision-2 review, folded)

| id | finding | disposition |
|---|---|---|
| **R72** | The acceptance criterion was unsatisfiable: 360 files ÷ `inspectAttempts = 50` per script = **≥8 scripts** (`internal/tools/inspect.go:31`), while the comparator could batch with `shell` in 3–5 calls — `BASELINE.md:1399` already records that shell one-liners read several manifests per call. `8 ≤ 0.5 × 5` is false. | **D21** makes the inspection budget a dial with a task-sized default under transport (400) and **D26** makes the pilot a 2×2 factorial with a paired sign test, so the budget and the exposure switch are told apart instead of confounded. |
| **R73** | Partial exclusivity reproduced v1's failure mode: transport hid `read`/`search`/`ls` but left `shell`, which is strictly more powerful for the same task, so a competent model could pass without ever calling `codemode` — and then fail the "one successful codemode use" rule. | **D22**: `shell` stays direct (removing it would remove command execution entirely, since v2.0 has no scripted mutations), and the pilot runs in a **shell-restricted configuration** — `-p` denies every command that would need approval (`internal/tools/tool.go:146`) — so the claim is scoped honestly to *inspection without host command execution*. |
| **R74** | The narrowed claim was not stated where a reader looks. | Goal and Non-goals now say read-only inspection transport; mutation bundling is explicitly out of scope. |
| **R75** | A resumed transport session under a non-transport policy became six tools, a refusal-only codemode and **no inspection at all** — a dead session on a core workflow. | **D24** refuses the resume with a typed error and two named remedies. |
| **R76** | `R1–R8` collided with the family's existing `R1–R71`. | This revision starts at `R72`; revision 2's rows are struck. |
| **R77** | `D12–D18` were redefined with different meanings than revision 1. | This revision starts at `D19`; revision 1/2 decisions keep their numbers only inside their own documents. |
| **R78** | The freshness stamp would hash the whole file per windowed read, against a 256 MiB source budget, once per `read({next})` page. | **D23** computes one stamp per file per execution (memoised), with the cost stated and a paged-read test. |
| **R79** | The additive network description advertises `tools.web.fetch/search` while `workspaceCapabilities()` has no web handler — a promise that refuses at dispatch. Preserving it "as a pin" would preserve the lie. | **D25** removes the web calls from the network description until v1.1 lands; the golden and SPECS move in the same commit. |
| **R80** | Medians at n=10 cannot separate 3 calls from 4, and the pass/adoption filters could silently shrink an arm to three trials. | **D26**: paired sign test plus a predeclared effect size, a minimum completed-trial floor, and a written "inconclusive" outcome. |
| **R81** | The spend guard was described as running "after completed trials"; it runs **before each launch**, per label (`spend_total`/`spend_guard`, `_lib/trial.sh:193`), so three labels × `-cap 20` is a US$60 hole. | Task 5 adds a **campaign ledger**: `-cap-scope <glob>` sums every results file the scope matches. |

## Why v2 exists

Six predeclared rounds spent US$0.3382 of Task 7's ceiling and established one fact
(`test/evals/BASELINE.md`, all tables): **v1 was adopted more and used worse.** Adoption went 2/6 →
8/10, the carry saving was real (2 106–2 555 tokens against 3 006–3 062), and no round ever beat the
control on turns, on either model. Round 6 refused the second script per message and lost worst of
all (ratio 0.99, steps 7.2 → 9.5) because the direct tools were still shipped — it punished the
shortcut instead of removing it.

Pi and OpenCode do not persuade the model; they remove the alternative. Pi's `exposure: direct |
codemode | deferred` lets a tool be callable from scripts and never declared; OpenCode's catalog says
its tools "cannot be called directly". Both expose **every** tool inside the sandbox, so one program
can read, transform and write. Revision 1 carries the full comparison with the package paths it was
read from (`@earendil-works/pi-codemode`, `pi-coding-agent/dist/extensions/codemode/tool.js`, the
OpenCode binary's `core/codemode` strings).

## Goal

With `codemode.exposure: "transport"`, workspace inspection is reachable **only from JavaScript**, and
one script carries a whole task, so a 360-file join costs 1–3 model-issued calls instead of 120+
direct reads — measured on a shell-restricted configuration, where the comparison is between
inspection paths and nothing else.

Transport as specified is a **package**: the exposure switch (D19/D20) and a task-sized inspection
budget (D21). The pilot is a 2×2 factorial (D26) precisely so the two can be told apart before either
ships — if the dial alone delivers, the switch is cut.

## Non-goals (v2.0)

- **Scripted mutations** (`write`/`edit`/`shell` inside a script) — the product question "can one
  program do the whole task" stays open; it returns only with a mutation fixture and its own plan.
- **Hiding `shell`** — it is the only command execution path until scripted mutations exist.
- **A promise engine** (real `Promise.allSettled`) — v2.1; the economics here are turns, not
  wall-clock, and the prompt must not promise concurrency the sandbox cannot deliver.
- **MCP deferral** (the 40-tool catalogue that motivates Pi's `deferred`) — v2.2.
- **Network inside a script** — v1.1's `codemode.network`; this revision only stops advertising it
  before it exists (R79).
- **Changing the eight tool schemas**, a one-script-per-message quota, or another additive-design
  prompt round — all measured or closed.

## Decisions

**D19 — exposure is a config switch.** `codemode.exposure`: `"additive"` (default) | `"transport"`.
Strict decode; an unknown value is a decode error; `transport` without `enabled: true` is a
validation error. A trusted project may veto either value (`additive` or `false`); a project value
alone warns, with the message shape `ResolveCodemode` already uses. **A user who never sets it sees no
change.**

**D20 — exposure is a property of the tool, and only the declared list moves.** `Registry` learns
exposure through an **optional** interface — `interface{ Exposure() Exposure }`, absent ⇒ `Direct` —
so `internal/tools/builtins.go` and `testdata/schemas.golden.json` stay byte-identical. `Specs()` is
what the provider is given and what `control.go:324` charges; `Registry.CodemodeOnly(names...)` wraps
the named tools, `ExposureOf(name)` reports the flag and `Names()` keeps the dispatch surface visible
to tests. There is deliberately **no** registry-level `Callable()`: the dispatcher is codemode's
capability table, not the registry, and a second authority would be a place to drift. A direct call to
a hidden tool is refused with "available only inside a codemode script". In transport,
`read`/`search`/`ls` are `Codemode`-exposed; `codemode` stays last. `glob` was never declared and
remains a capability.

**D21 — the inspection budget is a dial with a mode-dependent default.** New config key
`codemode.inspectAttempts` (50–2 000, default **50** for additive and **400** for transport; an
explicit value overrides either default). Additive's script is a step, so 50 bounds a batch;
transport's script *is* the task, so the cap must be able to cover one. The other budgets are
unchanged and still bind: 20 000 entries, 256 MiB source bytes, 64 KiB script, 1–30 s deadline. The
401st attempt in a 400-budget script latches at `limit`, naming the budget.

The dial exists because the pilot must separate the two variables it changes: transport is a package
(exposure **and** budget), and a second additive arm at 400 attempts answers whether exclusivity adds
anything once the budget is task-sized — see D26.

**D22 — `shell` stays direct; the pilot is shell-restricted.** The claim this revision tests is
*inspection* transport, so both arms run with commands that would need approval **denied** (headless
`-p` has no asker — `internal/tools/tool.go:146`), and the checker asserts zero successful `shell`
calls in every arm. A shell-enabled comparison is a different question (structure, not call count) and
is not claimed here.

**D23 — inspection parity is preserved, and the stamp is memoised.** A scripted `read` in transport:
records a **read stamp** for the whole file from the same open descriptor (size, mtime, SHA-256),
transfers it to `ReadTracker` only after a bounded reply, and sets `TestSeen`/clears `FailingTest`;
a scripted `search` sets `Searched` even with no matches; grants come from the same jail list
`prepare()` already computes (`globalSkills`, `builtinDir`, `promptsDir`, `spillDir`, `configFile`,
`start.go:182`). One stamp per file per execution — a 30-page `read({next})` sequence hashes the file
once, and the memo states that in a test. Additive keeps today's "inspection records no read"
contract (`TestCodemodeAsksNothingSpillsNothingAndRecordsNoRead`).

**D24 — a transport session does not resume into a wider or narrower surface.** If the stored header
says transport and the current policy is `additive` or `off`, `moca --continue` **exits with a typed
error** naming the two remedies (re-enable `transport`, or start a new session) instead of running a
six-tool agent that cannot inspect. The reverse (additive header, transport now) also refuses, for the
same reason: the frozen prompt and the declaration must agree.

**D25 — the network description stops advertising what it cannot dispatch.** `codemodeSpec(true)`
drops `tools.web.fetch/search` until v1.1 implements the capability rows; the web golden (628 B) and
SPECS move together. The `network` config key stays, with `ResolveCodemode`'s warning behaviour
unchanged, so v1.1 is additive again rather than a rewrite.

**D26 — the pilot is a 2×2 factorial, decided by pairs.** Four arms on `sweep-bulk`, all with commands
denied: `cm-off` (control), `cm-additive` (exposure additive, 50 attempts), `cm-additive-400`
(additive, 400), `cm-transport` (transport, 400). The two factors are **exposure** (direct inspection
available or not) and **budget** (50 or 400); the four arms cover the cells that matter.

- **Primary:** `cm-transport` uses **strictly fewer model-issued calls** than `cm-additive` in **≥ 8 of
  10 paired trials** (exact one-sided sign test, p ≈ 0.055 at 8/10) **and** the median ratio ≤ 0.5.
- **The factorial read, also predeclared:** if `cm-additive-400` is within 20 % of `cm-transport`, the
  budget was the fix and the exposure switch is not worth its complexity — ship the dial, drop
  D19/D20, and record that. If `cm-additive-400` sits near `cm-additive` while transport is far below,
  exclusivity is what pays, and the switch ships.
- **Secondary:** turns not higher; carry ratio ≤ 0.75 with ≥ 512 saved; no lost pass; checker green.
- **Inconclusive is a written outcome:** fewer than 9 completed pairs, or fewer than 7 passing trials in
  an arm, and the pilot reports nothing rather than a winner. One round, no threshold relaxation.
- **Cost:** 40 trials ≈ US$0.16 at the corpus model's measured rate; the `US$20` ceiling is a cap, not
  an estimate.

## Contract

### Configuration

```jsonc
"codemode": {
  "enabled": true,            // v1, unchanged
  "exposure": "transport",    // NEW: additive (default) | transport
  "inspectAttempts": 400,     // NEW: 50..2000; default 50 (additive) / 400 (transport)
  "network": false            // v1.1 key, unchanged; no web calls in the description until v1.1
}
```

### Declared tools

| `exposure` | declared to the model | callable from a script |
|---|---|---|
| `additive` | read, write, edit, shell, search, ls, web, mcp, codemode | read, search, ls, glob |
| `transport` | write, edit, shell, web, mcp, codemode | read, search, ls, glob |

`Builtins()` and `schemas.golden.json` are untouched; `TestDeclaredToolsTransport` and
`TestDeclaredToolsAdditive` pin both rows, `TestCodemodeWireDelta` gains a transport arm, and ACP's
call rendering gains a case for a transport session (fewer names, `codemode` still mapped by
`toolKind`).

### Failure classes

No new class. Scripted inspection keeps `refused` / `invalid_args` / `containment` / `limit` /
`cancelled` / `script`; budget exhaustion is `limit` with the message naming the configured budget, so
the model can see why its next read failed.

### Resume

| header | policy now | result |
|---|---|---|
| additive | additive | resume as today |
| transport | transport, `enabled: true` | resume, six declared, four callable |
| transport | additive / off | **refuse** (`resume_surface_mismatch`), exit non-zero, two remedies named |
| additive | transport | **refuse** (`resume_surface_mismatch`), same shape |

### Security

- Scripted reads respect `PathChecker` and the jail from `prepare()`; grants are typed and tested.
- The stamp is computed from the open descriptor, never by reopening the pathname.
- No mutation path exists in v2.0, so no approval flow changes; the zero-question invariant stands.
- `transport` cannot silently widen: the declared list is a pure function of the stored header and the
  current policy, and the mismatch case refuses.

## Tasks

Doc counts in every commit: a new `.go` file refreshes AGENTS.md's count in the same commit (stage it;
`git ls-files` reads the index); a new package refreshes SPECS §15's package/test lines; the mirror
(`internal/skills/builtin/self-knowledge/references/specs.md`) follows every SPECS edit.

### Task 1 — Exposure model and the declared list (unbilled)

**Files:** `internal/tools/tool.go` (optional `Exposable`, `Registry.Specs()` filtering,
`Registry.Callable()`), `internal/agent/start.go` (effective exposure into the registry build),
`internal/config/{config.go,project.go,sample.jsonc}` (both new keys: `exposure` values, and
`inspectAttempts` 50..2000 with its two mode-dependent defaults), `internal/session/entry.go`
(`CodemodeExposure`), `internal/agent/{resume.go,start.go}`, `internal/provider/wire_delta_test.go`,
`internal/tools/codemode_test.go`, `internal/acp/toolcall_test.go`.

- [ ] **Red:** transport declares six tools with `codemode` last; additive is byte-identical to today;
      `Names()` still lists all nine in both modes (the bridge is unchanged — exposure moves the
      declaration, not the dispatcher) and a direct call to a hidden tool is refused; an unknown
      exposure fails decode;
      `inspectAttempts` outside 50..2000 fails validation and its default follows the mode unless set;
      `transport` without `enabled` fails validation; a project `true` alone warns and a project
      `false` vetoes; the wire-delta transport arm is positive and < 1 KiB.
- [ ] Gate: `go test ./internal/tools ./internal/agent ./internal/provider ./internal/acp -race -count=8`.

### Task 2 — Transport read parity and the memoised stamp (unbilled)

**Files:** `internal/tools/codemode.go` (bridge ledger, grants, flags, and the budget handoff:
`mustInspection` currently constructs the Inspection with package constants — the holder's caps must
carry `inspectAttempts` into `NewInspection` as a field, so a script's budget follows the config),
`internal/tools/inspect.go` (stamp capture on the open descriptor, grants, `attempts` as a field),
`internal/tools/readtracker.go` (a `Record` entry point that does not reopen),
`internal/tools/read.go` / `search.go` (flag semantics to mirror), `internal/tools/inspect_glob_test.go`
(budget table).

- [ ] **Red:** a scripted `read` in transport satisfies a later `edit` of the same file
      (`existing file was never read` no longer fires); a scripted read of a file that changed after
      the read does not satisfy it; a 30-page `read({next})` hashes the file once (counter in the
      bridge); a read of the config file / a global skill file is granted and one outside the jail is
      `containment`; a scripted search sets `Searched` even with no matches; `TestSeen` is set by a
      scripted read of `*_test.go`; additive records no read and sets no flags (existing test
      unchanged); the default budget is 400 in transport and 50 in additive, an explicit
      `inspectAttempts` overrides both, and the next attempt past the budget latches at `limit`
      naming it.
- [ ] Gate: `go test ./internal/tools ./internal/codemode -race -count=8`.

### Task 3 — Resume refusal (unbilled)

**Files:** `internal/agent/{resume.go,start.go}`, `cmd/moca/*` (the error path), `internal/acp/toolcall_test.go`
(a pin, no behaviour change: `codemode` is already mapped to `execute` in `toolKinds`).

- [ ] **Red:** all four rows of the resume table; the refusal is typed, names both remedies and exits
      non-zero **without a provider call**; a transport session's ACP stream renders its `codemode`
      call as `execute` (regression pin); no TUI change is expected — the TUI enumerates no tool list.
- [ ] Gate: `go test ./internal/agent ./internal/session ./internal/tui ./internal/acp ./cmd/moca -race -count=1`.

### Task 4 — The fixture with real economics (unbilled)

**Files:** `test/evals/sweep-bulk/{prompt.txt,followup.txt,check.sh,gen.py,testdata/…}` (the generator
sits beside `check.sh`, not inside `fixture/`, so it never lands in the tree the agent inspects),
`config.cm-{off,additive,additive-400,transport}.jsonc`, `test/evals/run.sh` (`fresh_repo` runs
`gen.py` when present), `cmd/moca/pilot_arms_test.go` (four-arm rule), `PILOT` marker.

- [ ] **Red:** `gen.py` is deterministic (same tree hash twice) and generates 120 services × 3 files
      (+ `platform/defaults.yaml`, 361 files, 39 351 B measured);
      the checker rejects an answer missing one service and one with a wrong number; the four configs
      differ **only** inside `codemode` (`off` has no block; `additive` has `enabled:true`;
      `additive-400` adds `inspectAttempts:400`; `transport` is `enabled:true` +
      `exposure:"transport"`, whose default budget is already 400), and
      `TestPilotArmsDifferOnlyInCodemode` pins exactly that intra-block difference; the scenario is
      excluded from default enumeration.
- [ ] Gate: `bash test/evals/run.sh --selftest sweep-bulk`.

### Task 5 — Campaign ledger (unbilled)

**Files:** `test/evals/run.sh`, `test/evals/_lib/trial.sh` (`-cap-scope <glob>`; `spend_total` sums
every matching results file; the preflight refuses a pilot with `-cap` but no scope when more than
one arm is named), `--selftest` cases.

- [ ] **Red:** two labels under one scope share the ceiling (a synthetic 0.4 + 0.4 total stops a
      launch at cap 0.7); a scope matching nothing refuses; the single-label behaviour is unchanged.
- [ ] Gate: `bash test/evals/run.sh --selftest`.

### Task 6 — Pilot (billed; US$20 ceiling, own approval)

**Predeclared in `BASELINE.md` before the first trial.** `sweep-bulk`, 10 two-prompt trials per arm,
four arms (`cm-off`, `cm-additive`, `cm-additive-400`, `cm-transport`) under one `-cap-scope`, model
`opencode-go/deepseek-v4.1-flash`, one binary, `-j 1`, commands denied (D22). Records:
`BASELINE.md` gets the protocol, the table and the verdict; SPECS gets the outcome sentence.

- **Primary and factorial read:** exactly D26's rule, decided before the run.
- **Stopping rule:** one round. If the primary fails, transport does not pay and the codemode line
  closes with two negative results (additive and transport); if `additive-400` wins, the dial ships and
  the exposure switch is cut. No round 2, no threshold relaxation.

**Outcome (run 2026-11-24; full record in `test/evals/BASELINE.md`).** `v2pilot-off` 4/10,
`-add` 6/10, `-add400` 8/10, `-xport` 9/10; US$0.3242 of the US$20 ceiling. The primary **failed**:
transport carried less in 7/10 pairs (needed 8, p = 0.172) with a median pair ratio of 0.778 (needed
≤ 0.5) — the predeclared "inconclusive" outcome. Transport does not ship; `exposure` stays in the tree
at the `additive` default. The dial result is the campaign's clean effect (`add400` vs `add`: 8/10,
median 0.376, 14.0 vs 19.7 steps) and is unaffected by the defect below; whether 400 becomes the
additive default is an owner decision, not a pilot outcome. **Defect in the fixture, recorded:** 11 of
12 failures were the untouched-tree rule, which `prompt.txt` does not state — so the pass column is
not a capability figure. Before any pass-rate endpoint: state the rule in the prompt and grade the
answer before the tree in `check.sh`. That fix and any re-ask are unbilled work plus a fresh
predeclaration; nothing here is re-litigated by re-reading these rows.

### Task 7 — Docs and release gates (unbilled)

SPECS §1/§2/§4/§6.5/§10/§11/§13.5/§14/§15/§16, SECURITY (declared-list rule, resume refusal),
README, AGENTS ("eight built-in tools; a transport session declares six"), CHANGELOG Unreleased,
mirror, recount, `graphify update .`, full verification block.

## Risks and rollback

| risk | mitigation |
|---|---|
| The model writes failing JS and cannot inspect at all in transport | errors are typed and the model can retry; `additive` remains the default; the gate pins the failure text |
| A transport session surprises a user who turned codemode off | D24's typed refusal with two named remedies, before any provider call |
| The 400-attempt budget lets one script stall a run | the deadline, entry and source budgets still bind; the gate pins the 401st latch |
| The pilot shows nothing again | D26's "inconclusive" outcome and one-round stop; the docs then carry two negative results |
| v1's branch diverges | v2 branches from `feat/codemode-t2`; v1's tests, gate legs and BASELINE record are the regression suite |

## Verification

```bash
make build && make vet && make test -race
go test ./internal/tools ./internal/agent ./internal/codemode ./internal/provider -race -count=8
bash test/codemodegate/run.sh
bash test/acpgate/run.sh && bash test/goalgate/run.sh
bash test/evals/run.sh --selftest
go test -tags counts ./cmd/moca -run TestSpecsTestCount -count=1
```

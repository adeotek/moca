# codemode plan v3 — review

**Subject:** `docs/plans/2026-11-22-codemode-v3.md` (378 lines), compared with `docs/plans/2026-11-21-codemode-v2.md` (143 lines).
**Baseline:** `main` at `v0.8.4-4-g07727fe`, product code `81fc50d`.
**Method:** every file/line citation in v3 was checked against the tree; the pinned artifact (`qjs-wasi-reactor.wasm`, QuickJS-NG `v0.18.0`) was downloaded and parsed (imports, exports, memory section); the reactor's source (`qjs-wasi-reactor.c` at the tag) and wazero `v1.12.0` docs were read. External claims about OpenCode/Pi were not re-verified — they carry no weight in the design.

**Verdict:** v3 fixes v2's weakest link (the subprocess + cgroup/Job-Object platform contract) and should keep that direction. As written it cannot compile or run: two mechanical blockers, one missing capability in the vendored artifact, and a live security regression that v2 had caught by name. It also silently widens scope from v2's read-only inspection to effectful nested calls, and deletes v2's benefit gate without saying so.

---

## Blockers

### B1 — `tools` ↔ `codemode` import cycle

Global Constraints: "`internal/codemode` … imports nothing internal except `internal/llm`". Task 3 puts the adapter at `internal/tools/codemode.go` (`tools` → `codemode`); Task 4 defines `codemode.Bridge(reg *tools.Registry, env *tools.Env)` (`codemode` → `tools`). A cycle, and a break of AGENTS.md's tier rule (`tools` is a mutually-independent leaf tier). v2 avoided this structurally: `internal/codemode` dependency-free, adapter in `internal/codemode/tool`, a `go list -deps` test.

**Fix:** declare the boundary interfaces in `codemode` (`Dispatcher`, `ShouldStop`), implement them in `tools`, keep the adapter in `internal/tools/codemode.go`, and add a deps test asserting `go list -deps ./internal/codemode` mentions no `internal/` package.

### B2 — the vendored artifact has no seam for Go host functions

Parsed from the pinned binary:

- imports: `wasi_snapshot_preview1` only (21 functions: `fd_read`, `fd_write`, `fd_readdir`, `path_open`, `path_filestat_get`, `path_rename`, `path_unlink_file`, `poll_oneoff`, `clock_time_get`, `proc_exit`, …)
- exports: **297**, including `memory`, `_initialize`, `qjs_init`, `qjs_init_argv`, `qjs_get_context`, `qjs_destroy`, and ~290 `JS_*`
- memory: min 34 pages (2.1 MiB), no max encoded

There is no guest→host import, and wazero resolves imports at instantiation only (`JS_NewCFunction2` requires a *guest* function pointer). So "injected global functions that dispatch into a `tools.Registry`" (Task 4, and the architecture paragraph) is impossible with the upstream release asset. It requires a **custom-built** reactor that declares one import module (e.g. `moca.invoke`) and a C shim that marshals JS calls into it. That directly contradicts F5 ("Nothing else is compiled into it") and Task 1 ("verify … against the published release asset").

Two more consequences of the same fact:

- v3's F5 describes the exports as "`js_QuickJS`-shaped … (init / eval / get_result / free)" and Task 1 Step 3 asserts "`js_*` exports only". Both are wrong; that test fails against the real artifact.
- The reactor has no eval export at all: the host must drive `JS_Eval`/`JS_ExecutePendingJob` through `qjs_get_context()`. Doing that from Go means ~150 lines of QuickJS C-API calls per script. The shim belongs in C.

### B3 — one shared module is a one-shot

`qjs-wasi-reactor.c` holds `static JSRuntime *reactor_rt; static JSContext *reactor_ctx;`, and `qjs_init` returns `-1` when already initialised. Independently, wazero's `WithCloseOnContextDone` docs state that on cancel/timeout "the api.Module from which the functions are derived is made closed", and `api.Function.Call` "is not goroutine-safe".

v3's Task 2 Step 4 asserts the opposite ("one runtime, one module, per-process, and the interpreter's own state is per-`Execute` invocation"). Three problems: not re-entrant, permanently bricked by the first timeout or esc, and unsafe under ACP's concurrent sessions.

**Fix:** compile the module once, **instantiate per `Execute`**, close the instance after each call. That makes the `WithCloseOnContextDone` close benign, gives each script its own linear memory (hence its own `reactor_rt`), and keeps `Call` single-goroutine per instance.

Related, minor: `mod, _ := rt.Instantiate(...)` swallows the error that matters most (min-pages > limit, OOM at instantiation).

### B4 — `JS_EVAL_TYPE_MODULE` does not allow top-level `return`

The plan promises "top-level `await` and `return` work as pi's do", and its own example ends `return rows.split('\n').filter(...).length`. Top-level `return` is a SyntaxError in a module; top-level `await` is a SyntaxError in a script. One eval cannot have both.

**Fix:** evaluate a script-mode wrapper `(async()=>{` + src + `\n})()` with the prefix newline-free so reported line numbers stay aligned, then pump `JS_ExecutePendingJob` until the promise settles. `await` and `return` both work; bare-expression results are not supported and the description says so.

---

## Regressions against v2

### R1 — the read-tracker edit stamp is live and unaddressed

v2's finding, verbatim in the tree: `internal/tools/read.go:90` calls `env.Reads.Record(abs)` before the window is chosen, while `internal/tools/write.go:40` and `internal/tools/edit.go:54` only `Check` it. A script read therefore authorises a direct `write`/`edit` of a file the model never saw — and v3's bridge calls the real `read` tool, so a 50-call script can stamp dozens of paths the model has no window into. v2 prescribed save/restore around every nested dispatch. v3 says nothing.

### R2 — `write` and `edit` are in the bridge, undisclosed

Task 4's objective names five tools (`read`, `search`, `ls`, `web`, `mcp`); Step 3 says the dispatch table is "a literal map over the **seven** allowed names, with `shell` and `codemode` … absent". Eight builtins minus `shell` is seven — i.e. `write` and `edit` included. That is effectful nested execution, which v2 deferred to a separate proposal with a journal, undo, crash-`unknown` semantics and a mandatory steering barrier. It also contradicts v3's own Review Focus ("The tool computes pure data by default").

### R3 — steering is gone

`internal/agent/agent.go` still applies steering only after a whole tool batch; `PendingSteering` is at `internal/agent/control.go:230`. v2 made a nested-admission check mandatory before exposing effects. v3 exposes effects with up to 200 nested calls and no check.

### R4 — approvals collapse at scale, and `canAlways:false` has no mechanism

`Question.CanAlways` is set by the *called tool* (`internal/tools/tool.go:86`; `internal/mcp/tool.go:146` sets it true), not by the caller. The bridge can only force it false by wrapping `Env.Ask`, and no task's file list includes that wrapper. With `web`/`mcp` bridged, `ask` mode can emit up to `maxToolCalls` prompts per script, and a denied call is a catchable rejection the script can retry in a loop.

### R5 — display text as data re-opens v2's finding 5

v3 returns "exactly what the real `read` tool returns" — `N|content` plus a `[lines A-B of TOTAL]` footer (`internal/tools/read.go:28–33`), `search` clipped at 100 hits with a 5 000-hit spill cap (`internal/tools/search.go:19–21`), `ls` cut at 1 000 with `[… N more]` (`internal/tools/ls.go:61–68`). v2 explicitly forbade parsing that and carried `complete`/`next`/machine-readable reasons so a partial sweep could not read as an exhaustive answer. v3 has no coverage concept: silent truncation becomes a silent wrong answer.

### R6 — 4 MiB cannot do the job it exists for

The module's floor is 2.1 MiB; `WithMemoryLimitPages(64)` leaves ~1.9 MiB of JS heap. The feature's premise is holding a large tool result to filter it; one `read` window is capped at 50 K chars but `bindings` are uncapped in count and size. The memory budget is the one number the plan asserts without measuring.

### R7 — the resume table shrank

v2's four-row table (fresh off/on; resume missing bit; started-on-now-disabled with a **refusal-only** adapter so stored prompt and schema still agree) is one line plus `TestCodemodeResumeNeedsClear` for off→on only. Started-on → config-off is undefined, and it is the row with a real failure mode: the stored prompt advertises the tool while the registry builds eight.

---

## Evidence and process

### P1 — the benefit gate is deleted

v2's merge condition was a pilot on the same binary with predeclared thresholds, and "no signal ⇒ leave the candidate unmerged". v3 ships straight to a PR. The Out-of-scope line — "a scenario joins only if it would regress without the tool's presence" — is circular: that is exactly what cannot be known without running it.

### P2 — no dedicated hermetic gate

The repo carries one gate per contract (`goalgate`, `acpgate`, `contextgate`, `skillgate`, `shipgate`). v2 specified `test/codemodegate/` with canaries and a leak scan. v3 bolts one leg onto `skillgate`, whose stated contract is the self-managed-skills guard, and lists `test/evals/run.sh --selftest` among its gates, which exercises nothing in this feature.

### P3 — SPECS edits are incomplete

Task 8 touches §1/4/10/11.0/13.5/16. `docs/specs/SPECS.md:254` (§6.5 wire shape) and `:498` (§14 turn contract) both say "the eight tool(s)" — on the subscription route that is an actual wire contract (one namespace group). Also no enabled-path prompt instruction is specified anywhere; "byte-identical for `PromptInput{}`" is trivially true and pins nothing about the on delta. v2 named the exact sentence (`internal/agent/system-prompt.md`, the batch rule) and explicitly warned off the `{{rtk}}` line.

### P4 — Task 8 is guaranteed red

It edits `CHANGELOG.md` and re-copies `internal/skills/builtin/self-knowledge/references/specs.md`. `internal/skills/sync_test.go` compares **three** mirrors — `specs.md`, `readme.md`, `changelog.md` — so the changelog copy fails `TestSelfKnowledgeReferencesAreByteIdentical`. v2 said "three mirrors"; v3 kept one.

### P5 — version pinned in prose

"codemode (v0.8.6, opt-in)" in the §11.0 sentence and a `v0.8.6`-shaped CHANGELOG heading. The tree is at `v0.8.4-4`, CHANGELOG has an `Unreleased` section, and the house rule is a version per release, not per PR.

---

## Factual and citation errors

| v3 claim | Reality |
|---|---|
| F5 + Task 1 Step 3: exports are "`js_*` … init / eval / get_result / free" | `qjs_init`, `qjs_init_argv`, `qjs_get_context`, `qjs_destroy` + ~290 `JS_*` |
| F3 / Tech Stack: wazero is "MIT", "MIT; A/2" | Apache-2.0; the "A/2" fragment is garbled |
| `tools.Spill` at `internal/tools/tool.go:105` | `internal/tools/overflow.go:21`, signature `(env, kind, full)` |
| `ResolveSelfManagedSkills` at `internal/config/config.go:161` | `:128` |
| Task 6 calls "the jail's own `resolveDeep`" from `internal/tools` | unexported, `internal/permissions/jail.go:150` — not callable across the boundary |
| Design snippet `tools.Codemode(mgr, env)` | Task 3's signature is `Codemode(rt, reg, env)`; no `cfg.CodemodeOn()` exists in the design (it is `config.ResolveCodemode`) |
| "Decision 1/2/3", "F0", "§7", `ToolsInputFileTool` | none exist in v3 (v2 artifacts); the test is `TestBridgeNoPathsNoFileSurface` |
| Task numbers | canAlways is "Task 6" (bridge is Task 4); the `agent_test.go:349` edit is "Task 7" (it is in Task 5); Task 3 "consumes … the bridge, Task 5" (Task 4); Sequencing says "Tasks 2 and 4 strictly before Task 5" while the adapter in Task 3 needs both |
| Task 7: "all four entry surfaces" | three listed; headless (`-p`) has no file or test |
| Task 5: `TestSampleCoversCodemode` "no new test needed" | names a test that then does not exist |
| Config: user-layer `codemode.dataFiles` | contradicts Review Focus 1's "no project block ⇒ no path surface at all" |

---

## Minor

- `toolChoice` is a frozen schema field documented as a no-op — a permanent contract for nothing.
- Caps are "fixed by the build, not by config" *and* call parameters; `bindings` has no count or size cap.
- `wallMs` inside model-visible content makes otherwise-identical results byte-different (cache churn); it belongs in `Summary`.
- `bindings` + `dataFiles` never define how a binding name matches a glob.
- No perf budget. QuickJS under wazero's interpreter is ~10× slower than a native runtime; the risk table answers that with "switch to the compiler", which the portability rationale had rejected.

---

## What v3 gets right and v4 keeps

- **In-process, subprocess-free containment.** No cgroup v2 delegation, no Windows Job Object, no macOS exclusion, no worker protocol/handshake/reaping/version-skew code, one vendored artifact, six release targets unchanged, hard cancellation through `WithCloseOnContextDone`. This is the correct call and it is v3's main contribution.
- **Opt-in by the `skills.selfManaged` shape** (user enables; a trusted project file may only veto).
- **A ninth tool with its own golden schema**, not a quiet rewrite of the frozen eight.
- **Spill + cap on output**, riding the existing `tools.Spill`.
- **One outer `tool_result`**, with TUI/ACP rendering as one activity.

## Disposition

`docs/plans/2026-11-22-codemode-v4.md` implements the fixes: a custom reactor with a single `moca.invoke` import, per-`Execute` instantiation, a dependency-free `internal/codemode` with structural interfaces, structured inspection payloads instead of display text, read-only v1 scope, read-tracker save/restore, steering admission, a four-row resume table, the missing SPECS/mirror updates, a dedicated hermetic gate, and v2's benefit gate restored before the docs are written.

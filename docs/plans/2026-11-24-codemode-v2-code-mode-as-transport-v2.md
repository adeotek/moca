# codemode v2.0 — code mode as transport, implementation plan (revision 2)

**Status: design only; not approved for implementation or spend.** This is revision 2 of the [transport proposal](2026-11-24-codemode-v2-code-mode-as-transport.md), not a release or another revision of the earlier codemode v1 plan. Reviewed baseline: `77e3a16`; the runtime is the parked, unmerged `feat/codemode-t2` at `f35aa4b`. The predecessor is retained as the review record.

**Goal:** Find out whether routing workspace inspection through JavaScript reduces model-issued tool calls without costing work requests, correctness, or useful read access. The product comparison is with codemode **off**, not just with the other unshipped design.

**Architecture:** Keep the existing Go registry and QuickJS/WASM reactor. Give transport sessions a smaller direct registry and a separate, bounded inspection catalog. Connect successful inspections to the existing read/edit guards. Build the measurement before changing the behavior it measures.

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

This is evidence against more additive prompt tuning, not proof that transport wins. Direct shell batching was already competitive on `sweep` (`test/evals/BASELINE.md`, “What would change the answer”). Leave that alternative available in every arm.

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

The comparison suggests separating what the provider declares from what a script can call. It does **not** establish that moca needs mutation, private state, or concurrency. At this baseline codemode is the **ninth** tool, with four host-dispatchable workspace operations.

## What v1 got right and keeps

- The reactor, the async-IIFE contract, the failure ABI, the attempt/timeout/entry caps, the
  containment work, the cloud of hermetic tests, `tools.glob`, the result shapes, and the whole
  eval instrument (trial harness, carry measurement, `-cap`, predeclaration discipline).
- `builtins.go`'s eight tools and `internal/tools/testdata/schemas.golden.json` stay **byte-identical**.

## What revision 2 changes

| Review finding | Disposition and where it is tested |
|---|---|
| R1. Hiding `read` breaks existing-file edits and the failing-test protocol | Fix in the same change as exposure. Test successful, stale, refused and investigation-gated edits in Task 2. |
| R2. Inner dispatches and fixed trial turns measure the wrong thing | Count outer `tool_use` entries and assistant `Steps`; report inner operations and direct shell separately. Tasks 1 and 4. |
| R3. Changed paths cannot grade a read-only join; one script may not fit | Grade final prose and untouched tree; require a bounded, chunkable reference run through the binary before billing. Tasks 1 and 3. |
| R4. Scripted mutations have contradictory permissions and incomplete accounting | No mutation switch, mutation ABI or nested shell in this revision. Per-operation admission now; separate reopening criteria and audit fixture below. Task 2. |
| R5. Declared and scriptable sets, config veto and dispatch were underspecified | Enumerate both sets; define string precedence, header compatibility and direct-call rejection. Task 2. |
| R6. Special read roots and disabled resumes would strand the user | Preserve named read grants without inheriting unrestricted `yolo`; freeze declarations on resume and provide a fresh-session recovery. Task 2. |
| R7. Several verification targets did not test the promised behavior | Test actual provider requests and ACP events, existing wire tests, `make test`, and explicitly named pilot selftests. Tasks 2–5. |
| R8. Sequencing, discovery and the ship baseline | Instrument first; couple exposure with prompt/resume; document actual return shapes; compare the qualifying design with off. One separately approved round. Tasks 1–5. |

## Decisions and contract

### D12–D13. Configuration chooses a direct registry, not a new tool schema (R5)

The only new user/project key is `codemode.exposure`. Example user config fragment: `"codemode": {"enabled": true, "exposure": "transport"}`. Omitted means `additive`. Only the strings `"additive"` and `"transport"` are valid; reject booleans, explicit null, empty and unknown values at config load. Do not add `codemode.mutations`. Validate exposure even when disabled, then resolve the existing `enabled` and `network` switches with `ResolveCodemode`; an exposure value never enables codemode. Existing `network` semantics are not a new validation rule.

| Effective `enabled` | User exposure | Trusted project exposure | Fresh session |
|---|---|---|---|
| off (includes project `enabled:false` veto) | any valid | any valid | Off; preserve existing enable/veto diagnostics, no additional exposure message. |
| on | absent or additive | absent or additive | Additive. |
| on | absent or additive | transport | Additive; warn once that only the user can opt in. |
| on | transport | absent or transport | Transport. |
| on | transport | additive | Additive; note the project veto once. |

Ignore an untrusted project layer entirely. Preserve enable/network warnings when an exposure note also applies. Strict decode the same key in the user and trusted project paths; do not write defaults back to users' files.

**Ordered declarations and exact host catalog at `f35aa4b`:**

| Fresh mode | Direct tools, in provider order | Host-dispatchable inspection names |
|---|---|---|
| off | `read, write, edit, shell, search, ls, web, mcp` | none |
| additive | `read, write, edit, shell, search, ls, web, mcp, codemode` | `read, search, ls, glob` |
| transport | `write, edit, shell, web, mcp, codemode` | `read, search, ls, glob` |

The provider never declares `glob`. `tools.list()` and `tools.describe(name)` are transport-only metadata helpers, not additional workspace operations or provider tools. `write`, `edit`, `shell`, `web`, and the configured `mcp` proxy remain direct in transport. At this base the additive network **prelude** can name `tools.web.fetch/search`, but `workspaceCapabilities()` has no web handler (`internal/tools/codemode.go`). Preserve the additive network shape/refusal pins; do not count the stubs as callable, implement web, or grant network via exposure. For fresh transport use the workspace prelude and direct `web` even when network is configured; emit a note to that effect. A future real network bridge requires its own admission and compatibility review.

Build the direct registry from the ordered built-ins for the stored surface, excluding `read/search/ls` in transport, then replace the MCP stub as today and register the correct codemode variant last. This makes `Registry.Specs()` (both `agent.go` request construction and `control.go` context accounting) and `Registry.Run` agree **by construction**. A provider-emitted hidden name must fail the normal unknown-tool path, list only declared names, and produce one paired error result without touching disk or read state. Do not hand the full registry to JS or change `Tool`, `llm.ToolSpec`, `Builtins()` or the eight-schema golden.

### D14–D15. Inspection must still support a normal edit (R1, R4, R6)

**Successful transport reads count as session reads.** Preserve the structured payload and v1 cursor behavior. In `internal/tools/inspect.go` collect a separate full-file read stamp from the exact opened descriptor and raw bytes being scanned (size, modtime, SHA-256); the existing cursor digest covers only a span and is not a freshness stamp. Key it by the canonical path the direct edit/write resolver will check. Transfer a verified stamp to `ReadTracker` only after a successful bounded host reply; do not reopen the pathname after the scan to record potentially different bytes. Detect a replaced or changing file, binary input, invalid cursor, cancellation, budget exhaustion or response encoding failure; do not grant a **new** read on failure. A successful window of a regular text file can authorize an edit, as a direct window does; it is not evidence that every line was returned. Preserve exact-byte handling for CRLF, clipped lines and an empty file. Existing-file writes and edits still call `Reads.Check`; a new file still needs no read. The stamp is process-local, not restored from old transcript prose.

On successful transport inspection of a `*_test.go` file, set `TestSeen` and clear `FailingTest` as the direct read does; on a successful valid search, set `Searched`, including a valid search with no matches. `ls`, `glob`, discovery, guest text and refused calls do not satisfy those steps. An armed investigation still blocks a direct edit until **both** the test read and search occurred. Update the investigation banner and refusal to name the usable script operations in transport. Additive keeps its deliberate “inspection records no direct read” contract (`internal/tools/codemode_test.go`); do not change its golden output or guard timing.

**Read scope.** `prepare()` in `internal/agent/start.go` already knows the workspace, global and built-in skills dirs, prompts dir, the session's own spill dir and the exact config file. Pass those same read grants as typed directory/file grants into the transport bridge; do not derive them from the unrestricted `Env.Paths` in `yolo`. Relative paths stay workspace-relative. Accept an absolute workspace path or the explicitly granted external path (including the absolute path printed in a spill message). Use root-relative opens anchored in the selected host grant; file grants allow a read of that file only, not listing/searching its parent or following a replacement to a sibling. Bind cursors to grant identity as well as relative path. Never traverse from one root into another via a symlink. Do not mint approval or spill new files from the bridge. Reads of an approved file do not bestow write access: subsequent **direct** mutations still use the current mode, snapshot, skill and plan guards. Ordinary external paths outside those grants are deliberately unavailable to JS even in `yolo`; use a fresh additive session if the wider direct-read surface is needed.

**Admission and limits.** Keep the synchronous ABI and the existing outer checks. For transport put context/steering admission and the shared, latching 50-attempt budget at the host dispatcher **before** starting each validly framed nested operation, including lookup failures and metadata queries. Charge exactly once; refactor the primitive-local charging rather than charging both at entry and inside `Inspection`. Keep additive's existing cap and behavior. Recheck before publishing a read receipt; never derive `Steered` from guest text. Check the 50th/51st call, a mix of operation types, and steering queued between two calls: the second never starts, the first remains accounted for, the next outer result is paired and the steering is delivered once. Preserve the 20,000-entry and source-byte budgets, 64 KiB script limit, 1–30 s guest deadline, and slot recovery; do not claim the deadline preempts all host I/O.

**Deferred, not approved:** nested `write`, `edit`, and especially `shell`. Reopen write/edit only with a distinct read–transform–write fixture where direct edits demonstrably cost requests. That follow-up must get a separate opt-in; preserve same-mode direct behavior (ordinary in-root creation does not ask in `ask`), prohibit scripted writes outside the workspace even in `yolo`, convert an operation that actually requires a question into a refusal with zero asks, and keep snapshots, freshness, plan/skill guards and steering. Require per-operation admission/budget **before** effects, partial-failure tests, and host audit of operations and the paths actually written. Grade that fixture by the resulting tree; test the bounded audit separately. A shell receipt can prove a command ran, not every path it modified. It stays direct unless a separate design resolves that limitation. No `needs_approval` class is added by this plan.

### D16–D17. A small, discoverable script API (R3, R8)

No promise engine, concurrency promise, cross-script store, recursive dispatch or one-script-per-message quota. The transport prompt must say where inspection lives and when to split: **“Inspect through codemode. Batch related reads, return only what you need, and split work when a limit requires it. Edit and run commands directly.”** Update other transport-rendered read/search/verification and spill hints that would contradict it. Do not tell the model to use shell to evade inspection or insist on one program per task. Leave the off/additive prompts, spec variants and descriptions byte-identical.

Derive transport discovery from the same four capability rows that dispatch. `tools.list()` returns a sorted list of callable names; `tools.describe(name)` returns `{name, description, arguments, result}`. Include field names/types, optional fields, search semantics and continuation/completeness, not just input schemas: `read` → `lines[{n,text,clipped?}]`, optional `next`; `search` → `matches[{path,line,text,…}]` using case-insensitive substring matching, **not** direct search's RE2; `ls` → `entries[{name,kind}]`; `glob` → `paths`. All four expose `complete/reasons`. `read({next})` is exclusive with path/offset/limit and only valid in the same execution. Pin descriptors against actual returned objects and distinguish a caught unknown-name error (`unknown_tool`) from the outer reactor error; no guest-supplied class changes host control flow. Discovery cannot enumerate hidden implementations, MCP internals or future web stubs. Cap list output at 40 names / 2 KiB with an omission notice; cap a descriptor at 2 KiB and reject oversize catalog entries in tests rather than returning malformed shapes. Keep the transport spec ≤576 bytes and the minimal transport prompt fixture ≤6,528 bytes. Preserve v1's distinct additive network budgets. Keep summaries bounded inside the outer result budget; never take machine metrics from the rendered summary.

### D18. Header owns shape; current policy owns permission (R5–R7)

Persist an optional `codemodeExposure` on `session.Header` (write `transport` only). No codemode bit means off; legacy enabled headers without exposure mean additive. Never infer exposure from prompt text. Reject unknown nonempty header values explicitly. Fresh runs use the resolved config. On resume reuse the prompt bytes and direct registry order from the header:

| Stored surface | Current effective policy | Outcome |
|---|---|---|
| off | any | Eight direct tools; no retrofit. |
| additive | on, either exposure | Keep additive spec/prelude, four workspace ops and original bookkeeping; preserve stored/current network intersection. |
| additive | off | Existing refusal-only ninth tool; direct inspection still works. |
| transport | on + transport | Six direct tools, working transport bridge with current read grants; no stored permissions re-granted. |
| transport | off **or** additive (including project veto) | Same six and frozen prompt, refusal-only codemode; no engine, no root access, no hidden direct fallback. |

In that last row emit a startup/resume notice and a matching refusal: **“Inspection is unavailable in this resumed transport session. Start a fresh session without `--resume` or `--continue` to use the current tool set.”** Distinguish engine initialization failure from revoked policy and return an actionable diagnostic, not a suggestion to call hidden tools. Test a second process and a genuinely fresh session following the advice. TUI and ACP render actual calls, not a hypothetical tool-list endpoint; approval for direct mutations is unchanged.

## Tasks and acceptance gates

All work below is proposed, not executed. Tasks 1–3 are unbilled. Instrumentation may land on the experiment branch first; do not ship the feature to `main` between tasks. For each implementation change, first demonstrate the regression against the previous behavior, then make the same test pass. Update SPECS and its embedded mirror alongside each behavior change, not in a documentation-only cleanup at the end.

### Task 1 — Build the instrument and the read-only reference first (R2, R3, R8)

- In `test/evals/run.sh` extend `fresh_repo` for the **named** `sweep-bulk` fixture: run the in-repo `gen.py` into the temp workdir **before** committing the fixture; fail setup on error. Both `run_trial` and selftest use this path. Add `test/evals/sweep-bulk/` with `PILOT`, generator, `prompt.txt`, `followup.txt`, checker, and correct/wrong reference transcripts. No generator, oracle or precomputed answer goes in the evaluated worktree. Generate 120 named services × 3 data files, with service-specific references/overrides so a join, not copying a single column, is required. Compare Git **tree** IDs from two independent generations, not timestamp-dependent commit IDs.
- Give both user prompts an answerable task; the follow-up asks for the final sorted 120-row join. `check.sh` grades exact rows from the last assistant prose via `_lib/answer_text.sh`; use an independently computed expected answer outside the workspace. Require the same fixture commit, unchanged tracked files/index and no extra untracked files (isolate harness-owned artifacts). Mutate one value, omit/duplicate a service, reorder rows, omit the transcript, and modify/stage/commit/create a file. Each must fail for its own reason. No expected changed paths; a valid answer from direct tools or shell is just as correct.
- Add a scripted-provider reference leg to `test/codemodegate/codemodegate_test.go` that drives the real binary and reactor on the generated fixture. Use the parked additive bridge first. Consume actual returned data, including truncation/coverage; the provider must construct its answer from those results, **not** substitute the oracle. Run the same checker; perturb one input and prove the computed answer changes. Report scripts, attempts per script, result sizes, steps and first-resume carry with the tagged instrument. A read-per-file join needs at least eight scripts even before discovery (360 files; 50 attempts/script); 120 complete rows need not fit in one result either. Partition input and output, and never carry a v1 cursor between scripts. No paid run if the reference cannot finish with production limits and an untouched tree.
- In `test/evals/evalstats/main.go` and `carry.go`, add a distinct `-compare-transport` comparison for matched schema-2 rows; leave the historical `-compare-carry` verdict intact. For **each full two-prompt trial**, outer calls = sum of `Metrics.Calls` across all tool names, including failed and shell calls, and work steps = `Metrics.Steps` (assistant messages, not HTTP retries and not `Turns`, which must equal two). Multiple tool calls in one message add multiple outer calls but only one step. Keep pass, carry/`CarryMeasured`, direct `Calls["shell"]`, `CMAttempted`, `CMOk`, cost, and elapsed time. Include all matched trials, not just adopters or passes. Reject missing transcripts/observations, wrong tuples/arms, duplicates, nonzero exit mislabelled as a pass and missing planned replicates; an observed zero is not a missing value.
- For inner-operation diagnostics only, carry host counts by known name from `bridge.dispatch` into `tools.Result` and onto the enclosing **session entry** in `Agent.result`; do not put them in `llm.ToolResult.Content` or the provider request. Count starts, not successes; expose rejection/limit counts separately. Emit explicit empty counts for an instrumented script that started no operations, distinguish a missing receipt or a script that never ran. No paths, arguments, outputs or command text. A transcript without nested operations still has measurable outer calls. Test twelve direct reads vs one script issuing twelve reads: **12 vs 1 outer, 0 vs 12 inner**; one batched assistant message may give equal steps. A printed fake ledger must change none of these figures.
- Gate: `go test ./test/evals/evalstats ./internal/agent ./internal/tools -race -count=1`; `bash test/evals/run.sh --selftest sweep-bulk`; `make build && bash test/codemodegate/run.sh`. Wire the reference leg into this gate; exercise the carry hook separately using the tagged binary. The strict-decoded exposure arm configs land in Task 2, when the key exists.

### Task 2 — Land exposure, read parity, discovery, prompt and resume together (R1, R4–R8)

1. **Config and surfaces:** change `internal/config/{config.go,project.go,codemode_test.go}` and `internal/agent/start.go`; pin every resolver row, declaration name/order, direct hidden-call refusal and lookup error. Use the actual registry-built spec list in `internal/provider/wire_delta_test.go`, extending `TestAnthropicWireDeltaNoCache` and `TestAnthropicWireDeltaMovesTheBreakpoint`: same retained bytes/order with cache disabled; exactly one tools breakpoint on the new last entry with cache on. Preserve the off and additive goldens. Add `config.cm-{off,additive,transport}.jsonc` for `sweep-bulk`; extend `TestPilotArmsDifferOnlyInCodemode` to assert exact resolved surfaces, a one-key exposure difference between enabled arms, no codemode block on off, identical other config, same model.
2. **Read path:** change `internal/tools/{codemode.go,inspect.go,inspect_root.go,readtracker.go,tool.go}` and their tests. Exercise the D14–D15 matrix with the real bridge: nested read → direct edit/overwrite succeeds; external change → both refuse until reread; unread existing file still refuses; nested test read alone/search alone do not clear the armed gate, both together do. Pin additive nested read → direct edit still refuses. Test window, symlink replacement, binary, cursor and cancelled read cases. Test a read of a global and built-in skill, a prompt, the config file and a real spill from this session; deny sibling config files, another session’s spill, arbitrary outside paths and symlink escapes in **every** mode. Verify no unexpected questions, files, new spill or snapshots from inspection. Run the per-operation admission and 50/51 boundary tests before accepting any bookkeeping.
3. **API/prompt:** update `internal/codemode/{prelude.go,prelude.js}`, the transport spec in the adapter, `internal/agent/{prompt.go,system-prompt.md}` and transport-specific diagnostic text. New globals only on transport; compare actual discovery payloads and error paths to D16–D17. Test a script that uses a described return field to compute a value; a names-only catalog must fail the test. Keep off/additive renders and workspace/network goldens unchanged.
4. **Lifecycle and clients:** wire `internal/session/entry.go`, `internal/agent/{start.go,resume.go,control.go}` and tests. Pin every D18 row on both request construction and execution. Use `test/codemodegate` to capture real provider request bodies and a deliberately undeclared call; exercise resume/revocation/fresh recovery. In `internal/acp/updates_test.go` and `test/acpgate`, verify `session/update` emits `tool_call` / `tool_call_update` for the outer codemode call and no fictitious direct reads, including replay. TUI renders the outer tool and refusal/notice through existing events. Do **not** add an ACP `tools/list` method.
5. Gate the whole slice: `go test ./internal/config ./internal/session ./internal/tools ./internal/codemode ./internal/agent ./internal/provider ./internal/tui ./internal/acp ./cmd/moca -race -count=1`; `make build && bash test/codemodegate/run.sh && bash test/acpgate/run.sh`. A green declaration-only test cannot release this slice without the successful real edit and revoked-session recovery.

### Task 3 — Qualify the candidate and freeze the pilot protocol (R2, R3, R7, R8)

Repeat the reference on **transport**, with the same generator, joins, caps and checker. Send a scripted direct baseline through the off configuration as a sanity check; keep ordinary shell batching available. The reference is a feasibility test, not evidence of model adoption or savings. Verify each arm's first wire request; no hidden helper files or answers. Run checker selftests with explicit pilot names. Build a single instrumented binary with `go build -tags codemodecarry -o "$TMPDIR/moca-transport-carry" ./cmd/moca`; use an absolute `MOCA` path for the harness. With a local scripted provider, demonstrate a measured carry of zero survives while a missing observation fails. Record the binary digest, fixture tree ID, model, mode, caps, arm configs and comparator version in `test/evals/BASELINE.md` **before** calling a paid provider. No resizing the fixture, tuning the prompt or changing thresholds after seeing a billed row.

### Task 4 — One separately authorized pilot; off is the ship comparison (R2, R3, R8)

**Authorization required at this step:** request up to US$20 **total**, not per arm; ten two-prompt trials per arm on `sweep-bulk`, three arms (`cm-off`, `cm-additive`, `cm-transport`), one binary, `-j 1`, `ask`, `opencode-go/deepseek-v4.1-flash` only. No reruns to chase a number. Before spending, implement and dry-run a shared spend guard across all arm labels: `run.sh` currently checks `-cap` against one output file, after completed trials. Never launch three independent `-cap 20` runs. Stop if costs cannot be accounted for; use a provider-enforced ceiling or reserve a verified worst-case next-trial cost against the remaining budget before launch. If neither can enforce the approved ceiling, do not start. A short or incomplete pilot is inconclusive.

Predeclare two comparisons on the same matched trial IDs 1–10. Use median outer calls (for even N, mean of the central pair), mean `Steps`, aggregate pass count, and the existing paired 95% upper carry-ratio bound, with raw per-arm numbers. A candidate **qualifies** against a comparator only if the comparator median is nonzero, the candidate median is ≤0.5× that value, mean steps do not rise, carry-ratio upper bound is ≤0.75, and there are no fewer passes. Require at least one pass and one successful codemode use on a passing trial of the candidate; shell-only wins are not evidence for codemode. Report per-arm shell calls and failed/non-adopting trials; do not remove them. Historical v1 carry/adoption verdicts are not this comparator.

1. Compare transport and additive. Only a design meeting **all** criteria against the other qualifies as winner. Neither qualifies → record no demonstrated advantage; park both. No choosing the prettier secondary number.
2. Compare **that winner against off**, using exactly the same gates. A loss, tie on improvement, missing data or inconclusive result means no ship. A pass supports only opt-in behavior on this model/workload; report the three-arm table before asking for a merge. It does not approve a default change, release, further spend or a mutation experiment.

### Task 5 — Documentation, rollback and final gates (R7, R8)

Update `docs/specs/SPECS.md` (config, tool contracts, permissions, session/resume, prompt, §15 verification), `docs/SECURITY.md` (read grants and no nested mutations), `README.md`, `CHANGELOG.md` and `AGENTS.md`. Sync the embedded `readme.md`, `changelog.md` and `specs.md` in `internal/skills/builtin/self-knowledge/references/`. For implementation changes refresh the Go-file/package/test count pins as applicable; the Go-file count reads the index. Update the graph after code changes. Keep both parked-branch history and negative measurements. No version bump in the experiment.

**Rollback:** off remains the default. Select additive or disable codemode for **new** sessions; do not rewrite headers or live transcripts to make old sessions look additive. Preserve the refusal/recovery behavior for frozen transport headers if support is withdrawn. No change to the eight built-in schemas, and no claim that another provider/model shares the pilot result.

Final unbilled verification for the implemented candidate, from repo root:

```bash
make build && make vet && make test
go test ./internal/tools ./internal/agent ./internal/codemode ./internal/provider -race -count=8
bash test/codemodegate/run.sh && bash test/acpgate/run.sh
bash test/evals/run.sh --selftest sweep sweep-search sweep-bulk
go test -tags counts ./cmd/moca -run TestSpecsTestCount -count=1
```

`make test` already enables the race detector. Bare `--selftest` skips `PILOT` directories; it does not replace the named command. The release decision needs both the unbilled gates above and Task 4's approved comparison. This revision authorizes neither implementation nor billing.

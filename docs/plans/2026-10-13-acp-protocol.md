---
status: draft
execute: false
superseded-by: docs/superpowers/plans/2026-10-13-acp-batch-*.md
---

> ⛔ **DRAFT — NO GO. Do not implement from this file, and do not use it as instructions for any agent.**
> This is the original Hermes Agent plan. It was merged with the Claude Code plan, then amended and split into the batch files below. Kept for history only; parts of it are outdated or were rejected (see Appendix A of `docs/superpowers/plans/2026-10-13-acp-merged.md`).
> To implement `moca acp`, run the batch files in order, one per session:
>
>   1. `docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md`
>   2. `docs/superpowers/plans/2026-10-13-acp-batch-2-server.md`
>   3. `docs/superpowers/plans/2026-10-13-acp-batch-3-protocol-completeness.md`
>   4. `docs/superpowers/plans/2026-10-13-acp-batch-4-ship.md`

# ACP (Agent Client Protocol) server for moca — implementation plan

**Goal:** run moca as an ACP v1 agent subprocess (`moca acp`) so editors (Zed, VS Code via ACP extensions) and generic agent UIs (Paseo) can drive it over stdio — with the minimum surface those clients need, and zero impact on the existing CLI/TUI modes.

**Architecture:** a new leaf-adjacent package `internal/acp` (imports `agent`, `config`, `llm`, `provider`, `session`, `tools` — sits beside `tui` in the import graph, never imported by it) that owns the ndjson JSON-RPC wire, translates `agent.Event`s to `session/update` notifications, and bridges `tools.Asker` to `session/request_permission`. The agent core gets exactly one addition: a `Transcript()` accessor for replay. Everything else is new code behind a new subcommand.

**Wire facts (verified against the ACP v1 spec + the official Python SDK schema, `acp` 0.x, vendored in Hermes' venv):**

- **Transport**: stdio subprocess, **ndjson framing** — one JSON-RPC 2.0 message per `\n`-terminated line (SDK `task/sender.py` writes `json.dumps(...) + "\n"`, read loop is `readline()`). NOT `Content-Length` headers (that's the LSP-style variant some agents use; the official SDK and Zed's clients speak ndjson).
- **Version**: `protocolVersion` is a single int; v1 is current. If the client's version ≠ 1, respond with 1.
- **Session id**: opaque string to the client. We use moca's existing `id8` (8 hex chars) — `session/load` then maps straight onto `session.Find`.
- All paths absolute (1-based lines), camelCase JSON keys, snake_case discriminator values.

---

## In scope (v1)

| Surface | Decision |
|---|---|
| `initialize` | protocolVersion 1; `agentCapabilities.loadSession: true`; `promptCapabilities: {}` (text-only prompts); `mcpCapabilities: {}` (client-supplied servers ignored — moca runs its own from config); `authMethods: []`; `agentInfo{name:"moca", version}` |
| `session/new` | params `cwd` (absolute, validated) → `agent.Start`; respond `{sessionId, modes}` |
| `session/prompt` | text blocks only (joined with `\n\n`); runs the turn in a goroutine, streams updates, replies `{stopReason}` |
| `session/cancel` | notification → cancels the session's run context; prompt responds `stopReason:"cancelled"` |
| `session/update` | `user_message_chunk`, `agent_message_chunk`, `agent_thought_chunk`, `tool_call`, `tool_call_update`, `usage_update` |
| `session/request_permission` (agent→client) | the `tools.Asker` bridge; allow-once / reject / allow-always (gated on `Question.CanAlways`) |
| `session/load` | replay the stored transcript as updates, then `{}` — reuses `agent.Resume` + `session.Find` |
| `session/set_mode` | ACP modes `default` / `auto` / `yolo` / `plan` → `agent.SetMode`/`SetPlan`; refused while a turn is running |
| EOF / stdin close | close all agents (`a.Close()`), exit 0 |

## Out of scope (documented, not implemented)

- `session/list`, `session/delete`, `session/resume` (load covers editor needs; list/delete are capability-gated extras Paseo can live without in v1)
- `fs/read_text_file`, `fs/write_text_file`, `terminal/*`, `elicitation/*` — not advertised; moca's own tools do the IO
- `authenticate` / `logout` (no auth methods), `session/set_model` + `models` state (**unstable** SDK extension — model comes from moca config)
- `plan` updates (agent-plan board), `compaction_update`/`compaction_summary_chunk` (needs client `session.compaction` capability), `notice` (needs `session.notices`), `available_commands_update` (commands are a TUI-layer concept; ACP prompts are plain text)
- diff content blocks (`{type:"diff"}`) and `rawOutput` on tool calls — v1 sends `content` text (the result summary) + `rawInput`; diffs can follow later
- images/audio/embedded context in prompts (error: text-only)
- steering mid-turn (ACP clients wait for the turn; `session/prompt` during a running turn on the same session → JSON-RPC error)

## Design

### Package layout & import direction

```
cmd/moca/acp.go        runACP: config load, log start, server construction, run loop
internal/acp/
  wire.go              ndjson codec: read loop, mutex-guarded writer, JSON-RPC envelopes, id correlation
  types.go             ACP DTOs (hand-rolled structs + json tags, pinned to the SDK schema)
  server.go           dispatcher, session table, lifecycle (new/prompt-goroutine/cancel/load/set_mode/EOF)
  replay.go            session/load: session.Entry list → session/update stream (tool_use↔tool_result pairing)
  translate.go         pure: agent.Event → update structs; tool kind/title extraction; stop-reason mapping
  permissions.go       tools.Asker → session/request_permission bridge
  *_test.go            unit (translation/codec) + integration (scripted provider through the real agent loop)
```

Import direction (AGENTS.md rule): `agent` → `acp` → `cmd/moca`. `internal/acp` imports `agent, config, llm, provider, session, tools` and **never** `tui`. Update the AGENTS.md layout line (`agent → tui/acp`).

The only agent-core change: `func (a *Agent) Transcript() []session.Entry` — mu-guarded copy of `a.entries` (replay needs raw entries; `History()` applies compaction and drops tool structure).

### Concurrency model

- One read loop (main goroutine): parse each line → request / notification / response-to-our-request.
- `session/prompt` handler spawns a goroutine per turn; the reply is written when the turn ends. The read loop never blocks on a turn — **mandatory**, because permission requests and `session/cancel` must flow while a turn runs.
- Writes (updates, responses, agent→client requests) all go through one mutex-guarded writer.
- Multiple concurrent sessions allowed: map `sessionId → *acpSession{agent, runCtx+cancel, lastStop, msgId}`. Each `agent.Agent` is independent (own registry, env, session writer).
- `session/set_mode` on a session with a running turn → JSON-RPC error (code −32000, "mode can only be set between turns" — matches the TUI's `/mode` refusal semantics).
- JSON-RPC ids: `json.RawMessage`, echoed verbatim (clients use ints or strings). Our outgoing ids (permission requests) are ints from a counter, per-direction so no collision.

### Method table

| Method | Dir | Handler |
|---|---|---|
| `initialize` | req | version/capability negotiation (see wire facts); respond same version if 1, else 1 |
| `session/new` | req | validate absolute `cwd`; `agent.Start(StartOptions{Config, Workdir: cwd, Trusted: saved trust, Mode: cfg-resolved, Ask: bridge, Emit: translator, Slug: session.Slug(first text) or "acp"})`; store session; respond `{sessionId, modes}` |
| `session/prompt` | req | join text blocks; reject non-text blocks (−32602); empty prompt (−32602); run `a.Run(ctx, text)` in a goroutine; on end reply `{stopReason}`; on error reply JSON-RPC error (−32603, message) — except cancellation → `{stopReason:"cancelled"}` |
| `session/cancel` | notif | cancel the session's runCtx (unknown id: ignore — notifications get no response) |
| `session/load` | req | `session.Find(dir, sessionId, cwd)` + `session.InWorkdir` check (moca's cross-workdir refusal semantics hold; mismatch → −32000 naming both dirs); `agent.Resume`; replay via `replay.go`; respond `{}`; refuse while that session has a running turn |
| `session/set_mode` | req | map mode id (below); between-turns only; respond `{}`; emit `current_mode_update` |
| anything else | req | −32601 `method not found` |
| anything else | notif | ignore |
| malformed line | — | log to slog (stderr/file), continue — matches the official SDK's behavior |

### ACP modes ↔ moca modes

`availableModes`: `default` ("Default — permission rules apply, risky actions ask"), `auto` ("Auto — safe actions run, risky ones are refused or ask"), `yolo` ("Yolo — all permission checks off"), `plan` ("Plan — analyze and write an implementation plan to docs/plans/, no other writes").

- `set_mode("plan")` → `SetPlan(true)` (which lands on default per `plan.go`)
- `set_mode("default")` → `SetPlan(false)` + `SetMode(ModeDefault)`
- `set_mode("auto"|"yolo")` → `SetMode(...)` (SetMode itself drops plan first)
- After `session/load`, read `a.Mode()` + `a.Plan()` for `currentModeId`.

### Event → session/update translation

`messageId`: per assistant message — mint on first `TextDelta`/`ThinkingDelta` after a `StreamReset` or turn start; `toolCallId` = the provider's `call.ID` (already unique per session).

| moca event | ACP update |
|---|---|
| `TextDelta` | `{sessionUpdate:"agent_message_chunk", messageId, content:{type:"text", text}}` |
| `ThinkingDelta` | `{sessionUpdate:"agent_thought_chunk", messageId, content:{type:"text", text}}` |
| `StreamReset` | rotate `messageId` (next chunk starts a new message; nothing sent) |
| `ToolStart{Call}` | `{sessionUpdate:"tool_call", toolCallId, name: call.Name, title, kind, status:"pending", rawInput: <call.Input decoded>}` |
| `ToolEnd{Call, Result}` | `{sessionUpdate:"tool_call_update", toolCallId, status: completed\|failed, content:[{type:"content", content:{type:"text", text: Result.Summary}}], locations?}` |
| `TurnEnd` | nothing sent directly; drives `usage_update` + the prompt reply's stop reason |
| `Retry`, `Warning`, `SteeringApplied`, `Compacted`, `ModeChanged`, `PlanChanged` | dropped in v1 (no notice capability plumbing; mode changes are client-driven here) |

- At each `TurnEnd`: `usage_update` `{used: a.Status().ContextTokens, size: a.Status().Window, cost:{amount: a.Status().Cost, currency:"USD"}}`.
- User echo: on `session/prompt`, first send `user_message_chunk` (messageId = new id) with the joined text before running — clients render it as the turn's user message.

**Tool kind map** (`call.Name` → ACP `kind`): `read`→`read`, `ls`→`read`, `search`→`search`, `write`→`edit`, `edit`→`edit`, `shell`→`execute`, `web`→`fetch`, `mcp`→`other`.

**Title extraction** (from `call.Input`, ~25 lines): first string of `path` / `file` (read, write, edit, ls) → `read internal/agent/agent.go`; `command` (shell) → first 60 chars; `pattern` (search); `op`+`query`/`url` (web) → `web search "…"` / `fetch example.com`; `server`+`tool` (mcp) → `mcp context7/search`. Fallback: the input's first string value, else the tool name.

**Locations**: for `write`/`edit` calls, `locations:[{path: <abs path from input>}]` (no line info — omit `line`). Other tools: omit.

**Stop reasons** (`Outcome` + tracked `lastStop`, mirroring `oneshot.go`):

| Condition | stopReason |
|---|---|
| `err` is `ctx.Err()`/`context.Canceled` | `cancelled` |
| `Outcome.MaxSteps` | `max_turn_requests` |
| `Outcome.Stuck` (repeated-failure wrap-up) | `end_turn` (it ended with a summary; no ACP "stuck" reason) |
| `lastStop == StopLength` | `max_tokens` |
| `lastStop == StopRefusal` | `refusal` |
| otherwise | `end_turn` |

### Permissions bridge (`permissions.go`)

`tools.Asker` implementation per session:

- `Question{Kind, Subject, Detail, CanAlways}` → agent→client request `session/request_permission` with:
  - `toolCall`: the **currently executing** `toolCallId` + title + kind (the bridge tracks the in-flight call — `ToolStart` arrives before the tool runs, so the asker consults the session's current-call state), `status:"pending"`
  - `options`: `[{optionId:"allow-once", name:"Allow once", kind:"allow_once"}, {optionId:"reject", name:"Reject", kind:"reject_once"}]` + `[{optionId:"allow-always", name:"Allow always", kind:"allow_always"}]` iff `CanAlways` (matches the TUI's ctrl+a gating — `LearnableCommands` already prevents offering a dead allow-always)
- Client result `{outcome:{outcome:"selected", optionId}}` → `AllowOnce` / `AllowAlways` / `Deny`
- `{outcome:{outcome:"cancelled"}}` → `Deny` (turn is dying; `session/cancel` semantics)
- JSON-RPC **error** response or undecodable result → `Deny` (fail-closed)
- **No timeout** — the client owns the UI; `session/cancel` is the escape hatch. (Hermes' 60s approval timeout silently denies under Paseo with Auto Accept off — a documented pitfall; moca refuses to reproduce it.) Documented in SPECS.
- Question kind → ACP kind: `shell`→`execute`, `write`→`edit`, `web`→`fetch`, `mcp`→`other`.

Note: the write-tool's ask (`askOutsideWrite`) and the auto-mode ladder flow through the same `Env.Ask` — one bridge covers all askers, including `yolo`'s absence of asks entirely.

### `session/load` replay (`replay.go`)

From `a.Transcript()` (raw entries, append order):

| Entry | Updates |
|---|---|
| `message` (user) | `user_message_chunk` (messageId = entry.ID) |
| `message` (assistant) text blocks | `agent_message_chunk` per block (messageId = entry.ID) |
| `message` (assistant) thinking blocks | `agent_thought_chunk` |
| `tool_use` | `tool_call` (status `in_progress`) |
| `tool_result` (paired by `CallID`) | `tool_call_update` (status `completed`/`failed`, content = entry content) |
| unpaired `tool_use` (crash) | `tool_call_update` status `failed`, content `"no result recorded — the session was interrupted"` |
| `session`, `compaction`, `model_change`, `permission_mode`, `elision`, `snapshot`, `error` | skipped |

After replay: respond `{}` to `session/load`, then the session accepts prompts as usual (`agent.Resume` already restored state). Replay happens **before** the load reply (drain discipline).

### CLI wiring

- `cmd/moca/cli.go`: add `acp` to the known-subcommand set and the usage text (`moca acp — run as an ACP agent on stdio (for editors; ndjson JSON-RPC)`).
- `cmd/moca/main.go` `run()`: route in the config-needs-no-model group beside `mcp`/`login`: `case "acp": return runACP(ctx, o, cfg, path, stdin, stdout, stderr)`. `runACP` starts the log first (`startLog`) — a long-lived server wants diagnostics; **stdout belongs to the wire exclusively** (applog already discards slog's stderr default; `run()` calls `applog.Discard()` before routing — verified).
- No model at startup: fine. `session/new` fails with a clear JSON-RPC error pointing at the config (parity with the TUI's setup-mode message: `set "model" in ~/.config/moca/config.jsonc or run moca to configure`) — the client surfaces it in-editor.
- Project trust: reuse the oneshot helper semantics — trusted only if a saved `trust.json` decision exists for the workdir (no prompting channel; project rules load only when trusted). `--config` flag works as everywhere else.

---

## Tasks

> House rules: TDD (fail → implement → pass), one conventional commit per task (`feat(acp): …` / `test(acp): …`), `make vet` + `go test ./... -race -count=1` green before each commit, stage explicit paths. Run everything via mise (`make …` handles it).

### Task 1 — `internal/agent`: `Transcript()` accessor

- [ ] Test (`internal/agent/agent_test.go`): start via `startTest`, run one prompt, assert `Transcript()` returns entries whose IDs/parent chain match what the session file holds (read it back with `session.ReadFile`), and that a second call returns an equal but distinct slice (copy, not alias).
- [ ] Implement `func (a *Agent) Transcript() []session.Entry { a.mu.Lock(); defer a.mu.Unlock(); return append([]session.Entry(nil), a.entries...) }` in `agent.go` beside `History()`.
- [ ] Run: `go test ./internal/agent -run TestTranscript -race -count=1` (fail-first via a stub? No — smallest task, implement directly, the test pins behavior).

### Task 2 — `internal/acp/wire.go`: ndjson codec

- [ ] Types: `Message` (raw `json.RawMessage` id, `method`, `params`/`result`/`error`), `Request`, `Response`, outgoing-request future map (`map[int64]chan Response`), read `bufio.Scanner` (64 MiB buffer, like `session.ReadFile`), writer with `sync.Mutex`.
- [ ] API: `newCodec(r io.Reader, w io.Writer) *codec`; `(*codec).read() (Message, error)` (EOF propagates); `(*codec).send(v any) error` (marshal + `\n` + flush, mutex); `(*codec).sendRequest(method string, params any) (Response, error)` (registers future, id from counter); `(*codec).resolve(id, resp)` for the read loop to complete futures; `(*codec).sendResponse(id any, result any)` / `sendError(id any, code int, msg string)`. Standard codes: −32601 method not found, −32602 invalid params, −32603 internal.
- [ ] Tests: round-trip request/notification/response through an `io.Pipe` from two goroutines; id echo for string and number ids; future resolution by id; concurrent `send` from 8 goroutines under `-race` (no interleaved lines — each read line is valid JSON); EOF surfaces as error.
- [ ] Commit: `feat(acp): ndjson JSON-RPC codec`.

### Task 3 — `internal/acp/types.go`: ACP DTOs

- [ ] Hand-rolled structs with json tags, pinned to the SDK schema (verified there — do not improvise): `InitializeResult{ProtocolVersion int, AgentCapabilities{LoadSession bool, PromptCapabilities{}, McpCapabilities{}}, AgentInfo{Name, Title?, Version}, AuthMethods []any}`, `NewSessionResult{SessionID string, Modes *SessionModeState{CurrentModeID string, AvailableModes []SessionMode{ID, Name, Description?}}}`, `PromptResult{StopReason string}`, `sessionUpdate` variants (each with `SessionUpdate string \`json:"sessionUpdate"\`` discriminator + fields exactly as in the mapping table; `toolCall` content blocks as `[]ToolCallContent` with `Type:"content"` + embedded text block), `RequestPermissionParams/Result`, `UsageUpdate{Used, Size int, Cost *Cost{Amount float64, Currency string}}`, `LoadSessionParams{SessionID, Cwd string}`, `SetModeParams{SessionID, ModeID string}`, `CancelParams{SessionID string}`, `PromptParams{SessionID string, Prompt []ContentBlock{Type, Text}}`.
- [ ] Tests: golden JSON for one example of every update variant + initialize/new/prompt results (marshal → exact strings; these goldens ARE the wire contract — keep them terse and comment each with the schema source). Include the empty-object cases: `promptCapabilities:{}`, `mcpCapabilities:{}`.
- [ ] Commit: `feat(acp): wire DTOs pinned to the ACP v1 schema`.

### Task 4 — `internal/acp/translate.go`: event → update (pure)

- [ ] `type translator struct{ msgID string }` with `event(e agent.Event) []any` (0..n updates) implementing the mapping table: message-id rotation on `StreamReset`, mint on first delta; `toolCall` from `ToolStart` (kind map, title extraction, `rawInput` = `json.Unmarshal` of `call.Input` into `any` — invalid JSON: omit); `toolCallUpdate` from `ToolEnd` (status `failed` iff `Result.IsError`; `locations` for write/edit); `turnEnd` → no update but returns `(usageUpdate, stopReason)` via `stopReason(lastStop, outcome, err)`.
- [ ] Pure helpers in the same file: `toolKind(name)`, `toolTitle(call)`, `joinPrompt(blocks)`, `modeList()`, `currentMode(mode agent.PermMode, plan bool)`.
- [ ] Tests: table-driven over every event type incl. two `TextDelta` sharing one `messageId`, a `StreamReset` rotating it, an edit `ToolEnd` carrying `locations`, `rawInput` valid + invalid JSON, every stop-reason row of the map, `joinPrompt` joining text blocks and rejecting image blocks, title extraction per tool name.
- [ ] Commit: `feat(acp): event translation layer`.

### Task 5 — `internal/acp/server.go` (core lifecycle, no permissions yet)

- [ ] `type Server struct{ codec, sessions map[string]*acpSession, cfg config.Config, opts … }`; `Run(ctx)` = read loop + dispatch; per-session: `agent, runCancel, running bool, trans *translator, currentCall llm.ToolCall`.
- [ ] `initialize` (before it: refuse everything else with −32000 "not initialized"), `session/new` (absolute-cwd validation −32602; `agent.Start` with `Ask: nil` for now; failure → −32000 with the moca error text), `session/prompt` (goroutine turn, `user_message_chunk` echo, `a.Run`, reply per stop-reason map; provider error → −32603; `maxSteps` etc. per table), `session/cancel` (cancel runCtx; no-op if idle), unknown method −32601, unknown notification ignored, malformed line logged + skipped, EOF → `Close()` all agents, return.
- [ ] `session/set_mode` + `current_mode_update` emission; refuse while running (−32000).
- [ ] Integration tests (own mini `scriptServer` copy of the `agent_test.go` pattern — ~50 lines, openai-completions SSE over `httptest`, scripted turns incl. a tool-call turn): drive a full `initialize → session/new → session/prompt` over an in-memory pipe; assert the exact ndjson line sequence (echo, message chunks, usage_update, prompt response `end_turn`); a scripted tool-call turn asserts `tool_call` + `tool_call_update`; cancel mid-scripted-turn asserts `cancelled`; `set_mode` mid-run refuses; second `session/prompt` while running → error; EOF closes cleanly with the session file valid on disk (read back with `session.ReadFile`).
- [ ] Commit: `feat(acp): server lifecycle and prompt turns`.

### Task 6 — `internal/acp/permissions.go`: the asker bridge

- [ ] `func (s *acpSession) asker(ctx context.Context, q tools.Question) tools.Answer` — builds `RequestPermissionParams` (toolCall = the session's current in-flight call — set at `ToolStart`, cleared at `ToolEnd`; if no call is in flight (the write-tool's ask-write root asks outside a tracked call — no: it *is* inside the write tool call, the tracker covers it; fallback: a synthetic toolCall `{toolCallId:"ask", title: q.Subject, kind: mapped}`), options per `CanAlways`), sends via `codec.sendRequest`, decodes the outcome (selected → map optionId; cancelled → `Deny`; error → `Deny`).
- [ ] Wire it: `session/new` passes `Ask: s.asker`.
- [ ] Tests (harness = the test-side client goroutine reading our pipe and answering by `optionId`): scripted shell turn that triggers an ask → assert the request JSON on the wire (options, kinds, toolCallId matching the `tool_call` earlier), respond allow-once → run proceeds; respond reject → tool result is the refusal and the turn continues (the model sees the refused result); `CanAlways:false` → no allow-always option on the wire; respond an error object → `Deny`; no response channel needed for cancelled (unit-level: feed a cancelled outcome → `Deny`).
- [ ] Commit: `feat(acp): session/request_permission bridge`.

### Task 7 — `internal/acp/replay.go`: `session/load`

- [ ] Handler: `session.Find` + `InWorkdir` refusal (−32000 naming both dirs, mirroring `resolveResume`'s message), `agent.Resume`, replay per the entry table, respond `{}`; refuse while that session has a running turn; a loaded session replaces any same-id session entry (id collision is impossible — `Find` is by id8 — but handle a stale entry defensively).
- [ ] Tests: build a session via the Task-5 harness (two turns, one with a tool call), `session/load` it in a **fresh server** (new codec, same data dir), assert the replayed update sequence (user/agent/thought chunks, `tool_call`+`tool_call_update` paired, statuses), assert `{}` reply comes **after** the last replayed update, then a follow-up prompt continues the conversation (scripted provider sees the prior messages in its request body). Cross-workdir load refuses. Unpaired `tool_use` (append a synthetic truncated session file) → `failed` update.
- [ ] Commit: `feat(acp): session/load replay`.

### Task 8 — `cmd/moca`: the subcommand

- [ ] `cli.go`: usage line + allow `acp` in the subcommand set; `main.go`: route `case "acp"` in the config group; `acp.go`: `runACP` — `startLog`, build `acp.Server`, `Run(ctx)`; exit 0 on EOF, `exitRuntime` on a codec write error (stdout closed under us — the client died).
- [ ] Tests (`cmd/moca` style, hermetic XDG): arg parsing (`acp` accepted, `acp --config x`), routing (fake stdin EOF with a config that fails to load → exit 2; valid config + immediate EOF → exit 0, **nothing on stdout**, one clean log file), `--version` unaffected.
- [ ] Run the real binary probe: `printf '' | ./bin/moca acp` exits 0 silently; `printf '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}\n' | ./bin/moca acp` prints exactly one ndjson line with `agentInfo.name=moca` and exits 0 on EOF.
- [ ] Commit: `feat(cli): moca acp subcommand`.

### Task 9 — E2E gate (scripted, house pattern)

- [ ] `test/acp/gate.sh` (hermetic XDG dirs, mock openai-completions provider — reuse the `moca_gate5/mock_llm.py` pattern; a small python ACP **client** driving the real binary over pipes): initialize → new → prompt (text turn) → assert chunk+usage+`end_turn`; prompt with a scripted shell tool call requiring permission → assert the request_permission round-trip (auto-answer allow) and `tool_call`/`tool_call_update` on the wire; cancel mid-turn → `cancelled`; `session/load` of the just-written session in a second process; leak scan (no key material in any capture — config `apiKey` env-ref only).
- [ ] Run: `bash test/acp/gate.sh` → ALL PASS, evidence dir under `~/.hermes/cache/scratch/moca_acp_gate/`.
- [ ] Commit: `test(acp): scripted end-to-end gate`.

### Task 10 — ACP TCK pass (manual gate, not CI — settled)

- [ ] Clone `agentclientprotocol/acp-tck`, point it at `moca acp`, record the report (expect mandatory-tier passes; anything failing on an unadvertised capability is N/A). Keep the report path + verdicts in the PR description, not the repo (experimental tool, no CI dependency).

### Task 11 — Docs

- [ ] `docs/specs/SPECS.md`: new §"ACP server" (surface table, method table, mode mapping, permission bridge incl. the no-timeout decision + rationale, replay semantics, ndjson framing note, import direction); §2 CLI list gains `moca acp`; §15 test/pkgs counts (re-measured with the house commands — `go list ./... | wc -l`, RUN-line count).
- [ ] `README.md`: ACP section — Zed (`agents.json` / Assistant Panel: `{"moca": {"command": ["moca", "acp"], "roles": ["primary"]}}`-shaped snippet per Zed's current schema — verify against Zed docs at write time), Paseo (`paseo` provider entry `command: ["moca", "acp"]`), the text-only/no-set_model caveats.
- [ ] Docs bumped to **v0.5.0-beta** (settled): SPECS header current-version line, README status line + phase table, CHANGELOG new section (one bump per PR).
- [ ] `AGENTS.md`: layout/import-direction line (`agent → tui/acp`), the acp package one-liner.
- [ ] Commit: `docs: ACP server spec, README, changelog`.

### Task 12 — Graph + final gates

- [ ] `graphify update .` (code layer) — commit per the graphify-builds skill; doc layer via the assistant `--update` flow if the skill calls for it.
- [ ] `make vet && make test && make build` green; re-run `bash test/acp/gate.sh` once more on the built binary.
- [ ] PR: branch `phase/10-acp-server`, PR body with the TCK verdict + gate evidence.

---

## Settled decisions (Ben, 2026-10-13)

1. **`session/load`** — IN (Task 7).
2. **ACP modes** — all four exposed: `default` / `auto` / `yolo` / `plan`.
3. **Version** — v0.5.0-beta on this PR.
4. **TCK** — manual gate only (Task 10), not CI.
5. **`usage_update`** — IN.
6. **Permission request timeout** — NONE (recommendation, uncontested: `session/cancel` is the escape hatch; Hermes' 60s timeout silently denying under Paseo's non-auto-accept is a documented pitfall we should not import).

## Known limitations to document (not fix)

- Tool-call content is the result `Summary` (one line) — no diff blocks, no `rawOutput` in v1.
- Client-supplied `mcpServers` are ignored (moca runs its own; `mcpCapabilities:{}`).
- One `session/prompt` per session at a time (second concurrent prompt → error; steering is not exposed).
- No `session/list`: Paseo's import dialog won't list old moca sessions (create-through-UI works fine).
- Mode switches only between turns (matches the TUI's `/mode` refusal).

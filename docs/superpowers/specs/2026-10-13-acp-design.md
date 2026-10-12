# `moca acp` — Agent Client Protocol server (design)

Status: approved design, not implemented. On implementation, the normative
description moves into `docs/specs/SPECS.md` (new ACP section) in the same PR.

## Goal

Let ACP clients — Zed, VS Code ACP extensions, generic agent UIs such as
Paseo — drive moca as an external agent. Scope is the minimum those clients
need, delivered as a separate subcommand that does not touch the `-p` or TUI
code paths.

Editor configuration is just a command line:

```jsonc
// Zed settings.json
"agent_servers": { "moca": { "command": "moca", "args": ["acp"] } }
```

## Decisions

| Topic | Decision |
|---|---|
| Entry point | `moca acp` subcommand; one `case "acp":` in `cmd/moca/main.go` |
| Implementation | hand-rolled, stdlib-only `internal/acp` (mirrors `internal/mcp`); no SDK, no new dependency |
| Protocol version | ACP v1 (`protocolVersion: 1`) |
| Optional features in v1 | session modes, model picker (config option), `session/load` + `session/list`, slash commands |
| File access | tools keep using the disk directly; client `fs/*` and `terminal/*` are not used |
| Client-supplied MCP servers | ignored (debug-logged count); moca's own `mcp.servers` apply |
| Project trust | trusted by default (editor opening the folder is the trust act); `--no-approve` opts out |
| Auth | `authMethods: []`; missing model/credential → actionable JSON-RPC error pointing at `moca login` |

## Non-goals (v1)

Client fs/terminal delegation · client MCP servers · image/audio prompts ·
ACP terminal-auth / `authenticate` flows · effort config option · TUI
built-in commands over ACP (`/compact`, `/undo`, …) · steering of a running
prompt · `session/resume`, `session/close`, `session/delete` · elicitation.

## §1 Architecture

### CLI

```
moca acp [--model <provider/model>] [--effort <level>]
         [--mode default|auto|yolo|plan] [--no-approve] [--config <path>]
```

- Parses its own `flag.FlagSet` from `o.Sub[1:]` (same pattern as `login`).
  Global flags placed before `acp` (`--config`, `--model`, …) are honored too.
- Loads config once at startup. A config that fails to parse is a fatal
  startup error (stderr + exit 2). A missing `model` is **not** fatal at
  startup — `session/new` reports it.
- Starts the file log (`startLog`) like the other run modes.
- **stdout carries only JSON-RPC.** Nothing else may write to it.
  `applog.Discard()` already runs first in `run()`; stderr is used only for
  fatal startup errors.
- Exit: stdin EOF → close every session's agent (`Agent.Close`, which also
  stops MCP servers) and exit 0. SIGINT/SIGTERM → cancel running prompts,
  then the same.

### Package `internal/acp`

| File | Responsibility |
|---|---|
| `rpc.go` | JSON-RPC 2.0 connection over newline-delimited JSON: reader loop; each incoming request is dispatched on its own goroutine; notifications are dispatched inline in arrival order; writes go through a mutex; outgoing requests are correlated to responses through a pending-id map; `Close` fails pending calls |
| `protocol.go` | Go types for the ACP subset moca uses (requests, responses, `SessionUpdate` variants, content blocks, tool-call fields, permission options/outcomes, modes, config options, session info) |
| `server.go` | `Serve(ctx, in, out, Options)`; `initialize`; the method table; the `sessionId → *session` map |
| `session.go` | per-session state (agent, running prompt's cancel func, current tool-call id, current `messageId`); the `agent.Event` → `session/update` translator; the `tools.Asker` → `session/request_permission` bridge |
| `toolcall.go` | per-tool title / kind / locations / content / diff mapping |
| `replay.go` | `Agent.History()` → updates for `session/load` |
| `commands.go` | the slash-command list (`available_commands_update`) and expansion |

`Options` (filled by `cmd/moca/acp.go`): config, config path, CLI overrides
(model, effort, mode), trust default, version string, HTTP client (tests).

### Dependency rule

`agent → tui | acp`. `acp` imports `agent`, `tools`, `skills`, `session`,
`llm`, `config`; `acp` and `tui` never import each other. AGENTS.md and the
SPECS layout section are updated.

### Shared-code refactor (only one)

`tui.skillMessage` → `skills.SkillMessage(s Skill, args string) string`, so
the TUI and ACP build the same `/skill` message. The TUI calls the moved
function; behaviour is unchanged.

## §2 Protocol mapping

### `initialize`

Response:

```json
{
  "protocolVersion": 1,
  "agentCapabilities": {
    "loadSession": true,
    "promptCapabilities": { "image": false, "audio": false, "embeddedContext": true },
    "mcpCapabilities": { "http": false, "sse": false },
    "sessionCapabilities": { "list": {} }
  },
  "authMethods": [],
  "agentInfo": { "name": "moca", "title": "moca", "version": "<config.Version>" }
}
```

moca always answers `1`; a client that cannot speak it disconnects (per spec).

### `authenticate`

Returns an error: "moca uses its own credentials — run `moca login <provider>`
in a terminal".

### `session/new`

1. Validate `cwd`: absolute, an existing directory → else `-32602`.
2. `mcpServers`: ignored; the count is logged at debug level.
3. Trust: unless `--no-approve`, `config.LoadProject(cwd)` is merged
   (`cfg.Project`); a project file that fails to parse → error.
4. Mode: same resolution as the other front ends (`--mode` > trusted project
   `defaultMode` > config).
5. `agent.Start(StartOptions{Workdir: cwd, Slug: "acp", Trusted, Mode, Plan,
   Model, Effort, Emit: s.emit, Ask: s.ask})`. No model / no credential → error
   with the fix (`moca login <provider>`, set `"model"` in `<config path>`).
6. Respond `{sessionId: <moca id8>, modes, configOptions}`, then send
   `available_commands_update`.

The ACP `sessionId` **is** moca's session id8, so `session/list` and
`session/load` resolve it with the existing `session.Find` / `session.ListAll`.

### `session/prompt`

- Prompt content blocks are flattened in order into one user message:
  - `text` → verbatim;
  - `resource_link` → `@<path>` (a `file://` URI becomes a path, relative to
    cwd when inside it; other URIs verbatim) — the model reads it itself, as
    with the TUI's `@`;
  - `resource` (embedded) with `text` → a fenced block headed by its URI;
    a blob resource → `-32602`;
  - `image` / `audio` → `-32602` (capability is false).
- Slash commands: if the flattened text starts with `/name`, and `name` is a
  prompt template → `skills.ExpandPrompt(body, args)`; a skill →
  `skills.SkillMessage`; otherwise the text is sent unchanged.
- One prompt at a time per session; a second concurrent `session/prompt` →
  error "a prompt is already running".
- Runs `a.Run(ctx, text)` with a per-prompt cancellable context.
- `stopReason`:

| Outcome | `stopReason` |
|---|---|
| success | `end_turn` |
| ctx cancelled (`session/cancel`, shutdown) | `cancelled` |
| `Outcome.MaxSteps` / `Outcome.Stuck` | `max_turn_requests` |
| last turn `llm.StopLength` | `max_tokens` |
| last turn `llm.StopRefusal` | `refusal` |
| any other error | JSON-RPC error (`-32603`, message = error text) |

- After the run: reload prompts and skills (`a.ReloadSkills`,
  `skills.LoadPrompts`); re-send `available_commands_update` if the list changed.

### `session/cancel` (notification)

Cancels the running prompt's context. A pending permission request resolves to
`Deny` (the asker selects on `ctx.Done()`, as the TUI's does); `Run` returns
the ctx error → `stopReason: cancelled`. No running prompt → no-op.

### Events → `session/update`

| `agent.Event` | update |
|---|---|
| `TextDelta` | `agent_message_chunk` `{content: {type: text}, messageId}` |
| `ThinkingDelta` | `agent_thought_chunk` |
| `StreamReset` | no update; the next chunk uses a new `messageId` (sent chunks cannot be retracted) |
| `ToolStart` | `tool_call` `{toolCallId: call.ID, title, kind, status: in_progress, rawInput, locations}` |
| `ToolEnd` | `tool_call_update` `{toolCallId, status: completed\|failed, content}` |
| `ModeChanged`, `PlanChanged` | `current_mode_update` |
| `Warning` | the spec's user-facing notice update if the pinned schema defines it; else `agent_thought_chunk` prefixed `moca: ` |
| `Retry`, `TurnEnd`, `Resumed`, `SteeringApplied` | none (logged) |

### Tool calls

| Tool | `kind` | title | content on end |
|---|---|---|---|
| `read` | `read` | `read <path>` | `Result.Summary` only — never the file body |
| `write` | `edit` | `write <path>` | `diff {path, oldText: null, newText: <content>}` |
| `edit` | `edit` | `edit <path>` | `diff {path, oldText: old_string, newText: new_string}` |
| `shell` | `execute` | the command (first line, ≤ 80 chars) | output text, capped at 8 KB (tail kept) |
| `search` | `search` | `search <pattern>` | text, capped at 8 KB |
| `ls` | `search` | `ls <path>` | text, capped at 8 KB |
| `web` | `fetch` | `search <q>` / `fetch <url>` | text, capped at 8 KB |
| `mcp` | `other` | `mcp <server>/<tool>` (or the action) | text, capped at 8 KB |

- `locations`: `[{path}]` (absolute, resolved against cwd) when the input has a path.
- Errors (`Result.IsError`) → `status: failed` with the error text as content.
- Inputs that do not parse fall back to the tool name as title, no locations.

### `session/request_permission`

- `toolCall`: `{toolCallId: <current tool call>, title: "<Kind>: <Subject>",
  rawInput: {detail: <Detail>}}`. The current tool call is tracked from
  `ToolStart` (tools execute sequentially).
- `options`: `allow_once` "Allow once"; `allow_always` "Always allow" (only
  when `Question.CanAlways`); `reject_once` "Deny".
- Outcome: `selected` → `AllowOnce` / `AllowAlways` / `Deny`; `cancelled`, an
  error response or a closed connection → `Deny`.

### Modes (`session/set_mode`)

- `availableModes`: `default`, `auto`, `yolo`, `plan` (names and descriptions
  from the TUI badge wording). `plan` = `SetMode(default)` + `SetPlan(true)`;
  any other mode also sets `SetPlan(false)`.
- Initial `currentModeId` from the resolved mode / plan flag.
- Accepted while idle. Whether `Agent.SetMode` / `SetPlan` are safe during a
  run is verified first; if not, a mid-run `set_mode` → error "wait for the
  current prompt to finish".

### Model picker (`session/set_config_option`)

- `configOptions`: one `select` option, id `model`, category `model`, current
  value `a.Model().Qualified()`, options = `a.Models()` filtered by
  `a.HasCredential` (the current model is always included).
- `set_config_option {configId: "model", value}` → `a.SetModel(value, "")`
  (idle only) → respond with refreshed `configOptions`. An unknown configId or
  value → `-32602`.

### `session/list`

`session.ListAll(<data>/sessions, cwd, 100)` → `sessions: [{sessionId: id8,
cwd, title, updatedAt}]`, no `nextCursor`. `cwd` is optional in the spec:
without it, all sessions are listed (newest first, same cap) — if
`session.ListAll` cannot take an empty workdir filter, it gains that mode.

### `session/load`

1. `session.Find(dir, sessionId, cwd)`; `session.InWorkdir` must hold → else error.
2. `agent.Resume(StartOptions{…same as new…}, path)`.
3. Stream the **full** `a.History()`:
   - user text → `user_message_chunk` (tool-result-only user messages skipped);
   - assistant text → `agent_message_chunk`; thinking → `agent_thought_chunk`;
   - each tool call → `tool_call` with its final status and content from the
     matching tool result (same mapping as live);
   - a compaction summary message → one `agent_message_chunk`
     "_(earlier conversation was summarized)_".
4. Respond `{modes, configOptions}`; send `available_commands_update`.

Loading an id that is already open in this process reuses that agent (no
second `Resume`) but still replays its history; if a prompt is running on it →
error "session is busy".

### Slash commands

`available_commands_update` lists, in TUI precedence order (prompt templates
before skills; first name wins):

- prompt templates: `{name, description, input: {hint: <argument-hint>}}` —
  project templates (`<cwd>/.moca/prompts`) only when trusted, then global;
- skills: `{name, description, input: {hint: "optional request"}}`.

TUI built-ins (`/model`, `/compact`, `/undo`, …) are not exposed — modes and
model have native ACP UI.

## §3 Errors, testing, docs

### Errors

| Condition | Response |
|---|---|
| malformed JSON line | `-32700` (id null); connection stays up |
| unknown method | `-32601` (notifications: ignored, logged) |
| bad params, unknown `sessionId`, bad `cwd` | `-32602` |
| internal failure | `-32603` with the error text |

Logging follows SPECS §13.5: metadata only (method, session id8, durations,
stop reasons) — never prompt text, file contents or commands.

### Testing

- **Pinned schema** — the first task vendors ACP's `schema.json` from a
  pinned release into `internal/acp/testdata/`; the protocol tests check
  moca's emitted JSON against it. Exact discriminator strings and the
  notice / `configOptions` shapes are taken from this file, not from memory.
- `rpc_test.go` — framing, concurrent dispatch, outgoing-call correlation,
  malformed input, EOF shutdown.
- `server_test.go` — an in-process client over `io.Pipe` drives initialize →
  new → prompt (text, tool call, permission allow/deny) → cancel → set_mode →
  set_config_option → list → load, against the scripted provider fakes the
  agent tests use.
- Golden JSON for the event mapping and the tool-call table.
- `cmd/moca` — `moca acp` writes nothing but JSON-RPC to stdout; flag parsing.
- CI gate unchanged: `gofmt`, `go vet`, `go test ./... -race`.

### Manual gate (recorded in SPECS §15)

Zed with the config above: a prompt that reads and edits a file (diff shown), a
shell approval (allow once / always / deny), a cancel mid-run, a mode switch, a
model switch, a `/template` from the `/` menu, then reopening the session from
history.

### Docs

- `docs/specs/SPECS.md`: a new ACP section (wire contract, mapping tables,
  non-goals), CLI contract line, layout + dependency rule, §15 gate evidence.
- `README.md`: "Editors (ACP)" section with Zed, VS Code and Paseo snippets.
- `CHANGELOG.md` entry; `AGENTS.md` layout line (`acp` subcommand sibling file).

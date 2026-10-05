# moca — SPECS.md (current implemented state)

> **Living document — keep in sync with the code.** This file describes **what is implemented today**, precisely enough for a human or an agent to work on the project without reading every source file. It is updated at the end of every phase (and any PR that changes implemented behavior).
>
> `docs/specs/DESIGN.md` remains the v1 vision/contract (rev 11). Where this file and DESIGN.md disagree, **this file wins for the current state** — the disagreement is then a to-do for the next DESIGN revision.
>
> Last updated: 2026-10-05 · phase 3 (TUI, slash commands, status bar, steering, model switching; gate-found fixes: event-pipe deadlock, styled-print sanitizing) — gate evidence and deviations in `docs/plans/phase-3-tui.md` (Implementation notes); review docs in `docs/reviews/`.

---

## 1. Overview

moca is a minimal, token-efficient, provider-agnostic coding agent. Go 1.27.1 (pinned via `mise.toml`), stdlib + one parsing dependency (`mvdan.cc/sh/v3`). Module `github.com/adeotek/moca`, binary `moca`.

**Implemented:** phase 1 — streaming via three hand-rolled protocol codecs, JSONC config, catalog, retry, registry. Phase 2 — the seven frozen tools, symlink-resolving path jail + parsed shell analysis, approvals/trust/yolo, skills + AGENTS.md in a once-built system prompt, append-only JSONL sessions with pre-edit snapshots and undo, and the agent loop behind `moca -p`. Phase 3 — the interactive TUI (`moca` with no `-p`): inline immutable scrollback with numbered `▸`/`⋯` items and a pager, multi-line input with bracketed paste + chips, the status bar, steering during runs, interactive approvals, `!`/`!!`, and slash commands incl. cross-provider `/model` switching (`/compact` still prints "compaction lands in phase 4").

**Not implemented yet:** context compaction/resume (phase 4), MCP proxy (phase 5, the `mcp` tool is a frozen-schema stub), rtk/graphify integration (phase 6), OAuth login (phase 7). `moca login|logout|mcp` print `<cmd> lands in a later phase` and exit 2.

## 2. Build, test, verify

```bash
mise install                      # Go 1.27.1; `go` is not on the global PATH
export PATH="$(mise where go)/bin:$PATH"
go build -o bin/moca ./cmd/moca   # or: go build ./...
go test ./... -race
go vet ./... && gofmt -l .        # both must be clean
./bin/moca --version              # "moca 0.0.0-dev"
```

CI (`.github/workflows/ci.yml`) runs the same gate set — the gofmt check, `go vet`, `go build`, `go test ./... -race` — on every PR, every push to `main`, and manually; Go is pinned from `go.mod`, no secrets required.

- Version stamp: `config.Version`, set via `-ldflags "-X github.com/adeotek/moca/internal/config.Version=…"`, default `"0.0.0-dev"`.
- Repo flow: `main` is PR-only (local pre-push hook blocks pushes to `refs/heads/main`); one branch + PR per phase (`phase/N-<slug>`) or chore (`chore/<slug>`); conventional commits; one version bump per PR (none yet — v0.1.0 at phase 7).
- Gate scripts for the phase-1 style mock checks are not in the repo; a mock SSE server + a config pointing `baseUrl` at it reproduces them (see §10).

### Package layout (implemented)

```
cmd/moca/          main.go (entry, signals), cli.go (flags → Options), oneshot.go (-p agent path), tui.go (TUI path + trust)
internal/llm/      leaf types: Message, ContentBlock, ToolCall, ToolResult, Request, Usage, Event…
internal/config/   version, paths, JSONC pre-pass, typed config + validation, env indirection, AppendString (JSONC-preserving edit)
internal/compact/  chars/4 token estimate helpers (phase 4 grows this)
internal/provider/ SSE reader, error taxonomy, catalog, 3 protocol codecs, replay helpers,
                   retry wrapper, registry (incl. CheckCredential for model switches)
internal/tools/    Tool interface + registry, the 7 tools, ReadTracker, shell runner, ignore matcher
internal/permissions/ path jail, parsed shell analysis (+Windows best-effort), trust store, yolo checkers
internal/skills/   SKILL.md frontmatter/discovery, embedded built-ins, AGENTS.md/CLAUDE.md loader, prompt templates
internal/session/  JSONL transcript (entry/writer/rebuild/repair), snapshot store + undo
internal/agent/    system prompt, events, the loop, cross-provider transform + control surface, Start wiring
internal/tui/      Bubble Tea TUI: pure state (input/items/commands/status/format/sanitize/bridge) + glue (app/pager/trust)
```

Dependency direction: `llm`/`config` import nothing internal; `tools`, `permissions`, `session`, `skills`, `compact` never import each other (`permissions`/`session` satisfy `tools` interfaces structurally); `provider` imports `llm` + `config`; `agent` imports everything; `tui` imports `agent` + the pure packages but never the reverse; `cmd/moca` imports `agent` + `config` + `tui`. HTTP+SSE hand-rolled; non-UI dependency: `mvdan.cc/sh/v3` (shell parsing). UI dependencies (phase 3): `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`.

## 3. CLI surface (as implemented)

```
moca                         opens the interactive TUI in the current directory (phase 3; see §3.5)
moca -p "<prompt>"            one-shot agent run; prompt also on stdin (`-p -`, trailing newlines stripped)
  --model <provider/model>    override config model for this run
  --effort <level>            off|minimal|low|medium|high|xhigh|max (validated; clamped per model)
  --approve | --no-approve    project trust for this run (default: no; `--yolo` implies yes unless --no-approve)
  --yolo | --no-yolo          override config `yolo`; yolo turns every permission check off (§11)
  --resume <id8|last>         parsed; takes effect in phase 4
  --continue                  parsed; takes effect in phase 4
  --version
  --config <path>             dev/test: config file override (hidden from help text)
moca login|logout|mcp …       stubs: "<cmd> lands in a later phase", exit 2
```

**`-p` output contract.** stdout = the final assistant answer text only, printed once when the run completes (`Outcome.Text + "\n"`); a hard error prints nothing to stdout. stderr carries the live trace: text of turns that called tools (indented two spaces), `▸ <tool> <summary>` per successful call / `✗ <tool> <first error line>` per failed call, `retry N/M · <wait>` notices, `warning: …` lines (skill discovery), the yolo banner, and the exit summary `tokens <in+cache>/<out> · $<cost>`.

**Exit codes.** `0` completed · `1` provider/runtime error, a final `length`/`refusal` stop (`response truncated (token limit reached)` / `model refused to answer`; the partial answer is still printed), or a failed run setup (data dir, session files, skills — `agent.StartError`) · `2` config, model or usage error (incl. unset `env:` vars) · `3` maxSteps reached (the wrap-up summary is still printed) · `130` interrupted (SIGINT **and** SIGTERM cancel in-flight work). The TUI maps errors the same way (`exitFor`): SIGINT/SIGTERM → `130` (no message; the program runs with `tea.WithoutSignalHandler` — `main`'s signal context is the only handler, Bubble Tea's own raced it at shutdown and could hang), a recovered panic → `1`, ctrl+c/esc at the trust prompt → `130`; a clean quit after `/clear`/ctrl+c is `0`.

### 3.5 TUI (`moca`, no `-p`)

Inline Bubble Tea v2 TUI (no alt-screen except the pager). Layout top→bottom: scrollback (immutable; finished output goes through `tea.Println`) · 3-row input box · one-line status bar.

- **Trust**: `--approve`/`--no-approve` wins; else yolo → trusted; else the saved decision in `trust.json`; else, when the workdir has `.moca/`/`AGENTS.md`/`CLAUDE.md`, a one-time `[y/N]` prompt (saved). Project prompt templates load from `<workdir>/.moca/prompts` (trusted only) + `~/.config/moca/prompts`.
- **Status bar** (event-driven, bottom): `cwd · branch · provider/model · effort · ctx W · P% · in/out · $cost` (`sub` for OAuth; transient `retry n/m · t` replaces cost; red `YOLO` leads when yolo is on and is never dropped for width; fields drop in the order cwd → branch → tokens, then the model truncates; content is sanitized before styling). Branch resolves at init, every TurnEnd, and run end. Context `%` = `agent.Status().ContextTokens` (usage-anchored, §6) vs the catalog window.
- **Keys**: `enter` send · `shift+enter`/`alt+enter`/`ctrl+j` newline (one-time hint when the terminal reports no key disambiguation; the view requests `ReportAllKeysAsEscapeCodes`) · `↑/↓` history at single-line buffer edges · `esc` interrupt (approval visible → deny) · `ctrl+c` clear input / double-press ≤1s on empty quits · `ctrl+o` pager on the latest item · `alt+p` toggle paste chips · `g`/`G`/`q`/`esc`/arrows/pgup/pgdn in the pager (alt-screen, closed by `q`/`esc`).
- **Pager**: scrollback output produced while the alt-screen pager is open is held (Bubble Tea's `Println` is lost in the alt screen) and printed when it closes.
- **Input**: pastes insert at the cursor (CR/CRLF normalized to LF); chips `[paste N lines #K]` for >50-line or control-byte pastes (content verbatim until send; unsafe pastes never expand); a broken marker sends literally; history stores expanded text and re-chips control-byte entries on recall; `esc` returns queued steering above the draft.
- **Runs**: numbered items shared across tools and thinking (`▸ #7 tool summary`, `✗` on error, `⋯ #6 thinking N lines`; bodies open in the pager); streaming text commits completed lines to scrollback; `… <name>` while a tool runs; thinking is never expanded outside the pager. A message sent mid-run is **steering** (queued, drained after the current tool batch; printed `↳ queued:` / `↳ sent:`); a no-tool-call turn appends it and continues.
- **Commands**: `/model [id]` (list/switch; refuses while running; forfeits the prompt cache), `/effort [level]`, `/hard` (modelHard + high ↔ saved pair), `/yolo` (between runs; red bar field), `/clear` (new session file; old stays resumable; keeps the current model/effort — hard mode exits to its saved pair — and the commands approved with `[A]`; yolo resets to the startup flag), `/compact` (phase-4 note), `/cost`, `/undo`, `/copy` (OSC 52), `/show <n>`, `/help`; `/model`,`/effort`,`/hard`,`/yolo`,`/clear` are refused **while a run or a `!` command is in progress**. Prompt templates: `/name args` with `$ARGUMENTS`/`$@`/`$1..$9`/`$$`; built-in names win; unknown → error. `/`-escaped text: `//x` sends `/x`.
- **`!cmd` / `!!cmd`**: user-initiated shell in the workdir (30s timeout, `ShellEnv` hygiene, output truncated 30K, `[exit N]`); `!` additionally appends `"$ cmd\n<output>\n[exit N]"` as a user message (bypasses the model-facing analyser, §7); `!!` is display-only. `!` is refused while a run is in progress, and a run/`/clear` is refused while a `!` is still executing (its note would land between a tool batch's results or in the wrong session; `!!` still runs).
- **Approvals**: shell/MCP questions render `allow \`x\`? [a] once [A] always [d] deny` (`[A]` only when `CanAlways`; `rm` never offers it) — `A` persists the command into `config.jsonc` `shell.allow` via the comment-preserving `config.AppendString` (0600 + `.bak`, atomic rename). The typed draft survives: `a`/`A`/`d` only answer after a 700 ms typing pause (so letters meant for the draft cannot answer — or persist — a prompt that pops up mid-sentence), `esc` always denies, and `enter`/`ctrl+c` keep their normal meaning while a prompt is shown. `A` also adds the command to the running session's allow list; the config write goes through a symlinked `config.jsonc` and keeps the file's mode.
- **Bridge/threading**: the agent runs in a goroutine; `Emit` delivers events through a FIFO pipe (`eventPipe`) because `Program.Send` blocks on an unbuffered channel and `Update` runs on the event-loop goroutine (a synchronous emit — e.g. `/yolo` — would deadlock); the asker blocks the agent on a reply channel. The agent guards its shared state with a mutex, so the TUI's `Status()` polling during a run is race-free.
- **Rendering safety**: model/user/tool/error text goes through `Sanitize` (C0 controls → `^[`-style, DEL → `^?`, C1 controls U+0080–U+009F → their ESC form, e.g. U+009B → `^[[`) before it reaches the terminal — including the approval prompt and the status bar (sanitized *before* styling); styled lines are printed without sanitizing.

## 4. Config

- Path `~/.config/moca/config.jsonc` (`$XDG_CONFIG_HOME` honoured); data dir `~/.local/share/moca` (`$XDG_DATA_HOME`). Missing file → built-in defaults. Strict decode: **unknown keys are rejected**; errors carry `line:col`.
- **JSONC pre-pass** (`config.Standardize`): comments (`//`, `/* */`) are recognised only outside string literals (escapes honoured) and replaced by spaces — output has the **same length** as the input, so error offsets still map to the original line:col. A trailing comma (`,` followed only by whitespace/comments before `}`/`]`) is blanked. Unterminated `/*` → error. The §12 example of DESIGN.md is the byte-exact fixture (`internal/config/testdata/example.jsonc`).
- **Keys** (camelCase): `model`, `modelHard`, `providers.<name>` (`auth` = `api_key|oauth`, `apiKey` must be `"env:VAR"`, `baseUrl`, `baseUrls` per protocol, `protocol`, `models.<id>` with `protocol`/`contextWindow`/`maxOutputTokens`/`cost`), `shell.allow`, `mcp.idleTimeout` + `mcp.servers.<name>` (command|url, args, env, headers, description, approve), `snapshot.retentionDays` (0 = forever), `context.reserveTokens|keepRecentTokens|maxSteps`, `yolo`.
- **Defaults**: built-in providers `anthropic`/`opencode-go`/`openai` with `auth: api_key` and `env:ANTHROPIC_API_KEY` / `env:OPENCODE_API_KEY` / `env:OPENAI_API_KEY`; `shell.allow` = the 32-entry DESIGN §12 list; `mcp.idleTimeout` 600; `context` 16384/20000/40; `snapshot.retentionDays` 30; `yolo` false.
- **Validation**: model ids must be `provider/model` (split at first `/`); provider must exist; `apiKey` literal → rejected (must be `env:`); custom providers need `baseUrl`(+`baseUrls`) and `protocol`; model-level `protocol` must be one of the three; custom models must declare `contextWindow`; `contextWindow < 16384` refused; context values and `retentionDays >= 0` checked; each MCP server must have exactly one of `command`/`url`.
- **Env indirection**: `env:VAR` resolved lazily — an unset variable errors **when the first request to that provider is built** (naming the variable, `*config.EnvError` → exit 2), never at startup; unused providers don't block. `config.EnvRefs` lists every referenced variable (used by phase 2's shell hygiene).
- **Custom providers** (vLLM/LM Studio/Ollama…): any name + `baseUrl` + `protocol` + models with `contextWindow`; usable through the same registry path.
- **`config.AppendString(path, keyPath, value, init)`** (phase 3) — comment-preserving JSONC edit: appends `value` to the string array at `keyPath` (creating missing objects / the file, 0600), no-op when present, everything else byte-identical (string-literal-aware `Standardize` + an offset-tracking scanner; trailing commas and `//`/`/* */` comments survive, also those containing commas). Writes `<path>.bak` then an atomic rename. Used by the TUI's allow-always and (phase 5) by MCP approvals.

## 5. Model catalog, effort, thinking

`internal/provider/catalog.go` — `Model{Provider, ID, Protocol, ContextWindow, MaxOutput, Cost(USD/M input/output/cacheRead/cacheWrite), ThinkingMode, ThinkingLevelMap}`.

**Built-in catalog** (prices/limits verified 2026-10-04 — re-verify against the sources listed in the file's comment when vendor pages change; cost = standard non-tiered rates):

| provider | model | protocol | ctx | max out | in/out/cacheR/cacheW $/M | thinking |
|---|---|---|---|---|---|---|
| opencode-go | glm-5.3 | openai-completions | 1,000,000 | 131,072 | 1.4 / 4.4 / 0.26 / 0 | openai + glmThinking |
| opencode-go | glm-5.3-flash | openai-completions | 1,000,000 | 131,072 | 0.15 / 0.5 / 0.03 / 0 | openai + glmThinking |
| opencode-go | minimax-m3 | anthropic-messages | 1,000,000 | 131,072 | 0.3 / 1.2 / 0.06 / 0 | budget |
| opencode-go | kimi-k3 | openai-completions | 1,048,576 | 131,072 | 3 / 15 / 0.3 / 0 | openai + glmThinking |
| opencode-go | gpt-6-luna | openai-responses | 1,050,000 | 128,000 | 0.1 / 0.5 / 0.01 / 0.125 | openai + openaiReasoning |
| opencode-go | grok-4.7 | openai-responses | 500,000 | 500,000 | 2 / 6 / 0.5 / 0 | openai + openaiReasoning |
| anthropic | claude-opus-5-5 | anthropic-messages | 1,000,000 | 128,000 | 4 / 20 / 0.2 / 5 | adaptive |
| anthropic | claude-sonnet-5-5 | anthropic-messages | 1,000,000 | 128,000 | 2 / 10 / 0.2 / 2.5 | adaptive |
| anthropic | claude-haiku-4-5 | anthropic-messages | 200,000 | 64,000 | 1 / 5 / 0.1 / 1.25 | budget |
| openai | gpt-6-astra | openai-responses | 1,050,000 | 128,000 | 10 / 50 / 1 / 12.5 | openai + openaiReasoning |
| openai | gpt-6-luna | openai-responses | 1,050,000 | 128,000 | 0.1 / 0.5 / 0.01 / 0.125 | openai + openaiReasoning |

**Thinking modes & level maps** (a map value is the wire value; a missing key = unsupported level):

- `none` — no thinking; every effort clamps to `off`.
- `budget` (anthropic manual mode) — sends `thinking: {type:"enabled", budget_tokens:N}`; omitted when effort is `off` **or** `N == 0`.
- `adaptive` (current Claude) — sends `thinking: {type:"adaptive", display:"summarized"}` + `output_config.effort`; map `anthropicAdaptive` = {low, medium, high, max} (so `off`/`minimal` clamp up to `low`; `xhigh` clamps down to `high`).
- `openai` — completions: `reasoning_effort`; responses: `reasoning: {effort, summary:"auto"}`. Maps: `openaiReasoning` = {minimal, low, medium, high}; `glmThinking` = {off:"none", low, medium, high}.

**Effort clamping** (`Model.ClampEffort`): nearest supported level **at or below** the request; if none is below, the lowest supported; `off` on a model that cannot disable → lowest supported.

**Budget** (`Model.BudgetTokens(maxTokens, effort)`): `maxTokens < 2048` → `0` (thinking omitted — the API floor is 1024 and 1024 must stay free for the answer); else a share of `maxTokens`: minimal/low 25 %, medium 50 %, high 75 %, max 100 %, floored at 1024 and capped at `maxTokens − 1024`. (Effort is pre-clamped by the adapter, so only mapped levels arrive; `off` never sends thinking.)

**Max output** (`Model.MaxTokens(reserve)`): `min(MaxOutput, reserve)` — reserve = `context.reserveTokens` (16384).

**Cost** (`Model.CostOf(usage)`): `Σ tokens × rate / 1e6`.

## 6. Providers & registry

`provider.NewRegistry(cfg, httpClient, notify)` builds the catalog (built-ins + config overrides/additions) and validates that `model`/`modelHard` resolve.

- **Default base URLs** — `anthropic` → `https://api.anthropic.com`; `openai` → `https://api.openai.com/v1`; `opencode-go` → anthropic-messages `https://opencode.ai/zen/go`, openai-* `https://opencode.ai/zen/go/v1`. **Precedence**: `baseUrls[protocol]` > `baseUrl` > built-in default.
- **Model overrides merge, not replace**: a config entry for a catalog model changes only the fields it sets; a new model under a custom provider needs `protocol` + `contextWindow` (config validation); a new model under a **built-in** provider needs `protocol` (model-level or provider-level) — otherwise `NewRegistry` fails fast with the exact key path to add.
- **Credentials are lazy**: `api_key` resolves `env:VAR` at request time (missing → clear error naming the variable); `oauth` providers error with "run `moca login <provider>`" until phase 7 plugs a `CredentialFunc` in via `Registry.SetOAuth`.
- **OpenCode Go routing header**: every `opencode-go` request carries a stable per-conversation `x-opencode-session` (a `crypto/rand` string generated per process — one process is one conversation today; phase 3+ sessions must rebind it per conversation) and every request from every provider identifies the client as `User-Agent: moca/<version>`. The Go tier rejects session-less requests with HTTP 400 `MissingSessionID` (verified live 2026-10-05, https://opencode.ai/docs/go/#where-can-i-use-it); zen pay-as-you-go (`/zen/v1`) does not require it.
- `Resolve("provider/model")` → `(Model, Adapter)`; the adapter is already wrapped in the retry policy. `Models()` returns the catalog sorted (for phase 3's `/model`).

## 7. Protocol adapters (wire behavior)

All three: `post()` sends JSON with `Content-Type: application/json`, `Accept: text/event-stream`, `User-Agent: moca/<version>`; **a 2xx whose Content-Type is set and not `text/event-stream` is an error** ("unexpected … response, want text/event-stream: <body≤8KiB>", non-retryable) instead of a fake turn. Response bodies are closed on every path; stall timeout 90 s.

**anthropic-messages** — `POST {base}/v1/messages`; headers `anthropic-version: 2023-06-01`, `x-api-key` (or `Authorization: Bearer` for OAuth tokens).
- Body: `system` as one text block; `tools` (last tool carries `cache_control`); `tool_choice`; messages; thinking per §5. Cache breakpoints: system end + last tool + **the last eligible block of the last message** (text non-empty / tool_use / tool_result — never thinking/redacted/empty-text; walks back; omitted if none). `NoCacheWrite` (compaction) removes every `cache_control`.
- Stream: `message_start` (usage in), `content_block_*` (text/thinking/redacted_thinking/tool_use; `input_json_delta` assembled per index), `message_delta` (stop reason + output usage), `message_stop`. Missing `message_stop` at clean EOF → `io.ErrUnexpectedEOF` (retryable). In-stream `overloaded_error` → `*HTTPError{529}`. HTTP 400 "prompt is too long" → `ErrContextOverflow`.
- Thinking capture: `Model` = request model, `Signature` from `signature_delta`; `redacted_thinking` → `Redacted=true`, opaque `data` in `Signature`, text `[Reasoning redacted]`.
- Stop mapping: `tool_use`→tool_use, `max_tokens`/`model_context_window_exceeded`→length, `refusal`→refusal, else end_turn.

**openai-completions** — `POST {base}/chat/completions`; `Authorization: Bearer`. Body: system first; per message, text joined, tool calls with `arguments` as a JSON string, tool results as `role:"tool"`; `stream:true`, `stream_options.include_usage`; `reasoning_effort` per §5. Thinking blocks from history are **never sent** (no wire slot). Stream chunks: `content`, `reasoning_content`/`reasoning` (→ thinking events), `tool_calls[]` assembled **per `index`** (id/name/args may be split across deltas), `finish_reason`, usage (`prompt_tokens − cached_tokens` → Input, `cached_tokens` → CacheRead). Terminal: `[DONE]` or any `finish_reason`; otherwise `io.ErrUnexpectedEOF`. `content_filter`→refusal, `length`→length, `tool_calls`→tool_use; calls present with a `stop`/missing reason → tool_use. Overflow: 400 with `context_length_exceeded` / `maximum context length`.

**openai-responses** — `POST {base}/responses`; `Authorization: Bearer`. Body: `input` items (message text in/out, reasoning items with `encrypted_content`, function_call/function_call_output), `store:false`, `include:["reasoning.encrypted_content"]` (stateless reasoning replay), `instructions` = system, tools as `{type:function,…}`, `reasoning` per §5. Stream: `response.output_text.delta`, `response.reasoning_summary_text.delta` (accumulated per `item_id` as a fallback for the final item's summary), `response.output_item.done` (reasoning → thinking block with `ThinkingID`+`Signature`; function_call → call), `response.completed|incomplete` (usage; `incomplete`+`max_output_tokens`→length), `response.failed` (context_length_exceeded → `ErrContextOverflow`; server_error/rate_limit_exceeded → retryable 500). Terminal: completed/incomplete; otherwise `io.ErrUnexpectedEOF`.

**Tool calls — kept, never dropped.** Arguments are assembled and kept **as received**, even when cut off mid-JSON (`{}` only when nothing arrived). This is the phase-2 contract: the agent loop attaches a §6 error result ("your output was cut off at the token limit… split the work") to **every** call of a `length`-stopped turn, and the tool registry rejects invalid JSON with its own error result. A `tool_use` block that never received `content_block_stop` is kept with a nil input (not emitted). Calls are emitted to the event stream in order (anthropic: at `content_block_stop`; the OpenAI protocols: after the stream ends, index order). The stop reason is **never** force-upgraded from `length`/`refusal`; `end`+calls → `tool_use`.

**History replay** (`replay.go`):
- Thinking replays **verbatim** (anthropic `thinking`+signature / `redacted_thinking`+data; responses reasoning item) only when `block.Model == request model` **and** `Signature != ""` (responses also needs `ThinkingID` and not redacted). Anything else degrades to a text block `[prior reasoning]\n<text>` (anthropic / responses-as-output_text); redacted or empty-text blocks are dropped. Rationale: signatures/encrypted payloads are bound to the producing model — replaying them elsewhere (or unsigned) is a 400 on every later turn.
- Tool input on replay: nil or invalid JSON → `{}` in all three adapters (invalid `json.RawMessage` would otherwise fail `json.Marshal` or send `"arguments": ""`).

**Usage normalization** (all protocols): `Input` excludes cache reads/writes; `CacheRead`/`CacheWrite` carried separately; `Usage.Add` accumulates.

## 8. Retry, errors, exit mapping

`WithRetry(adapter, DefaultRetryPolicy(notify))` — uniform across protocols.

- **Retryable**: HTTP 408/429/5xx (incl. anthropic 529); `ErrStall`; `io.EOF`, `io.ErrUnexpectedEOF`; `ECONNRESET`, `ECONNREFUSED`, `ECONNABORTED`, `EPIPE`; net timeouts; HTTP/2 GOAWAY / stream-reset (matched by error text — the stdlib types are unexported). **Not retryable**: other 4xx, `ErrContextOverflow`, `context.Canceled`, non-SSE 2xx.
- **Schedule**: 1/2/4/8/16 s (±20 % jitter), max 5 attempts, only while nothing has been emitted. `Retry-After` (integer, fractional or HTTP-date; ≤0 → 0) overrides the wait when larger; the wait is capped at 60 s.
- **Mid-stream failure** (anything already emitted): emit `EventReset` (consumers discard partial output), pause (jittered first delay, retry-after honoured — announced via `Notify` like any retry), retry the same request **once**; a second failure surfaces. This retry is outside the 5-attempt budget.
- **Stall**: no bytes for 90 s → `ErrStall` (aborts the attempt; retried).
- **Errors**: `*HTTPError{Status, RetryAfter, Body≤8KiB}`; `ErrStall`; `ErrContextOverflow` (wrapped by adapters); `*config.EnvError`. `cmd` maps: `EnvError`→2, cancellation/ctx→130, everything else→1.

## 9. `-p` run semantics

`-p` builds the full agent (§14): resolves the model, clamps the effort, and runs one conversation whose workdir is the process cwd. Trust for the run: explicit `--approve`/`--no-approve` wins, else yolo → trusted, else **no** (project `.moca/**` and `AGENTS.md`/`CLAUDE.md` are ignored). Session slug = first five words of the prompt. The run ends when the model answers without tool calls; `context.maxSteps` batches trigger one wrap-up request (`tool_choice: none`, tools still listed) and exit 3.

## 10. Tools (seven, frozen)

Schemas are frozen since phase 2 (`internal/tools/testdata/schemas.golden.json`; changing one is a v2 discussion): `read, write, edit, shell, search, ls, mcp`. Execution: calls of one turn run **sequentially in emitted order**; a failed call does not cancel the rest; every call gets exactly one `tool_result`. Unknown tool → error listing the available tools; invalid JSON arguments → error telling the model to split the work (§6).

- `read` — `N|content` lines + `[lines A-B of TOTAL]` footer (+ `use offset=B+1 to continue` when more remain); caps 2000 lines / 50K chars per call; lines >2000 chars truncated with `[… line truncated]`; NUL in the first 8K → refused with size + MIME guess; a directory → "use ls"; non-regular files (FIFOs, devices) and files >256 MB are refused; the window is streamed with a bounded reader (no whole-file load); every successful read records mtime/size/sha256 (ReadTracker, streamed hash).
- `write` — creates files (parents created inside the jail); an existing file needs a prior read **and** must be unchanged on disk since; mode preserved (new files 0644); after writing, the file is recorded so consecutive edits don't need a re-read.
- `edit` — the edit ladder (§5): exact unique match → replace; several exact matches → error listing the lines + suggests `replace_all`; `replace_all` replaces every exact occurrence (never falls back to fuzzy); no exact match → one whitespace-insensitive match accepted and `new_string` re-indented to the file's indentation; none/ambiguous → model-facing error (mentions `N|` prefixes when the text looks like pasted read output); empty `old_string` → "use write"; identical strings → error; CRLF and BOM preserved; non-UTF-8 refused. Result: unified diff + `changed lines …; file now N lines`; Summary `path [+a −d]`.
- `shell` — stateless (cwd = jail root every call); `bash -c` (Unix) / `pwsh -NoProfile -Command` → `powershell.exe` fallback; stdin `/dev/null`; stdout+stderr merged; invalid UTF-8 bytes replaced; head+tail truncation at 30K chars with `[… N lines omitted]`; capture itself is bounded at 64K head+tail with a `[… N bytes omitted]` note; `cmd.WaitDelay` 2 s stops a straggler (e.g. a `setsid` descendant) holding the pipe from blocking after a kill; `[exit N]` always reported (`[timed out after Ns — process group killed]` on timeout); timeout 1–300 s (default 30) kills the whole process group; env = inherited minus every `config.EnvRefs` variable, plus `PAGER=cat GIT_PAGER=cat GIT_EDITOR=true GIT_TERMINAL_PROMPT=0`.
- `search` — RE2 regex; respects `.gitignore`/`.ignore` from the workdir down to the search path (deeper files override); files >4 MB are skipped with a visible note; skips hidden + binary; `glob` matched against the basename (or the relative path when it contains `/`); 200-hit cap with a visible note; `files_only`; output `path:line:text` (text cut at 300 chars).
- `ls` — one directory level, sorted; dirs `/`, symlinks `@`; 1000-entry cap; hidden entries only with `hidden:true`.
- `mcp` — frozen ~200-token schema (search/describe/call); every action returns "no MCP servers configured". Phase 5 registers the real tool with a byte-identical `tools.MCPSpec()`.

Approvals: allowlisted-but-unapproved → ask (AllowAlways persists via `Commands.Allow`); `rm` → ask every time (`CanAlways:false`, never allowlistable); `Ask == nil` (a `-p` run) = deny.

## 11. Permissions

- **Path jail** — root + read-only roots canonicalized once (abs + symlink resolution); relative paths join the root; `~` expands to `$HOME`; the target resolves symlinks on the deepest existing ancestor and re-appends the non-existent tail; reads are allowed inside the root or a read-only root, writes only inside the root; Windows comparison is case-insensitive and slash-normalized. Refusals name the jail ("outside the workdir … symlinks resolved").
- **Shell analysis** (Unix; parses with `mvdan.cc/sh` — unparseable input is refused): every command name is classified — **hard-deny** (no override): `sudo su doas dd shred chown` + `mkfs*`; **refused**: `eval source . exec`; builtins: `cd pwd echo printf test [ true false exit`; `rm`: ask every time; anything else: allowlisted or ask-once. Wrapper commands are unwrapped with their options parsed exactly (`env -u/-C/--unset=…`, `command -p`/`-v`, `time`, `nice`, `timeout`, `nohup`); an option the analyser cannot model refuses the command, and `env -S` is always refused. A command word containing backslashes, globs or `$'…'` strings is non-literal and refused like `$CMD`. Redirect and `tee` targets are jail-checked (`/dev/null|stdout|stderr` exempt; numeric fd dup like `2>&1` exempt); `<>` targets are checked; after a `cd`/`pushd`/`popd` every relative redirect/`tee` target is refused (the cwd can no longer be assumed). Every segment of `|`, `;`, `&&`, `||`, subshells and command substitutions is visited.
- **Windows** — best-effort (no PowerShell parser): first token of each `;`/`|`/`&&`/`||` segment, lowercased with `.exe` stripped; ask-list extended with `Remove-Item rm ri del erase rd rmdir`; `Format-Volume` hard-denied. Weaker than Unix, by design.
- **Trust store** — `~/.local/share/moca/trust.json` (0600, atomic write; keys are canonical paths): `Lookup`/`Set`. Trust gates `<workdir>/.moca/**` and `AGENTS.md`/`CLAUDE.md`. `-p` rule: explicit flag > yolo > **no** (saved decisions are ignored in `-p`); the TUI uses the saved decisions and asks once on first open of an untrusted workdir with project resources.
- **Yolo** (§7.5) — swaps the injected checkers (`permissions.Unjailed`, `permissions.AllowAll`, `tools.AutoAllow`) — never `if yolo` branches inside tools. Lifts: jail, shell analysis incl. hard-deny, approvals, trust. Stays on: `maxSteps`, read-before-write + staleness, snapshots, shell env hygiene / timeout / process-group kill / truncation. Recorded in the session header (`yolo`) and a `permission_mode` entry per `SetYolo` (between runs only).

## 12. Skills & project instructions

- **SKILL.md frontmatter** — `---`-delimited YAML subset: top-level scalars; quoted scalars unquoted; `|`/`>` block scalars (folded joins with spaces); nested maps/lists skipped; BOM + CRLF tolerated; missing frontmatter is an error.
- **Discovery** (`skills.Discover`) — precedence: project `<workdir>/.moca/skills` (trusted runs only), global `~/.config/moca/skills`, builtin `<data>/builtin-skills/<content hash>`; first source wins a name collision; sorted by name; a skill without a description is skipped and reported; symlinked skill directories are reported and skipped (their bodies would fall outside the read jail). Discovery problems surface as `Warning` events (`warning: …` lines in `-p`).
- **Built-ins** — `internal/skills/builtin/**` embedded (`embed`); `ExtractBuiltins(dataDir)` materializes them into `<data>/builtin-skills/<content hash>` (unique temp dir + rename; a lost rename race is success) so the model can `read` them (builtin dir is a read-only jail root). Currently: `rtk`.
- **Instructions** — `LoadInstructions`: global `AGENTS.md`, then (trusted only) `workdir/AGENTS.md`, else `workdir/CLAUDE.md`; each capped at 32K chars with `[… truncated at 32K chars]`.
- **Prompt templates** (phase 3) — `skills.LoadPrompts(dirs)` reads `<dir>/<name>.md` (optional frontmatter `description`/`argument-hint`; first dir wins a collision; sorted); `ExpandPrompt` substitutes `$ARGUMENTS`/`$@` (all args), `$1..$9` (whitespace-split) and `$$` (literal `$`). The TUI resolves `/name` against built-ins first, then templates.
- Only name + one-line description + absolute path enter the system prompt; bodies load on demand via `read`.

## 13. Sessions & snapshots

- **Transcript** — append-only JSONL at `~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl` (file 0600, dir 0700); every entry has `id`, `parentId` (chain), `type`, `ts`. Types: `session` (header: workdir, provider, model, **effective** effort, startedAt, mocaVersion, the full system prompt, yolo), `message`, `tool_use`, `tool_result`, `snapshot`, `error`, `permission_mode`, `model_change` (written by `/model`, `/effort`, `/hard`; `compaction` reserved for phase 4). `Slug`: first five words, lowercase ASCII, ≤32 chars. `ReadFile` tolerates a truncated last line; `Open` repairs it before appending (a partial line is dropped, a complete line missing only its newline gets one) and continues the chain.
- **Rebuild** (`session.Messages`) — an assistant `message` + its `tool_use` entries (by `messageId`) form one assistant message (calls after text/thinking); consecutive user-side entries merge into one user message (results first, append order); non-conversation entries are skipped.
- **Repair** — synthetic error `tool_result`s for every unanswered `tool_use` (`aborted by user` / `interrupted — moca exited before completion`); idempotent. The loop appends them on abort; resume uses them in phase 4.
- **Snapshots** — pre-edit content stored at `~/.local/share/moca/snapshot/<sha256>` (0600); git-tracked **clean** files are skipped (`git ls-files --error-unmatch` + empty `git status --porcelain`) and recorded `git:true`. `Undo` pops the newest record: git → message telling the user to `git restore -- <path>`; otherwise refuses if the file changed since moca wrote it, then restores content **and mode** (or deletes a file moca created). `Prune(days)` removes blobs older than `snapshot.retentionDays` (0 = keep forever) at startup.

## 14. Agent loop

- **Turn contract** — request = stored system prompt + rebuilt messages **transformed for the target model** + the seven tool specs + `tool_choice` + `maxTokens = Model.MaxTokens(context.reserveTokens)` + clamped effort. The response persists as an assistant `message` entry (text/thinking only) with `usage`/`model`/`cost`, then one `tool_use` entry per call, then a `TurnEnd` event.
- **Cross-provider transform** (phase 3, §3) — applied when building every request: thinking replays unchanged only when `block.Model == a.model.Qualified()` (adapters stamp the bare id at capture; the agent stamps the qualified id at persist time; the adapters compare against their own qualified model), anything else with text → `[prior reasoning]\n…` text block, empty → dropped; tool-call ids normalized deterministically (`NormalizeToolID`) on both use and result. Input history is never mutated.
- **Control surface** (phase 3) — `SetModel(q, effort)` (credential check first via `Registry.CheckCredential` — env/oauth resolution, no network; failure changes nothing), `SetEffort` (clamps, returns effective), `ToggleHard` (saves `{model,effort}` ↔ `modelHard`+high; both directions write `model_change`), `Models()`, `Status()` (model, effort, anchored context tokens, window, usage, cost, `Sub` = provider auth oauth, `Hard`, `Yolo`), `Steer`/`TakeSteering` (mutex-guarded queue), `AddNote` (`!` messages), `Undo`. All state the TUI reads during a run (queue, transcript mirror, usage/cost, anchor) is mutex-guarded; model/effort commands are between-runs only.
- **Loop** — no tool calls → drain steering, else done (`Outcome.Text`). Calls execute sequentially (§10); a `length`-stopped turn executes nothing — each call gets the §6 "cut off — split the work" error result instead, and a call whose arguments were cut off mid-JSON is persisted as `{}` so the run never dies on a marshal error. **Steering** is applied after every complete tool batch (before the next request); on abort the queue is untouched for `TakeSteering`. After `context.maxSteps` tool batches: one wrap-up request (user text "You have reached the step limit…", `tool_choice: none`, tools still listed) → `Outcome{MaxSteps:true}`; calls the wrap-up turn returns anyway get synthetic error results (`session.Repair`). Cancellation appends `aborted by user` results for every unanswered call and returns the ctx error.
- **Context estimate anchor** — after each successful turn: `anchor = UsageTokens(usage)`, `anchorEntries = len(entries)` (usage covers the request+response, so everything appended later is the delta); `SetModel`/`model_change` invalidates the anchor (fallback: chars/4 of the whole request). `compact.Tokens/MessageChars/RequestChars/UsageTokens` live in `internal/compact`.
- **Events** — `TextDelta`, `ThinkingDelta`, `StreamReset`, `ToolStart`, `ToolEnd{Call,Result}`, `TurnEnd{Message,Usage,Cost,Stop}`, `Retry{Notice}`, `Warning{Text}`, `YoloChanged`, `SteeringApplied{Texts}`.
- **System prompt** — built once per run by `agent.Start`, stored in the `session` entry, never rebuilt: environment (workdir, platform, date, git state — **no hostname**), tool rules, working style, token discipline, skills list (`- <name>: <desc> (<abs path>)`), MCP server roster, project instructions. The version stamp ends the core block; core stays ~40 lines.
- **Start wiring** — jail (root + global/builtin skills read-only) → shell analyser → builtins extraction → skills discovery → instructions → prompt → snapshot prune → session create (with the prompt) → `tools.Env` (jail, shell, ask, ReadTracker, snapshots, shell env with `config.EnvRefs` stripped) → registry → `New`. Yolo applies at construction when requested (header records it).

## 15. Verification status & known items

- Unit/integration: `go test ./... -race` — **219 top-level tests across 11 packages** (phase 2's ladders keep passing), plus phase-3 suites: the cross-provider transform (incl. same-bare-id/different-provider), model/effort switching + `CheckCredential` refusal, steering (lands after tool results / becomes the next message / returns on abort), the anchored context estimate, prompt templates, `AppendString` byte-exact + truncated-input fixtures, status-bar formatting + width dropping, input state (chips, marker collisions, history incl. control-byte recall re-chipping, ctrl+c), items, TUI glue (streaming commit, approval keys vs draft, thinking collapsed, paste, transient clear), and the event pipe (non-blocking, FIFO, pre-program buffering), plus the pass-3 review fixes (pager output hold, approval typing guard, `!` busy guard, paste at cursor/CRLF, C1 sanitising, symlink-safe `AppendString`, `/clear` carry-over, exit mapping) driven through a model built around a real agent.
- Reproducible local gates: forced-429 backoff (mock SSE server); protocol e2e through the real binary for anthropic-messages + openai-responses + stdin; §12 example decodes intact; schema golden.
- **Independent review (2026-10-05)**: `docs/reviews/2026-10-05-phase-2-tools-agent-loop.md` (verdict: Block) — all findings fixed with regression tests. **Phase-3 reviews**: pass 1 `docs/reviews/2026-10-05-phase-3-tui.md` (Approve with fixes: 1M/3L), pass 2 `docs/reviews/2026-10-05-phase-3-tui-pass-2.md` (Approve with fixes: 1H/2M/6L) — all fixed in `640be8e` with regression tests (startup panic from pre-program events, `AppendString` bounds, `/clear` session/state ordering, chip-marker collisions, live flush + transient clear on failed runs, mid-run `!` refusal, approval-prompt sanitizing, `GitBranch` timeouts); three items recorded as intentionally unchanged (see the plan's Review fixes).
- **Phase-3 gate (2026-10-05)** — mock-driven TUI session in a real tmux terminal (scripted openai-completions server): status bar from the first frame (`repo · main · fake/m · off · ctx 128k · …`), branch switch via the shell tool reflected in the bar, streamed text committed to scrollback, `⋯ #1 thinking 3 lines` / `▸ #2 ls . (1)` items, `/help /cost /model` (list + cross-model switch) `/effort` (clamp note) `/hard` (both directions restore) `/copy /show /compact`, missing-key switch refused naming `OPENAI_API_KEY` with the model unchanged, no-enhanced-keyboard hint + `alt+enter` newline, 120-line bracketed paste → chip → `alt+p` expand → send with all 120 lines intact in the transcript, steering verified in the captured request payload (`tool result → user "also check README"`), approvals (`a` ran; `A` appended `python3` to `shell.allow` with the file's comment and trailing comma byte-preserved + `.bak`; `rm` prompted without `[A]` and deny kept the file), `/yolo` on/off incl. a command running with no prompt and the 60-column bar keeping `YOLO`, `!!touch` local, `/undo` removing a created file, esc interrupting a running tool (`[interrupted]`, queued steering returned to the editor), `/clear` starting a new session file. The gate found and fixed two real bugs (event-pipe deadlock on `/yolo`; styled prints mangled by `Sanitize`) — see the phase-3 plan's Implementation notes.
- **Live gates (2026-10-05, real OpenCode key)**: phase-1 smoke (11/11) and phase-2 gates stay green. Phase 3 live: minimax-m3 (anthropic-messages, thinking + tool calls) → `/model` → glm-5.3-flash (openai-completions) → continue → `/model` back → continue, all clean (no 400), thinking blocks stamped `opencode-go/minimax-m3` with signatures, 2 `model_change` entries, real usage/cost in the bar (`4.4k/154 · $0.0004` → `6.6k/248 · $0.0009`).
- **Still open**: direct `api.anthropic.com` / `api.openai.com` endpoints (no vendor keys — the zen override paths validate the same codecs); multi-turn thinking replay + prompt caching + compaction (phase 4); `glm --effort off` behavioral verification (wire-accepted, §5 note).

## 16. Maintenance

- Update this file at the end of every phase PR, and in any PR that changes implemented behavior (CLI contract, config keys, catalog, wire behavior, tools, permissions, sessions, agent loop). Keep it **normative for the current state**; note deviations from DESIGN.md explicitly (as §15 does).
- Keep DESIGN.md as the v1 vision; when the code settles a DESIGN ambiguity, record the decision here first and flag it for the next DESIGN revision.

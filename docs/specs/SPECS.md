# moca — SPECS.md (current implemented state)

> **Living document — keep in sync with the code.** This file describes **what is implemented today**, precisely enough for a human or an agent to work on the project without reading every source file. It is updated at the end of every phase (and any PR that changes implemented behavior).
>
> `docs/specs/DESIGN.md` remains the v1 vision/contract (rev 11). Where this file and DESIGN.md disagree, **this file wins for the current state** — the disagreement is then a to-do for the next DESIGN revision.
>
> Last updated: 2026-10-05 · phase 1 (PR #2, `phase/1-skeleton-providers`).

---

## 1. Overview

moca is a minimal, token-efficient, provider-agnostic coding agent. Go 1.27.1 (pinned via `mise.toml`), **stdlib-only** so far. Module `github.com/adeotek/moca`, binary `moca`.

**Implemented:** phase 1 — one-shot `moca -p` streaming through three hand-rolled protocol codecs (anthropic-messages, openai-completions, openai-responses), JSONC config, built-in model catalog, uniform retry, registry, exit-code contract.

**Not implemented yet:** tools/agent loop (phase 2), TUI (phase 3), context compaction/resume (phase 4), MCP (phase 5), rtk/graphify integration (phase 6), OAuth login (phase 7). `moca` without `-p` prints `TUI lands in phase 3; use -p` and exits 2; `login|logout|mcp` print `<cmd> lands in a later phase` and exit 2.

## 2. Build, test, verify

```bash
mise install                      # Go 1.27.1; `go` is not on the global PATH
export PATH="$(mise where go)/bin:$PATH"
go build -o bin/moca ./cmd/moca   # or: go build ./...
go test ./... -race
go vet ./... && gofmt -l .        # both must be clean
./bin/moca --version              # "moca 0.0.0-dev"
```

- Version stamp: `config.Version`, set via `-ldflags "-X github.com/adeotek/moca/internal/config.Version=…"`, default `"0.0.0-dev"`.
- Repo flow: `main` is PR-only (local pre-push hook blocks pushes to `refs/heads/main`); one branch + PR per phase (`phase/N-<slug>`) or chore (`chore/<slug>`); conventional commits; one version bump per PR (none yet — v0.1.0 at phase 7).
- Gate scripts for the phase-1 style mock checks are not in the repo; a mock SSE server + a config pointing `baseUrl` at it reproduces them (see §10).

### Package layout (implemented)

```
cmd/moca/          main.go (entry, signals), cli.go (flags → Options), oneshot.go (-p path)
internal/llm/      leaf types: Message, ContentBlock, ToolCall, ToolResult, Request, Usage, Event…
internal/config/   version, paths, JSONC pre-pass, typed config + validation, env indirection
internal/provider/ SSE reader, error taxonomy, catalog, 3 protocol codecs, replay helpers,
                   retry wrapper, registry
```

Dependency direction: `llm` imports nothing internal; `config` imports nothing internal; `provider` imports `llm` + `config`; `cmd/moca` imports all three. No SDKs; HTTP+SSE hand-rolled.

## 3. CLI surface (as implemented)

```
moca -p "<prompt>"            one-shot; prompt also on stdin (`-p -`, trailing newlines stripped)
  --model <provider/model>    override config model for this run
  --effort <level>            off|minimal|low|medium|high|xhigh|max (validated; clamped per model)
  --approve | --no-approve    parsed (mutually exclusive); takes effect in phase 2
  --yolo | --no-yolo          parsed; overrides config `yolo`; takes effect in phase 2
  --resume <id8|last>         parsed; takes effect in phase 4
  --continue                  parsed; takes effect in phase 4
  --version
  --config <path>             dev/test: config file override (hidden from help text)
moca login|logout|mcp …       stubs: "<cmd> lands in a later phase", exit 2
```

**`-p` output contract.** stdout = the final assistant message text only. Text is **buffered until the stream settles** (a mid-stream retry replays the whole response; nothing may reach a pipe twice) and discarded on `EventReset`. The buffer is written on success *and* on failure (last attempt's partial text). A trailing `\n` is appended on success. stderr = one-liners: retry notices (`retry N/M · <wait> (<err>)`), the error itself, the exit summary (`tokens <in+cache>/<out> · $<cost>`), and `length`/`refusal` diagnostics.

**Exit codes.** `0` completed · `1` provider/runtime error, or a `length`/`refusal` stop (`response truncated (token limit reached)` / `model refused to answer`; partial text still printed) · `2` config or usage error (incl. unset `env:` vars) · `3` reserved for maxSteps (phase 2) · `130` interrupted (SIGINT **and** SIGTERM cancel in-flight requests).

## 4. Config

- Path `~/.config/moca/config.jsonc` (`$XDG_CONFIG_HOME` honoured); data dir `~/.local/share/moca` (`$XDG_DATA_HOME`). Missing file → built-in defaults. Strict decode: **unknown keys are rejected**; errors carry `line:col`.
- **JSONC pre-pass** (`config.Standardize`): comments (`//`, `/* */`) are recognised only outside string literals (escapes honoured) and replaced by spaces — output has the **same length** as the input, so error offsets still map to the original line:col. A trailing comma (`,` followed only by whitespace/comments before `}`/`]`) is blanked. Unterminated `/*` → error. The §12 example of DESIGN.md is the byte-exact fixture (`internal/config/testdata/example.jsonc`).
- **Keys** (camelCase): `model`, `modelHard`, `providers.<name>` (`auth` = `api_key|oauth`, `apiKey` must be `"env:VAR"`, `baseUrl`, `baseUrls` per protocol, `protocol`, `models.<id>` with `protocol`/`contextWindow`/`maxOutputTokens`/`cost`), `shell.allow`, `mcp.idleTimeout` + `mcp.servers.<name>` (command|url, args, env, headers, description, approve), `snapshot.retentionDays` (0 = forever), `context.reserveTokens|keepRecentTokens|maxSteps`, `yolo`.
- **Defaults**: built-in providers `anthropic`/`opencode-go`/`openai` with `auth: api_key` and `env:ANTHROPIC_API_KEY` / `env:OPENCODE_API_KEY` / `env:OPENAI_API_KEY`; `shell.allow` = the 32-entry DESIGN §12 list; `mcp.idleTimeout` 600; `context` 16384/20000/40; `snapshot.retentionDays` 30; `yolo` false.
- **Validation**: model ids must be `provider/model` (split at first `/`); provider must exist; `apiKey` literal → rejected (must be `env:`); custom providers need `baseUrl`(+`baseUrls`) and `protocol`; model-level `protocol` must be one of the three; custom models must declare `contextWindow`; `contextWindow < 16384` refused; context values and `retentionDays >= 0` checked; each MCP server must have exactly one of `command`/`url`.
- **Env indirection**: `env:VAR` resolved lazily — an unset variable errors **when the first request to that provider is built** (naming the variable, `*config.EnvError` → exit 2), never at startup; unused providers don't block. `config.EnvRefs` lists every referenced variable (used by phase 2's shell hygiene).
- **Custom providers** (vLLM/LM Studio/Ollama…): any name + `baseUrl` + `protocol` + models with `contextWindow`; usable through the same registry path.

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
- `Resolve("provider/model")` → `(Model, Adapter)`; the adapter is already wrapped in the retry policy. `Models()` returns the catalog sorted (for phase 3's `/model`).

## 7. Protocol adapters (wire behavior)

All three: `post()` sends JSON with `Content-Type: application/json`, `Accept: text/event-stream`; **a 2xx whose Content-Type is set and not `text/event-stream` is an error** ("unexpected … response, want text/event-stream: <body≤8KiB>", non-retryable) instead of a fake turn. Response bodies are closed on every path; stall timeout 90 s.

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

## 9. `-p` one-shot semantics

Builds `llm.Request{Model: catalog id, System: "You are moca, a coding agent." (placeholder — real prompt in phase 2), MaxTokens: model-capped reserve, Effort: config default medium clamped (or `--effort` clamped), one user message}` → streams through the registry adapter → buffers text (§3) → prints text, exit summary, and stop diagnostics.

## 10. Verification status & known items

- Unit/integration: `go test ./... -race` — 80 pass events, includes adapter wire-shape pins, SSE framing variants, retry/backoff/retry-after/cancel, registry, CLI exit codes, `-p` buffer-reset and stop-diagnostic tests.
- Reproducible local gates: forced-429 backoff (mock server: two 429+`Retry-After: 1` → stderr shows `retry 1/5`, `retry 2/5`, stdout `ok`, exit 0); protocol e2e through the real binary against mock SSE servers for anthropic-messages + openai-responses + stdin prompt; §12 example decodes intact (`TestSpecExampleDecodesIntact`).
- **Unverified / open**: live provider smoke (needs real keys — checklist in PR #2); `glmThinking[off]="none"` against the live opencode-go endpoint (if rejected, clamp `off`→`low` as a catalog-row fix); the `Model` field stores the **bare** model id, so two providers serving the same id (e.g. `gpt-6-luna` on `openai` and `opencode-go`) are indistinguishable for verbatim thinking replay — pin this before phase 3's cross-provider switch gate.

## 11. Maintenance

- Update this file at the end of every phase PR, and in any PR that changes implemented behavior (CLI contract, config keys, catalog, wire behavior, retry, exit codes). Keep it **normative for the current state**; note deviations from DESIGN.md explicitly (as §10 does).
- Keep DESIGN.md as the v1 vision; when the code settles a DESIGN ambiguity, record the decision here first and flag it for the next DESIGN revision.

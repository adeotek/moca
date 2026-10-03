# moca DESIGN.md — v1 contract

Status: **REV 8 — post-review fixes + improvements applied, awaiting final lock**. Revision log at bottom. Once locked, this doc is the source of truth; changes require a new revision, not silent drift.

## Purpose

A minimal, token-efficient, provider-agnostic coding agent. No frameworks, no binary plugin systems — extensibility is skills (§9) + slash commands + shell allowlist + the lazy MCP proxy (§10.5). The bet: **a tight agent loop + disciplined context handling beats a feature-rich one** on cost-per-task for everyday coding (read → edit → run tests → commit).

## 1. Identity

- `moca` — MO Coding Agent. Binary name = repo name = module path (`github.com/adeotek/moca`).
- Tagline: *the coding agent you can read in an afternoon.*

## 2. Architecture

```
cmd/moca/            main: flag parsing, mode select (TUI | -p one-shot)
internal/
  agent/             the loop: user msg → model → tool calls → results → repeat
  provider/          protocol adapters + provider configs + model catalog + auth
                     (anthropic-messages, openai-completions, openai-responses)
  tools/             Tool interface + registry + the 7 built-ins
                     (read/write/edit/shell/search/ls/mcp)
  context/           token accounting, truncation, summarization, cache markers
  session/           JSONL append-only transcripts, resume
  tui/               Bubble Tea: input, scrollback, status bar, spinner, diff view
  config/            single JSONC config, typed, validated
  skills/            Agent Skills loader (SKILL.md) + slash-command templates
  permissions/       path jail, shell allowlist, project trust, per-tool gates
  mcp/               lazy MCP proxy (§10.5): stdio + streamable HTTP transports,
                     discovery index, server lifecycle
```

**Direction of dependencies: `agent` may import everything; `tools`, `provider`, `session`, `context`, `config`, `skills`, `permissions`, `mcp` import nothing from each other except `config` types and the `tools` Tool interface (the `mcp` proxy registers into the tool registry through it). `tui` imports `agent` (drives it), never the reverse.** No cycles, ever. This is the whole "minimal" promise made structural.

## 3. Provider layer — protocols × providers × auth

Three orthogonal concepts (Pi's proven architecture, verified against its shipped catalog):

- **Protocol adapters** — request/response codecs over hand-rolled HTTP+SSE (~150 LOC each): `anthropic-messages`, `openai-completions`, `openai-responses`. A model declares which protocol it speaks.
- **Providers** — config bindings: name + optional `baseUrl` override + auth + model catalog.
- **Auth modes** — `api_key` (env-var indirection only, never literals in config) or `oauth` (`moca login <provider>`; tokens at `~/.local/share/moca/auth.json`, mode 0600, auto-refresh). SSH/headless fallback: when the local callback is unreachable (SSH, containers, WSL), `moca login` prints the authorize URL for copying + accepts the pasted code (phase 7).

**Day-1 provider set (all three ship in v0.1):**

| Provider | Base URL | Auth | Notes |
|---|---|---|---|
| `anthropic` | api.anthropic.com | `ANTHROPIC_API_KEY` or Claude Pro/Max subscription OAuth | native anthropic-messages |
| `opencode-go` | opencode.ai/zen/go (anthropic-messages) · /zen/go/v1 (openai-completions/responses) | `OPENCODE_API_KEY` (Zen) | **mixed-protocol catalog** — verified: glm-5.3 ($1.4/$4.4, 1M ctx), glm-5.3-flash ($0.15/$0.5), minimax-m3, kimi-k3, gpt-6-luna, grok-4.7 |
| `openai` | api.openai.com (+ ChatGPT backend for subscription) | `OPENAI_API_KEY` or ChatGPT Plus/Pro subscription OAuth | openai-responses / openai-completions |

Claude/ChatGPT subscription OAuth endpoints are **not yet verified** — phase 6 verifies them against live flows before implementation.

- **Optional per-provider `baseUrl`** — override a built-in's default base URL, or declare a custom provider (any name) with `baseUrl` + `protocol` + auth: any OpenAI-protocol endpoint (vLLM, LM Studio, Ollama, OpenAI-compatible gateways) is usable day 1. opencode-go's dual base URLs (one per protocol family) are built in and overridable the same way.

- **Model ids are provider-qualified** (`opencode-go/glm-5.3-flash`) — unambiguous everywhere, including slash commands.
- **Built-in model catalog**: id, protocol, context window, cost per M (input/output/cacheRead/cacheWrite), **thinkingLevelMap** — powers the live cost line in the status bar, the token-estimate fallback, and effort validation. User-extensible in config.
- **Effort is a first-class request parameter** (`off|minimal|low|medium|high|xhigh|max`): mapped per protocol (anthropic thinking budget; openai `reasoning_effort`), validated per model against the catalog's `thinkingLevelMap`, default `medium` where the model supports it. Set via `/effort`, shown in the status bar (§11).
- **Cache breakpoints**: explicit positions in `Request` (anthropic `cache_control` native; openai protocols via prefix-stability discipline).
- No SDKs. Hand-rolled HTTP + SSE.

## 4. Tools — the seven, frozen for v1

| Tool | Semantics |
|------|-----------|
| `read` | path + optional offset/limit. Lines `N|content`-style with total count. Caps at 2000 lines/50K chars per call. |
| `write` | path + full content. Refuses without a prior `read` in session (anti-overwrite guard). |
| `edit` | path + old_string + new_string. **Exact match first; if no exact hit, one whitespace/indent-tolerant fallback pass; still nothing or >1 match → structured error, no write.** Returns unified-diff-style result, never the full file. |
| `shell` | command + timeout (default 30s, max 300s). **`bash -lc` on Unix, `pwsh -NoProfile -Command` on Windows (falls back to `powershell.exe -NoProfile -Command` when pwsh is absent).** Runs in workdir, jailed (§7). Output head+tail truncated at 30K chars with `[… N lines omitted]`. |
| `search` | ripgrep-semantics in pure Go (pattern + glob + path). Line-numbered matches, 200-hit cap, `files_only` mode. No external rg dependency. |
| `ls` | path, one level, dirs suffixed `/`, hidden opt-in. |
| `mcp` | **Lazy MCP proxy (see §10.5).** Fixed ~200-token schema: `{action: "search" \| "call", server, tool, args, query}`. MCP server tool lists are NEVER injected into the prompt — discovery happens inside the tool over a cached name+description index; full schema is fetched only when a call is made. |

Schema churn rule: these JSON schemas are frozen after phase 2. New tool = v2 discussion — every session re-reads schemas (token cost) and tool-list churn kills prompt-cache reuse.

`write` and `edit` store a **pre-edit snapshot** of the target file (hash-addressed, `~/.local/share/moca/snapshot/`) before writing — best-effort cheap undo when the workdir isn't a git repo (OpenCode ships the same mechanism; landed phase 2). **Skipped when the target file is tracked and clean in git** (git is the undo); best-effort prune at startup of snapshots older than 30 days (config `snapshot.retentionDays`, `0` = keep forever).

## 5. Edit-tool contract

1. `old_string` must be **unique** in the file. Ambiguous → error listing hit line numbers, no write.
2. Exact match fails → single fallback pass: match modulo leading-whitespace on each line. Exactly one hit → apply. Zero or >1 → error.
3. `new_string == old_string` → rejected (no-op).
4. Success returns diff + changed line numbers + new total.
5. Errors are written for the **model** to recover: what failed, where, what to do next ("re-read lines 40-60 and retry with more context").

## 6. Context manager

Budgets are **token-denominated** (config-overridable) — a percentage conflates a 200K and a 1M window; the catalog already knows the window.

- **Trigger**: `estimatedTokens > contextWindow − reserveTokens` (default `reserveTokens` **16K** — room left for the response). Checked after every completed tool batch and before every new user prompt.
- **keepRecentTokens** (default **20K**): recent tokens kept verbatim; everything older → summarized. Replaces "last N turns".
- **Cut-point rule: never cut between a tool call and its result** (results stay with their call). Valid cuts: user messages, assistant messages, complete tool batches. A single user-message span exceeding keepRecent cuts mid-span at an assistant message (split span → two summaries merged: prior history + span prefix).
- **Compaction entry**: `summary` + `firstKeptEntryId` + `tokensBefore` + `usage` (the summary's own cost counts toward session totals). Repeated compactions summarize from the previous compaction's kept boundary, so kept messages are never silently dropped from later summaries.
- **Summary request**: cheap `model`, minimal effort, **prompt-cache writes disabled** — a one-off prompt; polluting the cache is pure loss.
- **Structured summary format** (Pi's proved shape): Goal / Constraints & Preferences / Progress (done · in-progress · blocked) / Key Decisions / Next Steps / Critical Context + `<read-files>`/`<modified-files>` lists, tracked **cumulatively** across compactions.
- **Serialization for summarization**: conversation flattened to text (`[User]:` … `[Assistant thinking]:` … `[Assistant tool calls]: read(path=…)` … `[Tool result]:` …), **tool results truncated at 2K chars** — the summarizer doesn't need full outputs; labeled `COMPACTION NOTE (lossy)`.
- **Overflow recovery**: provider context-overflow error or early `stop_reason: length` → **one compact-and-retry attempt**; retry runs as a fresh turn. Recovery failure surfaces the original error.
- **Token estimate** (trigger + status-bar usage %): chars/4 — a deliberate ASCII-leaning default; the catalog may carry per-model calibration later. Response `usage` stays the source of truth for session totals.
- `read` always returns line numbers + totals so re-reads target windows, never whole files.

Token efficiency = prompt caching (§3) + windowed reads + diff-shaped results + truncation + compaction. NOT stripped tool descriptions — descriptive schemas up front are cheaper than failed calls.

## 7. Permissions

- **Path jail**: **anchored at session-start workdir** (canonical absolute path); `read`/`write`/`edit` refuse outside; `~` expansion only inside the jail. cwd may move within the jail (`shell` cd), every check re-anchored to the startup root — a mid-session `cd` never moves the jail. Windows: normalized slash comparison.
- **shell**: deny-by-allowlist, first-token match, platform-agnostic list (Unix-only entries like `sed`/`awk` simply never fire on Windows). **Compound commands: every `|`/`;`/`&&`/`||` segment's first token is checked against the allowlist; the hard-deny set is checked in every segment.** Not allowlisted → refusal naming the prefix to ask about (TUI: one-key allow/deny). `rm`, `sudo`, `dd`, `shred`, and piping into them: **hard no in v1, no override**.
- Default allowlist (§12) includes the external tools (`rtk`, `graphify`) — see §10.
- **Project trust**: loading `<workdir>/.moca/` resources (skills, prompts) requires a trust decision for that directory — a cloned repo can plant them; that's prompt injection plus a slash-command surface. TUI: one-time prompt on first open of an untrusted workdir. One-shot `-p`: `--approve` / `--no-approve` flags, default **no** (project resources skipped). Saved decisions in `~/.local/share/moca/trust.json` (canonical paths). Global resources (`~/.config/moca/skills/`) are always trusted (user-installed by definition).
- **maxSteps** (config, default **40**): max agentic iterations per run; at the limit the model gets one final request instructing it to wrap up with text only (forced summarization of progress + remaining work). Runaway protection — a confused model looping `read` calls is the #1 token burner.
- One-shot `-p` mode: workdir-only, no interactive escalation; refusals are transcript errors. **Exit summary**: no status bar in `-p` — session tokens in/out + total cost print to stderr on exit.

## 8. Sessions

- `~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl`, append-only. **Every entry has an `id` + `parentId`** — linear chain in v1; the ids make a session tree a v2 extension, not a schema change (Pi's proven shape). Entry types: `session` (first entry: workdir, provider, model, effort, startedAt — powers `--continue`'s workdir→session mapping), `message` (user/assistant, **thinking blocks persisted**), `tool_use` + `tool_result`, `compaction`, `model_change` (`{model, effort}` — `/model` and `/effort` both land here; resume restores both), `error`.
- `compaction` entry: `summary`, `firstKeptEntryId`, `tokensBefore`, `usage` (§6). Request context rebuilds from the transcript: latest compaction summary + entries from `firstKeptEntryId` onward.
- Resume: `moca --resume <id8>` / `--resume last` / **`--continue`** (most recent session in this workdir).
- `usage` on every assistant turn → cumulative cost line live in the status bar.

## 9. Skills

- Agent Skills standard, same shape as Pi/Claude Code/OpenCode: `<workdir>/.moca/skills/<name>/SKILL.md` + `~/.config/moca/skills/` (the latter always trusted; the former requires project trust, §7).
- Frontmatter: `name`, `description`, optional `argument-hint`. Discovery = name + description in the system prompt only; body loaded on explicit request. Token-cheap by construction.
- **Body-load trigger**: the system prompt's skill list includes each skill's **absolute path**; the model loads the body with the plain `read` tool when it decides it's relevant. No dedicated `skill` tool, no new mechanism — ecosystem-compatible and phase-2-cheap by construction.
- **Ecosystem compatibility is a hard requirement**: any SKILL.md written for pi/claude-code/opencode (including graphify's, including pi packages) must load unchanged. Tested in phase 5.

## 10. External tools & extensibility — day 1

No binary plugin system in v1 (probably ever). Three composable, standards-based mechanisms:

1. **Skills** (§9) — the primary extension surface; ecosystem-compatible by design.
2. **Slash commands** — `~/.config/moca/prompts/<name>.md` invoked as `/name` (pi-compatible shape, optional `argument-hint`).
3. **Shell allowlist** (§7) — any CLI is integrable by allowlisting it; a skill teaches the model to use it well.

**Worked examples — must work in v0.1:**

- **rtk** (token-compressed CLI proxy: `rtk read/ls/git/diff/test/docker/gh/…`) — in the default allowlist; ships a built-in `rtk` skill teaching preferences. Natural extension of moca's mission: the system prompt's token-discipline section says *prefer rtk-prefixed variants where they exist*.
- **graphify** (codebase-graph CLI) — in the default allowlist; its SKILL.md is drop-in via the standard loader. `graphify install --platform moca` needs a one-line platform entry upstream — contribute it in phase 6; until merged, manual copy into the skills dir works identically.

### 10.5 MCP — lazy by design (rev 3)

MCP support ships in v1 **because Ben uses MCP servers daily** (OpenCode + Pi via `pi-mcp-adapter`, which validated this exact architecture). The token-lean approach, learned from pi-mcp-adapter's measured results (100 installed servers → 0 running at session start, ~200 tokens of prompt cost, vs 7 GB RAM / 10k+ tokens eager):

- **One fixed `mcp` proxy tool, ~200-token schema** — `{action: "search"|"call", server, tool, args, query}`. Server tool lists are NEVER injected into the prompt. This is what makes MCP compatible with moca's mission: the eager approach (inject every server's schemas) is the single fastest way to destroy a lean context.
- **Lazy lifecycle**: configured servers start on first call, stop after 10 idle minutes (config: `mcp.idleTimeout`, seconds). A stopped server keeps its tools discoverable (cached index); it restarts on next call (~0.1–0.3s local). No eager spawning at startup, ever.
- **Discovery index**: at first use (not startup), moca reads a server's tool list once, caches `{server, tool, name, description}`; `action: "search"` ranks over that index (word match in v1). Full input schema is fetched only when the model actually calls the tool.
- **Config** (same file, one block per server under `mcp.servers` — no user-keyed-map namespace collision: scalars like `idleTimeout` sit beside `servers`, not inside it; pi-mcp-adapter/Claude-Code-compatible server fields, JSONC shape per §12):

```jsonc
"mcp": {
  "idleTimeout": 600,                          // seconds, lazy stop
  "servers": {
    "context7":   { "url": "https://mcp.context7.com/mcp" },   // streamable HTTP
    "filesystem": { "command": "npx",                            // stdio
                     "args": ["-y", "@modelcontextprotocol/server-filesystem", "."] }
  }
}
```

- **HTTP server auth**: optional `headers` per server; values are env-indirection strings (`"Authorization": "env:SOME_VAR"`), consistent with the keys-never-literal rule (§3, §12). stdio servers never see more than the filtered env (below).

- **Transports**: stdio + streamable HTTP in v1. Legacy SSE and sampling/elicitation: no (v2 discussion).
- **Env hygiene**: stdio subprocesses get a filtered env (PATH, HOME, USER, LANG, TERM, TMPDIR, XDG_*) + explicit `env` entries only — same rule as Hermes's native MCP client (no accidental credential leakage to servers).
- **Path jail does NOT apply to MCP tools** — a server's own tool defines its scope (a GitHub server talks to GitHub). `approveTools`-style gating for risky MCP tools: v1 refuses calls to tools whose names match `create|delete|drop|remove|push|deploy` unless allowlisted per server in config (`mcp.servers.<name>.approve = ["create_issue"]`) or the user approves in TUI (one-key). One-shot `-p` mode: allowlist only, no interactive approval.
- **Import**: `moca mcp import` reads existing Claude Code / OpenCode / Pi MCP configs on the machine and writes them into moca's config (mechanical translation, previewed before writing). Day-1 convenience so your existing servers work immediately.

## 11. TUI

- Bubble Tea + lipgloss. Compact/dense per house style: tight padding, no banners.
- Layout: scrollback (fills) · input box · **status bar (bottom, below the input area, 1 line, from day 1)**.
- **Input area is a real multi-line text area** (cursor, wrapping, vertical scroll within the box):
  - **Multi-line copy/paste works natively.** Paste: bracketed-paste mode detects pastes and inserts the full content verbatim — never truncated, never auto-sent. A paste >50 lines renders as a collapsed `[paste N lines]` display chip in the input view (`alt+p` toggles) while the full content stays intact in the buffer. Copy: inline rendering (no alt-screen) and no mouse capture in v1 — native terminal selection + copy works over the whole scrollback; OSC 52 additionally writes selected text to the clipboard (works over SSH).
  - **`shift+enter` = new line** (primary). Technical note: plain terminals send identical bytes for Enter and Shift+Enter — distinguishing them requires enhanced keyboard reporting (Kitty keyboard protocol or xterm `modifyOtherKeys`). Phase 3 pins the Bubble Tea version with that support enabled and degrades gracefully: `alt+enter` / `ctrl+j` fallback bindings plus a one-time status hint when the terminal can't report it.
  - `enter` sends · `↑/↓` input history (at buffer edges) · `esc` interrupt · `ctrl+c` twice quit · `/` prefix = commands: `/model`, `/effort`, `/hard`, `/clear`, `/compact`, `/cost`.
- **Status bar — one line, always current, contains exactly:** `cwd` (home-abbreviated, e.g. `~/projects/moca`) · `branch` (git, dirty marker `*`, non-git = dimmed `-`) · `provider/model` + **effort** (e.g. `opencode-go/glm-5.3-flash · med`) · **context size and usage %** (e.g. `ctx 1M · 23%`) · **session tokens in/out** (e.g. `24k/6k`) · **session cost** (e.g. `$0.0412`), live per response.
  - cwd and branch update on every turn end (they can change mid-session via tools).
  - context usage % = estimated tokens of the next request against the catalog's context window; recalculated after every tool result and response.
  - session tokens/cost accumulate from `usage` in responses — cache reads/writes itemized in `/cost` detail, not the bar.
  - updates are event-driven, not polled; during streaming only the spinner changes.
- Streaming renders inline; tool calls render as collapsible one-liners (`▸ edit main.go [+3 −1]`); `v` opens the diff pager.
- **Thinking blocks render collapsed** as a one-liner (`⋯ thinking N lines`), toggle to expand — every catalog model emits reasoning; it is persisted in the session (§8) and serialized into compaction (§6), never shown expanded by default.
- **Input during a run**: a message sent while the agent works is **steering** — it enters after the current assistant turn completes (before the next tool batch). `esc` aborts the run; queued steering messages return to the editor. The input box is never dead.
- **`!` prefix** runs a shell command from the input box; output enters the conversation as a user message (`!!` runs it locally without model visibility). `!`/`!!` are user-initiated (the user's own hands, like any terminal) — they bypass the model-facing allowlist; the shell tool's own rules are unchanged.

## 12. Config

`~/.config/moca/config.jsonc` — single file, typed struct, fail-fast validation. **JSONC over TOML (rev 6):** stdlib-parseable (comment-strip pre-parse, Go `encoding/json` decodes the result — zero third-party config deps), and it matches the house dotfiles sync machinery (`merge-opencode-config.py` / `merge-pi-config.py` semantics: recursive object merge, live-only keys kept, ordered array union, scalar template-wins / `--live-wins` flip, `.bak` backup, byte-identical-if-unchanged so comments survive). JSONC is also the native shape of the OpenCode ecosystem configs `moca mcp import` reads — one parser everywhere.

**Comment stripping is string-literal-aware**: `//` and `/* */` are comments only outside quoted strings — the example below contains URLs with `//`, and a naive first-`//`-cut parser destroys them. Phase-1 parser test: the §12 example must round-trip byte-identical (values intact, comments stripped). Block comments `/* */` are stripped too; no trailing-comma forgiveness — `encoding/json` accepts them natively in slice/map literals.

```jsonc
{
  "defaultProvider": "opencode-go",
  "model":      "opencode-go/glm-5.3-flash",   // everyday turns — cheap, fast
  "modelHard":  "opencode-go/glm-5.3",          // /hard escalation: planning, gnarly debug

  "providers": {
    "anthropic":   { "auth": "api_key", "apiKeyEnv": "ANTHROPIC_API_KEY" },  // or "oauth" → moca login anthropic (Claude Pro/Max)
    "opencode-go": { "auth": "api_key", "apiKeyEnv": "OPENCODE_API_KEY" },   // OpenCode Zen subscription key
    "openai":      { "auth": "api_key", "apiKeyEnv": "OPENAI_API_KEY" },     // or "oauth" → moca login openai (ChatGPT Plus/Pro)
    // optional baseUrl override per provider; custom providers too, e.g.:
    // "vllm": { "baseUrl": "http://localhost:8000/v1", "protocol": "openai-responses", "auth": "api_key", "apiKeyEnv": "VLLM_API_KEY" }
  },

  "shell": {
    "allow": ["go", "git", "gh", "grep", "rg", "find", "ls", "cat", "head", "tail",
              "mkdir", "sed", "awk", "curl", "mise", "python", "pytest",
              "node", "npm", "docker", "kubectl", "terraform", "ansible",
              "rtk", "graphify"]
  },

  "mcp": {
    "idleTimeout": 600,                                        // seconds, lazy stop
    "servers": {
      "context7": { "url": "https://mcp.context7.com/mcp" }     // lazy: starts on first call, not at startup
    }
  },

  "snapshot": { "retentionDays": 30 },   // 0 = keep forever

  "context": { "reserveTokens": 16384, "keepRecentTokens": 20000, "maxSteps": 40 }
}
```

**Model routing:** `/effort` sets the effort level (validated against the catalog); `/hard` = switch to `modelHard` + `/effort high`; compaction summaries always run on the cheap `model` at minimal effort. No auto-escalation heuristics in v1 — explicit is cheaper to debug than clever. Keys never appear as literals — env indirection or OAuth token store only.

## 13. System prompt

~40 lines, frozen at phase 2: role, workdir, platform (`uname -a` / Windows equivalent), git state, tool contract summary, token discipline (incl. the rtk preference line), version stamp at the end (cache invalidates visibly when it changes). **Built once at session start, never rebuilt mid-session** — a rebuilt prompt embeds fresh git state and invalidates the prompt cache on every commit the agent itself makes (self-sabotage on the core edit→test→commit loop). Git state may go stale; the status bar is the live source. The version stamp changes only with the moca version or a config change.

## 14. Ship gate (v0.1 demo)

One session, real repo, unattended — **on the `opencode-go` provider** (proves multi-provider day 1):
1. `read` a failing test,
2. locate the bug with `search`,
3. `edit` the fix,
4. run `go test ./...` (or `rtk test -- go test ./...`) via shell — green,
5. commit on a feature branch,
6. the built-in `rtk` skill was discoverable and its guidance followed.

## 15. Explicit non-goals (v1)

subagents · hooks · plan mode · LSP · web browsing · image gen · voice · telemetry · **binary plugin/extension system** (skills + commands + allowlist + the §10.5 lazy `mcp` proxy are the extension model — a fifth mechanism needs a design revision) · `serve` mode · MCP sampling/elicitation/legacy-SSE (v2 discussion).

## Phase plan (each = one PR, reviewed)

1. **Skeleton + protocol adapters + streaming + JSONC config** — anthropic-messages + openai-completions codecs, api-key auth, string-literal-aware JSONC parser (§12). Gate: `moca -p 'hi'` streams **via anthropic AND opencode-go**; the §12 example round-trips byte-identical (URLs intact, comments stripped).
2. **Seven tools + agent loop + permissions + skills loader + session schema** — path jail (session-start anchored), allowlist (segment-checked), project trust, maxSteps, and the session schema (entry ids + types incl. `compaction` placeholder + `session` first entry, thinking persisted) land *with* the tools (no ungated phase); `mcp` tool ships as a stub returning "no servers configured" (schema frozen from day 1 — no churn later); pre-edit snapshots. Gates: edit-ladder 10/10 green, jail enforced, untrusted workdir's project skills NOT loaded (trust prompt works), maxSteps forces wrap-up, a skill discovered (name + description + absolute path in the system prompt).
3. **TUI shell + slash commands + status bar** — status bar (bottom, below input) from the first frame: cwd · branch · provider/model · effort · context size & usage % · session tokens · session cost. Thinking blocks render collapsed; steering input during runs; `!`/`!!` prefix (user-initiated, allowlist-bypassing). Gate: full session in TUI; `/model`, `/effort`, `/hard` work; bar reflects a `shell`-tool branch change at the next turn; a multi-line paste inserts verbatim and sends intact; `shift+enter` newline verified on an enhanced-keyboard terminal and fallback confirmed on one without; a steering message queued mid-run lands after the current assistant turn.
4. **Context manager + compaction + resume** — token budgets (reserve 16K / keepRecent 20K), cut-point rules, structured summaries with cumulative file tracking, overflow compact-and-retry, no-cache-write on summary requests; resume + `--continue` (session-entry workdir mapping, §8). Gate: long synthetic session crosses the token threshold and compacts (context shrinks, no tool result orphaned from its call); provider overflow triggers one compact-and-retry; resume + `--continue` rebuild request context from the transcript incl. compaction entries.
5. **MCP lazy proxy** — stdio + streamable HTTP transports, discovery index, lazy lifecycle, `moca mcp import`. Gates: a real server (e.g. context7 or filesystem) callable via the proxy with no server tool schemas in the prompt (verified by inspecting the request payload); server stopped after idle timeout; import converts an existing Claude-Code/OpenCode/Pi config.
6. **rtk + model_hard routing + graphify compatibility** — Gates: rtk-wrapped commands preferred in a real session; a graphify/pi SKILL.md loads unchanged.
7. **OAuth providers + upstream graphify PR + v0.1** — Claude subscription + ChatGPT subscription login (endpoints verified live first; SSH/headless fallback: copy-URL + paste-code when the local callback is unreachable), `graphify install --platform moca` upstream, polish, tag `v0.1.0` when §14 passes.

**Edit-ladder tests (phase 2, non-negotiable):** exact unique · exact ambiguous · whitespace-fallback hit · fallback ambiguous · no match · no-op new==old · CRLF file · unicode file · 50-line span · trailing-newline-missing file.

## Revision log

- **rev 8 (2026-10-03, Ben approved full review — fixes + improvements):** consistency pass + under-specification fixes from the rev-7 design review: Purpose/§2/README resynced to rev-3+ reality (7 tools, `mcp/` package in the tree, MCP in the extension model, README status/phases/non-goals rewritten) · §10.5 + §12 config blocks converted TOML→JSONC; MCP config restructured under `mcp.servers` (no user-keyed-map namespace collision: `mcp.idleTimeout` scalar beside `servers`, `mcp.servers.<name>.approve` gating) · JSONC comment stripping specified **string-literal-aware** (URLs with `//` survive; §12-example round-trip = phase-1 gate) · session schema: `model_change` carries `{model, effort}`; new `session` first entry (workdir, provider, model, effort, startedAt) powers `--continue`'s workdir mapping · skills body-load trigger specified (absolute path in the system prompt; body loaded via plain `read` — no new mechanism) · path jail **anchored at session-start workdir** (mid-session `cd` never moves the jail) · `!`/`!!` user-initiated, bypass the model-facing allowlist · compound shell commands: every `|`/`;`/`&&`/`||` segment allowlist-checked, hard-deny set checked in all segments · system prompt built once per session, never rebuilt (git state may go stale — prompt-cache discipline) · token estimate = chars/4 (catalog calibration later) · `-p` exit summary (tokens in/out + cost to stderr) · snapshots skipped for clean tracked files + 30-day prune (`snapshot.retentionDays`) · `pwsh`→`powershell.exe` fallback · optional per-provider `baseUrl` + custom providers (vLLM/LM Studio/Ollama day 1) · MCP HTTP `headers` with env indirection · `gh` in the default allowlist · `--continue` ownership moved to phase 4. *Addendum:* OAuth `moca login` SSH/headless fallback — copy-URL + paste-code when the local callback is unreachable (§3, phase 7).
- **rev 7 (2026-10-03, Ben approved design review):** compaction rewritten token-denominated (reserveTokens 16K / keepRecentTokens 20K, cut-point rules — never orphan a tool result, overflow compact-and-retry, no-cache-write on summary, structured summary + cumulative file tracking, 2K truncation in serialization) · session schema: entry ids + types incl. `compaction` (+ `--continue`) · thinking blocks persisted/rendered-collapsed · steering input during runs · project trust (`--approve`/`--no-approve`, trust.json) · maxSteps 40 runaway guard · `!` prefix · pre-edit snapshots. Sources: Pi's shipped compaction/security/usage docs, OpenCode's agents docs + on-disk data layout (snapshot/, tool-output/).
- **rev 6 (2026-10-03, Ben's question → adopted):** config format TOML → **JSONC** (`~/.config/moca/config.jsonc`, camelCase keys) — stdlib-parseable (comment-strip pre-parse + `encoding/json`, zero config deps), matches the house dotfiles merge machinery (merge-opencode-config.py / merge-pi-config.py semantics: recursive object merge, live-only keys kept, ordered array union, template-wins/`--live-wins`, `.bak`, byte-identical-if-unchanged), native shape of OpenCode-ecosystem configs `moca mcp import` reads.
- **rev 5 (2026-10-03, Ben's steering):** status bar moved to the **bottom, below the input area** · input area is a real multi-line text area with native multi-line paste (bracketed-paste, verbatim, never auto-sent; >50-line pastes render collapsed, buffer intact; native copy via inline rendering + OSC 52) · `shift+enter` = newline (enhanced keyboard reporting — Kitty protocol/xterm modifyOtherKeys — with `alt+enter`/`ctrl+j` fallbacks + one-time hint on plain terminals).
- **rev 4 (2026-10-03, Ben's steering):** status bar from day 1 with the exact field set: cwd · branch (dirty marker) · provider/model · **effort** · context size + usage % · session tokens in/out · session cost — effort promoted to a first-class request parameter (`/effort`, protocol-mapped, catalog-validated, default medium); `/hard` = model_hard + effort high; layout inverted (bar on top); context-usage estimate and per-response usage accounting pulled into phase 3.
- **rev 3 (2026-10-03, Ben's steering):** MCP support in v1, lazy by design — one fixed ~200-token `mcp` proxy tool (§10.5), server tool lists never enter the prompt, lazy server lifecycle (first-call start, 10-min idle stop), discovery index, stdio + streamable HTTP, filtered subprocess env, risky-tool gating, `moca mcp import`; tool count six → seven (schema frozen at phase 2 as a stub so no churn); phase plan six → seven phases (MCP gets its own PR).
- **rev 2 (2026-10-03, Ben's steering):** `bash` → `shell` (PowerShell on Windows) · `model` + `model_hard` from day 1 · three day-1 providers (OpenCode Go / Claude / OpenAI, incl. subscription OAuth) replacing the anthropic+openai-compat pair · external tools (rtk, graphify) from day 1 via skills/commands/allowlist · phase plan restructured (permissions pulled into phase 2, OAuth + upstream graphify PR in phase 6).
- **rev 1 (2026-10-03):** initial draft.

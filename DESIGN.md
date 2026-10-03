# moca DESIGN.md — v1 contract

Status: **REV 4 — steering applied, awaiting final lock**. Revision log at bottom. Once locked, this doc is the source of truth; changes require a new revision, not silent drift.

## Purpose

A minimal, token-efficient, provider-agnostic coding agent. No frameworks, no binary plugin systems, no MCP. The bet: **a tight agent loop + disciplined context handling beats a feature-rich one** on cost-per-task for everyday coding (read → edit → run tests → commit).

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
  tools/             Tool interface + registry + the 6 built-ins
  context/           token accounting, truncation, summarization, cache markers
  session/           JSONL append-only transcripts, resume
  tui/               Bubble Tea: input, scrollback, status bar, spinner, diff view
  config/            single TOML config, typed, validated
  skills/            Agent Skills loader (SKILL.md) + slash-command templates
  permissions/       path jail, shell allowlist, per-tool gates
```

**Direction of dependencies: `agent` may import everything; `tools`, `provider`, `session`, `context`, `config`, `skills`, `permissions` import nothing from each other except `config` types. `tui` imports `agent` (drives it), never the reverse.** No cycles, ever. This is the whole "minimal" promise made structural.

## 3. Provider layer — protocols × providers × auth

Three orthogonal concepts (Pi's proven architecture, verified against its shipped catalog):

- **Protocol adapters** — request/response codecs over hand-rolled HTTP+SSE (~150 LOC each): `anthropic-messages`, `openai-completions`, `openai-responses`. A model declares which protocol it speaks.
- **Providers** — config bindings: name + base URL + auth + model catalog.
- **Auth modes** — `api_key` (env-var indirection only, never literals in config) or `oauth` (`moca login <provider>`; tokens at `~/.local/share/moca/auth.json`, mode 0600, auto-refresh).

**Day-1 provider set (all three ship in v0.1):**

| Provider | Base URL | Auth | Notes |
|---|---|---|---|
| `anthropic` | api.anthropic.com | `ANTHROPIC_API_KEY` or Claude Pro/Max subscription OAuth | native anthropic-messages |
| `opencode-go` | opencode.ai/zen/go (anthropic-messages) · /zen/go/v1 (openai-completions/responses) | `OPENCODE_API_KEY` (Zen) | **mixed-protocol catalog** — verified: glm-5.3 ($1.4/$4.4, 1M ctx), glm-5.3-flash ($0.15/$0.5), minimax-m3, kimi-k3, gpt-6-luna, grok-4.7 |
| `openai` | api.openai.com (+ ChatGPT backend for subscription) | `OPENAI_API_KEY` or ChatGPT Plus/Pro subscription OAuth | openai-responses / openai-completions |

Claude/ChatGPT subscription OAuth endpoints are **not yet verified** — phase 6 verifies them against live flows before implementation.

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
| `shell` | command + timeout (default 30s, max 300s). **`bash -lc` on Unix, `pwsh -NoProfile -Command` on Windows.** Runs in workdir, jailed (§7). Output head+tail truncated at 30K chars with `[… N lines omitted]`. |
| `search` | ripgrep-semantics in pure Go (pattern + glob + path). Line-numbered matches, 200-hit cap, `files_only` mode. No external rg dependency. |
| `ls` | path, one level, dirs suffixed `/`, hidden opt-in. |
| `mcp` | **Lazy MCP proxy (see §10.5).** Fixed ~200-token schema: `{action: "search" \| "call", server, tool, args, query}`. MCP server tool lists are NEVER injected into the prompt — discovery happens inside the tool over a cached name+description index; full schema is fetched only when a call is made. |

Schema churn rule: these JSON schemas are frozen after phase 2. New tool = v2 discussion — every session re-reads schemas (token cost) and tool-list churn kills prompt-cache reuse.

## 5. Edit-tool contract

1. `old_string` must be **unique** in the file. Ambiguous → error listing hit line numbers, no write.
2. Exact match fails → single fallback pass: match modulo leading-whitespace on each line. Exactly one hit → apply. Zero or >1 → error.
3. `new_string == old_string` → rejected (no-op).
4. Success returns diff + changed line numbers + new total.
5. Errors are written for the **model** to recover: what failed, where, what to do next ("re-read lines 40-60 and retry with more context").

## 6. Context manager

Budgets (config-overridable): context window from catalog (fallback 200K); **compaction at 80%** — keep system prompt + last 4 turns verbatim, older turns → one summary request to the **default (cheap) model**, labeled `COMPACTION NOTE (lossy)`. Tool output caps per §4. `read` always returns line numbers + totals so re-reads target windows, never whole files.

Token efficiency = prompt caching (§3) + windowed reads + diff-shaped results + truncation + compaction. NOT stripped tool descriptions — descriptive schemas up front are cheaper than failed calls.

## 7. Permissions

- **Path jail**: workdir and children. `read`/`write`/`edit` refuse outside; `~` expansion only inside the jail. Windows: normalized slash comparison.
- **shell**: deny-by-allowlist, first-token match, platform-agnostic list (Unix-only entries like `sed`/`awk` simply never fire on Windows). Not allowlisted → refusal naming the prefix to ask about (TUI: one-key allow/deny). `rm`, `sudo`, `dd`, `shred`, and piping into them: **hard no in v1, no override**.
- Default allowlist (§12) includes the external tools (`rtk`, `graphify`) — see §10.
- One-shot `-p` mode: workdir-only, no interactive escalation; refusals are transcript errors.

## 8. Sessions

- `~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl`, append-only, one object per line: `{t, role, content, tool_use, tool_result, usage, model}`.
- Resume: `moca --resume <id8>` / `--resume last`. Tool schemas and compaction state rebuild from transcript.
- `usage` on every assistant turn → cumulative cost line live in the status bar.

## 9. Skills

- Agent Skills standard, same shape as Pi/Claude Code/OpenCode: `<workdir>/.moca/skills/<name>/SKILL.md` + `~/.config/moca/skills/`.
- Frontmatter: `name`, `description`, optional `argument-hint`. Discovery = name+description in the system prompt only; body loaded on explicit request. Token-cheap by construction.
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
- **Lazy lifecycle**: configured servers start on first call, stop after 10 idle minutes (config: `mcp.idle_timeout`). A stopped server keeps its tools discoverable (cached index); it restarts on next call (~0.1–0.3s local). No eager spawning at startup, ever.
- **Discovery index**: at first use (not startup), moca reads a server's tool list once, caches `{server, tool, name, description}`; `action: "search"` ranks over that index (word match in v1). Full input schema is fetched only when the model actually calls the tool.
- **Config** (same file, one block per server — pi-mcp-adapter/Claude-Code-compatible shape):

```toml
[mcp.context7]
url = "https://mcp.context7.com/mcp"     # streamable HTTP

[mcp.filesystem]
command = "npx"                            # stdio
args = ["-y", "@modelcontextprotocol/server-filesystem", "."]
```

- **Transports**: stdio + streamable HTTP in v1. Legacy SSE and sampling/elicitation: no (v2 discussion).
- **Env hygiene**: stdio subprocesses get a filtered env (PATH, HOME, USER, LANG, TERM, TMPDIR, XDG_*) + explicit `env` entries only — same rule as Hermes's native MCP client (no accidental credential leakage to servers).
- **Path jail does NOT apply to MCP tools** — a server's own tool defines its scope (a GitHub server talks to GitHub). `approveTools`-style gating for risky MCP tools: v1 refuses calls to tools whose names match `create|delete|drop|remove|push|deploy` unless allowlisted per server in config (`mcp.<name>.approve = ["create_issue"]`) or the user approves in TUI (one-key). One-shot `-p` mode: allowlist only, no interactive approval.
- **Import**: `moca mcp import` reads existing Claude Code / OpenCode / Pi MCP configs on the machine and writes them into moca's config (mechanical translation, previewed before writing). Day-1 convenience so your existing servers work immediately.

## 11. TUI

- Bubble Tea + lipgloss. Compact/dense per house style: tight padding, no banners.
- Layout: **status bar (top, 1 line, from day 1)** · scrollback (fills) · input box (bottom).
- **Status bar — one line, always current, contains exactly:** `cwd` (home-abbreviated, e.g. `~/projects/moca`) · `branch` (git, dirty marker `*`, non-git = dimmed `-`) · `provider/model` + **effort** (e.g. `opencode-go/glm-5.3-flash · med`) · **context size and usage %** (e.g. `ctx 1M · 23%`) · **session tokens in/out** (e.g. `24k/6k`) · **session cost** (e.g. `$0.0412`), live per response.
  - cwd and branch update on every turn end (they can change mid-session via tools).
  - context usage % = estimated tokens of the next request against the catalog's context window; recalculated after every tool result and response.
  - session tokens/cost accumulate from `usage` in responses — cache reads/writes itemized in `/cost` detail, not the bar.
  - updates are event-driven, not polled; during streaming only the spinner changes.
- Streaming renders inline; tool calls render as collapsible one-liners (`▸ edit main.go [+3 −1]`); `v` opens the diff pager.
- Keys: `enter` send · `alt+enter` newline · `↑/↓` history · `esc` interrupt · `ctrl+c` twice quit · `/` commands: `/model`, `/effort`, `/hard`, `/clear`, `/compact`, `/cost`.
- Paste >8 lines → `[paste N lines]` chip, `alt+p` expands.

## 12. Config

`~/.config/moca/config.toml` — single file, typed struct, fail-fast validation:

```toml
default_provider = "opencode-go"
model      = "opencode-go/glm-5.3-flash"   # everyday turns — cheap, fast
model_hard = "opencode-go/glm-5.3"         # /hard escalation: planning, gnarly debug

[providers.anthropic]
auth        = "api_key"      # or "oauth" → moca login anthropic (Claude Pro/Max)
api_key_env = "ANTHROPIC_API_KEY"

[providers.opencode-go]
auth        = "api_key"      # OpenCode Zen subscription key
api_key_env = "OPENCODE_API_KEY"

[providers.openai]
auth        = "api_key"      # or "oauth" → moca login openai (ChatGPT Plus/Pro)
api_key_env = "OPENAI_API_KEY"

[shell]
allow = ["go", "git", "grep", "rg", "find", "ls", "cat", "head", "tail",
         "mkdir", "sed", "awk", "curl", "mise", "python", "pytest",
         "node", "npm", "docker", "kubectl", "terraform", "ansible",
         "rtk", "graphify"]

[mcp.context7]                           # lazy: starts on first call, not at startup
url = "https://mcp.context7.com/mcp"

[context]
compact_at_pct = 80
keep_turns     = 4
```

**Model routing:** `/effort` sets the effort level (validated against the catalog); `/hard` = `/model model_hard` + `/effort high`; compaction summaries always run on the cheap `model` at minimal effort. No auto-escalation heuristics in v1 — explicit is cheaper to debug than clever. Keys never appear as literals — env indirection or OAuth token store only.

## 13. System prompt

~40 lines, frozen at phase 2: role, workdir, platform (`uname -a` / Windows equivalent), git state, tool contract summary, token discipline (incl. the rtk preference line), version stamp at the end (cache invalidates visibly when it changes).

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

1. **Skeleton + protocol adapters + streaming** — anthropic-messages + openai-completions codecs, api-key auth. Gate: `moca -p 'hi'` streams **via anthropic AND opencode-go**.
2. **Seven tools + agent loop + permissions + skills loader** — path jail and allowlist land *with* the tools (no ungated phase); `mcp` tool ships as a stub returning "no servers configured" (schema frozen from day 1 — no churn later). Gates: edit-ladder 10/10 green, jail enforced, a skill discovered.
3. **TUI shell + slash commands + status bar** — status bar from the first frame: cwd · branch · provider/model · effort · context size & usage % · session tokens · session cost. Gate: full session in TUI; `/model`, `/effort`, `/hard` work; bar reflects a `shell`-tool branch change at the next turn.
4. **Context manager + sessions** — Gate: compaction holds at 80%, resume works, live cost line.
5. **MCP lazy proxy** — stdio + streamable HTTP transports, discovery index, lazy lifecycle, `moca mcp import`. Gates: a real server (e.g. context7 or filesystem) callable via the proxy with no server tool schemas in the prompt (verified by inspecting the request payload); server stopped after idle timeout; import converts an existing Claude-Code/OpenCode/Pi config.
6. **rtk + model_hard routing + graphify compatibility** — Gates: rtk-wrapped commands preferred in a real session; a graphify/pi SKILL.md loads unchanged.
7. **OAuth providers + upstream graphify PR + v0.1** — Claude subscription + ChatGPT subscription login (endpoints verified live first), `graphify install --platform moca` upstream, polish, tag `v0.1.0` when §14 passes.

**Edit-ladder tests (phase 2, non-negotiable):** exact unique · exact ambiguous · whitespace-fallback hit · fallback ambiguous · no match · no-op new==old · CRLF file · unicode file · 50-line span · trailing-newline-missing file.

## Revision log

- **rev 4 (2026-10-03, Ben's steering):** status bar from day 1 with the exact field set: cwd · branch (dirty marker) · provider/model · **effort** · context size + usage % · session tokens in/out · session cost — effort promoted to a first-class request parameter (`/effort`, protocol-mapped, catalog-validated, default medium); `/hard` = model_hard + effort high; layout inverted (bar on top); context-usage estimate and per-response usage accounting pulled into phase 3.
- **rev 3 (2026-10-03, Ben's steering):** MCP support in v1, lazy by design — one fixed ~200-token `mcp` proxy tool (§10.5), server tool lists never enter the prompt, lazy server lifecycle (first-call start, 10-min idle stop), discovery index, stdio + streamable HTTP, filtered subprocess env, risky-tool gating, `moca mcp import`; tool count six → seven (schema frozen at phase 2 as a stub so no churn); phase plan six → seven phases (MCP gets its own PR).
- **rev 2 (2026-10-03, Ben's steering):** `bash` → `shell` (PowerShell on Windows) · `model` + `model_hard` from day 1 · three day-1 providers (OpenCode Go / Claude / OpenAI, incl. subscription OAuth) replacing the anthropic+openai-compat pair · external tools (rtk, graphify) from day 1 via skills/commands/allowlist · phase plan restructured (permissions pulled into phase 2, OAuth + upstream graphify PR in phase 6).
- **rev 1 (2026-10-03):** initial draft.

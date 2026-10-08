# moca DESIGN.md — v1 contract

Status: **REV 11 — plan review fixes, awaiting final lock**. Revision log at bottom. Once locked, this doc is the source of truth; changes require a new revision, not silent drift.

## Purpose

A minimal, token-efficient, provider-agnostic coding agent. No frameworks, no binary plugin systems — extensibility is skills (§9) + slash commands + shell allowlist + the lazy MCP proxy (§10.5). The bet: **a tight agent loop + disciplined context handling beats a feature-rich one** on cost-per-task for everyday coding (read → edit → run tests → commit).

## 1. Identity

- `moca` — MO Coding Agent. Binary name = repo name = module path (`github.com/adeotek/moca`).
- Tagline: *the coding agent you can read in an afternoon.*

## 2. Architecture

```
cmd/moca/            main: CLI surface (§12.5), mode select (TUI | -p one-shot)
internal/
  llm/               leaf: Message, ContentBlock, ToolCall, Usage, Request types — imports nothing internal
  agent/             the loop: user msg → model → tool calls → results → repeat
  provider/          protocol adapters + provider configs + model catalog + auth + retry
                     (anthropic-messages, openai-completions, openai-responses)
  tools/             Tool + PathChecker interfaces, registry, the built-ins
                     (read/write/edit/shell/search/ls; `mcp` is implemented in mcp/)
  compact/           token accounting, truncation, summarization, cache markers
  session/           JSONL append-only transcripts, resume, snapshots/undo
  tui/               Bubble Tea: input, scrollback, status bar, spinner, pager
  config/            single JSONC config, typed, validated
  skills/            Agent Skills loader (SKILL.md) + slash-command templates + AGENTS.md loader
  permissions/       path jail, shell command analysis + allowlist, project trust
  mcp/               lazy MCP proxy (§10.5): stdio + streamable HTTP transports,
                     discovery index, server lifecycle; registers the `mcp` tool
```

**Direction of dependencies: every package may import `llm` and `config`. `agent` may import everything. `provider`, `tools`, `compact`, `session`, `skills`, `permissions`, `mcp` import nothing from each other, with two named exceptions: `mcp` imports `tools` (to register through the Tool interface) and `permissions` implements interfaces declared in `tools` without importing it (Go structural typing). Cross-package behaviour is injected by `agent`: `compact` receives a `Summarize func(llm.Request) (llm.Message, error)` rather than importing `provider`; `tools` receives a `PathChecker`/`CommandChecker` rather than importing `permissions`. `tui` imports `agent` (drives it, consumes its event stream), never the reverse.** No cycles, ever. This is the whole "minimal" promise made structural. (`compact`, not `context` — never shadow the stdlib package every file needs.)

## 3. Provider layer — protocols × providers × auth

Three orthogonal concepts (Pi's proven architecture, verified against its shipped catalog):

- **Protocol adapters** — request/response codecs over hand-rolled HTTP+SSE (~150 LOC each): `anthropic-messages`, `openai-completions`, `openai-responses`. A model declares which protocol it speaks. All three land in phase 1.
- **Providers** — config bindings: name + optional `baseUrl` override + auth + model catalog.
- **Auth modes** — `api_key` (env indirection only: `"apiKey": "env:VAR"` — the one indirection syntax used everywhere in config, §10.5, §12) or `oauth` (`moca login <provider>`; tokens at `~/.local/share/moca/auth.json`, mode 0600, auto-refresh). SSH/headless fallback: when the local callback is unreachable (SSH, containers, WSL), `moca login` prints the authorize URL for copying + accepts the pasted code (phase 7).

**Day-1 provider set (all three ship in v0.1):**

| Provider | Base URL | Auth | Notes |
|---|---|---|---|
| `anthropic` | api.anthropic.com | `ANTHROPIC_API_KEY` — subscription OAuth **not permitted** (rev 12) | native anthropic-messages |
| `opencode-go` | opencode.ai/zen/go (anthropic-messages) · /zen/go/v1 (openai-completions/responses) | `OPENCODE_API_KEY` (Zen) | **mixed-protocol catalog** — verified: glm-5.3 ($1.4/$4.4, 1M ctx), glm-5.3-flash ($0.15/$0.5), minimax-m3, kimi-k3, gpt-6-luna, grok-4.7 |
| `openai` | api.openai.com (subscription: Responses API only) | `OPENAI_API_KEY` or ChatGPT Plus/Pro subscription OAuth — Sign in with ChatGPT open-source token sharing (rev 12) | openai-responses / openai-completions |

Subscription OAuth was verified live **before** implementation (phase 7, 2026-10-06 — `docs/specs/oauth-verification.md`): **`anthropic` ships `api_key` only** — Anthropic does not permit third-party developers to offer Claude.ai login or route Free/Pro/Max plan credentials; the OAuth option is dropped and config validation rejects `"auth": "oauth"` for it with the recorded reason. **`openai` ships OAuth** through the documented "Sign in with ChatGPT" open-source flow (dynamic client registration — no client secret; rotating refresh tokens, serialized across processes; Responses API only, `store:false`/`stream:true`; function tools grouped in a namespace); `api_key` remains supported for both. No workarounds, no spoofed client identities.

- **Optional per-provider `baseUrl`** — override a built-in's default base URL, or declare a custom provider (any name) with `baseUrl` + `protocol` + auth: any OpenAI-protocol endpoint (vLLM, LM Studio, Ollama, OpenAI-compatible gateways) is usable day 1. opencode-go's dual base URLs (one per protocol family) are built in and overridable the same way. Custom-provider models declare `contextWindow` in config (validated against §6's minimum).
- **Model ids are provider-qualified** (`opencode-go/glm-5.3-flash`) — unambiguous everywhere, including slash commands. (Hence no `defaultProvider` key: the model id names it.)
- **Built-in model catalog**: id, protocol, context window, max output tokens, cost per M (input/output/cacheRead/cacheWrite), **thinkingMode** (`none|budget|adaptive|openai`) + **thinkingLevelMap** — powers the live cost line in the status bar, the token-estimate fallback, and effort validation. User-extensible in config.
- **Effort is a first-class request parameter** (`off|minimal|low|medium|high|xhigh|max`): mapped per model via the catalog's `thinkingMode` + `thinkingLevelMap` — anthropic `adaptive` models (current Claude generation): `thinking: {type: "adaptive", display: "summarized"}` + `output_config.effort`; anthropic `budget` models (older, e.g. Haiku 4.5): `thinking: {type: "enabled", budget_tokens}`; openai protocols: `reasoning_effort`. Validated per model, default `medium` where the model supports it. **Unsupported level → clamp to the nearest supported level at or below it, or the lowest supported level if none is below** (e.g. `off` on a model whose thinking cannot be disabled → its lowest effort; status bar shows the effective level). Set via `/effort`, shown in the status bar (§11).
- **`max_tokens`** per request = catalog max output for the model, capped at `reserveTokens` (§6); for anthropic `budget`-mode models `budget_tokens` is clamped to `max_tokens − 4K` (min 1024) — the API rejects `budget ≥ max_tokens`. `adaptive` models take no budget (sending one is a 400).
- **Cache breakpoints**: explicit positions in `Request` — anthropic `cache_control` on (1) the end of tools+system prefix and (2) the last message, rolled forward each turn (≤4 breakpoints, the API's cap); openai protocols via prefix-stability discipline.
- **Retry policy** (in `provider`, uniform across adapters): 429 / 5xx / anthropic 529 / connection reset before the first streamed byte → exponential backoff with jitter (1s, 2s, 4s, 8s, 16s; 5 attempts), honouring `retry-after`; status bar shows `retry 2/5 · 4s`; `esc` cancels. SSE **stall timeout**: no bytes for 90s → abort that attempt and retry. **Mid-stream failure** (after content was streamed): no silent retry — the partial assistant message is discarded from the request context, an `error` entry is logged, and the turn is retried once from the same request; a second failure surfaces to the user. 4xx other than 429/overflow → no retry, surfaced.
- **Cross-provider history transform** (applied when building every request, so `/model` and `/hard` can switch model, provider or protocol mid-session): thinking blocks are replayed unchanged (text + signature/encrypted payload) **only to the exact model that produced them** — current Claude models bind thinking to the producing model, and editing replayed history invalidates it, so the transcript-to-request path is append-only for same-model turns; thinking from any other model is converted to a plain-text block labelled `[prior reasoning]` (empty-text thinking — the default `omitted` display — is dropped); tool-call ids are normalized to `[a-zA-Z0-9_-]{1,40}` with a stable per-session mapping. The transcript keeps the originals. Switching model forfeits the prompt cache — documented in `/model` and `/hard` output.
- No SDKs. Hand-rolled HTTP + SSE.

## 4. Tools — the seven, frozen for v1

| Tool | Semantics |
|------|-----------|
| `read` | path + optional offset/limit. Lines `N|content`-style with total count. Caps at 2000 lines/50K chars per call; **individual lines truncated at 2000 chars** (`[… line truncated]`, minified files). Binary files (NUL byte in first 8K) → refused with size + MIME guess. |
| `write` | path + full content. **New files: allowed. Existing files: refuses without a prior `read` of that file in this session, and refuses if the file changed on disk since that read** (mtime + size + hash recorded at read time) → error telling the model to re-read. |
| `edit` | path + old_string + new_string + optional `replace_all` (bool, default false). **Exact match first; if no exact hit, one whitespace/indent-tolerant fallback pass; still nothing or (without `replace_all`) >1 match → structured error, no write.** Same prior-read + unchanged-on-disk guard as `write`. Returns unified-diff-style result, never the full file. Full contract §5. |
| `shell` | command + timeout (default 30s, max 300s). **`bash -c` on Unix (non-login — no profile sourcing per call), `pwsh -NoProfile -Command` on Windows (falls back to `powershell.exe -NoProfile -Command` when pwsh is absent).** **Stateless: every call starts at the jail root** — `cd sub && make` works within one call; nothing carries over between calls. stdin = `/dev/null`; env adds `PAGER=cat GIT_PAGER=cat GIT_EDITOR=true GIT_TERMINAL_PROMPT=0` and removes every variable referenced by an `env:` indirection in config (provider keys never reach the model's shell). Runs in its own process group; timeout or `esc` kills the whole group. Commands are analysed before running (§7) — **not a sandbox**. Output head+tail truncated at 30K chars with `[… N lines omitted]`. |
| `search` | ripgrep-semantics in pure Go (pattern + glob + path). **Respects `.gitignore` / `.ignore` and skips hidden + binary files by default** (rg defaults). Regex is Go RE2 syntax (no lookaround/backrefs — stated in the tool description). Line-numbered matches, 200-hit cap, `files_only` mode. No external rg dependency. |
| `ls` | path, one level, dirs suffixed `/`, hidden opt-in. |
| `mcp` | **Lazy MCP proxy (see §10.5).** Fixed ~200-token schema: `{action: "search" \| "describe" \| "call", server, tool, args, query}`. MCP server tool lists are NEVER injected into the prompt — discovery happens inside the tool over a cached name+description index; `describe` returns one tool's full input schema; `call` executes. |

Schema churn rule: these JSON schemas are frozen after phase 2. New tool or parameter = v2 discussion — every session re-reads schemas (token cost) and tool-list churn kills prompt-cache reuse.

**Parallel tool calls**: a model turn may emit several tool calls; they execute **sequentially in emitted order** (deterministic transcripts, one permission prompt at a time). A refused or failed call does not cancel the rest; each gets its own result.

**Pre-edit snapshots + `/undo`**: `write` and `edit` store a **pre-edit snapshot** of the target file (hash-addressed, `~/.local/share/moca/snapshot/`, plus a per-session undo stack in the transcript; new-file creation records "did not exist") before writing. **Skipped when the target file is tracked and clean in git** (git is the undo — `/undo` says so and prints the `git restore` command). `/undo` (TUI) reverts the most recent snapshotted `write`/`edit` of this session (repeatable, stack order), refusing if the file changed since moca wrote it; changes made via `shell` are never covered. Best-effort prune at startup of snapshots older than 30 days (config `snapshot.retentionDays`, `0` = keep forever).

## 5. Edit-tool contract

1. `old_string` must be **non-empty** (file creation is `write`'s job) and **unique** in the file unless `replace_all: true`. Ambiguous without `replace_all` → error listing hit line numbers, no write. `replace_all` replaces every exact match and reports the count.
2. Exact match fails → single fallback pass: match modulo leading whitespace on each line (trailing whitespace ignored too). Exactly one hit → apply **with re-indentation**: compute the indentation delta between `old_string`'s first non-blank line and the matched file line, apply that delta to every line of `new_string` (tabs/spaces preserved as the file uses them). Zero or >1 hits → error. `replace_all` never uses the fallback.
3. `new_string == old_string` → rejected (no-op).
4. **Line endings & encoding**: matching runs on an LF-normalized view; the write restores the file's dominant line ending (CRLF stays CRLF). A UTF-8 BOM is preserved and never matchable. Non-UTF-8 files → refused.
5. Success returns diff + changed line numbers + new total.
6. Errors are written for the **model** to recover: what failed, where, what to do next ("re-read lines 40-60 and retry with more context").

## 6. Context manager

Budgets are **token-denominated** (config-overridable) — a percentage conflates a 200K and a 1M window; the catalog already knows the window.

- **Trigger**: `estimatedTokens > contextWindow − reserve` (default `reserveTokens` **16K** — room left for the response). Checked after every completed tool batch and before every new user prompt.
- **keepRecentTokens** (default **20K**): recent tokens kept verbatim; everything older → summarized. Replaces "last N turns".
- **Small windows** (local models via `baseUrl`): effective `reserve = min(reserveTokens, window/4)`, effective `keepRecent = min(keepRecentTokens, window/4)`. Config validation **refuses models with `contextWindow < 16K`**. Loop guard: if a compaction leaves the context still above the trigger, moca stops with an error naming the oversized entries instead of compacting again.
- **Cut-point rule: never cut between a tool call and its result** (results stay with their call). Valid cuts: user messages, assistant messages, complete tool batches. A single user-message span exceeding keepRecent cuts mid-span at an assistant message (split span → two summaries merged: prior history + span prefix).
- **Compaction entry**: `summary` + `firstKeptEntryId` + `tokensBefore` + `usage` (the summary's own cost counts toward session totals). Repeated compactions summarize from the previous compaction's kept boundary, so kept messages are never silently dropped from later summaries.
- **Summary request**: cheap `model`, effort `minimal` (clamped per §3), **prompt-cache writes disabled** — a one-off prompt; polluting the cache is pure loss. Summary capped at 4K output tokens.
- **Structured summary format** (Pi's proven shape): Goal / Constraints & Preferences / Progress (done · in-progress · blocked) / Key Decisions / Next Steps / Critical Context + `<read-files>`/`<modified-files>` lists, tracked **cumulatively** across compactions.
- **Serialization for summarization**: conversation flattened to text (`[User]:` … `[Assistant thinking]:` … `[Assistant tool calls]: read(path=…)` … `[Tool result]:` …), **tool results truncated at 2K chars** — the summarizer doesn't need full outputs; labeled `COMPACTION NOTE (lossy)`.
- **Overflow recovery**: provider context-overflow error, or `stop_reason: length` **when the request's input tokens were within `reserve` of the window** → **one compact-and-retry attempt**; retry runs as a fresh turn. Recovery failure surfaces the original error. A plain `length` stop with room left is an output cap, not overflow: the text is kept, a truncated tool call becomes an error result telling the model to split the work.
- **Token estimate** (trigger + status-bar usage %): **last response's `usage` input+output tokens + chars/4 of everything appended since** (falls back to chars/4 of the whole request before the first response, or after a model switch). Exact up to the delta, so code/CJK skew stays small. Response `usage` stays the source of truth for session totals.
- **maxSteps wrap-up** (§7) is sent with `tool_choice: none` (anthropic `none`; openai `"none"`), never by removing tools — the tool list is part of the cached prefix.
- `read` always returns line numbers + totals so re-reads target windows, never whole files.

Token efficiency = prompt caching (§3) + windowed reads + diff-shaped results + truncation + compaction. NOT stripped tool descriptions — descriptive schemas up front are cheaper than failed calls.

## 7. Permissions

**Threat model, stated plainly:** permissions guard against model mistakes and casual prompt injection; they are **not a sandbox**. `read`/`write`/`edit` are confined; `shell` is analysed, not contained — an allowlisted interpreter (`python`, `node`, `npm` scripts, `make`) can do anything the user can. OS-level sandboxing (bubblewrap/Landlock, `sandbox-exec`) is a v2 discussion.

- **Path jail** (`read`/`write`/`edit`/`ls`/`search`): **anchored at session-start workdir** (canonical absolute path, symlinks resolved). Every target is resolved with symlinks evaluated (`filepath.EvalSymlinks` on the deepest existing ancestor for new files) before the check — a symlink inside the workdir pointing outside is refused. `~` expands to `$HOME` as usual; the result is jail-checked like any path. Windows: normalized slash + case-insensitive comparison.
- **Read-only roots** (outside the jail, `read`/`ls`/`search` only, never write): the global skills dir `~/.config/moca/skills/`, the extracted built-in skills dir (§9), and — when trusted — `<workdir>/.moca/skills/` (already inside the jail). This is what lets §9's body-load-by-`read` work for global and built-in skills.
- **shell — command analysis (Unix)**: the command is parsed with `mvdan.cc/sh/v3/syntax` (moca's one non-UI third-party dependency, pure Go; a hand-rolled splitter cannot see `$(…)`, backticks, subshells, `&`, newlines or redirects). Rules:
  - **Every simple command anywhere in the tree** — pipelines, `;`/`&&`/`||`/`&`/newline lists, subshells, `{ }` groups, `$(…)` / backtick substitutions, process substitution — has its command name checked. Leading `VAR=val` assignments are skipped; wrappers `env`, `time`, `timeout`, `nice`, `nohup`, `command` are unwrapped and the wrapped command is checked.
  - Shell builtins `cd`, `pwd`, `echo`, `printf`, `test`, `[`, `true`, `false`, `exit` are implicitly allowed. `eval`, `source`/`.`, `exec`, and a non-literal command name (`$CMD args`) → refused.
  - **Output redirections** (`>`, `>>`, `&>`, `tee` args): literal targets must pass the path jail (`/dev/null` allowed); non-literal targets → refused.
  - Unparseable input → refused (the error says so — no best-effort guessing).
  - Not allowlisted → refusal naming the command to ask about (TUI: one-key allow-once / allow-always / deny; allow-always appends to config `shell.allow`).
  - **Hard-deny, no override**: `sudo`, `su`, `doas`, `dd`, `shred`, `mkfs*`, `chown`. **`rm` is ask-every-time**: never allowlistable, one-key approval per call in the TUI, refused in `-p`. (A blanket `rm` ban only pushes the model to `find -delete` or `python -c`, which are worse.)
- **shell on Windows**: no PowerShell parser in v1 — best-effort first-token check per `;`/`|`/`&&`/`||` segment, plus the hard-deny/ask set extended with `Remove-Item`, `rm`, `ri`, `del`, `erase`, `rd`, `rmdir`, `Format-Volume`. Documented as weaker than Unix.
- Default allowlist (§12) includes the external tools (`rtk`, `graphify`) — see §10.
- **Project trust**: loading `<workdir>/.moca/` resources (skills, prompts) and `<workdir>/AGENTS.md`/`CLAUDE.md` (§9.5) requires a trust decision for that directory — a cloned repo can plant them; that's prompt injection plus a slash-command surface. TUI: one-time prompt on first open of an untrusted workdir. One-shot `-p`: `--approve` / `--no-approve` flags, default **no** (project resources skipped). Saved decisions in `~/.local/share/moca/trust.json` (canonical paths). Global resources (`~/.config/moca/`) and built-in skills are always trusted.
- **maxSteps** (config, default **40**): max agentic iterations per run; at the limit the model gets one final request instructing it to wrap up with text only (forced summarization of progress + remaining work; sent with `tool_choice: none`, §6). Runaway protection — a confused model looping `read` calls is the #1 token burner.
- One-shot `-p` mode: workdir-only, no interactive escalation; refusals are transcript errors. Output contract in §12.5.
- Everything in this section except `maxSteps` can be switched off wholesale by **yolo mode** (§7.5).

### 7.5 Yolo mode

An explicit opt-in that turns **every permission check off**. It is for sandboxed or throwaway environments (containers, VMs, CI) and for users who accept that the model can do anything they can.

- **Activation**: `--yolo` for one run (TUI or `-p`); `"yolo": true` in config makes it the default, and `--no-yolo` overrides that for one run. `/yolo` in the TUI toggles it mid-session — **between runs only** (refused while a run is in progress; swapping checkers under a running agent is not thread-safe).
- **What turns off**: the path jail (read/write/edit/ls/search accept any path; relative paths still resolve against the workdir, `~` still expands); shell command analysis entirely (no allowlist, no hard-deny list, no `rm` prompt, no redirect checks, no refusal of `eval`/`$CMD`/unparseable input — the command runs as given); MCP call gating (every tool runs); approval prompts (none are shown); project trust (project resources load as trusted, unless `--no-approve` is given explicitly — an explicit flag wins).
- **What stays on** (not permissions): `maxSteps` runaway protection; the `write`/`edit` read-before-write and stale-on-disk guards (they prevent clobbering, not access); pre-edit snapshots + `/undo`; shell statelessness, timeout, process-group kill, output truncation; stripping config-referenced secrets from the shell env; session file modes.
- **Visibility**: the status bar shows a red `YOLO` field first while on (§11); turning it on prints `yolo mode: all permission checks are off` (TUI scrollback, or stderr in `-p`). The `session` entry records `yolo` at start, and every toggle writes a `permission_mode` entry (`{yolo}`) — an audit trail only: **resume never restores yolo**; it comes from the flag/config of the resuming process.

## 8. Sessions

- `~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl`, append-only; files mode **0600**, directory 0700 (tool output can contain secrets). `id8` = 8 random hex chars, regenerated on collision. **Every entry has an `id` + `parentId`** — linear chain in v1; the ids make a session tree a v2 extension, not a schema change (Pi's proven shape). Entry types:
  - `session` (first entry: workdir, provider, model, effort, yolo, startedAt, mocaVersion, **the full built system prompt** — resume reuses it verbatim, so resumed requests hit the same cache prefix and are reproducible; powers `--continue`'s workdir→session mapping),
  - `message` (user/assistant, **thinking blocks persisted** with their originating model id for §3's transform),
  - `tool_use` + `tool_result` (each `tool_use` entry carries the assistant message id it belongs to; results appended in call order),
  - `compaction`, `model_change` (`{model, effort}` — `/model`, `/effort`, `/hard` all land here; resume restores both), `permission_mode` (`{yolo}` — audit only, §7.5), `snapshot` (undo stack, §4), `error`.
- `compaction` entry: `summary`, `firstKeptEntryId`, `tokensBefore`, `usage` (§6). Request context rebuilds from the transcript: system prompt from the `session` entry + latest compaction summary + entries from `firstKeptEntryId` onward.
- **Transcript repair** (abort and crash): `esc` during a tool, or a resumed session whose last `tool_use` has no `tool_result`, gets a synthetic `tool_result` (`"aborted by user"` / `"interrupted — moca exited before completion"`) appended before the next request. Every provider rejects a tool call without a result; the transcript is always valid to resend.
- Resume: `moca --resume <id8>` / `--resume last` / **`--continue`** (most recent session in this workdir).
- `/clear` starts a **new session file** (the old one stays resumable); it is not an in-file marker.
- `usage` on every assistant turn → cumulative cost line live in the status bar.

## 9. Skills

- Agent Skills standard, same shape as Pi/Claude Code/OpenCode. Three sources, in precedence order (same name → the earlier wins):
  1. `<workdir>/.moca/skills/<name>/SKILL.md` — requires project trust (§7),
  2. `~/.config/moca/skills/<name>/SKILL.md` — always trusted,
  3. **built-in** skills (`rtk`), embedded in the binary (`embed.FS`) and extracted at startup to `~/.local/share/moca/builtin-skills/<mocaVersion>/` so they have real paths.
- Frontmatter: `name`, `description`, optional `argument-hint`. Discovery = name + description in the system prompt only; body loaded on explicit request. Token-cheap by construction.
- **Body-load trigger**: the system prompt's skill list includes each skill's **absolute path**; the model loads the body with the plain `read` tool when it decides it's relevant (global and built-in skill dirs are read-only roots, §7). No dedicated `skill` tool, no new mechanism — ecosystem-compatible and phase-2-cheap by construction.
- **Ecosystem compatibility is a hard requirement**: any SKILL.md written for pi/claude-code/opencode (including graphify's, including pi packages) must load unchanged. Tested in phase 6.

### 9.5 Project instructions (AGENTS.md)

- Loaded at session start into the system prompt (§13), in order: `~/.config/moca/AGENTS.md` (always), then `<workdir>/AGENTS.md` — or, if absent, `<workdir>/CLAUDE.md` — **only when the workdir is trusted** (§7).
- Each file capped at 32K chars (truncated with a visible note). Nested per-directory AGENTS.md files: v2 discussion.
- Part of the frozen system prompt: edits mid-session take effect in the next session.

## 10. External tools & extensibility — day 1

No binary plugin system in v1 (probably ever). Three composable, standards-based mechanisms:

1. **Skills** (§9) — the primary extension surface; ecosystem-compatible by design.
2. **Slash commands** — `~/.config/moca/prompts/<name>.md` (+ `<workdir>/.moca/prompts/` when trusted) invoked as `/name` (pi-compatible shape, optional `argument-hint`). Built-in command names win on collision.
3. **Shell allowlist** (§7) — any CLI is integrable by allowlisting it; a skill teaches the model to use it well.

**Worked examples — must work in v0.1:**

- **rtk** (token-compressed CLI proxy: `rtk read/ls/git/diff/test/docker/gh/…`) — in the default allowlist; ships a built-in `rtk` skill teaching preferences (§9 source 3). Natural extension of moca's mission: the system prompt's token-discipline section says *prefer rtk-prefixed variants where they exist*. The command analyser (§7) unwraps `rtk <cmd>` like `env`/`time`, so `rtk` never launders a non-allowlisted command.
- **graphify** (codebase-graph CLI) — in the default allowlist; its SKILL.md is drop-in via the standard loader. `graphify install --platform moca` needs a one-line platform entry upstream — contribute it in phase 7; until merged, manual copy into the skills dir works identically.

### 10.5 MCP — lazy by design (rev 3)

MCP support ships in v1 **because Ben uses MCP servers daily** (OpenCode + Pi via `pi-mcp-adapter`, which validated this exact architecture). The token-lean approach, learned from pi-mcp-adapter's measured results (100 installed servers → 0 running at session start, ~200 tokens of prompt cost, vs 7 GB RAM / 10k+ tokens eager):

- **One fixed `mcp` proxy tool, ~200-token schema** — `{action: "search"|"describe"|"call", server, tool, args, query}`. Server tool lists are NEVER injected into the prompt. This is what makes MCP compatible with moca's mission: the eager approach (inject every server's schemas) is the single fastest way to destroy a lean context.
  - `search` (`query`, optional `server`) → ranked `{server, tool, description}` hits from the index (word match in v1).
  - `describe` (`server`, `tool`) → that tool's full input schema + annotations (starts the server if needed).
  - `call` (`server`, `tool`, `args`) → executes; result text truncated at 30K chars like `shell`; non-text content (images, audio, embedded resources) replaced with `[<type> omitted]`.
- **Server roster in the system prompt**: one line per configured server — name + optional config `description` (~10 tokens each). The model knows what exists without any tool schemas.
- **Lazy lifecycle**: configured servers start on first `describe`/`call` (or an index miss, below), stop after 10 idle minutes (config: `mcp.idleTimeout`, seconds). A stopped server keeps its tools discoverable (persisted index); it restarts on next call (~0.1–0.3s local). No eager spawning at startup, ever.
- **Discovery index, persisted**: `~/.local/share/moca/mcp-index.json`, `{server, configHash, tools:[{name, description, annotations}]}`. Refreshed every time a server starts (its tool list is re-read then — free, it's running anyway) and invalidated when that server's config hash changes. A `search` over servers with no valid index entry starts only those servers to index them (one-time cost, then persisted); `moca mcp index` prebuilds the whole index from the CLI. This persistence is what makes "0 servers running at session start" hold across sessions.
- **Config** (same file, one block per server under `mcp.servers` — no user-keyed-map namespace collision: scalars like `idleTimeout` sit beside `servers`, not inside it; pi-mcp-adapter/Claude-Code-compatible server fields, JSONC shape per §12):

```jsonc
"mcp": {
  "idleTimeout": 600,                          // seconds, lazy stop
  "servers": {
    "context7":   { "url": "https://mcp.context7.com/mcp",       // streamable HTTP
                    "description": "library/API docs lookup" },
    "filesystem": { "command": "npx",                            // stdio
                    "args": ["-y", "@modelcontextprotocol/server-filesystem", "."] }
  }
}
```

- **HTTP server auth**: optional `headers` per server; values are literal or env-indirection strings (`"Authorization": "env:SOME_VAR"`), consistent with the secrets-never-literal rule (§3, §12). MCP OAuth (remote servers requiring an authorization flow): v2 discussion.
- **Transports**: stdio + streamable HTTP in v1. Legacy SSE and sampling/elicitation: no (v2 discussion).
- **Env hygiene**: stdio subprocesses get a filtered env (PATH, HOME, USER, LANG, TERM, TMPDIR, XDG_*) + explicit `env` entries only (values literal or `env:VAR`) — same rule as Hermes's native MCP client (no accidental credential leakage to servers).
- **Path jail does NOT apply to MCP tools** — a server's own tool defines its scope (a GitHub server talks to GitHub; a filesystem server reaches whatever its args grant).
- **Call gating — annotation-based, default-deny**: a `call` runs without asking only if the tool's MCP annotations say `readOnlyHint: true` (and not `destructiveHint: true`), **or** the tool is allowlisted per server (`mcp.servers.<name>.approve = ["create_issue"]`, `["*"]` = trust the whole server). Otherwise: TUI one-key approval (allow-once / allow-always → appends to `approve`); `-p` refuses. Name-pattern heuristics are not used — they miss `write_file`, `send_*`, `update_*`, `exec`, `merge`.
- **Import**: `moca mcp import` reads existing Claude Code / OpenCode / Pi MCP configs on the machine and writes them into moca's config (mechanical translation, previewed before writing). **Literal secrets are never copied**: `env`/`headers` values whose key matches `KEY|TOKEN|SECRET|PASSWORD|AUTH` (case-insensitive) or that look like bearer tokens are rewritten to `env:MOCA_MCP_<SERVER>_<KEY>` and the preview lists the variables to export. Day-1 convenience so your existing servers work immediately.

## 11. TUI

- Bubble Tea + lipgloss. Compact/dense per house style: tight padding, no banner boxes — a session opens with two compact welcome lines at the top of the scrollback: `moca <version>` and `How can I help you today?` (rev 13).
- Layout: scrollback (fills) · full-width dim rule · input box (3 rows, no prompt prefix) · full-width dim rule · **status bar (bottom, below the input area; 2 lines since rev 13, 1 line from day 1)**.
- **Rendering model: inline (no alt-screen), committed scrollback is immutable.** Finished output is printed into the terminal's own scrollback and never redrawn; only the live region (streaming response, input box, status bar) re-renders. Consequence, by design: "expanding" an item never rewrites history — it opens the **pager** (below).
- **Input area is a real multi-line text area** (cursor, wrapping, vertical scroll within the box; no prompt prefix/suffix — each line renders the text alone, rev 13):
  - **Multi-line copy/paste works natively.** Paste: bracketed-paste mode detects pastes and inserts the full content verbatim — never truncated, never auto-sent. A paste >50 lines renders as a collapsed `[paste N lines]` display chip in the input view (`alt+p` toggles) while the full content stays intact in the buffer. Copy: no mouse capture in v1 — native terminal selection + copy works over the whole scrollback; **`/copy`** writes the last assistant message to the clipboard via OSC 52 (works over SSH).
  - **`shift+enter` = new line** (primary). Technical note: plain terminals send identical bytes for Enter and Shift+Enter — distinguishing them requires enhanced keyboard reporting (Kitty keyboard protocol or xterm `modifyOtherKeys`). Phase 3 pins the Bubble Tea version with that support enabled and degrades gracefully: `alt+enter` / `ctrl+j` fallback bindings plus a one-time status hint when the terminal can't report it.
  - `enter` sends · `↑/↓` input history (at buffer edges) · `esc` interrupt (run) · `ctrl+c` clears a non-empty input; on an empty input, `ctrl+c` twice within 1s quits · `ctrl+o` opens the pager on the most recent collapsible item.
  - `/` prefix = commands: `/model`, `/effort`, `/hard`, `/yolo`, `/clear`, `/compact`, `/cost`, `/undo`, `/copy`, `/show <n>`, `/help`, `/exit` (`/q`, `/quit` aliases).
  - **`/hard` is a toggle**: first use saves the current `{model, effort}` and switches to `modelHard` + effort `high` (clamped, §3); second use restores the saved pair. Both directions write a `model_change` entry.
- **Status bar — two lines (rev 13), always current:** a red `YOLO` field first on line 1, **only while yolo mode is on** (§7.5; never dropped for width) · **line 1:** `version` (the git-describe build stamp, e.g. `v0.1.0-alpha-1-gc392fac`) · `cwd` (the session workdir, home-abbreviated, e.g. `~/projects/moca` — constant, since `shell` is stateless, §4) · `branch` (git, dirty marker `*`, non-git = dimmed `-`) · **line 2:** `provider/model` + **effort** (effective level, e.g. `opencode-go/glm-5.3-flash · med`) · **context size and usage %** (e.g. `ctx 1M · 23%`) · **session tokens in/out** (e.g. `24k/6k`) · **session cost** (e.g. `$0.0412`; `sub` for subscription-OAuth turns, where per-token cost is not billed), live per response.
  - branch updates on every turn end (tools can switch or dirty it mid-session).
  - context usage % = estimated tokens of the next request (§6) against the catalog's context window; recalculated after every tool result and response.
  - session tokens/cost accumulate from `usage` in responses — cache reads/writes itemized in `/cost` detail, not the bar.
  - transient states (`retry 2/5 · 4s`, §3) temporarily replace the cost field.
  - updates are event-driven, not polled; during streaming only the spinner changes.
  - shrink order: line 1 drops cwd → branch, then truncates the version; line 2 drops in/out, then truncates the model; neither line ever wraps.
- Streaming renders inline; tool calls render as numbered collapsible one-liners (`▸ #7 edit main.go [+3 −1]`); `ctrl+o` (latest) or `/show 7` opens the item — diff, full output — in the pager. Submitted messages and assistant responses carry distinct scrollback background colors (a light/dark pair chosen from the terminal background via `tea.BackgroundColorMsg`, dark until it is reported); thinking and tool lines stay plain.
- **Thinking blocks render collapsed** as a numbered one-liner (`⋯ #6 thinking N lines`), expanded only in the pager — every catalog model emits reasoning; it is persisted in the session (§8) and serialized into compaction (§6), never shown expanded by default.
- **Input during a run**: a message sent while the agent works is **steering** — it is appended **after the current tool batch's results**, before the next model request (no provider accepts a user message between a tool call and its result). If the current turn has no tool calls, it simply becomes the next user message. `esc` aborts the run (synthetic `tool_result` per §8); queued steering messages return to the editor. The input box is never dead.
- **`!` prefix** runs a shell command from the input box; output (truncated like the `shell` tool, 30K) enters the conversation as a user message (`!!` runs it locally without model visibility). `!`/`!!` are user-initiated (the user's own hands, like any terminal) — they bypass the model-facing analysis/allowlist; the shell tool's own rules are unchanged.

## 12. Config

`~/.config/moca/config.jsonc` — single file, typed struct, fail-fast validation. **JSONC over TOML (rev 6):** stdlib-parseable (comment-strip pre-parse, Go `encoding/json` decodes the result — zero third-party config deps), and it matches the house dotfiles sync machinery (`merge-opencode-config.py` / `merge-pi-config.py` semantics: recursive object merge, live-only keys kept, ordered array union, scalar template-wins / `--live-wins` flip, `.bak` backup, byte-identical-if-unchanged so comments survive). JSONC is also the native shape of the OpenCode ecosystem configs `moca mcp import` reads — one parser everywhere.

**The JSONC pre-pass is string-literal-aware and handles comments and trailing commas**: `//` and `/* */` are comments only outside quoted strings (escape sequences honoured) — the example below contains URLs with `//`, and a naive first-`//`-cut parser destroys them. A `,` followed only by whitespace/comments before `}` or `]` is removed in the same pass — `encoding/json` rejects trailing commas, and JSONC files (including the example below and the house merge scripts' output) routinely have them. Comments are replaced with spaces/newlines so `encoding/json` error offsets still map to the original line:col. **Phase-1 parser gate**: the §12 example decodes into the typed config struct with every value intact (URLs included), and the pre-pass is a no-op on plain JSON.

```jsonc
{
  "model":      "opencode-go/glm-5.3-flash",   // everyday turns — cheap, fast
  "modelHard":  "opencode-go/glm-5.3",          // /hard escalation: planning, gnarly debug

  "providers": {
    "anthropic":   { "auth": "api_key", "apiKey": "env:ANTHROPIC_API_KEY" },  // api_key only — subscription OAuth not permitted (§3, rev 12)
    "opencode-go": { "auth": "api_key", "apiKey": "env:OPENCODE_API_KEY" },   // OpenCode Zen subscription key
    "openai":      { "auth": "api_key", "apiKey": "env:OPENAI_API_KEY" },     // or "oauth" → moca login openai (ChatGPT Plus/Pro, §3)
    // optional baseUrl override per provider; custom providers too, e.g.:
    // "vllm": { "baseUrl": "http://localhost:8000/v1", "protocol": "openai-completions", "auth": "api_key",
    //           "apiKey": "env:VLLM_API_KEY", "models": { "qwen3-coder": { "contextWindow": 131072 } } }
  },

  "shell": {
    "allow": ["go", "git", "gh", "grep", "rg", "find", "ls", "cat", "head", "tail",
              "wc", "sort", "uniq", "diff", "which", "jq", "mkdir", "sed", "awk",
              "curl", "make", "mise", "python", "pytest", "node", "npm",
              "docker", "kubectl", "terraform", "ansible", "rtk", "graphify"]
  },

  "mcp": {
    "idleTimeout": 600,                                        // seconds, lazy stop
    "servers": {
      "context7": { "url": "https://mcp.context7.com/mcp",      // lazy: starts on first call, not at startup
                    "description": "library/API docs lookup" },
    },
  },

  "snapshot": { "retentionDays": 30 },   // 0 = keep forever

  "yolo": false,                         // true = all permission checks off by default (§7.5); --no-yolo overrides

  "context": { "reserveTokens": 16384, "keepRecentTokens": 20000, "maxSteps": 40 }
}
```

**Model routing:** `/effort` sets the effort level (validated against the catalog, clamped per §3); `/hard` toggles to `modelHard` + effort `high` and back (§11); compaction summaries always run on the cheap `model` at minimal effort. No auto-escalation heuristics in v1 — explicit is cheaper to debug than clever. **Secrets never appear as literals** — `env:VAR` indirection or the OAuth token store only; validation rejects an `apiKey` that isn't an `env:` reference.

### 12.5 CLI surface

```
moca                         TUI in the current directory
moca -p "<prompt>"           one-shot; prompt also accepted on stdin (`-p -`)
  --model <provider/model>   override config `model` for this session
  --effort <level>           override effort for this session
  --approve | --no-approve   project trust for this run (-p default: --no-approve)
  --yolo | --no-yolo         all permission checks off/on for this run (overrides config `yolo`, §7.5)
  --resume <id8|last>        resume a session        --continue   latest session in this workdir
moca login <provider>        OAuth login (openai; §3)  moca logout <provider>
moca mcp import              import MCP servers from Claude Code / OpenCode / Pi configs
moca mcp index               prebuild the persisted MCP discovery index
moca --version
```

**`-p` output contract**: stdout = the final assistant message text only (pipe-friendly); stderr = tool-call one-liners, refusals, retries, and the **exit summary** (session tokens in/out + total cost). **Exit codes**: `0` completed · `1` provider/runtime error · `2` config or usage error · `3` stopped at `maxSteps` (wrap-up text still printed) · `130` interrupted.

## 13. System prompt

~40 lines of moca text, frozen at phase 2: role, workdir, platform (OS/arch via `uname -sm` / Windows equivalent — no hostname), date, git state, tool contract summary (incl. "`shell` is stateless; use `cd dir && cmd`"), token discipline (incl. the rtk preference line), version stamp at the end (cache invalidates visibly when it changes). Appended after it: the skill list (name + description + absolute path, §9), the MCP server roster (§10.5), and project instructions (§9.5). **Built once at session start, stored in the `session` entry (§8), never rebuilt mid-session** — a rebuilt prompt embeds fresh git state and invalidates the prompt cache on every commit the agent itself makes (self-sabotage on the core edit→test→commit loop). Git state may go stale; the status bar is the live source. Resume reuses the stored prompt verbatim.

## 14. Ship gate (v0.1 demo)

One session, real repo, unattended — **on the `opencode-go` provider** (proves multi-provider day 1):
1. `read` a failing test,
2. locate the bug with `search`,
3. `edit` the fix,
4. run `go test ./...` (or `rtk test -- go test ./...`) via shell — green,
5. commit on a feature branch,
6. the built-in `rtk` skill was discoverable and its guidance followed.

## 15. Explicit non-goals (v1)

subagents · hooks · plan mode · LSP · web browsing · image gen · voice · telemetry · **binary plugin/extension system** (skills + commands + allowlist + the §10.5 lazy `mcp` proxy are the extension model — a fifth mechanism needs a design revision) · `serve` mode · OS-level shell sandboxing · nested per-directory AGENTS.md · MCP OAuth / sampling / elicitation / legacy-SSE (v2 discussion).

## Phase plan (each = one PR, reviewed)

Detailed task-level plans: `docs/plans/phase-<n>-*.md`.


1. **Skeleton + protocol adapters + streaming + JSONC config** — all three codecs (anthropic-messages, openai-completions, openai-responses), api-key auth, retry/backoff + stall timeout (§3), model catalog incl. thinkingLevelMap, string-literal-aware JSONC pre-pass with trailing-comma removal (§12), `-p` output contract + exit codes (§12.5). Gate: `moca -p 'hi'` streams **via anthropic, opencode-go (both protocol families) and openai**; the §12 example decodes intact (URLs, trailing commas); a forced 429 retries with backoff.
2. **Seven tools + agent loop + permissions + skills loader + session schema** — path jail (session-start anchored, symlink-resolved) + read-only skill roots, shell command analysis (`mvdan.cc/sh`, every simple command checked, redirect jail, hard-deny/ask set), project trust (`--approve`/`--no-approve` + trust.json; the interactive prompt lands with the TUI in phase 3), AGENTS.md loading, maxSteps (`tool_choice: none` wrap-up), sequential tool execution, the session schema (entry ids + types incl. `compaction` placeholder + `session` first entry with stored system prompt, thinking persisted, transcript repair) land *with* the tools (no ungated phase); `mcp` tool ships as a stub (`search`/`describe`/`call` → "no servers configured" — schema frozen from day 1, no churn later); pre-edit snapshots + undo stack; yolo mode (§7.5: `--yolo`/`--no-yolo`, config `yolo`, `permission_mode` entry). Gates: edit-ladder green, jail enforced (incl. symlink escape), shell-analysis ladder green, untrusted workdir's project skills/AGENTS.md NOT loaded under `--no-approve`, maxSteps forces wrap-up, a global skill's body loads via `read`, `-p --yolo` runs a non-allowlisted command, a hard-deny command and an out-of-workdir write that are all refused without it.
3. **TUI shell + slash commands + status bar** — status bar (bottom, below input) from the first frame: cwd · branch · provider/model · effort · context size & usage % · session tokens · session cost. Inline immutable scrollback + pager (`ctrl+o`, `/show`); thinking collapsed; steering after tool results; `!`/`!!` prefix; interactive trust + shell/MCP approval prompts; `/undo`, `/copy`, `/hard` toggle, `/yolo` toggle + red `YOLO` status field; cross-provider history transform (§3). Gate: full session in TUI; `/model`, `/effort`, `/hard` work, including a mid-session switch anthropic → opencode-go with thinking + tool calls in history; bar reflects a `shell`-tool branch change at the next turn; a multi-line paste inserts verbatim and sends intact; `shift+enter` newline verified on an enhanced-keyboard terminal and fallback confirmed on one without; a steering message queued mid-run lands after the current tool results; `/undo` reverts an edit; `/yolo` on → a command that would prompt runs without a prompt and the bar shows `YOLO`, `/yolo` off → prompts return.
4. **Context manager + compaction + resume** — token budgets (reserve 16K / keepRecent 20K, small-window scaling + loop guard), usage-anchored token estimate, cut-point rules, structured summaries with cumulative file tracking, overflow compact-and-retry, no-cache-write on summary requests; resume + `--continue` (session-entry workdir mapping and stored system prompt, §8). Gate: long synthetic session crosses the token threshold and compacts (context shrinks, no tool result orphaned from its call); a 32K-window custom model compacts without looping; provider overflow triggers one compact-and-retry; resume + `--continue` rebuild request context from the transcript incl. compaction entries; a session killed mid-tool resumes with a synthetic result.
5. **MCP lazy proxy** — stdio + streamable HTTP transports, persisted discovery index, `describe`, lazy lifecycle, annotation-based gating, `moca mcp import` (secret rewriting) + `moca mcp index`. Gates: a real server (e.g. context7 or filesystem) searchable, describable and callable via the proxy with no server tool schemas in the prompt (verified by inspecting the request payload); a second session searches it with **zero servers started**; server stopped after idle timeout; a non-read-only tool is refused in `-p` unless in `approve`; import converts an existing Claude-Code/OpenCode/Pi config with no literal secrets written.
6. **rtk + graphify + skills ecosystem compatibility** — Gates: rtk-wrapped commands preferred in a real session (and `rtk <non-allowlisted>` refused); a graphify/pi/claude-code SKILL.md loads unchanged.
7. **OAuth providers + upstream graphify PR + v0.1** — subscription OAuth per the §3 policy gate (verified 2026-10-06: **anthropic `api_key` only**; **openai ships** ChatGPT subscription login via Sign in with ChatGPT open-source token sharing; SSH/headless fallback: copy-URL + paste-code when the local callback is unreachable — `docs/specs/oauth-verification.md`), `graphify install --platform moca` upstream, polish, tag `v0.1.0` when §14 passes.

**Edit-ladder tests (phase 2, non-negotiable):** exact unique · exact ambiguous · `replace_all` (n hits) · whitespace-fallback hit with re-indentation (indent deeper + shallower) · fallback ambiguous · no match · no-op new==old · empty old_string · CRLF file (stays CRLF) · BOM file · unicode file · 50-line span · trailing-newline-missing file · write to new file (no read needed) · write/edit after an on-disk change (refused).

**Shell-analysis ladder (phase 2):** allowlisted simple · not allowlisted · `a | b` / `a && b` / `a; b` with b denied · `$(rm x)` and backtick substitution · `( sudo … )` subshell · `FOO=1 go test` · `timeout 5 rm x` (unwrapped → ask) · `> ../outside` redirect · `> /dev/null` · `eval` / `$CMD` · unparseable input · `cd sub && make`.

## Revision log

- **rev 13 (2026-10-08, Ben's request — TUI polish):** the status bar is **two lines** — line 1 `version · cwd · branch`, line 2 `provider/model · effort · ctx % · in/out · cost` — with the **build version** added as a field (the git-describe stamp); **full-width dim rules** separate the input area from the output above and the status bar below; the input area renders **without a prompt prefix** (the per-line `›` is gone); a session opens with **two welcome lines** at the top of the scrollback (`moca <version>` and a one-line greeting); **`/exit`** with `/q` and `/quit` aliases (same path as ctrl+c×2 — an in-flight run/compaction is cancelled and recorded); submitted messages and assistant responses render on **distinct scrollback background colors** (light/dark pairs chosen from the terminal background; thinking and tool lines stay plain). Implemented details: SPECS §3.5.
- **rev 12 (2026-10-06, phase 7 policy gate — verification results; spec change, part of the phase-7 PR for Ben's sign-off):** subscription OAuth verified live **before** implementation (`docs/specs/oauth-verification.md`, retrieved 2026-10-06). **anthropic → `api_key` only**: "Anthropic does not permit third-party developers to offer Claude.ai login into their own applications, or to route requests through Free, Pro, or Max plan credentials on behalf of their users" (Claude Code legal docs; server-side enforcement since 2026-01, formalized 2026-02, reaffirmed 2026-09) — the OAuth option is dropped from §3/§12/README and `providers.anthropic.auth: \"oauth\"` is rejected by config validation with the recorded reason. **openai → ships OAuth** via "Sign in with ChatGPT" open-source token sharing (documented, self-service dynamic client registration — no client secret; scopes `openid profile email offline_access resource.invoke chatgpt.tokens.use.direct`; rotating 30-day refresh tokens with serialized refreshes; Responses API only, `store:false`/`stream:true`, function tools grouped in a namespace; access tokens 1 h). `api_key` stays supported for both.
- **rev 11 (2026-10-04, Ben approved full plan review — fixes applied):** the seven per-phase plans (`docs/plans/`) reviewed line-by-line against this spec (5 reviews delegated, 2 in-session; every machine-checkable claim verified programmatically). Fixes: phase-1 allow-list test asserted 31, actual **32** · phase-3 `AppendString` trailing-comma branch was dead code (`Standardize` blanks the comma in `std` — the comma is now detected in the raw source; the byte-exact fixture passes) · phase-4 `build()` declaration gained the missing `snapshotDir` param (the call site already passed it) · phase-5 `Call()` idle-timer race — `ensure` + tool lookup + `busy++` + timer-stop now share **one** critical section (the old two-section window let an idle-timer fire nil the client and panic a call landing exactly at idle expiry); regression test `TestCallSurvivesIdleTimer` (1ns idle, 25 concurrent calls, `-race`) · phase-2: statelessness test uses `pwd -P` (physical path — macOS `/var` symlink), truncated-tool-call assertion made case-insensitive, skill-list line shape `- name: description (path)` pinned (phase-7's shipgate asserts the literal `- rtk:`) · §7.5 clarified: `/yolo` toggles between runs only (refused mid-run) · all plan headers re-cited to **rev 11**, sub-skill references de-prefixed (`subagent-driven-development` is not resolvable as `superpowers:` here) · phase-7 "rev-10 entry" wording future-proofed to "a new revision entry". One reported blocker **retracted after verification**: phase-2's "`callTool` undefined" did not exist in the file (the test uses the `run` helper throughout) — reported by a reviewer, repeated unverified; the retraction is recorded here so the plans' trustworthiness stays auditable. *Addendum (same day, review of rev 11):* phase-5 `manager_test.go` gained the missing `"sync"` import (`TestCallSurvivesIdleTimer` did not compile) · the now-unused, lock-free `Manager.lookup` removed (it called `ensure` without `st.mu`) · `Call` documents that a ... [truncated]
- **rev 10 (2026-10-04, Ben's steering):** **yolo mode** (§7.5) — `--yolo`/`--no-yolo`, config `"yolo"`, TUI `/yolo` toggle; turns off *every* permission check (path jail, shell analysis incl. hard-deny, MCP gating, approval prompts, project trust unless `--no-approve` is explicit); keeps maxSteps, read-before-write guards, snapshots/undo, shell hygiene and secret stripping; red `YOLO` status-bar field; `session.yolo` + `permission_mode` audit entries, never restored on resume. Phases 1–3 + 5 plans updated.
- **rev 9 (2026-10-04, Ben approved full review — fixes + improvements):** **Fixes:** JSONC pre-pass strips trailing commas (the §12 example had one; `encoding/json` rejects them — rev 8's claim was wrong) and the phase-1 gate is "decodes intact", not the contradictory "byte-identical with comments stripped" · global/built-in skill dirs are read-only roots (the jail blocked §9's body-load-by-`read`) · `mcp` gains `describe` (calling blind without a schema can't work) · phase cross-refs fixed (OAuth verification → phase 7, skills compat test → phase 6, graphify PR → phase 7) · `openai-responses` adapter assigned (phase 1) · phase-2 trust gate via flags (interactive prompt needs the TUI, phase 3) · `/hard` no longer double-booked in phase 6 · env indirection unified to `"env:VAR"` (`apiKeyEnv` → `apiKey`) · `defaultProvider` removed (model ids are provider-qualified). **Specified:** `shell` stateless (every call at the jail root; cd/cwd contradiction removed; status-bar cwd constant), `bash -c` not `-lc`, stdin `/dev/null`, pager/editor env, process-group kill, provider-key env stripped · edit contract: re-indentation on fallback, CRLF/BOM preservation, non-empty old_string, `replace_all` (before the phase-2 schema freeze) · `write`: new files allowed, stale-on-disk writes refused · steering placed after tool results; synthetic `tool_result` on abort/crash (transcript repair) · cross-provider history transform (thinking/signatures/tool ids) · retry/backoff + SSE stall timeout + mid-stream failure rule · sequential tool execution · `max_tokens`/thinking-budget rule · cache-breakpoint placement · effort clamping · maxSteps wrap-up via `tool_choice: none` · `length` stop ≠ overflow unless input near window · `/clear` = new session, `/hard` toggle, `ctrl+c` semantics. **Improvements:** honest threat model (permissions ≠ sandbox) + real shell parsing via `mvdan.cc/sh` (substitutions, subshells, redirects, wrappers); `rm` ask-every-time instead of hard-ban; Windows deny set incl. `Remove-Item` aliases · symlink-resolved path jail · package layout: `internal/llm` leaf types, `context` → `compact`, injected Summarize/checkers · usage-anchored token estimate · small-window budget scaling + 16K minimum + compaction loop guard · AGENTS.md/CLAUDE.md project instructions (§9.5, trust-gated) · built-in skills embedded + extracted · `/undo` over the snapshot stack, `/copy` (OSC 52), pager + numbered items (`ctrl+o`, `/show`) instead of expand-in-place under inline rendering · MCP: persisted discovery index + `moca mcp index`, server roster in the system prompt, annotation-based default-deny gating (replaces name regex), result truncation, import rewrites literal secrets · system prompt stored in the `session` entry (exact resume), no hostname · session files 0600 · `search` respects `.gitignore`, RE2 noted · `read` line-length cap + binary refusal · CLI surface + `-p` stdout/stderr/exit-code contract (§12.5) · subscription-OAuth policy gate (§3) · edit-ladder extended + new shell-analysis ladder. *Addendum (same day):* moved to `docs/specs/`, per-phase plans in `docs/plans/`; effort mapping split by `thinkingMode` (anthropic adaptive + `output_config.effort` vs legacy `budget_tokens`), clamp falls back to the lowest supported level, thinking replay bound to the producing **model** (not provider).
- **rev 8 (2026-10-03, Ben approved full review — fixes + improvements):** consistency pass + under-specification fixes from the rev-7 design review: Purpose/§2/README resynced to rev-3+ reality (7 tools, `mcp/` package in the tree, MCP in the extension model, README status/phases/non-goals rewritten) · §10.5 + §12 config blocks converted TOML→JSONC; MCP config restructured under `mcp.servers` (no user-keyed-map namespace collision: `mcp.idleTimeout` scalar beside `servers`, `mcp.servers.<name>.approve` gating) · JSONC comment stripping specified **string-literal-aware** (URLs with `//` survive; §12-example round-trip = phase-1 gate) · session schema: `model_change` carries `{model, effort}`; new `session` first entry (workdir, provider, model, effort, startedAt) powers `--continue`'s workdir mapping · skills body-load trigger specified (absolute path in the system prompt; body loaded via plain `read` — no new mechanism) · path jail **anchored at session-start workdir** (mid-session `cd` never moves the jail) · `!`/`!!` user-initiated, bypass the model-facing allowlist · compound shell commands: every `|`/`;`/`&&`/`||` segment allowlist-checked, hard-deny set checked in all segments · system prompt built once per session, never rebuilt (git state may go stale — prompt-cache discipline) · token estimate = chars/4 (catalog calibration later) · `-p` exit summary (tokens in/out + cost to stderr) · snapshots skipped for clean tracked files + 30-day prune (`snapshot.retentionDays`) · `pwsh`→`powershell.exe` fallback · optional per-provider `baseUrl` + custom providers (vLLM/LM Studio/Ollama day 1) · MCP HTTP `headers` with env indirection · `gh` in the default allowlist · `--continue` ownership moved to phase 4. *Addendum:* OAuth `moca login` SSH/headless fallback — copy-URL + paste-code when the local callback is unreachable (§3, phase 7).
- **rev 7 (2026-10-03, Ben approved design review):** compaction rewritten token-denominated (reserveTokens 16K / keepRecentTokens 20K, cut-point rules — never orphan a tool result, overflow compact-and-retry, no-cache-write on summary, structured summary + cumulative file tracking, 2K truncation in serialization) · session schema: entry ids + types incl. `compaction` (+ `--continue`) · thinking blocks persisted/rendered-collapsed · steering input during runs · project trust (`--approve`/`--no-approve`, trust.json) · maxSteps 40 runaway guard · `!` prefix · pre-edit snapshots. Sources: Pi's shipped compaction/security/usage docs, OpenCode's agents docs + on-disk data layout (snapshot/, tool-output/).
- **rev 6 (2026-10-03, Ben's question → adopted):** config format TOML → **JSONC** (`~/.config/moca/config.jsonc`, camelCase keys) — stdlib-parseable (comment-strip pre-parse + `encoding/json`, zero config deps), matches the house dotfiles merge machinery (merge-opencode-config.py / merge-pi-config.py semantics: recursive object merge, live-only keys kept, ordered array union, template-wins/`--live-wins`, `.bak`, byte-identical-if-unchanged), native shape of OpenCode-ecosystem configs `moca mcp import` reads.
- **rev 5 (2026-10-03, Ben's steering):** status bar moved to the **bottom, below the input area** · input area is a real multi-line text area with native multi-line paste (bracketed-paste, verbatim, never auto-sent; >50-line pastes render collapsed, buffer intact; native copy via inline rendering + OSC 52) · `shift+enter` = newline (enhanced keyboard reporting — Kitty protocol/xterm modifyOtherKeys — with `alt+enter`/`ctrl+j` fallbacks + one-time hint on plain terminals).
- **rev 4 (2026-10-03, Ben's steering):** status bar from day 1 with the exact field set: cwd · branch (dirty marker) · provider/model · **effort** · context size + usage % · session tokens in/out · session cost — effort promoted to a first-class request parameter (`/effort`, protocol-mapped, catalog-validated, default medium); `/hard` = model_hard + effort high; layout inverted (bar on top); context-usage estimate and per-response usage accounting pulled into phase 3.
- **rev 3 (2026-10-03, Ben's steering):** MCP support in v1, lazy by design — one fixed ~200-token `mcp` proxy tool (§10.5), server tool lists never enter the prompt, lazy server lifecycle (first-call start, 10-min idle stop), discovery index, stdio + streamable HTTP, filtered subprocess env, risky-tool gating, `moca mcp import`; tool count six → seven (schema frozen at phase 2 as a stub so no churn); phase plan six → seven phases (MCP gets its own PR).
- **rev 2 (2026-10-03, Ben's steering):** `bash` → `shell` (PowerShell on Windows) · `model` + `model_hard` from day 1 · three day-1 providers (OpenCode Go / Claude / OpenAI, incl. subscription OAuth) replacing the anthropic+openai-compat pair · external tools (rtk, graphify) from day 1 via skills/commands/allowlist · phase plan restructured (permissions pulled into phase 2, OAuth + upstream graphify PR in phase 6).
- **rev 1 (2026-10-03):** initial draft.

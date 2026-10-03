# moca DESIGN.md — v1 contract

Status: **DRAFT — awaiting Ben's redline**. Nothing below is code yet. Every numbered decision is up for challenge. Once signed off, this doc is the source of truth; changes require a new section, not silent drift.

## Purpose

A minimal, token-efficient, provider-agnostic coding agent. No frameworks, no plugin systems, no MCP. The bet: **a tight agent loop + disciplined context handling beats a feature-rich one** on cost-per-task for everyday coding (read → edit → run tests → commit).

## 1. Identity

- `moca` — MO Coding Agent. Binary name = repo name = module path (`github.com/adeotek/moca`).
- Tagline: *the coding agent you can read in an afternoon.*

## 2. Architecture

```
cmd/moca/            main: flag parsing, mode select (TUI | -p one-shot | serve later)
internal/
  agent/             the loop: user msg → model → tool calls → results → repeat
  provider/          Provider interface + anthropic/ (native) + openai/ (compat)
  tools/             Tool interface + registry + the 6 built-ins
  context/           token accounting, truncation, summarization, cache markers
  session/           JSONL append-only transcripts, resume
  tui/               Bubble Tea: input, scrollback, status bar, spinner, diff view
  config/            single config file, typed, validated
  skills/            Agent Skills standard loader (SKILL.md discovery)
  permissions/       path jail, bash allowlist, per-tool gates
```

**Direction of dependencies: `agent` may import everything; `tools`, `provider`, `session`, `context`, `config`, `skills`, `permissions` import nothing from each other except `config` types. `tui` imports `agent` (drives it), never the reverse.** No cycles, ever. This is the whole "minimal" promise made structural.

## 3. Provider layer

One interface, the smallest honest surface:

```go
type Provider interface {
    Stream(ctx context.Context, req Request) (<-chan Event, error)
    CountTokens(ctx, messages) (int, error)   // exact if API offers it, else heuristic
}
```

- `Request` carries: messages, tool schemas, **cache-breakpoint markers** (explicit positions where the provider's prompt cache can attach — Anthropic `cache_control` native, OpenAI-compat via prefix-stability), max_tokens, model.
- v1 ships **anthropic** (native API) and **openai-compat** (any /v1/chat/completions endpoint — covers OpenRouter, opencode-go, LM Studio, llama.cpp).
- No SDK — hand-rolled HTTP + SSE. SDKs are the second-biggest source of bloat after MCP, and SSE is ~150 lines.

## 4. Tools — the six, frozen for v1

| Tool | Semantics |
|------|-----------|
| `read` | path + optional offset/limit. Returns lines `N|content`-style with total line count. Caps at 2000 lines/50K chars per call. |
| `write` | path + full content. Refuses without a prior `read` in session (anti-overwrite guard). |
| `edit` | path + old_string + new_string. **Exact match first; if no exact hit, one whitespace/indent-tolerant fallback pass; if still nothing or >1 match → structured error, no write.** Returns unified-diff-style result, never the full file. |
| `bash` | command + timeout (default 30s, max 300s). Runs in workdir, jailed (§7). Output head+tail truncated at 30K chars with `[… N lines omitted]`. |
| `search` | ripgrep-style: pattern + glob + path. Content matches with line numbers, 200 hits cap, `files_only` mode. |
| `ls` | path, one level, dirs suffixed `/`, hidden opt-in. |

Schema churn rule: these JSON schemas are frozen after phase 2. Any new tool = v2 discussion, because every session re-reads the schemas (token cost) and tool-list churn kills prompt-cache reuse.

## 5. Edit-tool contract (the make-or-break detail)

1. `old_string` must be **unique** in the file. Ambiguous → error listing line numbers of each hit, no write.
2. Exact match fails → single fallback pass: match modulo leading-whitespace on each line (handles the model's slightly-off indentation). Exactly one fallback hit → apply. Zero or >1 → error.
3. `new_string == old_string` → rejected (no-op).
4. Success returns a diff + line numbers changed + new total line count.
5. Errors are written for the **model** to recover from: what failed, where, what to do next ("re-read lines 40-60 and retry with more context").

## 6. Context manager (the token-efficiency engine)

Budgets (all overridable in config):
- Context window: from provider metadata, fallback 200K.
- **Compaction trigger at 80%**: keep system prompt + last 4 turns verbatim; older turns → one summary request to the same model (cheap, cache-hit-priced). Summary is lossy by design and labeled `COMPACTION NOTE (lossy)`.
- Tool output caps as in §4. The `read` tool always returns line numbers + totals so a re-read can target a window, never the whole file again.

Token efficiency comes from: prompt caching (§3), windowed reads, diff-shaped results, truncation, compaction. NOT from stripping tool descriptions — descriptive schemas up front are cheaper than failed calls.

## 7. Permissions

- **Path jail**: workdir and its children. `read`/`write`/`edit` refuse anything outside; `~` expansion only inside the jail.
- **bash**: deny-by-allowlist. Default allow: `go git grep rg find ls cat head tail mkdir sed awk curl mise python pytest node npm docker kubectl terraform ansible`. Everything else → refusal message telling the model which prefix to ask the user about (TUI surfaces a one-key allow/deny). `rm`, `sudo`, `dd`, `shred`, anything with `|` into them not in allowlist → hard no in v1, no override.
- One-shot `-p` mode: workdir-only, no interactive escalation. Refusals are errors in the transcript.

## 8. Sessions

- `~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl`, append-only, one JSON object per line: `{t, role, content, tool_use, tool_result, usage, model}`.
- Resume: `moca --resume <id8>` (or `--resume last`). Tool schemas and compaction state rebuild from the transcript.
- `usage` on every assistant turn → cumulative cost line in the status bar, live.

## 9. Skills

- Agent Skills standard, same shape as Pi/Claude Code: `<workdir>/.moca/skills/<name>/SKILL.md` + `~/.config/moca/skills/`.
- Frontmatter: `name`, `description`, optional `argument-hint`. Loader names them in the system prompt (name+description only — never the body) and the model asks for the body explicitly. This is the token-cheap discovery pattern.

## 10. TUI

- Bubble Tea + lipgloss. Compact aesthetic per house style: 14px-equivalent density, tight padding, no banners.
- Layout: input box (bottom), scrollback (fills), status bar (1 line: model · tokens · cost · mode).
- Streaming text renders inline; tool calls render as collapsible one-line entries (`▸ edit main.go [+3 −1]`); diffs render in a pager view on demand (`v` on an entry).
- Keys: `enter` send · `alt+enter` newline · `↑/↓` history · `esc` interrupt · `ctrl+c` twice quit · `/` prefix = commands (`/model`, `/clear`, `/compact`, `/cost`).
- One pasted block >8 lines → shows `[paste N lines]` chip, `alt+p` to expand in place. Never auto-dump 200-line pastes into the input.

## 11. Config

`~/.config/moca/config.toml` (single file, typed struct, fail-fast validation, comments inline):

```toml
default_provider = "anthropic"        # | "openai-compat"
default_model    = "claude-sonnet-4-5"

[providers.anthropic]
api_key_env = "ANTHROPIC_API_KEY"     # env var name — never a literal key

[providers.openai-compat]
base_url    = "http://localhost:8080/v1"
api_key_env = "OPENAI_COMPAT_API_KEY"

[bash]
allow = ["go", "git", "grep", "rg", "find", "ls", "cat", "head", "tail",
         "mkdir", "sed", "awk", "curl", "mise", "python", "pytest",
         "node", "npm", "docker", "kubectl", "terraform", "ansible"]

[context]
compact_at_pct = 80
keep_turns     = 4
```

No environment auto-magic beyond `api_key_env` indirection. TOML not JSON/YAML — comments and diff-friendly edits.

## 12. System prompt

~40 lines, frozen at phase 2, includes: role, workdir, platform (`uname -a`), git state summary (branch, dirty), tool contract summary, token discipline rules ("prefer windowed reads; never re-read whole files; diffs not files"). Version stamp at the end so cache invalidates visibly when it changes.

## 13. Ship gate (v0.1 demo)

In one session, in a real repo, unattended:
1. read a failing test,
2. locate the bug with `search`,
3. `edit` the fix,
4. run `go test ./...` via bash — green,
5. commit on a feature branch.

If that doesn't pass end-to-end, v0.1 doesn't ship. Everything else is polish.

## 14. Explicit non-goals (v1)

MCP · subagents · hooks · plan mode · LSP · web browsing · image gen · voice · telemetry · plugin systems · anything that requires a `serve` mode.

## Phase plan (each = one PR, reviewed)

1. **Skeleton + provider + streaming** — `moca -p 'hi'` streams to stdout. Review focus: provider interface shape.
2. **Six tools + agent loop** — edit-ladder tests green (see phase-2 tests below). Review focus: edit contract §5.
3. **TUI shell** — full session runs inside Bubble Tea. Review focus: layout + keys §10.
4. **Context manager + sessions** — budget holds at 80%, resume works. Review focus: compaction §6.
5. **Permissions + openai-compat + skills** — allowlist enforced, skills discovered. Review focus: §7.
6. **Polish + v0.1** — ship gate §13 passes, tag `v0.1.0`.

**Edit-ladder tests (phase 2, non-negotiable):** exact unique · exact ambiguous · whitespace-fallback hit · fallback ambiguous · no match · no-op new==old · CRLF file · file with unicode · old_string spanning 50 lines · trailing-newline-missing file.

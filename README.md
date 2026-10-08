# moca — MO Coding Agent

[![CI](https://github.com/adeotek/moca/actions/workflows/ci.yml/badge.svg)](https://github.com/adeotek/moca/actions/workflows/ci.yml)

**A minimal, token-efficient, provider-agnostic coding agent: one Go binary, a Bubble Tea TUI, seven tools, no framework.**
Inspired by Claude Code, OpenCode and Pi — deliberately ~10% of their surface area. It competes on *cost per task* and *read-the-whole-codebase-in-an-hour transparency*, not features: prompt caching, windowed reads, diff-shaped results and token-denominated compaction keep a session cheap; the agent ships MCP support that never floods the prompt (one ~200-token lazy proxy), loads Agent-Skills `SKILL.md` files written for other tools unchanged, and prefers external token-savers (`rtk`, `graphify`) when they are installed.

**Status: v0.1.0 (pending the §14 ship gate; the tag lands when the gate's live legs pass).**

## Install

```bash
go install github.com/adeotek/moca/cmd/moca@latest
```

or take a release binary (linux/amd64 · linux/arm64 · darwin/amd64 · darwin/arm64 · windows/amd64) from the [releases page](https://github.com/adeotek/moca/releases).

## Configure

`~/.config/moca/config.jsonc` (JSONC — comments allowed; `$XDG_CONFIG_HOME` is honored). Minimal:

```jsonc
{
  "model":      "opencode-go/glm-5.3-flash",   // everyday turns — cheap, fast
  "modelHard":  "opencode-go/glm-5.3",         // /hard escalation: planning, gnarly debug

  "providers": {
    "opencode-go": { "auth": "api_key", "apiKey": "env:OPENCODE_API_KEY" },
    "anthropic":   { "auth": "api_key", "apiKey": "env:ANTHROPIC_API_KEY" },
    "openai":      { "auth": "api_key", "apiKey": "env:OPENAI_API_KEY" }
  }
}
```

Secrets never appear as literals: `apiKey` is an optional `env:VAR` reference — or drop it and store the credential with `/login` (TUI) / `moca login` below. A stored key wins over the env reference.

### Providers

| Provider | Auth | Notes |
|---|---|---|
| `opencode-go` | `OPENCODE_API_KEY` | OpenCode Zen (mixed-protocol catalog: glm-5.3, minimax-m3, kimi-k3, …) |
| `anthropic` | `ANTHROPIC_API_KEY` | Claude; subscription OAuth is **not permitted** by Anthropic's terms for third-party clients — API key only |
| `openai` | `OPENAI_API_KEY` **or** `moca login openai` | ChatGPT Plus/Pro subscription login via OpenAI's "Sign in with ChatGPT" open-source flow |
| any name | `baseUrl` + `protocol` | custom providers: vLLM, LM Studio, Ollama, OpenAI-compatible gateways |

**Credentials live in `~/.config/moca/auth.json`** (0600, never committed): `/login` in the TUI — pick a provider, then paste the API key (masked, never echoed) or sign in to a subscription in the browser — or `moca login <provider>` on the CLI (`--api-key` for a key; `echo -n "$KEY" | moca login anthropic` stores silently on a pipe). A stored key is used ahead of any `apiKey` env reference, and `moca logout <provider>` (or the TUI `/logout`) clears it. `moca login openai` opens the browser for consent (SSH/headless: it prints the URL and accepts a pasted code — `--no-browser` forces that mode); tokens are auto-refreshed under a cross-process lock. Set `"auth": "oauth"` for `providers.openai` to use the subscription — after a successful TUI sign-in the wizard offers to flip it for you. The subscription route serves the Responses API only, and model slugs must be available to your ChatGPT account — declare them under `providers.openai.models` when they are not in the built-in catalog.

## Use

```bash
moca                            # TUI in the current directory
moca -p "fix the failing test"  # one-shot; prompt also on stdin (-p -)
```

`-p` contract: stdout is the final answer text only; tool activity, retries and the token/cost summary go to stderr. Exit codes: `0` completed · `1` provider/runtime error · `2` config or usage error · `3` stopped at the step limit · `130` interrupted.

**TUI keys**

| Key | Action |
|---|---|
| `enter` | send |
| `shift+enter` (`alt+enter` / `ctrl+j` fallback) | newline |
| `esc` | interrupt the run |
| `ctrl+o` | pager on the latest item (full tool output / diff) |
| `alt+t` | pager on the latest thinking block (the whole reasoning chain) |
| `alt+p` | paste chips (large pastes collapsed, buffer intact) |
| `ctrl+c` ×2 | quit (or `/exit`) |
| `a` / `ctrl+a` / `d` | approval: allow once / allow always / deny |
| `↑`/`↓` · `tab` · `esc` | in the `/` dropdown: pick · complete · dismiss |

**Slash commands**: `/model` · `/effort` · `/hard` · `/yolo` · `/clear` · `/compact` · `/cost` · `/undo` · `/copy` · `/show <n>` · `/login` · `/logout` · `/help` · `/exit` (`/q`/`/quit`) — plus prompt templates (`~/.config/moca/prompts/<name>.md` becomes `/name`). Typing `/` opens a dropdown of every command and loaded template as you type — `↑`/`↓` to pick, `tab` to complete, `enter` on an exact name to run it. `!cmd` runs a command and feeds its output to the model; `!!cmd` runs it locally without telling the model. Typing during a run steers it after the current tool results.

## Extend

- **Skills** — `SKILL.md` directories in `~/.config/moca/skills/` (global) or `<repo>/.moca/skills/` (trusted projects). Any skill written for pi, Claude Code or OpenCode loads unchanged. See [docs/external-tools.md](docs/external-tools.md).
- **MCP** — `mcp.servers` in the config (stdio + streamable HTTP). Lazy by design: one fixed ~200-token `mcp` proxy tool, server tool lists never enter the prompt, servers start on first use and stop when idle. `moca mcp import` imports Claude Code / OpenCode / Pi server configs.
- **Project instructions** — `AGENTS.md` / `CLAUDE.md`, loaded only in trusted projects (`--approve`).
- **External tools** — `rtk` (token-compressed CLI output) and `graphify` (codebase knowledge graph) are allowlisted and ship built-in skills; see [docs/external-tools.md](docs/external-tools.md).

## Non-goals (v1, probably forever)

subagents · hooks · plan mode · LSP · web browsing · image gen · voice · telemetry · binary plugin system

## Development

```bash
mise install        # provisions Go 1.27.1 — pinned once, in go.mod
make build          # bin/moca, version-stamped from git describe
make test           # go test ./... -race -count=1
make release        # five cross-compiled binaries in dist/
```

`make` falls back to `mise x go -- go` when `go` is not on the PATH; with mise, the same targets run as `mise run build|test|vet|fmt|release|clean`.

## Phases

| # | Scope | Gate |
|---|-------|------|
| 1 | skeleton, 3 protocol adapters, streaming, retries, JSONC config | `-p 'hi'` streams via anthropic, opencode-go, openai; §12 example decodes intact — done |
| 2 | 7 tools + agent loop + permissions + yolo mode + skills + AGENTS.md + session schema | edit + shell-analysis ladders green, jail enforced, `--no-approve` honoured, `--yolo` lifts all checks — done |
| 3 | TUI shell + slash commands + status bar + steering + pager | full session in TUI, `/model` `/effort` `/hard` (cross-provider), `/undo` — done |
| 4 | context manager + compaction + resume | token-triggered compaction, `--continue` — done |
| 5 | MCP lazy proxy (stdio + streamable HTTP, persisted index, `mcp import`) | real server via proxy, no schemas in prompt, 0 servers at start — done |
| 6 | rtk + graphify + skills ecosystem compatibility | rtk preferred in real session, pi SKILL.md loads — done |
| 7 | OAuth providers + upstream graphify PR + v0.1 | ship-gate demo passes (§14); live legs + `v0.1.0` tag pending |

## Docs

[`docs/specs/SPECS.md`](docs/specs/SPECS.md) — the current implemented state · [`docs/specs/DESIGN.md`](docs/specs/DESIGN.md) — the v1 contract · [`docs/external-tools.md`](docs/external-tools.md) — rtk, graphify, skills, prompts.

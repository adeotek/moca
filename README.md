# moca — MO Coding Agent

[![CI](https://github.com/adeotek/moca/actions/workflows/ci.yml/badge.svg)](https://github.com/adeotek/moca/actions/workflows/ci.yml)

**A minimal, token-efficient, provider-agnostic coding agent: one Go binary, a Bubble Tea TUI, seven tools, no framework.**
Inspired by Claude Code, OpenCode and Pi — deliberately ~10% of their surface area. It competes on *cost per task* and *read-the-whole-codebase-in-an-hour transparency*, not features: prompt caching, windowed reads, diff-shaped results and token-denominated compaction keep a session cheap; the agent ships MCP support that never floods the prompt (one ~200-token lazy proxy), loads Agent-Skills `SKILL.md` files written for other tools unchanged, and prefers external token-savers (`rtk`, `graphify`) when they are installed.

**Status: v0.2.0-beta** — self-update, saved `/` commands and an Ollama provider; the §14 ship gate's live legs pass (evidence in SPECS §15).

## Install

```bash
go install github.com/adeotek/moca/cmd/moca@latest
```

**Install script** — downloads the release package, verifies it against the release's `checksums.txt`, and installs the binary — to `~/.local/bin` (`%LOCALAPPDATA%\Programs\moca` on Windows, added to your user PATH), or next to an existing `moca` when one is already on PATH:

```bash
# Linux
curl -fsSL https://raw.githubusercontent.com/adeotek/moca/main/install.sh | bash
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/adeotek/moca/main/install.ps1 | iex
```

Re-running the script is safe: an existing installation is updated in place, never an error. `--version vX.Y.Z` / `-Version vX.Y.Z` pins a release and `--dir <path>` / `-Dir <path>` picks the target directory (run the script directly for parameters). Or take a package from the [releases page](https://github.com/adeotek/moca/releases) — linux/amd64 · linux/arm64 · windows/amd64 are published by a manual **Release** workflow run (Actions → Release → Run workflow); other platforms (e.g. darwin) build from source with `make release`.

**Self-update** — an installed moca updates itself from the same releases:

```bash
moca update           # replace the binary with the latest release package for this platform
moca update --check   # only report whether a newer release exists
```

`moca update` verifies the package against the release's `checksums.txt` when published, then swaps the binary in place (atomic) — your config, credentials and sessions are never touched, and a running session keeps working (the new version starts on the next launch).

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

Optional TUI knob: `"tui": { "notify": "osc9" }` (`osc9` · `bell` · `off`) — a desktop notification (or bell) when a run longer than 30 s ends or an approval is waiting, only while the terminal is unfocused.

Secrets never appear as literals: `apiKey` is an optional `env:VAR` reference — or drop it and store the credential with `/login` (TUI) / `moca login` below. A stored key wins over the env reference.

### Providers

| Provider | Auth | Notes |
|---|---|---|
| `opencode-go` | `OPENCODE_API_KEY` | OpenCode Zen (mixed-protocol catalog: glm-5.3, minimax-m3, kimi-k3, …) |
| `anthropic` | `ANTHROPIC_API_KEY` | Claude; subscription OAuth is **not permitted** by Anthropic's terms for third-party clients — API key only |
| `openai` | `OPENAI_API_KEY` **or** `moca login openai` | ChatGPT Plus/Pro subscription login via OpenAI's "Sign in with ChatGPT" open-source flow |
| `ollama` | none | local or LAN [Ollama](https://ollama.com) server — models are discovered from it; see below |
| any name | `baseUrl` + `protocol` | custom providers: vLLM, LM Studio, OpenAI-compatible gateways |

#### Ollama (local or LAN)

```jsonc
{
  "model": "ollama/qwen3:8b",            // naming an ollama/… model is enough to enable it
  "providers": {
    "ollama": {}                         // same machine, default port: nothing else needed
    // "ollama": { "baseUrl": "http://nas.lan:11434" }       // another machine on the LAN
    // "ollama": { "models": { "qwen3:8b": { "contextWindow": 32768 } } }
  }
}
```

No API key and no model list: moca asks the server (`/api/tags`, `/api/show`) which models it has and uses the ones that support tool calling — `/model` re-reads them, so a model you `ollama pull` meanwhile shows up. The address is `providers.ollama.baseUrl`, else `$OLLAMA_HOST` (as for the `ollama` CLI), else `localhost:11434`; `nas`, `nas:11434` and `http://nas:11434` all work. For another machine, start *its* server listening on the network (`OLLAMA_HOST=0.0.0.0 ollama serve`). A key (`/login ollama`) is only for a server behind an authenticating proxy.

**Set the server's context length.** Ollama's OpenAI-compatible endpoint cannot choose the context window per request, so a window smaller than the conversation silently truncates the prompt. Start the server with a window of at least 16K — `OLLAMA_CONTEXT_LENGTH=32768 ollama serve` — and, if the model's Modelfile does not state `num_ctx`, tell moca the same number with `providers.ollama.models.<id>.contextWindow`. moca warns at startup when it had to guess (assuming 16384) or finds a window too small to work in. If the server is down, moca says so and how to start it, immediately.

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
| `esc` | interrupt the run (or a running `!cmd` / `/compact`) |
| `ctrl+o` | pager on the latest item (full tool output / diff), or on the full command of a pending approval |
| `alt+t` | pager on the latest thinking block (the whole reasoning chain) |
| `alt+p` | paste chips (large pastes collapsed, buffer intact) |
| `ctrl+r` | search prompt history (type to filter · `enter` uses the match, doesn't send) |
| `shift+tab` | cycle the effort level |
| `@` + path | complete a workdir file (`tab`/`enter` inserts it) |
| `ctrl+c` ×2 | quit (or `/exit`) |
| `a` / `ctrl+a` / `d` | approval: allow once / allow always / deny |
| `↑`/`↓` · `tab` · `esc` | in the `/` dropdown: pick · complete · dismiss |

**Slash commands**: `/model` · `/effort` · `/hard` · `/yolo` · `/clear` · `/resume` · `/compact` · `/cost` · `/undo` · `/copy` · `/show <n>` · `/login` · `/logout` · `/help` · `/exit` (`/q`/`/quit`) — plus prompt templates (`~/.config/moca/prompts/<name>.md` becomes `/name`). Typing `/` opens a dropdown of every command and loaded template as you type — `↑`/`↓` to pick, `tab` to complete, `enter` on an exact name to run it — and the command echoes into the transcript like your messages before its output. The first run drops a starter **`/create-command`** template into that directory: run it and the agent writes or updates the command for you — personal commands in `~/.config/moca/prompts/` (moca asks you to approve a write outside the workdir, once, never persistently) or project ones in `.moca/prompts/`. New commands are usable the moment the run that wrote them finishes — no restart. `!cmd` runs a command and feeds its output to the model; `!!cmd` runs it locally without telling the model. Typing during a run steers it after the current tool results.

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
| 7 | OAuth providers + upstream graphify PR + v0.1 | ship-gate demo passes (§14), live legs green (2026-10-07) — shipped as `v0.1.1-alpha`; current line `v0.2.0-beta` |

## Docs

[`docs/specs/SPECS.md`](docs/specs/SPECS.md) — the current implemented state · [`docs/specs/DESIGN.md`](docs/specs/DESIGN.md) — the v1 contract · [`docs/external-tools.md`](docs/external-tools.md) — rtk, graphify, skills, prompts.

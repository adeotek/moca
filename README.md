# moca — MO Coding Agent

Minimal, token-efficient, provider-agnostic coding agent. Single Go binary, Bubble Tea TUI, seven tools, no framework.

Inspired by Claude Code, OpenCode, and Pi — deliberately ~10% of their surface area. Competes on **cost per task** and **read-the-whole-codebase-in-an-hour** transparency, not features.

**Status: phase 1 implemented — `moca -p` streams via the anthropic-messages / openai-completions / openai-responses codecs; unit + mock-transport gates green, live provider smoke pending API keys.** Next: phase 2 (tools + agent loop). Plans: [`docs/plans/`](docs/plans/).

## Stack

- Go 1.27 (via mise) + Bubble Tea / lipgloss
- **Three providers from day 1**: `anthropic` (native + Claude subscription OAuth), `opencode-go` (OpenCode Zen, mixed-protocol catalog), `openai` (API + ChatGPT subscription OAuth) — plus optional per-provider `baseUrl` (any OpenAI-protocol endpoint: vLLM, LM Studio, Ollama)
- `model` + `modelHard` routing (`/hard` toggle), cheap default with explicit escalation; effort as a first-class request param (`/effort`); cross-provider model switching mid-session
- **MCP in v1, lazy by design** — one fixed ~200-token `mcp` proxy tool; server tool lists never enter the prompt ([DESIGN.md](docs/specs/DESIGN.md) §10.5)
- External tools (`rtk`, `graphify`) from day 1 via skills + slash commands + shell allowlist
- Project instructions via `AGENTS.md` (trust-gated); permissions are guard rails, not a sandbox ([DESIGN.md](docs/specs/DESIGN.md) §7); `--yolo` turns them all off (§7.5)

## Non-goals (v1, probably forever)

subagents · hooks · plan mode · LSP · web browsing · image gen · voice · telemetry · binary plugin system

## Phases

| # | Scope | Gate |
|---|-------|------|
| 1 | skeleton, 3 protocol adapters, streaming, retries, JSONC config | `-p 'hi'` streams via anthropic, opencode-go, openai; §12 example decodes intact |
| 2 | 7 tools + agent loop + permissions + yolo mode + skills + AGENTS.md + session schema | edit + shell-analysis ladders green, jail enforced, `--no-approve` honoured, `--yolo` lifts all checks |
| 3 | TUI shell + slash commands + status bar + steering + pager | full session in TUI, `/model` `/effort` `/hard` (cross-provider), `/undo` |
| 4 | context manager + compaction + resume | token-triggered compaction, `--continue` |
| 5 | MCP lazy proxy (stdio + streamable HTTP, persisted index, `mcp import`) | real server via proxy, no schemas in prompt, 0 servers at start |
| 6 | rtk + graphify + skills ecosystem compatibility | rtk preferred in real session, pi SKILL.md loads |
| 7 | OAuth providers + upstream graphify PR + v0.1 | ship-gate demo passes (§14) |

## Development

```bash
mise install        # go 1.27.1
go build ./...      # from phase 1 onward
go test ./...
```

# moca — MO Coding Agent

Minimal, token-efficient, provider-agnostic coding agent. Single Go binary, Bubble Tea TUI, seven tools, no framework.

Inspired by Claude Code, OpenCode, and Pi — deliberately ~10% of their surface area. Competes on **cost per task** and **read-the-whole-codebase-in-an-hour** transparency, not features.

**Status: phase 0 — DESIGN.md rev 8 (post-review fixes applied), awaiting lock.** No code until the doc is signed off.

## Stack

- Go 1.27 (via mise) + Bubble Tea / lipgloss
- **Three providers from day 1**: `anthropic` (native + Claude subscription OAuth), `opencode-go` (OpenCode Zen, mixed-protocol catalog), `openai` (API + ChatGPT subscription OAuth) — plus optional per-provider `baseUrl` (any OpenAI-protocol endpoint: vLLM, LM Studio, Ollama)
- `model` + `model_hard` routing (`/hard`), cheap default with explicit escalation; effort as a first-class request param (`/effort`)
- **MCP in v1, lazy by design** — one fixed ~200-token `mcp` proxy tool; server tool lists never enter the prompt (DESIGN.md §10.5)
- External tools (`rtk`, `graphify`) from day 1 via skills + slash commands + shell allowlist

## Non-goals (v1, probably forever)

subagents · hooks · plan mode · LSP · web browsing · image gen · voice · telemetry · binary plugin system

## Phases

| # | Scope | Gate |
|---|-------|------|
| 1 | skeleton, protocol adapters, streaming, JSONC config | `-p 'hi'` streams via anthropic AND opencode-go; §12 example parses intact |
| 2 | 7 tools + agent loop + permissions + skills + session schema | edit-ladder 10/10, jail enforced, trust prompt |
| 3 | TUI shell + slash commands + status bar + steering | full session in TUI, `/model` `/effort` `/hard` |
| 4 | context manager + compaction + resume | token-triggered compaction, `--continue` |
| 5 | MCP lazy proxy (stdio + streamable HTTP, `mcp import`) | real server via proxy, no schemas in prompt |
| 6 | rtk + model_hard routing + graphify compatibility | rtk preferred in real session, pi SKILL.md loads |
| 7 | OAuth providers + upstream graphify PR + v0.1 | ship-gate demo passes (§14) |

## Development

```bash
mise install        # go 1.27.1
go build ./...      # from phase 1 onward
go test ./...
```

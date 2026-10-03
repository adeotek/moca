# moca — MO Coding Agent

Minimal, token-efficient, provider-agnostic coding agent. Single Go binary, Bubble Tea TUI, six tools, no framework.

Inspired by Claude Code, OpenCode, and Pi — deliberately ~10% of their surface area. Competes on **cost per task** and **read-the-whole-codebase-in-an-hour** transparency, not features.

**Status: phase 0 — DESIGN.md rev 2 (steering applied), awaiting lock.** No code until the doc is signed off.

## Stack

- Go 1.27 (via mise) + Bubble Tea / lipgloss
- **Three providers from day 1**: `anthropic` (native + Claude subscription OAuth), `opencode-go` (OpenCode Zen, mixed-protocol catalog), `openai` (API + ChatGPT subscription OAuth)
- `model` + `model_hard` routing (`/hard`), cheap-default with explicit escalation
- External tools (`rtk`, `graphify`) from day 1 via skills + slash commands + shell allowlist

## Non-goals (v1, probably forever)

MCP · subagents · hooks · plan mode · LSP · web browsing · telemetry · binary plugin system

## Phases

| # | Scope | Gate |
|---|-------|------|
| 1 | skeleton, protocol adapters, streaming | `-p 'hi'` streams via anthropic AND opencode-go |
| 2 | 6 tools + agent loop + permissions + skills | edit-ladder 10/10, jail enforced |
| 3 | TUI shell + slash commands | full session in TUI, `/model` `/hard` |
| 4 | context manager + sessions | compaction at 80%, resume, live cost |
| 5 | rtk + model_hard + graphify compatibility | rtk preferred in real session, pi SKILL.md loads |
| 6 | OAuth providers + upstream graphify PR + v0.1 | ship-gate demo passes |

## Development

```bash
mise install        # go 1.27.1
go build ./...      # from phase 1 onward
go test ./...
```

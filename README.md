# moca — MO Coding Agent

Minimal, token-efficient, provider-agnostic coding agent. Single Go binary, Bubble Tea TUI, six tools, no framework.

Inspired by Claude Code, OpenCode, and Pi — deliberately ~10% of their surface area. Competes on **cost per task** and **read-the-whole-codebase-in-an-hour** transparency, not features.

**Status: phase 0 — design contract under review.** See [DESIGN.md](DESIGN.md). No code until the doc is signed off.

## Stack

- Go 1.27 (via mise) + Bubble Tea / lipgloss
- Providers: Anthropic (native) + OpenAI-compatible endpoint for v1

## Non-goals (v1, probably forever)

MCP · subagents · hooks · plan mode · LSP · web browsing · telemetry

## Phases

| # | Scope | Gate |
|---|-------|------|
| 1 | skeleton, config, Anthropic provider, streaming | `moca -p 'hi'` streams |
| 2 | 6 tools + agent loop | edit ladder tests green |
| 3 | TUI shell | full session in TUI |
| 4 | context manager + sessions | budget holds, resume works |
| 5 | permissions + OpenAI-compat + skills | allowlist enforced |
| 6 | polish + v0.1 | ship-gate demo passes |

## Development

```bash
mise install        # go 1.27.1
go build ./...      # from phase 1 onward
go test ./...
```

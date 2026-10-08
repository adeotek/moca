# Changelog

## v0.1.1 (2026-10-08)

Polish, a credential store in the config dir, `/` autocomplete, and a bigger catalog — still one binary, still no framework.

### Credentials & login (TUI `/login`, API keys for `moca login`)

- **`/login` in the TUI** — pick a provider, then paste an API key (masked, never echoed, never written to the transcript) or sign in to a subscription in the browser; while a sign-in is in flight the pasted redirect URL/code from the input box feeds the flow and `esc` cancels it. After a successful ChatGPT sign-in the wizard offers to flip `providers.openai.auth` to `"oauth"` for you. **`/logout`** revokes a subscription session best-effort and clears the stored token/API key.
- **Credentials live in `~/.config/moca/auth.json`** (0600) — beside the config file now, not the data dir (a v0.1 store is migrated automatically on first run); API keys and OAuth tokens share the file.
- **`apiKey` is optional in the config**: a key stored with `/login` or the CLI wins over the `env:VAR` reference, which stays as the fallback — an env var set by an external process still works, and `/logout` restores it. The stored key applies from the running session's next request.
- **`moca login <provider>` stores API keys too**: providers without an OAuth flow (e.g. `anthropic` — the recorded policy reason is printed, then the key prompt) prompt for one; the prompt shows only on a terminal, so `echo -n "$KEY" | moca login anthropic` stores silently. `--api-key` forces the key path for `openai`; `moca logout <provider>` clears token and key.

### TUI polish

- **`/` autocomplete dropdown** — typing `/` lists every built-in command and loaded prompt template (with one-line hints), filtered as you type; `↑`/`↓` pick, `tab` completes, `enter` on an exact name runs it, `esc` dismisses. Built-ins win name collisions with templates.
- Two-line status bar from the first frame: `version · cwd · branch` and `provider/model · effort · ctx % · in/out · cost`.
- Full-width rules around the input area (no prompt prefix), compact welcome lines, full-row backgrounds for submitted messages and assistant responses, muted tool/thinking lines, `/exit` (`/q`/`/quit` aliases).
- Shrinking frames can no longer strand stale rows in the scrollback when keystrokes coalesce into one rendered frame (the inline-renderer shrink guard now remembers the last frame the renderer actually flushed).

### Catalog & tooling

- +10 `opencode-go` models (deepseek v4.1/v4 flash & pro, kimi-k2.7-code, mimo-v2.6, qwen3.8 & qwen3.7), all live-verified; Go version pinned once in `go.mod`; the `make` targets mirrored as `mise run` tasks; `make vet` now includes the `gofmt` check; documentation trimmed to the two living specs.

## v0.1.0-alpha (2026-10-07)

First release. A minimal, token-efficient, provider-agnostic coding agent: one Go binary, a Bubble Tea TUI, seven tools, no framework.

### Providers & protocols

- Three protocol adapters, hand-rolled HTTP+SSE, no SDKs: `anthropic-messages`, `openai-completions`, `openai-responses`.
- Built-in providers: `anthropic` (API key; subscription OAuth is not permitted by Anthropic's terms), `opencode-go` (OpenCode Zen, mixed-protocol catalog incl. the Go-tier session header), `openai` (API key + ChatGPT subscription login). Custom providers via `baseUrl` + `protocol` (vLLM, LM Studio, Ollama, any OpenAI-compatible gateway).
- Retry/backoff with jitter honoring `retry-after`, SSE stall timeout, mid-stream failure rule; effort as a first-class request parameter with per-model clamping; cross-provider history transform (thinking replay bound to the producing model, tool-id normalization). Upstream failures the gateway labels `server_error` are retried even when framed as HTTP 400 (the opencode-go `[1210]` thinking-config flake); the default HTTP client bounds the response-header wait at 120 s so a silent provider cannot hang an unattended run.

### OAuth (subscription login)

- `moca login openai` / `moca logout openai`: OpenAI's "Sign in with ChatGPT" open-source token sharing — dynamic client registration (no secret), PKCE S256, loopback callback on `127.0.0.1`, nonce + RS256 ID-token validation against the vendor JWKS, rotating 30-day refresh tokens serialized across processes (0600 store at `~/.local/share/moca/auth.json`), SSH/headless copy-URL + paste-code fallback, best-effort revocation on logout.
- Anthropic ships API key only: the phase-7 policy gate verified the current terms prohibit third-party use of consumer-plan OAuth (`docs/specs/oauth-verification.md`); config validation rejects `"auth": "oauth"` for it with the recorded reason.

### Tools & permissions

- Seven frozen tools: `read`, `write`, `edit` (exact-match-first with one whitespace-tolerant fallback, re-indentation, CRLF/BOM preservation), `shell` (stateless, `bash -c` / `pwsh -NoProfile`, process-group kill), `search` (pure-Go, ripgrep semantics, `.gitignore`-aware), `ls`, and the lazy `mcp` proxy.
- Path jail anchored at the session workdir (symlink-resolved, read-only skill roots); shell command analysis via `mvdan.cc/sh` with hard-deny/ask ladders, compound-command checking and `rtk` unwrapping; project trust (`--approve`/`--no-approve`, `trust.json`); yolo mode (`--yolo`, `/yolo`) turns every check off between runs.
- Pre-edit snapshots + `/undo` (skipped for clean tracked files — git is the undo there).
- Failing-test investigation protocol: while a failing test run is unresolved, every tool result states the next step (read the failing test with the `read` tool; locate the cause with `search`), and `edit` refuses until both happened — the §14 loop's investigation steps, enforced rather than advisory.

### TUI

- Bubble Tea TUI with a bottom status bar from the first frame: cwd · git branch · provider/model · effort · context size & usage % · session tokens · cost (`sub` on subscription turns) · transient `retry n/m` and `YOLO` states.
- Multi-line input with bracketed paste (collapsed chips), `shift+enter` newline via enhanced keyboard reporting with fallbacks, steering during runs, `!`/`!!` shell prefixes, approvals (`a` once / `ctrl+a` always / `d` deny), pager (`ctrl+o`), `/copy` (OSC 52), slash commands `/model /effort /hard /yolo /clear /compact /cost /undo /copy /show /help` plus prompt templates.

### Context & sessions

- Token-denominated context manager: reserve/keepRecent budgets, usage-anchored estimates, cut-point rules that never orphan a tool call, structured summaries with cumulative file tracking, overflow compact-and-retry, no-cache-write summary requests.
- JSONL session transcripts (single-writer `flock`, 0600) with entry ids, thinking blocks, model changes and compaction entries; `--resume <id8|last>` and `--continue`.

### MCP

- Lazy MCP proxy: stdio + streamable HTTP transports, persisted discovery index, `search`/`describe`/`call` in one ~200-token tool — server tool lists never enter the prompt, servers start on first call and stop after idle. `moca mcp import` imports Claude Code / OpenCode / Pi server configs (secrets rewritten to `env:` references).

### External tools & the skills ecosystem

- `rtk` and `graphify` in the default allowlist, with built-in guidance skills; the shell analyser unwraps `rtk <cmd>` so it cannot launder a non-allowlisted command.
- Agent-Skills compatibility: `SKILL.md` files written for pi, Claude Code and OpenCode (graphify's included) load unchanged — proven by a vendored corpus with provenance and SHA-256 enforcement; prompt templates from Claude Code command files / pi templates drop in.
- `graphify install --platform moca` contributed upstream ([Graphify-Labs/graphify#4174](https://github.com/Graphify-Labs/graphify/pull/4174)).

### CLI

- `moca` (TUI), `moca -p` (one-shot, stdout = final text, exit codes 0/1/2/3/130), `--model`, `--effort`, `--approve`/`--no-approve`, `--yolo`/`--no-yolo`, `--resume`/`--continue`, `--version`, `--help`, `moca login`/`logout`, `moca mcp import`/`index`.
- `Makefile`: version-stamped builds (`git describe` → `-ldflags`), `make test` (`-race`), `make release` (five cross-compiled binaries).

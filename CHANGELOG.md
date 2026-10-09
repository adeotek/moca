# Changelog

## v0.3.0-beta (2026-10-09)

First-run setup mode and a web tool: a config without a model opens the TUI with `/login` + `/model` guidance instead of stopping, and the agent can fetch pages and search the web — still one binary, still no framework.

### Web

- **A `web` tool — fetch and search, no API key needed.** `fetch` GETs a URL and returns it as readable markdown (the page title becomes the headline), plain text or raw html; `search` returns ranked results with snippets. Search works out of the box via [Tavily](https://tavily.com)'s keyless mode — set `web.search.apiKey` (an `env:` reference like `env:TAVILY_API_KEY`) to lift the rate limit, or `web.search.provider: "exa"` with a key. Fetching is bounded like every other tool (5 redirects, 2 MiB, 60 K chars, 30 s default), and — like `curl` — needs no approval: it only reads. Fetched pages are untrusted data, never instructions.

### Plan mode

- **`--plan` / `/plan` — the agent writes the plan, not the code.** A plan-mode run analyzes the request and the codebase read-only, then writes an implementation plan to `docs/plans/<slug>.md` (goal · current state · `- [ ]` steps · verification · out of scope · risks) and stops — the shell, MCP calls and every write outside `docs/plans/*.md` are refused, so nothing else can change. `moca --plan -p "<request>"` for one-shot; `/plan` toggles it in the TUI (blue `PLAN` status field) and `/plan <request>` plans immediately. A run that would finish without the file is nudged once, then warned. Plan mode is a scope, not a permission — `--yolo` does not lift it.

### TUI

- **A fresh install opens the TUI instead of exiting** — with no `"model"` configured, `moca` used to stop with `moca: no model configured — set "model" in …`. Now it opens in *setup mode*: a red notice (`no provider configured — run /login first (store an API key or sign in), then /model picks a model`) rides the welcome print, the status bar carries `no provider configured — /login`, and sending a message repeats the notice instead of doing nothing. `/login` stores a credential, then `/model` opens the first-run picker — choosing a model checks its credential (a missing key says so and names `/login`), starts the session immediately and saves the choice as the config's `"model"`, so the next launch starts straight into a session. `/help`, `/exit`, `/resume`, `/sessions` and `!`/`!!` keep working throughout. `-p` without a model keeps the old error, and `--resume`/`--continue` need no configured model.

## v0.2.0-beta (2026-10-09)

Ollama support, a TUI review pass with saved `/` commands, and self-update from GitHub releases — still one binary, still no framework.

### Ollama (local or LAN)

- **New provider `ollama`** — a local or LAN [Ollama](https://ollama.com) server, no API key. Enable it by naming a model (`"model": "ollama/qwen3:8b"`, `--model ollama/…`) or with `"providers": { "ollama": {} }`; `baseUrl` or `$OLLAMA_HOST` points it at another machine. Models are discovered from the server (embedding and tool-less models are skipped; `thinking` models get effort levels) and `/model` re-reads them; models declared under `providers.ollama.models` override or add. See the README for the context-length caveat — moca warns when it has to assume the window.
- A server that is down fails at once with how to start it (no retry ladder); a model that was never pulled says `ollama pull <name>`. A key (`/login ollama`) is optional, for servers behind an authenticating proxy. Nothing is probed unless you opt in.

### TUI features

- **`/create-command`** — a starter command template, written on the first TUI run into `~/.config/moca/prompts/` (an existing file is never overwritten): run it and the agent creates or updates a saved slash command for you — personal ones in that directory (it asks you to approve the write, since it is outside the workdir — once per write, never persisted) or project ones in `.moca/prompts/`; new commands are usable as soon as the run finishes — no restart.
- **Markdown-lite responses** — `**bold**`, `` `code` ``, headings, bullets, quotes and fenced code blocks (a darker band, never word-wrapped) instead of raw markdown; `/copy` still copies the raw text.
- **Inline edit diffs** — a successful `edit` prints its changed lines (red/green, ≤ 8) under its item line.
- **Resume picks up where you left off** — `--resume`/`--continue` replay the last 3 turns into the scrollback; new **`/resume`** picker (this directory's other sessions) or `/resume <id8>` (an id from another directory is refused).
- **`/sessions`** — manage this directory's stored sessions in one place: every session listed (the open one marked `(current)`), `enter` switches to it, `ctrl+d` deletes it after a `y` confirm — the open session and sessions held open by another moca process refuse, and a file under a live writer is never unlinked; the list stays open for the next one. The newest 100 are listed; the title gives the real total when there are more.
- **`/model` picker** (providers without a key are marked) and **argument completion** in the `/` dropdown for `/model`, `/effort`, `/login`, `/logout`, `/show`, `/resume`.
- **`@path`** completes workdir files (gitignore-aware; the path is sent, the model reads the file itself).
- **Persistent prompt history** (`history.jsonl`, per workdir) and **`ctrl+r`** history search.
- **Context pressure** in the status bar: the percent turns yellow at 70%, red (with `/compact`) where auto-compaction fires.
- Richer activity row (tool operand, streamed-token estimate, queued steering count); pager search (`/`, `n`/`N`) and an "approval pending" marker; `shift+tab` cycles effort.
- Desktop notification / bell for a finished long run or a waiting approval while the terminal is unfocused (`tui.notify`, default `osc9`).
- Calmer layout: a blank row between tool items and the response, `›` messages hang their text, item lines stay on one row, `!cmd` echoes like other input, and the shift+enter hint shows once per machine.
- Scrollback colors follow the terminal's profile: `NO_COLOR` is honored, and 256-color terminals get approximated band colors instead of raw truecolor.

### TUI fixes

- **Approvals show the whole command** — up to 6 lines under the question (overflow counted), and `ctrl+o` pages all of it; the prompt used to show only the first line, so approving `python3` hid the heredoc it would run.
- **`esc` kills a running `!cmd`**, which now gets the activity row (spinner, elapsed time) like runs and `/compact` (`compacting… · esc to cancel`).
- **`ctrl+c` on an empty draft says `press ctrl+c again to quit`**; inside the pager it now also clears the textarea (the draft used to come back on the next key).
- **Exit** keeps the final status bar on screen and prints `session <id8> · resume with: moca --resume <id8>`.
- The status bar shows the branch of a fresh `git init` (no commit yet) instead of `-`.
- `/cost` uses the bar's compact token format and breaks the input down (fresh · cache read · cache write); `subscription` for OAuth.
- `/copy` keeps the last answer when the final turn had no text; failed tool items get a red `✗`; `thinking 1 line` (was `1 lines`); the pager title is clamped to one row and shows the scroll position.

### Self-update & release packages

- **Install scripts** — `install.sh` (Linux) and `install.ps1` (Windows) download the release package, verify it against the release's `checksums.txt`, and install `moca` to `~/.local/bin` (`%LOCALAPPDATA%\Programs\moca` on Windows, added to the user PATH) or next to an existing installation; re-running updates in place and never fails because moca is already installed (`--version`/`-Version` pins a release, `--dir`/`-Dir` picks the directory).
- **`moca update`** — replace the installed binary with the latest GitHub release package. It checks the newest published release (drafts skipped), picks the package for your platform (linux/amd64 · linux/arm64 · windows/amd64), verifies it against the release's `checksums.txt`, and swaps the binary in place (atomic; a symlinked launcher has its target replaced; sessions, config and credentials are never touched — the new version takes over on the next launch). `moca update --check` only reports. A development or `-dirty` build, or one newer than the release, reports "not newer"; a platform without a package (e.g. darwin — use `make release`) and an unparseable version are clear errors. It runs before the config is loaded, so a broken config cannot block an update.
- **Release workflow** — a manual run of the new **Release** action (Actions → Release → Run workflow; pushing a `v*` tag deliberately does not trigger it) builds and publishes the linux/amd64, linux/arm64 and windows/amd64 packages plus `checksums.txt` as a GitHub Release — exactly what `moca update` consumes; re-running replaces the release's assets in place, and a run from an **untagged** ref is a rehearsal: it creates a **draft** release that `moca update` and the installers ignore until you publish it by hand.

## v0.1.1 (2026-10-08)

Polish, a credential store in the config dir, `/` autocomplete, and a bigger catalog — still one binary, still no framework.

### Credentials & login (TUI `/login`, API keys for `moca login`)

- **`/login` in the TUI** — pick a provider, then paste an API key (masked, never echoed, never written to the transcript) or sign in to a subscription in the browser; while a sign-in is in flight the pasted redirect URL/code from the input box feeds the flow and `esc` cancels it. After a successful ChatGPT sign-in the wizard offers to flip `providers.openai.auth` to `"oauth"` for you. **`/logout`** revokes a subscription session best-effort and clears the stored token/API key.
- **Credentials live in `~/.config/moca/auth.json`** (0600) — beside the config file now, not the data dir (a v0.1 store is migrated automatically on first run); API keys and OAuth tokens share the file.
- **`apiKey` is optional in the config**: a key stored with `/login` or the CLI wins over the `env:VAR` reference, which stays as the fallback — an env var set by an external process still works, and `/logout` restores it. The stored key applies from the running session's next request.
- **`moca login <provider>` stores API keys too**: providers without an OAuth flow (e.g. `anthropic` — the recorded policy reason is printed, then the key prompt) prompt for one; the prompt shows only on a terminal, so `echo -n "$KEY" | moca login anthropic` stores silently. `--api-key` forces the key path for `openai`; `moca logout <provider>` clears token and key.

### TUI polish

- **`/` autocomplete dropdown** — typing `/` lists every built-in command and loaded prompt template (with one-line hints), filtered as you type; `↑`/`↓` pick, `tab` completes, `enter` on an exact name runs it, `esc` dismisses. Built-ins win name collisions with templates.
- **Commands echo as messages** — a submitted `/` command prints in the scrollback with the same `›` band as user messages, then its output (refusals included).
- **Welcome line**: the `moca` product name renders in orange (`#d97706`).
- **Thinking blocks** now print their item line when the block ends — before the response or tool item that follows, not after — and show how long they streamed (`⋯ #6 thinking 8 lines · 12s`).
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

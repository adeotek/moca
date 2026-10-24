# Mo Coding Agent (moca)

[![CI](https://github.com/adeotek/moca/actions/workflows/ci.yml/badge.svg)](https://github.com/adeotek/moca/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/adeotek/moca)](https://github.com/adeotek/moca/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**One Go binary. Eight tools. No framework.** A small, fast, provider-agnostic coding agent for the terminal: it speaks the Agent Client Protocol, so Zed, VS Code and Paseo can drive it; it loads the `SKILL.md` skills you already have, unchanged; and it publishes its benchmark numbers — including the runs it loses ([BENCHMARKS.md](docs/BENCHMARKS.md)).

moca is a coding agent for the terminal, inspired by Claude Code, OpenCode and Pi — deliberately about a tenth of their surface area. Prompt caching, windowed reads, diff-shaped results and token-denominated compaction keep a session cheap, and everything moca does ends up in plain session files you can read. It runs on Claude, ChatGPT (subscription login included), Ollama, OpenCode Zen, or any OpenAI-compatible endpoint. The agent is **Mo**; `moca` is the binary you run.

![moca: a prompt, a tool call, the approval picker, a diff and a green test run](docs/demo/moca-demo.gif)

- **Eight frozen tools** — `read` · `write` · `edit` · `shell` · `search` · `ls` · `web` (search + fetch) · `mcp`. Stable schemas by design; everything else is a [non-goal](#non-goals-v1-probably-forever).
- **MCP without prompt bloat** — one lazy ~200-token proxy tool; servers start on first use and their tool lists never enter your prompt.
- **Skills from other tools, unchanged** — `SKILL.md` skills written for pi, Claude Code or OpenCode load as-is, and every skill you add is invokable from the TUI as `/name` (agent-recorded `auto` notes are listed but never commands).
- **Web search + fetch built in** — keyless by default, no extra tool or API key needed.
- **Permission modes** — `default` / `auto` / `plan` / `yolo`, per run or per project.
- **Editor support** — `moca acp` speaks the Agent Client Protocol, so Zed, VS Code and Paseo can drive it.

**46 s median agent-run** on Terminal-Bench 2.0, the fastest of the four agents measured · **10/12** on that 12-task subset at moca's best, **8/12** at the newest run — the number moves, and every run is in [BENCHMARKS.md](docs/BENCHMARKS.md) with the losses named.

### Why not Claude Code, OpenCode or pi?

If you are short of accuracy on hard tasks today, use one of them: on the same 12 tasks pi solved 11/12 where moca solved 6/12 (moca v0.5.1-beta; its own best since is 10/12), and Claude Code brings Anthropic's own model. moca's case is the shape of the thing — one static binary with no runtime, eight tools whose schemas do not change under you, ~200 tokens of MCP no matter how many servers you have, skills that migrate unchanged, sessions you can read — and a codebase you can read in an afternoon. If that is not what you are short of, the others are better at what they do, and [the numbers say so](docs/BENCHMARKS.md).

## Install

One command, no toolchain. The installer downloads the release package for your platform, verifies it against the release's `checksums.txt`, and installs to `~/.local/bin` (`%LOCALAPPDATA%\Programs\moca` on Windows, added to your user PATH), or next to an existing `moca` when one is already on `PATH`.

| | |
|---|---|
| **macOS / Linux** | `curl -fsSL https://raw.githubusercontent.com/adeotek/moca/main/install.sh \| bash` |
| **macOS / Linux (Homebrew)** | `brew install adeotek/tap/moca` |
| **Windows** | `irm https://raw.githubusercontent.com/adeotek/moca/main/install.ps1 \| iex` |
| **Any (with Go)** | `go install github.com/adeotek/moca/cmd/moca@latest` |

Re-running is safe — an existing installation is updated in place, never an error. `--version vX.Y.Z` / `-Version` pins a release, `--dir <path>` / `-Dir` picks the target directory, `--help` / `-?` prints usage. Packages for **linux, darwin and windows on amd64 and arm64** are on the [releases page](https://github.com/adeotek/moca/releases), published by a manual **Release** workflow run (Actions → Release → Run workflow).

**Homebrew** — [`adeotek/homebrew-tap`](https://github.com/adeotek/homebrew-tap) installs the same prebuilt tarball (`brew install adeotek/tap/moca`), rendered from the release's `checksums.txt` so the formula and the release cannot disagree. That copy updates with `brew upgrade moca`, not `moca update` — see below.

**Self-update** — an installed moca updates itself from the same releases:

```bash
moca update           # replace the binary with the latest release package for this platform
moca update --check   # only report whether a newer release exists
```

`moca update` verifies the package against the release's `checksums.txt` when published, then swaps the binary in place (atomic) — your config, credentials and sessions are never touched, and a running session keeps working (the new version starts on the next launch).

A copy that a **package manager** installed is left alone: Homebrew, Scoop and WinGet own their own trees, so `moca update` names the manager's upgrade command and exits 2 instead of fighting it (`--force` overrides, for a copy you really do maintain by hand). `--check` stays read-only everywhere. The install scripts refuse the same way when the `moca` on `PATH` is a package manager's, before downloading anything — they name the manager's upgrade (or uninstall) command; `--dir` / `-Dir` installs a separate copy wherever you point it.

### Inspecting the config

```bash
moca config sample    # print a complete, commented user config (every option, defaults shown)
moca config check     # validate your config and the project's .moca/config.jsonc, listing every issue
```

`moca config sample` is the full reference: copy it to `~/.config/moca/config.jsonc` and edit — the active values are the built-in defaults, so an unchanged copy behaves identically. `moca config check` reports unknown keys, malformed JSONC, rule syntax and `line:col` for each issue (exit 2 when there are any); the project file is only read for a trusted workdir and says so when it isn't.

Each version extracts its built-in skills to its own directory under `~/.local/share/moca/builtin-skills/` and keeps the older ones, so a moca still running from the previous version is not pulled out from under. Once nothing older is running, `moca prune` removes every one but the current version's (`moca prune --dry-run` lists them first); sessions, snapshots, history and your config are never touched.

## Uninstall

Remove the binary (the config, data and state directories are kept):

```bash
# macOS and Linux
curl -fsSL https://raw.githubusercontent.com/adeotek/moca/main/install.sh | bash -s -- --uninstall
```

```powershell
# Windows (PowerShell)
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/adeotek/moca/main/install.ps1))) -Uninstall
```

Full uninstall — the binary plus `~/.config/moca`, `~/.local/share/moca` and `~/.local/state/moca` (config, credentials, sessions, logs; `$XDG_CONFIG_HOME`/`$XDG_DATA_HOME`/`$XDG_STATE_HOME` are honored): pass `--remove-data` / `-RemoveData` instead. Both scripts refuse to delete anything whose `--version` output does not identify it as moca.

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

**First run** — with no `"model"` configured, `moca` opens the TUI with a red notice instead of stopping: `/login` stores an API key or signs in, then `/model` picks a model and starts the session (your choice is saved here as `"model"`). `-p` still needs a model up front.

Optional TUI knobs: `"tui": { "notify": "osc9" }` (`osc9` · `bell` · `off`) — a desktop notification (or bell) when a run longer than 30 s ends or an approval is waiting, only while the terminal is unfocused — and `"tui": { "theme": { … } }` — every color the TUI draws, per role: `both`, `dark` and `light` groups (`dark`/`light` follow the terminal’s reported background, `both` fills whichever applies and loses to the variant’s own key), each value a `#rrggbb` hex or an ANSI color number `0`–`255`; an unset role keeps the built-in palette (`internal/tui/theme.go`), and the block is user-only — a project’s `.moca/config.jsonc` cannot carry colors. Tune it in [docs/themes/tui-theme-builder.html](docs/themes/tui-theme-builder.html), a live mock that emits the block.

Web search is keyless by default; to switch engines or lift tavily's rate limit: `"web": { "search": { "provider": "tavily", "apiKey": "env:TAVILY_API_KEY" } }` — `tavily` (default) or `exa` (a key is required). Fetching pages needs no config.

Secrets never appear as literals: `apiKey` is an optional `env:VAR` reference — or drop it and store the credential with `/login` (TUI) / `moca login` below. A stored key wins over the env reference.

### Providers

| Provider | Auth | Notes |
|---|---|---|
| `opencode-go` | `OPENCODE_API_KEY` | OpenCode Go tier (mixed-protocol catalog: glm-5.3, minimax-m3, kimi-k3, …) |
| `opencode-zen` | `OPENCODE_API_KEY` | OpenCode Zen, pay-as-you-go — same account key as the go tier (override with `providers.opencode-zen.apiKey`) |
| `openrouter` | `OPENROUTER_API_KEY` | one key, hundreds of models; ids keep their vendor prefix (`openrouter/anthropic/claude-sonnet-5.5`) |
| `anthropic` | `ANTHROPIC_API_KEY` | Claude; subscription OAuth is **not permitted** by Anthropic's terms for third-party clients — API key only |
| `openai` | `OPENAI_API_KEY` **or** `moca login openai` | ChatGPT Plus/Pro subscription login via OpenAI's "Sign in with ChatGPT" open-source flow |
| `ollama` | none | local or LAN [Ollama](https://ollama.com) server — models are discovered from it; see below |
| any name | `baseUrl` + `protocol` | custom providers: vLLM, LM Studio, OpenAI-compatible gateways |

**The model list comes from the provider.** Each provider's own `/models` endpoint is read at startup and cached for 24 h (one file per provider in `~/.local/share/moca/models-cache/`), so a model released yesterday shows up in `/model` without a moca release. Curated metadata (prices, thinking levels) still wins where it exists; a discovered model with no published price shows `—` rather than a fake `$0.00`, and one with no published window gets 16384 plus a startup warning. `/model` lists only providers you can actually run; a fresh install's first picker still shows everything, with a `/login <provider>` hint per row. `/clear-models-cache` empties the cache and re-reads the current model's provider at once — the session keeps every model it is using, and the other providers refetch on the next `/model`. A custom provider that declares its own models is used as declared.

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

No API key and no model list: moca asks the server (`/api/tags`, `/api/show`) which models it has and uses the ones that support tool calling — `/model` re-reads them, so a model you `ollama pull` meanwhile shows up. The address is `providers.ollama.baseUrl`, else `$OLLAMA_HOST` (as for the `ollama` CLI), else `localhost:11434`; `nas`, `nas:11434` and `http://nas:11434` all work. For another machine, start *its* server listening on the network (`OLLAMA_HOST=0.0.0.0 ollama serve`). A key (`/login ollama`) is only for a server behind an authenticating proxy. If the server is down, moca says so and how to start it, immediately.

**Set the server's context length.** Ollama's OpenAI-compatible endpoint cannot choose the context window per request, so a window smaller than the conversation silently truncates the prompt. Start the server with a window of at least 16K — `OLLAMA_CONTEXT_LENGTH=32768 ollama serve` — and, if the model's Modelfile does not state `num_ctx`, tell moca the same number with `providers.ollama.models.<id>.contextWindow`. moca warns at startup when it had to guess (assuming 16384) or finds a window too small to work in.

**Credentials live in `~/.config/moca/auth.json`** (0600, never committed). `/login` in the TUI: pick a provider, then paste the API key (masked, never echoed) or sign in to a subscription in the browser. On the CLI: `moca login <provider>` (`--api-key` for a key; `echo -n "$KEY" | moca login anthropic` stores silently on a pipe), and `moca logout <provider>` (or the TUI `/logout`) clears it. A stored key is used ahead of any `apiKey` env reference.

`moca login openai` opens the browser for consent (SSH/headless: it prints the URL and accepts a pasted code — `--no-browser` forces that mode); tokens are auto-refreshed under a cross-process lock. Set `"auth": "oauth"` for `providers.openai` to use the subscription — after a successful TUI sign-in the wizard offers to flip it for you. The subscription route serves the Responses API only, and model slugs must be available to your ChatGPT account — declare them under `providers.openai.models` when they are not in the built-in catalog.

## Editors (ACP)

`moca acp` speaks the [Agent Client Protocol](https://agentclientprotocol.com) on stdio, so ACP editors can drive moca. It uses your existing config and credentials (`moca login` in a terminal first) — the agent advertises one `Sign in` method, and answering it re-checks that credential and reports moca's own fix when there is none.

```jsonc
// Zed — settings.json
"agent_servers": { "moca": { "command": "moca", "args": ["acp"] } }
```

VS Code (any ACP client extension) and Paseo take the same command line: `moca acp`. Optional flags apply to every session the editor opens: `--model provider/model`, `--effort <level>`, `--mode default|auto|yolo|plan`, `--approve` / `--no-approve`. By default a folder you already answered in moca's TUI trust prompt keeps that answer; any other folder the editor opens is trusted (project rules, AGENTS.md, project skills/prompts and project MCP servers apply).

What you get: streamed replies and thinking, tool calls with diffs, approval prompts, cancel, the permission-mode picker, the model and effort pickers, your prompt templates and skills in the `/` menu, session titles, context-usage and compaction info, warnings as notices (where the editor supports them), and session history: reopen, resume, close and delete past moca sessions of the folder. Not supported: images, editor-provided MCP servers (configure them in moca), and TUI-only commands (`/compact`, `/undo`, …).

## Debugging / logs

moca keeps a small diagnostic log per run in `~/.local/state/moca/logs/` (or `$XDG_STATE_HOME/moca/logs/`). It records what moca did — retries, MCP servers starting and stopping, how a run ended — but never your prompts, code or keys. If something goes wrong, attach the newest file to your bug report.

- More detail for one run: `MOCA_LOG=debug moca …`
- Turn it off: `"log": { "level": "off" }` in the config, or `MOCA_LOG=off`.
- Old logs are removed after `snapshot.retentionDays` (30 days by default).

## Permission modes & rules

Every shell command the agent proposes runs through a permission ladder. Three modes decide how strict:

| Mode | Behavior |
|---|---|
| `ask` | every non-allowlisted command asks (once / for this session / always / deny) — this was called `default` before v0.7.2, and `default` still works as its name |
| `auto` | **the default since v0.7.2**: safe actions (reads, builds, tests, confined edits) just run; risky ones (`git push`, installs…) ask every time; commands the tables can't judge get a one-shot classifier opinion — allow or ask |
| `yolo` | no permission checks |

A fresh install starts at `auto`; `--mode ask`, `/mode ask` or `"defaultMode": "ask"` in the config switches to the strictest interactive mode. Auto mode's classifier is one extra model call only when the risk table cannot decide a command, and `"auto": {"classifier": false}` turns it off (unsure commands then ask).

Pick per run with `moca --mode ask|auto|yolo|plan` (`default` is still accepted — it is what earlier releases called `ask`) — `plan` is **plan mode** as a flag value (the plan scope: the run writes `docs/plans/`, nothing else; the v0.4.0-beta dev builds spelled these `--plan`, `--yolo` and `--no-yolo`, which now map to `--mode plan`, `--mode yolo` and `--mode default`) — or in the config with `"permissions": { "defaultMode": "auto" }` (user level; a trusted project's `.moca/config.jsonc` can pin it too and wins) — or in the TUI with **shift+tab** (`default → auto → plan → yolo`; `/mode` for the same, and plan rides only on `default`).

```jsonc
// ~/.config/moca/config.jsonc
{
  "permissions": {
    "defaultMode": "auto",                // ask | auto | plan | yolo (default: auto)
    "allow": ["rtk *", "make *", "git log *"],
    "ask":   ["git commit *"],
    "deny":  ["rm -rf *", "git push --force*"],
    "auto":  { "classifier": true }       // "auto.model" defaults to your "model"
  }
}

// <project>/.moca/config.jsonc — applies only when the workdir is trusted
// (moca asks once on first open). Holds a permissions object (the startup mode
// included — default|auto|plan, winning over the user's) and an mcp block —
// never colors (tui is user-only), never yolo, a "*" allow rule, a "*" MCP
// approval or auto.model: a cloned repository cannot start permission-free
// sessions, blanket-allow commands or servers, or pick what model bills its
// classifier.
{
  "permissions": {
    "defaultMode": "plan",              // this project's startup mode (wins over the user's)
    "allow": ["docker compose *"],
    "deny":  ["terraform apply *"]
  },
  "mcp": {
    "servers": {
      "docs":   { "command": "npx", "args": ["-y", "docs-mcp"] }, // a server of this project's own
      "github": { "approve": ["create_issue"] }                   // an approval patch on a server your own config defines
    }
  }
}
```

Rules are whitespace-separated words with bash-style wildcards: `git push *` (prefix), `*.go` in a word, `rm *` beats `rm` on 100 spellings, and `mkfs*` style names still hard-deny no matter what the rules say. Project rules win over user rules; within one list, deny wins over ask, ask over allow. `shell.allow` keeps working and folds into the same user list (the approval prompt's *allow always* now writes a `permissions.allow` rule into the trusted project's `.moca/config.jsonc`, not `shell.allow`) — except for `rm`, which always asks unless a real `permissions.allow` rule covers the exact command. Safe things (reads, builds, tests, confined file edits, plain downloads) need no rule in auto mode; the risk table covers them.

**In the TUI** the mode badge always leads status line 2, directly before the model (`Ask` / green `Auto` / red `Yolo`, plus blue `Plan` when plan mode is on) — and the model field renders in that same plan blue. An approval prompt is a picker: `↑`/`↓` and `enter` alongside `o` (allow once) / `s` (allow for this session — memory only, offered on `rm` and the other ask-every-time questions too) / `a` (allow always — the one answer that saves) / `d` (deny); with text in the input box `↑`/`↓` keep their history meaning and `enter` submits, while the four letters answer only after a ~0.7 s pause (a capital always types).

## Use

```bash
moca                            # TUI in the current directory
moca --mode auto -p "fix the failing test"  # one-shot; prompt also on stdin (-p -)
moca --mode plan -p "add a --json flag"   # write docs/plans/<slug>.md, change nothing else
moca execute docs/plans/add-json-flag.md   # execute it step by step, ticking each - [ ] box
moca --continue                 # resume the latest session of this directory (or --resume <id8|last>)
```

`-p` contract: stdout is the final answer text only; tool activity, retries and the token/cost summary go to stderr. Exit codes: `0` completed · `1` provider/runtime error · `2` config or usage error · `3` incomplete (step limit, or stopped after repeating the same failing call) · `130` interrupted.

Sessions are append-only files under `~/.local/share/moca/sessions/`, keyed by working directory: `--continue` reopens this directory's latest, `--resume <id8>` any of its sessions and `--resume last` its newest (a session resumes only from the directory it started in), and `/sessions` lists, switches or deletes them in the TUI.

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
| `shift+tab` | cycle the permission mode (ask → auto → plan → yolo) |
| `ctrl+t` | cycle the effort level |
| `@` + path | complete a workdir file (`tab`/`enter` inserts it) |
| `ctrl+c` ×2 | quit (or `/exit`) |
| `o` / `↑↓` `enter` / `s` / `a` / `d` | approval picker: allow once / move+confirm / allow for this session (memory only, never written) / allow always (saved into the project's `.moca/config.jsonc`) / deny (letters and arrows only after a 700 ms pause, arrows+enter only with an empty draft) |
| `↑`/`↓` · `tab` · `esc` | in the `/` dropdown: pick · complete · dismiss |

**Slash commands**: `/model` · `/effort` · `/mode` · `/hard` · `/yolo` · `/plan` · `/execute` · `/clear` · `/resume` · `/sessions` · `/compact` · `/cost` · `/undo` · `/copy` · `/show <n>` · `/remember <text>` · `/login` · `/logout` · `/help` · `/exit` (`/q`/`/quit`). Typing `/` opens a dropdown of every command, loaded template (`prompt:`) and skill (`skill:`) as you type — `↑`/`↓` pick, `tab` completes, `enter` on an exact name runs it, and the command echoes into the transcript like your messages before its output.

Prompt templates (`~/.config/moca/prompts/<name>.md` becomes `/name`) and skills (every skill you add is invokable as `/name`; an agent-recorded `auto` note is listed but never a command) join the same dropdown. **`/create-command`** is a built-in skill (nothing is dropped into your config dir): run it and the agent writes or updates the command for you — personal commands in `~/.config/moca/prompts/` (moca asks you to approve a write outside the workdir — once or for the session, never persistently) or project ones in `.moca/prompts/`. New commands and skills are usable the moment the run that wrote them finishes — no restart.

`!cmd` runs a command and feeds its output to the model; `!!cmd` runs it locally without telling the model. Typing during a run steers it after the current tool results.

**`/plan <request>`** (or `--mode plan`) turns a run into plan mode: the agent reads (shell, web and MCP stay available for investigation under their normal permission rules), writes an implementation plan to `docs/plans/`, and every write outside `docs/plans/*.md` is refused.

## Extend

- **Skills** — `SKILL.md` directories in `~/.config/moca/skills/` (global) or `<repo>/.moca/skills/` (trusted projects). Any skill written for pi, Claude Code or OpenCode loads unchanged, and every skill you add is invokable from the TUI as `/name` unless it is an agent-recorded `auto` note (the `/` picker tags skills `skill:`). moca ships three built-in skills that update with the binary: `create-command`, `self-knowledge` (answers questions about moca itself from embedded copies of these docs) and `self-configure` (edits your config, commands and skills when you ask — config writes raise the approval prompt, and `auth.json` stays out of reach). It also plants two standard skills, `rtk` and `graphify`, in `~/.config/moca/skills/` on start when they are missing — they are yours from then on: edit or replace them (moca never rewrites an existing one), or keep an empty `~/.config/moca/skills/<name>/` to opt out. A seeded skill is hidden from the model on a host where its CLI is not installed. See [docs/specs/external-tools.md](docs/specs/external-tools.md).
- **MCP** — `mcp.servers` in the config (stdio + streamable HTTP), plus `mcp.servers` in a trusted project's `.moca/config.jsonc` and the project's own Claude Code / OpenCode / Pi files (`.mcp.json`, `opencode.json[c]`, `.pi/mcp.json`) — a project server starts only while that workdir is trusted, your config wins a name both define, and a duplicate warns with both file paths. Lazy by design: one fixed ~200-token `mcp` proxy tool, server tool lists never enter the prompt, servers start on first use and stop when idle. `moca mcp import` imports Claude Code / OpenCode / Pi server configs; `moca mcp index` prebuilds the discovery index from the CLI.
- **Project instructions** — `AGENTS.md` / `CLAUDE.md`, loaded only in trusted projects (`--approve`).
- **External tools** — `rtk` (token-compressed CLI output) and `graphify` (codebase knowledge graph) are allowlisted, and a skill teaching each is seeded into your skills dir on start (see Skills above). See [docs/specs/external-tools.md](docs/specs/external-tools.md).

### Self-managed skills

Self-managed project skills are an opt-in feedback loop: instead of relearning the same thing every session, moca can record a reusable working rule as an ordinary project file that later sessions read from their prompt. It writes them in-turn with the existing `write`/`edit` tools — no ninth tool, no session-end pass, no second model call — into `<repo>/.moca/skills/moca-<domain>/SKILL.md`, so a lesson learned once (the command that actually verifies a change, a build quirk, a convention you stated) lands in the repository with your code: `0644`, visible in `git status`, reviewable in a diff.

```jsonc
// ~/.config/moca/config.jsonc
{
  "skills": {
    "selfManaged": true,   // off by default; the only opt-in
    "recordNudge": true    // default: one end-of-run recording nudge (false = the mid-run hint)
  }
}
```

**What it records.** A rule the project's own tooling proved by running, a repeated pattern, or something you stated — never a statement about the code, a file inventory or one-off task detail. `auto: true` in the frontmatter marks the file as agent-maintained; the body is one rule per line:

```markdown
---
name: moca-build
description: Build and verify this repository
auto: true
---

# Build

- **when** verifying a change: `make test` is the gate — `go test ./...` misses the generated fixtures (×1, 2026-10-18)
- **when** touching the CLI contract: update docs/specs/SPECS.md in the same change (user, 2026-10-18)
```

**Using it.** With the switch on there is nothing to invoke — the agent records when it has something worth keeping, and `/remember <text>` makes it explicit, e.g. `/remember always run make vet before calling a change done`.

**The contract.** 12 rule lines per skill, 240 runes a line, 1500 a body, 8 auto skills per project, 2 new skills per run. Provenance is checked against the host clock, not the model's word: a new line starts `×1`, a line the agent later revisits unchanged bumps once to `×2`, and a `(user, …)` line comes only from `/remember <text>` (the TUI says so when the switch is off) and never expires. Detection is in-turn: a one-shot hint fires at a decision point — a guard refused the agent, a third identical command failure in a row, a steer, a recovery after a failed verification, or a cold repository's first green check — and the recording is a `write`/`edit` like any other. Since v0.7.1 the invitation lands at the **end of the run** by default: one nudge before the final answer asks for the record (or for one line saying nothing durable came up), and `"skills": {"recordNudge": false}` — in the user config or a trusted project file — brings back the mid-run hint instead.

**The files are yours.** While the switch is on, `write`/`edit` validate the exact bytes that would land:

- a skill you hand-wrote has no `auto: true`, and the agent cannot edit it unless your own typed text in that run names it — `/remember` counts, a `-p` prompt does not;
- the marker cannot be stripped, a skill the agent creates must be an auto skill, and a case-variant spelling of a path inside the skills dir is refused;
- a credential-shaped value is refused, and the caps check what a write *changes*, so an already-over-cap file stays editable;
- `shell` refuses the common redirect and `tee` targets into that directory — an interpreter can still write there, so the next session re-validates every stored auto skill at load and drops a contract-breaking one from the listing with one warning.

**Per project.** A trusted repository's `.moca/config.jsonc` can opt out — of the feature, of the end-of-run nudge, or of both — and can never opt in:

```jsonc
{
  "skills": {
    "selfManaged": false,   // this repository takes no part
    "recordNudge": false    // …or keep the feature and the mid-run hint only
  }
}
```

Both keys come from the project layer only when the repository is trusted, and a user's `recordNudge: false` outranks a project `true`.

A project's `true` is declarative: it warns you when your user config does not enable the feature, and never enables it alone. Project skills load only in a trusted workdir, so `-p` needs `--approve`; `moca acp` trusts by default, which makes an editor the widest learning surface. Plan mode never learns (writes stay in `docs/plans/`); `moca execute` does. In the prompt, auto skills are listed with `(auto)` and a read-only "fallible memory — verify against the repo" framing, and they never become `/name` slash commands.

**Expiry.** At an interactive start (TUI, ACP) a `×1` rule nobody confirmed for 90 days is dropped, and an auto skill left with only headings loses its `SKILL.md`; a `-p` run only reports what it would remove, and `(user, …)` rules never expire. Turning the switch off deletes nothing and takes effect in a new process — a resumed session keeps the prompt it started with, so one that began while the switch was on refuses auto-skill writes for its whole life. A committed `skills` block is a config error for moca older than v0.7.0 (strict decoding), so a team upgrades before committing `.moca/config.jsonc`.

## Non-goals (v1, probably forever)

subagents · hooks · LSP · image gen · voice · telemetry · binary plugin system

## Development

```bash
mise install        # provisions Go 1.27.1 (pinned once, in go.mod) and gopls
make build          # bin/moca, version-stamped from git describe
make test           # go test ./... -race -count=1
make vet            # go vet ./... + gofmt -l . (both must be clean — the CI gate)
make release        # six cross-compiled binaries in dist/ (linux, darwin, windows × amd64/arm64)
```

`make` falls back to `mise x go -- go` when `go` is not on the PATH; with mise, the same targets run as `mise run build|test|vet|fmt|release|clean`. Two scripted harnesses sit beside the unit suites (both need a model key in the repo `.env`, both build first): the **eval corpus** (`bash test/evals/run.sh -n 3 -l <label>` — twelve scenarios with per-run metrics recorded under `test/evals/results/`, baseline in `test/evals/BASELINE.md`) and the **ship gate** (`bash test/shipgate/run.sh` — §14, a real bug behind a failing test fixed end to end; the checker is `//go:build shipgate`, so plain `go test ./...` skips it).

## Docs

[`docs/specs/SPECS.md`](docs/specs/SPECS.md) — the single source of truth for design, specs, features and constraints · [`BENCHMARKS.md`](docs/BENCHMARKS.md) — every measured run, losses included · [`docs/specs/external-tools.md`](docs/specs/external-tools.md) — rtk, graphify, skills, prompts · [`docs/themes/tui-theme-builder.html`](docs/themes/tui-theme-builder.html) — live TUI theme builder (emits a `tui.theme` block) · [`CONTRIBUTING.md`](docs/CONTRIBUTING.md) · [`SECURITY.md`](docs/SECURITY.md).

## Status

Latest release: **[github.com/adeotek/moca/releases/latest](https://github.com/adeotek/moca/releases/latest)** · full history in the [changelog](CHANGELOG.md) · published benchmark numbers, losses included, in [BENCHMARKS.md](docs/BENCHMARKS.md).

# moca — MO Coding Agent

[![CI](https://github.com/adeotek/moca/actions/workflows/ci.yml/badge.svg)](https://github.com/adeotek/moca/actions/workflows/ci.yml)

**A minimal, token-efficient, provider-agnostic coding agent: one Go binary, a Bubble Tea TUI, eight tools, no framework.**
Inspired by Claude Code, OpenCode and Pi — deliberately ~10% of their surface area. It competes on *cost per task* and *read-the-whole-codebase-in-an-hour transparency*, not features: prompt caching, windowed reads, diff-shaped results and token-denominated compaction keep a session cheap; the agent ships MCP support that never floods the prompt (one ~200-token lazy proxy), loads Agent-Skills `SKILL.md` files written for other tools unchanged, searches the web and fetches pages without another tool (keyless by default), and prefers external token-savers (`rtk`, `graphify`) when they are installed.

The eight tools: `read` · `write` · `edit` · `shell` · `search` · `ls` · `web` (search + fetch) · `mcp` (the lazy proxy — servers extend it on your say-so). The eight schemas are frozen by design; everything else is [Non-goals](#non-goals-v1-probably-forever).

**Status: v0.6.5** — documentation and eval-harness correctness only, no behaviour change: the `openai-responses` history-bound question is **closed by a live probe** (a kept `function_call` whose reasoning item was dropped is accepted there — 200, with a 400 control — so the kept-tail strip and elision stay enabled and `HistoryBound()` stays adaptive-only), and the `vague` eval's `-to f -- -300` case is **settled and tightened** (`-to f` takes Celsius input, so −300 °C is below absolute zero: the checker is right and the models miss the bound; the suite gained two per-scale valid-side cases). Before that (v0.6.4): one working-style rule, from the cell diagnostics on the four contested Terminal-Bench cells (3 attempts × 2 models: fix-git 6/6, custom-memory 5/6, cobol 5/6, **mailman 3/6** after five consecutive round failures): **the handoff is part of the task** — when a flow hands off to another service (mail, a queue, a daemon), put the message where the recipient actually reads it and verify it arrives; a local send that never reaches its destination is not a delivery. Every winning mailman attempt did the delivery-path work (Postfix transport/alias maps, the LMTP map, the runners started by hand, the confirmation verified in `/var/mail/<user>`) while every losing one set `SubscriptionPolicy.open` and skipped the MTA. Before that (v0.6.3): two working-style corrections from the Terminal-Bench round 4 (subset 8/12, each failing cell diagnosed): the **pending state is part of the behaviour** (a confirmation flow is only satisfied when the request does not take effect before confirmation and the confirmation is delivered and answered — a setting that skips the step satisfies the name, not the flow; this is the mailman cell) and **a run has a wall-clock budget** (a probe that has answered the question ends the investigation; a dependency's internals are a last resort, bounded to one falsifiable hypothesis — this is the cobol cell). Before that (v0.6.2): history correctness plus two Terminal-Bench round-3 lessons (subset 10/12): plan mode stops rewriting earlier messages when its envelope toggles (the flag lives on the entry, and turning it off appends a note that the earlier envelopes no longer apply), a compaction's kept tail stops replaying its thinking blocks on routes that do not bind them (preserved-thinking/adaptive routes keep the tail verbatim and skip unforced elision entirely), `xhigh` joins the adaptive effort map, and a provider-reported thinking drop is counted per turn and warned once per run. Headless (`-p`) runs whose reply would end the run with no tool call at all get one nudge to act, and the working style gains "the stated behaviour outranks a name it gives". The eval corpus gains a prompt-injection safety scenario plus a transcript secret scan. Before that (v0.6.1): benchmark round 2: a second Terminal-Bench 2.0 run with v0.6.0 (subset 7/12, up from 6/12; diagnostic set 6/8, up from 3/8) turned the four repeatable gaps into working-style rules — a stateful file gets a byte copy as the first action and a read never shares a shell call with a stateful open (SQLite deletes a WAL it cannot parse), the finish state a checker will see stays pristine, an inconsistency you notice is a defect to investigate rather than explain away, and a build that compiles must still exercise every module and entry point the task names. The shell enforces the stateful-file rule too: a `sqlite3` open that would consume an unparseable `-wal`/`-journal` sidecar is refused until the run has copied the file. Before that (v0.6.0): the TUI's colors are configurable now: a `tui.theme` block (`both`/`dark`/`light` groups of color roles; a `#rrggbb` hex or an ANSI color number per role) in your config (user-only — a project file cannot set colors). The permission mode a session starts in is configurable the same way — `permissions.defaultMode` takes `default | auto | plan | yolo` at the user level, a trusted project's file may pin `default | auto | plan` (never `yolo`) and wins; `plan` starts the plan scope, and `moca execute` skips a configured `plan` layer (the next layer decides). Also in this release: a hardening pass from the Terminal-Bench 2.0 review and the mid-October model change: the zen tier's glm serving variants became thinking-only, so the effort maps were re-verified against the live endpoint — `glm-5.3-flash` accepts only `low|high|max`, and moca's default `medium` now clamps to `low` instead of being sent as-is and 400-ing mid-task. The agent gains a **progress nudge** (a run that has taken 12 tool batches without creating or editing a file is told once to act on what it knows or name the concrete blocker) and the working style now time-boxes reconnaissance, demands verification that could actually fail, forbids presenting reconstructed data as fact, and says to copy stateful files before probing them (opening a SQLite database checkpoints it and can delete its WAL). Before that (v0.5.1-beta): project-scoped MCP servers — a trusted workdir's `.moca/config.jsonc` `mcp` block and its Claude Code / OpenCode / Pi files (`.mcp.json`, `opencode.json`, `opencode.jsonc`, `.pi/mcp.json`) join your own servers (a duplicate name warns and names both files), and *allow always* answers — for commands and MCP tools alike — are saved in that project file instead of your global config. Also: the approval panel shows its options once (the picker, no longer also an inline hint line), and shifted symbols reach the input intact (`shift+/` used to arrive as `/` on terminals with the kitty keyboard protocol). Before that (v0.5.0-beta): an ACP server (`moca acp`) so Zed, VS Code ACP extensions and Paseo can drive moca: streamed replies and thinking, tool calls with diffs, approvals, cancel, mode/model/effort pickers, `/` commands, session history, usage and cost (the TUI and `-p` are unchanged). Before that (v0.4.1-beta): `rtk` and `graphify` become seeded standard skills (moca plants them in `~/.config/moca/skills/` on start when they are missing — edit, replace or opt out of them and a later moca leaves them alone) and `moca prune` clears the built-in skill extractions older versions left in the data dir; plan execution is `moca execute <plan.md>` / `/execute` (`--do` and `/do` are gone) and Shift+Tab cycles `default → auto → plan → yolo`. Previously: a permissions overhaul: three permission modes (`default` / `auto` / `yolo`) with a single `--mode default|auto|yolo|plan` flag, layered `allow/ask/deny` rules with wildcards (user config + a trusted per-project `.moca/config.jsonc`), auto mode (safe actions run, risky ones ask, unsure ones get a classifier's second opinion), Shift+Tab mode cycling, an arrow-key approval picker, `${…}` prompt templates, a role-colored status bar and skills invokable from the `/` picker (tagged `prompt:`/`skill:`); the install scripts gained `--uninstall` / `-Uninstall`, every run writes a bounded diagnostic log, and moca ships `self-knowledge` and `self-configure` built-ins so it can answer questions about itself and manage its own config; plus first-run setup mode, the `web` tool, plan mode, self-update, saved `/` commands and an Ollama provider; the §14 ship gate's live legs pass (evidence in SPECS §15).

## Install

```bash
go install github.com/adeotek/moca/cmd/moca@latest
```

**Install scripts** — `install.sh` (Linux) and `install.ps1` (Windows) download the release package for your platform, verify it against the release's `checksums.txt`, and install the binary — to `~/.local/bin` (`%LOCALAPPDATA%\Programs\moca` on Windows, added to your user PATH), or next to an existing `moca` when one is already on PATH:

```bash
# Linux
curl -fsSL https://raw.githubusercontent.com/adeotek/moca/main/install.sh | bash
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/adeotek/moca/main/install.ps1 | iex
```

Re-running a script is safe: an existing installation is updated in place, never an error. `--version vX.Y.Z` / `-Version vX.Y.Z` pins a release and `--dir <path>` / `-Dir <path>` picks the target directory; `--help` / `-?` prints the script's usage. `--uninstall` / `-Uninstall` removes the installed binary instead of installing — the config, data and state directories are left untouched; `--remove-data` / `-RemoveData` is the full uninstall: the binary plus `~/.config/moca`, `~/.local/share/moca` and `~/.local/state/moca` (config, credentials, sessions, logs) go too. Or take a package from the [releases page](https://github.com/adeotek/moca/releases) — linux/amd64 · linux/arm64 · windows/amd64 are published by a manual **Release** workflow run (Actions → Release → Run workflow); other platforms (e.g. darwin) build from source with `make release`.

**Self-update** — an installed moca updates itself from the same releases:

```bash
moca update           # replace the binary with the latest release package for this platform
moca update --check   # only report whether a newer release exists
```

`moca update` verifies the package against the release's `checksums.txt` when published, then swaps the binary in place (atomic) — your config, credentials and sessions are never touched, and a running session keeps working (the new version starts on the next launch).

Each version extracts its built-in skills to its own directory under `~/.local/share/moca/builtin-skills/` and keeps the older ones, so a moca still running from the previous version is not pulled out from under. Once nothing older is running, `moca prune` removes every one but the current version's (`moca prune --dry-run` lists them first); sessions, snapshots, history and your config are never touched.

## Uninstall

Remove the binary (the config, data and state directories are kept):

```bash
# Linux
curl -fsSL https://raw.githubusercontent.com/adeotek/moca/main/install.sh | bash -s -- --uninstall
```

```powershell
# Windows (PowerShell)
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/adeotek/moca/main/install.ps1))) -Uninstall
```

Full uninstall — the binary plus `~/.config/moca`, `~/.local/share/moca` and `~/.local/state/moca` (config, credentials, sessions, logs; `$XDG_CONFIG_HOME`/`$XDG_DATA_HOME`/`$XDG_STATE_HOME` are honored):

```bash
# Linux
curl -fsSL https://raw.githubusercontent.com/adeotek/moca/main/install.sh | bash -s -- --remove-data
```

```powershell
# Windows (PowerShell)
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/adeotek/moca/main/install.ps1))) -RemoveData
```

Both scripts remove the `moca` found on `PATH` (or the one in `--dir <path>` / `-Dir <path>`), and refuse to delete anything whose `--version` output does not identify it as moca.

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

Optional TUI knobs: `"tui": { "notify": "osc9" }` (`osc9` · `bell` · `off`) — a desktop notification (or bell) when a run longer than 30 s ends or an approval is waiting, only while the terminal is unfocused — and `"tui": { "theme": { … } }` — every color the TUI draws, per role: `both`, `dark` and `light` groups (`dark`/`light` follow the terminal’s reported background, `both` fills whichever applies and loses to the variant’s own key), each value a `#rrggbb` hex or an ANSI color number `0`–`255`; an unset role keeps the built-in palette (`internal/tui/theme.go`), and the block is user-only — a project’s `.moca/config.jsonc` cannot carry colors. Tune it in [docs/tui-theme-builder.html](docs/tui-theme-builder.html), a live mock that emits the block.

Web search is keyless by default; to switch engines or lift tavily's rate limit: `"web": { "search": { "provider": "tavily", "apiKey": "env:TAVILY_API_KEY" } }` — `tavily` (default) or `exa` (a key is required). Fetching pages needs no config.

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

## Editors (ACP)

`moca acp` speaks the [Agent Client Protocol](https://agentclientprotocol.com) on stdio, so ACP editors can drive moca. It uses your existing config and credentials (`moca login` in a terminal first).

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
| `default` | every non-allowlisted command asks (once / always / deny) |
| `auto` | safe actions (reads, builds, tests, confined edits) just run; risky ones (`git push`, installs…) ask every time; commands the tables can't judge get a one-shot classifier opinion — allow or ask |
| `yolo` | no permission checks |

Pick per run with `moca --mode default|auto|yolo|plan` — `plan` is **plan mode** as a flag value (the plan scope: the run writes `docs/plans/`, nothing else; the v0.4.0-beta dev builds spelled these `--plan`, `--yolo` and `--no-yolo`, which now map to `--mode plan`, `--mode yolo` and `--mode default`) — in the config with `"permissions": { "defaultMode": "auto" }` — user level; a trusted project's `.moca/config.jsonc` can pin it too and wins; any of `default | auto | plan | yolo` — or in the TUI with **shift+tab** (`default → auto → plan → yolo`, `/mode` for the same; plan rides only on `default`).

```jsonc
// ~/.config/moca/config.jsonc
{
  "permissions": {
    "defaultMode": "default",             // default | auto | plan | yolo
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

**In the TUI** the mode badge always leads status line 2, directly before the model (`Default` / green `Auto` / red `Yolo`, plus blue `Plan` when plan mode is on) — and the model field renders in that same plan blue. An approval prompt is a picker: `↑`/`↓` and `enter` alongside the classic `a` / `ctrl+a` / `d` — with text in the input box, keys go to the draft as usual.

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
| `shift+tab` | cycle the permission mode (default → auto → plan → yolo) |
| `ctrl+t` | cycle the effort level |
| `@` + path | complete a workdir file (`tab`/`enter` inserts it) |
| `ctrl+c` ×2 | quit (or `/exit`) |
| `a` / `↑↓` `enter` / `ctrl+a` / `d` | approval picker: allow once / move+confirm / allow always (saved into the project's `.moca/config.jsonc`) / deny (arrows+enter only with an empty draft) |
| `↑`/`↓` · `tab` · `esc` | in the `/` dropdown: pick · complete · dismiss |

**Slash commands**: `/model` · `/effort` · `/mode` · `/hard` · `/yolo` · `/plan` · `/execute` · `/clear` · `/resume` · `/sessions` · `/compact` · `/cost` · `/undo` · `/copy` · `/show <n>` · `/login` · `/logout` · `/help` · `/exit` (`/q`/`/quit`) — plus prompt templates (`~/.config/moca/prompts/<name>.md` becomes `/name`) and skills (`self-knowledge`, `create-command`, rtk's own — any skill is invokable as `/name`). Typing `/` opens a dropdown of every command, loaded template and skill as you type — templates are tagged `prompt:` and skills `skill:`, in that muted hint region; `↑`/`↓` to pick, `tab` to complete, `enter` on an exact name to run it — and the command echoes into the transcript like your messages before its output. **`/create-command`** is a built-in skill (nothing is dropped into your config dir): run it and the agent writes or updates the command for you — personal commands in `~/.config/moca/prompts/` (moca asks you to approve a write outside the workdir, once, never persistently) or project ones in `.moca/prompts/`. New commands and skills are usable the moment the run that wrote them finishes — no restart. `!cmd` runs a command and feeds its output to the model; `!!cmd` runs it locally without telling the model. Typing during a run steers it after the current tool results. **`/plan <request>`** (or `--mode plan`) turns a run into **plan mode**: the agent reads (shell, web and MCP stay available for investigation under their normal permission rules), writes an implementation plan to `docs/plans/`, and every write outside `docs/plans/*.md` is refused.

## Extend

- **Skills** — `SKILL.md` directories in `~/.config/moca/skills/` (global) or `<repo>/.moca/skills/` (trusted projects). Any skill written for pi, Claude Code or OpenCode loads unchanged, and every discovered skill is invokable from the TUI as `/name` (the `/` picker tags skills `skill:`). moca ships three built-in skills that update with the binary: `create-command`, `self-knowledge` (answers questions about moca itself from embedded copies of these docs) and `self-configure` (edits your config, commands and skills when you ask — config writes raise the approval prompt, and `auth.json` stays out of reach). It also plants two standard skills, `rtk` and `graphify`, in `~/.config/moca/skills/` on start when they are missing — they are yours from then on: edit or replace them (moca never rewrites an existing one), or keep an empty `~/.config/moca/skills/<name>/` to opt out. A seeded skill is hidden from the model on a host where its CLI is not installed. See [docs/specs/external-tools.md](docs/specs/external-tools.md).
- **MCP** — `mcp.servers` in the config (stdio + streamable HTTP), plus `mcp.servers` in a trusted project's `.moca/config.jsonc` and the project's own Claude Code / OpenCode / Pi files (`.mcp.json`, `opencode.json[c]`, `.pi/mcp.json`) — a project server starts only while that workdir is trusted, your config wins a name both define, and a duplicate warns with both file paths. Lazy by design: one fixed ~200-token `mcp` proxy tool, server tool lists never enter the prompt, servers start on first use and stop when idle. `moca mcp import` imports Claude Code / OpenCode / Pi server configs; `moca mcp index` prebuilds the discovery index from the CLI.
- **Project instructions** — `AGENTS.md` / `CLAUDE.md`, loaded only in trusted projects (`--approve`).
- **External tools** — `rtk` (token-compressed CLI output) and `graphify` (codebase knowledge graph) are allowlisted, and a skill teaching each is seeded into your skills dir on start (see Skills above). See [docs/specs/external-tools.md](docs/specs/external-tools.md).

## Non-goals (v1, probably forever)

subagents · hooks · LSP · image gen · voice · telemetry · binary plugin system

## Development

```bash
mise install        # provisions Go 1.27.1 (pinned once, in go.mod) and gopls
make build          # bin/moca, version-stamped from git describe
make test           # go test ./... -race -count=1
make vet            # go vet ./... + gofmt -l . (both must be clean — the CI gate)
make release        # five cross-compiled binaries in dist/
```

`make` falls back to `mise x go -- go` when `go` is not on the PATH; with mise, the same targets run as `mise run build|test|vet|fmt|release|clean`. Two scripted harnesses sit beside the unit suites (both need a model key in the repo `.env`, both build first): the **eval corpus** (`bash test/evals/run.sh -n 3 -l <label>` — eight scenarios with per-run metrics recorded under `test/evals/results/`, baseline in `test/evals/BASELINE.md`) and the **ship gate** (`bash test/shipgate/run.sh` — §14, a real bug behind a failing test fixed end to end; the checker is `//go:build shipgate`, so plain `go test ./...` skips it).

## Docs

[`docs/specs/SPECS.md`](docs/specs/SPECS.md) — the single source of truth for design, specs, features and constraints · [`docs/specs/external-tools.md`](docs/specs/external-tools.md) — rtk, graphify, skills, prompts. · [`docs/tui-theme-builder.html`](docs/tui-theme-builder.html) — live TUI theme builder (emits a `tui.theme` block).

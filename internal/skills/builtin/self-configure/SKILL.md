---
name: self-configure
description: Manage moca's own configuration and installation when the user asks to change settings, modes, permission rules, models, prompts or skills — or to update the binary. Config edits need the user's approval; auth.json and the running binary are off-limits.
---
Changing how this moca installation behaves, at the user's request.

## What you can change
- **User config** — the live `config.jsonc` (the environment's moca-dirs line names its directory). Read it, then edit it with `edit`/`write`: it is outside the workdir, so moca raises the approval prompt every time — that is expected. If the user denies the write, stop and ask what they want.
- **Project config** — `<workdir>/.moca/config.jsonc`: permissions only (allow/ask/deny rules, defaultMode). It lives inside the workdir and takes effect only when the project is trusted.
- **Prompt templates** — `<prompts dir>/<name>.md`, one file per `/name` command (frontmatter `description`, optional `argument-hint`; body placeholders `$$ARGUMENTS`, `$$1..$$9`, `$$$$`). Writes there raise the approval prompt.
- **Skills** — `<skills dir>/<name>/SKILL.md` with frontmatter `name` and `description` (optional `argument-hint`); supporting files live beside it. Writes there raise the approval prompt.
- **The session's permission mode** — suggest the user type `/mode default|auto|yolo|plan` (or shift+tab); never change it yourself.

## How
- `read` the file first — `edit` requires it. Use exact replacements: JSONC comments must survive the edit.
- The config decode is strict: unknown keys are rejected; `apiKey` must be an `env:VAR` reference, never a literal; `permissions.defaultMode` is `default|auto|yolo` (not `plan` — plan is a run flag).

## Verify
- Re-read the file after writing.
- Then run `moca mcp index` via shell: it loads and validates the config up front — exit 2 with a `line:col` position means the file is broken (this is the config check to use; there is no `moca doctor` in this version).

## Never
- `auth.json` (in the same directory as the config): credentials belong to `/login` or `moca login <provider>` — the user types those. Never ask for, read, write or echo keys.
- The data dir (sessions, trust.json, snapshots) and the running binary.

## Installation
- `moca update --check` reports whether a newer release exists (safe, read-only). The user runs `moca update` themselves to install it. Install/uninstall paths are in the readme reference.
- `moca prune --dry-run` lists the older built-in skill extractions that previous versions left in the data dir (safe, read-only). The user runs `moca prune` themselves to delete them, when no older moca is open; it never touches sessions, config or your skills.

## When it takes effect
- A new prompt or skill is usable as soon as this run ends (`/` picker reloads; `/help` lists them) — no restart.
- A system-prompt listing refresh needs a new session; config changes apply to the next launch.

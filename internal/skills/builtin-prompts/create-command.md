---
description: Create or update a moca slash command (a saved prompt template)
argument-hint: "<name> [what it should do]"
---
Create or update a moca slash command: a saved prompt template that runs as `/<name>` in the TUI.

The request: $ARGUMENTS

## Where commands live
- `{{prompts_dir}}/<name>.md` — the user's personal commands. Use this unless the request is clearly project-specific. The directory is outside the workdir, so moca asks the user to approve the write; that prompt is expected — proceed once it is answered — and if the user denies it, stop and ask what they want.
- `<workdir>/.moca/prompts/<name>.md` — project commands, loaded when the project is trusted and shared with the repo. Choose this when the command belongs to the project or when the user asks for it.

## File format
Optional frontmatter, then the prompt body that is sent as the user's message when the command runs:

    ---
    description: One line — what the command does (shown in /help and the dropdown)
    argument-hint: "<name> [what]"   # optional; the usage hint next to the name
    ---
    The prompt body. Placeholders: $$ARGUMENTS or $$@ (all arguments as typed),
    $$1..$$9 (whitespace-split arguments), $$$$ (a literal $).

## Rules
- The name is the file name without `.md`: one lowercase word or kebab-case. Built-in names win over a template of the same name — never use: model, effort, hard, yolo, clear, resume, compact, cost, undo, copy, show, login, logout, help, exit (nor the aliases q/quit).
- Existing file: `read` it first, keep everything that still applies and change only what the request asks for.
- The body is a prompt for the agent, not documentation for a person: address the work directly ("Run the tests, then …"), use the placeholders where arguments belong, and keep the command focused on one task (aim for under ~30 lines).

## Steps
1. Decide the name and the scope (personal unless the request is project-specific).
2. `read` the existing file when updating.
3. Write it with the write or edit tool.
4. Tell the user the path written and how to run it (`/<name>` with its arguments, if any); the command appears when this run finishes — no restart.

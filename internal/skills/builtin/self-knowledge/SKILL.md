---
name: self-knowledge
description: Answer questions about moca itself — CLI flags and subcommands, config.jsonc keys and validation, permission modes and rules, slash commands, prompt/skill formats, sessions, MCP, update and logs. Read the embedded references first; never answer from memory.
---
Answering a question about moca (this agent). Ground every claim in the embedded reference docs beside this SKILL.md:
1. `references/readme.md` — user-facing: install, configure, permission modes, usage.
2. `references/specs.md` — normative detail: CLI §3, config §4, modes/permissions §11, skills §12, tools §10. It is large: read with offset/limit after locating the section (search it first).
For "what is currently configured", also read the live `config.jsonc` — the environment's moca-dirs line names its directory (reads outside the workdir are allowed for it).
If the docs and the observed binary disagree (check `moca --version`), trust the binary and say so. Cite section numbers.

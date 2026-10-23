# Security policy

## Supported versions

The latest release only. `moca update` keeps you current (`moca update --check` says whether
there is anything to install).

## Reporting

Open a private advisory: https://github.com/adeotek/moca/security/advisories/new

Do not open a public issue for a vulnerability, and do not paste a working exploit into a
Discussions thread.

## In scope

- **Credential handling** — `~/.config/moca/auth.json` must stay `0600`, and a token or API key
  must never reach the diagnostic log, a session file, a snapshot or a tool result.
- **The permission ladder** — a command that should have asked and did not, a rule that matches
  more than it says, a confinement (`workdir` + temp) that can be left with a path spelling, a
  symlink or a wrapper.
- **Project trust** — a `.moca/config.jsonc`, `AGENTS.md`, project skill/prompt or project MCP
  server from an untrusted workdir being applied.
- **MCP** — a server started for a workdir that is not trusted, or a proxy call whose approval
  is skipped.
- **Web fetch** — the SSRF guards (private/link-local hosts, redirect hops) being bypassed.
- **Session and snapshot files** — data they should not disclose, or a file mode that makes them
  world-readable.
- **The self-managed skills guard** — an agent write landing in a skill you hand-wrote, or the
  caps and provenance rules being bypassed.

## Out of scope

- Prompt-injection *content* a model produces, and a run obeying it in a workdir **you** trusted
  (the guard, not the model, is the security boundary — the corpus measures it, `test/evals`).
- Issues confined to a model provider's own behaviour.
- Anything that requires running moca with `--mode yolo`, or that needs an attacker who can
  already write to your config directory.
- A local Ollama/LAN MCP endpoint being reachable by design.

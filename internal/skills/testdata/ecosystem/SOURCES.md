# Ecosystem corpus — provenance and licenses

Byte-for-byte copies of real `SKILL.md` files written for other agent
platforms — graphify, Claude Code, pi, OpenCode — plus synthetic edge cases.
`ecosystem_test.go` proves they load through `Discover` **unchanged**:
fixtures are never edited; parser fixes go into `frontmatter.go`.

## Vendored files

| Corpus path | Origin | Version / pin | License |
|---|---|---|---|
| `graphify/graphify/` | `graphify install` output for the claude platform (byte-identical to the pi variant); PyPI package `graphifyy` — <https://github.com/Graphify-Labs/graphify> | graphify 0.9.77 | Apache-2.0 OR MIT (dual) |
| `claude-code/skill-creator/` | <https://github.com/anthropics/skills> → `skills/skill-creator` | commit `683bc88e56f3e09ba94f7055977f3d3aa499f202` | Apache-2.0 (`LICENSE.txt` kept) |
| `claude-code/frontend-design/` | same repo → `skills/frontend-design` | same commit | Apache-2.0 (`LICENSE.txt` kept) |
| `pi/pi-subagents/` | npm package `pi-subagents` → `skills/pi-subagents` | 0.76.1 | MIT |
| `pi/mcp-scripting/` | npm package `pi-mcp-adapter` → `skills/mcp-scripting` | 5.1.0 | MIT |
| `opencode/dotnet-unit-testing/` | the moca author's dotfiles (`opencode/skills/dotnet-unit-testing`), used daily with OpenCode | — | MIT (declared in the skill's frontmatter) |

Each skill keeps the upstream directory shape but only a representative
supporting file (plus the upstream license file where the skill ships one).

**Evaluated, deliberately not vendored:** Anthropic's document skills
(`docx`, `pdf`, `pptx`, `xlsx`) are source-available, not redistributable.

## Synthetic edge cases

Written for this test (MIT, like the rest of the repo), under `synthetic/`:

- `crlf` — CRLF line endings;
- `bom` — a UTF-8 BOM before the frontmatter;
- `folded` — `description: >` spanning a blank line (folds to one space);
- `literal` — `description: |` keeping its newlines, blank line included;
- `colon-unquoted` — an unquoted description containing `": "`;
- `rich-keys` — `license`, `allowed-tools`, `argument-hint` and a nested
  `metadata:` map alongside the required keys;
- `long-description` — a description of exactly 1,200 characters, kept whole.

`.gitattributes` pins `-text` on the byte-exact fixtures (`crlf`, `bom`) so
git never rewrites those bytes; the CRLF and BOM cases depend on exact bytes.

## SHA-256 of the vendored files

```
0d542e0c8804e39aa7f37eb00da5a762149dc682d7829451287e11b938e94594  claude-code/frontend-design/LICENSE.txt
d91970639e9f5c37682ac7ab60094d35f1c7c1f38d731bd56396563aee10c1d3  claude-code/frontend-design/SKILL.md
bc6b3af2f331cbc7fb0da1344efb2cbe5877a31498b4d70dbc7000f3405a1362  claude-code/skill-creator/LICENSE.txt
dcd4803e61e913e6fc27294184cd3a71f09f5e924ff20c8a9a20173e7b3c2bcf  claude-code/skill-creator/SKILL.md
67cf5703402013936c8fb75ad6a1afecd8841d45cc5e606b634eb05825fde365  claude-code/skill-creator/scripts/quick_validate.py
44b54637560fadb98fd8dc0769cf3979dbdeeaf4c75971a81ea620f243e16cae  graphify/graphify/SKILL.md
e563ddcb1e155aa230f107e5ef9380bc1249c5cd8241128de7ed8a7bd9c20cf5  graphify/graphify/references/query.md
4dbc6489b55a467ae3542965b6dc27ddd311666770b94afabc7b49b241b269a3  opencode/dotnet-unit-testing/SKILL.md
db96fc0178885b161fe07575a103c6d0e42cd0f0a7e5272f74ef1af66c604b97  pi/mcp-scripting/SKILL.md
86dd00b77f366333682b21e07f91fb7bac9e7faf5d2ac122d0b4efd4d02b8307  pi/mcp-scripting/references/jev.md
41afc1f9e42006dad5188876f5af3fc5ae667d982e8d9ec88c720cbcedc35a4a  pi/pi-subagents/SKILL.md
2c41ea67edc0b9552058ae135b71d04d653d94e8180410417036e5353f8580bc  pi/pi-subagents/references/review-and-validation.md
```

# Phase 6 — rtk, graphify, Skills Ecosystem Compatibility — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The day-1 external tools work as specified. The shell analyser unwraps `rtk <cmd>`, so `rtk` never launders a non-allowlisted command. The built-in `rtk` skill matches the real rtk CLI and is followed in a real session. graphify's SKILL.md, and SKILL.md / prompt-template files written for pi, Claude Code and OpenCode, load unchanged.

**Architecture:** No new packages. `permissions` gains an rtk-aware unwrapping rule driven by a small table of rtk subcommands, captured from the real CLI. `skills` gains a compatibility corpus under `testdata/ecosystem/` plus tolerant-parsing fixes found by it. The built-in skill text is revised against `rtk --help`. Everything else is verification.

**Tech Stack:** Go 1.27.1; the `rtk` and `graphify` CLIs installed locally for the live gates.

**Spec:** `docs/specs/DESIGN.md` (rev 11) — §7 (shell analysis, default allowlist), §9 (skills sources, ecosystem compatibility), §10 (rtk + graphify worked examples), §13 (rtk preference line), phase plan item 6.

**Builds on:** Phases 1–5. Uses `permissions.Shell` (`unwrap`, `classify`, `wordLit`), `skills.ParseFrontmatter/Discover/LoadPrompts/ExpandPrompt`, `skills/builtin/rtk/SKILL.md`, `tools.readTool` with read-only roots, `agent.BuildSystemPrompt`.

## Global Constraints

- `rtk` and `graphify` are in the default allowlist (phase 1 `DefaultShellAllow`).
- `rtk <cmd>` is unwrapped like `env`/`time`: the wrapped command must pass the same analysis (allowlist, hard-deny, `rm` ask-every-time). rtk's own non-wrapping subcommands (reading files, listing, stats) are allowed as `rtk` itself.
- "Ecosystem compatibility is a hard requirement": any SKILL.md written for pi / claude-code / opencode (including graphify's and pi packages) loads **unchanged** — no edits to fixture files to make them pass.
- Skill discovery stays name + description + absolute path in the prompt; bodies are loaded with `read` (§9).
- The system prompt's token-discipline section keeps the line *prefer rtk-prefixed variants where they exist* (phase 2, frozen).

## Review Focus

1. **`rtk` with flags before the subcommand (`rtk -v git status`, `rtk --ultra-compact ls`)** → flags are skipped, and the wrapped/sub command is classified correctly. Tested in Task 2.
2. **`rtk proxy rm -rf x` / `rtk test -- sudo make`** → the wrapped command after `proxy` or `--` is checked (`rm` ask-every-time, `sudo` hard-deny). Tested in Task 2.
3. **A SKILL.md whose `description` contains `: ` (e.g. `description: Use when: the user asks…`) unquoted** → the full text after the first `: ` is the description. Tested in Task 3.
4. **A skill directory with supporting files (`scripts/run.sh`, `references/api.md`) in the global or built-in dir** → the model can `read` them (read-only roots cover the whole skill dir), and `shell` runs `bash ~/.config/moca/skills/x/scripts/run.sh` only if `bash` is allowlisted. Tested in Task 4.
5. **A Claude Code command file (`.claude/commands/review.md`) copied into `~/.config/moca/prompts/`** with `allowed-tools` / `model` frontmatter → loads; unknown keys ignored; `$ARGUMENTS` expands. Tested in Task 5.

---

## File Structure

```
internal/permissions/
  rtk.go rtk_test.go                rtk subcommand table + unwrap rule
  shell.go                          (modified) consult rtk rule in unwrap
internal/skills/
  testdata/ecosystem/<source>/<skill>/SKILL.md   real-world corpus
  testdata/ecosystem/SOURCES.md                  provenance + licenses
  ecosystem_test.go                              corpus must load unchanged
  frontmatter.go                                 (modified only if the corpus finds a gap)
  builtin/rtk/SKILL.md                           (revised against the real CLI)
internal/agent/
  agent_test.go                                  (+ skill-dir read test)
docs/
  external-tools.md                              how rtk/graphify integrate; manual graphify install
```

---

### Task 1: Capture the real rtk CLI surface

**Files:**
- Create: `internal/permissions/rtk.go` (table only in this task)

**Interfaces:**
- Produces: `var rtkSelf map[string]bool` — rtk subcommands that **don't** run another program (e.g. `read`, `ls`, `gain`, `init`, `config`, `help`, `version`). `var rtkProxy map[string]bool` — subcommands whose remaining argv is an arbitrary command (`proxy`). `var rtkDashDash map[string]bool` — subcommands that run the command after `--` (e.g. `test`). Every other first word is treated as a **wrapped command name** (`rtk git …`, `rtk docker …`, `rtk gh …`).

- [x] **Step 1: Inspect the installed rtk**

Run: `rtk --version && rtk --help` and `rtk <sub> --help` for each listed subcommand. For each subcommand, record:
- whether it executes an external program;
- if so, which argv word names that program: the subcommand itself (`git`), the word after `--`, or the rest of the argv (`proxy`).

- [x] **Step 2: Write the table** from that record:

```go
// internal/permissions/rtk.go
package permissions

// rtk subcommand classes, captured from `rtk --help` (rtk vX.Y.Z, YYYY-MM-DD).
// Re-check when bumping the rtk version the built-in skill documents.
var (
	// run no external program: allowed as plain `rtk`
	rtkSelf = map[string]bool{"read": true, "ls": true, "gain": true, "init": true, "config": true, "help": true, "version": true}
	// remaining argv is a command: `rtk proxy <cmd> …`
	rtkProxy = map[string]bool{"proxy": true}
	// command follows `--`: `rtk test -- go test ./...`
	rtkDashDash = map[string]bool{"test": true}
)
```

Replace the example entries with exactly what Step 1 found. Fill in the version/date comment.

- [x] **Step 3: Commit** — `git add internal/permissions/rtk.go && git commit -m "chore(permissions): record rtk subcommand classes from the real CLI"`

---

### Task 2: Unwrap `rtk` in the shell analyser

**Files:**
- Modify: `internal/permissions/rtk.go`, `internal/permissions/shell.go`
- Test: `internal/permissions/rtk_test.go`

**Interfaces:**
- Produces: `func rtkTarget(args []*syntax.Word, i int) (next int, self bool, err error)`. `args[i]` is the `rtk` word. Flags (`-x`, `--long`, `--long=value`) after `rtk` are skipped. Then:
  - an `rtkSelf` sub → `self = true` (classify as `rtk` only);
  - an `rtkProxy` sub → `next` = index of the word after it;
  - an `rtkDashDash` sub → `next` = index after the first `--` (missing `--` → `self = true`: rtk runs its default for that subcommand);
  - otherwise → `next = index of the sub word` (it *is* the wrapped command).
  
  No word left → `self = true`. A non-literal sub word → error `non-literal command name refused`.
- `unwrap` change: when the current command name is `rtk`, first classify `rtk` itself (it must be allowlisted). Then call `rtkTarget`; if `!self`, continue unwrapping from `next` (so `rtk proxy timeout 5 rm x` → `rm`).

`classify` must run for both `rtk` and the wrapped name. Restructure the CallExpr branch so `unwrap` returns the **list** of command-name indexes to classify: wrappers like `env`/`time` stay transparent (not classified), while `rtk` is classified **and** unwrapped.

- [x] **Step 1: Write failing tests**

```go
// internal/permissions/rtk_test.go
package permissions

import (
	"slices"
	"strings"
	"testing"
)

func TestRtkUnwrap(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	s := NewShell([]string{"rtk", "git", "go", "make", "docker"}, j, "linux")
	cases := map[string]verdict{
		"rtk git status":               {},
		"rtk -v git status":            {},
		"rtk --ultra-compact git log":  {},
		"rtk read big.log":             {},
		"rtk ls src":                   {},
		"rtk test -- go test ./...":    {},
		"rtk python x.py":              {need: []string{"python"}},
		"rtk proxy python x":           {need: []string{"python"}},
		"rtk proxy rm -rf build":       {every: []string{"rm"}},
		"rtk proxy timeout 5 rm x":     {every: []string{"rm"}},
		"rtk test -- sudo make":        {deny: "sudo"},
		"rtk $X":                       {deny: "non-literal"},
		"rtk git log | rtk proxy curl": {need: []string{"curl"}},
	}
	for cmd, want := range cases {
		need, every, err := s.Check(cmd)
		if want.deny != "" {
			if err == nil || !strings.Contains(err.Error(), want.deny) {
				t.Errorf("%q: want deny %q, got %v", cmd, want.deny, err)
			}
			continue
		}
		if err != nil || !slices.Equal(need, want.need) || !slices.Equal(every, want.every) {
			t.Errorf("%q: need=%v every=%v err=%v; want %+v", cmd, need, every, err, want)
		}
	}
	noRtk := NewShell([]string{"git"}, j, "linux")
	if need, _, _ := noRtk.Check("rtk git status"); !slices.Equal(need, []string{"rtk"}) {
		t.Fatal("rtk itself must be allowlisted", need)
	}
}
```

Adjust the `rtk read` / `rtk ls` / `rtk test` rows to the subcommands Task 1 recorded. The proxy / `--` / plain-wrapper rows and the deny rows must stay.

- [x] **Step 2: Run** — `go test ./internal/permissions/ -run Rtk` → FAIL.

- [x] **Step 3: Implement**

```go
// internal/permissions/rtk.go — append:

func rtkTarget(args []*syntax.Word, i int) (int, bool, error) {
	j := i + 1
	for j < len(args) {
		w, ok := wordLit(args[j])
		if !ok {
			return 0, false, fmt.Errorf("non-literal command name refused (e.g. $CMD); write the command out")
		}
		if !strings.HasPrefix(w, "-") {
			break
		}
		j++
	}
	if j >= len(args) {
		return 0, true, nil
	}
	sub, _ := wordLit(args[j])
	switch {
	case rtkSelf[sub]:
		return 0, true, nil
	case rtkProxy[sub]:
		if j+1 >= len(args) {
			return 0, true, nil
		}
		return j + 1, false, nil
	case rtkDashDash[sub]:
		for k := j + 1; k < len(args); k++ {
			if w, _ := wordLit(args[k]); w == "--" {
				if k+1 < len(args) {
					return k + 1, false, nil
				}
				return 0, true, nil
			}
		}
		return 0, true, nil
	}
	return j, false, nil
}
```

(Add imports `fmt`, `strings`, `mvdan.cc/sh/v3/syntax` to `rtk.go`.)

In `shell.go`, change `unwrap` to return `([]int, error)`: the indexes of every name that must be classified.

```go
// unwrap walks wrapper commands and rtk. Transparent wrappers (env, time,
// timeout, nice, nohup, command) are not classified; rtk is classified and
// then unwrapped. Returns indexes into args to classify.
func unwrap(args []*syntax.Word) ([]int, error) {
	var out []int
	i := 0
	for i < len(args) {
		name, ok := wordLit(args[i])
		if !ok {
			return append(out, i), nil // caller reports non-literal
		}
		switch filepath.Base(name) {
		case "rtk":
			out = append(out, i)
			next, self, err := rtkTarget(args, i)
			if err != nil {
				return nil, err
			}
			if self {
				return out, nil
			}
			i = next
			continue
		case "env", "time", "nohup", "nice", "timeout", "command":
			n, lookup := skipWrapper(args, i) // the phase-2 per-wrapper flag skipping, extracted
			if lookup {
				return out, nil
			}
			i = n
			continue
		}
		return append(out, i), nil
	}
	return out, nil
}
```

Extract phase 2's per-wrapper switch bodies into `skipWrapper(args, i) (next int, lookupOnly bool)` unchanged. In the `CallExpr` branch, loop over the returned indexes:
- a non-literal word → the non-literal error;
- otherwise `classify(name)`;
- if any index names `tee`, apply the tee target check from that index.

- [x] **Step 4: Run** — `go test ./internal/permissions/ -v` → PASS. The phase-2 ladder must still pass unchanged.

- [x] **Step 5: Commit**

```bash
git add internal/permissions
git commit -m "feat(permissions): unwrap rtk so it never launders a non-allowlisted command"
```

---

### Task 3: Ecosystem SKILL.md corpus

**Files:**
- Create: `internal/skills/testdata/ecosystem/…`, `internal/skills/testdata/ecosystem/SOURCES.md`, `internal/skills/ecosystem_test.go`
- Modify: `internal/skills/frontmatter.go` only if a corpus file fails

**Interfaces:**
- Produces: `TestEcosystemSkillsLoadUnchanged` — every `testdata/ecosystem/<source>/<skill>/SKILL.md` loads via `Discover`, with a non-empty description equal to the expected value in `testdata/ecosystem/expected.json` (`{"<source>/<skill>": {"name": …, "description": …}}`).

- [x] **Step 1: Assemble the corpus (verbatim copies, never edited)**

Collect at least these, byte-for-byte. Keep each skill's directory structure, but SKILL.md plus at most one supporting file is enough. Record the URL, commit and license of each in `SOURCES.md`:
- `graphify/graphify` — graphify's SKILL.md (from its repo, or from `graphify install` output for another platform);
- `claude-code/<2 skills>` — from the public Anthropic skills repo, picking one that has supporting files;
- `pi/<2 skills>` — including one distributed as a **pi package**;
- `opencode/<1 skill>`;
- `synthetic/edge-cases` — written for this test (not third-party):
  - CRLF line endings;
  - a BOM;
  - folded `description: >`;
  - literal `description: |`;
  - an unquoted description containing `: `;
  - `allowed-tools: [Bash, Read]`;
  - a nested `metadata:` map;
  - a `license:` key;
  - a 1,200-character description (kept whole — the prompt shows it whole; don't truncate silently).

Only include third-party files whose licenses permit redistribution. If one doesn't, record the expected name/description in `expected.json`, keep the file **out** of the repo, and test it in the live gate instead.

- [x] **Step 2: Write the test**

```go
// internal/skills/ecosystem_test.go
package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEcosystemSkillsLoadUnchanged(t *testing.T) {
	var expected map[string]struct{ Name, Description string }
	b, err := os.ReadFile("testdata/ecosystem/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(b, &expected)
	sources, _ := os.ReadDir("testdata/ecosystem")
	seen := 0
	for _, src := range sources {
		if !src.IsDir() {
			continue
		}
		got, errs := Discover([]Dir{{Path: filepath.Join("testdata/ecosystem", src.Name()), Source: src.Name()}})
		for _, e := range errs {
			t.Errorf("%s: %v", src.Name(), e)
		}
		for _, s := range got {
			key := src.Name() + "/" + filepath.Base(filepath.Dir(s.Path))
			want, ok := expected[key]
			if !ok {
				t.Errorf("%s: no expected entry", key)
				continue
			}
			if s.Name != want.Name || s.Description != want.Description {
				t.Errorf("%s:\n got  %q / %q\n want %q / %q", key, s.Name, s.Description, want.Name, want.Description)
			}
			seen++
		}
	}
	if seen < len(expected) {
		t.Fatalf("loaded %d of %d expected skills", seen, len(expected))
	}
}
```

- [x] **Step 3: Run** — `go test ./internal/skills/ -run Ecosystem -v`. For every failure, fix **the parser** (`frontmatter.go`), never the fixture. Add a focused unit test in `frontmatter_test.go` for each fix. Likely gaps:
  - indentation-sensitive folded blocks that contain blank lines (keep paragraph breaks as `\n` for `|`, as a single space for `>`);
  - `description: "multi \"quoted\""` escapes (unquote with `strconv.Unquote` for double quotes; `''` → `'` for single quotes);
  - list-valued keys written inline vs as `- item` blocks (both skipped).

- [x] **Step 4: Commit**

```bash
git add internal/skills
git commit -m "test(skills): ecosystem SKILL.md corpus (graphify, claude-code, pi, opencode) loads unchanged"
```

---

### Task 4: Skill supporting files are readable; built-in rtk skill verified

**Files:**
- Modify: `internal/skills/builtin/rtk/SKILL.md`
- Test: `internal/agent/agent_test.go` (extend)

**Interfaces:**
- No new API. This task verifies that phase 2's read-only roots cover entire skill directories, and makes the rtk skill text match the real CLI.

- [x] **Step 1: Write the failing/guard test**

```go
func TestModelCanReadGlobalSkillSupportFiles(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "moca", "skills", "deploy", "references")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "api.md"), []byte("API NOTES"), 0o644)
	r := a.opts.Tools.Run(context.Background(), a.opts.Env, llm.ToolCall{Name: "read",
		Input: json.RawMessage(fmt.Sprintf(`{"path":%q}`, filepath.Join(dir, "api.md")))})
	if r.IsError || !strings.Contains(r.Content, "API NOTES") {
		t.Fatal(r.Content)
	}
	w := a.opts.Tools.Run(context.Background(), a.opts.Env, llm.ToolCall{Name: "write",
		Input: json.RawMessage(fmt.Sprintf(`{"path":%q,"content":"x"}`, filepath.Join(dir, "api.md")))})
	if !w.IsError {
		t.Fatal("skill dirs are read-only")
	}
}
```

This test sets up the skill dir **after** `Start`. That works because the jail's read-only roots are directory prefixes. If `NewJail` dropped the root because it didn't exist yet at `canonical()` time, fix `NewJail`: keep non-existent read-only roots as `filepath.Abs` paths instead of skipping them. Then make the test pass.

- [x] **Step 2: Run** — `go test ./internal/agent/ -run SkillSupport -v` → fix `NewJail` if needed → PASS.

- [x] **Step 3: Revise `builtin/rtk/SKILL.md`** against Task 1's record:
  - every command shown must exist in the installed rtk;
  - the "Instead of / Use" table uses real subcommands;
  - the fallback instruction names the real raw-output escape (`rtk proxy <cmd>` if that's what rtk provides);
  - the allowlist note matches Task 2 (`rtk <cmd>` is allowed only when `<cmd>` is).
  
  Keep it under ~40 lines: every session pays for the description, and the body when loaded.

- [x] **Step 4: Bump the extraction version**

Built-ins extract to `builtin-skills/<mocaVersion>/`. During development (`0.0.0-dev`), an existing extraction would shadow the revised file. Change `ExtractBuiltins` to also re-extract when the embedded content hash differs from a `.hash` file in the target dir. Add a test: extract, then modify the embedded content via a test-only override var, then extract again → the file is updated.

- [x] **Step 5: Run** — `go test ./... -race` → PASS. **Commit**

```bash
git add internal/skills internal/permissions internal/agent
git commit -m "feat(skills): rtk skill verified against the real CLI; built-ins re-extract on content change"
```

---

### Task 5: Prompt-template compatibility (Claude Code commands, pi prompts)

**Files:**
- Create: `internal/skills/testdata/prompts/claude-review.md`, `internal/skills/testdata/prompts/pi-commit.md`
- Test: `internal/skills/prompts_test.go` (extend)

- [x] **Step 1: Fixtures**

`claude-review.md` (Claude Code command shape):

```markdown
---
allowed-tools: Bash(git diff:*), Read
argument-hint: [file]
description: Review a file for bugs
model: claude-opus-x
---
Review $ARGUMENTS for bugs. Focus on: $1
```

`pi-commit.md` (pi prompt-template shape, body using positional args):

```markdown
---
description: Write a commit message
---
Write a conventional commit message for the staged changes. Scope: $1. Extra: $@
```

- [x] **Step 2: Test**

```go
func TestEcosystemPromptsLoad(t *testing.T) {
	ps := LoadPrompts([]Dir{{Path: "testdata/prompts", Source: "global"}})
	if len(ps) != 2 || ps[0].Name != "claude-review" || ps[0].Description != "Review a file for bugs" || ps[0].ArgumentHint != "[file]" {
		t.Fatalf("%+v", ps)
	}
	if got := ExpandPrompt(ps[0].Body, "main.go"); got != "Review main.go for bugs. Focus on: main.go\n" {
		t.Fatalf("%q", got)
	}
	if got := ExpandPrompt(ps[1].Body, "api fixes"); got != "Write a conventional commit message for the staged changes. Scope: api. Extra: api fixes\n" {
		t.Fatalf("%q", got)
	}
}
```

- [x] **Step 3: Run** — PASS (fix the parser if not, with a unit test). **Commit** — `git commit -am "test(skills): Claude Code / pi prompt templates load and expand"`

---

### Task 6: Docs + live gates

**Files:**
- Create: `docs/external-tools.md`

- [x] **Step 1: Write `docs/external-tools.md`** (short):
  - how moca integrates CLIs (allowlist + skill);
  - rtk: what the built-in skill does, how the analyser treats `rtk <cmd>`;
  - graphify: until `graphify install --platform moca` lands upstream (phase 7), install manually with `mkdir -p ~/.config/moca/skills/graphify && cp <graphify skill dir>/* ~/.config/moca/skills/graphify/` (give the exact source path that graphify uses for other platforms);
  - using skills/prompts from Claude Code / pi / OpenCode: copy or symlink their dirs into `~/.config/moca/skills/` and `~/.config/moca/prompts/`.

- [x] **Step 2: Live gate — rtk preferred in a real session**

With rtk installed, in a scratch Go repo with a failing test:
`moca -p 'run the tests and tell me what fails' --model opencode-go/glm-5.3-flash`
Expected: stderr shows a `shell rtk test -- go test ./…` (or `rtk go test`, per the real CLI), not bare `go test`, and the session file's tool_use entries confirm it. If the model doesn't prefer rtk, first check that the rtk skill is in the stored system prompt. Then tune the **skill's description** (the only part always in context) and re-run. Do not touch the frozen system prompt.

- [x] **Step 3: Live gate — rtk can't launder**

`moca -p 'run: rtk proxy python3 -c "print(1)"'` with `python3` not allowlisted → refused in `-p` (`✗ shell refused: python3 …`).

- [x] **Step 4: Live gate — graphify SKILL.md loads unchanged**

Copy graphify's skill dir into `~/.config/moca/skills/graphify/` (no edits). Then `moca -p 'what does the graphify skill do? read it'` → stderr shows `▸ read …/graphify/SKILL.md`, and the answer reflects its content. In a repo with `graphify-out/graph.json`, ask a codebase question → the model runs `graphify query "…"` (allowlisted).

- [x] **Step 5: Live gate — pi package skill + Claude Code skill**

Point `~/.config/moca/skills/` at (symlink) one pi-package skill and one Claude Code skill. Both appear in the stored system prompt's skill list, and asking for each triggers a `read` of its SKILL.md.

- [x] **Step 6: README + commit**: status `phase 6 done — rtk, graphify, ecosystem skills`; `git add docs README.md && git commit -m "docs: external tools guide; phase 6 gate passed"`.

## Implementation notes (2026-10-06)

**The real rtk CLI is a superset of the plan's assumptions (rtk 0.51.0, 88 subcommands).** Probes: `rtk test echo x` runs the command *directly* — the plan's "missing `--` → self" reading was wrong and would have been a laundering hole; `rtk err`, `rtk summary` and `rtk run` also take the command as the first positional word; `rtk run -c` / `--shell` take a shell command *string*; `rtk find … -exec …` forwards execution (the same trust as running `find` directly — the analyser never inspects a tool's own argv; noted in `rtk.go`). Task 2 therefore unified `proxy`/`test`/`err`/`summary`/`run` into one runner class — the first non-flag word (or the word after `--`) is classified; shell-string options and unknown options refuse; the plan's `rtkDashDash` sketch was replaced.

**Task 4 Step 4 was already solved differently.** Phase 2 shipped content-hash-keyed extraction (`builtin-skills/<hash>`), not `builtin-skills/<mocaVersion>/`; the `.hash`-file scheme was unnecessary. `ExtractBuiltins` now delegates to `extractBuiltins(fsys, …)`, and a MapFS test pins re-extraction on content change.

**Task 4's jail fix was real.** `NewJail` silently dropped read-only roots that did not exist yet (`EvalSymlinks` failed on them); switched to `resolveDeep`. `TestJailKeepsNotYetExistingReadOnlyRoot` fails against the pre-fix code.

**Corpus provenance.** Vendored from the real installed distributions: graphify's skill from its `install` output (Apache-2.0 OR MIT; the claude and pi installs are byte-identical, the opencode variant differs in platform dispatch wording), `anthropics/skills` at commit `683bc88e56f3e09ba94f7055977f3d3aa499f202` (Apache-2.0; skill-creator brings supporting files), npm `pi-subagents` 0.76.1 + `pi-mcp-adapter` 5.1.0 (MIT), an OpenCode-format skill from the author's dotfiles (MIT-declared). Anthropic's document skills are source-available → deliberately not vendored. SHA-256s in `SOURCES.md`. Parser fixes the corpus forced: folded `>` blank lines (double space → single space); from the plan's gap list: Go-style unescaping for well-formed double-quoted values, `''` collapsing for single-quoted ones (unit tests in `frontmatter_test.go`).

**Gate deviation (plan Task 6 Step 5).** The plan said to symlink the pi / Claude Code skills into `~/.config/moca/skills`; symlinked skill directories are skipped by design (phase 2 — bodies load through the read jail), so the gate copied them.

**Gate results (scripted provider, real CLIs; scratch `moca_gate6/`).** A: graphify + pi-subagents + skill-creator staged skills all appear in the stored system prompt (plus the `- rtk:` line and the rtk-preference line); their bodies and graphify's `references/query.md` load through `read`. B: `rtk proxy python3 -c 'print(1)'` refused (`python3 is not allowlisted…`) while `rtk ls .` / `rtk read note.txt` ran (exit 0). C: `graphify --version` → `graphify 0.9.77`; `graphify query "what runs the agent loop?"` → a real traversal of the committed graph (1629 nodes, 345 found, budget-truncated).

**Live legs (2026-10-06, real OpenCode Go `glm-5.3-flash`; key from the repo `.env` via `env:OPENCODE_GO_KEY`; scratch `moca_gate6live/`).** First run: with the original skill description the model ran bare `go test` (and PATH-fumbled — `go` is mise-only on this host, so the harness now exports it). The rtk skill **description was sharpened** with concrete wrapper examples (`rtk test go test ./...`, `rtk err <cmd>`, `rtk read <file>`, …) — rerun: `rtk test -- go test ./...` ✓. `rtk proxy python3 -c 'print(1)'` → refused verbatim by the allowlist ✓. graphify: the model read its SKILL.md, ran `graphify query "What runs the agent loop?"` against the committed graph, and source-verified with `rtk grep` ✓ — and the analyser correctly refused an attempted `"$PYTHON" -c …` bootstrap (non-literal command name) before the model recovered and queried directly. pi-subagents (npm pi package) + skill-creator (Claude Code) read and summarized ✓. Transient Go-tier flake seen twice before any token was spent: `HTTP 400 … [1210] This model always engages in thinking…`; an immediate retry succeeded, and direct curl probes of every `reasoning_effort` value returned 200 (backend-dependent, not request shape).

**Docs.** `docs/external-tools.md` (rtk/graphify semantics + how to install skills and prompts from other tools), README status, SPECS §1/§11/§12/§15.


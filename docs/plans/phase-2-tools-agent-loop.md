# Phase 2 — Seven Tools, Agent Loop, Permissions, Skills, Session Schema — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `moca -p "<task>"` runs a real agent loop: the seven frozen tools behind a symlink-resolving path jail and a parsed-shell command analyser, project trust, skills + AGENTS.md in a once-built system prompt, sequential tool execution, maxSteps wrap-up, and every step persisted to an append-only JSONL session with pre-edit snapshots.

**Architecture:** `tools` declares the `Tool` interface plus the small checker/snapshot interfaces it needs, and implements the six local tools and the `mcp` stub. `permissions` implements those checker interfaces structurally, without importing `tools`. `session` owns the transcript schema and the snapshot store. `skills` loads SKILL.md files, extracts embedded built-ins, and loads AGENTS.md. `agent` wires everything: it builds the system prompt once, runs the model → tools → model loop, persists entries, and emits events. `cmd/moca` swaps phase 1's direct adapter call for `agent`.

**Tech Stack:** Go 1.27.1 stdlib + `mvdan.cc/sh/v3/syntax` (the one non-UI third-party dependency, §7). `embed`, `regexp` (RE2), `os/exec`, `crypto/sha256`.

**Spec:** `docs/specs/DESIGN.md` (rev 9) — §2, §4, §5, §7, §8, §9, §9.5, §12.5, §13, phase plan item 2, edit-ladder + shell-analysis ladder.

**Builds on:** Phase 1 (`docs/plans/phase-1-skeleton-providers.md`). Uses `llm.*`, `config.Config/EnvRefs/DataDir/ConfigDir/Version`, `provider.Registry/Model/Adapter/RetryNotice`, `cmd/moca` `Options`/exit constants.

## Global Constraints

- Dependency direction (§2): `tools`, `permissions`, `session`, `skills` never import each other. `permissions` and `session` satisfy `tools` interfaces **structurally**, so those interface methods use only builtin, stdlib, `llm` or `config` types. `agent` imports everything.
- Tool schemas are **frozen at the end of this phase** (§4): names `read write edit shell search ls mcp`. The parameters below are final. A golden test locks them.
- `read`: caps 2000 lines / 50K chars per call; each line truncated at 2000 chars with `[… line truncated]`; NUL byte in first 8K → refused with size + MIME guess.
- `write`/`edit`: new files allowed; existing files need a prior `read` in this session and must be unchanged on disk since it (mtime + size + sha256).
- `shell`: `bash -c` (non-login), `pwsh -NoProfile -Command` → fallback `powershell.exe -NoProfile -Command`; stateless (cwd = jail root every call); stdin `/dev/null`; env adds `PAGER=cat GIT_PAGER=cat GIT_EDITOR=true GIT_TERMINAL_PROMPT=0` and removes every `config.EnvRefs` variable; own process group, killed whole on timeout/cancel; timeout default 30s, max 300s; output head+tail truncated at 30K chars with `[… N lines omitted]`.
- `search`: RE2 regex, respects `.gitignore`/`.ignore`, skips hidden + binary, 200-hit cap, `files_only`.
- Shell analysis hard-deny (no override): `sudo su doas dd shred mkfs* chown`. `rm` ask every time (never allowlistable, refused in `-p`). Implicit builtins: `cd pwd echo printf test [ true false exit`. Refused: `eval source . exec`, non-literal command names, non-literal redirect targets, unparseable input.
- Project trust gates `<workdir>/.moca/**` and `<workdir>/AGENTS.md`/`CLAUDE.md`. `-p` default is **no** unless `--approve` (§7).
- Session files `~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl`, file 0600, dir 0700; every entry has `id` + `parentId`.
- Snapshots in `~/.local/share/moca/snapshot/` (hash-addressed); skipped for git-tracked clean files; prune older than `snapshot.retentionDays` at startup.
- `maxSteps` default 40; the wrap-up request uses `tool_choice: none` and never removes tools.
- Yolo mode (§7.5): `--yolo` / config `yolo` turns **every** permission check off (jail, shell analysis incl. hard-deny, approvals, MCP gating, project trust unless `--no-approve` is explicit). `maxSteps`, read-before-write guards, snapshots and shell env hygiene stay on. The session records it (`session.yolo`, `permission_mode`).
- Tool calls in one turn execute sequentially in emitted order; a failed call does not cancel the rest.
- AGENTS.md / CLAUDE.md capped at 32K chars each, with a visible truncation note.
- System prompt: built once, stored in the `session` entry, never rebuilt; no hostname.

## Review Focus

1. **`edit` on a file whose content the model copied from `read` output with the `N|` prefixes still attached** → no match. The error must say to drop the line-number prefixes, not just "not found". Tested in Task 6.
2. **`write` into a directory that doesn't exist yet (`internal/new/pkg/x.go`)** → parents created inside the jail; a parent path that escapes via a symlink is refused before any mkdir. Tested in Task 7.
3. **`shell` command producing no trailing newline, binary garbage, or >30K output on stderr only** → output is merged stdout+stderr, valid UTF-8 (invalid bytes replaced), head+tail truncated, and the exit code is always reported. Tested in Task 9.
4. **A tool call whose arguments are invalid JSON (truncated at `max_tokens`) or name an unknown tool** → error result telling the model what to do; the loop continues, and the transcript stays valid to resend. Tested in Tasks 1 and 15.
5. **SKILL.md with Windows line endings, a BOM, folded (`>`) or literal (`|`) multi-line `description`, or unknown frontmatter keys (`allowed-tools`, `metadata:` maps)** → loads with the right description, nothing crashes. Tested in Task 12.

---

## File Structure

```
internal/tools/
  tool.go            Tool, Result, Env, PathChecker, CommandChecker, Snapshotter, Asker, Registry
  tool_test.go       registry dispatch + schema golden test
  testdata/schemas.golden.json
  readtracker.go     ReadTracker (stamp at read; check before write)
  read.go  read_test.go
  ls.go    ls_test.go
  ignore.go ignore_test.go     gitignore matcher
  search.go search_test.go
  editapply.go editapply_test.go   pure edit algorithm (edit ladder)
  write.go edit.go writeedit_test.go
  shellrun.go shellrun_unix.go shellrun_windows.go shellrun_test.go
  shell.go shell_test.go
  mcpstub.go
  builtins.go        Builtins()
internal/permissions/
  jail.go jail_test.go
  shell.go shell_test.go         Unix analysis (mvdan.cc/sh)
  shell_windows_rules.go         best-effort segment checker (all platforms; selected by goos)
  trust.go trust_test.go
internal/skills/
  frontmatter.go frontmatter_test.go
  skills.go skills_test.go
  builtin.go builtin/rtk/SKILL.md
  instructions.go instructions_test.go
internal/session/
  entry.go           Entry + payload types
  writer.go writer_test.go
  rebuild.go rebuild_test.go     Messages(), Repair()
  snapshot.go snapshot_test.go   Snapshots (Guard/Undo/Prune)
internal/agent/
  prompt.go prompt_test.go
  events.go
  agent.go agent_test.go
  start.go           Start(): wires jail, shell, trust, skills, session, prompt
cmd/moca/
  oneshot.go         rewritten on top of agent
  cli_test.go        extended
```

---

### Task 1: Tool interface, registry, schema golden

**Files:**
- Create: `internal/tools/tool.go`, `internal/tools/builtins.go` (initially empty slice)
- Test: `internal/tools/tool_test.go`

**Interfaces:**
- Consumes: `llm.ToolSpec`, `llm.ToolCall`.
- Produces:

```go
type Result struct {
	Content string // what the model sees
	IsError bool
	Summary string // UI one-liner suffix, e.g. "main.go [+3 −1]"
	Detail  string // pager body (diff, full output); "" → Content
}
type Tool interface {
	Spec() llm.ToolSpec
	Run(ctx context.Context, env *Env, input json.RawMessage) Result
}
type PathChecker interface {
	Resolve(path string, write bool) (string, error) // absolute, symlink-resolved, jail-checked
}
type CommandChecker interface {
	Check(command string) (needApproval, askEveryTime []string, err error)
	Allow(name string)
}
type Snapshotter interface {
	Guard(absPath string, write func() error) error
}
type Answer int // Deny, AllowOnce, AllowAlways
type Question struct { Kind, Subject, Detail string; CanAlways bool }
type Asker func(ctx context.Context, q Question) Answer
type Env struct {
	Root     string
	Paths    PathChecker
	Commands CommandChecker
	Ask      Asker
	Reads    *ReadTracker
	Snap     Snapshotter
	ShellEnv []string
}
func NewRegistry(ts ...Tool) *Registry
func (r *Registry) Register(t Tool)          // replace by name, keep position
func (r *Registry) Specs() []llm.ToolSpec    // stable order
func (r *Registry) Run(ctx context.Context, env *Env, call llm.ToolCall) Result
func decode[T any](input json.RawMessage, v *T) *Result // nil on success; error Result otherwise
func errorf(format string, a ...any) Result
```

- [ ] **Step 1: Write the failing test**

```go
// internal/tools/tool_test.go
package tools

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

var update = flag.Bool("update", false, "rewrite golden files")

type echoTool struct{ name string }

func (e echoTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: e.name, Description: "echo", Schema: json.RawMessage(`{"type":"object"}`)}
}
func (e echoTool) Run(_ context.Context, _ *Env, in json.RawMessage) Result {
	var v struct{ Msg string `json:"msg"` }
	if r := decode(in, &v); r != nil {
		return *r
	}
	return Result{Content: e.name + ":" + v.Msg}
}

func TestRegistryDispatch(t *testing.T) {
	r := NewRegistry(echoTool{"a"}, echoTool{"b"})
	if got := r.Run(context.Background(), &Env{}, llm.ToolCall{Name: "b", Input: json.RawMessage(`{"msg":"x"}`)}); got.Content != "b:x" {
		t.Fatal(got)
	}
	unk := r.Run(context.Background(), &Env{}, llm.ToolCall{Name: "zz", Input: json.RawMessage(`{}`)})
	if !unk.IsError || !strings.Contains(unk.Content, "read, ") && !strings.Contains(unk.Content, "a, b") {
		t.Fatalf("unknown tool must list available tools: %+v", unk)
	}
	bad := r.Run(context.Background(), &Env{}, llm.ToolCall{Name: "a", Input: json.RawMessage(`{"msg":"x`)})
	if !bad.IsError || !strings.Contains(bad.Content, "invalid JSON") || !strings.Contains(bad.Content, "smaller") {
		t.Fatalf("truncated args: %+v", bad)
	}
	r.Register(echoTool{"a"})
	if len(r.Specs()) != 2 || r.Specs()[0].Name != "a" {
		t.Fatal("Register replaces in place")
	}
}

func TestSchemasFrozen(t *testing.T) {
	got, _ := json.MarshalIndent(NewRegistry(Builtins()...).Specs(), "", "  ")
	const golden = "testdata/schemas.golden.json"
	if *update {
		os.WriteFile(golden, got, 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("tool schemas changed. They are frozen after phase 2 (DESIGN.md §4): " +
			"a change is a v2 discussion. Run with -update only during phase 2.")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/tools/` → FAIL (`undefined: NewRegistry`).

- [ ] **Step 3: Implement `tool.go` + empty `builtins.go`**

```go
// internal/tools/tool.go

// Package tools defines the Tool interface and the built-in tools. It never
// imports permissions or session; those implement the small interfaces below
// structurally and agent injects them through Env.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type Result struct {
	Content string
	IsError bool
	Summary string
	Detail  string
}

type Tool interface {
	Spec() llm.ToolSpec
	Run(ctx context.Context, env *Env, input json.RawMessage) Result
}

type PathChecker interface {
	Resolve(path string, write bool) (string, error)
}

type CommandChecker interface {
	Check(command string) (needApproval, askEveryTime []string, err error)
	Allow(name string)
}

type Snapshotter interface {
	Guard(absPath string, write func() error) error
}

type Answer int

const (
	Deny Answer = iota
	AllowOnce
	AllowAlways
)

// Question is an approval request. Kind is "shell" or "mcp"; Subject the
// command name or server/tool; Detail the full command or args.
type Question struct {
	Kind, Subject, Detail string
	CanAlways             bool
}

type Asker func(ctx context.Context, q Question) Answer

type Env struct {
	Root     string
	Paths    PathChecker
	Commands CommandChecker
	Ask      Asker
	Reads    *ReadTracker
	Snap     Snapshotter
	ShellEnv []string
}

type Registry struct {
	order []string
	tools map[string]Tool
}

func NewRegistry(ts ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range ts {
		r.Register(t)
	}
	return r
}

func (r *Registry) Register(t Tool) {
	name := t.Spec().Name
	if _, ok := r.tools[name]; !ok {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

func (r *Registry) Specs() []llm.ToolSpec {
	out := make([]llm.ToolSpec, len(r.order))
	for i, n := range r.order {
		out[i] = r.tools[n].Spec()
	}
	return out
}

func (r *Registry) Run(ctx context.Context, env *Env, call llm.ToolCall) Result {
	t, ok := r.tools[call.Name]
	if !ok {
		return errorf("unknown tool %q; available: %s", call.Name, strings.Join(r.order, ", "))
	}
	if !json.Valid(call.Input) {
		return errorf("invalid JSON arguments for %s (the call was probably cut off at the output limit). "+
			"Split the work into smaller calls, e.g. write a large file in parts with edit.", call.Name)
	}
	return t.Run(ctx, env, call.Input)
}

func decode[T any](input json.RawMessage, v *T) *Result {
	dec := json.NewDecoder(strings.NewReader(string(input)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		r := errorf("invalid arguments: %v", err)
		return &r
	}
	return nil
}

func errorf(format string, a ...any) Result {
	return Result{Content: fmt.Sprintf(format, a...), IsError: true}
}
```

```go
// internal/tools/builtins.go
package tools

// Builtins returns the seven frozen tools in schema order (§4). Tasks 3–10
// add each tool here.
func Builtins() []Tool { return []Tool{} }
```

- [ ] **Step 4: Create an empty golden and run**

Run: `mkdir -p internal/tools/testdata && echo '[]' > internal/tools/testdata/schemas.golden.json && go test ./internal/tools/ -update && go test ./internal/tools/`
Expected: PASS. The golden is regenerated with `-update` at the end of Task 10, once all seven tools exist.

- [ ] **Step 5: Commit**

```bash
git add internal/tools
git commit -m "feat(tools): Tool interface, registry, injected checker interfaces, schema golden"
```

---

### Task 2: Path jail with read-only roots

**Files:**
- Create: `internal/permissions/jail.go`
- Test: `internal/permissions/jail_test.go`

**Interfaces:**
- Produces: `func NewJail(root string, readOnly []string) (*Jail, error)`; `func (j *Jail) Resolve(path string, write bool) (string, error)` (satisfies `tools.PathChecker`); `func (j *Jail) Root() string`.
- Rules: `root` and each read-only root are canonicalized once (abs + `EvalSymlinks`). A relative `path` joins `root`. `~` / `~/x` expands to `$HOME`. The target resolves with `EvalSymlinks` on the deepest existing ancestor, and the remaining components are appended. Allowed when inside `root`, or (only if `!write`) inside a read-only root. On Windows the comparison is case-insensitive and slash-normalized.

- [ ] **Step 1: Write failing tests**

```go
// internal/permissions/jail_test.go
package permissions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (root, outside, ro string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "work")
	outside = filepath.Join(base, "outside")
	ro = filepath.Join(base, "skills")
	for _, d := range []string{root, outside, ro, filepath.Join(root, "sub")} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o644)
	os.WriteFile(filepath.Join(ro, "SKILL.md"), []byte("k"), 0o644)
	os.Symlink(outside, filepath.Join(root, "escape"))
	os.Symlink(filepath.Join(root, "sub"), filepath.Join(root, "inner"))
	return
}

func TestJail(t *testing.T) {
	root, outside, ro := setup(t)
	j, err := NewJail(root, []string{ro})
	if err != nil {
		t.Fatal(err)
	}
	ok := []struct {
		p     string
		write bool
	}{
		{"a.go", true}, {"sub/new/deep.go", true}, {"inner/x", true},
		{filepath.Join(root, "sub"), false}, {filepath.Join(ro, "SKILL.md"), false},
	}
	for _, c := range ok {
		if _, err := j.Resolve(c.p, c.write); err != nil {
			t.Errorf("Resolve(%q, %v) refused: %v", c.p, c.write, err)
		}
	}
	bad := []struct {
		p     string
		write bool
	}{
		{"../outside/secret", false}, {filepath.Join(outside, "secret"), false},
		{"escape/secret", false}, {"escape/new.txt", true},
		{filepath.Join(ro, "SKILL.md"), true}, {"/etc/passwd", false},
	}
	for _, c := range bad {
		if _, err := j.Resolve(c.p, c.write); err == nil {
			t.Errorf("Resolve(%q, %v) must be refused", c.p, c.write)
		} else if !strings.Contains(err.Error(), "outside") {
			t.Errorf("error should explain the jail: %v", err)
		}
	}
}

func TestJailResolvedPathIsCanonical(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	got, _ := j.Resolve("inner/f.go", true)
	if got != filepath.Join(j.Root(), "sub", "f.go") {
		t.Fatalf("symlink must resolve: %s", got)
	}
}

func TestJailTildeExpands(t *testing.T) {
	root, _, _ := setup(t)
	t.Setenv("HOME", root)
	j, _ := NewJail(root, nil)
	if _, err := j.Resolve("~/x.txt", true); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/permissions/` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/permissions/jail.go

// Package permissions implements the path jail, shell command analysis and
// project trust. It implements interfaces declared in tools without
// importing it (§2).
package permissions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Jail struct {
	root     string
	readOnly []string
}

func NewJail(root string, readOnly []string) (*Jail, error) {
	r, err := canonical(root)
	if err != nil {
		return nil, fmt.Errorf("jail root: %w", err)
	}
	j := &Jail{root: r}
	for _, ro := range readOnly {
		if c, err := canonical(ro); err == nil {
			j.readOnly = append(j.readOnly, c)
		}
	}
	return j, nil
}

func (j *Jail) Root() string { return j.root }

func canonical(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveDeep evaluates symlinks on the deepest existing ancestor and
// re-appends the non-existent tail (new files, new dirs).
func resolveDeep(p string) (string, error) {
	var tail []string
	cur := p
	for {
		r, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(append([]string{r}, tail...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p, nil
		}
		tail = append([]string{filepath.Base(cur)}, tail...)
		cur = parent
	}
}

func within(base, p string) bool {
	if runtime.GOOS == "windows" {
		base, p = strings.ToLower(filepath.ToSlash(base)), strings.ToLower(filepath.ToSlash(p))
		return p == base || strings.HasPrefix(p, base+"/")
	}
	return p == base || strings.HasPrefix(p, base+string(filepath.Separator))
}

func (j *Jail) Resolve(path string, write bool) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(j.root, path)
	}
	res, err := resolveDeep(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if within(j.root, res) {
		return res, nil
	}
	if !write {
		for _, ro := range j.readOnly {
			if within(ro, res) {
				return res, nil
			}
		}
		return "", fmt.Errorf("%s is outside the workdir %s (symlinks resolved); reading is limited to the workdir and skill directories", path, j.root)
	}
	return "", fmt.Errorf("%s is outside the workdir %s (symlinks resolved); writes are confined to the workdir", path, j.root)
}
```

- [ ] **Step 4: Run** — `go test ./internal/permissions/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/permissions
git commit -m "feat(permissions): symlink-resolving path jail with read-only skill roots"
```

---

### Task 3: `read` tool + ReadTracker

**Files:**
- Create: `internal/tools/readtracker.go`, `internal/tools/read.go`; modify `builtins.go`
- Test: `internal/tools/read_test.go`

**Interfaces:**
- Produces:
  - `type ReadTracker struct{…}`; `func NewReadTracker() *ReadTracker`; `func (t *ReadTracker) Record(abs string) error`; `func (t *ReadTracker) Check(abs string) error` (nil if never-existed file; error if exists and not read, or changed since).
  - `read` schema: `{"path": string (required), "offset": integer ≥1 (1-based line), "limit": integer ≥1}`.
  - Output: `N|content` lines, then a footer `[lines A-B of TOTAL]` (plus `— use offset=B+1 to continue` when more remain).
  - Test helper (used by later tasks' tests): `func testEnv(t *testing.T) (*Env, string)` in `read_test.go` builds an `Env` over a temp dir with a permissive fake `PathChecker`.

- [ ] **Step 1: Write failing tests**

```go
// internal/tools/read_test.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rootChecker: minimal PathChecker for tool tests (the real jail is tested in permissions).
type rootChecker struct{ root string }

func (c rootChecker) Resolve(p string, _ bool) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(c.root, p)
	}
	if !strings.HasPrefix(p, c.root) {
		return "", fmt.Errorf("%s is outside the workdir", p)
	}
	return filepath.Clean(p), nil
}

func testEnv(t *testing.T) (*Env, string) {
	t.Helper()
	root := t.TempDir()
	return &Env{Root: root, Paths: rootChecker{root}, Reads: NewReadTracker()}, root
}

func run(t *testing.T, tool Tool, env *Env, args any) Result {
	t.Helper()
	b, _ := json.Marshal(args)
	return tool.Run(context.Background(), env, b)
}

func TestReadWindowAndFooter(t *testing.T) {
	env, root := testEnv(t)
	var sb strings.Builder
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&sb, "line%d\n", i)
	}
	os.WriteFile(filepath.Join(root, "f.txt"), []byte(sb.String()), 0o644)
	r := run(t, readTool{}, env, map[string]any{"path": "f.txt", "offset": 3, "limit": 2})
	if r.IsError || r.Content != "3|line3\n4|line4\n[lines 3-4 of 10 — use offset=5 to continue]" {
		t.Fatalf("%q", r.Content)
	}
	r = run(t, readTool{}, env, map[string]any{"path": "f.txt", "offset": 9})
	if !strings.HasSuffix(r.Content, "[lines 9-10 of 10]") {
		t.Fatalf("%q", r.Content)
	}
}

func TestReadCaps(t *testing.T) {
	env, root := testEnv(t)
	os.WriteFile(filepath.Join(root, "long.txt"), []byte(strings.Repeat("x\n", 2500)), 0o644)
	r := run(t, readTool{}, env, map[string]any{"path": "long.txt"})
	if !strings.Contains(r.Content, "[lines 1-2000 of 2500") {
		t.Fatal("2000-line cap")
	}
	os.WriteFile(filepath.Join(root, "wide.txt"), []byte(strings.Repeat("y", 5000)+"\n"), 0o644)
	r = run(t, readTool{}, env, map[string]any{"path": "wide.txt"})
	if !strings.Contains(r.Content, "[… line truncated]") || len(r.Content) > 2200 {
		t.Fatal("per-line cap")
	}
	os.WriteFile(filepath.Join(root, "chars.txt"), []byte(strings.Repeat(strings.Repeat("z", 1000)+"\n", 100)), 0o644)
	r = run(t, readTool{}, env, map[string]any{"path": "chars.txt"})
	if len(r.Content) > 50_000+200 || !strings.Contains(r.Content, "use offset=") {
		t.Fatalf("50K char cap: %d", len(r.Content))
	}
}

func TestReadBinaryAndDirAndMissing(t *testing.T) {
	env, root := testEnv(t)
	os.WriteFile(filepath.Join(root, "b.png"), append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0), 0o644)
	r := run(t, readTool{}, env, map[string]any{"path": "b.png"})
	if !r.IsError || !strings.Contains(r.Content, "binary") || !strings.Contains(r.Content, "image/png") {
		t.Fatalf("%q", r.Content)
	}
	os.Mkdir(filepath.Join(root, "d"), 0o755)
	if r := run(t, readTool{}, env, map[string]any{"path": "d"}); !r.IsError || !strings.Contains(r.Content, "ls") {
		t.Fatalf("dir → suggest ls: %q", r.Content)
	}
	if r := run(t, readTool{}, env, map[string]any{"path": "nope"}); !r.IsError {
		t.Fatal("missing file")
	}
	if r := run(t, readTool{}, env, map[string]any{"path": "f", "offset": 0}); !r.IsError {
		t.Fatal("offset must be >= 1")
	}
}

func TestReadTracker(t *testing.T) {
	env, root := testEnv(t)
	p := filepath.Join(root, "t.txt")
	os.WriteFile(p, []byte("a\n"), 0o644)
	if err := env.Reads.Check(p); err == nil {
		t.Fatal("unread existing file must fail Check")
	}
	run(t, readTool{}, env, map[string]any{"path": "t.txt"})
	if err := env.Reads.Check(p); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte("b\n"), 0o644)
	if err := env.Reads.Check(p); err == nil || !strings.Contains(err.Error(), "re-read") {
		t.Fatalf("changed on disk: %v", err)
	}
	if err := env.Reads.Check(filepath.Join(root, "new.txt")); err != nil {
		t.Fatal("non-existent file needs no read")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/tools/ -run Read` → FAIL.

- [ ] **Step 3: Implement `readtracker.go`**

```go
// internal/tools/readtracker.go
package tools

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"
)

type stamp struct {
	mod  time.Time
	size int64
	sum  [32]byte
}

// ReadTracker remembers the on-disk state of every file read (or written) in
// this session. write/edit refuse existing files that were never read or
// that changed since (§4).
type ReadTracker struct {
	mu sync.Mutex
	m  map[string]stamp
}

func NewReadTracker() *ReadTracker { return &ReadTracker{m: map[string]stamp{}} }

func stampOf(abs string) (stamp, error) {
	fi, err := os.Stat(abs)
	if err != nil {
		return stamp{}, err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return stamp{}, err
	}
	return stamp{fi.ModTime(), fi.Size(), sha256.Sum256(b)}, nil
}

func (t *ReadTracker) Record(abs string) error {
	s, err := stampOf(abs)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.m[abs] = s
	t.mu.Unlock()
	return nil
}

func (t *ReadTracker) Check(abs string) error {
	cur, err := stampOf(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	t.mu.Lock()
	prev, ok := t.m[abs]
	t.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s exists and has not been read in this session; read it first", abs)
	}
	if prev.size != cur.size || prev.sum != cur.sum || !prev.mod.Equal(cur.mod) {
		return fmt.Errorf("%s changed on disk since you read it; re-read it before writing", abs)
	}
	return nil
}
```

- [ ] **Step 4: Implement `read.go` and register it**

```go
// internal/tools/read.go
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const (
	readMaxLines   = 2000
	readMaxChars   = 50_000
	readMaxLineLen = 2000
)

type readTool struct{}

func (readTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "read", Description: "Read a text file. Returns lines as `N|content` plus a " +
		"`[lines A-B of TOTAL]` footer. At most 2000 lines or 50K chars per call; use offset/limit to page " +
		"through large files instead of re-reading them whole. Lines longer than 2000 chars are truncated. " +
		"Binary files are refused. A file must be read before write/edit may change it.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File path, relative to the workdir or absolute"},` +
			`"offset":{"type":"integer","minimum":1,"description":"1-based first line (default 1)"},` +
			`"limit":{"type":"integer","minimum":1,"description":"Max lines to return (default/max 2000)"}},` +
			`"required":["path"],"additionalProperties":false}`)}
}

func (readTool) Run(_ context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path   string `json:"path"`
		Offset *int   `json:"offset"`
		Limit  *int   `json:"limit"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	offset, limit := 1, readMaxLines
	if a.Offset != nil {
		offset = *a.Offset
	}
	if a.Limit != nil {
		limit = min(*a.Limit, readMaxLines)
	}
	if offset < 1 || limit < 1 {
		return errorf("offset and limit must be >= 1")
	}
	abs, err := env.Paths.Resolve(a.Path, false)
	if err != nil {
		return errorf("%v", err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return errorf("%v", err)
	}
	if fi.IsDir() {
		return errorf("%s is a directory; use ls", a.Path)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return errorf("%v", err)
	}
	if bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
		return errorf("%s is a binary file (%d bytes, %s); not shown", a.Path, len(data), http.DetectContentType(data))
	}
	env.Reads.Record(abs)

	text := strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := strings.Split(text, "\n")
	if text == "" {
		lines = nil
	}
	total := len(lines)
	if total == 0 {
		return Result{Content: "[empty file]", Summary: a.Path}
	}
	if offset > total {
		return errorf("offset %d is past the end (%d lines)", offset, total)
	}
	var sb strings.Builder
	last := offset - 1
	for i := offset - 1; i < total && i < offset-1+limit; i++ {
		line := lines[i]
		if len(line) > readMaxLineLen {
			line = line[:readMaxLineLen] + "[… line truncated]"
		}
		entry := fmt.Sprintf("%d|%s\n", i+1, line)
		if sb.Len()+len(entry) > readMaxChars && i > offset-1 {
			break
		}
		sb.WriteString(entry)
		last = i + 1
	}
	footer := fmt.Sprintf("[lines %d-%d of %d]", offset, last, total)
	if last < total {
		footer = fmt.Sprintf("[lines %d-%d of %d — use offset=%d to continue]", offset, last, total, last+1)
	}
	sb.WriteString(footer)
	return Result{Content: sb.String(), Summary: fmt.Sprintf("%s %d-%d/%d", a.Path, offset, last, total)}
}
```

In `builtins.go`: `return []Tool{readTool{}}`.

- [ ] **Step 5: Run** — `go test ./internal/tools/ -run Read -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools
git commit -m "feat(tools): read with windowing, caps, binary refusal, read tracking"
```

---

### Task 4: `ls` tool

**Files:**
- Create: `internal/tools/ls.go`; modify `builtins.go`
- Test: `internal/tools/ls_test.go`

**Interfaces:**
- `ls` schema: `{"path": string (default "."), "hidden": boolean (default false)}`. Output: one entry per line, sorted, dirs suffixed `/`, symlinks suffixed `@`; capped at 1000 entries with `[… N more]`.

- [ ] **Step 1: Write failing test**

```go
// internal/tools/ls_test.go
package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLs(t *testing.T) {
	env, root := testEnv(t)
	os.Mkdir(filepath.Join(root, "dir"), 0o755)
	os.WriteFile(filepath.Join(root, "b.go"), nil, 0o644)
	os.WriteFile(filepath.Join(root, ".env"), nil, 0o644)
	os.Symlink("b.go", filepath.Join(root, "link"))
	r := run(t, lsTool{}, env, map[string]any{})
	if r.Content != "b.go\ndir/\nlink@" {
		t.Fatalf("%q", r.Content)
	}
	r = run(t, lsTool{}, env, map[string]any{"path": ".", "hidden": true})
	if r.Content != ".env\nb.go\ndir/\nlink@" {
		t.Fatalf("%q", r.Content)
	}
	if r := run(t, lsTool{}, env, map[string]any{"path": "b.go"}); !r.IsError {
		t.Fatal("ls on a file is an error")
	}
	os.Mkdir(filepath.Join(root, "empty"), 0o755)
	if r := run(t, lsTool{}, env, map[string]any{"path": "empty"}); r.Content != "[empty directory]" {
		t.Fatalf("%q", r.Content)
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/tools/ -run Ls` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/tools/ls.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type lsTool struct{}

func (lsTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "ls", Description: "List one directory level, sorted. Directories end with `/`, " +
		"symlinks with `@`. Hidden entries only with hidden=true. Use search to find files recursively.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"Directory (default: workdir)"},` +
			`"hidden":{"type":"boolean","description":"Include dotfiles (default false)"}},` +
			`"additionalProperties":false}`)}
}

func (lsTool) Run(_ context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path   string `json:"path"`
		Hidden bool   `json:"hidden"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	if a.Path == "" {
		a.Path = "."
	}
	abs, err := env.Paths.Resolve(a.Path, false)
	if err != nil {
		return errorf("%v", err)
	}
	ents, err := os.ReadDir(abs)
	if err != nil {
		return errorf("%v", err)
	}
	var out []string
	for _, e := range ents {
		name := e.Name()
		if !a.Hidden && strings.HasPrefix(name, ".") {
			continue
		}
		switch {
		case e.Type()&os.ModeSymlink != 0:
			name += "@"
		case e.IsDir():
			name += "/"
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return Result{Content: "[empty directory]", Summary: a.Path}
	}
	more := 0
	if len(out) > 1000 {
		more, out = len(out)-1000, out[:1000]
	}
	s := strings.Join(out, "\n")
	if more > 0 {
		s += fmt.Sprintf("\n[… %d more]", more)
	}
	return Result{Content: s, Summary: fmt.Sprintf("%s (%d)", a.Path, len(out)+more)}
}
```

`os.ReadDir` already returns entries sorted by name. Register `lsTool{}` after `readTool{}`. The final order is set in Task 10.

- [ ] **Step 4: Run** — `go test ./internal/tools/ -run Ls -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tools
git commit -m "feat(tools): ls"
```

---

### Task 5: gitignore matcher + `search` tool

**Files:**
- Create: `internal/tools/ignore.go`, `internal/tools/search.go`; modify `builtins.go`
- Test: `internal/tools/ignore_test.go`, `internal/tools/search_test.go`

**Interfaces:**
- Produces:
  - `type ignoreRule struct{ base string; re *regexp.Regexp; neg, dirOnly bool }`; `func parseIgnore(base string, data []byte) []ignoreRule`; `func ignored(rules []ignoreRule, rel string, isDir bool) bool` (`rel` is slash-separated relative to the walk root; last matching rule wins).
  - `search` schema: `{"pattern": string (required, RE2), "path": string (default "."), "glob": string (filename glob, e.g. "*.go"; matched against the base name, or against the relative path if it contains "/"), "files_only": boolean}`. Output: `path:line:text` per hit (text cut at 300 chars), or one path per line with `files_only`. Cap 200 hits → `[… results capped at 200; narrow the pattern or path]`.

- [ ] **Step 1: Write failing tests**

```go
// internal/tools/ignore_test.go
package tools

import "testing"

func TestIgnore(t *testing.T) {
	rules := parseIgnore("", []byte("# comment\n*.log\n/build\nnode_modules/\n!keep.log\ndocs/**/*.tmp\n\\#hash\n"))
	sub := parseIgnore("pkg", []byte("gen.go\n"))
	all := append(rules, sub...)
	cases := []struct {
		rel   string
		dir   bool
		want  bool
	}{
		{"a.log", false, true}, {"x/y/a.log", false, true}, {"keep.log", false, false},
		{"build", true, true}, {"x/build", true, false},
		{"node_modules", true, true}, {"a/node_modules", true, true}, {"node_modules", false, false},
		{"docs/a/b/c.tmp", false, true}, {"docs/c.tmp", false, true}, {"other/c.tmp", false, false},
		{"#hash", false, true},
		{"pkg/gen.go", false, true}, {"pkg/deep/gen.go", false, true}, {"gen.go", false, false},
	}
	for _, c := range cases {
		if got := ignored(all, c.rel, c.dir); got != c.want {
			t.Errorf("ignored(%q, dir=%v) = %v, want %v", c.rel, c.dir, got, c.want)
		}
	}
}
```

```go
// internal/tools/search_test.go
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(body), 0o644)
}

func TestSearch(t *testing.T) {
	env, root := testEnv(t)
	write(t, root, "a.go", "package a\nfunc Foo() {}\n")
	write(t, root, "sub/b.go", "// Foo here\nfunc Bar() {}\n")
	write(t, root, "sub/c.txt", "Foo text\n")
	write(t, root, ".hidden/d.go", "Foo\n")
	write(t, root, "vendor/e.go", "Foo\n")
	write(t, root, ".gitignore", "vendor/\n")
	write(t, root, "bin.dat", "Foo\x00\x01")

	r := run(t, searchTool{}, env, map[string]any{"pattern": `Foo`})
	want := "a.go:2:func Foo() {}\nsub/b.go:1:// Foo here\nsub/c.txt:1:Foo text"
	if r.Content != want {
		t.Fatalf("got\n%s\nwant\n%s", r.Content, want)
	}
	r = run(t, searchTool{}, env, map[string]any{"pattern": `Foo`, "glob": "*.go", "files_only": true})
	if r.Content != "a.go\nsub/b.go" {
		t.Fatalf("%q", r.Content)
	}
	r = run(t, searchTool{}, env, map[string]any{"pattern": `Foo`, "path": "sub", "glob": "sub/*.txt"})
	if r.Content != "sub/c.txt:1:Foo text" {
		t.Fatalf("%q", r.Content)
	}
	if r := run(t, searchTool{}, env, map[string]any{"pattern": `(?<=x)y`}); !r.IsError || !strings.Contains(r.Content, "RE2") {
		t.Fatalf("lookbehind must explain RE2: %q", r.Content)
	}
	if r := run(t, searchTool{}, env, map[string]any{"pattern": `nomatch_zzz`}); r.Content != "[no matches]" {
		t.Fatalf("%q", r.Content)
	}
}

func TestSearchCap(t *testing.T) {
	env, root := testEnv(t)
	var sb strings.Builder
	for i := range 300 {
		fmt.Fprintf(&sb, "hit %d\n", i)
	}
	write(t, root, "many.txt", sb.String())
	r := run(t, searchTool{}, env, map[string]any{"pattern": `hit`})
	if strings.Count(r.Content, "\n") != 200 || !strings.Contains(r.Content, "capped at 200") {
		t.Fatalf("cap: %d lines", strings.Count(r.Content, "\n"))
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/tools/ -run 'Ignore|Search'` → FAIL.

- [ ] **Step 3: Implement `ignore.go`**

```go
// internal/tools/ignore.go
package tools

import (
	"path"
	"regexp"
	"strings"
)

type ignoreRule struct {
	base    string // slash path of the dir holding the ignore file, "" = root
	re      *regexp.Regexp
	neg     bool
	dirOnly bool
}

// parseIgnore implements the gitignore subset rg honours: comments, blank
// lines, ! negation, trailing / (dirs only), leading or inner / (anchored to
// base), * ? [..] and ** globs, and \ escapes for a leading # or !.
func parseIgnore(base string, data []byte) []ignoreRule {
	var rules []ignoreRule
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := ignoreRule{base: base}
		if strings.HasPrefix(line, "!") {
			r.neg, line = true, line[1:]
		} else if strings.HasPrefix(line, `\`) {
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly, line = true, strings.TrimSuffix(line, "/")
		}
		anchored := strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		expr := globToRegexp(line)
		if !anchored {
			expr = `(?:.*/)?` + expr
		}
		re, err := regexp.Compile("^" + expr + "$")
		if err != nil {
			continue
		}
		r.re = re
		rules = append(rules, r)
	}
	return rules
}

func globToRegexp(g string) string {
	var sb strings.Builder
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch {
		case strings.HasPrefix(g[i:], "**/"):
			sb.WriteString(`(?:.*/)?`)
			i += 2
		case strings.HasPrefix(g[i:], "/**") && i+3 == len(g):
			sb.WriteString(`/.*`)
			i += 2
		case strings.HasPrefix(g[i:], "**"):
			sb.WriteString(`.*`)
			i++
		case c == '*':
			sb.WriteString(`[^/]*`)
		case c == '?':
			sb.WriteString(`[^/]`)
		case c == '[':
			end := strings.IndexByte(g[i:], ']')
			if end < 0 {
				sb.WriteString(`\[`)
				continue
			}
			cls := g[i+1 : i+end]
			if strings.HasPrefix(cls, "!") {
				cls = "^" + cls[1:]
			}
			sb.WriteString("[" + cls + "]")
			i += end
		case c == '\\' && i+1 < len(g):
			i++
			sb.WriteString(regexp.QuoteMeta(string(g[i])))
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return sb.String()
}

func ignored(rules []ignoreRule, rel string, isDir bool) bool {
	out := false
	for _, r := range rules {
		sub := rel
		if r.base != "" {
			if !strings.HasPrefix(rel, r.base+"/") {
				continue
			}
			sub = rel[len(r.base)+1:]
		}
		if r.dirOnly && !isDir {
			continue
		}
		if r.re.MatchString(sub) {
			out = !r.neg
		}
	}
	return out
}

func joinRel(dir, name string) string {
	if dir == "" || dir == "." {
		return name
	}
	return path.Join(dir, name)
}
```

**Parent-directory semantics:** a file under an ignored directory is never reached, because the walker skips ignored directories with `fs.SkipDir`. So `ignored` only needs to evaluate the entry itself.

- [ ] **Step 4: Implement `search.go`**

```go
// internal/tools/search.go
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const searchCap = 200

type searchTool struct{}

func (searchTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "search", Description: "Search file contents recursively with a regular expression " +
		"(Go RE2 syntax: no lookahead/lookbehind or backreferences; use (?i) for case-insensitive). " +
		"Respects .gitignore/.ignore and skips hidden and binary files. Returns `path:line:text`, " +
		"or only paths with files_only. At most 200 hits.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"pattern":{"type":"string","description":"RE2 regular expression"},` +
			`"path":{"type":"string","description":"Directory or file to search (default: workdir)"},` +
			`"glob":{"type":"string","description":"Filename filter, e.g. *.go or src/*.ts"},` +
			`"files_only":{"type":"boolean","description":"List matching files only"}},` +
			`"required":["pattern"],"additionalProperties":false}`)}
}

func (searchTool) Run(ctx context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Pattern   string `json:"pattern"`
		Path      string `json:"path"`
		Glob      string `json:"glob"`
		FilesOnly bool   `json:"files_only"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		return errorf("invalid pattern (Go RE2 syntax — no lookaround or backreferences): %v", err)
	}
	if a.Path == "" {
		a.Path = "."
	}
	start, err := env.Paths.Resolve(a.Path, false)
	if err != nil {
		return errorf("%v", err)
	}
	var hits []string
	capped := false
	var rules []ignoreRule
	loadIgnores := func(dirAbs, rel string) {
		for _, n := range []string{".gitignore", ".ignore"} {
			if b, err := os.ReadFile(filepath.Join(dirAbs, n)); err == nil {
				rules = append(rules, parseIgnore(rel, b)...)
			}
		}
	}
	// display paths relative to the workdir when inside it
	display := func(abs string) string {
		if r, err := filepath.Rel(env.Root, abs); err == nil && !strings.HasPrefix(r, "..") {
			return filepath.ToSlash(r)
		}
		return abs
	}
	err = filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return ctx.Err()
		}
		rel, _ := filepath.Rel(start, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			if d.IsDir() {
				loadIgnores(p, "")
			}
		} else {
			if strings.HasPrefix(d.Name(), ".") || ignored(rules, rel, d.IsDir()) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				loadIgnores(p, rel)
				return nil
			}
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		disp := display(p)
		if a.Glob != "" {
			target := path.Base(disp)
			if strings.Contains(a.Glob, "/") {
				target = disp
			}
			if ok, _ := path.Match(a.Glob, target); !ok {
				return nil
			}
		}
		data, err := os.ReadFile(p)
		if err != nil || bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
			return nil
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 64<<10), 4<<20)
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			if !re.MatchString(line) {
				continue
			}
			if len(hits) == searchCap {
				capped = true
				return fs.SkipAll
			}
			if a.FilesOnly {
				hits = append(hits, disp)
				break
			}
			if len(line) > 300 {
				line = line[:300] + "…"
			}
			hits = append(hits, fmt.Sprintf("%s:%d:%s", disp, n, line))
		}
		return nil
	})
	if err != nil {
		return errorf("%v", err)
	}
	if len(hits) == 0 {
		return Result{Content: "[no matches]", Summary: a.Pattern + " (0)"}
	}
	out := strings.Join(hits, "\n")
	if capped {
		out += "\n[… results capped at 200; narrow the pattern or path]"
	}
	return Result{Content: out, Summary: fmt.Sprintf("%s (%d)", a.Pattern, len(hits))}
}
```

Register `searchTool{}`.

- [ ] **Step 5: Run** — `go test ./internal/tools/ -run 'Ignore|Search' -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools
git commit -m "feat(tools): pure-Go search respecting .gitignore/.ignore, RE2, 200-hit cap"
```

---

### Task 6: Edit algorithm (edit ladder)

**Files:**
- Create: `internal/tools/editapply.go`
- Test: `internal/tools/editapply_test.go`

**Interfaces:**
- Produces:
  - `type hunk struct { Line int; Old, New []string }` (Line = 1-based start in the original file)
  - `func applyEdit(content []byte, oldS, newS string, replaceAll bool) (out []byte, hunks []hunk, err error)`
  - `func formatDiff(path string, origLines, newLines []string, hunks []hunk) string` — `@@ -a,b +c,d @@` hunks with 3 lines of context, `-`/`+`/` ` prefixes.
  - `func splitLines(s string) []string`
  - Errors are written for the model (§5.6). Exact messages are pinned by tests.

- [ ] **Step 1: Write the edit ladder (failing)**

```go
// internal/tools/editapply_test.go
package tools

import (
	"strings"
	"testing"
)

func mustApply(t *testing.T, in, old, new string, all bool) string {
	t.Helper()
	out, _, err := applyEdit([]byte(in), old, new, all)
	if err != nil {
		t.Fatalf("applyEdit: %v", err)
	}
	return string(out)
}

func applyErr(t *testing.T, in, old, new string, all bool) string {
	t.Helper()
	_, _, err := applyEdit([]byte(in), old, new, all)
	if err == nil {
		t.Fatal("want error")
	}
	return err.Error()
}

func TestEditLadder(t *testing.T) {
	t.Run("exact unique", func(t *testing.T) {
		if got := mustApply(t, "a\nb\nc\n", "b\n", "B\n", false); got != "a\nB\nc\n" {
			t.Fatal(got)
		}
	})
	t.Run("exact ambiguous lists lines", func(t *testing.T) {
		e := applyErr(t, "x\ny\nx\n", "x", "z", false)
		if !strings.Contains(e, "2 matches") || !strings.Contains(e, "lines 1, 3") || !strings.Contains(e, "replace_all") {
			t.Fatal(e)
		}
	})
	t.Run("replace_all n hits", func(t *testing.T) {
		out, hunks, err := applyEdit([]byte("x\ny\nx\nx\n"), "x", "z", true)
		if err != nil || string(out) != "z\ny\nz\nz\n" || len(hunks) != 3 {
			t.Fatal(string(out), len(hunks), err)
		}
	})
	t.Run("replace_all never uses fallback", func(t *testing.T) {
		applyErr(t, "\tx := 1\n", "    x := 1", "y", true)
	})
	t.Run("fallback deeper indent", func(t *testing.T) {
		in := "func f() {\n\t\tif a {\n\t\t\tb()\n\t\t}\n}\n"
		got := mustApply(t, in, "if a {\n\tb()\n}", "if a {\n\tc()\n\td()\n}", false)
		if got != "func f() {\n\t\tif a {\n\t\t\tc()\n\t\t\td()\n\t\t}\n}\n" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("fallback shallower indent", func(t *testing.T) {
		in := "class A:\n    def f(self):\n        return 1\n"
		got := mustApply(t, in, "        def f(self):\n            return 1", "        def f(self):\n            return 2", false)
		if got != "class A:\n    def f(self):\n        return 2\n" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("fallback ambiguous", func(t *testing.T) {
		e := applyErr(t, "  a\n  b\n    a\n    b\n", "a\nb", "c", false)
		if !strings.Contains(e, "whitespace-insensitive") {
			t.Fatal(e)
		}
	})
	t.Run("no match", func(t *testing.T) {
		e := applyErr(t, "a\n", "zzz", "y", false)
		if !strings.Contains(e, "not found") || !strings.Contains(e, "re-read") {
			t.Fatal(e)
		}
	})
	t.Run("no match with line-number prefixes", func(t *testing.T) {
		e := applyErr(t, "a\nb\n", "1|a\n2|b", "c", false)
		if !strings.Contains(e, "N|") {
			t.Fatalf("must explain read prefixes: %s", e)
		}
	})
	t.Run("no-op", func(t *testing.T) {
		if e := applyErr(t, "a\n", "a", "a", false); !strings.Contains(e, "identical") {
			t.Fatal(e)
		}
	})
	t.Run("empty old_string", func(t *testing.T) {
		if e := applyErr(t, "a\n", "", "b", false); !strings.Contains(e, "write") {
			t.Fatal(e)
		}
	})
	t.Run("CRLF stays CRLF", func(t *testing.T) {
		got := mustApply(t, "a\r\nb\r\nc\r\n", "b\nc", "B\nC", false)
		if got != "a\r\nB\r\nC\r\n" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("BOM preserved and unmatchable", func(t *testing.T) {
		got := mustApply(t, "\ufeffa\nb\n", "a", "A", false)
		if got != "\ufeffA\nb\n" {
			t.Fatalf("%q", got)
		}
		applyErr(t, "\ufeffa\n", "\ufeffa", "b", false)
	})
	t.Run("unicode", func(t *testing.T) {
		if got := mustApply(t, "naïve → 日本\n", "→ 日本", "← 中文", false); got != "naïve ← 中文\n" {
			t.Fatal(got)
		}
	})
	t.Run("50-line span", func(t *testing.T) {
		var in, old, nw strings.Builder
		in.WriteString("head\n")
		for i := range 50 {
			line := strings.Repeat("x", i%7) + "\n"
			in.WriteString(line)
			old.WriteString(line)
			nw.WriteString("y" + line)
		}
		in.WriteString("tail\n")
		got := mustApply(t, in.String(), old.String(), nw.String(), false)
		if !strings.HasPrefix(got, "head\ny") || !strings.HasSuffix(got, "tail\n") {
			t.Fatal(got[:20])
		}
	})
	t.Run("missing trailing newline preserved", func(t *testing.T) {
		if got := mustApply(t, "a\nb", "b", "c", false); got != "a\nc" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("non-UTF-8 refused", func(t *testing.T) {
		if e := applyErr(t, "a\xff\n", "a", "b", false); !strings.Contains(e, "UTF-8") {
			t.Fatal(e)
		}
	})
}

func TestFormatDiff(t *testing.T) {
	orig := "1\n2\n3\n4\n5\n6\n7\n8\n"
	out, hunks, _ := applyEdit([]byte(orig), "5\n", "five\n", false)
	d := formatDiff("f", splitLines(orig), splitLines(string(out)), hunks)
	want := "--- f\n+++ f\n@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+five\n 6\n 7\n 8\n"
	if d != want {
		t.Fatalf("got\n%s\nwant\n%s", d, want)
	}
}
```

The edit-ladder items "write to new file (no read needed)" and "write/edit after an on-disk change (refused)" are tool-level. They are covered in Task 7.

- [ ] **Step 2: Run** — `go test ./internal/tools/ -run 'EditLadder|FormatDiff'` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/tools/editapply.go
package tools

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type hunk struct {
	Line     int // 1-based start line in the original
	Old, New []string
}

var bom = []byte{0xEF, 0xBB, 0xBF}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

var lineNumPrefix = regexp.MustCompile(`(?m)^\s*\d+\|`)

// applyEdit implements DESIGN.md §5 on an LF-normalized, BOM-stripped view
// and restores the file's dominant line ending and BOM on output.
func applyEdit(content []byte, oldS, newS string, replaceAll bool) ([]byte, []hunk, error) {
	if oldS == "" {
		return nil, nil, errors.New("old_string is empty; to create or overwrite a whole file use write")
	}
	if oldS == newS {
		return nil, nil, errors.New("old_string and new_string are identical; nothing to change")
	}
	hasBOM := bytes.HasPrefix(content, bom)
	content = bytes.TrimPrefix(content, bom)
	if !utf8.Valid(content) {
		return nil, nil, errors.New("file is not valid UTF-8; edit refused")
	}
	crlf := bytes.Count(content, []byte("\r\n"))
	lf := bytes.Count(content, []byte("\n")) - crlf
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	oldS = strings.ReplaceAll(oldS, "\r\n", "\n")
	newS = strings.ReplaceAll(newS, "\r\n", "\n")

	var out string
	var hunks []hunk
	n := strings.Count(text, oldS)
	switch {
	case n == 1 || (n > 1 && replaceAll):
		out, hunks = replaceExact(text, oldS, newS)
	case n > 1:
		return nil, nil, fmt.Errorf("old_string has %d matches (lines %s); add surrounding context to make it unique, or set replace_all", n, joinInts(matchLines(text, oldS)))
	case replaceAll:
		return nil, nil, errors.New("old_string not found (replace_all only matches exactly); re-read the file and copy the text verbatim")
	default:
		var err error
		out, hunks, err = replaceFuzzy(text, oldS, newS)
		if err != nil {
			return nil, nil, err
		}
	}
	if crlf > lf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	res := []byte(out)
	if hasBOM {
		res = append(append([]byte{}, bom...), res...)
	}
	return res, hunks, nil
}

func lineOf(text string, byteOff int) int { return strings.Count(text[:byteOff], "\n") + 1 }

func matchLines(text, s string) []int {
	var ls []int
	for off := 0; ; {
		i := strings.Index(text[off:], s)
		if i < 0 {
			return ls
		}
		ls = append(ls, lineOf(text, off+i))
		off += i + len(s)
	}
}

func joinInts(xs []int) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = fmt.Sprint(x)
	}
	return strings.Join(s, ", ")
}

// replaceExact replaces every exact occurrence; each hunk spans the whole
// lines touched by one occurrence.
func replaceExact(text, oldS, newS string) (string, []hunk) {
	var sb strings.Builder
	var hunks []hunk
	off := 0
	for {
		i := strings.Index(text[off:], oldS)
		if i < 0 {
			break
		}
		start, end := off+i, off+i+len(oldS)
		ls := strings.LastIndex(text[:start], "\n") + 1
		le := len(text)
		if j := strings.IndexByte(text[end:], '\n'); j >= 0 {
			le = end + j
		}
		hunks = append(hunks, hunk{
			Line: lineOf(text, ls),
			Old:  strings.Split(text[ls:le], "\n"),
			New:  strings.Split(text[ls:start]+newS+text[end:le], "\n"),
		})
		sb.WriteString(text[off:start])
		sb.WriteString(newS)
		off = end
	}
	sb.WriteString(text[off:])
	return sb.String(), hunks
}
```

Note one edge: when `oldS` ends with `"\n"`, `text[end:le]` is the whole following line, so hunk `Old`/`New` include one extra unchanged line. `formatDiff` trims common leading/trailing lines per hunk, so the diff stays minimal.

Continue `editapply.go`:

```go
func indentOf(s string) string { return s[:len(s)-len(strings.TrimLeft(s, " \t"))] }

func replaceFuzzy(text, oldS, newS string) (string, []hunk, error) {
	fileLines := strings.Split(text, "\n")
	oldLines := splitLines(oldS)
	norm := func(s string) string { return strings.TrimSpace(s) }
	var starts []int
	for i := 0; i+len(oldLines) <= len(fileLines); i++ {
		ok := true
		for j, ol := range oldLines {
			if norm(fileLines[i+j]) != norm(ol) {
				ok = false
				break
			}
		}
		if ok {
			starts = append(starts, i)
		}
	}
	if len(starts) == 0 {
		msg := "old_string not found (exact or whitespace-insensitive). Re-read the relevant lines and retry with the exact text"
		if lineNumPrefix.MatchString(oldS) {
			msg += "; old_string looks like it includes read's `N|` line-number prefixes — drop them"
		}
		return "", nil, errors.New(msg)
	}
	if len(starts) > 1 {
		ls := make([]int, len(starts))
		for k, s := range starts {
			ls[k] = s + 1
		}
		return "", nil, fmt.Errorf("old_string has no exact match and %d whitespace-insensitive matches (lines %s); add more context", len(starts), joinInts(ls))
	}
	at := starts[0]
	// indentation delta between old_string's first non-blank line and the file line it matched
	var oldInd, fileInd string
	for j, ol := range oldLines {
		if strings.TrimSpace(ol) != "" {
			oldInd, fileInd = indentOf(ol), indentOf(fileLines[at+j])
			break
		}
	}
	var repl []string
	for _, nl := range splitLines(newS) {
		if strings.TrimSpace(nl) == "" {
			repl = append(repl, "")
			continue
		}
		ind := indentOf(nl)
		switch {
		case strings.HasPrefix(ind, oldInd):
			ind = fileInd + ind[len(oldInd):]
		case strings.HasPrefix(oldInd, ind):
			removed := oldInd[len(ind):]
			if strings.HasSuffix(fileInd, removed) {
				ind = fileInd[:len(fileInd)-len(removed)]
			} else {
				ind = fileInd[:max(0, len(fileInd)-len(removed))]
			}
		}
		repl = append(repl, ind+strings.TrimLeft(nl, " \t"))
	}
	orig := fileLines[at : at+len(oldLines)]
	out := append(append(append([]string{}, fileLines[:at]...), repl...), fileLines[at+len(oldLines):]...)
	return strings.Join(out, "\n"), []hunk{{Line: at + 1, Old: orig, New: repl}}, nil
}

// formatDiff renders unified-diff hunks with 3 lines of context, computed
// from the original and new full line slices. Common leading/trailing lines
// inside a hunk become context.
func formatDiff(path string, orig, upd []string, hunks []hunk) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", path, path)
	shift := 0
	for _, h := range hunks {
		o, n := h.Old, h.New
		pre := 0
		for pre < len(o) && pre < len(n) && o[pre] == n[pre] {
			pre++
		}
		suf := 0
		for suf < len(o)-pre && suf < len(n)-pre && o[len(o)-1-suf] == n[len(n)-1-suf] {
			suf++
		}
		oStart := h.Line - 1 + pre // 0-based first changed orig line
		oDel := o[pre : len(o)-suf]
		nAdd := n[pre : len(n)-suf]
		ctxA := max(0, oStart-3)
		ctxB := min(len(orig), oStart+len(oDel)+3)
		nStart := ctxA + shift
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", ctxA+1, ctxB-ctxA, nStart+1, ctxB-ctxA-len(oDel)+len(nAdd))
		for _, l := range orig[ctxA:oStart] {
			sb.WriteString(" " + l + "\n")
		}
		for _, l := range oDel {
			sb.WriteString("-" + l + "\n")
		}
		for _, l := range nAdd {
			sb.WriteString("+" + l + "\n")
		}
		for _, l := range orig[oStart+len(oDel) : ctxB] {
			sb.WriteString(" " + l + "\n")
		}
		shift += len(nAdd) - len(oDel)
	}
	return sb.String()
}
```

`formatDiff` doesn't use `upd` for rendering; it's kept in the signature so the caller can compute the new total. Drop the parameter if `go vet`/staticcheck complains, and update the test accordingly.

- [ ] **Step 4: Run** — `go test ./internal/tools/ -run 'EditLadder|FormatDiff' -v` → PASS. If "fallback shallower indent" fails, check the `HasPrefix(oldInd, ind)` branch: old indent is 8 spaces, file indent is 4, and the second new line has 12 spaces. That line takes the first branch: `"    " + "    "` gives 8. Correct.

- [ ] **Step 5: Commit**

```bash
git add internal/tools/editapply*.go
git commit -m "feat(tools): edit algorithm — exact/fallback/replace_all, re-indent, CRLF/BOM, model-facing errors"
```

---

### Task 7: `write` + `edit` tools with read guard and snapshots

**Files:**
- Create: `internal/tools/write.go`, `internal/tools/edit.go`; modify `builtins.go`
- Test: `internal/tools/writeedit_test.go`

**Interfaces:**
- Consumes: `applyEdit`, `formatDiff`, `ReadTracker`, `Snapshotter`, `PathChecker`.
- Produces:
  - `write` schema `{"path": string, "content": string}` (both required). Creates parent dirs. Result: `created <path> (N lines)` / `wrote <path> (N lines, was M)`. Summary `path [+N −M]`.
  - `edit` schema `{"path", "old_string", "new_string": string (required), "replace_all": boolean}`. Result: diff + `changed lines A-B; file now N lines` (multiple hunks: `changed lines A-B, C-D`). Summary `path [+a −d]`.
  - Both call `env.Reads.Check(abs)` first, then perform the write inside `env.Snap.Guard(abs, fn)` when `env.Snap != nil`, then `env.Reads.Record(abs)` (so consecutive edits don't need a re-read).
  - Writes preserve the existing file mode; new files are 0644.

- [ ] **Step 1: Write failing tests**

```go
// internal/tools/writeedit_test.go
package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSnap struct{ paths []string }

func (f *fakeSnap) Guard(p string, w func() error) error { f.paths = append(f.paths, p); return w() }

func TestWriteNewFileNoReadNeeded(t *testing.T) {
	env, root := testEnv(t)
	snap := &fakeSnap{}
	env.Snap = snap
	r := run(t, writeTool{}, env, map[string]any{"path": "pkg/new/x.go", "content": "package x\n"})
	if r.IsError || !strings.Contains(r.Content, "created") {
		t.Fatal(r.Content)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "pkg/new/x.go")); string(b) != "package x\n" {
		t.Fatal("content")
	}
	if len(snap.paths) != 1 {
		t.Fatal("new-file write must go through the snapshotter (records 'did not exist')")
	}
}

func TestWriteExistingNeedsReadAndFreshness(t *testing.T) {
	env, root := testEnv(t)
	p := filepath.Join(root, "f.txt")
	os.WriteFile(p, []byte("old\n"), 0o600)
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "x"}); !r.IsError || !strings.Contains(r.Content, "read it first") {
		t.Fatal(r.Content)
	}
	run(t, readTool{}, env, map[string]any{"path": "f.txt"})
	os.WriteFile(p, []byte("changed\n"), 0o600)
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "x"}); !r.IsError || !strings.Contains(r.Content, "re-read") {
		t.Fatal(r.Content)
	}
	run(t, readTool{}, env, map[string]any{"path": "f.txt"})
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "new\n"}); r.IsError {
		t.Fatal(r.Content)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatal("mode must be preserved")
	}
	// a second write right after our own write needs no re-read
	if r := run(t, writeTool{}, env, map[string]any{"path": "f.txt", "content": "again\n"}); r.IsError {
		t.Fatal(r.Content)
	}
}

func TestWriteOutsideJailRefusedBeforeMkdir(t *testing.T) {
	env, root := testEnv(t)
	r := run(t, writeTool{}, env, map[string]any{"path": "/tmp/../etc/moca-x/y", "content": "x"})
	if !r.IsError {
		t.Fatal("outside jail")
	}
	if _, err := os.Stat(filepath.Join(root, "etc")); err == nil {
		t.Fatal("no dirs may be created")
	}
}

func TestEditTool(t *testing.T) {
	env, root := testEnv(t)
	p := filepath.Join(root, "m.go")
	os.WriteFile(p, []byte("package m\n\nfunc A() int {\n\treturn 1\n}\n"), 0o644)
	if r := run(t, editTool{}, env, map[string]any{"path": "m.go", "old_string": "return 1", "new_string": "return 2"}); !r.IsError {
		t.Fatal("edit needs prior read")
	}
	run(t, readTool{}, env, map[string]any{"path": "m.go"})
	r := run(t, editTool{}, env, map[string]any{"path": "m.go", "old_string": "return 1", "new_string": "return 2"})
	if r.IsError || !strings.Contains(r.Content, "-\treturn 1") || !strings.Contains(r.Content, "+\treturn 2") ||
		!strings.Contains(r.Content, "changed lines 4-4; file now 5 lines") || r.Summary != "m.go [+1 −1]" {
		t.Fatalf("%q / %q", r.Content, r.Summary)
	}
	r = run(t, editTool{}, env, map[string]any{"path": "m.go", "old_string": "zzz", "new_string": "y"})
	if !r.IsError {
		t.Fatal("structured error")
	}
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), "return 2") {
		t.Fatal("failed edit must not write")
	}
	if r := run(t, editTool{}, env, map[string]any{"path": "nope.go", "old_string": "a", "new_string": "b"}); !r.IsError {
		t.Fatal("edit on missing file")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/tools/ -run 'Write|EditTool'` → FAIL.

- [ ] **Step 3: Implement `write.go`**

```go
// internal/tools/write.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type writeTool struct{}

func (writeTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "write", Description: "Create a file or replace its entire content. New files need " +
		"no prior read; an existing file must have been read in this session and be unchanged on disk since. " +
		"Parent directories are created. Prefer edit for changes to existing files.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File path, relative to the workdir or absolute"},` +
			`"content":{"type":"string","description":"Full file content"}},` +
			`"required":["path","content"],"additionalProperties":false}`)}
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(s, "\n"), "\n") + 1
}

// guardedWrite applies the read guard, snapshot and mode preservation shared
// by write and edit.
func guardedWrite(env *Env, abs string, data []byte) error {
	if err := env.Reads.Check(abs); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(abs); err == nil {
		mode = fi.Mode().Perm()
	}
	do := func() error {
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		return os.WriteFile(abs, data, mode)
	}
	var err error
	if env.Snap != nil {
		err = env.Snap.Guard(abs, do)
	} else {
		err = do()
	}
	if err != nil {
		return err
	}
	return env.Reads.Record(abs)
}

func (writeTool) Run(_ context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	abs, err := env.Paths.Resolve(a.Path, true)
	if err != nil {
		return errorf("%v", err)
	}
	if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
		return errorf("%s is a directory", a.Path)
	}
	prev, readErr := os.ReadFile(abs)
	if err := guardedWrite(env, abs, []byte(a.Content)); err != nil {
		return errorf("%v", err)
	}
	n := countLines(a.Content)
	if readErr != nil {
		return Result{Content: fmt.Sprintf("created %s (%d lines)", a.Path, n), Summary: fmt.Sprintf("%s [+%d]", a.Path, n)}
	}
	m := countLines(string(prev))
	return Result{Content: fmt.Sprintf("wrote %s (%d lines, was %d)", a.Path, n, m), Summary: fmt.Sprintf("%s [+%d −%d]", a.Path, n, m)}
}
```

- [ ] **Step 4: Implement `edit.go`**

```go
// internal/tools/edit.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type editTool struct{}

func (editTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "edit", Description: "Replace old_string with new_string in a file that was read in " +
		"this session. old_string must match exactly once (include surrounding lines to disambiguate) unless " +
		"replace_all is set. If no exact match exists, one whitespace-insensitive match is accepted and " +
		"new_string is re-indented to fit. Copy text without read's `N|` prefixes. Returns a unified diff.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File path"},` +
			`"old_string":{"type":"string","description":"Exact text to replace (non-empty)"},` +
			`"new_string":{"type":"string","description":"Replacement text"},` +
			`"replace_all":{"type":"boolean","description":"Replace every exact occurrence (default false)"}},` +
			`"required":["path","old_string","new_string"],"additionalProperties":false}`)}
}

func (editTool) Run(_ context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	abs, err := env.Paths.Resolve(a.Path, true)
	if err != nil {
		return errorf("%v", err)
	}
	orig, err := os.ReadFile(abs)
	if err != nil {
		return errorf("%v (edit changes existing files; use write to create one)", err)
	}
	if err := env.Reads.Check(abs); err != nil {
		return errorf("%v", err)
	}
	out, hunks, err := applyEdit(orig, a.OldString, a.NewString, a.ReplaceAll)
	if err != nil {
		return errorf("%s: %v", a.Path, err)
	}
	if err := guardedWrite(env, abs, out); err != nil {
		return errorf("%v", err)
	}
	norm := func(b []byte) []string { return splitLines(strings.ReplaceAll(strings.TrimPrefix(string(b), "\ufeff"), "\r\n", "\n")) }
	oL, nL := norm(orig), norm(out)
	diff := formatDiff(a.Path, oL, nL, hunks)
	adds, dels := 0, 0
	for _, l := range strings.Split(diff, "\n")[2:] { // skip the ---/+++ header
		switch {
		case strings.HasPrefix(l, "+"):
			adds++
		case strings.HasPrefix(l, "-"):
			dels++
		}
	}
	var ranges []string
	shift := 0
	for _, h := range hunks {
		start := h.Line + shift
		ranges = append(ranges, fmt.Sprintf("%d-%d", start, start+max(len(h.New), 1)-1))
		shift += len(h.New) - len(h.Old)
	}
	content := fmt.Sprintf("%schanged lines %s; file now %d lines", diff, strings.Join(ranges, ", "), len(nL))
	return Result{Content: content, Summary: fmt.Sprintf("%s [+%d −%d]", a.Path, adds, dels), Detail: diff}
}
```

Register `writeTool{}` and `editTool{}`.

- [ ] **Step 5: Run** — `go test ./internal/tools/ -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools
git commit -m "feat(tools): write and edit with read-before-write guard, staleness check, snapshot hook"
```

---

### Task 8: Shell command analysis (shell-analysis ladder)

**Files:**
- Create: `internal/permissions/shell.go`, `internal/permissions/shell_windows_rules.go`
- Test: `internal/permissions/shell_test.go`
- Modify: `go.mod`/`go.sum` (`go get mvdan.cc/sh/v3@latest`)

**Interfaces:**
- Consumes: `*Jail` (Task 2) for redirect targets.
- Produces:
  - `func NewShell(allow []string, jail *Jail, goos string) *Shell`
  - `func (s *Shell) Check(command string) (needApproval, askEveryTime []string, err error)` and `func (s *Shell) Allow(name string)` — together they satisfy `tools.CommandChecker`.
  - `var hardDeny = map[string]bool{"sudo","su","doas","dd","shred","chown"}` + prefix `mkfs`.
  - `var wrappers` = `env time timeout nice nohup command`. `rtk` is added in phase 6.
  - Windows (`goos == "windows"`): `checkWindows`: split on `;`, `|`, `&&`, `||`; first token per segment (lowercased, `.exe` stripped). Hard-deny/ask extended with `Remove-Item rm ri del erase rd rmdir Format-Volume`. These go into askEveryTime, except `Format-Volume`, which is hard-denied.

- [ ] **Step 1: Add the dependency**

Run: `go get mvdan.cc/sh/v3@latest && go mod tidy`
Expected: `go.mod` gains `require mvdan.cc/sh/v3 vX.Y.Z`.

- [ ] **Step 2: Write the ladder (failing)**

```go
// internal/permissions/shell_test.go
package permissions

import (
	"slices"
	"strings"
	"testing"
)

func newTestShell(t *testing.T) *Shell {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	return NewShell([]string{"go", "git", "make", "grep", "cat", "tee", "ls"}, j, "linux")
}

type verdict struct {
	need, every []string
	deny        string // substring of error, "" = no error
}

func TestShellAnalysisLadder(t *testing.T) {
	s := newTestShell(t)
	cases := map[string]verdict{
		"go test ./...":                  {},
		"python x.py":                    {need: []string{"python"}},
		"git log | python -c 1":          {need: []string{"python"}},
		"go build && python x":           {need: []string{"python"}},
		"go vet; python x":               {need: []string{"python"}},
		"echo $(rm x)":                   {every: []string{"rm"}},
		"echo `rm x`":                    {every: []string{"rm"}},
		"( sudo ls )":                    {deny: "sudo"},
		"FOO=1 go test":                  {},
		"timeout 5 rm x":                 {every: []string{"rm"}},
		"env A=1 B=2 nice -n 5 python x": {need: []string{"python"}},
		"cat a > ../outside/x":           {deny: "outside"},
		"cat a > /dev/null 2>&1":         {},
		"cat a >> out.txt":               {},
		"cat a | tee ../outside/log":     {deny: "outside"},
		"eval ls":                        {deny: "eval"},
		"$CMD args":                      {deny: "non-literal"},
		"cat a > $OUT":                   {deny: "non-literal"},
		"if then fi (":                   {deny: "parse"},
		"cd sub && make":                 {},
		"mkfs.ext4 /dev/sda":             {deny: "mkfs"},
		"source env.sh":                  {deny: "source"},
		". env.sh":                       {deny: "."},
		"exec go":                        {deny: "exec"},
		"cat <(rm x)":                    {every: []string{"rm"}},
		"{ go test; python y; }":         {need: []string{"python"}},
		"go test & python bg":            {need: []string{"python"}},
		"go test\npython y":              {need: []string{"python"}},
		`"go" test`:                      {},
		"command -v python":              {},
		"rm -rf build":                   {every: []string{"rm"}},
		"/usr/bin/sudo ls":               {deny: "sudo"},
	}
	for cmd, want := range cases {
		need, every, err := s.Check(cmd)
		if want.deny != "" {
			if err == nil || !strings.Contains(err.Error(), want.deny) {
				t.Errorf("%q: want deny containing %q, got err=%v need=%v every=%v", cmd, want.deny, err, need, every)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected deny %v", cmd, err)
			continue
		}
		if !slices.Equal(need, want.need) || !slices.Equal(every, want.every) {
			t.Errorf("%q: need=%v every=%v, want need=%v every=%v", cmd, need, every, want.need, want.every)
		}
	}
}

func TestAllowAndRmNeverAllowlistable(t *testing.T) {
	s := newTestShell(t)
	s.Allow("python")
	if need, _, _ := s.Check("python x"); len(need) != 0 {
		t.Fatal("Allow must stick")
	}
	s.Allow("rm")
	if _, every, _ := s.Check("rm x"); len(every) != 1 {
		t.Fatal("rm stays ask-every-time even if allowed")
	}
	s.Allow("sudo")
	if _, _, err := s.Check("sudo x"); err == nil {
		t.Fatal("hard-deny has no override")
	}
}

func TestWindowsBestEffort(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	s := NewShell([]string{"go", "git"}, j, "windows")
	if need, every, err := s.Check("go test ./...; Remove-Item x"); err != nil || len(need) != 0 || !slices.Equal(every, []string{"remove-item"}) {
		t.Fatal(need, every, err)
	}
	if _, _, err := s.Check("Format-Volume C"); err == nil {
		t.Fatal("Format-Volume hard-denied")
	}
	if need, _, _ := s.Check("git status | python.exe x"); !slices.Equal(need, []string{"python"}) {
		t.Fatal(need)
	}
}
```

- [ ] **Step 3: Run** — `go test ./internal/permissions/ -run 'Shell|Allow|Windows'` → FAIL.

- [ ] **Step 4: Implement `shell.go`**

```go
// internal/permissions/shell.go
package permissions

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/syntax"
)

var hardDeny = map[string]bool{"sudo": true, "su": true, "doas": true, "dd": true, "shred": true, "chown": true}

var builtinsOK = map[string]bool{"cd": true, "pwd": true, "echo": true, "printf": true, "test": true,
	"[": true, "true": true, "false": true, "exit": true}

var refused = map[string]bool{"eval": true, "source": true, ".": true, "exec": true}

var askEvery = map[string]bool{"rm": true}

// Shell analyses commands for the model-facing shell tool (§7). Not a
// sandbox: an allowlisted interpreter can do anything the user can.
type Shell struct {
	mu    sync.Mutex
	allow map[string]bool
	jail  *Jail
	goos  string
}

func NewShell(allow []string, jail *Jail, goos string) *Shell {
	s := &Shell{allow: map[string]bool{}, jail: jail, goos: goos}
	for _, a := range allow {
		s.allow[a] = true
	}
	return s
}

func (s *Shell) Allow(name string) {
	s.mu.Lock()
	s.allow[strings.ToLower(name)] = true
	s.allow[name] = true
	s.mu.Unlock()
}

func (s *Shell) allowed(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.allow[name]
}

type verdictAcc struct {
	need, every []string
}

func (v *verdictAcc) add(list *[]string, name string) {
	if !slices.Contains(*list, name) {
		*list = append(*list, name)
	}
}

func (s *Shell) classify(name string, v *verdictAcc) error {
	base := filepath.Base(name)
	switch {
	case hardDeny[base] || strings.HasPrefix(base, "mkfs"):
		return fmt.Errorf("%s is never allowed (hard-deny)", base)
	case refused[base]:
		return fmt.Errorf("%s is refused: it runs code the analyser cannot see", base)
	case builtinsOK[base]:
	case askEvery[base]:
		v.add(&v.every, base)
	case !s.allowed(base):
		v.add(&v.need, base)
	}
	return nil
}

// wordLit returns the literal value of a word made only of literal and
// quoted-literal parts.
func wordLit(w *syntax.Word) (string, bool) {
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(x.Value)
		case *syntax.SglQuoted:
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, q := range x.Parts {
				l, ok := q.(*syntax.Lit)
				if !ok {
					return "", false
				}
				sb.WriteString(l.Value)
			}
		default:
			return "", false
		}
	}
	return sb.String(), true
}

// unwrap skips wrapper commands and their options/arguments, returning the
// index of the wrapped command name in args (or -1 if none).
func unwrap(args []*syntax.Word) (int, error) {
	i := 0
	for i < len(args) {
		name, ok := wordLit(args[i])
		if !ok {
			return i, nil
		}
		switch filepath.Base(name) {
		case "env":
			i++
			for i < len(args) {
				a, _ := wordLit(args[i])
				if strings.HasPrefix(a, "-") || strings.Contains(a, "=") {
					i++
					continue
				}
				break
			}
		case "time", "nohup":
			i++
			for i < len(args) {
				if a, _ := wordLit(args[i]); strings.HasPrefix(a, "-") {
					i++
					continue
				}
				break
			}
		case "nice":
			i++
			for i < len(args) {
				a, _ := wordLit(args[i])
				if a == "-n" {
					i += 2
					continue
				}
				if strings.HasPrefix(a, "-") {
					i++
					continue
				}
				break
			}
		case "timeout":
			i++
			for i < len(args) {
				a, _ := wordLit(args[i])
				if a == "-s" || a == "-k" || a == "--signal" || a == "--kill-after" {
					i += 2
					continue
				}
				if strings.HasPrefix(a, "-") {
					i++
					continue
				}
				break
			}
			i++ // the duration
		case "command":
			if i+1 < len(args) {
				if a, _ := wordLit(args[i+1]); a == "-v" || a == "-V" {
					return -1, nil // lookup only
				}
			}
			i++
		default:
			return i, nil
		}
	}
	return -1, nil
}

func (s *Shell) checkTarget(w *syntax.Word) error {
	t, ok := wordLit(w)
	if !ok {
		return fmt.Errorf("non-literal redirect/tee target refused")
	}
	if t == "/dev/null" || t == "/dev/stdout" || t == "/dev/stderr" {
		return nil
	}
	if _, err := s.jail.Resolve(t, true); err != nil {
		return err
	}
	return nil
}

func (s *Shell) Check(command string) ([]string, []string, error) {
	if s.goos == "windows" {
		return s.checkWindows(command)
	}
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse command (refused, no best-effort guessing): %v", err)
	}
	var v verdictAcc
	var firstErr error
	fail := func(e error) {
		if firstErr == nil {
			firstErr = e
		}
	}
	syntax.Walk(f, func(n syntax.Node) bool {
		if firstErr != nil {
			return false
		}
		switch x := n.(type) {
		case *syntax.Stmt:
			for _, r := range x.Redirs {
				switch r.Op {
				case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll, syntax.ClbOut:
					if err := s.checkTarget(r.Word); err != nil {
						fail(err)
					}
				case syntax.DplOut:
					if t, ok := wordLit(r.Word); !ok || (t != "-" && strings.Trim(t, "0123456789") != "") {
						if err := s.checkTarget(r.Word); err != nil {
							fail(err)
						}
					}
				}
			}
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				return true // pure assignment
			}
			idx, _ := unwrap(x.Args)
			if idx < 0 {
				return true // wrappers are transparent; nothing wrapped (or `command -v`)
			}
			name, ok := wordLit(x.Args[idx])
			if !ok {
				fail(fmt.Errorf("non-literal command name refused (e.g. $CMD); write the command out"))
				return false
			}
			if err := s.classify(name, &v); err != nil {
				fail(err)
				return false
			}
			if filepath.Base(name) == "tee" {
				for _, a := range x.Args[idx+1:] {
					if l, ok := wordLit(a); ok && strings.HasPrefix(l, "-") {
						continue
					}
					if err := s.checkTarget(a); err != nil {
						fail(err)
					}
				}
			}
		}
		return true
	})
	if firstErr != nil {
		return nil, nil, firstErr
	}
	return v.need, v.every, nil
}
```

```go
// internal/permissions/shell_windows_rules.go
package permissions

import (
	"fmt"
	"regexp"
	"strings"
)

var winSplit = regexp.MustCompile(`&&|\|\||[;|]`)

var winAsk = map[string]bool{"remove-item": true, "rm": true, "ri": true, "del": true, "erase": true, "rd": true, "rmdir": true}

// checkWindows is best-effort (no PowerShell parser in v1, §7): the first
// token of every ;/|/&&/|| segment is checked. Documented as weaker than Unix.
func (s *Shell) checkWindows(command string) ([]string, []string, error) {
	var v verdictAcc
	for _, seg := range winSplit.Split(command, -1) {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(fields[0]), ".exe")
		switch {
		case name == "format-volume" || hardDeny[name]:
			return nil, nil, fmt.Errorf("%s is never allowed (hard-deny)", name)
		case winAsk[name]:
			v.add(&v.every, name)
		case builtinsOK[name]:
		case !s.allowed(name):
			v.add(&v.need, name)
		}
	}
	return v.need, v.every, nil
}
```

- [ ] **Step 5: Run** — `go test ./internal/permissions/ -v`
Expected: PASS. Two ladder entries depend on parser behaviour and need a check during implementation:
- `"if then fi ("` must fail to parse. If mvdan parses it, replace the fixture with `"echo 'unterminated"`.
- `". env.sh"`: the `.` builtin's name literal is `"."`, which `refused` catches.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/permissions
git commit -m "feat(permissions): parsed shell command analysis (mvdan.cc/sh) + Windows best-effort"
```

---

### Task 9: Shell runner + `shell` tool

**Files:**
- Create: `internal/tools/shellrun.go`, `internal/tools/shellrun_unix.go` (`//go:build !windows`), `internal/tools/shellrun_windows.go` (`//go:build windows`), `internal/tools/shell.go`; modify `builtins.go`
- Test: `internal/tools/shellrun_test.go`, `internal/tools/shell_test.go`

**Interfaces:**
- Produces:
  - `type ShellOutput struct { Output string; ExitCode int; TimedOut bool }`
  - `func RunShell(ctx context.Context, dir string, env []string, command string, timeout time.Duration) (ShellOutput, error)` — reused by the TUI's `!`/`!!` (phase 3).
  - `func ShellEnv(base []string, strip []string) []string` — removes `strip` names, sets the four pager/editor vars.
  - `func Truncate(s string, max int) string` — head+tail line-based, `[… N lines omitted]`.
  - `shell` schema `{"command": string (required), "timeout": integer seconds 1..300, default 30}`. Content: the output (truncated), then `[exit N]` (`[timed out after Ns — process group killed]`). `IsError` when exit≠0, on timeout, or when refused.
  - Approval flow: `need` → `env.Ask(Question{Kind:"shell", Subject:name, Detail:command, CanAlways:true})`. On `AllowAlways` → `env.Commands.Allow(name)`. `every` → `Ask` with `CanAlways:false`. Any `Deny` → refusal naming the command. `env.Ask == nil` means deny.

- [ ] **Step 1: Write failing tests**

```go
// internal/tools/shellrun_test.go
//go:build !windows

package tools

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRunShellBasics(t *testing.T) {
	dir := t.TempDir()
	out, err := RunShell(context.Background(), dir, os.Environ(), "pwd; echo err >&2; printf noeol; exit 3", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Output, dir) || !strings.Contains(out.Output, "err") || !strings.HasSuffix(out.Output, "noeol") || out.ExitCode != 3 {
		t.Fatalf("%+v", out)
	}
}

func TestRunShellStdinIsDevNull(t *testing.T) {
	out, _ := RunShell(context.Background(), t.TempDir(), os.Environ(), "cat; echo done", 5*time.Second)
	if strings.TrimSpace(out.Output) != "done" {
		t.Fatalf("stdin must be /dev/null, got %q", out.Output)
	}
}

func TestRunShellTimeoutKillsGroup(t *testing.T) {
	start := time.Now()
	out, _ := RunShell(context.Background(), t.TempDir(), os.Environ(), "sleep 30 & sleep 30; echo never", 300*time.Millisecond)
	if !out.TimedOut || time.Since(start) > 5*time.Second || strings.Contains(out.Output, "never") {
		t.Fatalf("%+v after %v", out, time.Since(start))
	}
}

func TestRunShellInvalidUTF8(t *testing.T) {
	out, _ := RunShell(context.Background(), t.TempDir(), os.Environ(), `printf 'a\377b'`, 5*time.Second)
	if out.Output != "a\uFFFDb" {
		t.Fatalf("%q", out.Output)
	}
}

func TestShellEnv(t *testing.T) {
	env := ShellEnv([]string{"PATH=/bin", "OPENAI_API_KEY=sk", "PAGER=less", "HOME=/h"}, []string{"OPENAI_API_KEY"})
	for _, want := range []string{"PATH=/bin", "HOME=/h", "PAGER=cat", "GIT_PAGER=cat", "GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0"} {
		if !slices.Contains(env, want) {
			t.Errorf("missing %s in %v", want, env)
		}
	}
	for _, e := range env {
		if strings.HasPrefix(e, "OPENAI_API_KEY=") || e == "PAGER=less" {
			t.Errorf("leaked %s", e)
		}
	}
}

func TestTruncate(t *testing.T) {
	var sb strings.Builder
	for i := range 10000 {
		sb.WriteString(strings.Repeat("x", 9) + string(rune('0'+i%10)) + "\n")
	}
	got := Truncate(sb.String(), 30_000)
	if len(got) > 30_100 || !strings.Contains(got, "lines omitted]") {
		t.Fatalf("len %d", len(got))
	}
	if Truncate("short", 30_000) != "short" {
		t.Fatal("no-op under cap")
	}
}
```

```go
// internal/tools/shell_test.go
//go:build !windows

package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

type fakeCmds struct {
	need, every []string
	err         error
	allowed     []string
}

func (f *fakeCmds) Check(string) ([]string, []string, error) { return f.need, f.every, f.err }
func (f *fakeCmds) Allow(n string)                           { f.allowed = append(f.allowed, n) }

func TestShellToolApprovals(t *testing.T) {
	env, _ := testEnv(t)
	env.ShellEnv = os.Environ()
	cmds := &fakeCmds{need: []string{"python"}}
	env.Commands = cmds
	var asked []Question
	env.Ask = func(_ context.Context, q Question) Answer { asked = append(asked, q); return AllowAlways }
	r := run(t, shellTool{}, env, map[string]any{"command": "echo hi"})
	if r.IsError || !strings.Contains(r.Content, "hi") || !strings.Contains(r.Content, "[exit 0]") {
		t.Fatal(r.Content)
	}
	if len(asked) != 1 || !asked[0].CanAlways || len(cmds.allowed) != 1 {
		t.Fatalf("asked %+v allowed %v", asked, cmds.allowed)
	}
	cmds.need, cmds.every = nil, []string{"rm"}
	env.Ask = func(context.Context, Question) Answer { return Deny }
	r = run(t, shellTool{}, env, map[string]any{"command": "rm x"})
	if !r.IsError || !strings.Contains(r.Content, "rm") || !strings.Contains(r.Content, "refused") {
		t.Fatal(r.Content)
	}
	env.Ask = nil
	if r := run(t, shellTool{}, env, map[string]any{"command": "rm x"}); !r.IsError {
		t.Fatal("no asker = deny (one-shot mode)")
	}
	if r := run(t, shellTool{}, env, map[string]any{"command": "x", "timeout": 301}); !r.IsError {
		t.Fatal("timeout max 300")
	}
}

func TestShellToolRunsAtRoot(t *testing.T) {
	env, root := testEnv(t)
	env.ShellEnv = os.Environ()
	env.Commands = &fakeCmds{}
	os.Mkdir(root+"/sub", 0o755)
	run(t, shellTool{}, env, map[string]any{"command": "cd sub"})
	r := run(t, shellTool{}, env, map[string]any{"command": "pwd"})
	if !strings.HasPrefix(strings.TrimSpace(r.Content), root+"\n") && !strings.Contains(r.Content, root+"\n[exit") {
		t.Fatalf("stateless: every call starts at the root, got %q", r.Content)
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/tools/ -run 'Shell|Truncate'` → FAIL.

- [ ] **Step 3: Implement the runner**

```go
// internal/tools/shellrun.go
package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ShellOutput struct {
	Output   string
	ExitCode int
	TimedOut bool
}

var forcedEnv = []string{"PAGER=cat", "GIT_PAGER=cat", "GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0"}

func ShellEnv(base []string, strip []string) []string {
	drop := map[string]bool{"PAGER": true, "GIT_PAGER": true, "GIT_EDITOR": true, "GIT_TERMINAL_PROMPT": true}
	for _, s := range strip {
		drop[s] = true
	}
	var out []string
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if !drop[k] {
			out = append(out, kv)
		}
	}
	return append(out, forcedEnv...)
}

func RunShell(ctx context.Context, dir string, env []string, command string, timeout time.Duration) (ShellOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	name, args, err := shellCommand(command)
	if err != nil {
		return ShellOutput{}, err
	}
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, env
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	cmd.Stdin = devnull
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return ShellOutput{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var werr error
	select {
	case werr = <-done:
	case <-ctx.Done():
		killProcessGroup(cmd)
		werr = <-done
	}
	out := ShellOutput{Output: strings.ToValidUTF8(buf.String(), "\uFFFD")}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
	}
	var ee *exec.ExitError
	switch {
	case werr == nil:
	case errors.As(werr, &ee):
		out.ExitCode = ee.ExitCode()
	default:
		return out, werr
	}
	if ctx.Err() != nil && !out.TimedOut {
		return out, ctx.Err() // parent cancelled (esc / Ctrl-C)
	}
	return out, nil
}

// Truncate keeps whole lines from the head and tail, ~max/2 chars each.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	lines := strings.SplitAfter(s, "\n")
	half := max / 2
	var head, tail []string
	n := 0
	i := 0
	for ; i < len(lines) && n+len(lines[i]) <= half; i++ {
		head = append(head, lines[i])
		n += len(lines[i])
	}
	n = 0
	j := len(lines) - 1
	for ; j >= i && n+len(lines[j]) <= half; j-- {
		tail = append([]string{lines[j]}, tail...)
		n += len(lines[j])
	}
	omitted := j - i + 1
	return strings.Join(head, "") + fmt.Sprintf("[… %d lines omitted]\n", omitted) + strings.Join(tail, "")
}
```

```go
// internal/tools/shellrun_unix.go
//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

func shellCommand(command string) (string, []string, error) {
	return "bash", []string{"-c", command}, nil
}

func setProcessGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
```

```go
// internal/tools/shellrun_windows.go
//go:build windows

package tools

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
)

func shellCommand(command string) (string, []string, error) {
	for _, sh := range []string{"pwsh", "powershell.exe"} {
		if p, err := exec.LookPath(sh); err == nil {
			return p, []string{"-NoProfile", "-Command", command}, nil
		}
	}
	return "", nil, errors.New("neither pwsh nor powershell.exe found on PATH")
}

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}
```

- [ ] **Step 4: Implement the tool**

```go
// internal/tools/shell.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

const shellMaxOutput = 30_000

type shellTool struct{}

func (shellTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "shell", Description: "Run a shell command (bash -c on Unix, PowerShell on Windows). " +
		"Stateless: every call starts in the workdir, so use `cd dir && cmd` within one call. stdin is empty; " +
		"stdout and stderr are merged; output over 30K chars keeps head and tail. Commands are checked against " +
		"an allowlist; some need user approval and some are never allowed.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"command":{"type":"string","description":"The command line"},` +
			`"timeout":{"type":"integer","minimum":1,"maximum":300,"description":"Seconds (default 30)"}},` +
			`"required":["command"],"additionalProperties":false}`)}
}

func approve(ctx context.Context, env *Env, names []string, command string, canAlways bool) error {
	for _, n := range names {
		ans := Deny
		if env.Ask != nil {
			ans = env.Ask(ctx, Question{Kind: "shell", Subject: n, Detail: command, CanAlways: canAlways})
		}
		switch ans {
		case Deny:
			if canAlways {
				return fmt.Errorf("refused: `%s` is not allowlisted and the user did not approve it; ask the user to allow it or use another approach", n)
			}
			return fmt.Errorf("refused: `%s` needs explicit user approval for every call and it was not given", n)
		case AllowAlways:
			env.Commands.Allow(n)
		}
	}
	return nil
}

func (shellTool) Run(ctx context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Command string `json:"command"`
		Timeout *int   `json:"timeout"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	timeout := 30
	if a.Timeout != nil {
		timeout = *a.Timeout
	}
	if timeout < 1 || timeout > 300 {
		return errorf("timeout must be 1..300 seconds")
	}
	if strings.TrimSpace(a.Command) == "" {
		return errorf("empty command")
	}
	need, every, err := env.Commands.Check(a.Command)
	if err != nil {
		return errorf("refused: %v", err)
	}
	if err := approve(ctx, env, need, a.Command, true); err != nil {
		return errorf("%v", err)
	}
	if err := approve(ctx, env, every, a.Command, false); err != nil {
		return errorf("%v", err)
	}
	out, err := RunShell(ctx, env.Root, env.ShellEnv, a.Command, time.Duration(timeout)*time.Second)
	if err != nil {
		return errorf("%v", err)
	}
	body := Truncate(out.Output, shellMaxOutput)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	status := fmt.Sprintf("[exit %d]", out.ExitCode)
	if out.TimedOut {
		status = fmt.Sprintf("[timed out after %ds — process group killed]", timeout)
	}
	short := a.Command
	if len(short) > 60 {
		short = short[:57] + "..."
	}
	return Result{Content: body + status, IsError: out.ExitCode != 0 || out.TimedOut,
		Summary: fmt.Sprintf("%s %s", short, status), Detail: Truncate(out.Output, shellMaxOutput)}
}
```

Register `shellTool{}`.

- [ ] **Step 5: Run** — `go test ./internal/tools/ -race -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools
git commit -m "feat(tools): stateless shell with process-group kill, env hygiene, approvals, truncation"
```

---

### Task 10: `mcp` stub + final tool order + freeze the schemas

**Files:**
- Create: `internal/tools/mcpstub.go`; modify `internal/tools/builtins.go`; regenerate `internal/tools/testdata/schemas.golden.json`

**Interfaces:**
- Produces: `mcpStub` with the **final** `mcp` schema: `{"action": enum ["search","describe","call"] (required), "server": string, "tool": string, "args": object, "query": string}`. Every action returns error content `no MCP servers configured`. Phase 5 replaces it via `Registry.Register` with an identical `Spec()`.
- `func MCPSpec() llm.ToolSpec` — exported so phase 5's real tool returns the byte-identical spec.
- `Builtins()` order: `read, write, edit, shell, search, ls, mcp` (§4 table order).

- [ ] **Step 1: Implement**

```go
// internal/tools/mcpstub.go
package tools

import (
	"context"
	"encoding/json"

	"github.com/adeotek/moca/internal/llm"
)

// MCPSpec is the frozen ~200-token schema of the lazy MCP proxy (§10.5).
// The phase-5 implementation in internal/mcp returns exactly this spec.
func MCPSpec() llm.ToolSpec {
	return llm.ToolSpec{Name: "mcp", Description: "Use tools from configured MCP servers (listed in the system " +
		"prompt). action=search finds tools by keyword (query, optional server); action=describe returns one " +
		"tool's input schema (server, tool); action=call runs it (server, tool, args). Describe before the first call of a tool.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"action":{"type":"string","enum":["search","describe","call"]},` +
			`"server":{"type":"string"},"tool":{"type":"string"},` +
			`"args":{"type":"object","description":"Tool arguments for call"},` +
			`"query":{"type":"string","description":"Keywords for search"}},` +
			`"required":["action"],"additionalProperties":false}`)}
}

type mcpStub struct{}

func (mcpStub) Spec() llm.ToolSpec { return MCPSpec() }

func (mcpStub) Run(context.Context, *Env, json.RawMessage) Result {
	return errorf("no MCP servers configured")
}
```

```go
// internal/tools/builtins.go
package tools

// Builtins returns the seven frozen tools in schema order (§4).
func Builtins() []Tool {
	return []Tool{readTool{}, writeTool{}, editTool{}, shellTool{}, searchTool{}, lsTool{}, mcpStub{}}
}
```

- [ ] **Step 2: Freeze**

Run: `go test ./internal/tools/ -run TestSchemasFrozen -update && go test ./internal/tools/`
Expected: PASS. Inspect `testdata/schemas.golden.json`: seven entries, in order. From now on the golden only changes through a design revision.

- [ ] **Step 3: Commit**

```bash
git add internal/tools
git commit -m "feat(tools): mcp stub with final schema; freeze all seven tool schemas"
```

---

### Task 11: Project trust store

**Files:**
- Create: `internal/permissions/trust.go`
- Test: `internal/permissions/trust_test.go`

**Interfaces:**
- Produces: `func LoadTrust(path string) (*TrustStore, error)` (missing file → empty); `func (t *TrustStore) Lookup(dir string) (trusted, known bool)`; `func (t *TrustStore) Set(dir string, trusted bool) error` (atomic write, file 0600, dir 0700; keys are canonical paths).
- Decision rule used by `agent.Start` (Task 16): `-p` → `--approve`/`--no-approve` if given, else **no**. This follows §7's "default no" literally, and saved decisions are ignored in `-p`. The TUI (phase 3) → the saved decision if known, else the interactive prompt.

- [ ] **Step 1: Write failing test**

```go
// internal/permissions/trust_test.go
package permissions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrustStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data", "trust.json")
	ts, err := LoadTrust(path)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "w")
	os.Mkdir(work, 0o755)
	if _, known := ts.Lookup(work); known {
		t.Fatal("empty store")
	}
	if err := ts.Set(work, true); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "lnk")
	os.Symlink(work, link)
	ts2, _ := LoadTrust(path)
	if tr, known := ts2.Lookup(link); !known || !tr {
		t.Fatal("canonical path lookup via symlink")
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/permissions/trust.go
package permissions

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

type TrustStore struct {
	mu   sync.Mutex
	path string
	m    map[string]bool
}

func LoadTrust(path string) (*TrustStore, error) {
	t := &TrustStore{path: path, m: map[string]bool{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return t, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &t.m); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TrustStore) Lookup(dir string) (bool, bool) {
	c, err := canonical(dir)
	if err != nil {
		return false, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	v, ok := t.m[c]
	return v, ok
}

func (t *TrustStore) Set(dir string, trusted bool) error {
	c, err := canonical(dir)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.m[c] = trusted
	b, _ := json.MarshalIndent(t.m, "", "  ")
	t.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return err
	}
	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.path)
}
```

- [ ] **Step 4: Run** — `go test ./internal/permissions/ -run Trust -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/permissions
git commit -m "feat(permissions): project trust store"
```

---

### Task 12: Skills loader, built-in extraction, AGENTS.md

**Files:**
- Create: `internal/skills/frontmatter.go`, `internal/skills/skills.go`, `internal/skills/builtin.go`, `internal/skills/builtin/rtk/SKILL.md`, `internal/skills/instructions.go`
- Test: `internal/skills/frontmatter_test.go`, `internal/skills/skills_test.go`, `internal/skills/instructions_test.go`

**Interfaces:**
- Produces:
  - `func ParseFrontmatter(data []byte) (map[string]string, []byte, error)` — `---`-delimited YAML subset: `key: value`, quoted scalars, `|`/`>` block scalars (folded joins lines with spaces), nested maps/lists skipped, BOM + CRLF tolerated.
  - `type Skill struct { Name, Description, ArgumentHint, Path, Source string }` (`Path` = absolute SKILL.md path).
  - `type Dir struct { Path, Source string }`
  - `func Discover(dirs []Dir) ([]Skill, []error)` — first source wins on a name collision; result sorted by name.
  - `func ExtractBuiltins(dataDir, version string) (string, error)` — writes the embedded `builtin/**` to `<dataDir>/builtin-skills/<version>/` if missing, returns that dir.
  - `type Instruction struct { Path, Content string; Truncated bool }`
  - `func LoadInstructions(globalDir, workdir string, trusted bool) ([]Instruction, error)` — order: `globalDir/AGENTS.md`, then (trusted only) `workdir/AGENTS.md` or else `workdir/CLAUDE.md`; each capped at 32K chars with `[… truncated at 32K chars]`.

- [ ] **Step 1: Write failing tests**

```go
// internal/skills/frontmatter_test.go
package skills

import "testing"

func TestFrontmatter(t *testing.T) {
	src := "\ufeff---\r\nname: graphify\r\ndescription: >\r\n  Build a knowledge\r\n  graph of code.\r\nallowed-tools: Bash\r\nmetadata:\r\n  version: 2\r\n  tags: [a, b]\r\nargument-hint: \"[path]\"\r\n---\r\n# Body\r\n"
	fm, body, err := ParseFrontmatter([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if fm["name"] != "graphify" || fm["description"] != "Build a knowledge graph of code." || fm["argument-hint"] != "[path]" {
		t.Fatalf("%#v", fm)
	}
	if _, ok := fm["version"]; ok {
		t.Fatal("nested keys must not leak to top level")
	}
	if string(body) != "# Body\n" {
		t.Fatalf("%q", body)
	}
	fm, _, _ = ParseFrontmatter([]byte("---\ndescription: |\n  line one\n  line two\nname: 'x'\n---\n"))
	if fm["description"] != "line one\nline two" || fm["name"] != "x" {
		t.Fatalf("%#v", fm)
	}
	if _, _, err := ParseFrontmatter([]byte("no frontmatter")); err == nil {
		t.Fatal("missing frontmatter is an error")
	}
}
```

```go
// internal/skills/skills_test.go
package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func mkSkill(t *testing.T, dir, name, desc string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, name), 0o755)
	os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\nbody\n"), 0o644)
}

func TestDiscoverPrecedence(t *testing.T) {
	proj, global := t.TempDir(), t.TempDir()
	mkSkill(t, proj, "deploy", "project deploy")
	mkSkill(t, global, "deploy", "global deploy")
	mkSkill(t, global, "notes", "take notes")
	os.MkdirAll(filepath.Join(global, "broken"), 0o755)
	os.WriteFile(filepath.Join(global, "broken", "SKILL.md"), []byte("---\nname: broken\n---\n"), 0o644)
	got, errs := Discover([]Dir{{proj, "project"}, {global, "global"}, {filepath.Join(proj, "missing"), "x"}})
	if len(got) != 2 || got[0].Name != "deploy" || got[0].Description != "project deploy" || got[1].Name != "notes" {
		t.Fatalf("%+v", got)
	}
	if !filepath.IsAbs(got[0].Path) {
		t.Fatal("absolute path")
	}
	if len(errs) != 1 {
		t.Fatalf("missing description is reported, missing dir is not: %v", errs)
	}
}

func TestExtractBuiltins(t *testing.T) {
	data := t.TempDir()
	dir, err := ExtractBuiltins(data, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(data, "builtin-skills", "1.2.3") {
		t.Fatal(dir)
	}
	got, errs := Discover([]Dir{{dir, "builtin"}})
	if len(errs) != 0 || len(got) != 1 || got[0].Name != "rtk" {
		t.Fatalf("%+v %v", got, errs)
	}
}
```

```go
// internal/skills/instructions_test.go
package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadInstructions(t *testing.T) {
	global, work := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(global, "AGENTS.md"), []byte("global rules"), 0o644)
	os.WriteFile(filepath.Join(work, "CLAUDE.md"), []byte("claude rules"), 0o644)
	got, _ := LoadInstructions(global, work, false)
	if len(got) != 1 || got[0].Content != "global rules" {
		t.Fatalf("untrusted: %+v", got)
	}
	got, _ = LoadInstructions(global, work, true)
	if len(got) != 2 || got[1].Content != "claude rules" {
		t.Fatalf("CLAUDE.md fallback: %+v", got)
	}
	os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte(strings.Repeat("a", 40_000)), 0o644)
	got, _ = LoadInstructions(global, work, true)
	if !got[1].Truncated || !strings.HasSuffix(got[1].Content, "[… truncated at 32K chars]") || !strings.HasSuffix(got[1].Path, "AGENTS.md") {
		t.Fatal("AGENTS.md wins over CLAUDE.md and is capped")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/skills/` → FAIL.

- [ ] **Step 3: Implement `frontmatter.go`**

```go
// internal/skills/frontmatter.go
package skills

import (
	"bytes"
	"errors"
	"strings"
)

// ParseFrontmatter reads the YAML-subset frontmatter used by SKILL.md and
// prompt templates across pi / claude-code / opencode. Only top-level
// scalar keys are returned; nested maps and lists are skipped.
func ParseFrontmatter(data []byte) (map[string]string, []byte, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, nil, errors.New("no frontmatter (file must start with ---)")
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil, nil, errors.New("unterminated frontmatter")
	}
	head := text[4 : 4+end]
	body := strings.TrimPrefix(text[4+end+4:], "\n")
	lines := strings.Split(head, "\n")
	fm := map[string]string{}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if l == "" || strings.HasPrefix(l, "#") || l[0] == ' ' || l[0] == '\t' || strings.HasPrefix(l, "- ") {
			continue
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if v == "|" || v == ">" || v == "|-" || v == ">-" {
			var block []string
			for i+1 < len(lines) && (lines[i+1] == "" || lines[i+1][0] == ' ' || lines[i+1][0] == '\t') {
				i++
				block = append(block, strings.TrimSpace(lines[i]))
			}
			sep := "\n"
			if v[0] == '>' {
				sep = " "
			}
			fm[k] = strings.TrimSpace(strings.Join(block, sep))
			continue
		}
		if v == "" {
			continue // nested map or list follows
		}
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		fm[k] = v
	}
	return fm, []byte(body), nil
}
```

- [ ] **Step 4: Implement `skills.go`, `builtin.go`, `instructions.go`**

```go
// internal/skills/skills.go

// Package skills loads Agent Skills (SKILL.md), embedded built-in skills and
// project instructions (AGENTS.md). Only name + description + absolute path
// enter the system prompt; bodies are loaded by the model with `read` (§9).
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Skill struct {
	Name, Description, ArgumentHint, Path, Source string
}

type Dir struct{ Path, Source string }

func Discover(dirs []Dir) ([]Skill, []error) {
	seen := map[string]bool{}
	var out []Skill
	var errs []error
	for _, d := range dirs {
		ents, err := os.ReadDir(d.Path)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(d.Path, e.Name(), "SKILL.md")
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			fm, _, err := ParseFrontmatter(data)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", p, err))
				continue
			}
			name := fm["name"]
			if name == "" {
				name = e.Name()
			}
			if strings.TrimSpace(fm["description"]) == "" {
				errs = append(errs, fmt.Errorf("%s: missing description; skill skipped", p))
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			abs, _ := filepath.Abs(p)
			out = append(out, Skill{Name: name, Description: fm["description"], ArgumentHint: fm["argument-hint"], Path: abs, Source: d.Source})
		}
	}
	slices.SortFunc(out, func(a, b Skill) int { return strings.Compare(a.Name, b.Name) })
	return out, errs
}
```

```go
// internal/skills/builtin.go
package skills

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed builtin
var builtinFS embed.FS

// ExtractBuiltins materializes the embedded skills so they have real paths
// the model can `read` (a read-only jail root, §7).
func ExtractBuiltins(dataDir, version string) (string, error) {
	dst := filepath.Join(dataDir, "builtin-skills", version)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	tmp := dst + ".tmp"
	os.RemoveAll(tmp)
	err := fs.WalkDir(builtinFS, "builtin", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("builtin", p)
		target := filepath.Join(tmp, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := builtinFS.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	return dst, os.Rename(tmp, dst)
}
```

```go
// internal/skills/instructions.go
package skills

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const instructionCap = 32_000

type Instruction struct {
	Path      string
	Content   string
	Truncated bool
}

func LoadInstructions(globalDir, workdir string, trusted bool) ([]Instruction, error) {
	var out []Instruction
	load := func(p string) (bool, error) {
		b, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		in := Instruction{Path: p, Content: string(b)}
		if len(in.Content) > instructionCap {
			in.Content, in.Truncated = in.Content[:instructionCap]+"\n[… truncated at 32K chars]", true
		}
		out = append(out, in)
		return true, nil
	}
	if _, err := load(filepath.Join(globalDir, "AGENTS.md")); err != nil {
		return nil, err
	}
	if trusted {
		found, err := load(filepath.Join(workdir, "AGENTS.md"))
		if err != nil {
			return nil, err
		}
		if !found {
			if _, err := load(filepath.Join(workdir, "CLAUDE.md")); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
```

`internal/skills/builtin/rtk/SKILL.md` (phase 6 verifies it against rtk's real CLI and refines it):

```markdown
---
name: rtk
description: Token-compressed CLI proxy. Prefer `rtk <cmd>` over raw git/ls/grep/test/diff/docker/gh output — same information, far fewer tokens.
---
# rtk — prefer compressed command output

`rtk` wraps common CLIs and condenses their output while keeping every signal
(errors, failing tests, changed files). Use it whenever a wrapped variant exists:

| Instead of | Use |
|---|---|
| `git status` / `git log` / `git diff` | `rtk git status` / `rtk git log` / `rtk git diff` |
| `go test ./...` / `pytest` / `npm test` | `rtk test -- go test ./...` (any test runner after `--`) |
| `ls -R`, `tree` | `rtk ls <dir>` |
| `cat big.log` | `rtk read big.log` |
| `docker ps`, `gh pr list` | `rtk docker ps`, `rtk gh pr list` |

Rules:
- If rtk output is unusable (empty when output was expected, garbled, or contradicting the exit code), re-run as `rtk proxy <cmd>` for the raw output.
- `rtk` is allowlisted, but the wrapped command is checked too: `rtk <cmd>` is only allowed when `<cmd>` is.
- Don't wrap commands whose full output you must parse exactly (e.g. `git diff` you will apply as a patch).
```

- [ ] **Step 5: Run** — `go test ./internal/skills/ -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/skills
git commit -m "feat(skills): SKILL.md discovery, embedded built-ins (rtk), AGENTS.md/CLAUDE.md loading"
```

---

### Task 13: Session schema, writer, rebuild, transcript repair

**Files:**
- Create: `internal/session/entry.go`, `internal/session/writer.go`, `internal/session/rebuild.go`
- Test: `internal/session/writer_test.go`, `internal/session/rebuild_test.go`

**Interfaces:**
- Produces (phase 4 extends `Messages` with compaction; the names here are final):

```go
const (TypeSession="session"; TypeMessage="message"; TypeToolUse="tool_use"; TypeToolResult="tool_result";
       TypeCompaction="compaction"; TypeModelChange="model_change"; TypeSnapshot="snapshot"; TypeError="error")
type Header struct { Workdir, Provider, Model, Effort string; StartedAt time.Time; MocaVersion, SystemPrompt string }
type ToolUse struct { MessageID string; Call llm.ToolCall }
type Compaction struct { Summary, FirstKeptEntryID string; TokensBefore int; Usage llm.Usage; ReadFiles, ModifiedFiles []string }
type ModelChange struct { Model, Effort string }
type SnapshotRec struct { Path, Blob, After string; Git bool; Mode uint32 }  // Blob "" = file did not exist
type ErrorInfo struct { Message string }
type Entry struct {
	ID, ParentID, Type string; Time time.Time
	Session *Header; Message *llm.Message; ToolUse *ToolUse; ToolResult *llm.ToolResult
	Usage *llm.Usage; Model string; Cost float64
	Compaction *Compaction; ModelChange *ModelChange; Snapshot *SnapshotRec; Error *ErrorInfo
}
func Create(dir string, h Header, slug string) (*Writer, error)
func Open(path string) (*Writer, []Entry, error)
func (w *Writer) Append(e Entry) (Entry, error)
func (w *Writer) Path() string
func (w *Writer) ID8() string
func (w *Writer) Close() error
func ReadFile(path string) ([]Entry, error)
func Slug(prompt string) string
func Messages(entries []Entry) []llm.Message
func Repair(entries []Entry, reason string) []Entry  // synthetic tool_result entries (un-appended)
const AbortedByUser = "aborted by user"
const Interrupted = "interrupted — moca exited before completion"
```

- Rebuild rule: an assistant `message` entry plus every `tool_use` entry whose `MessageID` equals its id form one assistant `llm.Message`, with tool_use blocks appended after the text/thinking. Consecutive user-side entries (`tool_result`, user `message`) merge into one user `llm.Message`, results first in append order. `session`, `model_change`, `snapshot`, `error` entries are skipped.

- [ ] **Step 1: Write failing tests**

```go
// internal/session/writer_test.go
package session

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

func TestCreateAppendRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	w, err := Create(dir, Header{Workdir: "/w", Model: "a/b", SystemPrompt: "SYS"}, Slug("Fix the failing TestFoo in pkg/x, please!"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`/\d{4}-\d{2}-\d{2}-fix-the-failing-testfoo-in-[0-9a-f]{8}\.jsonl$`).MatchString(w.Path()) {
		t.Fatal(w.Path())
	}
	e1, _ := w.Append(Entry{Type: TypeMessage, Message: &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}})
	e2, _ := w.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "x"}})
	w.Close()
	if e2.ParentID != e1.ID || e1.ParentID == "" || e1.Time.IsZero() {
		t.Fatal("chain")
	}
	got, err := ReadFile(w.Path())
	if err != nil || len(got) != 3 || got[0].Type != TypeSession || got[0].Session.SystemPrompt != "SYS" {
		t.Fatalf("%+v %v", got, err)
	}
	fi, _ := os.Stat(w.Path())
	di, _ := os.Stat(dir)
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Fatal("modes")
	}
	w2, entries, err := Open(w.Path())
	if err != nil || len(entries) != 3 {
		t.Fatal(err)
	}
	e3, _ := w2.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "y"}})
	if e3.ParentID != e2.ID {
		t.Fatal("Open continues the chain")
	}
	_ = time.Now
}

func TestSlug(t *testing.T) {
	if Slug("") != "session" || Slug("!!!") != "session" || Slug("Héllo Wörld") != "h-llo-w-rld" {
		t.Fatal(Slug("Héllo Wörld"))
	}
}
```

```go
// internal/session/rebuild_test.go
package session

import (
	"encoding/json"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func txt(r llm.Role, s string) *llm.Message {
	return &llm.Message{Role: r, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: s}}}
}

func TestMessagesRebuild(t *testing.T) {
	es := []Entry{
		{ID: "s", Type: TypeSession},
		{ID: "u1", Type: TypeMessage, Message: txt(llm.RoleUser, "q")},
		{ID: "a1", Type: TypeMessage, Message: txt(llm.RoleAssistant, "looking")},
		{ID: "t1", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{}`)}}},
		{ID: "t2", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c2", Name: "ls", Input: json.RawMessage(`{}`)}}},
		{ID: "r1", Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x"}},
		{ID: "sn", Type: TypeSnapshot, Snapshot: &SnapshotRec{Path: "/f"}},
		{ID: "r2", Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c2", Content: "y"}},
		{ID: "u2", Type: TypeMessage, Message: txt(llm.RoleUser, "steer")},
		{ID: "a2", Type: TypeMessage, Message: txt(llm.RoleAssistant, "done")},
	}
	m := Messages(es)
	if len(m) != 4 {
		t.Fatalf("want user, assistant, user, assistant; got %d", len(m))
	}
	if len(m[1].Content) != 3 || m[1].Content[2].ToolCall.ID != "c2" {
		t.Fatalf("assistant + 2 tool_use: %+v", m[1].Content)
	}
	if len(m[2].Content) != 3 || m[2].Content[0].ToolResult.CallID != "c1" || m[2].Content[2].Text != "steer" {
		t.Fatalf("results then steering text: %+v", m[2].Content)
	}
}

func TestRepair(t *testing.T) {
	es := []Entry{
		{ID: "a1", Type: TypeMessage, Message: txt(llm.RoleAssistant, "")},
		{ID: "t1", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c1"}}},
		{ID: "t2", Type: TypeToolUse, ToolUse: &ToolUse{MessageID: "a1", Call: llm.ToolCall{ID: "c2"}}},
		{ID: "r1", Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "ok"}},
	}
	fix := Repair(es, Interrupted)
	if len(fix) != 1 || fix[0].ToolResult.CallID != "c2" || fix[0].ToolResult.Content != Interrupted || !fix[0].ToolResult.IsError {
		t.Fatalf("%+v", fix)
	}
	if len(Repair(append(es, fix...), Interrupted)) != 0 {
		t.Fatal("idempotent")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/session/` → FAIL.

- [ ] **Step 3: Implement `entry.go`**

```go
// internal/session/entry.go

// Package session owns the append-only JSONL transcript (§8) and the
// pre-edit snapshot store (§4).
package session

import (
	"time"

	"github.com/adeotek/moca/internal/llm"
)

const (
	TypeSession     = "session"
	TypeMessage     = "message"
	TypeToolUse     = "tool_use"
	TypeToolResult  = "tool_result"
	TypeCompaction  = "compaction"
	TypeModelChange = "model_change"
	TypeSnapshot    = "snapshot"
	TypeError       = "error"
)

const (
	AbortedByUser = "aborted by user"
	Interrupted   = "interrupted — moca exited before completion"
)

type Header struct {
	Workdir      string    `json:"workdir"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Effort       string    `json:"effort"`
	StartedAt    time.Time `json:"startedAt"`
	MocaVersion  string    `json:"mocaVersion"`
	SystemPrompt string    `json:"systemPrompt"`
}

type ToolUse struct {
	MessageID string       `json:"messageId"`
	Call      llm.ToolCall `json:"call"`
}

type Compaction struct {
	Summary          string    `json:"summary"`
	FirstKeptEntryID string    `json:"firstKeptEntryId"`
	TokensBefore     int       `json:"tokensBefore"`
	Usage            llm.Usage `json:"usage"`
	ReadFiles        []string  `json:"readFiles,omitempty"`
	ModifiedFiles    []string  `json:"modifiedFiles,omitempty"`
}

type ModelChange struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

type SnapshotRec struct {
	Path  string `json:"path"`
	Blob  string `json:"blob,omitempty"`  // sha256 of pre-edit content; "" = did not exist
	After string `json:"after,omitempty"` // sha256 of content moca wrote
	Git   bool   `json:"git,omitempty"`   // skipped: tracked + clean in git
	Mode  uint32 `json:"mode,omitempty"`  // pre-edit permission bits, restored by undo
}

type ErrorInfo struct {
	Message string `json:"message"`
}

type Entry struct {
	ID          string          `json:"id"`
	ParentID    string          `json:"parentId,omitempty"`
	Type        string          `json:"type"`
	Time        time.Time       `json:"ts"`
	Session     *Header         `json:"session,omitempty"`
	Message     *llm.Message    `json:"message,omitempty"`
	ToolUse     *ToolUse        `json:"toolUse,omitempty"`
	ToolResult  *llm.ToolResult `json:"toolResult,omitempty"`
	Usage       *llm.Usage      `json:"usage,omitempty"`
	Model       string          `json:"model,omitempty"` // producing model on assistant messages
	Cost        float64         `json:"cost,omitempty"`
	Compaction  *Compaction     `json:"compaction,omitempty"`
	ModelChange *ModelChange    `json:"modelChange,omitempty"`
	Snapshot    *SnapshotRec    `json:"snapshot,omitempty"`
	Error       *ErrorInfo      `json:"error,omitempty"`
}
```

- [ ] **Step 4: Implement `writer.go`**

```go
// internal/session/writer.go
package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Writer struct {
	mu   sync.Mutex
	f    *os.File
	path string
	id8  string
	last string
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug: first five words of the prompt, lowercased, ASCII-only, ≤32 chars.
func Slug(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) > 5 {
		words = words[:5]
	}
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.Join(words, " ")), "-"), "-")
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	if s == "" {
		return "session"
	}
	return s
}

func Create(dir string, h Header, slug string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	os.Chmod(dir, 0o700)
	if h.StartedAt.IsZero() {
		h.StartedAt = time.Now()
	}
	for range 10 {
		id8 := newID()[:8]
		p := filepath.Join(dir, fmt.Sprintf("%s-%s-%s.jsonl", h.StartedAt.Format("2006-01-02"), slug, id8))
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		w := &Writer{f: f, path: p, id8: id8}
		if _, err := w.Append(Entry{Type: TypeSession, Session: &h}); err != nil {
			f.Close()
			return nil, err
		}
		return w, nil
	}
	return nil, errors.New("could not allocate a unique session id")
}

func Open(path string) (*Writer, []Entry, error) {
	entries, err := ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}
	base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	w := &Writer{f: f, path: path, id8: base[max(0, len(base)-8):]}
	if len(entries) > 0 {
		w.last = entries[len(entries)-1].ID
	}
	return w, entries, nil
}

func (w *Writer) Append(e Entry) (Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e.ID == "" {
		e.ID = newID()
	}
	e.ParentID = w.last
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	b, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	if _, err := w.f.Write(append(b, '\n')); err != nil {
		return e, err
	}
	w.last = e.ID
	return e, nil
}

func (w *Writer) Path() string { return w.path }
func (w *Writer) ID8() string  { return w.id8 }
func (w *Writer) Close() error { return w.f.Close() }

// ReadFile tolerates a truncated last line (crash mid-write).
func ReadFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			break
		}
		out = append(out, e)
	}
	return out, sc.Err()
}
```

- [ ] **Step 5: Implement `rebuild.go`**

```go
// internal/session/rebuild.go
package session

import "github.com/adeotek/moca/internal/llm"

// Messages rebuilds request messages from transcript entries (no
// compaction handling yet — phase 4 adds it).
func Messages(entries []Entry) []llm.Message {
	var out []llm.Message
	assistantIdx := map[string]int{}
	for _, e := range entries {
		switch e.Type {
		case TypeMessage:
			if e.Message.Role == llm.RoleAssistant {
				out = append(out, llm.Message{Role: llm.RoleAssistant, Content: append([]llm.ContentBlock{}, e.Message.Content...)})
				assistantIdx[e.ID] = len(out) - 1
				continue
			}
			appendUser(&out, e.Message.Content...)
		case TypeToolUse:
			if i, ok := assistantIdx[e.ToolUse.MessageID]; ok {
				call := e.ToolUse.Call
				out[i].Content = append(out[i].Content, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: &call})
			}
		case TypeToolResult:
			r := *e.ToolResult
			appendUser(&out, llm.ContentBlock{Type: llm.BlockToolResult, ToolResult: &r})
		}
	}
	return out
}

func appendUser(out *[]llm.Message, blocks ...llm.ContentBlock) {
	if n := len(*out); n > 0 && (*out)[n-1].Role == llm.RoleUser {
		(*out)[n-1].Content = append((*out)[n-1].Content, blocks...)
		return
	}
	*out = append(*out, llm.Message{Role: llm.RoleUser, Content: append([]llm.ContentBlock{}, blocks...)})
}

// Repair returns synthetic error results for every tool_use without a
// tool_result, in call order. The caller appends them before the next
// request (abort: AbortedByUser; resume after crash: Interrupted).
func Repair(entries []Entry, reason string) []Entry {
	answered := map[string]bool{}
	for _, e := range entries {
		if e.Type == TypeToolResult {
			answered[e.ToolResult.CallID] = true
		}
	}
	var fix []Entry
	for _, e := range entries {
		if e.Type == TypeToolUse && !answered[e.ToolUse.Call.ID] {
			fix = append(fix, Entry{Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: e.ToolUse.Call.ID, Content: reason, IsError: true}})
		}
	}
	return fix
}
```

- [ ] **Step 6: Run** — `go test ./internal/session/ -v` → PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/session
git commit -m "feat(session): JSONL schema with id/parentId chain, rebuild, transcript repair"
```

---

### Task 14: Snapshots + undo stack

**Files:**
- Create: `internal/session/snapshot.go`
- Test: `internal/session/snapshot_test.go`

**Interfaces:**
- Consumes: `Writer.Append`, `SnapshotRec`.
- Produces:
  - `func NewSnapshots(w *Writer, blobDir string, prior []Entry) *Snapshots` — `prior` rebuilds the undo stack on resume (phase 4).
  - `func (s *Snapshots) Guard(abs string, write func() error) error` — satisfies `tools.Snapshotter`. If the file is tracked and clean in git → record `{Git:true}`. Otherwise store the pre-edit content at `blobDir/<sha256>` (or `Blob:""` if absent), run `write`, then record `After = sha256(new content)` and append a `snapshot` entry.
  - `func (s *Snapshots) Undo() (string, error)` — pops the newest record. Git record → message `"<path> was clean in git — restore it with: git restore -- <path>"`. Otherwise it refuses if the current sha ≠ `After` ("changed since moca wrote it"), then restores the blob or deletes the file when `Blob == ""`, and appends an `error`-free `snapshot` entry with `Path` + `Blob:"undo"` marker so resume skips undone records.
  - `func Prune(blobDir string, days int) error` — removes blobs with mtime older than `days` (0 = no-op).
  - `func gitClean(abs string) bool` — `git -C <dir> ls-files --error-unmatch -- <file>` succeeds AND `git -C <dir> status --porcelain -- <file>` is empty.

- [ ] **Step 1: Write failing tests**

```go
// internal/session/snapshot_test.go
package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newSnap(t *testing.T) (*Snapshots, string) {
	t.Helper()
	base := t.TempDir()
	w, _ := Create(filepath.Join(base, "s"), Header{}, "t")
	t.Cleanup(func() { w.Close() })
	return NewSnapshots(w, filepath.Join(base, "blobs"), nil), base
}

func TestSnapshotUndoEditAndCreate(t *testing.T) {
	s, base := newSnap(t)
	f := filepath.Join(base, "f.txt")
	os.WriteFile(f, []byte("v1"), 0o640)
	s.Guard(f, func() error { return os.WriteFile(f, []byte("v2"), 0o644) })
	os.Chmod(f, 0o644) // undo must restore 0640 regardless of the current mode
	n := filepath.Join(base, "new.txt")
	s.Guard(n, func() error { return os.WriteFile(n, []byte("x"), 0o644) })

	if msg, err := s.Undo(); err != nil || !strings.Contains(msg, "new.txt") {
		t.Fatal(msg, err)
	}
	if _, err := os.Stat(n); !os.IsNotExist(err) {
		t.Fatal("undo of a create deletes the file")
	}
	if _, err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f); string(b) != "v1" {
		t.Fatal(string(b))
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode not restored: %v", fi.Mode())
	}
	if _, err := s.Undo(); err == nil {
		t.Fatal("empty stack")
	}
}

func TestUndoRefusesIfChangedSince(t *testing.T) {
	s, base := newSnap(t)
	f := filepath.Join(base, "f.txt")
	os.WriteFile(f, []byte("v1"), 0o644)
	s.Guard(f, func() error { return os.WriteFile(f, []byte("v2"), 0o644) })
	os.WriteFile(f, []byte("user edit"), 0o644)
	if _, err := s.Undo(); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatal(err)
	}
}

func TestGitCleanSkipsSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	s, base := newSnap(t)
	repo := filepath.Join(base, "repo")
	os.Mkdir(repo, 0o755)
	gitc := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(string(out))
		}
	}
	gitc("init", "-q")
	f := filepath.Join(repo, "a.go")
	os.WriteFile(f, []byte("x"), 0o644)
	gitc("add", ".")
	gitc("commit", "-qm", "i")
	s.Guard(f, func() error { return os.WriteFile(f, []byte("y"), 0o644) })
	msg, err := s.Undo()
	if err != nil || !strings.Contains(msg, "git restore -- ") {
		t.Fatal(msg, err)
	}
	if ents, _ := os.ReadDir(filepath.Join(base, "blobs")); len(ents) != 0 {
		t.Fatal("no blob for git-clean files")
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old")
	os.WriteFile(old, nil, 0o600)
	os.Chtimes(old, time.Now().AddDate(0, 0, -40), time.Now().AddDate(0, 0, -40))
	os.WriteFile(filepath.Join(dir, "new"), nil, 0o600)
	Prune(dir, 30)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old blob pruned")
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatal("fresh blob kept")
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/session/snapshot.go
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Snapshots struct {
	mu    sync.Mutex
	w     *Writer
	dir   string
	stack []SnapshotRec
}

func NewSnapshots(w *Writer, blobDir string, prior []Entry) *Snapshots {
	s := &Snapshots{w: w, dir: blobDir}
	for _, e := range prior {
		if e.Type != TypeSnapshot {
			continue
		}
		if e.Snapshot.Blob == "undo" {
			if n := len(s.stack); n > 0 {
				s.stack = s.stack[:n-1]
			}
			continue
		}
		s.stack = append(s.stack, *e.Snapshot)
	}
	return s
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func gitClean(abs string) bool {
	dir, file := filepath.Dir(abs), filepath.Base(abs)
	if exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", "--", file).Run() != nil {
		return false
	}
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain", "--", file).Output()
	return err == nil && len(strings.TrimSpace(string(out))) == 0
}

func (s *Snapshots) Guard(abs string, write func() error) error {
	rec := SnapshotRec{Path: abs}
	prev, err := os.ReadFile(abs)
	switch {
	case err == nil && gitClean(abs):
		rec.Git = true
	case err == nil:
		rec.Blob = sum(prev)
		if fi, err := os.Stat(abs); err == nil {
			rec.Mode = uint32(fi.Mode().Perm())
		}
		if err := os.MkdirAll(s.dir, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(s.dir, rec.Blob), prev, 0o600); err != nil {
			return err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := write(); err != nil {
		return err
	}
	if now, err := os.ReadFile(abs); err == nil {
		rec.After = sum(now)
	}
	s.mu.Lock()
	s.stack = append(s.stack, rec)
	s.mu.Unlock()
	_, err = s.w.Append(Entry{Type: TypeSnapshot, Snapshot: &rec})
	return err
}

func (s *Snapshots) Undo() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.stack) == 0 {
		return "", errors.New("nothing to undo in this session (changes made via shell are never covered)")
	}
	rec := s.stack[len(s.stack)-1]
	pop := func() {
		s.stack = s.stack[:len(s.stack)-1]
		s.w.Append(Entry{Type: TypeSnapshot, Snapshot: &SnapshotRec{Path: rec.Path, Blob: "undo"}})
	}
	if rec.Git {
		pop()
		return fmt.Sprintf("%s was clean in git — restore it with: git restore -- %s", rec.Path, rec.Path), nil
	}
	cur, err := os.ReadFile(rec.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if sum(cur) != rec.After {
		return "", fmt.Errorf("%s changed since moca wrote it; undo refused", rec.Path)
	}
	if rec.Blob == "" {
		if err := os.Remove(rec.Path); err != nil {
			return "", err
		}
		pop()
		return fmt.Sprintf("removed %s (moca created it)", rec.Path), nil
	}
	b, err := os.ReadFile(filepath.Join(s.dir, rec.Blob))
	if err != nil {
		return "", fmt.Errorf("snapshot blob missing (pruned?): %w", err)
	}
	mode := fs.FileMode(0o644)
	if rec.Mode != 0 {
		mode = fs.FileMode(rec.Mode)
	}
	if err := os.WriteFile(rec.Path, b, mode); err != nil {
		return "", err
	}
	pop()
	return fmt.Sprintf("restored %s", rec.Path), nil
}

func Prune(blobDir string, days int) error {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	ents, err := os.ReadDir(blobDir)
	if err != nil {
		return nil
	}
	for _, e := range ents {
		if fi, err := e.Info(); err == nil && fi.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(blobDir, e.Name()))
		}
	}
	return nil
}
```

- [ ] **Step 4: Run** — `go test ./internal/session/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/session
git commit -m "feat(session): pre-edit snapshots, undo stack, git-clean skip, prune"
```

---

### Task 15: System prompt builder

**Files:**
- Create: `internal/agent/prompt.go`
- Test: `internal/agent/prompt_test.go`

**Interfaces:**
- Produces:
  - `type ServerLine struct { Name, Description string }`
  - `type PromptInput struct { Workdir, OS, Arch, Date, Git, Version string; Skills []skills.Skill; Servers []ServerLine; Instructions []skills.Instruction }`
  - `func BuildSystemPrompt(in PromptInput) string`
  - `func GitState(dir string) string` — `branch <name>[, N uncommitted changes]` / `not a git repository`.
  - `func Platform() (os, arch string)` — `uname -s`/`uname -m` style names from `runtime` (`linux`→`Linux`, `darwin`→`Darwin`, `windows`→`Windows`; `amd64`→`x86_64`, `arm64`→`arm64`).

- [ ] **Step 1: Write failing test**

```go
// internal/agent/prompt_test.go
package agent

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func TestBuildSystemPrompt(t *testing.T) {
	p := BuildSystemPrompt(PromptInput{
		Workdir: "/w", OS: "Linux", Arch: "x86_64", Date: "2026-10-04", Git: "branch main, 2 uncommitted changes", Version: "0.1.0",
		Skills:       []skills.Skill{{Name: "rtk", Description: "compressed output", Path: "/d/rtk/SKILL.md"}},
		Servers:      []ServerLine{{Name: "context7", Description: "docs lookup"}},
		Instructions: []skills.Instruction{{Path: "/w/AGENTS.md", Content: "Use tabs."}},
	})
	for _, want := range []string{"/w", "Linux x86_64", "2026-10-04", "branch main", "`shell` is stateless",
		"rtk", "- rtk: compressed output (/d/rtk/SKILL.md)", "- context7: docs lookup", "Use tabs.", "moca 0.1.0"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Count(BuildSystemPrompt(PromptInput{}), "\n") > 60 {
		t.Fatal("core prompt must stay ~40 lines")
	}
	// version stamp sits at the end of the moca text (before appended sections)
	core := BuildSystemPrompt(PromptInput{Version: "9"})
	if !strings.HasSuffix(strings.TrimSpace(core), "moca 9") {
		t.Fatal("version stamp last")
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/agent/prompt.go
package agent

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/adeotek/moca/internal/skills"
)

type ServerLine struct{ Name, Description string }

type PromptInput struct {
	Workdir, OS, Arch, Date, Git, Version string
	Skills                                []skills.Skill
	Servers                               []ServerLine
	Instructions                          []skills.Instruction
}

const coreTemplate = `You are moca, a coding agent working in a user's repository through tools.

# Environment
- Workdir: %s (all relative paths resolve here; file tools cannot leave it)
- Platform: %s %s
- Date: %s
- Git at session start: %s (may be stale; check with git when it matters)

# Tools
- read before you write or edit an existing file; edit refuses files you have not read or that changed since.
- Prefer edit (small exact replacements) over write for existing files. Copy old_string without read's N| prefixes.
- read pages large files: use offset/limit instead of re-reading whole files.
- search finds code (RE2 regex, respects .gitignore); ls lists one directory.
- ` + "`shell` is stateless" + `: every call starts in the workdir. Use ` + "`cd dir && cmd`" + ` in one call.
- shell commands are checked against an allowlist. If one is refused, do not retry variants that
  do the same thing (find -delete, python -c …); explain what you need and ask the user.
- Tool calls in one turn run in order; a failed call does not stop the rest.
- mcp gives access to the MCP servers listed below: search, then describe, then call.

# Working style
- Do the task end to end: understand, change, verify (build/tests), then report briefly.
- Keep changes minimal and in the style of the surrounding code. Don't add unrequested features.
- When something fails, read the error and fix the cause; don't loop on the same failing call.
- Final answer: what changed, how it was verified, anything left open. No filler.

# Token discipline
- Every tool result costs tokens on every later turn. Read windows, not whole files; search before reading.
- Prefer rtk-prefixed variants where they exist (e.g. ` + "`rtk git status`, `rtk test -- go test ./...`" + `).
- Don't echo file contents or tool output back to the user; summarize.

moca %s`

func BuildSystemPrompt(in PromptInput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, coreTemplate, in.Workdir, in.OS, in.Arch, in.Date, in.Git, in.Version)
	if len(in.Skills) > 0 {
		sb.WriteString("\n\n# Skills\nLoad a skill's instructions with read on its path when the task matches.\n")
		for _, s := range in.Skills {
			fmt.Fprintf(&sb, "- %s: %s (%s)\n", s.Name, oneLine(s.Description), s.Path)
		}
	}
	if len(in.Servers) > 0 {
		sb.WriteString("\n# MCP servers (use the mcp tool)\n")
		for _, s := range in.Servers {
			if s.Description != "" {
				fmt.Fprintf(&sb, "- %s: %s\n", s.Name, s.Description)
			} else {
				fmt.Fprintf(&sb, "- %s\n", s.Name)
			}
		}
	}
	for _, i := range in.Instructions {
		fmt.Fprintf(&sb, "\n# Project instructions (%s)\n%s\n", i.Path, strings.TrimSpace(i.Content))
	}
	return sb.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func Platform() (string, string) {
	osName := map[string]string{"linux": "Linux", "darwin": "Darwin", "windows": "Windows"}[runtime.GOOS]
	if osName == "" {
		osName = runtime.GOOS
	}
	arch := map[string]string{"amd64": "x86_64", "386": "i686"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	return osName, arch
}

func GitState(dir string) string {
	branch, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "not a git repository"
	}
	s := "branch " + strings.TrimSpace(string(branch))
	if st, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output(); err == nil {
		if n := len(strings.Split(strings.TrimSpace(string(st)), "\n")); strings.TrimSpace(string(st)) != "" {
			s += fmt.Sprintf(", %d uncommitted changes", n)
		}
	}
	return s
}
```

- [ ] **Step 4: Run** — `go test ./internal/agent/ -run Prompt -v` → PASS. Then read the rendered prompt once (`go test -run Prompt -v` with a `t.Log(p)`) and check the line count. The core must stay ~40 lines (§13).

- [ ] **Step 5: Commit**

```bash
git add internal/agent
git commit -m "feat(agent): frozen system prompt builder"
```

---

### Task 16: Agent loop + `Start` wiring

**Files:**
- Create: `internal/agent/events.go`, `internal/agent/agent.go`, `internal/agent/start.go`
- Test: `internal/agent/agent_test.go`

**Interfaces:**
- Consumes: everything above + `provider.Registry`/`Model`/`Adapter`/`RetryNotice`.
- Produces (phase 3 TUI and phase 4 compaction build on these; names are final):

```go
// events.go
type Event interface{ isEvent() }
type TextDelta struct{ Text string }
type ThinkingDelta struct{ Text string }
type StreamReset struct{}
type ToolStart struct{ Call llm.ToolCall }
type ToolEnd struct{ Call llm.ToolCall; Result tools.Result }
type TurnEnd struct{ Message llm.Message; Usage llm.Usage; Cost float64; Stop llm.StopReason }
type Retry struct{ Notice provider.RetryNotice }

// agent.go
type Options struct {
	Config    config.Config
	Providers *provider.Registry
	Tools     *tools.Registry
	Env       *tools.Env
	Session   *session.Writer
	Snapshots *session.Snapshots
	System    string
	Model     string     // qualified
	Effort    llm.Effort // requested; clamped per model
	Emit      func(Event)
	Prior     []session.Entry // resume (phase 4); nil for a new session
}
type Outcome struct { Text string; MaxSteps bool }
func New(o Options) (*Agent, error)
func (a *Agent) Run(ctx context.Context, prompt string) (Outcome, error)
func (a *Agent) Model() provider.Model
func (a *Agent) Effort() llm.Effort          // effective (clamped)
func (a *Agent) Totals() (llm.Usage, float64)
func (a *Agent) Session() *session.Writer

// start.go
type StartOptions struct {
	Config   config.Config
	Workdir  string
	Model    string // "" → Config.Model
	Effort   string // "" → model default
	Trusted  bool
	Ask      tools.Asker
	Emit     func(Event)
	HTTP     *http.Client
	Slug     string
}
func Start(o StartOptions) (*Agent, error)
```

- Loop contract:
  1. Append the user `message` entry.
  2. Loop: build the request (`System`, rebuilt messages, `Tools.Specs()`, `ToolChoiceAuto`, `MaxTokens = m.MaxTokens(cfg.Context.ReserveTokens)`, clamped effort). Stream with `Emit` forwarding. Append the assistant `message` entry (text/thinking blocks only) with `Usage`, `Model`, `Cost`, then one `tool_use` entry per call.
  3. No tool calls → return `Outcome{Text}`.
  4. Execute calls **sequentially**: `ToolStart`, `Tools.Run`, append `tool_result`, `ToolEnd`. If `ctx` is cancelled before a call, append `AbortedByUser` results for it and every remaining call (`session.Repair`), then return `ctx.Err()`.
  5. Stop `length` with tool calls → each call gets an error result telling the model the output was cut off and to split the work (it is not executed).
  6. After `Config.Context.MaxSteps` tool batches, send one wrap-up: append user text `"You have reached the step limit for this run. Do not call tools. Summarize what you did, what remains, and how to continue."` and request with `ToolChoiceNone`, tools still listed. Return `Outcome{Text, MaxSteps:true}`.
  7. Provider error → append an `error` entry and return the error.

`Start` builds, in order:
  1. `jail` over `Workdir`, with read-only roots `ConfigDir()/skills`, the builtin-skills dir and, when trusted, `<workdir>/.moca/skills`.
  2. `NewShell(cfg.Shell.Allow, jail, runtime.GOOS)`.
  3. `ExtractBuiltins(DataDir(), Version)` and skills dirs `[workdir/.moca/skills (trusted only), ConfigDir()/skills, builtin]`.
  4. `LoadInstructions(ConfigDir(), Workdir, Trusted)`.
  5. MCP server roster from `cfg.MCP.Servers` (sorted).
  6. The prompt.
  7. `session.Prune(DataDir()/snapshot, cfg.RetentionDays())`.
  8. `session.Create(DataDir()/sessions, Header{…SystemPrompt}, slug)` and `NewSnapshots`.
  9. `tools.Env{…, ShellEnv: tools.ShellEnv(os.Environ(), config.EnvRefs(cfg))}`.
  10. `New(...)`.

- [ ] **Step 1: Write failing tests (scripted adapter via a fake provider server)**

```go
// internal/agent/agent_test.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// scriptServer serves openai-completions SSE responses from a script, one
// per request, and records request bodies.
type scriptServer struct {
	mu     sync.Mutex
	turns  []string
	bodies []map[string]any
	srv    *httptest.Server
}

func textTurn(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

func toolTurn(calls ...[2]string) string {
	var sb strings.Builder
	for i, c := range calls {
		args, _ := json.Marshal(c[1])
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"id\":\"call_%d\",\"function\":{\"name\":%q,\"arguments\":%s}}]}}]}\n\n", i, i, c[0], args)
	}
	sb.WriteString("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
	return sb.String()
}

func newScript(t *testing.T, turns ...string) *scriptServer {
	s := &scriptServer{turns: turns}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		s.bodies = append(s.bodies, m)
		w.Header().Set("Content-Type", "text/event-stream")
		if len(s.turns) == 0 {
			io.WriteString(w, textTurn("(script exhausted)"))
			return
		}
		io.WriteString(w, s.turns[0])
		s.turns = s.turns[1:]
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func startTest(t *testing.T, s *scriptServer, maxSteps int) (*Agent, string, *[]Event) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	work := t.TempDir()
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m","context":{"maxSteps":%d},
		"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536}}}}}`, maxSteps, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	var evs []Event
	a, err := Start(StartOptions{Config: cfg, Workdir: work, Emit: func(e Event) { evs = append(evs, e) }, HTTP: s.srv.Client(), Slug: "t"})
	if err != nil {
		t.Fatal(err)
	}
	return a, work, &evs
}

func TestLoopReadEditVerify(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"read", `{"path":"a.txt"}`}, [2]string{"ls", `{}`}),
		toolTurn([2]string{"edit", `{"path":"a.txt","old_string":"bug","new_string":"fix"}`}),
		textTurn("Fixed."),
	)
	a, work, evs := startTest(t, s, 40)
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("bug\n"), 0o644)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || out.Text != "Fixed." || out.MaxSteps {
		t.Fatal(out, err)
	}
	if b, _ := os.ReadFile(filepath.Join(work, "a.txt")); string(b) != "fix\n" {
		t.Fatal(string(b))
	}
	// third request carries both tool results of turn 1 + the edit result, in order
	msgs := s.bodies[2]["messages"].([]any)
	var roles []string
	for _, m := range msgs {
		roles = append(roles, m.(map[string]any)["role"].(string))
	}
	if strings.Join(roles, ",") != "system,user,assistant,tool,tool,assistant,tool" {
		t.Fatal(roles)
	}
	var starts int
	for _, e := range *evs {
		if _, ok := e.(ToolStart); ok {
			starts++
		}
	}
	if starts != 3 {
		t.Fatal("tool events")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if entries[0].Type != session.TypeSession || !strings.Contains(entries[0].Session.SystemPrompt, "moca") {
		t.Fatal("session header with stored prompt")
	}
	if got := session.Messages(entries); len(got) != 6 {
		t.Fatalf("transcript rebuilds the same conversation: %d", len(got))
	}
}

func TestFailedCallDoesNotCancelRest(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"read", `{"path":"nope"}`}, [2]string{"ls", `{}`}), textTurn("ok"))
	a, _, evs := startTest(t, s, 40)
	a.Run(context.Background(), "go")
	var ends []ToolEnd
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok {
			ends = append(ends, te)
		}
	}
	if len(ends) != 2 || !ends[0].Result.IsError || ends[1].Result.IsError {
		t.Fatal(ends)
	}
}

func TestMaxStepsWrapUp(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}), toolTurn([2]string{"ls", `{}`}), textTurn("Summary: did ls twice."))
	a, _, _ := startTest(t, s, 2)
	out, err := a.Run(context.Background(), "loop")
	if err != nil || !out.MaxSteps || out.Text != "Summary: did ls twice." {
		t.Fatal(out, err)
	}
	last := s.bodies[len(s.bodies)-1]
	if last["tool_choice"] != "none" || last["tools"] == nil {
		t.Fatal("wrap-up: tool_choice none, tools still listed (cache prefix)")
	}
}

func TestShellRefusedInOneShot(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"shell", `{"command":"rm -rf x"}`}), textTurn("couldn't"))
	a, _, evs := startTest(t, s, 40)
	a.Run(context.Background(), "clean")
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok && (!te.Result.IsError || !strings.Contains(te.Result.Content, "rm")) {
			t.Fatal(te.Result)
		}
	}
}

func TestAbortWritesSyntheticResults(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}, [2]string{"ls", `{}`}))
	a, _, _ := startTest(t, s, 40)
	ctx, cancel := context.WithCancel(context.Background())
	a.opts.Emit = func(e Event) {
		if _, ok := e.(ToolEnd); ok {
			cancel()
		}
	}
	_, err := a.Run(ctx, "x")
	if err == nil {
		t.Fatal("cancelled run returns ctx error")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if len(session.Repair(entries, "")) != 0 {
		t.Fatal("every tool_use must have a result after abort")
	}
	last := entries[len(entries)-1]
	if last.ToolResult == nil || last.ToolResult.Content != session.AbortedByUser {
		t.Fatalf("%+v", last)
	}
}

func TestLengthStopTruncatedToolCall(t *testing.T) {
	trunc := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c\",\"function\":{\"name\":\"write\",\"arguments\":\"{\\\"path\\\":\\\"x\\\",\\\"content\\\":\\\"aaa\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n"
	s := newScript(t, trunc, textTurn("ok"))
	a, work, evs := startTest(t, s, 40)
	a.Run(context.Background(), "x")
	if _, err := os.Stat(filepath.Join(work, "x")); err == nil {
		t.Fatal("truncated call must not execute")
	}
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok && !strings.Contains(te.Result.Content, "split") {
			t.Fatal(te.Result.Content)
		}
	}
	_ = llm.StopLength
}
```

- [ ] **Step 2: Run** — `go test ./internal/agent/` → FAIL.

- [ ] **Step 3: Implement `events.go`**

```go
// internal/agent/events.go
package agent

import (
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/tools"
)

// Event is what the agent reports to its driver (TUI or -p printer).
type Event interface{ isEvent() }

type TextDelta struct{ Text string }
type ThinkingDelta struct{ Text string }
type StreamReset struct{}
type ToolStart struct{ Call llm.ToolCall }
type ToolEnd struct {
	Call   llm.ToolCall
	Result tools.Result
}
type TurnEnd struct {
	Message llm.Message
	Usage   llm.Usage
	Cost    float64
	Stop    llm.StopReason
}
type Retry struct{ Notice provider.RetryNotice }

func (TextDelta) isEvent()     {}
func (ThinkingDelta) isEvent() {}
func (StreamReset) isEvent()   {}
func (ToolStart) isEvent()     {}
func (ToolEnd) isEvent()       {}
func (TurnEnd) isEvent()       {}
func (Retry) isEvent()         {}
```

- [ ] **Step 4: Implement `agent.go`**

```go
// internal/agent/agent.go

// Package agent runs the loop: user message → model → tool calls → results
// → repeat. It is the only package that imports all the others (§2).
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

const wrapUpText = "You have reached the step limit for this run. Do not call tools. " +
	"Summarize what you did, what remains, and how to continue."

type Options struct {
	Config    config.Config
	Providers *provider.Registry
	Tools     *tools.Registry
	Env       *tools.Env
	Session   *session.Writer
	Snapshots *session.Snapshots
	System    string
	Model     string
	Effort    llm.Effort
	Emit      func(Event)
	Prior     []session.Entry
}

type Outcome struct {
	Text     string
	MaxSteps bool
}

type Agent struct {
	opts    Options
	model   provider.Model
	adapter provider.Adapter
	effort  llm.Effort
	entries []session.Entry // full transcript (in-memory mirror)
	usage   llm.Usage
	cost    float64
}

func New(o Options) (*Agent, error) {
	a := &Agent{opts: o, entries: o.Prior}
	if err := a.setModel(o.Model, o.Effort); err != nil {
		return nil, err
	}
	for _, e := range o.Prior {
		if e.Usage != nil {
			a.usage = a.usage.Add(*e.Usage)
			a.cost += e.Cost
		}
	}
	return a, nil
}

func (a *Agent) setModel(q string, e llm.Effort) error {
	m, ad, err := a.opts.Providers.Resolve(q)
	if err != nil {
		return err
	}
	if e == "" {
		e = m.DefaultEffort()
	}
	a.model, a.adapter, a.effort = m, ad, m.ClampEffort(e)
	return nil
}

func (a *Agent) Model() provider.Model         { return a.model }
func (a *Agent) Effort() llm.Effort             { return a.effort }
func (a *Agent) Totals() (llm.Usage, float64)   { return a.usage, a.cost }
func (a *Agent) Session() *session.Writer       { return a.opts.Session }
func (a *Agent) emit(e Event) {
	if a.opts.Emit != nil {
		a.opts.Emit(e)
	}
}

func (a *Agent) append(e session.Entry) (session.Entry, error) {
	e, err := a.opts.Session.Append(e)
	if err == nil {
		a.entries = append(a.entries, e)
	}
	return e, err
}

func userText(s string) *llm.Message {
	return &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: s}}}
}

func (a *Agent) request(choice llm.ToolChoice) llm.Request {
	return llm.Request{
		Model:      a.model.ID,
		System:     a.opts.System,
		Messages:   session.Messages(a.entries),
		Tools:      a.opts.Tools.Specs(),
		ToolChoice: choice,
		MaxTokens:  a.model.MaxTokens(a.opts.Config.Context.ReserveTokens),
		Effort:     a.effort,
	}
}

// turn streams one model response and persists it. It returns the response
// and the persisted tool calls.
func (a *Agent) turn(ctx context.Context, choice llm.ToolChoice) (llm.Response, []llm.ToolCall, error) {
	resp, err := a.adapter.Stream(ctx, a.request(choice), func(e llm.Event) {
		switch e.Type {
		case llm.EventText:
			a.emit(TextDelta{e.Text})
		case llm.EventThinking:
			a.emit(ThinkingDelta{e.Text})
		case llm.EventReset:
			a.emit(StreamReset{})
		}
	})
	if err != nil {
		a.append(session.Entry{Type: session.TypeError, Error: &session.ErrorInfo{Message: err.Error()}})
		return resp, nil, err
	}
	var body []llm.ContentBlock
	var calls []llm.ToolCall
	for _, c := range resp.Message.Content {
		if c.Type == llm.BlockToolUse {
			calls = append(calls, *c.ToolCall)
		} else {
			body = append(body, c)
		}
	}
	cost := a.model.CostOf(resp.Usage)
	a.usage, a.cost = a.usage.Add(resp.Usage), a.cost+cost
	u := resp.Usage
	msgEntry, err := a.append(session.Entry{Type: session.TypeMessage, Model: a.model.Qualified(), Usage: &u, Cost: cost,
		Message: &llm.Message{Role: llm.RoleAssistant, Content: body}})
	if err != nil {
		return resp, nil, err
	}
	for _, c := range calls {
		if _, err := a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: msgEntry.ID, Call: c}}); err != nil {
			return resp, nil, err
		}
	}
	a.emit(TurnEnd{Message: resp.Message, Usage: resp.Usage, Cost: cost, Stop: resp.Stop})
	return resp, calls, nil
}

func textOf(m llm.Message) string {
	var sb strings.Builder
	for _, c := range m.Content {
		if c.Type == llm.BlockText {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

func (a *Agent) result(call llm.ToolCall, r tools.Result) error {
	_, err := a.append(session.Entry{Type: session.TypeToolResult,
		ToolResult: &llm.ToolResult{CallID: call.ID, Content: r.Content, IsError: r.IsError}})
	a.emit(ToolEnd{Call: call, Result: r})
	return err
}

// abort appends synthetic results for every unanswered call.
func (a *Agent) abort(reason string) {
	for _, fix := range session.Repair(a.entries, reason) {
		a.append(fix)
	}
}

func (a *Agent) Run(ctx context.Context, prompt string) (Outcome, error) {
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(prompt)}); err != nil {
		return Outcome{}, err
	}
	for step := 0; ; step++ {
		if step == a.opts.Config.Context.MaxSteps {
			a.append(session.Entry{Type: session.TypeMessage, Message: userText(wrapUpText)})
			resp, _, err := a.turn(ctx, llm.ToolChoiceNone)
			if err != nil {
				return Outcome{}, err
			}
			return Outcome{Text: textOf(resp.Message), MaxSteps: true}, nil
		}
		resp, calls, err := a.turn(ctx, llm.ToolChoiceAuto)
		if err != nil {
			return Outcome{}, err
		}
		if len(calls) == 0 {
			return Outcome{Text: textOf(resp.Message)}, nil
		}
		for _, call := range calls {
			if ctx.Err() != nil {
				a.abort(session.AbortedByUser)
				return Outcome{}, ctx.Err()
			}
			a.emit(ToolStart{Call: call})
			var r tools.Result
			if resp.Stop == llm.StopLength {
				r = tools.Result{IsError: true, Content: fmt.Sprintf("your output was cut off at the token limit, so this %s call "+
					"was not executed. Split the work into smaller calls (e.g. write a short file, then extend it with edit).", call.Name)}
			} else {
				r = a.opts.Tools.Run(ctx, a.opts.Env, call)
			}
			if err := a.result(call, r); err != nil {
				return Outcome{}, err
			}
		}
		if ctx.Err() != nil {
			a.abort(session.AbortedByUser)
			return Outcome{}, ctx.Err()
		}
	}
}
```

- [ ] **Step 5: Implement `start.go`**

```go
// internal/agent/start.go
package agent

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tools"
)

type StartOptions struct {
	Config  config.Config
	Workdir string
	Model   string
	Effort  string
	Trusted bool
	Ask     tools.Asker
	Emit    func(Event)
	HTTP    *http.Client
	Slug    string
}

func Start(o StartOptions) (*Agent, error) {
	cfg := o.Config
	if o.Model != "" {
		cfg.Model = o.Model
	}
	hc := o.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	var emit func(Event) = o.Emit
	reg, err := provider.NewRegistry(cfg, hc, func(n provider.RetryNotice) {
		if emit != nil {
			emit(Retry{n})
		}
	})
	if err != nil {
		return nil, err
	}
	builtinDir, err := skills.ExtractBuiltins(config.DataDir(), config.Version)
	if err != nil {
		return nil, err
	}
	globalSkills := filepath.Join(config.ConfigDir(), "skills")
	projectSkills := filepath.Join(o.Workdir, ".moca", "skills")
	jail, err := permissions.NewJail(o.Workdir, []string{globalSkills, builtinDir})
	if err != nil {
		return nil, err
	}
	var dirs []skills.Dir
	if o.Trusted {
		dirs = append(dirs, skills.Dir{Path: projectSkills, Source: "project"})
	}
	dirs = append(dirs, skills.Dir{Path: globalSkills, Source: "global"}, skills.Dir{Path: builtinDir, Source: "builtin"})
	sk, _ := skills.Discover(dirs)
	instr, err := skills.LoadInstructions(config.ConfigDir(), o.Workdir, o.Trusted)
	if err != nil {
		return nil, err
	}
	var servers []ServerLine
	for name, s := range cfg.MCP.Servers {
		servers = append(servers, ServerLine{Name: name, Description: s.Description})
	}
	slices.SortFunc(servers, func(a, b ServerLine) int { return strings.Compare(a.Name, b.Name) })
	osName, arch := Platform()
	system := BuildSystemPrompt(PromptInput{Workdir: jail.Root(), OS: osName, Arch: arch,
		Date: time.Now().Format("2006-01-02"), Git: GitState(jail.Root()), Version: config.Version,
		Skills: sk, Servers: servers, Instructions: instr})

	session.Prune(filepath.Join(config.DataDir(), "snapshot"), cfg.RetentionDays())
	m, _, err := reg.Resolve(cfg.Model)
	if err != nil {
		return nil, err
	}
	effort := m.DefaultEffort()
	if o.Effort != "" {
		e, _ := llm.ParseEffort(o.Effort)
		effort = m.ClampEffort(e)
	}
	slug := o.Slug
	if slug == "" {
		slug = "session"
	}
	w, err := session.Create(filepath.Join(config.DataDir(), "sessions"), session.Header{
		Workdir: jail.Root(), Provider: m.Provider, Model: cfg.Model, Effort: string(effort),
		MocaVersion: config.Version, SystemPrompt: system}, slug)
	if err != nil {
		return nil, err
	}
	snaps := session.NewSnapshots(w, filepath.Join(config.DataDir(), "snapshot"), nil)
	env := &tools.Env{Root: jail.Root(), Paths: jail, Commands: permissions.NewShell(cfg.Shell.Allow, jail, runtime.GOOS),
		Ask: o.Ask, Reads: tools.NewReadTracker(), Snap: snaps,
		ShellEnv: tools.ShellEnv(os.Environ(), config.EnvRefs(cfg))}
	return New(Options{Config: cfg, Providers: reg, Tools: tools.NewRegistry(tools.Builtins()...), Env: env,
		Session: w, Snapshots: snaps, System: system, Model: cfg.Model, Effort: effort, Emit: o.Emit})
}
```

`projectSkills` needs no read-only root: when trusted it is already inside the jail. The header records the **effective** (clamped) effort, which is why the model resolves before `session.Create`.

- [ ] **Step 6: Run** — `go test ./internal/agent/ -race -v` → PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/agent
git commit -m "feat(agent): loop with sequential tools, maxSteps wrap-up, abort repair; Start wiring"
```

---

### Task 17: Yolo mode (§7.5)

**Files:**
- Create: `internal/permissions/yolo.go`, `internal/agent/yolo.go`
- Modify: `internal/tools/tool.go` (add `AutoAllow`), `internal/session/entry.go`, `internal/agent/agent.go`, `internal/agent/start.go`
- Test: `internal/permissions/yolo_test.go`, `internal/agent/yolo_test.go`

**Interfaces:**
- Yolo is implemented by **swapping the injected checkers**, never by `if yolo` branches inside tools. That is the payoff of §2's injection design: the tools are unchanged.
- `permissions`:
  - `type Unjailed struct{…}`; `func NewUnjailed(root string) *Unjailed`; `func (u *Unjailed) Resolve(path string, write bool) (string, error)` — `~` expands, relative paths join `root`, `filepath.Clean`; never refuses. Satisfies `tools.PathChecker`.
  - `type AllowAll struct{}`; `Check` returns `nil, nil, nil` for any input, unparseable included; `Allow` is a no-op. Satisfies `tools.CommandChecker`.
- `tools`: `func AutoAllow(context.Context, Question) Answer { return AllowOnce }` — the yolo `Asker`, which auto-approves anything that still asks (MCP gating in phase 5).
- `session`:
  - `Header.Yolo bool` (`json:"yolo,omitempty"`);
  - `const TypePermissionMode = "permission_mode"`;
  - `type PermissionMode struct { Yolo bool \`json:"yolo"\` }`;
  - `Entry.PermissionMode *PermissionMode` (`json:"permissionMode,omitempty"`).
  
  `Messages` already skips unknown entry types; `Repair` ignores it.
- `agent`:
  - `StartOptions.Yolo bool`.
  - `Agent` keeps the strict set it was built with: `strict struct{ paths tools.PathChecker; cmds tools.CommandChecker; ask tools.Asker }`.
  - `func (a *Agent) Yolo() bool`.
  - `func (a *Agent) SetYolo(on bool)`: swaps `Env.Paths/Commands/Ask` (yolo: `NewUnjailed(root)`, `AllowAll{}`, `tools.AutoAllow`; off: the strict set), appends a `permission_mode` entry, and emits `YoloChanged{On bool}`. **It must not be called while `Run` is in progress.** The TUI only allows `/yolo` between runs (phase 3), which keeps `Env` free of data races without locks.
  - `Start` applies yolo at construction **without** a `permission_mode` entry; `Header.Yolo` records it.
- Trust rule for callers (`-p` here, TUI in phase 3): an explicit `--approve`/`--no-approve` wins; otherwise yolo → trusted; otherwise the existing default.
- What yolo does **not** touch: `maxSteps`, `ReadTracker` (read-before-write + stale-on-disk), snapshots, `ShellEnv` secret stripping, shell timeout / process-group kill / truncation.

- [ ] **Step 1: Write failing tests**

```go
// internal/permissions/yolo_test.go
package permissions

import (
	"path/filepath"
	"testing"
)

func TestUnjailedAndAllowAll(t *testing.T) {
	root := t.TempDir()
	u := NewUnjailed(root)
	if p, err := u.Resolve("/etc/hosts", true); err != nil || p != "/etc/hosts" {
		t.Fatal(p, err)
	}
	if p, _ := u.Resolve("sub/x.go", true); p != filepath.Join(root, "sub/x.go") {
		t.Fatal(p)
	}
	for _, cmd := range []string{"sudo rm -rf /tmp/x", "eval $CMD", "if then fi (", "cat > ../outside"} {
		if need, every, err := (AllowAll{}).Check(cmd); need != nil || every != nil || err != nil {
			t.Errorf("%q: %v %v %v", cmd, need, every, err)
		}
	}
}
```

```go
// internal/agent/yolo_test.go
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/session"
)

func TestYoloTurnsEveryCheckOff(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "out.txt")
	s := newScript(t,
		toolTurn(
			[2]string{"shell", `{"command":"uname"}`},                    // not allowlisted
			[2]string{"shell", `{"command":"eval echo hi"}`},             // refused by analysis
			[2]string{"shell", `{"command":"chown $(id -u) a.txt"}`},     // hard-deny, harmless here
			[2]string{"write", fmt.Sprintf(`{"path":%q,"content":"x"}`, outside)}, // outside the jail
			[2]string{"edit", `{"path":"a.txt","old_string":"a","new_string":"b"}`}, // never read: guard stays on
		),
		textTurn("done"),
		toolTurn([2]string{"shell", `{"command":"uname"}`}),
		textTurn("refused"),
	)
	a, work, evs := startTest(t, s, 40, func(o *StartOptions) { o.Yolo = true })
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("a\n"), 0o644)
	if _, err := a.Run(context.Background(), "go"); err != nil {
		t.Fatal(err)
	}
	var ends []ToolEnd
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok {
			ends = append(ends, te)
		}
	}
	for i, te := range ends[:4] {
		if te.Result.IsError {
			t.Errorf("call %d must run under yolo: %s", i, te.Result.Content)
		}
	}
	if !ends[4].Result.IsError || !strings.Contains(ends[4].Result.Content, "read it first") {
		t.Fatal("read-before-write guard is not a permission; it stays on")
	}
	if b, _ := os.ReadFile(outside); string(b) != "x" {
		t.Fatal("write outside the workdir")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if !entries[0].Session.Yolo {
		t.Fatal("header records yolo")
	}

	a.SetYolo(false)
	*evs = nil
	a.Run(context.Background(), "again")
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok && !te.Result.IsError {
			t.Fatal("checks are back after /yolo off")
		}
	}
	entries, _ = session.ReadFile(a.Session().Path())
	var modes int
	for _, e := range entries {
		if e.Type == session.TypePermissionMode {
			modes++
		}
	}
	if modes != 1 {
		t.Fatal("toggle writes one permission_mode entry")
	}
}
```

Change Task 16's `startTest` signature to `startTest(t *testing.T, s *scriptServer, maxSteps int, mods ...func(*StartOptions))` and apply the `mods` to the `StartOptions` before `Start`. Existing calls compile unchanged.

- [ ] **Step 2: Run** — `go test ./internal/permissions/ ./internal/agent/ -run 'Unjailed|Yolo'` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/permissions/yolo.go
package permissions

import (
	"os"
	"path/filepath"
	"strings"
)

// Unjailed is the yolo-mode PathChecker (§7.5): it resolves paths like the
// jail does but never refuses.
type Unjailed struct{ root string }

func NewUnjailed(root string) *Unjailed { return &Unjailed{root: root} }

func (u *Unjailed) Resolve(path string, _ bool) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(u.root, path)
	}
	return filepath.Clean(path), nil
}

// AllowAll is the yolo-mode CommandChecker: no analysis, no hard-deny.
type AllowAll struct{}

func (AllowAll) Check(string) ([]string, []string, error) { return nil, nil, nil }
func (AllowAll) Allow(string)                             {}
```

```go
// internal/tools/tool.go — add:

// AutoAllow is the yolo-mode Asker: every approval is granted once.
func AutoAllow(context.Context, Question) Answer { return AllowOnce }
```

```go
// internal/agent/yolo.go
package agent

import (
	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

type YoloChanged struct{ On bool }

func (YoloChanged) isEvent() {}

func (a *Agent) Yolo() bool { return a.yolo }

func (a *Agent) applyYolo(on bool) {
	env := a.opts.Env
	if on {
		env.Paths, env.Commands, env.Ask = permissions.NewUnjailed(env.Root), permissions.AllowAll{}, tools.AutoAllow
	} else {
		env.Paths, env.Commands, env.Ask = a.strict.paths, a.strict.cmds, a.strict.ask
	}
	a.yolo = on
}

// SetYolo toggles yolo mode between runs (never during Run).
func (a *Agent) SetYolo(on bool) {
	a.applyYolo(on)
	a.append(session.Entry{Type: session.TypePermissionMode, PermissionMode: &session.PermissionMode{Yolo: on}})
	a.emit(YoloChanged{On: on})
}
```

In `agent.go`, add the fields `yolo bool` and `strict struct{ paths tools.PathChecker; cmds tools.CommandChecker; ask tools.Asker }`. In `New`, record `a.strict` from `o.Env` **before** anything else. In `start.go`:
- set `Header.Yolo = o.Yolo`;
- after `New(...)`, call `if o.Yolo { a.applyYolo(true) }`.

- [ ] **Step 4: Run** — `go test ./... -race` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/permissions internal/tools internal/session internal/agent
git commit -m "feat: yolo mode — swap in permissive checkers; audit entry; guards that aren't permissions stay on"
```

---

### Task 18: `-p` on top of the agent (output contract, trust flags, yolo, exit 3)

**Files:**
- Modify: `cmd/moca/oneshot.go` (rewrite), `cmd/moca/main.go`
- Test: `cmd/moca/cli_test.go` (extend)

**Interfaces:**
- Consumes: `agent.Start`, `agent.Event` types, `Options.Approve`.
- Produces: the §12.5 contract. stdout = `Outcome.Text + "\n"` only, printed at the end; there is no live streaming to stdout from this phase on. stderr gets:
  - text from turns that called tools (indented, as it streams);
  - `▸ <tool> <Summary>` per `ToolEnd`, or `✗ <tool> <first line of error>` when the result is an error;
  - `retry n/max · wait` per `Retry`;
  - the exit summary `tokens <in>/<out> · $<cost>`.
- Exit codes: `0` done, `3` max steps (wrap-up text still on stdout), `130` cancelled, `1` provider/runtime error, `2` config/usage error.
- Trust: `Approve` explicit → it; else yolo → trusted; else **no**.
- Yolo: `o.YoloOn(cfg)` → `StartOptions.Yolo`; when on, the first stderr line is `yolo mode: all permission checks are off`.
- Slug: `session.Slug(prompt)`.

- [ ] **Step 1: Extend tests**

Append to `cmd/moca/cli_test.go`:

```go
func TestOneShotToolLoopContract(t *testing.T) {
	turns := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"checking\",\"tool_calls\":[{\"index\":0,\"id\":\"c\",\"function\":{\"name\":\"ls\",\"arguments\":\"{}\"}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"All good.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n",
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, turns[min(n, len(turns)-1)])
		n++
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	code := run(context.Background(), []string{"--config", cfg, "-p", "check"}, nil, &out, &errb)
	if code != 0 || out.String() != "All good.\n" {
		t.Fatalf("code %d stdout %q", code, out.String())
	}
	if !strings.Contains(errb.String(), "▸ ls") || !strings.Contains(errb.String(), "checking") || !strings.Contains(errb.String(), "tokens ") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestOneShotMaxStepsExit3(t *testing.T) {
	tool := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c\",\"function\":{\"name\":\"ls\",\"arguments\":\"{}\"}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n"
	final := "data: {\"choices\":[{\"delta\":{\"content\":\"stopped\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if n < 1 {
			io.WriteString(w, tool)
		} else {
			io.WriteString(w, final)
		}
		n++
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := writeCfg(t, `{"model":"loc/m","context":{"maxSteps":1},"providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 3 || out.String() != "stopped\n" {
		t.Fatalf("code %d out %q", code, out.String())
	}
}

func TestNoApproveSkipsProjectInstructions(t *testing.T) {
	var sawRules bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sawRules = strings.Contains(string(b), "PROJECT-RULE-XYZ")
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte("PROJECT-RULE-XYZ"), 0o644)
	t.Chdir(work)
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb)
	if sawRules {
		t.Fatal("default -p must not load project AGENTS.md")
	}
	run(context.Background(), []string{"--config", cfg, "--approve", "-p", "x"}, nil, &out, &errb)
	if !sawRules {
		t.Fatal("--approve loads it")
	}
	sawRules = false
	errb.Reset()
	run(context.Background(), []string{"--config", cfg, "--yolo", "-p", "x"}, nil, &out, &errb)
	if !sawRules || !strings.HasPrefix(errb.String(), "yolo mode: all permission checks are off") {
		t.Fatal("yolo trusts project resources and warns on stderr")
	}
	sawRules = true
	run(context.Background(), []string{"--config", cfg, "--yolo", "--no-approve", "-p", "x"}, nil, &out, &errb)
	if sawRules {
		t.Fatal("explicit --no-approve wins over yolo")
	}
}
```

Delete the phase-1 `TestOneShotStreamsToStdout` stderr assertion on `tokens 3/2` if the fake server no longer sends usage. Keep the stdout assertion (`hello\n`).

- [ ] **Step 2: Run** — `go test ./cmd/moca/` → FAIL.

- [ ] **Step 3: Rewrite `oneshot.go`**

```go
// cmd/moca/oneshot.go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/session"
)

func exitFor(ctx context.Context, err error) int {
	var ee *config.EnvError
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		return exitInterrupted
	case errors.As(err, &ee):
		return exitUsage
	default:
		return exitRuntime
	}
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return s
}

func runOneShot(ctx context.Context, o Options, cfg config.Config, stdout, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	var turnText strings.Builder
	emit := func(e agent.Event) {
		switch e := e.(type) {
		case agent.TextDelta:
			turnText.WriteString(e.Text)
		case agent.StreamReset:
			turnText.Reset()
		case agent.TurnEnd:
			if e.Stop == "tool_use" && turnText.Len() > 0 {
				fmt.Fprintln(stderr, "  "+strings.ReplaceAll(strings.TrimSpace(turnText.String()), "\n", "\n  "))
			}
			turnText.Reset()
		case agent.ToolEnd:
			if e.Result.IsError {
				fmt.Fprintf(stderr, "✗ %s %s\n", e.Call.Name, firstLine(e.Result.Content))
			} else {
				fmt.Fprintf(stderr, "▸ %s %s\n", e.Call.Name, e.Result.Summary)
			}
		case agent.Retry:
			fmt.Fprintf(stderr, "retry %d/%d · %s\n", e.Notice.Attempt, e.Notice.Max, e.Notice.Wait.Round(1e8))
		}
	}
	yolo := o.YoloOn(cfg)
	trusted := yolo
	if o.Approve != nil {
		trusted = *o.Approve
	}
	if yolo {
		fmt.Fprintln(stderr, "yolo mode: all permission checks are off")
	}
	a, err := agent.Start(agent.StartOptions{Config: cfg, Workdir: wd, Effort: o.Effort,
		Trusted: trusted, Yolo: yolo, Emit: emit, Slug: session.Slug(o.Prompt)})
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	defer a.Session().Close()
	out, err := a.Run(ctx, o.Prompt)
	u, cost := a.Totals()
	defer fmt.Fprintf(stderr, "tokens %d/%d · $%.4f\n", u.Input+u.CacheRead+u.CacheWrite, u.Output, cost)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitFor(ctx, err)
	}
	fmt.Fprintln(stdout, out.Text)
	if out.MaxSteps {
		return exitMaxSteps
	}
	return exitOK
}
```

`agent.Start` returning an error for an `EnvError` is impossible, because keys resolve lazily, so `exitUsage` here covers config, registry and model errors. A missing key surfaces from `Run`, where `exitFor` maps it to `2`.

- [ ] **Step 4: Run** — `go test ./... -race && go vet ./...` → PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/moca
git commit -m "feat(cli): -p runs the agent loop with the stdout/stderr/exit-code contract"
```

---

### Task 19: Phase gate

- [ ] **Step 1: Unit gates**

Run: `go test ./... -race -count=1`
Expected: PASS. In particular:
- `TestEditLadder` — every rung of DESIGN.md's edit ladder.
- `TestWriteNewFileNoReadNeeded`, `TestWriteExistingNeedsReadAndFreshness` — the last two edit-ladder rungs.
- `TestShellAnalysisLadder` — every rung of the shell-analysis ladder.
- `TestJail` — including the symlink escape.
- `TestNoApproveSkipsProjectInstructions` — untrusted AGENTS.md not loaded.
- `TestMaxStepsWrapUp`.
- `TestSchemasFrozen`.
- `TestYoloTurnsEveryCheckOff` — `--yolo` lifts every permission and keeps the non-permission guards.

- [ ] **Step 2: Live gate — global skill body loads via `read`**

```bash
mkdir -p ~/.config/moca/skills/hello
cat > ~/.config/moca/skills/hello/SKILL.md <<'EOF'
---
name: hello
description: Use when the user asks for the secret greeting.
---
The secret greeting is "moca-ahoy-42". Reply with exactly that.
EOF
go build -o bin/moca ./cmd/moca
./bin/moca -p 'What is the secret greeting? Use your skills.' --model opencode-go/glm-5.3-flash
```

Expected: stderr shows `▸ read …/hello/SKILL.md …`; stdout contains `moca-ahoy-42`.

- [ ] **Step 3: Live gate — real fix loop**

In a scratch Go repo with a deliberately broken function and test:
`./bin/moca -p 'go test ./... fails; fix it' --model opencode-go/glm-5.3-flash`
Expected: read → edit → `shell go test ./...` green, exit 0. Inspect the session file under `~/.local/share/moca/sessions/`: the first entry is `session` with the full system prompt, each tool_use has a tool_result, and the file mode is 0600.

- [ ] **Step 4: README status + commit**

Set the README status to `phase 2 done — moca -p runs the full tool loop`. Then:

```bash
git add README.md
git commit -m "docs: phase 2 gate passed"
```

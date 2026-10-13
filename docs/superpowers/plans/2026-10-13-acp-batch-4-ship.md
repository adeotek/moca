---
status: ready
batch: 4 of 4
tasks: 13-16
---

# `moca acp` — Batch 4 of 4: Ship — `moca acp` subcommand, end-to-end gate, documentation, manual gates

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this batch task by task in this session (the recommended mode on a usage-limited plan). superpowers:subagent-driven-development also works, at a higher cost. Steps use checkbox (`- [ ]`) syntax for tracking.

**This batch:** Tasks 13–16 of the ACP server plan — the `moca acp` subcommand with the per-workdir trust resolver, the hermetic end-to-end gate over the built binary, the SPECS/README/CHANGELOG/AGENTS docs (v0.5.0-beta), and the manual Zed + ACP TCK gates, ending with the PR.

**Whole feature:** `moca acp` serves the Agent Client Protocol (v1) on stdio so Zed, VS Code ACP extensions and Paseo can drive moca. It's a stdlib-only package `internal/acp` (bidirectional ndjson JSON-RPC 2.0 + ACP types + a server mapping each ACP session onto one `*agent.Agent`), wired through `moca acp` in `cmd/moca`.

**Tech Stack:** Go 1.27, stdlib only (`encoding/json`, `bufio`, `sync`, `context`); existing packages `agent`, `tools`, `skills`, `session`, `llm`, `config`, `provider`.

## Read only what this batch needs

This file is the **complete** instruction set for Tasks 13–16. The design decisions are already made and their code is written out below. To keep context and usage small:

1. **Do not open** the other plan files: `docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md`, `docs/superpowers/plans/2026-10-13-acp-batch-2-server.md`, `docs/superpowers/plans/2026-10-13-acp-batch-3-protocol-completeness.md`, `docs/superpowers/plans/2026-10-13-acp-merged.md`, `docs/superpowers/plans/2026-10-13-acp.md`, `docs/plans/2026-10-13-acp-protocol.md`, or the spec `docs/superpowers/specs/2026-10-13-acp-design.md`. They are drafts or other batches; everything you need is here.
- **Exception:** Task 15 may read `docs/superpowers/specs/2026-10-13-acp-design.md` (its tables, as that step says) and the specific `docs/specs/SPECS.md` sections it edits. Find those sections with `grep -n '^## ' docs/specs/SPECS.md`; never read the 190 KB file whole.
2. **Code from earlier batches is in the repo, not in other plans.** When a task's **Interfaces → Consumes** names a function or type, check it in the code (LSP `hover` / `documentSymbol` / `goToDefinition`, or `grep -n` for the symbol). Read whole files only when the task modifies them.
3. **Orientation:** if you need it, use `graphify query "<question>"` (a small scoped subgraph). Don't read `graphify-out/GRAPH_REPORT.md` or browse source trees.
4. **Tests:** run the exact per-step commands (`-run '…'`). Run the full suite once, at the end of the batch.
5. **Scope:** implement only Tasks 13–16. If a step seems to need something from a later task, it doesn't: do what the step says and note the question for the user.

## Before you start

Batch 3 must be complete. Check it before anything else; this command only reads files and runs tests:

```bash
git switch phase/8-acp
grep -q '"\$/cancel_request"' internal/acp/rpc.go && grep -q '"session/delete"' internal/acp/server.go \
  && grep -q 'compaction_summary_chunk' internal/acp/protocol.go && go test ./internal/acp -race -count=1
```

If it fails, **stop** and report which check failed. Do not repair earlier batches from memory, and do not open the batch 3 plan to finish it yourself unless the user asks.

## Global Constraints


- No new module dependencies: `go.mod` must not change.
- Import direction: `acp` may import `agent`, `tools`, `skills`, `session`, `llm`, `config`, `provider`. It must never import `tui`, and `tui` must never import `acp`. `internal/applog` is imported only by `cmd/moca`; `acp` logs through `log/slog`.
- **stdout in `moca acp` carries JSON-RPC only.** Nothing may print to stdout except `Conn`.
- Logging is metadata only (SPECS §13.5): methods, session id8, durations, stop reasons, counts. Never log prompt text, file contents, command text or keys.
- ACP protocol version: `1`. Session ids are moca session id8s (8 hex chars).
- Tool output text in tool-call content is capped at **8 KB** (`outputCap = 8 << 10`). `shell` keeps the tail and the other tools keep the head. `read` never sends the file body.
- `agent.SetMode`, `SetPlan` and `SetModel` are between-runs only. ACP `set_mode` and `set_config_option` are refused while a prompt runs.
- The eight tool schemas are frozen: do not touch `internal/tools/testdata/schemas.golden.json`.
- CI gate must stay green: `gofmt -l .` empty, `go vet ./...`, `go build ./...`, `go test ./... -race`. Run `make vet` and `make test` (with `export PATH="$(mise where go)/bin:$PATH"` when `go` is not on PATH).
- Branch `phase/8-acp`, created from `feat/acp` (the branch that carries these plan files; created from `main` it would not contain them). Batch 1 creates it; later batches check it out. Use conventional commits. Each commit message ends with a `Co-Authored-By:` trailer naming the model actually doing the work, e.g. `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`. **AGENTS.md: never commit without the user's explicit approval.** Get that approval once at the start of execution; if it isn't given, skip every "Commit" step and leave the changes staged.
- After code changes run `graphify update .` (project CLAUDE.md).
- **Permission requests have no timeout** (settled). `session/cancel` and stdin EOF are the only ways out of a pending request. A timeout would silently deny under clients without auto-accept (a known pitfall in other ACP agents), so do not add one.
- **Project trust per workdir** (merged rule): an explicit `--approve`/`--no-approve` wins; else yolo mode trusts; else a saved `trust.json` decision for that workdir is honoured; else trusted (the editor opening the folder is the trust act). ACP never writes `trust.json`.
- **`usage_update` ships** (settled). It is stable in `schema-v1.25.0`; if a newer pinned tag dropped it, it would be removed and listed as a known limitation (Task 2, Step 2).
- **Client-gated updates.** `notice`, `compaction_update` and `compaction_summary_chunk` are sent **only** when the client advertised `clientCapabilities.session.notices` / `.compaction` at `initialize` (spec MUST). Otherwise moca falls back to a thought line or the summarized-note chunk.
- **ACP v1 only.** The schema is pinned to the newest `schema-v1.x.y` tag under `schema/v1/`; never vendor a `schema-v2.*` alpha or a crate tag (`vX.Y.Z`).
- **Import alias.** Every `internal/acp` file that uses `internal/session` imports it as `msession`: the package declares `type session struct`, and Go rejects the clash.
- Version: this PR is **v0.5.0-beta** (settled; one bump per PR). The binary's version comes from the release ldflags (`internal/config/version.go` defaults to `0.0.0-dev`), so the bump is in docs (SPECS header, README status, CHANGELOG) and the release tag.
- Schema feature gates decided in batch 1 are recorded in `internal/acp/testdata/SOURCE.md` (`## Feature gates`). If a gate there says a feature is absent or renamed, apply that to every step below that uses it.

## Review Focus (items owned by this batch)

- **Anything but JSON-RPC on stdout** (a stray `fmt.Println`, a log line, usage text on a flag error). It corrupts the editor's stream. Pinned by `TestACPServesOnlyJSONRPC` and `TestACPBadFlags` (Task 13) and the non-JSON-RPC line check in the gate (Task 14).
- **A folder the user answered "no" to in the TUI trust prompt.** It must stay untrusted in the editor unless `--approve` is passed, and ACP must never write `trust.json`. Pinned by `TestACPTrust` (Task 13).
- **The API key leaking** into stdout, stderr or the debug log of a real binary run. Pinned by the gate's leak scan, which fails on zero log files (Task 14).
- **Editor quits (stdin EOF).** The binary must exit 0 within seconds with an empty stderr, and the session must reopen from a second process. Pinned by gate legs 4–5 (Task 14).
- **Docs claiming unshipped behaviour.** SPECS/README/CHANGELOG must match the code. If Task 2's feature gates (recorded in `internal/acp/testdata/SOURCE.md`) dropped anything, the docs list it under non-goals (Task 15).

## Files this batch touches

| | Path |
|---|---|
| create | `cmd/moca/acp.go` |
| create | `cmd/moca/acp_test.go` |
| modify | `cmd/moca/main.go` |
| modify | `cmd/moca/cli.go` |
| modify | `cmd/moca/cli_test.go` |
| create | `test/acpgate/acpgate_test.go` |
| create | `test/acpgate/run.sh` |
| modify | `docs/specs/SPECS.md` |
| modify | `README.md` |
| modify | `CHANGELOG.md` |
| modify | `AGENTS.md` |

---

### Task 13: `moca acp` subcommand

**Files:**
- Create: `cmd/moca/acp.go`, `cmd/moca/acp_test.go`
- Modify: `cmd/moca/main.go` (`case "acp":`)
- Modify: `cmd/moca/cli.go` (subcommand whitelist, usage)
- Modify: `cmd/moca/cli_test.go` (`TestHelpListsSurface` wants `"moca acp"`)

**Interfaces:**
- Consumes: `acp.Serve`, `acp.Options` (Task 5), `startLog(cfg, cfgPath, o, stderr) (func(), error)`, `cliModes`, `llm.ParseEffort`, `config.Version`, `config.ResolveMode` (Task 3), `config.DataDir()`, `permissions.LoadTrust(path) (*TrustStore, error)`, `(*TrustStore).Lookup(dir) (trusted, known bool)`, exit codes.
- Produces: `func runACP(ctx context.Context, o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout, stderr io.Writer) int`, `func acpTrust(approve *bool, yolo bool, trustPath string) func(cwd string) (bool, error)`

- [ ] **Step 1: Write the failing tests**

`cmd/moca/acp_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/permissions"
)

const acpCfg = `{"model":"loc/m","providers":{"loc":{"baseUrl":"http://127.0.0.1:1","protocol":"openai-completions",
	"auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":65536}}}}}`

func TestACPServesOnlyJSONRPC(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // the run's log file
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, acpCfg)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "acp"}, in, &out, &errb); code != exitOK {
		t.Fatalf("exit %d stderr %q", code, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout must hold only the initialize response, got %q", out.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil || m["jsonrpc"] != "2.0" || m["result"] == nil {
		t.Fatalf("not a JSON-RPC response: %q", lines[0])
	}
}

// The resolver is decideTrust without the prompt: flag > yolo > saved
// decision > trusted. A saved "no" from the TUI must survive in an editor.
func TestACPTrust(t *testing.T) {
	dir := t.TempDir()
	trustPath := filepath.Join(dir, "trust.json")
	yes, no, fresh := filepath.Join(dir, "yes"), filepath.Join(dir, "no"), filepath.Join(dir, "new")
	for _, d := range []string{yes, no, fresh} {
		os.MkdirAll(d, 0o755) // trust keys are canonical (symlink-resolved) paths
	}
	ts, err := permissions.LoadTrust(trustPath)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Set(yes, true) != nil || ts.Set(no, false) != nil {
		t.Fatal("seeding trust.json failed")
	}
	before, _ := os.ReadFile(trustPath)
	tr, fa := true, false
	cases := []struct {
		name    string
		approve *bool
		yolo    bool
		cwd     string
		want    bool
	}{
		{"saved no", nil, false, no, false},
		{"saved yes", nil, false, yes, true},
		{"unknown dir", nil, false, fresh, true},
		{"--approve beats saved no", &tr, false, no, true},
		{"--no-approve beats everything", &fa, true, yes, false},
		{"yolo trusts", nil, true, no, true},
	}
	for _, c := range cases {
		got, err := acpTrust(c.approve, c.yolo, trustPath)(c.cwd)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v err %v, want %v", c.name, got, err, c.want)
		}
	}
	if after, _ := os.ReadFile(trustPath); string(after) != string(before) {
		t.Error("the resolver must never write trust.json")
	}
}

func TestACPBadFlags(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := writeCfg(t, acpCfg)
	for _, args := range [][]string{{"acp", "--bogus"}, {"acp", "--mode", "turbo"}, {"acp", "--effort", "ultra"}, {"acp", "extra"},
		{"acp", "--approve", "--no-approve"}} {
		var out, errb bytes.Buffer
		code := run(context.Background(), append([]string{"--config", cfg}, args...), strings.NewReader(""), &out, &errb)
		if code != exitUsage || out.Len() != 0 {
			t.Errorf("%v: exit %d stdout %q", args, code, out.String())
		}
	}
}
```

In `cmd/moca/cli_test.go` `TestHelpListsSurface`, add `"moca acp"` to the `want` list.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./cmd/moca -run 'ACP|Help' -race -count=1`
Expected: FAIL (`unknown command "acp"` → exit 2; usage missing `moca acp`; `undefined: acpTrust`)

- [ ] **Step 3: Write `cmd/moca/acp.go`**

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/acp"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/permissions"
)

const acpUsage = "usage: moca acp [--model <provider/model>] [--effort <level>] [--mode default|auto|yolo|plan] [--approve | --no-approve]"

// runACP serves the Agent Client Protocol on stdin/stdout (`moca acp`):
// editors launch it as a child process. stdout carries JSON-RPC only;
// diagnostics go to the log file. Trust is decided per workdir by acpTrust.
func runACP(ctx context.Context, o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("moca acp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	model := fs.String("model", o.Model, "provider/model")
	effort := fs.String("effort", o.Effort, "effort level")
	mode := fs.String("mode", "", "permission mode")
	approve := fs.Bool("approve", false, "trust project resources")
	noApprove := fs.Bool("no-approve", false, "do not trust project resources")
	if err := fs.Parse(o.Sub[1:]); err != nil || fs.NArg() > 0 {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stdout, acpUsage)
			return exitOK
		}
		fmt.Fprintln(stderr, "moca:", acpUsage)
		return exitUsage
	}
	if *mode == "" && o.Mode != nil {
		*mode = *o.Mode
	}
	if *mode != "" && !slices.Contains(cliModes, *mode) {
		fmt.Fprintf(stderr, "moca: --mode must be one of %s, got %q\n", strings.Join(cliModes, ", "), *mode)
		return exitUsage
	}
	if *effort != "" {
		if _, err := llm.ParseEffort(*effort); err != nil {
			fmt.Fprintln(stderr, "moca:", err)
			return exitUsage
		}
	}
	if *approve && *noApprove {
		fmt.Fprintln(stderr, "moca: --approve and --no-approve are mutually exclusive")
		return exitUsage
	}
	explicit := o.Approve // global flags before `acp`
	if *approve || *noApprove {
		v := *approve
		explicit = &v
	}
	// yolo trusts (§7.5); decided without the project layer, as the TUI does.
	m, _ := config.ResolveMode(*mode, cfg)
	trust := acpTrust(explicit, m == "yolo", filepath.Join(config.DataDir(), "trust.json"))
	closeLog, err := startLog(cfg, cfgPath, o, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	defer closeLog()
	err = acp.Serve(ctx, stdin, stdout, acp.Options{Config: cfg, ConfigPath: cfgPath, Model: *model, Effort: *effort,
		Mode: *mode, Trust: trust, Version: config.Version})
	if err != nil && ctx.Err() == nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	return exitOK
}

// acpTrust resolves project trust per workdir: decideTrust (tui.go) without
// its interactive prompt — an ACP server has no terminal to ask on. An
// explicit flag wins; else yolo trusts; else a saved decision is honoured
// (a "no" given in the TUI stays a no); else trusted, since the editor
// opening the folder is the trust act. Nothing is written to trust.json.
func acpTrust(approve *bool, yolo bool, trustPath string) func(cwd string) (bool, error) {
	return func(cwd string) (bool, error) {
		if approve != nil {
			return *approve, nil
		}
		if yolo {
			return true, nil
		}
		ts, err := permissions.LoadTrust(trustPath)
		if err != nil {
			return false, err
		}
		if v, known := ts.Lookup(cwd); known {
			return v, nil
		}
		return true, nil
	}
}
```

- [ ] **Step 4: Wire the subcommand**

In `cmd/moca/cli.go` `parseArgs`, extend the whitelist condition to `o.Sub[0] == "login" || o.Sub[0] == "logout" || o.Sub[0] == "mcp" || o.Sub[0] == "update" || o.Sub[0] == "acp"`.

In `printUsage`, add after the `moca mcp index` line:

```
  moca acp                     serve the Agent Client Protocol on stdio (Zed, VS Code, Paseo, …)
    --model / --effort / --mode   as above, for every session the editor opens
    --approve / --no-approve   trust / do not trust project resources (default: the
                               saved decision for the folder, else trusted)
```

In `cmd/moca/main.go`, add to the subcommand `switch o.Sub[0]`:

```go
		case "acp":
			return runACP(ctx, o, cfg, path, stdin, stdout, stderr)
```

- [ ] **Step 5: Run the tests and the CI gate**

Run: `go test ./cmd/moca ./internal/acp -race -count=1 && make vet && make test`
Expected: PASS; `make vet` clean

- [ ] **Step 6: Smoke-test the binary by hand**

```bash
make build
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}' \
  "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"session/new\",\"params\":{\"cwd\":\"$PWD\",\"mcpServers\":[]}}" | bin/moca acp
```

Expected: two JSON lines (the initialize result; a session/new result with `sessionId`, `modes`, `configOptions`), followed by an `available_commands_update` notification and nothing else on stdout. This needs a configured model in `~/.config/moca/config.jsonc`; without one, the second line is a `-32000` error naming `moca login`.

- [ ] **Step 7: Commit**

```bash
git add cmd/moca
git commit -m "feat: moca acp subcommand"
graphify update .
```

---

### Task 14: End-to-end gate over the built binary

The `internal/acp` tests drive `Serve` in-process. This gate drives the **built binary** the way an editor does: a child process, pipes, flags and real stdout. It catches what in-process tests can't — stray stdout writes from `cmd/moca`, log setup, exit codes, and a second process reopening a session. It follows the house `test/shipgate` pattern (a bash wrapper plus a Go checker behind a build tag), but is fully hermetic: the provider is an in-test `httptest` server, so no API key or network is needed.

**Files:**
- Create: `test/acpgate/acpgate_test.go` (`//go:build acpgate`), `test/acpgate/run.sh`

**Interfaces:**
- Consumes: the built `bin/moca`; the ACP wire only. The gate deliberately builds raw JSON and does not import `internal/acp`, so it checks the wire, not our own types.
- Produces: `bash test/acpgate/run.sh` → `ok`.

- [ ] **Step 1: Write the gate**

`test/acpgate/acpgate_test.go`:

```go
//go:build acpgate

// Package acpgate drives the built `moca acp` binary as an editor would:
// a child process on pipes, a scripted openai-completions provider, two
// processes sharing one data dir. Run through test/acpgate/run.sh.
package acpgate

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var mocaBin = flag.String("moca", "", "path to the built moca binary")

const secret = "acpgate-not-a-real-key-7f3a"

// provider serves scripted SSE turns; a turn equal to "BLOCK" waits until
// the request is cancelled (the cancel leg).
func provider(t *testing.T, turns ...string) *httptest.Server {
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		mu.Lock()
		turn := "data: {\"choices\":[{\"delta\":{\"content\":\"(exhausted)\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
		if len(turns) > 0 {
			turn, turns = turns[0], turns[1:]
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if turn == "BLOCK" {
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		io.WriteString(w, turn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func text(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

func shell(cmd string) string {
	args, _ := json.Marshal(fmt.Sprintf(`{"command":%q}`, cmd))
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_0\",\"function\":{\"name\":\"shell\",\"arguments\":%s}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", args)
}

// proc is one `moca acp` child with a line reader over its stdout.
type proc struct {
	t      *testing.T
	cmd    *exec.Cmd
	in     io.WriteCloser
	lines  chan map[string]any
	stderr strings.Builder
	raw    strings.Builder // every stdout byte, for the leak scan (read after readDone)
	wmu    sync.Mutex      // send runs from the test and from the cancel leg's goroutine
	next   int
	readDone chan struct{} // closed when stdout hit EOF
}

func start(t *testing.T, env []string) *proc {
	p := &proc{t: t, lines: make(chan map[string]any, 1024), readDone: make(chan struct{})}
	p.cmd = exec.Command(*mocaBin, "acp")
	p.cmd.Env = env
	p.cmd.Stderr = &p.stderr
	p.in, _ = p.cmd.StdinPipe()
	out, _ := p.cmd.StdoutPipe()
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			p.raw.WriteString(sc.Text() + "\n")
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil || m["jsonrpc"] != "2.0" {
				t.Errorf("stdout carried a non-JSON-RPC line: %q", sc.Text())
				continue
			}
			p.lines <- m
		}
		close(p.lines)
		close(p.readDone)
	}()
	return p
}

// exit closes stdin and waits for the process: stdout must reach EOF first
// (os/exec: Wait must not run before the pipe reads complete).
func (p *proc) exit() error {
	p.t.Helper()
	p.in.Close()
	select {
	case <-p.readDone:
	case <-time.After(10 * time.Second):
		p.t.Fatal("moca acp did not close stdout after stdin EOF")
	}
	return p.cmd.Wait()
}

func (p *proc) send(v map[string]any) {
	v["jsonrpc"] = "2.0"
	b, _ := json.Marshal(v)
	p.wmu.Lock()
	defer p.wmu.Unlock()
	p.in.Write(append(b, '\n'))
}

// call sends a request and collects everything up to its response.
// onPerm answers session/request_permission requests met on the way.
func (p *proc) call(method string, params any, onPerm func(map[string]any) map[string]any) (resp map[string]any, updates []map[string]any) {
	p.t.Helper()
	p.next++
	id := float64(p.next)
	p.send(map[string]any{"id": id, "method": method, "params": params})
	timeout := time.After(30 * time.Second)
	for {
		select {
		case m, ok := <-p.lines:
			if !ok {
				p.t.Fatalf("%s: stdout closed; stderr %q", method, p.stderr.String())
			}
			switch {
			case m["method"] == "session/update":
				updates = append(updates, m["params"].(map[string]any)["update"].(map[string]any))
			case m["method"] == "session/request_permission":
				p.send(map[string]any{"id": m["id"], "result": onPerm(m["params"].(map[string]any))})
			case m["id"] == id:
				return m, updates
			}
		case <-timeout:
			p.t.Fatalf("%s: no response in 30s", method)
		}
	}
}

func kinds(ups []map[string]any) []string {
	var out []string
	for _, u := range ups {
		out = append(out, u["sessionUpdate"].(string))
	}
	return out
}

func TestACPGate(t *testing.T) {
	if *mocaBin == "" {
		t.Fatal("usage: go test -tags acpgate ./test/acpgate -args -moca <binary>")
	}
	work, data, cfgDir, state := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(work, "victim.txt"), []byte("x"), 0o644)
	llm := provider(t, text("Hello from the gate."), shell("rm victim.txt"), text("Deleted."), "BLOCK")
	os.MkdirAll(filepath.Join(cfgDir, "moca"), 0o700)
	os.WriteFile(filepath.Join(cfgDir, "moca", "config.jsonc"), []byte(fmt.Sprintf(
		`{"model":"fake/m","providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:ACPGATE_KEY","models":{"m":{"contextWindow":65536}}}}}`,
		llm.URL)), 0o600)
	env := append(os.Environ(), "XDG_DATA_HOME="+data, "XDG_CONFIG_HOME="+cfgDir, "XDG_STATE_HOME="+state, "ACPGATE_KEY="+secret,
		"MOCA_LOG=debug") // the most verbose log is the strictest leak scan
	allow := func(map[string]any) map[string]any {
		return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow-once"}}
	}

	p := start(t, env)
	if r, _ := p.call("initialize", map[string]any{"protocolVersion": 1}, allow); r["result"].(map[string]any)["agentInfo"].(map[string]any)["name"] != "moca" {
		t.Fatalf("initialize: %v", r)
	}
	r, _ := p.call("session/new", map[string]any{"cwd": work, "mcpServers": []any{}}, allow)
	sid := r["result"].(map[string]any)["sessionId"].(string)

	// 1. A text turn streams and ends the turn.
	prompt := func(s string) map[string]any { return map[string]any{"sessionId": sid, "prompt": []any{map[string]any{"type": "text", "text": s}}} }
	r, ups := p.call("session/prompt", prompt("say hello"), allow)
	if r["result"].(map[string]any)["stopReason"] != "end_turn" || !strings.Contains(fmt.Sprint(ups), "Hello from the gate.") {
		t.Fatalf("text turn: %v %v", r, kinds(ups))
	}

	// 2. A shell call asks; allow-once runs it.
	perms := 0
	r, ups = p.call("session/prompt", prompt("delete victim.txt"), func(m map[string]any) map[string]any { perms++; return allow(m) })
	if perms != 1 || r["result"].(map[string]any)["stopReason"] != "end_turn" {
		t.Fatalf("approval turn: perms %d resp %v updates %v", perms, r, kinds(ups))
	}
	if _, err := os.Stat(filepath.Join(work, "victim.txt")); !os.IsNotExist(err) {
		t.Fatal("approved rm did not run")
	}

	// 3. Cancel mid-turn.
	done := make(chan map[string]any, 1)
	go func() { r, _ := p.call("session/prompt", prompt("take forever"), allow); done <- r }()
	time.Sleep(500 * time.Millisecond)
	p.send(map[string]any{"method": "session/cancel", "params": map[string]any{"sessionId": sid}})
	select {
	case r := <-done:
		if r["result"].(map[string]any)["stopReason"] != "cancelled" {
			t.Fatalf("cancel: %v", r)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cancel did not end the turn")
	}

	// 4. stdin EOF → exit 0, quickly, with a silent stderr.
	if err := p.exit(); err != nil {
		t.Fatalf("exit: %v; stderr %q", err, p.stderr.String())
	}
	if p.stderr.Len() != 0 {
		t.Fatalf("stderr must stay empty: %q", p.stderr.String())
	}

	// 5. A second process reopens the session; replay comes before the response.
	q := start(t, env)
	q.call("initialize", map[string]any{"protocolVersion": 1}, allow)
	r, ups = q.call("session/load", map[string]any{"sessionId": sid, "cwd": work, "mcpServers": []any{}}, allow)
	if r["error"] != nil || !strings.Contains(fmt.Sprint(ups), "say hello") || !strings.Contains(fmt.Sprint(ups), "Hello from the gate.") {
		t.Fatalf("load: %v updates %v", r, kinds(ups))
	}
	// 5b. Close, then delete: the session leaves session/list.
	if r, _ := q.call("session/close", map[string]any{"sessionId": sid}, allow); r["error"] != nil {
		t.Fatalf("close: %v", r)
	}
	if r, _ := q.call("session/delete", map[string]any{"sessionId": sid}, allow); r["error"] != nil {
		t.Fatalf("delete: %v", r)
	}
	r, _ = q.call("session/list", map[string]any{"cwd": work}, allow)
	if n := len(r["result"].(map[string]any)["sessions"].([]any)); n != 0 {
		t.Fatalf("deleted session still listed (%d)", n)
	}
	if err := q.exit(); err != nil {
		t.Fatalf("second process exit: %v", err)
	}

	// 6. Leak scan: the key never appears in stdout, stderr or the log.
	logs, _ := filepath.Glob(filepath.Join(state, "moca", "logs", "*"))
	if len(logs) == 0 {
		t.Fatal("no log file to scan")
	}
	captured := p.raw.String() + p.stderr.String() + q.raw.String() + q.stderr.String()
	for _, l := range logs {
		b, _ := os.ReadFile(l)
		captured += string(b)
	}
	if strings.Contains(captured, secret) {
		t.Fatal("the API key leaked into captured output")
	}
}
```

`test/acpgate/run.sh`:

```bash
#!/usr/bin/env bash
# ACP gate — drives the built `moca acp` over stdio against a scripted
# provider (hermetic: no key, no network).
#
#   make build && bash test/acpgate/run.sh
#
# Overrides: MOCA=<binary> to point at a different build.
set -u
GATE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$GATE/../.." && pwd)"
MOCA="${MOCA:-$REPO/bin/moca}"
if ! command -v go >/dev/null 2>&1; then
  gobin="$(mise where go 2>/dev/null || true)"
  [ -n "${gobin:-}" ] && export PATH="$gobin/bin:$PATH"
fi
[ -x "$MOCA" ] || { echo "build moca first: make build (looked for $MOCA)"; exit 2; }
cd "$REPO" && go test -tags acpgate -count=1 -race ./test/acpgate -args -moca "$MOCA"
```

The log glob matches `startLog` (`cmd/moca/log.go`: `config.StateDir()/logs`, and `StateDir` is `$XDG_STATE_HOME/moca`). The zero-files check makes a moved log dir fail loudly instead of passing an empty scan.

- [ ] **Step 2: Run it**

Run: `make build && bash test/acpgate/run.sh`
Expected: `ok  github.com/adeotek/moca/test/acpgate`. If a leg fails, fix the cause in its owning task's package with a regression unit test there (not in the gate), rebuild, rerun.

- [ ] **Step 3: Commit**

```bash
git add test/acpgate/acpgate_test.go test/acpgate/run.sh
git commit -m "test(acp): hermetic end-to-end gate over the built binary"
```

---

### Task 15: Documentation

**Files:**
- Modify: `docs/specs/SPECS.md` (new `## 9.5 ACP server (`moca acp`)`; §1 implemented line; §3 CLI surface; layout block; dependency rule)
- Modify: `README.md` (status line; new "Editors (ACP)" section after Configure)
- Modify: `CHANGELOG.md` (new `## v0.5.0-beta` entry)
- Modify: `AGENTS.md` (layout line: sibling files include `acp`; dependency rule `agent → tui | acp`)

- [ ] **Step 1: SPECS.md**

Add the section `## 9.5 ACP server (\`moca acp\`)` after §9. Its content is the spec's §1–§3, reworded as *implemented* behaviour (present tense):
- the CLI line and flags;
- stdout purity;
- trust default;
- the method list;
- the event table and the tool table (kinds, titles, 8 KB cap, read summary only, diffs);
- permissions (options, `rm` never offers allow-always, cancel → deny, **no timeout** — with the rationale: `session/cancel` is the escape hatch, and a timeout silently denies under clients without auto-accept);
- trust (per workdir: flag > yolo > saved `trust.json` decision > trusted; never written by ACP);
- `usage_update` after each model turn (or, if Task 2 dropped it, list it under non-goals);
- modes (idle only);
- the model config option;
- list/load (id8 only, same workdir, full replay);
- slash commands;
- error codes;
- `session/resume`, `session/close` (cancels, then frees), `session/delete` (closes an idle open session first; refuses a running one or one held by another process);
- `$/cancel_request` in both directions (`-32800`), and `-32002` for unknown sessions;
- client-gated updates: `notice` (Warning, Retry) and `compaction_summary_chunk` + `compaction_update`, only with the client's `session.notices` / `session.compaction` capability, with their fallbacks;
- `session_info_update` (title once, `updatedAt` per prompt), the `effort` config option (models with ≥ 2 levels), `config_option_update` after a prompt when the options changed;
- non-goals: client fs/terminal delegation, client MCP servers, images/audio, `authenticate`/`logout` flows, `additionalDirectories`, terminal tool content, elicitation, `plan` updates, TUI built-ins, and all unstable methods (`session/fork`, `providers/*`, `nes/*`, `document/*`, `mcp/message`).

Copy the tables from `docs/superpowers/specs/2026-10-13-acp-design.md` and adjust anything the implementation changed (e.g. a JSON tag renamed in Task 2). Also:
- §3: add the `moca acp` usage lines exactly as in `printUsage`.
- Layout block (`internal/…` list near line 67): add `internal/acp/      ACP server: hand-rolled JSON-RPC peer, protocol subset, sessions over agent.Agent`.
- §1 "Implemented": append "ACP server (`moca acp`, §9.5)".
- Header: current version **v0.5.0-beta**.
- §2 (build, test, verify): add the gate command `make build && bash test/acpgate/run.sh`.

- [ ] **Step 2: README.md**

Add an `## Editors (ACP)` section after `## Configure`:

````markdown
## Editors (ACP)

`moca acp` speaks the [Agent Client Protocol](https://agentclientprotocol.com) on stdio, so ACP editors can drive moca. It uses your existing config and credentials (`moca login` in a terminal first).

```jsonc
// Zed — settings.json
"agent_servers": { "moca": { "command": "moca", "args": ["acp"] } }
```

VS Code (any ACP client extension) and Paseo take the same command line: `moca acp`. Optional flags apply to every session the editor opens: `--model provider/model`, `--effort <level>`, `--mode default|auto|yolo|plan`, `--approve` / `--no-approve`. By default a folder you already answered in moca's TUI trust prompt keeps that answer; any other folder the editor opens is trusted (project rules, AGENTS.md and project skills/prompts apply).

What you get: streamed replies and thinking, tool calls with diffs, approval prompts, cancel, the permission-mode picker, the model and effort pickers, your prompt templates and skills in the `/` menu, session titles, context-usage and compaction info, warnings as notices (where the editor supports them), and session history: reopen, resume, close and delete past moca sessions of the folder. Not supported: images, editor-provided MCP servers (configure them in moca), and TUI-only commands (`/compact`, `/undo`, …).
````

Update the status line to v0.5.0-beta and mention the ACP server. In the Phases table, add row 8: `| 8 | ACP server (` + "`moca acp`" + `) | Zed gate + ACP TCK + test/acpgate — done |`, and change row 7's "current line" to `v0.5.0-beta`.

- [ ] **Step 3: CHANGELOG.md**

Add at the top:

```markdown
## v0.5.0-beta

### Editors

- **`moca acp` — an Agent Client Protocol server.** Zed, VS Code ACP extensions and agent UIs like Paseo can drive moca: `{"command": "moca", "args": ["acp"]}`. Streamed replies and thinking, tool calls with diffs, approvals (`allow once` / `always` / `deny`), cancel, a permission-mode picker (`default` / `auto` / `yolo` / `plan`), a model picker, prompt templates and skills as `/` commands, session history (`session/list`, `session/load` replaying the whole conversation, `session/resume`, `session/close`, `session/delete`), an effort picker, session titles (`session_info_update`), context/cost via `usage_update`, compaction and warning notices for editors that support them, and request cancellation (`$/cancel_request`). Permission requests never time out (cancel instead). Hand-rolled JSON-RPC, no new dependency; the TUI and `-p` are unchanged. Trust: a folder's saved trust answer is honoured, any other folder is trusted (`--approve` / `--no-approve` override). Editor-supplied MCP servers and images are not supported yet.
```

- [ ] **Step 4: AGENTS.md**

In "Layout & dependency rules":
- Change "Sibling files handle `-p`, the TUI, login, `mcp` and `update` subcommands." to "Sibling files handle `-p`, the TUI, login, `mcp`, `update` and `acp` subcommands."
- Change "→ `agent` (everything) → `tui` (agent + pure pkgs)" to "→ `agent` (everything) → `tui` | `acp` (agent + pure pkgs; never each other)".

- [ ] **Step 5: Verify and commit**

Run: `make vet && make test`
Expected: clean / PASS (the SPECS §4 fixture `internal/config/testdata/example.jsonc` is unaffected because no config key was added)

```bash
git add docs/specs/SPECS.md README.md CHANGELOG.md AGENTS.md
git commit -m "docs: moca acp in SPECS, README, CHANGELOG and AGENTS"
```

---

### Task 16: Manual gates (Zed, ACP TCK) and finish

**Files:**
- Modify: `docs/specs/SPECS.md` §15 (gate evidence)

- [ ] **Step 1: Configure Zed**

`make build`, then in Zed's `settings.json` add `"agent_servers": {"moca-dev": {"command": "<repo>/bin/moca", "args": ["acp"]}}`. Open a scratch git repo containing a small Go or text project, and open the agent panel with "moca-dev".

- [ ] **Step 2: Walk the checklist; note pass/fail for each**

1. A prompt streams a reply; thinking shows (on a thinking model).
2. "Read X and change Y": the read card shows `read <path>`; the edit card shows a diff; the file changes on disk.
3. "Delete tmp.txt with rm": an approval prompt appears with Allow once / Deny (no Always). Deny → the card fails; repeat with Allow once → the file is gone.
4. A long task → press stop: the turn ends as cancelled, and the next prompt works.
5. Mode picker: switch to Plan, ask for a change → only `docs/plans/*.md` is written; switch back to Default.
6. Model picker: switch models; the next reply comes from the new model (check `~/.local/state/moca/logs/`).
7. The `/` menu lists your prompt templates and skills; running one expands it.
8. Restart Zed, open the thread history, reopen the session: the full conversation is replayed, including tool cards, and a follow-up prompt works.
9. `grep -c . ~/.local/state/moca/logs/<today>-*.log` shows ACP entries and no prompt text.
10. The usage display (if Zed shows `usage_update`) moves after each turn.
11. A folder you previously answered "no" to in the TUI trust prompt: project prompts are absent from the `/` menu.
12. The thread title becomes the first line of the first prompt; the effort picker appears for a thinking model and switching it changes the next request's effort (check the log).
13. Delete a thread from history: it disappears, and its file is gone from `~/.local/share/moca/sessions/`.

- [ ] **Step 3: Record and fix**

Fix any failure as its own test-first change in the owning task's package (an ACP client quirk becomes a regression test in `internal/acp`). Record the gate in SPECS §15 in the existing style: date, Zed version, model, the 13 results, and any deviations. Then:

```bash
git add docs/specs/SPECS.md
git commit -m "docs(specs): ACP Zed gate evidence"
```

- [ ] **Step 4: ACP TCK (manual, not CI — settled)**

Clone `agentclientprotocol/acp-tck` outside the repo (it is untrusted third-party code: run it in its own directory, never from inside moca's tree), and point it at `<repo>/bin/moca acp` with a hermetic config (the Task 14 provider pattern, or a real model). Record each verdict. Mandatory-tier checks must pass; a failure on a capability moca does not advertise (fs, terminal, images, client MCP) is N/A. Put the report path and verdicts in the PR description, not in the repo (the TCK is experimental, so there's no CI dependency). Fix real failures test-first in the owning package.

- [ ] **Step 5: Final gates**

```bash
gofmt -l .            # expect no output
make vet && make test
make build && bash test/acpgate/run.sh
graphify update .
```

Expected: all green. If `graphify update` changed `graphify-out/`, commit it (with approval) as `chore: graphify update for acp`.

- [ ] **Step 6: Finish the branch**

Use superpowers:finishing-a-development-branch. `main` is PR-only (pre-push hook), so open a PR from `phase/8-acp`. The PR body carries: the acpgate output, the Zed checklist (13 results), the TCK verdicts, and the Task 2 feature-gate outcome (`usage_update` / `messageId` / unstable defs).

---

## Batch 4 completion

- [ ] Full checks: `gofmt -l .` (expect no output), `make vet`, `make test` (add `export PATH="$(mise where go)/bin:$PATH"` when `go` is not on PATH).
- [ ] Every checkbox above is ticked, or the user agreed to skip it.
- [ ] Commits: if the user approved committing at the start, every task has its commit; otherwise everything is staged and nothing committed.
- [ ] `graphify update .` (project rule after code changes).

**This is the last batch.** Task 16's final step finishes the branch (PR). Report the PR URL and the gate evidence to the user.

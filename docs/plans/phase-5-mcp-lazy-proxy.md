# Phase 5 — MCP Lazy Proxy — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The frozen `mcp` tool becomes a real lazy proxy. Configured MCP servers (stdio and streamable HTTP) are discoverable through a persisted name+description index, so **zero servers start at session start**. They start on first `describe`/`call`, stop after the idle timeout, and calls are gated by MCP annotations or a per-server approve list. `moca mcp import` brings existing Claude Code / OpenCode / Pi servers over without copying literal secrets; `moca mcp index` prebuilds the index.

**Architecture:** New package `internal/mcp`:
- a JSON-RPC 2.0 core with two transports (stdio subprocess, streamable HTTP);
- a `Manager` that owns per-server lifecycle (lazy start, idle stop, restart) and the persisted `Index`;
- the `mcp` proxy tool (implements `tools.Tool`, returns `tools.MCPSpec()` byte-identically);
- the importer.

`agent.build` swaps the stub for the proxy when servers are configured, and `Agent.Close` stops servers. The TUI's allow-always learns the `mcp` kind.

**Tech Stack:** Go 1.27.1 stdlib (`os/exec`, `net/http`, `encoding/json`, `crypto/sha256`). No MCP SDK.

**Spec:** `docs/specs/DESIGN.md` (rev 11) — §4 (`mcp` row), §10.5 (all), §12 (`mcp` config block), §12.5 (`moca mcp import`, `moca mcp index`), §13 (server roster), phase plan item 5.

**Builds on:** Phases 1–4. Uses `tools.Tool/Env/Result/Asker/Question/Answer/MCPSpec/Truncate`, `config.MCPServer/MCPConfig/ResolveEnv/AppendString/DataDir/ConfigFile`, `agent.build/Options`, the TUI approval flow, `cmd/moca` `Options.Sub`.

## Global Constraints

- The `mcp` tool schema is **frozen** (phase 2 golden): `{action: search|describe|call, server, tool, args, query}`. The proxy's `Spec()` must return `tools.MCPSpec()`, and the phase-2 golden test must still pass.
- Server tool lists are **never** injected into the prompt. The prompt has only the roster lines from phase 2 (`- name: description`).
- `search(query, server?)` → ranked `{server, tool, description}` hits (word match).
- `describe(server, tool)` → the full input schema + annotations (starts the server if needed).
- `call(server, tool, args)` → result text truncated at **30K** chars; non-text content → `[<type> omitted]`.
- Lifecycle: start on first `describe`/`call` (or an index miss during `search`); stop after `mcp.idleTimeout` seconds (default 600) idle; a stopped server stays discoverable via the persisted index; restart on demand. **No eager spawning at startup.**
- Index file `~/.local/share/moca/mcp-index.json`: `{server: {configHash, tools:[{name, description, annotations}]}}`. It is refreshed on every server start, and an entry is invalid when that server's config hash changes.
- Transports: stdio + streamable HTTP only. No legacy SSE, sampling, elicitation or MCP OAuth.
- stdio env: only `PATH HOME USER LANG TERM TMPDIR XDG_*` from moca's env + the server's explicit `env` entries (literal or `env:VAR`).
- HTTP `headers`: literal or `env:VAR`.
- Path jail does not apply to MCP tools.
- Yolo mode (§7.5) needs no MCP-specific code: its `Env.Ask` is `tools.AutoAllow`, so every gated call runs, including in `-p`.
- Gating: run without asking iff (`readOnlyHint == true` and `destructiveHint != true`) or the tool is in `mcp.servers.<name>.approve` (or that list contains `"*"`). Otherwise TUI prompt (allow-once / allow-always → appended to `approve`); `-p` refuses. No name-pattern heuristics.
- Import: secrets never copied literally. `env`/`headers` values whose **key** matches `KEY|TOKEN|SECRET|PASSWORD|AUTH` (case-insensitive), or whose **value** looks like a bearer token, become `env:MOCA_MCP_<SERVER>_<KEY>`. The preview lists the variables to export. `${VAR}` references become `env:VAR`.

## Review Focus

1. **A stdio server that writes logs to stdout before/among JSON-RPC lines, or exits during `initialize`** → non-JSON lines are skipped and logged to the server's stderr ring buffer. An early exit returns `server <name> exited: <last stderr lines>` instead of hanging. Tested in Task 2.
2. **Two `call`s to the same stopped server back-to-back (the model emits them in one turn; execution is sequential, but the idle timer may fire between them)** → exactly one process start per need; a stop racing a call can't kill a server mid-call (the idle timer resets on call start and is checked under the server lock). Tested in Task 4.
3. **`search` with `server` set to an unknown name, or `call` with `args` omitted** → an error listing the configured servers; omitted `args` is sent as `{}`. Tested in Task 5.
4. **A server whose `tools/list` paginates (`nextCursor`)** → every page is fetched and indexed. Tested in Task 2.
5. **An HTTP server that answers a POST with an SSE stream carrying a server→client request or notification before the response** → those are ignored, and the matching `id` response is returned. Tested in Task 3.

---

## File Structure

```
internal/mcp/
  jsonrpc.go            message types, id allocation, errors
  stdio.go stdio_test.go         stdio transport (+ TestHelperProcess fake server)
  http.go http_test.go           streamable HTTP transport
  sse.go                         minimal SSE reader (mcp may not import provider)
  client.go                      initialize / tools/list (paged) / tools/call over a transport
  index.go index_test.go         persisted index + config hash + search ranking
  manager.go manager_test.go     lazy lifecycle, idle stop, index refresh
  tool.go tool_test.go           the mcp proxy tool + gating + result rendering
  importer.go importer_test.go   Claude Code / OpenCode / Pi config import
  testdata/                      import fixtures
internal/config/
  edit.go                        (+ SetObjectEntry)
internal/agent/
  start.go                       register proxy when servers exist; Close()
internal/tui/
  app.go                         allow-always for mcp kind; Close on exit
cmd/moca/
  mcp.go                         `moca mcp import [--yes]`, `moca mcp index`
```

---

### Task 1: JSON-RPC core + stdio transport

**Files:**
- Create: `internal/mcp/jsonrpc.go`, `internal/mcp/stdio.go`, `internal/mcp/client.go`
- Modify: `internal/tools/shellrun.go`, `shellrun_unix.go`, `shellrun_windows.go` (export the process-group helpers)
- Test: `internal/mcp/stdio_test.go`

**Interfaces:**
- Produces:

```go
// jsonrpc.go
type request struct { JSONRPC string `json:"jsonrpc"`; ID *int64 `json:"id,omitempty"`; Method string `json:"method"`; Params any `json:"params,omitempty"` }
type response struct { JSONRPC string; ID *json.RawMessage; Method string; Result json.RawMessage; Error *rpcError }
type rpcError struct { Code int; Message string }
func (e *rpcError) Error() string

// transport: one in-flight-capable message pipe.
type transport interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close() error
}

// stdio.go
func startStdio(ctx context.Context, name string, s config.MCPServer, baseEnv []string) (transport, error)
func FilterEnv(base []string) []string // PATH HOME USER LANG TERM TMPDIR XDG_*
func serverEnv(base []string, extra map[string]string) ([]string, error) // resolves env: refs

// client.go
type Annotations struct { Title string `json:"title,omitempty"`; ReadOnlyHint *bool `json:"readOnlyHint,omitempty"`; DestructiveHint *bool `json:"destructiveHint,omitempty"`; IdempotentHint *bool `json:"idempotentHint,omitempty"`; OpenWorldHint *bool `json:"openWorldHint,omitempty"` }
type Tool struct { Name, Description string; InputSchema json.RawMessage `json:"inputSchema"`; Annotations Annotations `json:"annotations"` }
type Content struct { Type, Text, MimeType string; Resource json.RawMessage }
type CallResult struct { Content []Content; IsError bool `json:"isError"` }
type client struct { t transport; name string }
func initialize(ctx context.Context, t transport) error // protocolVersion "2025-06-18", clientInfo {moca, config.Version}; then notifications/initialized
func (c *client) listTools(ctx context.Context) ([]Tool, error) // follows nextCursor
func (c *client) callTool(ctx context.Context, name string, args json.RawMessage) (CallResult, error)
```

- stdio framing: one JSON object per line on stdin/stdout. A reader goroutine:
  - routes responses to pending calls by id;
  - answers server→client requests (`ping` → `{}`; anything else → error `-32601`);
  - ignores notifications;
  - logs non-JSON lines into a 4 KiB stderr ring.
- stderr is drained into the same ring. When the process exits, every pending call fails with `server <name> exited: <ring tail>`.
- Process group kill on `Close` via `tools.SetProcessGroup` / `tools.KillProcessGroup` (exported in this task from phase 2's shell runner).

- [x] **Step 1: Write the fake server + failing tests**

```go
// internal/mcp/stdio_test.go
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

// TestHelperProcess is the fake stdio MCP server, run as a subprocess.
// Behaviour switches: MOCA_FAKE_MODE = "" | "noisy" | "die" | "paged".
func TestHelperProcess(t *testing.T) {
	if os.Getenv("MOCA_FAKE_MCP") != "1" {
		return
	}
	mode := os.Getenv("MOCA_FAKE_MODE")
	if mode == "die" {
		fmt.Fprintln(os.Stderr, "fatal: missing API token")
		os.Exit(3)
	}
	if f := os.Getenv("MOCA_FAKE_STARTS"); f != "" { // count spawns
		fh, _ := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		fh.WriteString("x")
		fh.Close()
	}
	in := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	if mode == "noisy" {
		fmt.Println("server starting up...")
	}
	for in.Scan() {
		var req struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.Unmarshal(in.Bytes(), &req)
		if req.ID == nil {
			continue
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{"name": "fake", "version": "1"}}
			if mode == "noisy" {
				out.Encode(map[string]any{"jsonrpc": "2.0", "id": 999, "method": "ping"}) // server→client request
			}
		case "tools/list":
			var p struct{ Cursor string `json:"cursor"` }
			json.Unmarshal(req.Params, &p)
			ro := true
			page1 := []map[string]any{
				{"name": "read_doc", "description": "Read library documentation", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"lib": map[string]any{"type": "string"}}}, "annotations": map[string]any{"readOnlyHint": ro}},
				{"name": "create_issue", "description": "Create an issue", "inputSchema": map[string]any{"type": "object"}},
			}
			if mode == "paged" && p.Cursor == "" {
				result = map[string]any{"tools": page1[:1], "nextCursor": "p2"}
			} else if mode == "paged" {
				result = map[string]any{"tools": page1[1:]}
			} else {
				result = map[string]any{"tools": page1}
			}
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			json.Unmarshal(req.Params, &p)
			result = map[string]any{"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("%s(%v)", p.Name, p.Arguments)},
				{"type": "image", "data": "AAAA", "mimeType": "image/png"}}}
		default:
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "nope"}})
			continue
		}
		if mode == "noisy" {
			fmt.Println("log: handled", req.Method)
		}
		out.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}
}

func fakeServer(mode string) config.MCPServer {
	return config.MCPServer{Command: os.Args[0], Args: []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{"MOCA_FAKE_MCP": "1", "MOCA_FAKE_MODE": mode}}
}

func dial(t *testing.T, s config.MCPServer) *client {
	t.Helper()
	tr, err := startStdio(context.Background(), "fake", s, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tr.Close() })
	if err := initialize(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
	return &client{t: tr, name: "fake"}
}

func TestStdioListAndCall(t *testing.T) {
	c := dial(t, fakeServer("noisy"))
	tools, err := c.listTools(context.Background())
	if err != nil || len(tools) != 2 || *tools[0].Annotations.ReadOnlyHint != true {
		t.Fatal(tools, err)
	}
	r, err := c.callTool(context.Background(), "read_doc", json.RawMessage(`{"lib":"go"}`))
	if err != nil || r.Content[0].Text != "read_doc(map[lib:go])" || r.Content[1].Type != "image" {
		t.Fatal(r, err)
	}
}

func TestStdioPaged(t *testing.T) {
	tools, err := dial(t, fakeServer("paged")).listTools(context.Background())
	if err != nil || len(tools) != 2 {
		t.Fatal(tools, err)
	}
}

func TestStdioEarlyExit(t *testing.T) {
	tr, err := startStdio(context.Background(), "fake", fakeServer("die"), os.Environ())
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = initialize(ctx, tr)
	}
	if err == nil || !strings.Contains(err.Error(), "missing API token") {
		t.Fatalf("exit surfaces stderr, no hang: %v", err)
	}
}

func TestFilterEnv(t *testing.T) {
	got := FilterEnv([]string{"PATH=/bin", "HOME=/h", "OPENAI_API_KEY=x", "XDG_CONFIG_HOME=/c", "AWS_SECRET=y", "TERM=xterm"})
	if !slices.Equal(got, []string{"PATH=/bin", "HOME=/h", "XDG_CONFIG_HOME=/c", "TERM=xterm"}) {
		t.Fatal(got)
	}
	t.Setenv("MOCA_T_TOK", "sekret")
	env, err := serverEnv([]string{"PATH=/bin"}, map[string]string{"TOKEN": "env:MOCA_T_TOK", "MODE": "x"})
	if err != nil || !slices.Contains(env, "TOKEN=sekret") || !slices.Contains(env, "MODE=x") {
		t.Fatal(env, err)
	}
}
```

The fake server's own env must pass `MOCA_FAKE_*` through. `serverEnv` adds the explicit `Env` map, so the filter doesn't drop them.

- [x] **Step 2: Run** — `go test ./internal/mcp/` → FAIL.

- [x] **Step 3: Implement `jsonrpc.go` + `client.go`**

```go
// internal/mcp/jsonrpc.go

// Package mcp is moca's lazy MCP proxy (§10.5): JSON-RPC over stdio and
// streamable HTTP, a persisted discovery index, lazy server lifecycle, and
// the `mcp` tool. No SDK.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

type request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message) }

type transport interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close() error
}
```

```go
// internal/mcp/client.go
package mcp

import (
	"context"
	"encoding/json"

	"github.com/adeotek/moca/internal/config"
)

const protocolVersion = "2025-06-18"

type Annotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Annotations Annotations     `json:"annotations"`
}

type Content struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	MimeType string          `json:"mimeType,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

type CallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError"`
}

type client struct {
	t    transport
	name string
}

func initialize(ctx context.Context, t transport) error {
	_, err := t.Call(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "moca", "version": config.Version},
	})
	if err != nil {
		return err
	}
	return t.Notify(ctx, "notifications/initialized", nil)
}

func (c *client) listTools(ctx context.Context) ([]Tool, error) {
	var all []Tool
	cursor := ""
	for {
		var params any
		if cursor != "" {
			params = map[string]string{"cursor": cursor}
		}
		raw, err := c.t.Call(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Tools...)
		if page.NextCursor == "" {
			return all, nil
		}
		cursor = page.NextCursor
	}
}

func (c *client) callTool(ctx context.Context, name string, args json.RawMessage) (CallResult, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	raw, err := c.t.Call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return CallResult{}, err
	}
	var r CallResult
	err = json.Unmarshal(raw, &r)
	return r, err
}
```

- [x] **Step 4: Implement `stdio.go` (+ platform files)**

```go
// internal/mcp/stdio.go
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

var keepEnv = []string{"PATH", "HOME", "USER", "LANG", "TERM", "TMPDIR"}

func FilterEnv(base []string) []string {
	var out []string
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "XDG_") || contains(keepEnv, k) {
			out = append(out, kv)
		}
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func serverEnv(base []string, extra map[string]string) ([]string, error) {
	env := FilterEnv(base)
	for k, v := range extra {
		r, err := config.ResolveEnv(v)
		if err != nil {
			return nil, err
		}
		env = append(env, k+"="+r)
	}
	return env, nil
}

type ring struct {
	mu  sync.Mutex
	buf []byte
}

func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > 4096 {
		r.buf = r.buf[len(r.buf)-4096:]
	}
	r.mu.Unlock()
	return len(p), nil
}

func (r *ring) tail() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(r.buf)), "\n")
	return strings.Join(lines[max(0, len(lines)-5):], " | ")
}

type stdioTransport struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	wmu     sync.Mutex
	nextID  atomic.Int64
	mu      sync.Mutex
	pending map[int64]chan response
	dead    error
	logs    *ring
	done    chan struct{}
}

func startStdio(ctx context.Context, name string, s config.MCPServer, baseEnv []string) (transport, error) {
	env, err := serverEnv(baseEnv, s.Env)
	if err != nil {
		return nil, fmt.Errorf("mcp server %s: %w", name, err)
	}
	cmd := exec.Command(s.Command, s.Args...)
	cmd.Env = env
	logs := &ring{}
	cmd.Stderr = logs
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	tools.SetProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp server %s: %w", name, err)
	}
	t := &stdioTransport{name: name, cmd: cmd, stdin: stdin, pending: map[int64]chan response{}, logs: logs, done: make(chan struct{})}
	go t.readLoop(stdout)
	go func() {
		werr := cmd.Wait()
		t.fail(fmt.Errorf("server %s exited (%v): %s", name, werr, logs.tail()))
		close(t.done)
	}()
	return t, nil
}

func (t *stdioTransport) fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dead == nil {
		t.dead = err
	}
	for id, ch := range t.pending {
		ch <- response{Error: &rpcError{Code: -32000, Message: t.dead.Error()}}
		delete(t.pending, id)
	}
}

func (t *stdioTransport) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		var m response
		if json.Unmarshal(line, &m) != nil || m.JSONRPC != "2.0" {
			t.logs.Write(append(append([]byte{}, line...), '\n'))
			continue
		}
		if m.Method != "" { // server→client request or notification
			if m.ID != nil {
				var reply any = map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": rpcError{-32601, "not supported by moca"}}
				if m.Method == "ping" {
					reply = map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{}}
				}
				t.write(reply)
			}
			continue
		}
		if m.ID == nil {
			continue
		}
		var id int64
		if json.Unmarshal(*m.ID, &id) != nil {
			continue
		}
		t.mu.Lock()
		ch := t.pending[id]
		delete(t.pending, id)
		t.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	}
}

func (t *stdioTransport) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.wmu.Lock()
	defer t.wmu.Unlock()
	_, err = t.stdin.Write(append(b, '\n'))
	return err
}

func (t *stdioTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.nextID.Add(1)
	ch := make(chan response, 1)
	t.mu.Lock()
	if t.dead != nil {
		t.mu.Unlock()
		return nil, t.dead
	}
	t.pending[id] = ch
	t.mu.Unlock()
	if err := t.write(request{JSONRPC: "2.0", ID: &id, Method: method, Params: params}); err != nil {
		select {
		case <-t.done:
			return nil, t.dead
		default:
			return nil, err
		}
	}
	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	case r := <-ch:
		if r.Error != nil {
			return nil, r.Error
		}
		return r.Result, nil
	}
}

func (t *stdioTransport) Notify(_ context.Context, method string, params any) error {
	return t.write(request{JSONRPC: "2.0", Method: method, Params: params})
}

func (t *stdioTransport) Close() error {
	t.stdin.Close()
	tools.KillProcessGroup(t.cmd)
	<-t.done
	return nil
}
```

Process-group handling is shared with the shell tool. In `internal/tools/shellrun_unix.go` and `shellrun_windows.go`, rename `setProcessGroup` → `SetProcessGroup` and `killProcessGroup` → `KillProcessGroup` (exported), and update their callers in `shellrun.go`. `mcp` may import `tools` (§2), and this avoids duplicating platform code.

- [x] **Step 5: Run** — `go test ./internal/mcp/ -race -v` → PASS.

- [x] **Step 6: Commit**

```bash
git add internal/mcp internal/tools
git commit -m "feat(mcp): JSON-RPC client and stdio transport with filtered env"
```

---

### Task 2: Streamable HTTP transport

**Files:**
- Create: `internal/mcp/http.go`, `internal/mcp/sse.go`
- Test: `internal/mcp/http_test.go`

**Interfaces:**
- Produces: `func startHTTP(name string, s config.MCPServer, hc *http.Client) (transport, error)`.
  - Each `Call`/`Notify` is a POST to `s.URL` with `Content-Type: application/json`, `Accept: application/json, text/event-stream`, configured headers (resolved via `config.ResolveEnv`), `Mcp-Session-Id` once the server provides it, and `MCP-Protocol-Version` after `initialize`.
  - Response: `application/json` → a single JSON-RPC message; `text/event-stream` → read events until one carries a response with our id (requests/notifications ignored). 202 for notifications.
  - 404 with a session id → the session expired; return the error `session expired` (the manager restarts the server object).
  - `Close` sends `DELETE` with the session id (errors ignored).
  - `sse.go`: `readEvents(r io.Reader, fn func(data string) error) error` — `data:` lines only, CRLF tolerant. Kept separate from `provider`'s reader per §2; no stall timeout (bounded by ctx).

- [x] **Step 1: Write failing tests**

```go
// internal/mcp/http_test.go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

func fakeHTTPServer(t *testing.T, sse bool) (*httptest.Server, *[]http.Header) {
	var hdrs []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(200)
			return
		}
		hdrs = append(hdrs, r.Header.Clone())
		b, _ := io.ReadAll(r.Body)
		var req struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		json.Unmarshal(b, &req)
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any = map[string]any{}
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sess-1")
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{"name": "search_docs", "description": "Search docs", "inputSchema": map[string]any{"type": "object"}}}}
		}
		resp, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		if sse && req.Method == "tools/list" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n")
			fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":77,\"method\":\"sampling/createMessage\"}\n\n")
			fmt.Fprintf(w, "data: %s\n\n", resp)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &hdrs
}

func TestHTTPTransport(t *testing.T) {
	for _, sse := range []bool{false, true} {
		srv, hdrs := fakeHTTPServer(t, sse)
		t.Setenv("MOCA_T_MCP_TOK", "Bearer abc")
		tr, err := startHTTP("ctx7", config.MCPServer{URL: srv.URL, Headers: map[string]string{"Authorization": "env:MOCA_T_MCP_TOK"}}, srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		if err := initialize(context.Background(), tr); err != nil {
			t.Fatal(err)
		}
		c := &client{t: tr, name: "ctx7"}
		tools, err := c.listTools(context.Background())
		if err != nil || len(tools) != 1 || tools[0].Name != "search_docs" {
			t.Fatalf("sse=%v: %v %v", sse, tools, err)
		}
		last := (*hdrs)[len(*hdrs)-1]
		if last.Get("Mcp-Session-Id") != "sess-1" || last.Get("Authorization") != "Bearer abc" || last.Get("MCP-Protocol-Version") != "2025-06-18" {
			t.Fatalf("headers: %v", last)
		}
		tr.Close()
	}
}
```

- [x] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/mcp/sse.go
package mcp

import (
	"bufio"
	"io"
	"strings"
)

// readEvents is a minimal SSE reader for MCP streamable HTTP. mcp cannot
// import provider (§2), and MCP needs no stall timeout (ctx bounds it).
func readEvents(r io.Reader, fn func(data string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 64<<20)
	var data []string
	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		d := strings.Join(data, "\n")
		data = nil
		return fn(d)
	}
	for sc.Scan() {
		line := strings.TrimSuffix(sc.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(v, " "))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	return sc.Err()
}
```

```go
// internal/mcp/http.go
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/adeotek/moca/internal/config"
)

var errFound = errors.New("found")

type httpTransport struct {
	name    string
	url     string
	headers map[string]string
	hc      *http.Client
	nextID  atomic.Int64
	mu      sync.Mutex
	session string
	proto   string
}

func startHTTP(name string, s config.MCPServer, hc *http.Client) (transport, error) {
	h := map[string]string{}
	for k, v := range s.Headers {
		r, err := config.ResolveEnv(v)
		if err != nil {
			return nil, fmt.Errorf("mcp server %s: %w", name, err)
		}
		h[k] = r
	}
	return &httpTransport{name: name, url: s.URL, headers: h, hc: hc}, nil
}

func (t *httpTransport) post(ctx context.Context, body any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	t.mu.Lock()
	if t.session != "" {
		req.Header.Set("Mcp-Session-Id", t.session)
	}
	if t.proto != "" {
		req.Header.Set("MCP-Protocol-Version", t.proto)
	}
	t.mu.Unlock()
	return t.hc.Do(req)
}

func (t *httpTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.nextID.Add(1)
	resp, err := t.post(ctx, request{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("mcp server %s: %w", t.name, err)
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.session = sid
		t.mu.Unlock()
	}
	if resp.StatusCode == http.StatusNotFound && t.session != "" {
		return nil, fmt.Errorf("mcp server %s: session expired", t.name)
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("mcp server %s: HTTP %d: %s", t.name, resp.StatusCode, b)
	}
	var out response
	match := func(data string) error {
		var m response
		if json.Unmarshal([]byte(data), &m) != nil || m.Method != "" || m.ID == nil {
			return nil
		}
		var got int64
		if json.Unmarshal(*m.ID, &got) == nil && got == id {
			out = m
			return errFound
		}
		return nil
	}
	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if ct == "text/event-stream" {
		if err := readEvents(resp.Body, match); err != nil && !errors.Is(err, errFound) {
			return nil, err
		}
	} else {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		match(string(b))
	}
	if out.ID == nil {
		return nil, fmt.Errorf("mcp server %s: no response for %s", t.name, method)
	}
	if out.Error != nil {
		return nil, out.Error
	}
	if method == "initialize" {
		var r struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(out.Result, &r)
		t.mu.Lock()
		t.proto = r.ProtocolVersion
		t.mu.Unlock()
	}
	return out.Result, nil
}

func (t *httpTransport) Notify(ctx context.Context, method string, params any) error {
	resp, err := t.post(ctx, request{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *httpTransport) Close() error {
	t.mu.Lock()
	sid := t.session
	t.mu.Unlock()
	if sid == "" {
		return nil
	}
	req, _ := http.NewRequest(http.MethodDelete, t.url, nil)
	req.Header.Set("Mcp-Session-Id", sid)
	if resp, err := t.hc.Do(req); err == nil {
		resp.Body.Close()
	}
	return nil
}
```

- [x] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/mcp && git commit -m "feat(mcp): streamable HTTP transport with session id and SSE responses"`

---

### Task 3: Persisted index, config hash, search ranking

**Files:**
- Create: `internal/mcp/index.go`
- Test: `internal/mcp/index_test.go`

**Interfaces:**
- Produces:
  - `type IndexTool struct { Name, Description string; Annotations Annotations }`
  - `type IndexEntry struct { ConfigHash string; Tools []IndexTool; UpdatedAt time.Time }`
  - `type Index struct { path string; mu sync.Mutex; Servers map[string]IndexEntry }`
  - `func LoadIndex(path string) (*Index, error)` (missing or corrupt → empty)
  - `func (ix *Index) Save() error` (atomic, 0600)
  - `func (ix *Index) Valid(server, hash string) (IndexEntry, bool)`
  - `func (ix *Index) Put(server, hash string, tools []Tool)`
  - `func ConfigHash(s config.MCPServer) string` — sha256 of the canonical JSON of the fields that affect the tool list (`command,args,env,url,headers`). `description`/`approve` are excluded, so editing them doesn't force a re-index.
  - `type Hit struct { Server, Tool, Description string; score int }`
  - `func Rank(query string, entries map[string]IndexEntry, server string, limit int) []Hit` — lowercase word tokens (split on non-alnum, `_` and `-` included as separators). Score: +3 per token equal to a name token, +2 per token contained in the name, +1 per token contained in the description. Ties broken by server, then tool name. Zero-score hits are dropped. An empty query lists all tools (score 0 allowed) up to `limit`.

- [x] **Step 1: Write failing tests**

```go
// internal/mcp/index_test.go
package mcp

import (
	"path/filepath"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

func TestIndexPersistAndHash(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mcp-index.json")
	ix, _ := LoadIndex(p)
	s := config.MCPServer{Command: "npx", Args: []string{"x"}}
	h := ConfigHash(s)
	ix.Put("fs", h, []Tool{{Name: "read_file", Description: "Read a file"}})
	ix.Save()
	ix2, _ := LoadIndex(p)
	if _, ok := ix2.Valid("fs", h); !ok {
		t.Fatal("persisted")
	}
	s.Description = "changed"
	if ConfigHash(s) != h {
		t.Fatal("description does not affect the hash")
	}
	s.Args = []string{"y"}
	if _, ok := ix2.Valid("fs", ConfigHash(s)); ok {
		t.Fatal("config change invalidates")
	}
}

func TestRank(t *testing.T) {
	entries := map[string]IndexEntry{
		"github":  {Tools: []IndexTool{{Name: "create_issue", Description: "Create a GitHub issue"}, {Name: "list_issues", Description: "List issues in a repo"}, {Name: "get_file", Description: "Get file contents"}}},
		"context7": {Tools: []IndexTool{{Name: "get-library-docs", Description: "Fetch documentation for a library"}}},
	}
	hits := Rank("create issue", entries, "", 10)
	if len(hits) < 2 || hits[0].Tool != "create_issue" || hits[1].Tool != "list_issues" {
		t.Fatal(hits)
	}
	if hits := Rank("docs", entries, "github", 10); len(hits) != 0 {
		t.Fatal("server filter")
	}
	if hits := Rank("", entries, "context7", 10); len(hits) != 1 {
		t.Fatal("empty query lists")
	}
}
```

- [x] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/mcp/index.go
package mcp

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/adeotek/moca/internal/config"
)

type IndexTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Annotations Annotations `json:"annotations"`
}

type IndexEntry struct {
	ConfigHash string      `json:"configHash"`
	Tools      []IndexTool `json:"tools"`
	UpdatedAt  time.Time   `json:"updatedAt"`
}

type Index struct {
	path    string
	mu      sync.Mutex
	Servers map[string]IndexEntry `json:"servers"`
}

func LoadIndex(path string) (*Index, error) {
	ix := &Index{path: path, Servers: map[string]IndexEntry{}}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, ix)
		if ix.Servers == nil {
			ix.Servers = map[string]IndexEntry{}
		}
	}
	return ix, nil
}

func (ix *Index) Save() error {
	ix.mu.Lock()
	b, _ := json.MarshalIndent(ix, "", "  ")
	ix.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(ix.path), 0o700); err != nil {
		return err
	}
	tmp := ix.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ix.path)
}

func (ix *Index) Valid(server, hash string) (IndexEntry, bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	e, ok := ix.Servers[server]
	return e, ok && e.ConfigHash == hash
}

func (ix *Index) Put(server, hash string, tools []Tool) {
	e := IndexEntry{ConfigHash: hash, UpdatedAt: time.Now()}
	for _, t := range tools {
		e.Tools = append(e.Tools, IndexTool{Name: t.Name, Description: t.Description, Annotations: t.Annotations})
	}
	ix.mu.Lock()
	ix.Servers[server] = e
	ix.mu.Unlock()
}

func ConfigHash(s config.MCPServer) string {
	b, _ := json.Marshal(struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}{s.Command, s.Args, s.Env, s.URL, s.Headers}) // encoding/json sorts map keys
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type Hit struct {
	Server, Tool, Description string
	score                     int
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func Rank(query string, entries map[string]IndexEntry, server string, limit int) []Hit {
	q := words(query)
	var hits []Hit
	for srv, e := range entries {
		if server != "" && srv != server {
			continue
		}
		for _, t := range e.Tools {
			nameWords := words(t.Name)
			name, desc := strings.ToLower(t.Name), strings.ToLower(t.Description)
			score := 0
			for _, w := range q {
				if slices.Contains(nameWords, w) {
					score += 3
				} else if strings.Contains(name, w) {
					score += 2
				}
				if strings.Contains(desc, w) {
					score++
				}
			}
			if len(q) > 0 && score == 0 {
				continue
			}
			hits = append(hits, Hit{srv, t.Name, t.Description, score})
		}
	}
	slices.SortFunc(hits, func(a, b Hit) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.Server, b.Server), cmp.Compare(a.Tool, b.Tool))
	})
	return hits[:min(limit, len(hits))]
}
```

`TestRank` expects `list_issues` (score: "issue" contained in name +2, in description +1 → 3) after `create_issue` (create +3 +1, issue +2 +1 → 7). Matching is substring-based, so `issue` matches `issues`.

- [x] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/mcp && git commit -m "feat(mcp): persisted discovery index, config hash, word-match ranking"`

---

### Task 4: Manager — lazy lifecycle + idle stop

**Files:**
- Create: `internal/mcp/manager.go`
- Test: `internal/mcp/manager_test.go`

**Interfaces:**
- Produces:

```go
type Options struct {
	HTTP    *http.Client
	BaseEnv []string // os.Environ()
}
type Manager struct{…}
func NewManager(servers map[string]config.MCPServer, idle time.Duration, ix *Index, o Options) *Manager
func (m *Manager) Servers() []string                                    // sorted configured names
func (m *Manager) Search(ctx context.Context, query, server string) ([]Hit, error)
func (m *Manager) Describe(ctx context.Context, server, tool string) (Tool, error)
func (m *Manager) Call(ctx context.Context, server, tool string, args json.RawMessage) (CallResult, Tool, error)
func (m *Manager) Running() []string
func (m *Manager) IndexAll(ctx context.Context) (map[string]int, map[string]error)
func (m *Manager) Close()
```

- Per server: `state{mu sync.Mutex; cl *client; tools []Tool; timer *time.Timer; busy int}`.
  - `ensure(ctx, name)`, under `state.mu`: if not running, start the transport (stdio or HTTP), `initialize`, `listTools`, `ix.Put` + `ix.Save()`; on error → close the transport and return.
  - `Call`: under the lock, `busy++` and stop the timer; release the lock during the RPC; then under the lock `busy--` and, if `busy == 0`, reset the timer to `idle`. The timer func takes the lock and stops the server only if `busy == 0`.
  - A `session expired` (HTTP) or `exited` (stdio) error on call → mark stopped, `ensure` again, retry once.
- `Search`: for each configured server (or just `server`), use `ix.Valid(name, ConfigHash(cfg))`. Invalid/missing → `ensure` (which indexes it); the started server then follows the normal idle timeout. Unknown `server` → error `unknown MCP server "x" (configured: a, b)`.
- `Describe` / `Call`: unknown server → same error; unknown tool → `server x has no tool "y"; use action=search`.

- [x] **Step 1: Write failing tests**

```go
// internal/mcp/manager_test.go
package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

func countingServer(t *testing.T) (config.MCPServer, func() int) {
	f := filepath.Join(t.TempDir(), "starts")
	s := fakeServer("")
	s.Env["MOCA_FAKE_STARTS"] = f
	return s, func() int { b, _ := os.ReadFile(f); return len(b) }
}

func TestLazyStartAndPersistedIndex(t *testing.T) {
	s, starts := countingServer(t)
	ixPath := filepath.Join(t.TempDir(), "ix.json")
	ix, _ := LoadIndex(ixPath)
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	if starts() != 0 || len(m.Running()) != 0 {
		t.Fatal("nothing starts at construction")
	}
	hits, err := m.Search(context.Background(), "documentation", "")
	if err != nil || len(hits) != 1 || hits[0].Tool != "read_doc" || starts() != 1 {
		t.Fatal(hits, err, starts())
	}
	m.Close()

	// second "session": same index file, zero starts for search
	ix2, _ := LoadIndex(ixPath)
	m2 := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix2, Options{BaseEnv: os.Environ()})
	defer m2.Close()
	if hits, _ := m2.Search(context.Background(), "issue", ""); len(hits) != 1 || starts() != 1 {
		t.Fatal("search served from the persisted index; no new process", starts())
	}
	tool, err := m2.Describe(context.Background(), "docs", "read_doc")
	if err != nil || !strings.Contains(string(tool.InputSchema), "lib") || starts() != 2 {
		t.Fatal(tool, err)
	}
	r, _, err := m2.Call(context.Background(), "docs", "read_doc", json.RawMessage(`{"lib":"x"}`))
	if err != nil || r.Content[0].Text == "" || starts() != 2 {
		t.Fatal("call reuses the running server", r, err)
	}
}

func TestIdleStopAndRestart(t *testing.T) {
	s, starts := countingServer(t)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, 150*time.Millisecond, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	m.Call(context.Background(), "docs", "read_doc", nil)
	time.Sleep(400 * time.Millisecond)
	if len(m.Running()) != 0 {
		t.Fatal("stopped after idle timeout")
	}
	if _, _, err := m.Call(context.Background(), "docs", "read_doc", nil); err != nil || starts() != 2 {
		t.Fatal("restarts on demand", err, starts())
	}
}

// TestCallSurvivesIdleTimer (rev 11): a call landing exactly at idle expiry
// must not hit a nil client. 1ns idle → every call races the timer; run with
// -race. The one-critical-section Call (ensure + lookup + busy++ + timer stop
// under st.mu) closes the old two-section window.
func TestCallSurvivesIdleTimer(t *testing.T) {
	s, starts := countingServer(t)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Nanosecond, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	var wg sync.WaitGroup
	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := m.Call(context.Background(), "docs", "read_doc", nil); err != nil {
				t.Errorf("call at idle expiry: %v", err)
			}
		}()
	}
	wg.Wait()
	if starts() < 1 {
		t.Fatal("server started at least once:", starts())
	}
}

func TestUnknownServerAndTool(t *testing.T) {
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": fakeServer("")}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	if _, err := m.Search(context.Background(), "x", "nope"); err == nil || !strings.Contains(err.Error(), "configured: docs") {
		t.Fatal(err)
	}
	if _, _, err := m.Call(context.Background(), "docs", "nope", nil); err == nil || !strings.Contains(err.Error(), "action=search") {
		t.Fatal(err)
	}
}
```

- [x] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/mcp/manager.go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/config"
)

type Options struct {
	HTTP    *http.Client
	BaseEnv []string
}

type state struct {
	mu    sync.Mutex
	cfg   config.MCPServer
	cl    *client
	tools []Tool
	timer *time.Timer
	busy  int
}

type Manager struct {
	idle    time.Duration
	ix      *Index
	o       Options
	servers map[string]*state
}

func NewManager(servers map[string]config.MCPServer, idle time.Duration, ix *Index, o Options) *Manager {
	if o.HTTP == nil {
		o.HTTP = http.DefaultClient
	}
	m := &Manager{idle: idle, ix: ix, o: o, servers: map[string]*state{}}
	for n, s := range servers {
		m.servers[n] = &state{cfg: s}
	}
	return m
}

func (m *Manager) Servers() []string {
	var out []string
	for n := range m.servers {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

func (m *Manager) get(name string) (*state, error) {
	st, ok := m.servers[name]
	if !ok {
		return nil, fmt.Errorf("unknown MCP server %q (configured: %s)", name, strings.Join(m.Servers(), ", "))
	}
	return st, nil
}

// ensure starts the server if needed. Caller holds st.mu.
func (m *Manager) ensure(ctx context.Context, name string, st *state) error {
	if st.cl != nil {
		return nil
	}
	var tr transport
	var err error
	if st.cfg.URL != "" {
		tr, err = startHTTP(name, st.cfg, m.o.HTTP)
	} else {
		tr, err = startStdio(ctx, name, st.cfg, m.o.BaseEnv)
	}
	if err != nil {
		return err
	}
	if err := initialize(ctx, tr); err != nil {
		tr.Close()
		return err
	}
	cl := &client{t: tr, name: name}
	tools, err := cl.listTools(ctx)
	if err != nil {
		tr.Close()
		return err
	}
	st.cl, st.tools = cl, tools
	m.ix.Put(name, ConfigHash(st.cfg), tools)
	m.ix.Save()
	m.armTimer(name, st)
	return nil
}

// armTimer (re)starts the idle timer. Caller holds st.mu.
func (m *Manager) armTimer(name string, st *state) {
	if st.timer != nil {
		st.timer.Stop()
	}
	st.timer = time.AfterFunc(m.idle, func() {
		st.mu.Lock()
		defer st.mu.Unlock()
		if st.busy == 0 && st.cl != nil {
			st.cl.t.Close()
			st.cl = nil
		}
	})
}

func (m *Manager) Running() []string {
	var out []string
	for _, n := range m.Servers() {
		st := m.servers[n]
		st.mu.Lock()
		if st.cl != nil {
			out = append(out, n)
		}
		st.mu.Unlock()
	}
	return out
}

func (m *Manager) Search(ctx context.Context, query, server string) ([]Hit, error) {
	names := m.Servers()
	if server != "" {
		if _, err := m.get(server); err != nil {
			return nil, err
		}
		names = []string{server}
	}
	entries := map[string]IndexEntry{}
	var errs []string
	for _, n := range names {
		st := m.servers[n]
		if e, ok := m.ix.Valid(n, ConfigHash(st.cfg)); ok {
			entries[n] = e
			continue
		}
		st.mu.Lock()
		err := m.ensure(ctx, n, st)
		st.mu.Unlock()
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		entries[n], _ = m.ix.Valid(n, ConfigHash(st.cfg))
	}
	hits := Rank(query, entries, server, 20)
	if len(hits) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return hits, nil
}

func (m *Manager) Describe(ctx context.Context, server, tool string) (Tool, error) {
	st, err := m.get(server)
	if err != nil {
		return Tool{}, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := m.ensure(ctx, server, st); err != nil {
		return Tool{}, err
	}
	for _, t := range st.tools {
		if t.Name == tool {
			return t, nil
		}
	}
	return Tool{}, fmt.Errorf("server %s has no tool %q; use action=search", server, tool)
}

func (m *Manager) Call(ctx context.Context, server, tool string, args json.RawMessage) (CallResult, Tool, error) {
	for attempt := 0; ; attempt++ {
		// One critical section (rev 11): ensure + tool lookup + busy++ + timer
		// stop. The old two-section window (lookup unlocked its own lock, then
		// Call re-locked) let the idle timer fire in between, close the
		// transport and nil st.cl — a call landing exactly at idle expiry
		// then panicked on the nil client.
		st, err := m.get(server)
		if err != nil {
			return CallResult{}, Tool{}, err
		}
		st.mu.Lock()
		if err := m.ensure(ctx, server, st); err != nil {
			st.mu.Unlock()
			return CallResult{}, Tool{}, err
		}
		var t Tool
		for _, x := range st.tools {
			if x.Name == tool {
				t = x
				break
			}
		}
		if t.Name == "" {
			st.mu.Unlock()
			return CallResult{}, Tool{}, fmt.Errorf("server %s has no tool %q; use action=search", server, tool)
		}
		st.busy++
		if st.timer != nil {
			st.timer.Stop()
		}
		cl := st.cl
		st.mu.Unlock()
		res, err := cl.callTool(ctx, tool, args)
		st.mu.Lock()
		st.busy--
		// A dead transport is shared: closing it here also fails any other
		// in-flight call on this server, which then spends its own single
		// retry. Errors, not panics — acceptable for v1.
		retry := err != nil && attempt == 0 && (strings.Contains(err.Error(), "session expired") || strings.Contains(err.Error(), "exited"))
		if retry && st.cl == cl {
			cl.t.Close()
			st.cl = nil
		}
		if st.busy == 0 && st.cl != nil {
			m.armTimer(server, st)
		}
		st.mu.Unlock()
		if retry {
			continue
		}
		return res, t, err
	}
}

func (m *Manager) IndexAll(ctx context.Context) (map[string]int, map[string]error) {
	counts, errs := map[string]int{}, map[string]error{}
	for _, n := range m.Servers() {
		st := m.servers[n]
		st.mu.Lock()
		err := m.ensure(ctx, n, st)
		if err == nil {
			counts[n] = len(st.tools)
		}
		st.mu.Unlock()
		if err != nil {
			errs[n] = err
		}
	}
	return counts, errs
}

func (m *Manager) Close() {
	for _, st := range m.servers {
		st.mu.Lock()
		if st.timer != nil {
			st.timer.Stop()
		}
		if st.cl != nil {
			st.cl.t.Close()
			st.cl = nil
		}
		st.mu.Unlock()
	}
}
```

- [x] **Step 4: Run** — `go test ./internal/mcp/ -race -v` → PASS.

- [x] **Step 5: Commit**

```bash
git add internal/mcp
git commit -m "feat(mcp): manager with lazy start, idle stop, restart, index refresh"
```

---

### Task 5: The `mcp` proxy tool + gating

**Files:**
- Create: `internal/mcp/tool.go`
- Test: `internal/mcp/tool_test.go`

**Interfaces:**
- Consumes: `tools.Tool`, `tools.Env.Ask`, `tools.MCPSpec`, `tools.Truncate`, `Manager`.
- Produces:
  - `func NewTool(m *Manager, servers map[string]config.MCPServer) *ProxyTool`
  - `func (p *ProxyTool) Spec() llm.ToolSpec` → `tools.MCPSpec()`
  - `func (p *ProxyTool) Run(ctx context.Context, env *tools.Env, input json.RawMessage) tools.Result`
  - `func (p *ProxyTool) Allowed(server, tool string, a Annotations) bool` — read-only rule or the approve list (incl. `"*"`) or a session allow-always.
  - Output formats:
    - `search` → one line per hit: `server/tool — description` (description cut at 160 chars), footer `[describe before calling]`; no hits → `[no matching MCP tools]`.
    - `describe` → `server/tool\n<description>\nannotations: readOnly=… destructive=…\ninput schema:\n<pretty JSON>`.
    - `call` → text blocks joined with `\n`; `image`/`audio`/`resource`/`resource_link` → `[<type> omitted]`; truncated at 30K; `IsError` from the result. Summary `server/tool`.
  - Gating question: `tools.Question{Kind: "mcp", Subject: server + "/" + tool, Detail: string(args), CanAlways: true}`. `AllowAlways` → session allow plus TUI persistence (Task 6). `Deny` / no asker → error `refused: server/tool is not marked read-only and is not in mcp.servers.<server>.approve`.

- [x] **Step 1: Write failing tests**

```go
// internal/mcp/tool_test.go
package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

func newProxy(t *testing.T, approve []string) *ProxyTool {
	s := fakeServer("")
	s.Approve = approve
	servers := map[string]config.MCPServer{"docs": s}
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(servers, time.Minute, ix, Options{BaseEnv: os.Environ()})
	t.Cleanup(m.Close)
	return NewTool(m, servers)
}

func runP(p *ProxyTool, env *tools.Env, args map[string]any) tools.Result {
	b, _ := json.Marshal(args)
	return p.Run(context.Background(), env, b)
}

func TestProxySpecFrozen(t *testing.T) {
	a, _ := json.Marshal(newProxy(t, nil).Spec())
	b, _ := json.Marshal(tools.MCPSpec())
	if string(a) != string(b) {
		t.Fatal("proxy must return the frozen schema")
	}
}

func TestProxySearchDescribeCall(t *testing.T) {
	p := newProxy(t, nil)
	env := &tools.Env{}
	r := runP(p, env, map[string]any{"action": "search", "query": "documentation"})
	if !strings.Contains(r.Content, "docs/read_doc — Read library documentation") {
		t.Fatal(r.Content)
	}
	r = runP(p, env, map[string]any{"action": "describe", "server": "docs", "tool": "read_doc"})
	if !strings.Contains(r.Content, `"lib"`) || !strings.Contains(r.Content, "readOnly=true") {
		t.Fatal(r.Content)
	}
	r = runP(p, env, map[string]any{"action": "call", "server": "docs", "tool": "read_doc", "args": map[string]any{"lib": "go"}})
	if r.IsError || !strings.Contains(r.Content, "read_doc(map[lib:go])") || !strings.Contains(r.Content, "[image omitted]") {
		t.Fatal(r)
	}
}

func TestProxyGating(t *testing.T) {
	p := newProxy(t, nil)
	r := runP(p, &tools.Env{}, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"})
	if !r.IsError || !strings.Contains(r.Content, "mcp.servers.docs.approve") {
		t.Fatal("non-read-only refused without asker (-p)", r.Content)
	}
	var asked []tools.Question
	env := &tools.Env{Ask: func(_ context.Context, q tools.Question) tools.Answer { asked = append(asked, q); return tools.AllowAlways }}
	if r := runP(p, env, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"}); r.IsError {
		t.Fatal(r.Content)
	}
	runP(p, env, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"})
	if len(asked) != 1 || asked[0].Kind != "mcp" || asked[0].Subject != "docs/create_issue" {
		t.Fatal("allow-always sticks for the session", asked)
	}
	if r := runP(newProxy(t, []string{"*"}), &tools.Env{}, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"}); r.IsError {
		t.Fatal(`approve ["*"] trusts the server`)
	}
	if r := runP(newProxy(t, nil), &tools.Env{Ask: tools.AutoAllow}, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"}); r.IsError {
		t.Fatal("yolo (AutoAllow asker) runs non-read-only tools without asking")
	}
}

func TestProxyArgErrors(t *testing.T) {
	p := newProxy(t, nil)
	if r := runP(p, &tools.Env{}, map[string]any{"action": "describe", "server": "docs"}); !r.IsError || !strings.Contains(r.Content, "tool") {
		t.Fatal(r.Content)
	}
	if r := runP(p, &tools.Env{}, map[string]any{"action": "search", "server": "zzz"}); !r.IsError || !strings.Contains(r.Content, "configured: docs") {
		t.Fatal(r.Content)
	}
}
```

- [x] **Step 2: Run** — FAIL. **Step 3: Implement**

```go
// internal/mcp/tool.go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

type ProxyTool struct {
	m       *Manager
	approve map[string][]string
	mu      sync.Mutex
	session map[string]bool // allow-always in this session: "server/tool"
}

func NewTool(m *Manager, servers map[string]config.MCPServer) *ProxyTool {
	p := &ProxyTool{m: m, approve: map[string][]string{}, session: map[string]bool{}}
	for n, s := range servers {
		p.approve[n] = s.Approve
	}
	return p
}

func (p *ProxyTool) Spec() llm.ToolSpec { return tools.MCPSpec() }

func isTrue(b *bool) bool { return b != nil && *b }

func (p *ProxyTool) Allowed(server, tool string, a Annotations) bool {
	if isTrue(a.ReadOnlyHint) && !isTrue(a.DestructiveHint) {
		return true
	}
	if l := p.approve[server]; slices.Contains(l, "*") || slices.Contains(l, tool) {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.session[server+"/"+tool]
}

func errResult(format string, a ...any) tools.Result {
	return tools.Result{Content: fmt.Sprintf(format, a...), IsError: true}
}

func (p *ProxyTool) Run(ctx context.Context, env *tools.Env, input json.RawMessage) tools.Result {
	var a struct {
		Action string          `json:"action"`
		Server string          `json:"server"`
		Tool   string          `json:"tool"`
		Args   json.RawMessage `json:"args"`
		Query  string          `json:"query"`
	}
	if err := json.Unmarshal(input, &a); err != nil {
		return errResult("invalid arguments: %v", err)
	}
	need := func(fields ...string) *tools.Result {
		for _, f := range fields {
			if (f == "server" && a.Server == "") || (f == "tool" && a.Tool == "") {
				r := errResult("action=%s needs %s", a.Action, strings.Join(fields, " and "))
				return &r
			}
		}
		return nil
	}
	switch a.Action {
	case "search":
		hits, err := p.m.Search(ctx, a.Query, a.Server)
		if err != nil {
			return errResult("%v", err)
		}
		if len(hits) == 0 {
			return tools.Result{Content: "[no matching MCP tools]", Summary: "search " + a.Query}
		}
		var sb strings.Builder
		for _, h := range hits {
			d := h.Description
			if len(d) > 160 {
				d = d[:157] + "..."
			}
			fmt.Fprintf(&sb, "%s/%s — %s\n", h.Server, h.Tool, strings.Join(strings.Fields(d), " "))
		}
		sb.WriteString("[describe before calling]")
		return tools.Result{Content: sb.String(), Summary: fmt.Sprintf("search %q (%d)", a.Query, len(hits))}
	case "describe":
		if r := need("server", "tool"); r != nil {
			return *r
		}
		t, err := p.m.Describe(ctx, a.Server, a.Tool)
		if err != nil {
			return errResult("%v", err)
		}
		var schema []byte
		if len(t.InputSchema) > 0 {
			var v any
			json.Unmarshal(t.InputSchema, &v)
			schema, _ = json.MarshalIndent(v, "", "  ")
		}
		return tools.Result{Content: fmt.Sprintf("%s/%s\n%s\nannotations: readOnly=%v destructive=%v\ninput schema:\n%s",
			a.Server, t.Name, t.Description, isTrue(t.Annotations.ReadOnlyHint), isTrue(t.Annotations.DestructiveHint), schema),
			Summary: "describe " + a.Server + "/" + a.Tool}
	case "call":
		if r := need("server", "tool"); r != nil {
			return *r
		}
		t, err := p.m.Describe(ctx, a.Server, a.Tool) // starts the server, gives fresh annotations
		if err != nil {
			return errResult("%v", err)
		}
		if !p.Allowed(a.Server, a.Tool, t.Annotations) {
			ans := tools.Deny
			if env.Ask != nil {
				ans = env.Ask(ctx, tools.Question{Kind: "mcp", Subject: a.Server + "/" + a.Tool, Detail: string(a.Args), CanAlways: true})
			}
			switch ans {
			case tools.Deny:
				return errResult("refused: %s/%s is not marked read-only and is not in mcp.servers.%s.approve; the user did not approve it",
					a.Server, a.Tool, a.Server)
			case tools.AllowAlways:
				p.mu.Lock()
				p.session[a.Server+"/"+a.Tool] = true
				p.mu.Unlock()
			}
		}
		res, _, err := p.m.Call(ctx, a.Server, a.Tool, a.Args)
		if err != nil {
			return errResult("%v", err)
		}
		var parts []string
		for _, c := range res.Content {
			if c.Type == "text" {
				parts = append(parts, c.Text)
			} else {
				parts = append(parts, fmt.Sprintf("[%s omitted]", c.Type))
			}
		}
		out := tools.Truncate(strings.Join(parts, "\n"), 30_000)
		return tools.Result{Content: out, IsError: res.IsError, Summary: a.Server + "/" + a.Tool}
	}
	return errResult("action must be search, describe or call")
}
```

- [x] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/mcp && git commit -m "feat(mcp): proxy tool with annotation/approve gating and result rendering"`

---

### Task 6: Wire into agent + TUI + `-p`

**Files:**
- Modify: `internal/agent/start.go` (`build`), `internal/agent/agent.go` (`Close`), `internal/tui/app.go`, `cmd/moca/oneshot.go`
- Test: `internal/agent/agent_test.go` (extend)

**Interfaces:**
- In `build`: if `len(cfg.MCP.Servers) > 0`:
  - `ix := mcp.LoadIndex(DataDir()/mcp-index.json)`;
  - `mgr := mcp.NewManager(cfg.MCP.Servers, time.Duration(cfg.MCP.IdleTimeout)*time.Second, ix, mcp.Options{BaseEnv: os.Environ()})`;
  - `reg.Register(mcp.NewTool(mgr, cfg.MCP.Servers))`;
  - keep `mgr` on the agent.
- `func (a *Agent) Close() error` — stops MCP servers, closes the session writer. Both `-p` and the TUI call `Close` (replace their `Session().Close()` calls).
- TUI approval for `Kind == "mcp"` with `A` → `config.AppendString(cfgPath, []string{"mcp","servers",server,"approve"}, tool, nil)`. `Subject` is `server/tool`; split at the first `/`.
- The phase-2 roster lines are unchanged (no tool lists in the prompt).

- [x] **Step 1: Write failing test**

```go
func TestMCPNoSchemasInPromptAndZeroStarts(t *testing.T) {
	// configure the fake stdio server from internal/mcp via its helper binary is not reachable here;
	// use a command that would fail loudly if spawned:
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTestWith(t, s, `"mcp":{"servers":{"never":{"command":"/nonexistent/should-not-spawn","description":"docs lookup"}}}`, "")
	defer a.Close()
	a.Run(context.Background(), "hi")
	body := s.bodies[0]
	tools := body["tools"].([]any)
	if len(tools) != 7 {
		t.Fatalf("still exactly 7 tools, got %d", len(tools))
	}
	sys := body["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(sys, "- never: docs lookup") {
		t.Fatal("roster line present")
	}
	// spawning /nonexistent would have produced an error entry; none should exist
	entries, _ := session.ReadFile(a.Session().Path())
	for _, e := range entries {
		if e.Type == session.TypeError {
			t.Fatal("no server activity at session start", e.Error.Message)
		}
	}
}
```

- [x] **Step 2: Run** — FAIL (`a.Close` undefined). **Step 3: Implement** per the interfaces. **Step 4: Run** — `go test ./... -race` → PASS.

- [x] **Step 5: Commit**

```bash
git add internal/agent internal/tui cmd/moca
git commit -m "feat(agent): register the MCP proxy when servers are configured; allow-always persists to approve"
```

---

### Task 7: `config.SetObjectEntry` + `moca mcp import`

**Files:**
- Modify: `internal/config/edit.go` (add `SetObjectEntry`)
- Create: `internal/mcp/importer.go`, `internal/mcp/testdata/claude.json`, `internal/mcp/testdata/opencode.jsonc`, `internal/mcp/testdata/pi-mcp.json`, `cmd/moca/mcp.go`
- Test: `internal/config/edit_test.go` (extend), `internal/mcp/importer_test.go`

**Interfaces:**
- `func config.SetObjectEntry(path string, keyPath []string, key string, rawJSON string) (added bool, err error)` — inserts `"key": rawJSON` into the object at `keyPath` (creating missing parents like `AppendString`). If `key` already exists → `added=false`, unchanged. Same `.bak` + atomic write + comment preservation.
- Importer:
  - `type Source struct { Tool, Path string }` (`Tool` ∈ `claude-code|opencode|pi`)
  - `func DiscoverSources(home, cwd string) []Source` — existing files among: `~/.claude.json` (top-level `mcpServers` and `projects[<cwd>].mcpServers`), `<cwd>/.mcp.json` (`mcpServers`), `~/.config/opencode/opencode.json` / `opencode.jsonc` (`mcp`), and Pi's MCP config. **For Pi, read pi-mcp-adapter's README at implementation time for its config path(s)** and add them here. Its documented format is the Claude-compatible `mcpServers` shape, which `parseClaudeShape` handles.
  - `func ParseSource(src Source, cwd string) (map[string]config.MCPServer, error)`:
    - Claude shape: `{command, args, env}` | `{type:"http"|"sse", url, headers}`. `sse` servers are **skipped** with a warning: legacy SSE is unsupported.
    - OpenCode shape: `{type:"local", command:[cmd, args...], environment:{…}, enabled}` | `{type:"remote", url, headers, enabled}`. `enabled:false` → skipped.
    - All files are parsed with `config.Standardize` first (JSONC tolerant).
  - `type EnvVar struct { Name, Server, Field, Key string }` (`Field` = `env` | `headers`)
  - `func RewriteSecrets(server string, s config.MCPServer) (config.MCPServer, []EnvVar)`:
    - `${VAR}` / `$VAR` whole-value refs → `env:VAR`, with no export needed.
    - Values already `env:` → kept.
    - Key matching `(?i)key|token|secret|password|auth`, **or** a value matching `^(Bearer|Basic) \S+` / `^(sk-|ghp_|gho_|github_pat_|glpat-|xox[abp]-|AKIA)\S+` → `env:MOCA_MCP_<SERVER>_<KEY>` (upper-cased, non-alnum → `_`), recorded in the returned list.
    - Other literal values are kept.
  - `func Plan(sources []Source, existing map[string]config.MCPServer, cwd string) (adds map[string]config.MCPServer, vars []EnvVar, notes []string)` — first source wins on a name conflict; names already in moca's config → note `skipped <name>: already configured`.
- CLI `moca mcp import [--yes]`:
  - Print the preview: per server, the JSONC block that will be added, then `export MOCA_MCP_…=<value from <tool> config>` lines naming the source **without printing the secret value**, then the notes.
  - Without `--yes`, ask `write N servers to <config>? [y/N]` (stdin).
  - On yes → `SetObjectEntry(ConfigFile(), ["mcp","servers"], name, json)` for each.
- CLI `moca mcp index`: builds the Manager over the config servers, calls `IndexAll`, prints `name: N tools` / `name: error …`, then `Close`. Exit 1 if any server failed.

- [x] **Step 1: Fixtures**

`internal/mcp/testdata/claude.json`:

```json
{
  "mcpServers": {
    "github": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"],
                "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_literalsecret123", "LOG_LEVEL": "info" } },
    "linear": { "type": "http", "url": "https://mcp.linear.app/mcp", "headers": { "Authorization": "Bearer lin_abc" } },
    "old":    { "type": "sse", "url": "https://legacy.example/sse" }
  },
  "projects": { "/work/app": { "mcpServers": { "pg": { "command": "pg-mcp", "env": { "DATABASE_URL": "${DATABASE_URL}" } } } } }
}
```

`internal/mcp/testdata/opencode.jsonc`:

```jsonc
{
  // opencode config
  "mcp": {
    "context7": { "type": "remote", "url": "https://mcp.context7.com/mcp", "enabled": true },
    "fs": { "type": "local", "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem", "."], "environment": { "API_KEY": "plain" } },
    "off": { "type": "local", "command": ["x"], "enabled": false },
  },
}
```

`internal/mcp/testdata/pi-mcp.json` — the Claude shape, with one server `"github"` (duplicate name to test first-wins) and one `"brave": {"command":"brave-mcp","env":{"BRAVE_API_KEY":"BSA123"}}`.

- [x] **Step 2: Write failing tests**

```go
// internal/mcp/importer_test.go
package mcp

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

func TestImportPlan(t *testing.T) {
	srcs := []Source{{"claude-code", "testdata/claude.json"}, {"opencode", "testdata/opencode.jsonc"}, {"pi", "testdata/pi-mcp.json"}}
	adds, vars, notes := Plan(srcs, map[string]config.MCPServer{"context7": {}}, "/work/app")
	if _, ok := adds["old"]; ok {
		t.Fatal("legacy SSE skipped")
	}
	if _, ok := adds["off"]; ok {
		t.Fatal("disabled skipped")
	}
	if _, ok := adds["context7"]; ok {
		t.Fatal("existing kept")
	}
	gh := adds["github"]
	if gh.Env["GITHUB_PERSONAL_ACCESS_TOKEN"] != "env:MOCA_MCP_GITHUB_GITHUB_PERSONAL_ACCESS_TOKEN" || gh.Env["LOG_LEVEL"] != "info" {
		t.Fatal(gh.Env)
	}
	if adds["linear"].Headers["Authorization"] != "env:MOCA_MCP_LINEAR_AUTHORIZATION" || adds["linear"].URL == "" {
		t.Fatal(adds["linear"])
	}
	if adds["pg"].Env["DATABASE_URL"] != "env:DATABASE_URL" {
		t.Fatal("${VAR} → env:VAR", adds["pg"])
	}
	if fs := adds["fs"]; fs.Command != "npx" || len(fs.Args) != 3 || fs.Env["API_KEY"] != "env:MOCA_MCP_FS_API_KEY" {
		t.Fatal(fs)
	}
	if adds["brave"].Env["BRAVE_API_KEY"] != "env:MOCA_MCP_BRAVE_BRAVE_API_KEY" {
		t.Fatal("pi source")
	}
	for _, s := range adds {
		for _, v := range s.Env {
			if strings.Contains(v, "ghp_") || strings.Contains(v, "BSA123") || strings.Contains(v, "lin_abc") {
				t.Fatal("literal secret copied")
			}
		}
	}
	if len(vars) != 4 { // github, linear, fs, brave — the skipped duplicate pi/github adds none
		t.Fatalf("vars to export: %+v", vars)
	}
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "old") || !strings.Contains(joined, "context7") || !strings.Contains(joined, "github") {
		t.Fatal(notes)
	}
}
```

```go
// append to internal/config/edit_test.go
func TestSetObjectEntry(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	os.WriteFile(p, []byte("{\n  // c\n  \"mcp\": { \"servers\": { \"a\": {\"url\":\"https://x\"} } }\n}\n"), 0o600)
	added, err := SetObjectEntry(p, []string{"mcp", "servers"}, "b", `{"command": "y"}`)
	if err != nil || !added {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	c, err := Parse(b)
	if err != nil || c.MCP.Servers["b"].Command != "y" || c.MCP.Servers["a"].URL != "https://x" || !strings.Contains(string(b), "// c") {
		t.Fatalf("%s %v", b, err)
	}
	if added, _ := SetObjectEntry(p, []string{"mcp", "servers"}, "a", `{}`); added {
		t.Fatal("existing key untouched")
	}
}
```

- [x] **Step 3: Run** — FAIL.

- [x] **Step 4: Implement `SetObjectEntry`** in `internal/config/edit.go`. Reuse `scanner.value(keyPath)`:
  - **Path fully present (an object)**: check `key` among its members. To find members, scan the object's span with a fresh `scanner` and collect top-level keys. If `key` is absent, insert `"\n  "+quote(key)+": "+rawJSON+","` right after `{`, or with no comma when the object is empty.
  - **Path partially present**: insert the missing chain `nested(keyPath[depth:]...)`, ending in `{ key: raw }`, after the deepest object's `{`, as in `AppendString`.
  - Write `.bak` and replace atomically.

Factor the shared write tail of `AppendString` into `writeEdited(path string, src, out []byte) error`.

- [x] **Step 5: Implement `importer.go`**

```go
// internal/mcp/importer.go
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adeotek/moca/internal/config"
)

type Source struct{ Tool, Path string }

type EnvVar struct{ Name, Server, Field, Key string }

func DiscoverSources(home, cwd string) []Source {
	cands := []Source{
		{"claude-code", filepath.Join(home, ".claude.json")},
		{"claude-code", filepath.Join(cwd, ".mcp.json")},
		{"opencode", filepath.Join(home, ".config", "opencode", "opencode.json")},
		{"opencode", filepath.Join(home, ".config", "opencode", "opencode.jsonc")},
		// pi: add the pi-mcp-adapter config path(s) from its README (Claude-compatible mcpServers shape)
	}
	var out []Source
	for _, c := range cands {
		if _, err := os.Stat(c.Path); err == nil {
			out = append(out, c)
		}
	}
	return out
}

type claudeServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type opencodeServer struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Enabled     *bool             `json:"enabled"`
}

func readJSONC(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	std, err := config.Standardize(b)
	if err != nil {
		return err
	}
	return json.Unmarshal(std, v)
}

// ParseSource returns servers plus notes (skips).
func ParseSource(src Source, cwd string) (map[string]config.MCPServer, []string, error) {
	out := map[string]config.MCPServer{}
	var notes []string
	addClaude := func(m map[string]claudeServer) {
		for n, s := range m {
			switch {
			case s.Type == "sse":
				notes = append(notes, fmt.Sprintf("skipped %s (%s): legacy SSE transport is not supported", n, src.Tool))
			case s.URL != "":
				out[n] = config.MCPServer{URL: s.URL, Headers: s.Headers}
			case s.Command != "":
				out[n] = config.MCPServer{Command: s.Command, Args: s.Args, Env: s.Env}
			}
		}
	}
	switch src.Tool {
	case "claude-code", "pi":
		var f struct {
			MCPServers map[string]claudeServer `json:"mcpServers"`
			Projects   map[string]struct {
				MCPServers map[string]claudeServer `json:"mcpServers"`
			} `json:"projects"`
		}
		if err := readJSONC(src.Path, &f); err != nil {
			return nil, nil, err
		}
		addClaude(f.MCPServers)
		if p, ok := f.Projects[cwd]; ok {
			addClaude(p.MCPServers)
		}
	case "opencode":
		var f struct {
			MCP map[string]opencodeServer `json:"mcp"`
		}
		if err := readJSONC(src.Path, &f); err != nil {
			return nil, nil, err
		}
		for n, s := range f.MCP {
			if s.Enabled != nil && !*s.Enabled {
				notes = append(notes, fmt.Sprintf("skipped %s (opencode): disabled", n))
				continue
			}
			switch {
			case s.Type == "remote":
				out[n] = config.MCPServer{URL: s.URL, Headers: s.Headers}
			case len(s.Command) > 0:
				out[n] = config.MCPServer{Command: s.Command[0], Args: s.Command[1:], Env: s.Environment}
			}
		}
	}
	return out, notes, nil
}

var (
	secretKey   = regexp.MustCompile(`(?i)key|token|secret|password|auth`)
	secretValue = regexp.MustCompile(`^(?:(?:Bearer|Basic) \S+|(?:sk-|ghp_|gho_|github_pat_|glpat-|xox[abp]-|AKIA)\S+)`)
	varRef      = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)
	nonAlnum    = regexp.MustCompile(`[^A-Z0-9]+`)
)

func envName(server, key string) string {
	return "MOCA_MCP_" + strings.Trim(nonAlnum.ReplaceAllString(strings.ToUpper(server), "_"), "_") + "_" +
		strings.Trim(nonAlnum.ReplaceAllString(strings.ToUpper(key), "_"), "_")
}

func RewriteSecrets(server string, s config.MCPServer) (config.MCPServer, []EnvVar) {
	var vars []EnvVar
	rewrite := func(field string, m map[string]string) map[string]string {
		if m == nil {
			return nil
		}
		out := map[string]string{}
		for k, v := range m {
			switch {
			case strings.HasPrefix(v, "env:"):
				out[k] = v
			case varRef.MatchString(v):
				out[k] = "env:" + varRef.FindStringSubmatch(v)[1]
			case secretKey.MatchString(k) || secretValue.MatchString(v):
				name := envName(server, k)
				out[k] = "env:" + name
				vars = append(vars, EnvVar{Name: name, Server: server, Field: field, Key: k})
			default:
				out[k] = v
			}
		}
		return out
	}
	s.Env = rewrite("env", s.Env)
	s.Headers = rewrite("headers", s.Headers)
	return s, vars
}

func Plan(sources []Source, existing map[string]config.MCPServer, cwd string) (map[string]config.MCPServer, []EnvVar, []string) {
	adds := map[string]config.MCPServer{}
	var vars []EnvVar
	var notes []string
	origin := map[string]string{}
	for _, src := range sources {
		servers, n, err := ParseSource(src, cwd)
		notes = append(notes, n...)
		if err != nil {
			notes = append(notes, fmt.Sprintf("could not read %s: %v", src.Path, err))
			continue
		}
		for name, s := range servers {
			if _, ok := existing[name]; ok {
				notes = append(notes, fmt.Sprintf("skipped %s: already configured", name))
				continue
			}
			if o, ok := origin[name]; ok {
				notes = append(notes, fmt.Sprintf("skipped %s from %s: already taken from %s", name, src.Tool, o))
				continue
			}
			s2, v := RewriteSecrets(name, s)
			adds[name], origin[name] = s2, src.Tool
			vars = append(vars, v...)
		}
	}
	return adds, vars, notes
}
```

- [x] **Step 6: Implement `cmd/moca/mcp.go`** with `runMCP(ctx, o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout, stderr io.Writer) int`, dispatching `import` (`--yes` parsed from `o.Sub`) and `index`. In `main.go`, route `o.Sub[0] == "mcp"` there (before the model check: `mcp` subcommands don't need a model).

- [x] **Step 7: Run** — `go test ./... -race` → PASS.

- [x] **Step 8: Commit**

```bash
git add internal/config internal/mcp cmd/moca
git commit -m "feat(mcp): moca mcp import (secret rewriting) and moca mcp index"
```

---

### Task 8: Phase gate

- [x] **Unit**: `go test ./... -race -count=1` → PASS. In particular:
  - `TestProxySpecFrozen` and phase 2's `TestSchemasFrozen` are unchanged;
  - `TestLazyStartAndPersistedIndex` (second session searches with zero starts);
  - `TestIdleStopAndRestart`;
  - `TestProxyGating`;
  - `TestImportPlan`;
  - `TestMCPNoSchemasInPromptAndZeroStarts`.
- [x] **Live: real server via proxy.** Add `filesystem` (stdio, `npx -y @modelcontextprotocol/server-filesystem .`) and `context7` (HTTP) to the config. Run `moca mcp index` → counts printed. In the TUI ask `look up the Go net/http docs via MCP`: the model calls `mcp search` → `describe` → `call` on context7. Capture one request body (temporary debug env `MOCA_DUMP_REQUESTS=1` writing to `/tmp`, or a local proxy) and confirm `tools` has exactly 7 entries and no server tool schema appears anywhere in the payload.
- [x] **Live: zero servers at start.** Open a new session, then `pgrep -fl server-filesystem` → nothing. Ask a question needing only `search` → still nothing running. Then `call` → it appears. Wait `idleTimeout` (set it to 30 for the test) → gone.
- [x] **Live: gating.** `moca -p 'use the filesystem MCP server to write hello to x.txt'` → refused (`write_file` is not read-only) and exit 0 with an explanation. Add `"approve": ["write_file"]` → it works. Remove it again and run with `--yolo` → it works.
- [x] **Live: import.** On a machine with Claude Code / OpenCode configs: `moca mcp import` → the preview shows env names and no secret values; after `y`, `grep -E 'ghp_|sk-|Bearer ' ~/.config/moca/config.jsonc` → nothing.
- [x] **README + commit**: status `phase 5 done — MCP lazy proxy`; `git commit -m "docs: phase 5 gate passed"`.

---

## Implementation notes (added after implementation + gate runs)

Deviations from the task snippets, all deliberate; the plan's intent is kept.

1. **`EnvVar` gained a `Tool` field** — the plan's own export-hint requirement (`<value from <tool> config>`) cannot be rendered from the specified `{Name, Server, Field, Key}`; `Plan` stamps the winning source's tool onto each var. `RewriteSecrets` keeps its specified signature.
2. **stdio `Call` write-error path waits for the exit** — when the stdin write fails because the child died, returning the bare `EPIPE` would violate Review Focus 1 (the test wants the stderr tail). The call now waits on `done` (bounded by ctx) and returns the recorded exit error; `dead` is captured under the lock (race cleanliness).
3. **Beyond the plan's tests** (same code, more coverage): `TestDiscoverSources` (Pi/shared paths), `TestParsePiShape` (`stdio`/`http`/`streamable-http` types, `enabled:false`, `${VAR}` in a pi file), the agent test also asserts the **proxy replaced the stub** (a direct `mcp` tool run no longer answers "no MCP servers configured"), `TestAllowAlwaysMCPPersists` (the TUI `ctrl+a` path writes `mcp.servers.ctx7.approve`; the written config re-parses; the `/clear` copy learns it), `TestMCPImportCLI` (declined run writes nothing; accepted run rewrites secrets; `mcp index` no-op; unknown subcommand exit 2), `SetObjectEntry` also checks the invalid-value refusal leaves the file untouched.
4. **`SetObjectEntry` validates `rawJSON` before any write** and shares `prepEdit`/`writeEdited` with `AppendString` (the plan asked for the write-tail factor; the head factor followed to keep symlink/mode behavior identical). Missing-file creation and the “key exists” check are covered by `TestSetObjectEntry`.
5. **`cmd/moca` routes `mcp` before the model check**, as the plan specified; both subcommands load config only.
6. **The tmux quit check needs two `ctrl+c` presses within 1 s** (the input's documented double-press window) — a first attempt with 1 s between presses left the TUI open (and its server running, correctly); the gate re-ran with presses 0.2 s apart.

Gate evidence (2026-10-06, scratch dir `~/.hermes/cache/scratch/moca_gate5/`, real MCP servers + a scripted openai-completions provider; the OpenCode key that previous phases' scratch runs used was pruned by the 24 h sweep, so the real-model legs are recorded as pending for Ben):

- **`moca mcp index`** against the real `@modelcontextprotocol/server-filesystem` via `npx -y` (stdio): `fs: 14 tools`, exit 0 — and against the real context7 server over **streamable HTTP**: `ctx7: 2 tools`. Annotations came back exactly as DESIGN assumed (`read_text_file` readOnly, `write_file` readOnly=false destructive=true).
- **No schemas in the prompt (captured payloads)**: first `-p` request body carries exactly 7 tools — `read, write, edit, shell, search, ls, mcp` — with no server tool name or schema anywhere in it; the search hits (`fs/read_file — …`) appear only in the *tool result* of the following request, where they belong.
- **Zero servers at start / index-served search**: a search-only run observed **0** `server-filesystem` processes before, during and after a successful 14-hit search that used the persisted index.
- **Lazy start + Close**: a describe+call run observed the server appear during the call (`seen_during=1`) and **gone** (`after=0`) once moca exited — `Agent.Close` stops the process group. Real file content (`hello gate5`) came back through `read_text_file`.
- **Idle stop (tmux TUI, `idleTimeout: 30`)**: `start=0`, `after_search=0`, `after_call=2` (npx + node), `after_idle35s=0`. A separate run quit the TUI immediately after a call → `after_quit=0` (0 strays).
- **Gating ladder (`-p`)**: no approve → `refused: fs/write_file is not marked read-only and is not in mcp.servers.fs.approve; the user did not approve it`, no file written, **exit 0** with the model's explanation; `"approve": ["write_file"]` → file written; `--yolo` → file written.
- **Live HTTP call through the proxy**: search → describe → call on context7; the first two attempts returned context7's own validation errors through the proxy (the tool needs both `libraryName` and `query` — visible in the describe schema), the third returned real library results (`/samber/slog-http`, …).
- **Import**: the host's real configs contain no MCP servers (`.claude.json` `mcpServers: {}`, no `mcp` key in `opencode.jsonc`, no Pi files) → the correct `nothing to import` with notes. With a scratch HOME carrying realistic Claude/OpenCode/Pi files, the preview showed the JSONC blocks + export hints and **no secret value** (`(clean)` scans); the accepted write produced a config that re-parses, with every secret rewritten (`env:MOCA_MCP_GITHUB_GITHUB_PERSONAL_ACCESS_TOKEN`, `env:MOCA_MCP_LINEAR_AUTHORIZATION`, `env:MOCA_MCP_BRAVE_BRAVE_API_KEY`), `PLAIN_VAR` left literal, `sse`/disabled/duplicate names skipped with notes, and a second run skipped everything as already configured.
- **Deliberate gate deviation**: the import leg wrote to a scratch config, not `~/.config/moca/config.jsonc` — writing servers into the user's live (dotfiles-managed) config is a machine change the plan's literal wording did not consider safe to do unasked. The preview leg ran against the real host files.
- **Not verified here (pending an OpenCode key)**: a real model driving search→describe→call in the TUI, and the real-model gating run (`-p` with a live model) — the same code paths are covered by the scripted runs above plus the unit suites; the key used by phases 1–4's live gates was pruned from scratch before this session.

## Review fixes (2026-10-06, two independent subagent reviews)

Pass 1 (`docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy.md`, spec/plan conformance): **Approve with fixes — 0 High / 2 Medium / 5 Low.** Pass 2 (`docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy-pass-2.md`, adversarial quality): **Approve with fixes — 0 High / 3 Medium / 7 Low** (its header line says 6 Low while seven Low sections follow — a counting typo, the document is otherwise consistent and is left as written). All confirmed findings are fixed with regression tests; **307 top-level tests, race-clean; gofmt/vet clean**. Every behaviour-pinning test was verified to **fail against the pre-fix sources** (fixed files stashed, tests kept) before landing — see the cross-verification note below.

- **Pass-1 M1 + Pass-2 M3 (one root cause) — the restart-once retry predicate string-matched error text.** A server-returned error containing "exited" (or "session expired") restarted a healthy server, killing in-flight sibling calls; an aborted SSE response ("no response for …") never restarted though the design promised self-heal. The transports now return sentinels — `errTransportDead` (stdio exit; HTTP response that ends without our id, or a body read error) and `errSessionExpired` (404 once a session exists) — and `Manager.Call` retries on `errors.Is` only. `TestServerErrorTextDoesNotRestart` (pre-fix: two starts for a `-32000 job exited` error), `TestHTTPRestartOnSessionExpired` (404 → transport closed → re-initialize → the second session serves the call, exactly one restart), `TestHTTPNotifyStatusChecked`.
- **Pass-1 M2 — the stdio write-failure path could return `ctx.Err()` over the recorded exit error.** The branch now prefers the already-recorded `dead` error (checked under the lock) and only falls back to ctx after waiting; a deadline racing a dead server still surfaces `server <name> exited … : <stderr tail>` (and the sentinel, so the retry fires). `TestStdioWriteEPIPE` (write against a closed pipe must return the exit error wrapping `errTransportDead`).
- **Pass-2 M1 — an HTTP server that answers `text/event-stream` and wedges hung a call forever.** The manager's default client is now `&http.Client{Timeout: 15m}` (`defaultHTTPTimeout`; `Options.HTTP` still overrides for tests). `TestDefaultHTTPClientHasTimeout`.
- **Pass-2 M2 — `moca mcp import` copied literal secrets outside `env`/`headers`.** `literalSecretFields` scans `command`, every `args` entry and the URL (token-shaped values with a length floor, `Bearer/Basic` schemes, and `://user:pass@` userinfo); such a server is **skipped with a note** rather than imported with the secret copied (rewriting positional args would be guesswork). `TestImportSkipsLiteralSecretsOutsideEnv`.
- **Pass-1 L3 + Pass-2 L2 — two env/headers keys could normalize to one export variable.** `Plan` detects duplicate generated names (within a server and across accepted servers) and skips the server with a note naming the variable. `TestImportEnvNameCollisionSkipped`.
- **Pass-1 L2 — a corrupt index file could serve a half-parsed map.** `LoadIndex` resets to empty on any unmarshal error (the "corrupt → empty" contract). `TestLoadIndexCorruptResets` (pre-fix: the partially-decoded entry was served).
- **Pass-1 L4 + Pass-2 L3 — search descriptions were cut mid-rune.** `cutRunes` snaps the 157-byte cut to a rune boundary. `TestCutRunesSafe`, `TestProxySearchDescriptionRuneSafe` (pre-fix: invalid UTF-8 in the tool result).
- **Pass-1 L5 — `Close` waited unboundedly for a SIGKILLed child.** The wait is bounded at 5 s; `Agent.Close` at session end can no longer hang on a child stuck in uninterruptible sleep.
- **Pass-2 L4 — a failing call racing `Close` could respawn a server nothing would stop.** `Manager.closed` (atomic) is set first in `Close`; the retry loop and `ensure` refuse to start anything after it. `TestClosedManagerDoesNotRespawn` (pre-fix: a second start).
- **Pass-2 L6 — servers echoing string JSON-RPC ids were silently unsupported.** `responseID` accepts int64 and quoted-int64 echoes; `null`/fractions/garbage still do not parse. `TestResponseID`.
- **Pass-2 L7 — the import summary counted already-present servers as written.** The closing line now separates written from already-present-kept.
- **Pass-2 L1 (Notify status) and L5 (missing error-path tests)** — both fixed: the notification status is checked (404 → `session expired`), and the previously untested error paths now have tests (404 restart, EPIPE path, corrupt index, id echo, notify status, closed manager).
- **Not changed, recorded**: pass-1 L1 (closing a dead transport also fails sibling in-flight calls, each spending its own retry) — documented v1 semantics in the code; the sentinel change only narrows when that happens (a genuine transport death, never a server error string).

### Cross-verification (2026-10-06)

Re-ran `gofmt -l .` / `go vet ./...` / `go test ./... -race -count=1` on the fixed tree (307 tests, clean) and, for each behaviour-pinning test, ran it with the fixed sources stashed and the tests kept: `TestServerErrorTextDoesNotRestart` (2 starts pre-fix), `TestClosedManagerDoesNotRespawn` (respawned pre-fix), `TestImportSkipsLiteralSecretsOutsideEnv`, `TestImportEnvNameCollisionSkipped`, `TestLoadIndexCorruptResets` (half-index served pre-fix), `TestHTTPNotifyStatusChecked` (nil error pre-fix), `TestProxySearchDescriptionRuneSafe` / `TestCutRunesSafe`, `TestDefaultHTTPClientHasTimeout` — all failed pre-fix and pass now. `TestHTTPRestartOnSessionExpired` and `TestStdioWriteEPIPE` pass both before and after by design (they pin the restart machinery and the stderr-tail contract, which the string-match era already satisfied for those exact paths). The live-gate legs recorded above were not re-run (no provider key); nothing in the fix set touches the manager's lifecycle or the transports' happy paths.

## Review fixes — pass 3 (2026-10-06)

Pass 3 (`docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy-pass-3.md`, post-fix re-review): **Approve with fixes — 0 High / 3 Medium / 9 Low.** All fixed with regression tests (new file `internal/mcp/hardening_test.go`, plus table entries in `config_test.go`, a stronger agent test and a restored fixture); 324 top-level tests, race-clean (stable over `-count=8` for the `mcp` package); gofmt/vet clean. The Mediums and the first Lows were first reproduced with throwaway `go test -overlay` tests before any change.

- **M1 — stdio exit raced the stdout reader.** `cmd.StdoutPipe`'s read end is closed by `Wait` the moment the process is reaped, discarding unread output: a fatal message on stdout was missing from the error in ~5% of runs and a reply written just before an exit was lost in ~3–4%. stdout is now an `os.Pipe` moca owns; the exit handler lets the reader drain (500 ms grace, plus `cmd.WaitDelay` for a grandchild holding the pipes) before failing pending calls. `TestStdioStdoutFatalAlwaysSurfaces`, `TestStdioReplyThenExitKeepsReply` (200 spawns each), `TestStdioGrandchildHoldingStdoutDoesNotHangExit`.
- **A latent bug found while fixing L1:** a stdio exit *during* a call was delivered to the waiting call as a plain `rpcError`, so it never carried `errTransportDead` and was never classified as a dead transport (only an exit *before* the call was). Pending calls now receive the transport's real error.
- **M2 — import copied credentials embedded in `env`/`headers` values under innocuous keys** (`DATABASE_URL=postgres://user:pw@host`, `DSN=… password=…`). `RewriteSecrets` now also rewrites values with URL userinfo, a credential-shaped token anywhere, or a `password=` pair. `TestImportRewritesEmbeddedSecretsInValues`, `TestRewriteSecretsValueRuleAlone`; the `claude.json` fixture's token value (a secret-scrubber artifact) is restored. `Plan` now iterates servers in sorted order so which one wins an export-name collision is stable.
- **M3 — HTTP `Close` was unbounded** (DELETE with no context, under the server lock). It is bounded to 3 s. `TestHTTPCloseBounded`.
- **L1 — the restart-once replayed a possibly-executed `tools/call`.** A dead transport is always dropped (the next use restarts it), but the call is replayed once only when that cannot run it twice: `errNotSent` (never left), session expired, or a read-only non-destructive tool; otherwise the error says the call may have executed. `TestHTTPAbortedStreamReplayRule`, `TestStdioCrashMidCallReplayRule`, `TestStdioDeadBetweenCallsRestarts` (these also cover the aborted-SSE and stdio-restart paths that had no tests, L7).
- **L2** — explicit `"args": null` is sent as `{}`. `TestCallToolNullArgsSentAsEmptyObject`.
- **L3** — `search` names the servers it could not index (and an unsaved index) in bracketed notes, and remembers a failed start for 2 minutes instead of respawning the server on every query (`describe`/`call` always retry). `TestSearchReportsAndRemembersFailedServer`, `TestSearchReportsUnsavedIndex`. `Manager.Search` now returns `([]Hit, []string, error)`.
- **L4** — the handshake (start + initialize + tools/list) is bounded to 60 s, each `tools/call` to 15 min (a timed-out server is stopped), and `tools/list` pagination stops on a repeated cursor or after 100 pages. `Options.HandshakeTimeout`/`CallTimeout` override the defaults. `TestHandshakeTimeout`, `TestCallTimeoutStopsWedgedServer`, `TestListToolsRepeatedCursor`.
- **L5** — `Index.Save` writes a unique temp file and merges with the on-disk entries (a process's own re-indexed servers win), so concurrent moca processes no longer corrupt or drop each other's entries. `TestIndexSaveMergesConcurrentWriters`.
- **L6** — a negative `mcp.idleTimeout` is rejected by `Validate`.
- **L7** — the agent-level zero-start test now uses a marker-file command (an eager spawn in `build` would have failed silently and passed before); the aborted-SSE / stdio-restart paths and the value-only secret rule are tested (see above).
- **L8** — the stdio env filter compares names case-insensitively on Windows and keeps the variables Windows needs to run a process (`SystemRoot`, `USERPROFILE`, `APPDATA`, …). `TestKeepNameWindows` (logic only; not run on Windows).
- **L9** — SPECS §10.5: the garbled zero-start sentence and the `glpat-` prefix are corrected, and the new behaviours are documented.
- **Not changed, recorded:** `Describe` on an already-running server does not reset the idle timer (the timer arms at start and after every call, as documented); a stdout line longer than 64 MiB ends the reader (a bounded edge, noted in pass 1).

## Review fixes — pass 4 (2026-10-06)

Pass 4 (`docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy-pass-4.md`, maintainer re-review of the pass-3 fix commit `9179ee0`): **Approve with fixes — 0 High / 0 Medium / 2 Low.** All pass-3 findings verified as fixed in code with their tests; the reworked exit path re-checked for new races (concurrent `pr.Close()`/reader, single-sender channel discipline, bounded `Close`); suite green (325 tests, race-clean, `internal/mcp` stable over `-count=4`), CI green on `9179ee0`. Both new Lows fixed:

- **L1** — the `mcp.idleTimeout` validation message said "must be > 0 seconds" while only negative values are rejected (0 = the default 600). Message corrected.
- **L2** — `serverEnv` appended the server's explicit `env` entries after the inherited ones, so an explicit `PATH`/`HOME`/`TERM`/`XDG_*` override produced a duplicate name — and duplicate env names resolve platform-dependently (glibc keeps the first), silently defeating the override. The inherited entry is now removed first (case-insensitive on Windows). `TestServerEnvExplicitEntryReplacesInherited` fails against the pre-fix code and passes after.

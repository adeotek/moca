---
status: ready
batch: 1 of 4
tasks: 1-4
---

# `moca acp` — Batch 1 of 4: Foundations — JSON-RPC connection, schema & types, shared helpers, tool-call mapping

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this batch task by task in this session (the recommended mode on a usage-limited plan). superpowers:subagent-driven-development also works, at a higher cost. Steps use checkbox (`- [ ]`) syntax for tracking.

**This batch:** Tasks 1–4 of the ACP server plan — the stdlib JSON-RPC peer, the ACP v1 types checked against the pinned schema, three shared-helper moves (TUI/`-p` behaviour unchanged), and the pure tool-call mapping. No server yet.

**Whole feature:** `moca acp` serves the Agent Client Protocol (v1) on stdio so Zed, VS Code ACP extensions and Paseo can drive moca. It's a stdlib-only package `internal/acp` (bidirectional ndjson JSON-RPC 2.0 + ACP types + a server mapping each ACP session onto one `*agent.Agent`), wired through `moca acp` in `cmd/moca`.

**Tech Stack:** Go 1.27, stdlib only (`encoding/json`, `bufio`, `sync`, `context`); existing packages `agent`, `tools`, `skills`, `session`, `llm`, `config`, `provider`.

## Read only what this batch needs

This file is the **complete** instruction set for Tasks 1–4. The design decisions are already made and their code is written out below. To keep context and usage small:

1. **Do not open** the other plan files: `docs/superpowers/plans/2026-10-13-acp-batch-2-server.md`, `docs/superpowers/plans/2026-10-13-acp-batch-3-protocol-completeness.md`, `docs/superpowers/plans/2026-10-13-acp-batch-4-ship.md`, `docs/superpowers/plans/2026-10-13-acp-merged.md`, `docs/superpowers/plans/2026-10-13-acp.md`, `docs/plans/2026-10-13-acp-protocol.md`, or the spec `docs/superpowers/specs/2026-10-13-acp-design.md`. They are drafts or other batches; everything you need is here.
2. **Code from earlier batches is in the repo, not in other plans.** When a task's **Interfaces → Consumes** names a function or type, check it in the code (LSP `hover` / `documentSymbol` / `goToDefinition`, or `grep -n` for the symbol). Read whole files only when the task modifies them.
3. **Orientation:** if you need it, use `graphify query "<question>"` (a small scoped subgraph). Don't read `graphify-out/GRAPH_REPORT.md` or browse source trees.
4. **Tests:** run the exact per-step commands (`-run '…'`). Run the full suite once, at the end of the batch.
5. **Scope:** implement only Tasks 1–4. If a step seems to need something from a later task, it doesn't: do what the step says and note the question for the user.

## Before you start

This is the first batch: there is no earlier work to check. Start from a clean `feat/acp` checkout:

```bash
git switch feat/acp && git pull --ff-only && git status --short   # expect no output
```

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
- Task 2 Step 2 decides the schema feature gates (`messageId`, `usage_update`, def names). Record each outcome in `internal/acp/testdata/SOURCE.md` under a `## Feature gates` heading (one line each: present / renamed to X / absent). Later batches read that file instead of this plan.

## Review Focus (items owned by this batch)

- **Client disconnects while a permission request is pending.** On stdin EOF the pending call must resolve to `Deny`, the run must end and `Serve` must return without hanging. Pinned by `TestCallFailsOnEOF` (Task 1) and `TestEOFDuringPermissionDenies` (Task 6).
- **Very large JSON lines.** An embedded resource can carry a whole file, so a multi-MB request line must parse (scanner buffer up to 64 MB). Pinned by `TestLargeLine` (Task 1).
- **UTF-8 at the 8 KB cap.** Truncated tool output must stay valid UTF-8 (JSON-encoding broken runes turns them into U+FFFD garbage). Pinned by `TestCapKeepsUTF8` (Task 4).

## Files this batch touches

| | Path |
|---|---|
| create | `internal/acp/rpc.go` |
| test | `internal/acp/rpc_test.go` |
| create | `internal/acp/protocol.go` |
| create | `internal/acp/testdata/schema.json` |
| create | `internal/acp/testdata/SOURCE.md` |
| test | `internal/acp/schema_test.go` |
| test | `internal/acp/protocol_test.go` |
| modify | `internal/skills/prompts.go` |
| modify | `internal/tui/commands.go` |
| modify | `internal/session/rebuild.go` |
| modify | `internal/tui/replay.go` |
| modify | `internal/config/config.go` |
| modify | `cmd/moca/cli.go` |
| test | `internal/skills/prompts_test.go` |
| test | `internal/config/config_test.go` |
| create | `internal/acp/toolcall.go` |
| test | `internal/acp/toolcall_test.go` |

---

### Task 1: JSON-RPC connection (`rpc.go`)

**Files:**
- Create: `internal/acp/rpc.go`
- Test: `internal/acp/rpc_test.go`

**Interfaces:**
- Produces:
  - `type Handler interface { Request(ctx context.Context, method string, params json.RawMessage) (any, error); Notify(ctx context.Context, method string, params json.RawMessage) }`
  - `func NewConn(w io.Writer, h Handler) *Conn`
  - `func (c *Conn) Serve(ctx context.Context, r io.Reader) error`: returns at EOF or when ctx is done, after in-flight handlers return.
  - `func (c *Conn) Notify(method string, params any) error`
  - `func (c *Conn) Call(ctx context.Context, method string, params, result any) error`
  - `type Deferred struct { Result any; Then func() }`: a handler may return it to run `Then` after the response is written.
  - `type RPCError struct { Code int; Message string; Data any }` (implements `error`)
  - constants `codeParse=-32700`, `codeInvalidRequest=-32600`, `codeMethodNotFound=-32601`, `codeInvalidParams=-32602`, `codeInternal=-32603`, `codeAuthRequired=-32000`
  - `func invalidParams(format string, a ...any) error`, `func busy(msg string) error`
  - `var errClosed`

- [ ] **Step 1: Create the branch**

```bash
git switch -c phase/8-acp feat/acp   # from the branch that carries the plans
```

- [ ] **Step 2: Write the failing tests**

`internal/acp/rpc_test.go`:

```go
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// echoHandler answers "echo" with its params, "slow" after release is
// closed, and records notifications in order.
type echoHandler struct {
	mu      sync.Mutex
	notes   []string
	release chan struct{}
	then    chan struct{}
}

func (h *echoHandler) Request(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "echo":
		return params, nil
	case "slow":
		<-h.release
		return map[string]string{"ok": "slow"}, nil
	case "deferred":
		return Deferred{Result: map[string]int{"n": 1}, Then: func() { close(h.then) }}, nil
	case "bad":
		return nil, invalidParams("bad %s", "input")
	}
	return nil, &RPCError{Code: codeMethodNotFound, Message: "method not found: " + method}
}

func (h *echoHandler) Notify(ctx context.Context, method string, params json.RawMessage) {
	h.mu.Lock()
	h.notes = append(h.notes, method)
	h.mu.Unlock()
}

// pipePeer runs a Conn on pipes and returns the client's ends.
func pipePeer(t *testing.T, h Handler) (*Conn, io.WriteCloser, *bufio.Scanner, chan error) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := NewConn(outW, h)
	done := make(chan error, 1)
	go func() { done <- c.Serve(context.Background(), inR); outW.Close() }()
	sc := bufio.NewScanner(outR)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	t.Cleanup(func() { inW.Close() })
	return c, inW, sc, done
}

func readMsg(t *testing.T, sc *bufio.Scanner) map[string]any {
	t.Helper()
	if !sc.Scan() {
		t.Fatalf("no message: %v", sc.Err())
	}
	var m map[string]any
	if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
		t.Fatalf("bad JSON %q: %v", sc.Text(), err)
	}
	return m
}

func TestRequestResponse(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, `{"jsonrpc":"2.0","id":7,"method":"echo","params":{"a":1}}`+"\n")
	m := readMsg(t, sc)
	if m["id"] != float64(7) || m["result"].(map[string]any)["a"] != float64(1) || m["jsonrpc"] != "2.0" {
		t.Fatalf("got %v", m)
	}
}

func TestErrors(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, "{not json\n")
	if m := readMsg(t, sc); m["error"].(map[string]any)["code"] != float64(codeParse) || m["id"] != nil {
		t.Fatalf("parse error: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":"x","method":"bad","params":{}}`+"\n")
	if m := readMsg(t, sc); m["id"] != "x" || m["error"].(map[string]any)["code"] != float64(codeInvalidParams) {
		t.Fatalf("invalid params: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":2,"method":"nope"}`+"\n")
	if m := readMsg(t, sc); m["error"].(map[string]any)["code"] != float64(codeMethodNotFound) {
		t.Fatalf("method not found: %v", m)
	}
}

func TestConcurrentDispatch(t *testing.T) {
	h := &echoHandler{release: make(chan struct{})}
	_, in, sc, _ := pipePeer(t, h)
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"slow"}`+"\n")
	io.WriteString(in, `{"jsonrpc":"2.0","id":2,"method":"echo","params":1}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(2) {
		t.Fatalf("a blocked request held up the next one: %v", m)
	}
	close(h.release)
	if m := readMsg(t, sc); m["id"] != float64(1) {
		t.Fatalf("got %v", m)
	}
}

func TestNotificationsInOrder(t *testing.T) {
	h := &echoHandler{}
	_, in, sc, _ := pipePeer(t, h)
	for _, m := range []string{"a", "b", "c"} {
		io.WriteString(in, `{"jsonrpc":"2.0","method":"`+m+`"}`+"\n")
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"echo","params":0}`+"\n")
	readMsg(t, sc)
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.Join(h.notes, "") != "abc" {
		t.Fatalf("notes %v", h.notes)
	}
}

func TestDeferredRunsAfterResponse(t *testing.T) {
	h := &echoHandler{then: make(chan struct{})}
	_, in, sc, _ := pipePeer(t, h)
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"deferred"}`+"\n")
	m := readMsg(t, sc)
	if m["result"].(map[string]any)["n"] != float64(1) {
		t.Fatalf("got %v", m)
	}
	select {
	case <-h.then:
	case <-time.After(5 * time.Second):
		t.Fatal("Then never ran")
	}
}

func TestOutgoingCall(t *testing.T) {
	c, in, sc, _ := pipePeer(t, &echoHandler{})
	got := make(chan error, 1)
	var res struct{ V string }
	go func() { got <- c.Call(context.Background(), "client/thing", map[string]int{"q": 1}, &res) }()
	m := readMsg(t, sc)
	if m["method"] != "client/thing" {
		t.Fatalf("got %v", m)
	}
	id, _ := json.Marshal(m["id"])
	io.WriteString(in, `{"jsonrpc":"2.0","id":`+string(id)+`,"result":{"V":"yes"}}`+"\n")
	if err := <-got; err != nil || res.V != "yes" {
		t.Fatalf("err %v res %+v", err, res)
	}
}

func TestCallFailsOnEOF(t *testing.T) {
	c, in, sc, done := pipePeer(t, &echoHandler{})
	got := make(chan error, 1)
	go func() { got <- c.Call(context.Background(), "client/thing", nil, nil) }()
	readMsg(t, sc)
	in.Close()
	select {
	case err := <-got:
		if !errors.Is(err, errClosed) {
			t.Fatalf("err %v, want errClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Call hung after EOF")
	}
	if err := <-done; err != nil {
		t.Fatalf("Serve: %v", err)
	}
}

func TestLargeLine(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	big := strings.Repeat("x", 3<<20)
	go io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"echo","params":"`+big+`"}`+"\n")
	if m := readMsg(t, sc); len(m["result"].(string)) != len(big) {
		t.Fatal("large line lost")
	}
}

// Updates, responses and outgoing calls share one writer: under -race,
// concurrent Notify calls must never interleave bytes within a line.
func TestConcurrentWritesAreWholeLines(t *testing.T) {
	c, _, sc, _ := pipePeer(t, &echoHandler{})
	const g, n = 16, 50
	var wg sync.WaitGroup
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				c.Notify("session/update", map[string]string{"pad": strings.Repeat("y", 512)})
			}
		}()
	}
	for i := 0; i < g*n; i++ {
		if m := readMsg(t, sc); m["method"] != "session/update" {
			t.Fatalf("line %d: %v", i, m)
		}
	}
	wg.Wait()
}

func TestUnknownNotificationGetsNoReply(t *testing.T) {
	h := &echoHandler{}
	_, in, sc, _ := pipePeer(t, h)
	io.WriteString(in, `{"jsonrpc":"2.0","method":"$/whatever"}`+"\n")
	io.WriteString(in, `{"jsonrpc":"2.0","id":9,"method":"echo","params":0}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(9) {
		t.Fatalf("a notification was answered: %v", m)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/acp -race -count=1`
Expected: FAIL (build error: `undefined: NewConn` etc.)

- [ ] **Step 4: Write the implementation**

`internal/acp/rpc.go`:

```go
// Package acp serves the Agent Client Protocol (ACP v1) over stdio: editors
// such as Zed launch `moca acp` as a child process and drive moca sessions
// through it. JSON-RPC 2.0 is hand-rolled (as in internal/mcp); the peer is
// bidirectional — it serves the client's requests and calls the client back
// (session/request_permission).
package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
	codeAuthRequired   = -32000 // ACP: authentication required
)

// RPCError is a JSON-RPC error object; handlers return it to choose the code.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

func invalidParams(format string, a ...any) error {
	return &RPCError{Code: codeInvalidParams, Message: fmt.Sprintf(format, a...)}
}

// busy reports a request the session cannot take while a prompt runs.
func busy(msg string) error { return &RPCError{Code: codeInvalidRequest, Message: msg} }

var errClosed = errors.New("acp: connection closed")

// Handler serves incoming messages. Request runs on its own goroutine;
// Notify runs on the read loop, in arrival order, and must not block.
type Handler interface {
	Request(ctx context.Context, method string, params json.RawMessage) (any, error)
	Notify(ctx context.Context, method string, params json.RawMessage)
}

// Deferred lets a handler run follow-up work after its response is written
// (updates the client can only route once it has the response, e.g.
// available_commands_update after session/new).
type Deferred struct {
	Result any
	Then   func()
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Conn is one JSON-RPC peer over newline-delimited JSON.
type Conn struct {
	w       io.Writer
	wmu     sync.Mutex
	h       Handler
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan message
	closed  bool
	wg      sync.WaitGroup
}

func NewConn(w io.Writer, h Handler) *Conn {
	return &Conn{w: w, h: h, pending: map[int64]chan message{}}
}

// Serve reads messages until r ends or ctx is done, then cancels in-flight
// handlers' context, fails pending outgoing calls, and waits for the
// handlers to return. EOF is a normal end (nil error).
func (c *Conn) Serve(ctx context.Context, r io.Reader) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	lines := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 64<<20)
		for sc.Scan() {
			lines <- bytes.Clone(sc.Bytes())
		}
		readErr <- sc.Err()
		close(lines)
	}()
	var err error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case line, ok := <-lines:
			if !ok {
				err = <-readErr
				break loop
			}
			c.dispatch(ctx, line)
		}
	}
	cancel()
	c.failPending()
	c.wg.Wait()
	return err
}

func (c *Conn) dispatch(ctx context.Context, line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var m message
	if err := json.Unmarshal(line, &m); err != nil {
		c.write(message{ID: json.RawMessage("null"), Error: &RPCError{Code: codeParse, Message: "parse error"}})
		return
	}
	switch {
	case m.Method != "" && len(m.ID) > 0:
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.serveRequest(ctx, m)
		}()
	case m.Method != "":
		c.h.Notify(ctx, m.Method, m.Params)
	case len(m.ID) > 0:
		c.deliver(m)
	}
}

func (c *Conn) serveRequest(ctx context.Context, m message) {
	res, err := c.h.Request(ctx, m.Method, m.Params)
	var then func()
	if d, ok := res.(Deferred); ok {
		res, then = d.Result, d.Then
	}
	out := message{ID: m.ID}
	if err != nil {
		var re *RPCError
		if !errors.As(err, &re) {
			re = &RPCError{Code: codeInternal, Message: err.Error()}
		}
		out.Error = re
	} else if b, merr := json.Marshal(res); merr != nil {
		out.Error = &RPCError{Code: codeInternal, Message: merr.Error()}
	} else {
		out.Result = b
	}
	c.write(out)
	if then != nil && out.Error == nil {
		then()
	}
}

func (c *Conn) deliver(m message) {
	id, err := strconv.ParseInt(string(m.ID), 10, 64)
	if err != nil {
		return
	}
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- m
	}
}

func (c *Conn) failPending() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

func (c *Conn) write(m message) error {
	m.JSONRPC = "2.0"
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.w.Write(append(b, '\n'))
	return err
}

// Notify sends a notification to the client.
func (c *Conn) Notify(method string, params any) error {
	p, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(message{Method: method, Params: p})
}

// Call sends a request to the client and waits for its response; result
// (if non-nil) receives the decoded result. A closed connection is errClosed.
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	p, err := json.Marshal(params)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errClosed
	}
	c.nextID++
	id := c.nextID
	ch := make(chan message, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.write(message{ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method, Params: p}); err != nil {
		return err
	}
	select {
	case m, ok := <-ch:
		if !ok {
			return errClosed
		}
		if m.Error != nil {
			return m.Error
		}
		if result != nil {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS (all 10 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/acp/rpc.go internal/acp/rpc_test.go
git commit -m "feat(acp): bidirectional JSON-RPC 2.0 connection"
```

---

### Task 2: ACP types, pinned schema, conformance checker

**Files:**
- Create: `internal/acp/protocol.go`
- Create: `internal/acp/testdata/schema.json`, `internal/acp/testdata/SOURCE.md`
- Test: `internal/acp/schema_test.go`, `internal/acp/protocol_test.go`

**Interfaces:**
- Produces (all in `protocol.go`):
  - `const ProtocolVersion = 1`
  - update discriminators `updUserMessage, updAgentMessage, updThought, updToolCall, updToolUpdate, updCurrentMode, updCommands, updUsage`
  - `type UsageUpdate struct { SessionUpdate string; Used, Size int; Cost *Cost }`, `type Cost struct { Amount float64; Currency string }`
  - `statusInProgress, statusCompleted, statusFailed`; `stopEndTurn, stopMaxTokens, stopMaxTurns, stopRefusal, stopCancelled`; `optAllowOnce, optAllowAlways, optReject`
  - types `Implementation, InitializeRequest, InitializeResponse, AgentCapabilities, PromptCapabilities, MCPCapabilities, SessionCapabilities, AuthMethod, NewSessionRequest, NewSessionResponse, LoadSessionRequest, LoadSessionResponse, ListSessionsRequest, ListSessionsResponse, SessionInfo, PromptRequest, PromptResponse, CancelNotification, SetModeRequest, SetConfigOptionRequest, SetConfigOptionResponse, ContentBlock, EmbeddedResource, SessionNotification, ContentChunk, ToolCall, ToolCallLocation, ToolCallContent, CurrentModeUpdate, AvailableCommandsUpdate, AvailableCommand, AvailableCommandInput, SessionModeState, SessionMode, ConfigOption, ConfigOptionValue, RequestPermissionRequest, PermissionOption, RequestPermissionResponse, PermissionOutcome`
  - helpers `func strp(s string) *string`
- Produces (tests): `func conform(t *testing.T, def string, v any)`, which checks `v`'s JSON against `$defs[def]` of the pinned schema.

- [ ] **Step 1: Vendor the schema (data only, never executed)**

The upstream repo publishes three kinds of release in one stream: Rust crate releases (`v1.11.0`), v1 schema releases (`schema-v1.25.0`) and v2 schema alphas (`schema-v2.0.0-alpha.8`). So `releases/latest` can return any of the three. Since `schema-v1.25.0`, the files also live under `schema/v1/` (with `schema/v2/` beside it), so the old `schema/schema.json` path returns 404. moca speaks ACP **v1**: pin the newest `schema-v1.x.y` tag, and vendor the `v1` stable and unstable schemas plus `meta.json` (the method table).

```bash
TAG=$(gh api 'repos/agentclientprotocol/agent-client-protocol/releases?per_page=100' --paginate --jq '.[].tag_name' \
  | grep -E '^schema-v1\.[0-9]+\.[0-9]+$' | sort -V | tail -1)
echo "$TAG"   # schema-v1.25.0 or newer; never a v2 alpha or a crate tag
mkdir -p internal/acp/testdata
for f in schema.json schema.unstable.json meta.json; do
  curl -fsSL "https://raw.githubusercontent.com/agentclientprotocol/agent-client-protocol/$TAG/schema/v1/$f" -o "internal/acp/testdata/$f" || exit 1
done
python3 -I -c "import json; d=json.load(open('internal/acp/testdata/schema.json')); print(sorted(d['\$defs'].keys()))"
python3 -I -c "import json; m=json.load(open('internal/acp/testdata/meta.json')); print(m['version'], sorted(m['agentMethods'].values()))"
```

The second print must start with `1`. If `schema/v1/` 404s for the chosen tag, the layout has moved again: list the tag's tree (`gh api "repos/agentclientprotocol/agent-client-protocol/git/trees/$TAG?recursive=1" --jq '.tree[].path' | grep '^schema/'`) and take the **v1** stable schema, unstable schema and meta file from it — never a `v2` path. As of `schema-v1.25.0`, every def this plan uses is in the stable `schema.json`: `SessionNotification`, `InitializeResponse`, `NewSessionResponse`, `PromptResponse`, `RequestPermissionRequest`, `LoadSessionResponse`, `ListSessionsResponse`, `SetSessionConfigOptionResponse`, `ResumeSessionResponse`, `CloseSessionResponse`, `DeleteSessionResponse`, `Notice`, `CompactionUpdate`, `SessionInfoUpdate`, `ConfigOptionUpdate`, `UsageUpdate`, `CancelRequestNotification`. `conform` loads the unstable schema as a fallback only. Where a newer tag renames a def, use the schema's name in the tests; only names may differ from this plan.

Write `internal/acp/testdata/SOURCE.md`:

```markdown
ACP JSON schema (protocol v1), vendored for conformance tests (data only).
Source: https://github.com/agentclientprotocol/agent-client-protocol, tag <TAG>,
files schema/v1/schema.json, schema/v1/schema.unstable.json, schema/v1/meta.json.
Pick the newest `schema-v1.x.y` tag (the repo also tags crate releases `vX.Y.Z`
and v2 alphas `schema-v2.*`). Refresh: re-run Task 2 Step 1 of
docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md and fix whatever `go test ./internal/acp` reports.
```

Replace `<TAG>` with the printed tag.

- [ ] **Step 2: Check the shapes this plan assumes against the schema**

```bash
python3 -I - <<'EOF'
import json
d=json.load(open('internal/acp/testdata/schema.json'))
defs=d.get('$defs') or d.get('definitions')
for name in ['ContentChunk','ToolCall','ToolCallUpdate','ToolKind','ToolCallStatus','StopReason','PermissionOptionKind','AvailableCommand','SessionConfigOption']:
    print(name, json.dumps(defs.get(name), indent=None)[:600] if name in defs else 'MISSING')
EOF
```

Confirm:
- `ToolKind` contains `read, edit, execute, search, fetch, other`.
- `ToolCallStatus` contains `in_progress, completed, failed`.
- `StopReason` contains `end_turn, max_tokens, max_turn_requests, refusal, cancelled`.
- `PermissionOptionKind` contains `allow_once, allow_always, reject_once`.

Also look for the `usage_update` variant: run `grep -c '"usage_update"' internal/acp/testdata/schema.json` (and the same on `schema.unstable.json`). It is stable as of `schema-v1.25.0`. If it is present in either, keep `UsageUpdate` and its `conform` case. Check its property names (`used`, `size`, `cost{amount,currency}`) and rename tags to match. If it is absent from both, delete `UsageUpdate`, `Cost`, `updUsage`, the conform case and Task 5's `usage_update` step, and add "no `usage_update` (not in the pinned schema)" to the SPECS non-goals in Task 15.

If `ContentChunk` has no `messageId` property, delete the `MessageID` field from `ContentChunk` in Step 4, and delete every `MessageID:` assignment and the `msgIDs` map in Task 5. If `SessionConfigOption`'s select variant uses different property names than `id, name, category, type, currentValue, options[{value, name}]`, rename the JSON tags of `ConfigOption`/`ConfigOptionValue` to match. Tags are the only thing that may change.

- [ ] **Step 3: Write the conformance checker and the failing protocol tests**

`internal/acp/schema_test.go`:

```go
package acp

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// node is a JSON-schema fragment: only the keywords conform reads.
type node struct {
	Ref        string           `json:"$ref"`
	Type       any              `json:"type"`
	Required   []string         `json:"required"`
	Properties map[string]*node `json:"properties"`
	Items      *node            `json:"items"`
	Const      any              `json:"const"`
	Enum       []any            `json:"enum"`
	OneOf      []*node          `json:"oneOf"`
	AnyOf      []*node          `json:"anyOf"`
	AllOf      []*node          `json:"allOf"`
}

var (
	defsOnce sync.Once
	defs     map[string]*node
)

func loadDefs(t *testing.T) map[string]*node {
	t.Helper()
	defsOnce.Do(func() {
		defs = map[string]*node{}
		for _, f := range []string{"testdata/schema.json", "testdata/schema.unstable.json"} {
			b, err := os.ReadFile(f)
			if err != nil {
				continue // the unstable schema is optional
			}
			var doc struct {
				Defs  map[string]*node `json:"$defs"`
				Defs2 map[string]*node `json:"definitions"`
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				panic(fmt.Sprintf("%s: %v", f, err))
			}
			for k, v := range doc.Defs {
				if _, ok := defs[k]; !ok {
					defs[k] = v
				}
			}
			for k, v := range doc.Defs2 {
				if _, ok := defs[k]; !ok {
					defs[k] = v
				}
			}
		}
	})
	if len(defs) == 0 {
		t.Fatal("no ACP schema in testdata (Task 2 Step 1)")
	}
	return defs
}

// conform checks v's JSON against the pinned schema's def: types, required
// properties, const/enum values, $ref/allOf, and union branches (oneOf/anyOf:
// at least one must match). Deliberately partial — it pins what moca emits.
func conform(t *testing.T, def string, v any) {
	t.Helper()
	d := loadDefs(t)
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var x any
	json.Unmarshal(b, &x)
	if _, ok := d[def]; !ok {
		t.Fatalf("schema has no def %q", def)
	}
	if err := check(d, &node{Ref: "#/$defs/" + def}, x, def); err != nil {
		t.Errorf("%v\njson: %s", err, b)
	}
}

func resolve(d map[string]*node, n *node) (*node, error) {
	for n.Ref != "" {
		name := n.Ref[strings.LastIndex(n.Ref, "/")+1:]
		next, ok := d[name]
		if !ok {
			return nil, fmt.Errorf("unknown $ref %s", n.Ref)
		}
		n = next
	}
	return n, nil
}

func jsonType(x any) string {
	switch v := x.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		if v == float64(int64(v)) {
			return "integer"
		}
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	default:
		return "object"
	}
}

func typeOK(n *node, x any) bool {
	var want []string
	switch t := n.Type.(type) {
	case string:
		want = []string{t}
	case []any:
		for _, s := range t {
			want = append(want, s.(string))
		}
	default:
		return true
	}
	got := jsonType(x)
	return slices.Contains(want, got) || (got == "integer" && slices.Contains(want, "number"))
}

func check(d map[string]*node, n *node, x any, path string) error {
	n, err := resolve(d, n)
	if err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	if !typeOK(n, x) {
		return fmt.Errorf("%s: type %s, want %v", path, jsonType(x), n.Type)
	}
	for _, sub := range n.AllOf {
		if err := check(d, sub, x, path); err != nil {
			return err
		}
	}
	if n.Const != nil && !reflect.DeepEqual(n.Const, x) {
		return fmt.Errorf("%s: %v, want const %v", path, x, n.Const)
	}
	if len(n.Enum) > 0 && !slices.ContainsFunc(n.Enum, func(e any) bool { return reflect.DeepEqual(e, x) }) {
		return fmt.Errorf("%s: %v not in %v", path, x, n.Enum)
	}
	if alts := append(append([]*node{}, n.OneOf...), n.AnyOf...); len(alts) > 0 {
		var errs []string
		matched := false
		for _, a := range alts {
			if err := check(d, a, x, path); err == nil {
				matched = true
				break
			} else {
				errs = append(errs, err.Error())
			}
		}
		if !matched {
			return fmt.Errorf("%s: no alternative matches: %s", path, strings.Join(errs, " | "))
		}
	}
	if obj, ok := x.(map[string]any); ok {
		for _, r := range n.Required {
			if _, ok := obj[r]; !ok {
				return fmt.Errorf("%s: missing required %q", path, r)
			}
		}
		for k, p := range n.Properties {
			if v, ok := obj[k]; ok {
				if err := check(d, p, v, path+"."+k); err != nil {
					return err
				}
			}
		}
	}
	if arr, ok := x.([]any); ok && n.Items != nil {
		for i, v := range arr {
			if err := check(d, n.Items, v, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestConformRejectsBadShapes(t *testing.T) {
	d := loadDefs(t)
	bad := SessionNotification{SessionID: "abcd1234", Update: map[string]any{"sessionUpdate": "no_such_update"}}
	b, _ := json.Marshal(bad)
	var x any
	json.Unmarshal(b, &x)
	if check(d, &node{Ref: "#/$defs/SessionNotification"}, x, "n") == nil {
		t.Fatal("checker accepted an unknown sessionUpdate")
	}
}
```

`internal/acp/protocol_test.go`:

```go
package acp

import (
	"encoding/json"
	"os"
	"testing"
)

// Every method moca serves or calls must exist in the pinned v1 method table
// (meta.json): a typo or a renamed method fails here, not in an editor.
func TestMethodsInPinnedMeta(t *testing.T) {
	b, err := os.ReadFile("testdata/meta.json")
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Version         int               `json:"version"`
		AgentMethods    map[string]string `json:"agentMethods"`
		ClientMethods   map[string]string `json:"clientMethods"`
		ProtocolMethods map[string]string `json:"protocolMethods"`
	}
	if err := json.Unmarshal(b, &meta); err != nil || meta.Version != 1 {
		t.Fatalf("meta.json: version %d, err %v (want a v1 schema)", meta.Version, err)
	}
	known := map[string]bool{}
	for _, m := range []map[string]string{meta.AgentMethods, meta.ClientMethods, meta.ProtocolMethods} {
		for _, v := range m {
			known[v] = true
		}
	}
	for _, m := range acpMethods {
		if !known[m] {
			t.Errorf("method %q is not in the pinned ACP v1 schema", m)
		}
	}
}

func TestProtocolShapesConform(t *testing.T) {
	conform(t, "InitializeResponse", InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{LoadSession: true,
			PromptCapabilities:  PromptCapabilities{EmbeddedContext: true},
			SessionCapabilities: SessionCapabilities{List: &struct{}{}}},
		AuthMethods: []AuthMethod{}, AgentInfo: Implementation{Name: "moca", Title: "moca", Version: "1.0.0"},
	})
	conform(t, "PromptResponse", PromptResponse{StopReason: stopEndTurn})
	for _, kind := range []string{updAgentMessage, updThought, updUserMessage} {
		conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
			Update: ContentChunk{SessionUpdate: kind, Content: ContentBlock{Type: "text", Text: "hi"}, MessageID: newMessageID()}})
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: ToolCall{
		SessionUpdate: updToolCall, ToolCallID: "1-call_0", Title: "read a.go", Kind: "read", Status: statusInProgress,
		RawInput: []byte(`{"path":"a.go"}`), Locations: []ToolCallLocation{{Path: "/w/a.go"}}}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: ToolCall{
		SessionUpdate: updToolUpdate, ToolCallID: "1-call_0", Status: statusCompleted,
		Content: []ToolCallContent{
			{Type: "content", Content: &ContentBlock{Type: "text", Text: "ok"}},
			{Type: "diff", Path: "/w/a.go", OldText: strp("a"), NewText: strp("b")},
			{Type: "diff", Path: "/w/new.go", NewText: strp("")},
		}}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
		Update: CurrentModeUpdate{SessionUpdate: updCurrentMode, CurrentModeID: "plan"}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: AvailableCommandsUpdate{
		SessionUpdate: updCommands, AvailableCommands: []AvailableCommand{{Name: "review", Description: "d", Input: &AvailableCommandInput{Hint: "file"}}}}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
		Update: UsageUpdate{SessionUpdate: updUsage, Used: 1200, Size: 65536, Cost: &Cost{Amount: 0.0123, Currency: "USD"}}})
	conform(t, "RequestPermissionRequest", RequestPermissionRequest{SessionID: "abcd1234",
		ToolCall: ToolCall{ToolCallID: "1-call_0", Title: "shell: rm"},
		Options: []PermissionOption{{OptionID: optAllowOnce, Name: "Allow once", Kind: "allow_once"},
			{OptionID: optAllowAlways, Name: "Always allow", Kind: "allow_always"},
			{OptionID: optReject, Name: "Deny", Kind: "reject_once"}}})
}
```

- [ ] **Step 4: Run them to verify they fail**

Run: `go test ./internal/acp -run 'Conform|Protocol' -race -count=1`
Expected: FAIL (build error: undefined types)

- [ ] **Step 5: Write `protocol.go`**

```go
package acp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
)

// ProtocolVersion is the ACP major version moca speaks.
const ProtocolVersion = 1

// acpMethods lists every method name moca serves or calls; a test checks
// each against the pinned schema's meta.json. Later tasks append to it as
// they add methods.
var acpMethods = []string{
	"initialize", "authenticate", "session/new", "session/prompt", "session/cancel",
	"session/update", "session/request_permission",
	"session/set_mode", "session/set_config_option", "session/list", "session/load",
}

// session/update discriminators.
const (
	updUserMessage  = "user_message_chunk"
	updAgentMessage = "agent_message_chunk"
	updThought      = "agent_thought_chunk"
	updToolCall     = "tool_call"
	updToolUpdate   = "tool_call_update"
	updCurrentMode  = "current_mode_update"
	updCommands     = "available_commands_update"
	updUsage        = "usage_update"
)

const (
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
	statusFailed     = "failed"

	stopEndTurn   = "end_turn"
	stopMaxTokens = "max_tokens"
	stopMaxTurns  = "max_turn_requests"
	stopRefusal   = "refusal"
	stopCancelled = "cancelled"
)

// Permission option ids are moca's; their kinds are ACP's.
const (
	optAllowOnce   = "allow-once"
	optAllowAlways = "allow-always"
	optReject      = "reject"
)

type Implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type InitializeRequest struct {
	ProtocolVersion int `json:"protocolVersion"`
}

type InitializeResponse struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AuthMethods       []AuthMethod      `json:"authMethods"`
	AgentInfo         Implementation    `json:"agentInfo"`
}

type AgentCapabilities struct {
	LoadSession         bool                `json:"loadSession"`
	PromptCapabilities  PromptCapabilities  `json:"promptCapabilities"`
	MCPCapabilities     MCPCapabilities     `json:"mcpCapabilities"`
	SessionCapabilities SessionCapabilities `json:"sessionCapabilities"`
}

type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type MCPCapabilities struct {
	HTTP bool `json:"http"`
	SSE  bool `json:"sse"`
}

// SessionCapabilities: a non-nil List advertises session/list.
type SessionCapabilities struct {
	List *struct{} `json:"list,omitempty"`
}

type AuthMethod struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NewSessionRequest struct {
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

type NewSessionResponse struct {
	SessionID     string            `json:"sessionId"`
	Modes         *SessionModeState `json:"modes,omitempty"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
}

type LoadSessionRequest struct {
	SessionID  string            `json:"sessionId"`
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

type LoadSessionResponse struct {
	Modes         *SessionModeState `json:"modes,omitempty"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
}

type ListSessionsRequest struct {
	Cwd    string  `json:"cwd,omitempty"`
	Cursor *string `json:"cursor,omitempty"`
}

type ListSessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type SessionInfo struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type PromptRequest struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

type PromptResponse struct {
	StopReason string `json:"stopReason"`
}

type CancelNotification struct {
	SessionID string `json:"sessionId"`
}

type SetModeRequest struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type SetConfigOptionRequest struct {
	SessionID string          `json:"sessionId"`
	ConfigID  string          `json:"configId"`
	Value     json.RawMessage `json:"value"`
}

type SetConfigOptionResponse struct {
	ConfigOptions []ConfigOption `json:"configOptions"`
}

// ContentBlock covers the prompt blocks moca reads (text, resource_link,
// resource) and the text blocks it sends.
type ContentBlock struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	URI      string            `json:"uri,omitempty"`
	Name     string            `json:"name,omitempty"`
	Resource *EmbeddedResource `json:"resource,omitempty"`
}

type EmbeddedResource struct {
	URI      string  `json:"uri"`
	MimeType string  `json:"mimeType,omitempty"`
	Text     *string `json:"text,omitempty"`
	Blob     *string `json:"blob,omitempty"`
}

// SessionNotification is the session/update params; Update is one of the
// update structs below (each carries its sessionUpdate discriminator).
type SessionNotification struct {
	SessionID string `json:"sessionId"`
	Update    any    `json:"update"`
}

type ContentChunk struct {
	SessionUpdate string       `json:"sessionUpdate"`
	Content       ContentBlock `json:"content"`
	MessageID     string       `json:"messageId,omitempty"`
}

// ToolCall is the tool_call update, a tool_call_update (unchanged fields
// omitted), and the toolCall of a permission request (no sessionUpdate).
type ToolCall struct {
	SessionUpdate string             `json:"sessionUpdate,omitempty"`
	ToolCallID    string             `json:"toolCallId"`
	Title         string             `json:"title,omitempty"`
	Kind          string             `json:"kind,omitempty"`
	Status        string             `json:"status,omitempty"`
	RawInput      json.RawMessage    `json:"rawInput,omitempty"`
	Locations     []ToolCallLocation `json:"locations,omitempty"`
	Content       []ToolCallContent  `json:"content,omitempty"`
}

type ToolCallLocation struct {
	Path string `json:"path"`
}

// ToolCallContent is {type:"content", content} or {type:"diff", path, oldText, newText}.
type ToolCallContent struct {
	Type    string        `json:"type"`
	Content *ContentBlock `json:"content,omitempty"`
	Path    string        `json:"path,omitempty"`
	OldText *string       `json:"oldText,omitempty"`
	NewText *string       `json:"newText,omitempty"`
}

type CurrentModeUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	CurrentModeID string `json:"currentModeId"`
}

// UsageUpdate reports context fill and spend after each model turn.
type UsageUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	Used          int    `json:"used"`
	Size          int    `json:"size"`
	Cost          *Cost  `json:"cost,omitempty"`
}

type Cost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type AvailableCommandsUpdate struct {
	SessionUpdate     string             `json:"sessionUpdate"`
	AvailableCommands []AvailableCommand `json:"availableCommands"`
}

type AvailableCommand struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       *AvailableCommandInput `json:"input,omitempty"`
}

type AvailableCommandInput struct {
	Hint string `json:"hint"`
}

type SessionModeState struct {
	CurrentModeID  string        `json:"currentModeId"`
	AvailableModes []SessionMode `json:"availableModes"`
}

type SessionMode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ConfigOption struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Category     string              `json:"category,omitempty"`
	Type         string              `json:"type"`
	CurrentValue string              `json:"currentValue"`
	Options      []ConfigOptionValue `json:"options"`
}

type ConfigOptionValue struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

type RequestPermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  ToolCall           `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type RequestPermissionResponse struct {
	Outcome PermissionOutcome `json:"outcome"`
}

// PermissionOutcome: {outcome:"selected", optionId} or {outcome:"cancelled"}.
type PermissionOutcome struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId,omitempty"`
}

func strp(s string) *string { return &s }

// newMessageID returns a random UUID (v4 layout): a fresh id per streamed
// message lets a client start a new bubble after a stream restart.
func newMessageID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS. If `conform` reports a mismatch, fix the JSON tag or omitempty in `protocol.go` to match the schema, and record the fix in the commit body. Never edit the schema to match the code.

- [ ] **Step 7: Commit**

```bash
git add internal/acp/protocol.go internal/acp/protocol_test.go internal/acp/schema_test.go internal/acp/testdata
git commit -m "feat(acp): protocol types checked against the pinned ACP v1 schema"
```

---

### Task 3: Shared helpers (skill message, summary prefix, mode resolution)

These are three small moves so the TUI, `-p` and ACP share one implementation. Behaviour does not change.

**Files:**
- Modify: `internal/skills/prompts.go` (add `SkillMessage`)
- Modify: `internal/tui/commands.go:91,99-107` (call `skills.SkillMessage`, delete `skillMessage`)
- Modify: `internal/session/rebuild.go:53` (export `SummaryPrefix`)
- Modify: `internal/tui/replay.go:39` (use `session.SummaryPrefix`)
- Modify: `internal/config/config.go` (add `ResolveMode`)
- Modify: `cmd/moca/cli.go` (`Options.ResolveMode` delegates)
- Test: `internal/skills/prompts_test.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `func skills.SkillMessage(s skills.Skill, args string) string`
  - `const session.SummaryPrefix = "[Summary of earlier conversation]"`
  - `func config.ResolveMode(flag string, c config.Config) (mode string, plan bool)`

- [ ] **Step 1: Write the failing tests**

Append to `internal/skills/prompts_test.go`:

```go
func TestSkillMessage(t *testing.T) {
	s := Skill{Name: "deploy", Path: "/x/deploy/SKILL.md"}
	if got := SkillMessage(s, ""); got != "Load and follow the deploy skill (/x/deploy/SKILL.md)." {
		t.Errorf("no args: %q", got)
	}
	if got := SkillMessage(s, "staging"); got != "Load and follow the deploy skill (/x/deploy/SKILL.md) for this request: staging" {
		t.Errorf("args: %q", got)
	}
}
```

Append to `internal/config/config_test.go`:

```go
func TestResolveMode(t *testing.T) {
	base := Config{}
	proj := Config{Project: &PermissionsConfig{DefaultMode: "auto"}}
	cases := []struct {
		flag     string
		cfg      Config
		mode     string
		plan     bool
	}{
		{"", base, base.Mode(), false},
		{"", proj, "auto", false},
		{"yolo", proj, "yolo", false},
		{"plan", proj, "default", true},
	}
	for _, c := range cases {
		m, p := ResolveMode(c.flag, c.cfg)
		if m != c.mode || p != c.plan {
			t.Errorf("ResolveMode(%q) = %q,%v; want %q,%v", c.flag, m, p, c.mode, c.plan)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/skills ./internal/config -run 'SkillMessage|ResolveMode' -race -count=1`
Expected: FAIL (`undefined: SkillMessage`, `undefined: ResolveMode`)

- [ ] **Step 3: Implement the moves**

In `internal/skills/prompts.go`, add (with `"fmt"` in the imports if it is not already there):

```go
// SkillMessage is what a /skill invocation sends: the model loads the body
// with read (the message names the path) and follows it. Shared by the TUI
// and the ACP server.
func SkillMessage(s Skill, args string) string {
	if args == "" {
		return fmt.Sprintf("Load and follow the %s skill (%s).", s.Name, s.Path)
	}
	return fmt.Sprintf("Load and follow the %s skill (%s) for this request: %s", s.Name, s.Path, args)
}
```

In `internal/tui/commands.go`: delete the `skillMessage` function and its comment (lines 99–107), and change line 91 to `Text: skills.SkillMessage(k, args)}, nil`. Then run `grep -rn "skillMessage" internal/tui`; any remaining test references become `skills.SkillMessage`.

In `internal/session/rebuild.go`, add above the function containing line 53:

```go
// SummaryPrefix opens the user message that carries a compaction summary;
// front ends recognize it when replaying history.
const SummaryPrefix = "[Summary of earlier conversation]"
```

and change line 53 to `Text: SummaryPrefix + "\n" + c.Summary`. In `internal/tui/replay.go`, delete `const summaryPrefix = …` (line 39) and replace its uses with `session.SummaryPrefix` (the package is already imported).

In `internal/config/config.go`, add next to `func (c Config) Mode()`:

```go
// ResolveMode resolves a run's permission mode and plan scope (§3): an
// explicit flag value wins — "plan" is the plan scope at the default mode —
// else a trusted project file's defaultMode, else the config's mode.
// Shared by the CLI front ends and the ACP server.
func ResolveMode(flag string, c Config) (string, bool) {
	switch {
	case flag == "plan":
		return "default", true
	case flag != "":
		return flag, false
	case c.Project != nil && c.Project.DefaultMode != "":
		return c.Project.DefaultMode, false
	}
	return c.Mode(), false
}
```

In `cmd/moca/cli.go`, replace the body of `func (o Options) ResolveMode(cfg config.Config) (string, bool)`, keeping its doc comment:

```go
func (o Options) ResolveMode(cfg config.Config) (string, bool) {
	flag := ""
	if o.Mode != nil {
		flag = *o.Mode
	}
	return config.ResolveMode(flag, cfg)
}
```

- [ ] **Step 4: Run the full suite (the moves must not change behaviour)**

Run: `go test ./internal/skills ./internal/config ./internal/session ./internal/tui ./cmd/moca -race -count=1 && make vet`
Expected: PASS; `make vet` clean

- [ ] **Step 5: Commit**

```bash
git add internal/skills internal/tui/commands.go internal/tui/replay.go internal/session/rebuild.go internal/config cmd/moca/cli.go
git commit -m "refactor: share SkillMessage, SummaryPrefix and ResolveMode across front ends"
```

---

### Task 4: Tool-call mapping (`toolcall.go`)

**Files:**
- Create: `internal/acp/toolcall.go`
- Test: `internal/acp/toolcall_test.go`

**Interfaces:**
- Consumes: `ToolCall`, `ToolCallContent`, `ToolCallLocation`, `ContentBlock`, `strp`, status/update constants (Task 2); `llm.ToolCall{ID, Name string; Input json.RawMessage}`; `tools.Result{Content string; IsError bool; Summary, Detail string}`.
- Produces:
  - `const outputCap = 8 << 10`
  - `func startUpdate(id, cwd string, call llm.ToolCall) ToolCall`: a `tool_call` with status `in_progress`.
  - `func endUpdate(id, cwd string, call llm.ToolCall, r tools.Result) ToolCall`: a `tool_call_update` with status `completed`/`failed`.
  - `func capHead(s string, n int) string`, `func capTail(s string, n int) string`

- [ ] **Step 1: Write the failing tests**

`internal/acp/toolcall_test.go`:

```go
package acp

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

func call(name, input string) llm.ToolCall {
	return llm.ToolCall{ID: "call_0", Name: name, Input: json.RawMessage(input)}
}

func TestStartUpdateTable(t *testing.T) {
	cases := []struct {
		name, input, title, kind, loc string
	}{
		{"read", `{"path":"a.go"}`, "read a.go", "read", "/w/a.go"},
		{"write", `{"path":"/abs/b.go","content":"x"}`, "write /abs/b.go", "edit", "/abs/b.go"},
		{"edit", `{"path":"a.go","old_string":"a","new_string":"b"}`, "edit a.go", "edit", "/w/a.go"},
		{"shell", `{"command":"go test ./...\necho hi"}`, "go test ./...", "execute", ""},
		{"search", `{"pattern":"TODO","path":"internal"}`, "search TODO", "search", "/w/internal"},
		{"ls", `{}`, "ls .", "search", ""},
		{"web", `{"op":"search","query":"acp spec"}`, "search acp spec", "fetch", ""},
		{"web", `{"op":"fetch","url":"https://x.dev"}`, "fetch https://x.dev", "fetch", ""},
		{"mcp", `{"action":"call","server":"gh","tool":"issues"}`, "mcp gh/issues", "other", ""},
		{"mcp", `{"action":"search","query":"pr"}`, "mcp search pr", "other", ""},
		{"read", `{"path":`, "read", "read", ""}, // cut-off input: name only, no rawInput
	}
	for _, c := range cases {
		u := startUpdate("1-call_0", "/w", call(c.name, c.input))
		if u.SessionUpdate != updToolCall || u.Status != statusInProgress || u.Title != c.title || u.Kind != c.kind {
			t.Errorf("%s %s: got %+v", c.name, c.input, u)
		}
		if c.loc == "" && len(u.Locations) != 0 || c.loc != "" && (len(u.Locations) != 1 || u.Locations[0].Path != c.loc) {
			t.Errorf("%s: locations %+v, want %q", c.name, u.Locations, c.loc)
		}
		if json.Valid([]byte(c.input)) != (u.RawInput != nil) {
			t.Errorf("%s: rawInput %s", c.name, u.RawInput)
		}
		conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: u})
	}
}

func TestShellTitleCappedAt80(t *testing.T) {
	u := startUpdate("1", "/w", call("shell", `{"command":"`+strings.Repeat("é", 100)+`"}`))
	if utf8.RuneCountInString(u.Title) != 80 || !utf8.ValidString(u.Title) {
		t.Fatalf("title %q", u.Title)
	}
}

func TestEndUpdateContent(t *testing.T) {
	u := endUpdate("1", "/w", call("edit", `{"path":"a.go","old_string":"bug","new_string":"fix"}`), tools.Result{Summary: "a.go [+1 −1]"})
	if u.Status != statusCompleted || len(u.Content) != 1 || u.Content[0].Type != "diff" ||
		u.Content[0].Path != "/w/a.go" || *u.Content[0].OldText != "bug" || *u.Content[0].NewText != "fix" {
		t.Errorf("edit: %+v", u)
	}
	u = endUpdate("1", "/w", call("write", `{"path":"n.go","content":""}`), tools.Result{})
	if c := u.Content[0]; c.Type != "diff" || c.OldText != nil || c.NewText == nil || *c.NewText != "" {
		t.Errorf("write: %+v", u.Content)
	}
	u = endUpdate("1", "/w", call("read", `{"path":"a.go"}`), tools.Result{Content: "1|SECRET BODY", Summary: "a.go 1 line"})
	if b, _ := json.Marshal(u); strings.Contains(string(b), "SECRET BODY") || !strings.Contains(string(b), "a.go 1 line") {
		t.Errorf("read leaked its body or lost its summary: %s", b)
	}
	u = endUpdate("1", "/w", call("shell", `{"command":"false"}`), tools.Result{Content: "exit 1", IsError: true})
	if u.Status != statusFailed || u.Content[0].Content.Text != "exit 1" {
		t.Errorf("failed shell: %+v", u)
	}
	u = endUpdate("1", "/w", call("shell", `{"command":"yes"}`), tools.Result{Content: strings.Repeat("a", outputCap) + "TAIL"})
	if txt := u.Content[0].Content.Text; !strings.HasSuffix(txt, "TAIL") || len(txt) > outputCap+64 {
		t.Errorf("shell output not tail-capped: len %d", len(txt))
	}
	u = endUpdate("1", "/w", call("search", `{"pattern":"x"}`), tools.Result{Content: "HEAD" + strings.Repeat("a", outputCap)})
	if txt := u.Content[0].Content.Text; !strings.HasPrefix(txt, "HEAD") || len(txt) > outputCap+64 {
		t.Errorf("search output not head-capped: len %d", len(txt))
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: u})
}

func TestCapKeepsUTF8(t *testing.T) {
	s := strings.Repeat("é", outputCap) // 2 bytes per rune: the cap lands mid-rune
	for _, got := range []string{capHead(s, outputCap), capTail(s, outputCap)} {
		if !utf8.ValidString(got) {
			t.Fatal("cap split a rune")
		}
	}
	if capHead("short", outputCap) != "short" || capTail("short", outputCap) != "short" {
		t.Fatal("short strings must pass unchanged")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'Update|Cap|Title' -race -count=1`
Expected: FAIL (`undefined: startUpdate`)

- [ ] **Step 3: Write the implementation**

`internal/acp/toolcall.go`:

```go
package acp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// outputCap bounds the tool output copied into a tool call's content: the
// editor shows it, the model already has the full result.
const outputCap = 8 << 10

// toolInput is the union of the eight frozen tools' input fields moca maps.
type toolInput struct {
	Path      string  `json:"path"`
	Content   *string `json:"content"`
	OldString string  `json:"old_string"`
	NewString string  `json:"new_string"`
	Command   string  `json:"command"`
	Pattern   string  `json:"pattern"`
	Op        string  `json:"op"`
	URL       string  `json:"url"`
	Query     string  `json:"query"`
	Action    string  `json:"action"`
	Server    string  `json:"server"`
	Tool      string  `json:"tool"`
}

var toolKinds = map[string]string{
	"read": "read", "write": "edit", "edit": "edit", "shell": "execute",
	"search": "search", "ls": "search", "web": "fetch",
}

func toolKind(name string) string {
	if k, ok := toolKinds[name]; ok {
		return k
	}
	return "other"
}

// parseInput decodes call input; a cut-off or invalid input yields ok=false.
func parseInput(raw json.RawMessage) (toolInput, bool) {
	var in toolInput
	if !json.Valid(raw) || json.Unmarshal(raw, &in) != nil {
		return toolInput{}, false
	}
	return in, true
}

func toolTitle(name string, in toolInput, ok bool) string {
	if !ok {
		return name
	}
	switch name {
	case "read", "write", "edit":
		if in.Path != "" {
			return name + " " + in.Path
		}
	case "shell":
		line, _, _ := strings.Cut(strings.TrimSpace(in.Command), "\n")
		if r := []rune(line); len(r) > 80 {
			line = string(r[:80])
		}
		if line != "" {
			return line
		}
	case "search":
		return "search " + in.Pattern
	case "ls":
		if in.Path == "" {
			return "ls ."
		}
		return "ls " + in.Path
	case "web":
		if in.Op == "search" {
			return "search " + in.Query
		}
		return "fetch " + in.URL
	case "mcp":
		if in.Action == "call" {
			return fmt.Sprintf("mcp %s/%s", in.Server, in.Tool)
		}
		return strings.TrimSpace("mcp " + in.Action + " " + in.Query)
	}
	return name
}

func absPath(cwd, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(cwd, p)
}

// startUpdate announces a tool call (status in_progress).
func startUpdate(id, cwd string, call llm.ToolCall) ToolCall {
	in, ok := parseInput(call.Input)
	u := ToolCall{SessionUpdate: updToolCall, ToolCallID: id, Title: toolTitle(call.Name, in, ok),
		Kind: toolKind(call.Name), Status: statusInProgress}
	if ok {
		u.RawInput = call.Input
		if in.Path != "" && call.Name != "web" && call.Name != "mcp" {
			u.Locations = []ToolCallLocation{{Path: absPath(cwd, in.Path)}}
		}
	}
	return u
}

// endUpdate reports a finished call: a diff for write/edit, the summary for
// read (never the file body), capped output text for the rest.
func endUpdate(id, cwd string, call llm.ToolCall, r tools.Result) ToolCall {
	u := ToolCall{SessionUpdate: updToolUpdate, ToolCallID: id, Status: statusCompleted}
	if r.IsError {
		u.Status = statusFailed
		u.Content = textContent(capHead(r.Content, outputCap))
		return u
	}
	in, ok := parseInput(call.Input)
	switch {
	case call.Name == "edit" && ok:
		u.Content = []ToolCallContent{{Type: "diff", Path: absPath(cwd, in.Path), OldText: strp(in.OldString), NewText: strp(in.NewString)}}
	case call.Name == "write" && ok && in.Content != nil:
		u.Content = []ToolCallContent{{Type: "diff", Path: absPath(cwd, in.Path), NewText: strp(*in.Content)}}
	case call.Name == "read":
		u.Content = textContent(r.Summary)
	case call.Name == "shell":
		u.Content = textContent(capTail(r.Content, outputCap))
	default:
		u.Content = textContent(capHead(r.Content, outputCap))
	}
	return u
}

func textContent(s string) []ToolCallContent {
	if s == "" {
		return nil
	}
	return []ToolCallContent{{Type: "content", Content: &ContentBlock{Type: "text", Text: s}}}
}

// capHead keeps the first n bytes (cut back to a rune boundary) and notes the rest.
func capHead(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + fmt.Sprintf("\n[… %d more bytes not shown]", len(s)-cut)
}

// capTail keeps the last n bytes (moved forward to a rune boundary).
func capTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	start := len(s) - n
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return fmt.Sprintf("[… %d earlier bytes not shown]\n", start) + s[start:]
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/acp/toolcall.go internal/acp/toolcall_test.go
git commit -m "feat(acp): map moca tool calls to ACP tool_call updates"
```

---

## Batch 1 completion

- [ ] Full checks: `gofmt -l .` (expect no output), `make vet`, `make test` (add `export PATH="$(mise where go)/bin:$PATH"` when `go` is not on PATH).
- [ ] Every checkbox above is ticked, or the user agreed to skip it.
- [ ] Commits: if the user approved committing at the start, every task has its commit; otherwise everything is staged and nothing committed.
- [ ] `graphify update .` (project rule after code changes).

**Stop here.** Do not start the next batch in this session. Tell the user batch 1 is done, give the commit range, and say the next file is `docs/superpowers/plans/2026-10-13-acp-batch-2-server.md`, to be run in a fresh session (`/clear` first).

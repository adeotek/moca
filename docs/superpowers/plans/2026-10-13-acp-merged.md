---
status: draft
execute: false
superseded-by: docs/superpowers/plans/2026-10-13-acp-batch-*.md
---

> ⛔ **DRAFT — NO GO. Do not implement from this file, and do not use it as instructions for any agent.**
> This is the full merged plan, kept as the reference and the merge record (Appendix A). Its tasks were split, unchanged, into the batch files below, which are the only executable plans.
> To implement `moca acp`, run the batch files in order, one per session:
>
>   1. `docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md`
>   2. `docs/superpowers/plans/2026-10-13-acp-batch-2-server.md`
>   3. `docs/superpowers/plans/2026-10-13-acp-batch-3-protocol-completeness.md`
>   4. `docs/superpowers/plans/2026-10-13-acp-batch-4-ship.md`

# `moca acp` (Agent Client Protocol server) Implementation Plan — merged

> **Provenance.** This plan merges `docs/superpowers/plans/2026-10-13-acp.md` (Claude Code, "CC") and `docs/plans/2026-10-13-acp-protocol.md` (Hermes Agent, "HA"). The CC plan is the backbone because its tasks carry complete, code-checked implementations. HA contributes `usage_update`, the honoured saved-trust decision, the documented no-timeout rule, extra wire tests, a hermetic end-to-end gate over the real binary, the ACP TCK run and the version bump. Every point where the two plans differed is recorded, with its reason, in [Appendix A](#appendix-a--merge-decisions).

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `moca acp` subcommand that serves the Agent Client Protocol (v1) over stdio so Zed, VS Code ACP extensions and generic ACP UIs (Paseo) can drive moca.

**Architecture:** A new stdlib-only package `internal/acp`: a bidirectional newline-delimited JSON-RPC 2.0 connection (`rpc.go`), the ACP type subset (`protocol.go`), and a server that maps each ACP session onto one `*agent.Agent`. `agent.StartOptions.Emit` becomes a `session/update` translator and `agent.StartOptions.Ask` becomes a `session/request_permission` round-trip. It is the third front end beside the TUI and `-p`, wired through one `case "acp":` in `cmd/moca/main.go`; the TUI and `-p` paths are untouched apart from two shared-helper moves.

**Tech Stack:** Go 1.27 (stdlib only: `encoding/json`, `bufio`, `sync`, `context`); existing packages `agent`, `tools`, `skills`, `session`, `llm`, `config`.

**Spec:** `docs/superpowers/specs/2026-10-13-acp-design.md`. Read it first; this plan implements it, with the amendments in Appendix A, which win where they differ from the spec: honoured saved trust, `usage_update`, the no-timeout rule and deduplicated mode updates.

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
- Branch `phase/8-acp` off `main`. Use conventional commits. Each commit message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. **AGENTS.md: never commit without the user's explicit approval.** Get that approval once at the start of execution; if it isn't given, skip every "Commit" step and leave the changes staged.
- After code changes run `graphify update .` (project CLAUDE.md).
- **Permission requests have no timeout** (settled). `session/cancel` and stdin EOF are the only ways out of a pending request. A timeout would silently deny under clients without auto-accept (a known pitfall in other ACP agents), so do not add one.
- **Project trust per workdir** (merged rule): an explicit `--approve`/`--no-approve` wins; else yolo mode trusts; else a saved `trust.json` decision for that workdir is honoured; else trusted (the editor opening the folder is the trust act). ACP never writes `trust.json`.
- **`usage_update` ships** (settled). It is stable in `schema-v1.25.0`; if a newer pinned tag dropped it, it would be removed and listed as a known limitation (Task 2, Step 2).
- **Client-gated updates.** `notice`, `compaction_update` and `compaction_summary_chunk` are sent **only** when the client advertised `clientCapabilities.session.notices` / `.compaction` at `initialize` (spec MUST). Otherwise moca falls back to a thought line or the summarized-note chunk.
- **ACP v1 only.** The schema is pinned to the newest `schema-v1.x.y` tag under `schema/v1/`; never vendor a `schema-v2.*` alpha or a crate tag (`vX.Y.Z`).
- **Import alias.** Every `internal/acp` file that uses `internal/session` imports it as `msession`: the package declares `type session struct`, and Go rejects the clash.
- Version: this PR is **v0.5.0-beta** (settled; one bump per PR). The binary's version comes from the release ldflags (`internal/config/version.go` defaults to `0.0.0-dev`), so the bump is in docs (SPECS header, README status, CHANGELOG) and the release tag.

## Review Focus

1. **Provider tool-call ids repeat across turns.** OpenAI-compatible servers number calls `call_0`, `call_1`, … per response. Each ACP `toolCallId` must still be unique within the session, or the editor overwrites an earlier tool card. Pinned by `TestToolCallIDsUniqueAcrossTurns` (Task 5).
2. **`session/load` with `"last"` or a malformed id.** `session.Find` accepts `"last"`, so ACP must reject anything that isn't an id8 rather than silently resuming the newest session. Pinned by `TestLoadRejectsNonID8` (Task 9).
3. **Client disconnects while a permission request is pending.** On stdin EOF the pending call must resolve to `Deny`, the run must end and `Serve` must return without hanging. Pinned by `TestCallFailsOnEOF` (Task 1) and `TestEOFDuringPermissionDenies` (Task 6).
4. **Very large JSON lines.** An embedded resource can carry a whole file, so a multi-MB request line must parse (scanner buffer up to 64 MB). Pinned by `TestLargeLine` (Task 1).
5. **UTF-8 at the 8 KB cap.** Truncated tool output must stay valid UTF-8 (JSON-encoding broken runes turns them into U+FFFD garbage). Pinned by `TestCapKeepsUTF8` (Task 4).

---

## File map

| File | Status | Responsibility |
|---|---|---|
| `internal/acp/rpc.go` | create | JSON-RPC 2.0 peer: read loop, concurrent request dispatch, ordered notifications, outgoing calls, `Deferred` follow-ups |
| `internal/acp/protocol.go` | create | ACP types and constants used by moca |
| `internal/acp/testdata/schema.json` (+ `SOURCE.md`) | create | pinned upstream ACP schema |
| `internal/acp/schema_test.go` | create | partial JSON-schema conformance checker (`conform`) |
| `internal/acp/toolcall.go` | create | tool → title/kind/locations/content/diff |
| `internal/acp/server.go` | create | `Options`, `Serve`, method table, session map, `initialize`, `session/new`, `session/prompt`, `session/cancel` |
| `internal/acp/session.go` | create | per-session state, event translator, permission bridge, run state |
| `internal/acp/commands.go` | create | prompt flattening, slash commands |
| `internal/acp/modes.go` | create | modes and model config option |
| `internal/acp/history.go` | create | `session/list`, `session/load`, `session/resume`, `session/close`, `session/delete`, replay |
| `internal/acp/*_test.go` | create | tests; `script_test.go` and `client_test.go` are harnesses; `lifecycle_test.go` (Task 11), `updates_test.go` (Task 12, with the agent-less `bareSession` harness) |
| `internal/acp/testdata/meta.json` | create | pinned v1 method table; `TestMethodsInPinnedMeta` checks every method moca uses against it |
| `internal/skills/prompts.go` | modify | gains `SkillMessage` (moved from tui) |
| `internal/session/rebuild.go` | modify | exports `SummaryPrefix` |
| `internal/config/config.go` | modify | gains `ResolveMode` (moved from cmd/moca) |
| `internal/tui/commands.go`, `internal/tui/replay.go` | modify | use the moved helpers |
| `cmd/moca/cli.go` | modify | `acp` in the subcommand whitelist and usage; `Options.ResolveMode` delegates |
| `cmd/moca/main.go` | modify | `case "acp":` |
| `cmd/moca/acp.go`, `cmd/moca/acp_test.go` | create | `runACP`, the per-workdir trust resolver |
| `test/acpgate/acpgate_test.go`, `test/acpgate/run.sh` | create | hermetic end-to-end gate driving the built binary (build tag `acpgate`) |
| `docs/specs/SPECS.md`, `README.md`, `CHANGELOG.md`, `AGENTS.md` | modify | docs |

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
git switch -c phase/8-acp
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
docs/superpowers/plans/2026-10-13-acp-merged.md and fix whatever `go test ./internal/acp` reports.
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

### Task 5: Server core: initialize, session/new, session/prompt, event translation

**Files:**
- Create: `internal/acp/server.go`, `internal/acp/session.go`
- Test: `internal/acp/script_test.go`, `internal/acp/client_test.go`, `internal/acp/server_test.go`

**Interfaces:**
- Consumes: Task 1 (`Conn`, `Deferred`, `RPCError`, codes), Task 2 types, Task 3 `config.ResolveMode`, Task 4 `startUpdate`/`endUpdate`; `agent.Start(agent.StartOptions) (*agent.Agent, error)`; `(*agent.Agent).Run(ctx, string) (agent.Outcome, error)`, `.Close() error`, `.Session().ID8() string`; `agent.Outcome{Text string; MaxSteps, Stuck bool}`; events `agent.TextDelta{Text}`, `ThinkingDelta{Text}`, `StreamReset{}`, `ToolStart{Call}`, `ToolEnd{Call, Result}`, `TurnEnd{Message, Usage, Cost, Stop}`, `Warning{Text}`.
- Produces:
  - `type Options struct { Config config.Config; ConfigPath, Model, Effort, Mode string; Trust func(cwd string) (bool, error); Version string; HTTP *http.Client }` (`Trust` nil = every workdir trusted)
  - `func Serve(ctx context.Context, r io.Reader, w io.Writer, o Options) error`
  - `type Server` with `Request`/`Notify` (implements `Handler`), `func (srv *Server) lookup(id string) (*session, error)`, `func (srv *Server) sessionConfig(cwd string) (cfg config.Config, trusted bool, err error)`, `func (srv *Server) startOptions(cfg config.Config, s *session) agent.StartOptions`, `func (srv *Server) add(s *session)`, `func (srv *Server) get(id string) (*session, bool)`
  - `type session` with `newSession(srv *Server, cwd string, trusted bool) *session`, `attach(*agent.Agent)`, `markReady()`, `send(any)`, `emit(agent.Event)`, `begin(ctx) (context.Context, error)`, `end()`, `cancelPrompt()`, `idle(func() (any, error)) (any, error)`, `currentTool() string`, `openTool(callID string) string`, `closeTool(callID string) (string, bool)`, `stopReason(ctx, agent.Outcome, error) (any, error)`
  - `func decode[T any](params json.RawMessage) (T, error)`
  - test harness: `newScript`, `textTurn`, `toolTurn`, `testOptions`, `startClient`, `(*testClient).call/notify/updatesOf/newSession/prompt/agentText`, `selected(optionID)`, `cancelledOutcome()`

- [ ] **Step 1: Write the test harnesses**

`internal/acp/script_test.go` is a copy of the agent tests' scripted openai-completions server. The agent's helpers are unexported test code in another package, so they can't be imported.

```go
package acp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// scriptServer serves openai-completions SSE responses from a script, one per request.
type scriptServer struct {
	mu    sync.Mutex
	turns []string
	srv   *httptest.Server
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
		io.Copy(io.Discard, r.Body)
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
```

`internal/acp/client_test.go` plays the editor:

```go
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

type wireMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// testOptions isolates the data/config dirs and points the fake provider
// (models fake/m and fake/m2) at s.
func testOptions(t *testing.T, s *scriptServer) Options {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m","context":{"maxSteps":40},
		"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536},"m2":{"contextWindow":65536}}}}}`, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	return Options{Config: cfg, Version: "test", HTTP: s.srv.Client()} // Trust nil: trusted
}

// testClient drives Serve over pipes, records session/update payloads and
// answers session/request_permission through onPermission.
type testClient struct {
	t            *testing.T
	in           *io.PipeWriter
	wmu          sync.Mutex
	mu           sync.Mutex
	next         int
	waiting      map[string]chan wireMsg
	updates      []map[string]any
	perms        []RequestPermissionRequest
	onPermission func(RequestPermissionRequest) RequestPermissionResponse
	done         chan struct{} // closed when Serve returns (receivable any number of times)
}

func selected(id string) RequestPermissionResponse {
	return RequestPermissionResponse{Outcome: PermissionOutcome{Outcome: "selected", OptionID: id}}
}

func cancelledOutcome() RequestPermissionResponse {
	return RequestPermissionResponse{Outcome: PermissionOutcome{Outcome: "cancelled"}}
}

func startClient(t *testing.T, o Options) *testClient {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &testClient{t: t, in: inW, waiting: map[string]chan wireMsg{}, done: make(chan struct{}),
		onPermission: func(RequestPermissionRequest) RequestPermissionResponse { return selected(optAllowOnce) }}
	go func() {
		if err := Serve(context.Background(), inR, outW, o); err != nil {
			t.Errorf("Serve: %v", err)
		}
		outW.Close()
		close(c.done)
	}()
	go c.read(outR)
	t.Cleanup(func() {
		inW.Close()
		select {
		case <-c.done:
		case <-time.After(10 * time.Second):
			t.Error("Serve did not return after stdin EOF")
		}
	})
	return c
}

func (c *testClient) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var m wireMsg
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil || m.JSONRPC != "2.0" {
			c.t.Errorf("server wrote a non-JSON-RPC line: %q", sc.Text())
			continue
		}
		switch m.Method {
		case "session/update":
			var n struct {
				Update map[string]any `json:"update"`
			}
			json.Unmarshal(m.Params, &n)
			c.mu.Lock()
			c.updates = append(c.updates, n.Update)
			c.mu.Unlock()
		case "session/request_permission":
			var req RequestPermissionRequest
			json.Unmarshal(m.Params, &req)
			c.mu.Lock()
			c.perms = append(c.perms, req)
			c.mu.Unlock()
			go func(id json.RawMessage) {
				b, _ := json.Marshal(c.onPermission(req))
				c.write(wireMsg{JSONRPC: "2.0", ID: id, Result: b})
			}(m.ID)
		case "":
			c.mu.Lock()
			ch := c.waiting[string(m.ID)]
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		}
	}
}

func (c *testClient) write(m wireMsg) {
	b, _ := json.Marshal(m)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.in.Write(append(b, '\n'))
}

// call sends a request and waits (20 s) for its response.
func (c *testClient) call(method string, params any) (json.RawMessage, *RPCError) {
	c.mu.Lock()
	c.next++
	id := strconv.Itoa(c.next)
	ch := make(chan wireMsg, 1)
	c.waiting[id] = ch
	c.mu.Unlock()
	p, _ := json.Marshal(params)
	c.write(wireMsg{JSONRPC: "2.0", ID: json.RawMessage(id), Method: method, Params: p})
	select {
	case m := <-ch:
		return m.Result, m.Error
	case <-time.After(20 * time.Second):
		c.t.Errorf("%s: no response", method)
		return nil, &RPCError{Message: "timeout"}
	}
}

func (c *testClient) notify(method string, params any) {
	p, _ := json.Marshal(params)
	c.write(wireMsg{JSONRPC: "2.0", Method: method, Params: p})
}

func (c *testClient) updatesOf(kind string) []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []map[string]any
	for _, u := range c.updates {
		if u["sessionUpdate"] == kind {
			out = append(out, u)
		}
	}
	return out
}

// agentText concatenates every agent_message_chunk's text.
func (c *testClient) agentText() string {
	var sb strings.Builder
	for _, u := range c.updatesOf(updAgentMessage) {
		sb.WriteString(u["content"].(map[string]any)["text"].(string))
	}
	return sb.String()
}

func (c *testClient) newSession(cwd string) string {
	c.t.Helper()
	res, err := c.call("session/new", NewSessionRequest{Cwd: cwd, MCPServers: []json.RawMessage{}})
	if err != nil {
		c.t.Fatalf("session/new: %v", err.Message)
	}
	var r NewSessionResponse
	json.Unmarshal(res, &r)
	return r.SessionID
}

// prompt sends a text prompt and returns the stop reason.
func (c *testClient) prompt(sid, text string) (string, *RPCError) {
	res, err := c.call("session/prompt", PromptRequest{SessionID: sid, Prompt: []ContentBlock{{Type: "text", Text: text}}})
	if err != nil {
		return "", err
	}
	var r PromptResponse
	json.Unmarshal(res, &r)
	return r.StopReason, nil
}
```

- [ ] **Step 2: Write the failing server tests**

`internal/acp/server_test.go`:

```go
package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestInitialize(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	res, err := c.call("initialize", InitializeRequest{ProtocolVersion: 1})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r InitializeResponse
	json.Unmarshal(res, &r)
	if r.ProtocolVersion != 1 || r.AgentInfo.Name != "moca" || r.AuthMethods == nil {
		t.Fatalf("%s", res)
	}
	conform(t, "InitializeResponse", r)
}

func TestUnknownMethodAndSession(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	if _, err := c.call("session/nope", map[string]any{}); err == nil || err.Code != codeMethodNotFound {
		t.Fatalf("unknown method: %+v", err)
	}
	if _, err := c.prompt("deadbeef", "hi"); err == nil || err.Code != codeInvalidParams {
		t.Fatalf("unknown session: %+v", err)
	}
}

func TestNewSessionValidatesCwd(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	for _, cwd := range []string{"relative/dir", filepath.Join(t.TempDir(), "missing")} {
		if _, err := c.call("session/new", NewSessionRequest{Cwd: cwd}); err == nil || err.Code != codeInvalidParams {
			t.Errorf("cwd %q: %+v", cwd, err)
		}
	}
}

func TestNewSessionWithoutModel(t *testing.T) {
	o := testOptions(t, newScript(t))
	o.Config.Model = ""
	o.ConfigPath = "/cfg/config.jsonc"
	c := startClient(t, o)
	_, err := c.call("session/new", NewSessionRequest{Cwd: t.TempDir()})
	if err == nil || err.Code != codeAuthRequired || !strings.Contains(err.Message, "moca login") || !strings.Contains(err.Message, "/cfg/config.jsonc") {
		t.Fatalf("%+v", err)
	}
}

func TestPromptStreamsTextAndEndsTurn(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t, textTurn("Hello there."))))
	sid := c.newSession(t.TempDir())
	if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(sid) {
		t.Fatalf("sessionId %q is not a moca id8", sid)
	}
	stop, err := c.prompt(sid, "hi")
	if err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if got := c.agentText(); got != "Hello there." {
		t.Fatalf("agent text %q", got)
	}
	for _, u := range c.updatesOf(updAgentMessage) {
		conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: u})
	}
	if c.updatesOf(updUserMessage) != nil {
		t.Fatal("a live prompt must not echo the user message (the client already shows it)")
	}
	usage := c.updatesOf(updUsage)
	if len(usage) != 1 || usage[0]["size"] != float64(65536) || usage[0]["used"].(float64) <= 0 {
		t.Fatalf("usage_update after the turn: %v", usage)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: usage[0]})
}

func TestPromptToolCall(t *testing.T) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("hello\n"), 0o644)
	c := startClient(t, testOptions(t, newScript(t, toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("Read it."))))
	sid := c.newSession(work)
	if stop, err := c.prompt(sid, "read a.txt"); err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	starts, ends := c.updatesOf(updToolCall), c.updatesOf(updToolUpdate)
	if len(starts) != 1 || len(ends) != 1 {
		t.Fatalf("starts %v ends %v", starts, ends)
	}
	if starts[0]["kind"] != "read" || starts[0]["status"] != statusInProgress || ends[0]["status"] != statusCompleted ||
		starts[0]["toolCallId"] != ends[0]["toolCallId"] {
		t.Fatalf("start %v end %v", starts[0], ends[0])
	}
}

func TestToolCallIDsUniqueAcrossTurns(t *testing.T) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("x\n"), 0o644)
	// Both turns' calls arrive as call_0 from the provider.
	c := startClient(t, testOptions(t, newScript(t,
		toolTurn([2]string{"read", `{"path":"a.txt"}`}),
		toolTurn([2]string{"ls", `{}`}),
		textTurn("ok"))))
	sid := c.newSession(work)
	c.prompt(sid, "go")
	starts := c.updatesOf(updToolCall)
	if len(starts) != 2 || starts[0]["toolCallId"] == starts[1]["toolCallId"] {
		t.Fatalf("tool call ids not unique: %v", starts)
	}
}

func TestProjectUntrustedSkipsProjectFile(t *testing.T) {
	work := t.TempDir()
	os.MkdirAll(filepath.Join(work, ".moca"), 0o755)
	os.WriteFile(filepath.Join(work, ".moca", "config.jsonc"), []byte("{not valid"), 0o644)
	o := testOptions(t, newScript(t, textTurn("ok")))
	o.Trust = func(cwd string) (bool, error) { return cwd != work, nil } // trust is per workdir
	c := startClient(t, o)
	c.newSession(work) // an untrusted workdir's broken project file is ignored entirely
	o2 := testOptions(t, newScript(t))
	c2 := startClient(t, o2)
	if _, err := c2.call("session/new", NewSessionRequest{Cwd: work}); err == nil {
		t.Fatal("a trusted workdir's broken project file must be reported")
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test ./internal/acp -run 'Initialize|Unknown|NewSession|Prompt|ToolCallIDs|Project' -race -count=1`
Expected: FAIL (`undefined: Serve`, `undefined: Options`)

- [ ] **Step 4: Write `server.go`**

```go
package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
)

// Options configures Serve; cmd/moca fills it from the config and flags.
type Options struct {
	Config     config.Config
	ConfigPath string
	Model      string // --model override ("" = config)
	Effort     string // --effort override
	Mode       string // --mode value ("" = resolved per session)
	// Trust decides project trust for a session's workdir (cmd/moca's
	// resolver: flag > yolo > saved trust.json decision > trusted). nil
	// trusts every workdir (tests).
	Trust   func(cwd string) (bool, error)
	Version string
	HTTP       *http.Client // tests
}

// Server maps ACP sessions onto moca agents.
type Server struct {
	opts     Options
	conn     *Conn
	mu       sync.Mutex
	sessions map[string]*session
}

// Serve speaks ACP on r/w until r ends or ctx is done, then closes every
// session's agent (which also stops its MCP servers).
func Serve(ctx context.Context, r io.Reader, w io.Writer, o Options) error {
	srv := &Server{opts: o, sessions: map[string]*session{}}
	srv.conn = NewConn(w, srv)
	err := srv.conn.Serve(ctx, r)
	srv.closeAll()
	return err
}

func (srv *Server) closeAll() {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for id, s := range srv.sessions {
		s.agent.Close()
		delete(srv.sessions, id)
	}
}

func decode[T any](params json.RawMessage) (T, error) {
	var v T
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	if err := json.Unmarshal(params, &v); err != nil {
		return v, invalidParams("invalid params: %v", err)
	}
	return v, nil
}

func (srv *Server) Request(ctx context.Context, method string, params json.RawMessage) (any, error) {
	slog.Debug("acp request", "method", method)
	switch method {
	case "initialize":
		return srv.initialize(params)
	case "authenticate":
		return nil, &RPCError{Code: codeAuthRequired, Message: "moca uses its own credentials: run `moca login <provider>` in a terminal"}
	case "session/new":
		return srv.newSession(params)
	case "session/prompt":
		return srv.prompt(ctx, params)
	}
	return nil, &RPCError{Code: codeMethodNotFound, Message: "method not found: " + method}
}

func (srv *Server) Notify(ctx context.Context, method string, params json.RawMessage) {
	if method != "session/cancel" {
		slog.Debug("acp: ignored notification", "method", method)
		return
	}
	n, err := decode[CancelNotification](params)
	if err != nil {
		return
	}
	if s, ok := srv.get(n.SessionID); ok {
		s.cancelPrompt()
	}
}

func (srv *Server) initialize(params json.RawMessage) (any, error) {
	if _, err := decode[InitializeRequest](params); err != nil {
		return nil, err
	}
	return InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{
			PromptCapabilities: PromptCapabilities{EmbeddedContext: true},
		},
		AuthMethods: []AuthMethod{},
		AgentInfo:   Implementation{Name: "moca", Title: "moca", Version: srv.opts.Version},
	}, nil
}

func (srv *Server) add(s *session) {
	srv.mu.Lock()
	srv.sessions[s.id] = s
	srv.mu.Unlock()
}

func (srv *Server) get(id string) (*session, bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	s, ok := srv.sessions[id]
	return s, ok
}

func (srv *Server) lookup(id string) (*session, error) {
	if s, ok := srv.get(id); ok {
		return s, nil
	}
	return nil, invalidParams("unknown session %q", id)
}

// sessionConfig validates cwd and returns the config for a session there
// (the --model override, and the project file when the workdir is trusted)
// plus the trust decision.
func (srv *Server) sessionConfig(cwd string) (config.Config, bool, error) {
	cfg := srv.opts.Config
	if !filepath.IsAbs(cwd) {
		return cfg, false, invalidParams("cwd must be an absolute path, got %q", cwd)
	}
	if fi, err := os.Stat(cwd); err != nil || !fi.IsDir() {
		return cfg, false, invalidParams("cwd %s is not a directory", cwd)
	}
	if srv.opts.Model != "" {
		cfg.Model = srv.opts.Model
	}
	if cfg.Model == "" {
		return cfg, false, &RPCError{Code: codeAuthRequired, Message: fmt.Sprintf(
			"no model configured: run `moca login <provider>` in a terminal, then set \"model\" in %s (or pass --model to `moca acp`)", srv.opts.ConfigPath)}
	}
	trusted := true
	if srv.opts.Trust != nil {
		t, err := srv.opts.Trust(cwd)
		if err != nil {
			return cfg, false, &RPCError{Code: codeInternal, Message: err.Error()}
		}
		trusted = t
	}
	if trusted {
		p, err := config.LoadProject(cwd)
		if err != nil {
			return cfg, false, &RPCError{Code: codeInternal, Message: err.Error()}
		}
		cfg.Project = p
	}
	return cfg, trusted, nil
}

func (srv *Server) startOptions(cfg config.Config, s *session) agent.StartOptions {
	mode, plan := config.ResolveMode(srv.opts.Mode, cfg)
	return agent.StartOptions{Config: cfg, Workdir: s.cwd, Model: srv.opts.Model, Effort: srv.opts.Effort,
		Trusted: s.trusted, Mode: agent.PermMode(mode), Plan: plan, Ask: s.ask, Emit: s.emit,
		HTTP: srv.opts.HTTP, Slug: "acp"}
}

func (srv *Server) newSession(params json.RawMessage) (any, error) {
	req, err := decode[NewSessionRequest](params)
	if err != nil {
		return nil, err
	}
	cfg, trusted, err := srv.sessionConfig(req.Cwd)
	if err != nil {
		return nil, err
	}
	if len(req.MCPServers) > 0 {
		slog.Debug("acp: client MCP servers ignored", "count", len(req.MCPServers))
	}
	s := newSession(srv, req.Cwd, trusted)
	a, err := agent.Start(srv.startOptions(cfg, s))
	if err != nil {
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	s.attach(a)
	srv.add(s)
	slog.Info("acp session", "session", s.id, "trusted", s.trusted)
	return Deferred{Result: NewSessionResponse{SessionID: s.id}, Then: s.markReady}, nil
}

func (srv *Server) prompt(ctx context.Context, params json.RawMessage) (any, error) {
	req, err := decode[PromptRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	runCtx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer s.end()
	var text string
	for _, b := range req.Prompt {
		if b.Type == "text" {
			text += b.Text
		}
	}
	if text == "" {
		return nil, invalidParams("empty prompt")
	}
	out, err := s.agent.Run(runCtx, text)
	return s.stopReason(runCtx, out, err)
}
```

- [ ] **Step 5: Write `session.go`**

```go
package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// session is one ACP session: a moca agent plus the state that maps its
// events onto session/update notifications.
type session struct {
	srv     *Server
	cwd     string
	trusted bool
	agent   *agent.Agent
	id      string

	// runMu guards cancel; begin/end and the idle-only setters hold it, so
	// set_mode / set_config_option never overlap a running prompt.
	runMu  sync.Mutex
	cancel context.CancelFunc // non-nil while a prompt runs

	// sendMu orders writes and guards ready/early: updates emitted before
	// the client has the session id (agent.Start warnings) are held, then
	// flushed by markReady after the response is written.
	sendMu sync.Mutex
	ready  bool
	early  []any

	// mu guards the event-mapping state, written from the agent's run goroutine.
	mu       sync.Mutex
	msgIDs   map[string]string // update kind → messageId of the current message
	toolSeq  int
	toolIDs  map[string]string // provider call id → ACP toolCallId (open calls)
	curTool  string
	lastStop llm.StopReason
}

func newSession(srv *Server, cwd string, trusted bool) *session {
	return &session{srv: srv, cwd: cwd, trusted: trusted, msgIDs: map[string]string{}, toolIDs: map[string]string{}}
}

// attach binds the started agent; its session id8 is the ACP sessionId.
func (s *session) attach(a *agent.Agent) {
	s.agent = a
	s.id = a.Session().ID8()
}

func (s *session) markReady() {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.ready = true
	for _, u := range s.early {
		s.notify(u)
	}
	s.early = nil
}

func (s *session) send(u any) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if !s.ready {
		s.early = append(s.early, u)
		return
	}
	s.notify(u)
}

func (s *session) notify(u any) {
	if err := s.srv.conn.Notify("session/update", SessionNotification{SessionID: s.id, Update: u}); err != nil {
		slog.Debug("acp: update not sent", "session", s.id, "error", err)
	}
}

// chunk builds a text chunk of kind, keeping one messageId per message.
func (s *session) chunk(kind, text string) ContentChunk {
	s.mu.Lock()
	id := s.msgIDs[kind]
	if id == "" {
		id = newMessageID()
		s.msgIDs[kind] = id
	}
	s.mu.Unlock()
	return ContentChunk{SessionUpdate: kind, Content: ContentBlock{Type: "text", Text: text}, MessageID: id}
}

// nextMessage starts new messages: after a turn, or when a retried stream
// restarts (chunks already sent cannot be retracted).
func (s *session) nextMessage() {
	s.mu.Lock()
	clear(s.msgIDs)
	s.mu.Unlock()
}

// openTool assigns the ACP id for a provider call id. Provider ids repeat
// across turns (openai-compatible servers number calls call_0, call_1, …
// per response); an ACP toolCallId must be unique in the session.
func (s *session) openTool(callID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolSeq++
	id := fmt.Sprintf("%d-%s", s.toolSeq, callID)
	s.toolIDs[callID] = id
	s.curTool = id
	return id
}

// closeTool returns the ACP id of an open call; known=false for a result
// without a start (a cut-off turn's synthetic result), which gets a fresh id.
func (s *session) closeTool(callID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.toolIDs[callID]
	if !ok {
		s.toolSeq++
		id = fmt.Sprintf("%d-%s", s.toolSeq, callID)
	}
	delete(s.toolIDs, callID)
	if s.curTool == id {
		s.curTool = ""
	}
	return id, ok
}

func (s *session) currentTool() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.curTool
}

// emit is the agent's StartOptions.Emit: agent events → session/update.
func (s *session) emit(e agent.Event) {
	switch e := e.(type) {
	case agent.TextDelta:
		if e.Text != "" {
			s.send(s.chunk(updAgentMessage, e.Text))
		}
	case agent.ThinkingDelta:
		if e.Text != "" {
			s.send(s.chunk(updThought, e.Text))
		}
	case agent.StreamReset:
		s.nextMessage()
	case agent.TurnEnd:
		s.mu.Lock()
		s.lastStop = e.Stop
		s.mu.Unlock()
		s.nextMessage()
		// TurnEnd is emitted outside a.mu (agent.go turn()), so Status is safe here.
		if s.agent != nil {
			st := s.agent.Status()
			s.send(UsageUpdate{SessionUpdate: updUsage, Used: st.ContextTokens, Size: st.Window,
				Cost: &Cost{Amount: st.Cost, Currency: "USD"}})
		}
	case agent.ToolStart:
		s.send(startUpdate(s.openTool(e.Call.ID), s.cwd, e.Call))
	case agent.ToolEnd:
		id, known := s.closeTool(e.Call.ID)
		if !known {
			s.send(startUpdate(id, s.cwd, e.Call))
		}
		s.send(endUpdate(id, s.cwd, e.Call, e.Result))
	case agent.Warning:
		s.send(s.chunk(updThought, "moca: "+e.Text+"\n"))
	}
}

// ask is the agent's StartOptions.Ask; Task 6 replaces this body with the
// session/request_permission round-trip.
func (s *session) ask(ctx context.Context, q tools.Question) tools.Answer { return tools.Deny }

// begin marks a prompt as running and returns its cancellable context.
func (s *session) begin(ctx context.Context) (context.Context, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.cancel != nil {
		return nil, busy("a prompt is already running in this session")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Lock()
	s.lastStop = ""
	s.mu.Unlock()
	return ctx, nil
}

func (s *session) end() {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *session) cancelPrompt() {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// idle runs f only while no prompt runs: the agent's mode and model setters
// are between-runs only.
func (s *session) idle(f func() (any, error)) (any, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.cancel != nil {
		return nil, busy("wait for the current prompt to finish")
	}
	return f()
}

// stopReason maps a finished run onto the prompt response (§2 table).
func (s *session) stopReason(ctx context.Context, out agent.Outcome, err error) (any, error) {
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		return PromptResponse{StopReason: stopCancelled}, nil
	case err != nil:
		slog.Info("acp prompt failed", "session", s.id)
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	case out.MaxSteps || out.Stuck:
		return PromptResponse{StopReason: stopMaxTurns}, nil
	}
	s.mu.Lock()
	last := s.lastStop
	s.mu.Unlock()
	switch last {
	case llm.StopLength:
		return PromptResponse{StopReason: stopMaxTokens}, nil
	case llm.StopRefusal:
		return PromptResponse{StopReason: stopRefusal}, nil
	}
	return PromptResponse{StopReason: stopEndTurn}, nil
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): initialize, session/new and session/prompt over moca agents"
```

---

### Task 6: Permissions and cancellation

**Files:**
- Modify: `internal/acp/session.go` (replace the `ask` stub)
- Test: `internal/acp/permission_test.go`

**Interfaces:**
- Consumes: `tools.Question{Kind, Subject, Detail string; CanAlways bool}`, `tools.Answer` (`Deny`, `AllowOnce`, `AllowAlways`), `Conn.Call`, `session.currentTool`.
- Produces: `func (s *session) ask(ctx context.Context, q tools.Question) tools.Answer`

`rm` is in the shell analyser's ask-every-time class (`internal/permissions/shell.go`, `askEvery`). In default mode it always asks, and allow-always is never offered for it (`CanAlways=false`). That makes it a deterministic approval trigger for these tests.

- [ ] **Step 1: Write the failing tests**

`internal/acp/permission_test.go`:

```go
package acp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func rmScript(t *testing.T) (*scriptServer, string) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "x.txt"), []byte("x"), 0o644)
	return newScript(t, toolTurn([2]string{"shell", `{"command":"rm x.txt"}`}), textTurn("done")), work
}

func TestPermissionAllowOnce(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	if stop, err := c.prompt(sid, "delete x"); err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if len(c.perms) != 1 {
		t.Fatalf("permission requests: %d", len(c.perms))
	}
	p := c.perms[0]
	if p.ToolCall.ToolCallID != c.updatesOf(updToolCall)[0]["toolCallId"] {
		t.Errorf("permission not tied to the running tool call: %+v", p.ToolCall)
	}
	kinds := map[string]bool{}
	for _, o := range p.Options {
		kinds[o.Kind] = true
	}
	if !kinds["allow_once"] || !kinds["reject_once"] || kinds["allow_always"] {
		t.Errorf("options %+v (rm must not offer allow_always)", p.Options)
	}
	conform(t, "RequestPermissionRequest", p)
	if _, err := os.Stat(filepath.Join(work, "x.txt")); !os.IsNotExist(err) {
		t.Fatal("approved rm did not run")
	}
}

func TestPermissionDeny(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse { return selected(optReject) }
	sid := c.newSession(work)
	c.prompt(sid, "delete x")
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("denied rm ran")
	}
	if ends := c.updatesOf(updToolUpdate); len(ends) != 1 || ends[0]["status"] != statusFailed {
		t.Fatalf("denied call should fail: %v", ends)
	}
}

func TestCancelDuringPermission(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	second := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.prompt(sid, "again") // a concurrent prompt is refused
		second <- err
		c.notify("session/cancel", CancelNotification{SessionID: sid})
		time.Sleep(50 * time.Millisecond)
		return cancelledOutcome()
	}
	stop, err := c.prompt(sid, "delete x")
	if err != nil || stop != stopCancelled {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if e := <-second; e == nil || e.Code != codeInvalidRequest {
		t.Fatalf("concurrent prompt: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("cancelled rm ran")
	}
}

func TestEOFDuringPermissionDenies(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		c.in.Close() // the editor goes away mid-approval
		select {}
	}
	go c.prompt(sid, "delete x")
	select {
	case <-c.done:
	case <-time.After(10 * time.Second):
		t.Fatal("Serve hung after EOF during a permission request")
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("rm ran without an answer")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'Permission|Cancel|EOF' -race -count=1`
Expected: FAIL (`TestPermissionAllowOnce`: 0 permission requests, the file still exists)

- [ ] **Step 3: Replace the `ask` stub in `session.go`**

```go
// ask is the agent's StartOptions.Ask: a session/request_permission
// round-trip tied to the running tool call. A cancelled outcome, an error
// or a closed connection denies.
func (s *session) ask(ctx context.Context, q tools.Question) tools.Answer {
	id := s.currentTool()
	if id == "" {
		id = s.openTool("ask")
	}
	opts := []PermissionOption{{OptionID: optAllowOnce, Name: "Allow once", Kind: "allow_once"}}
	if q.CanAlways {
		opts = append(opts, PermissionOption{OptionID: optAllowAlways, Name: "Always allow", Kind: "allow_always"})
	}
	opts = append(opts, PermissionOption{OptionID: optReject, Name: "Deny", Kind: "reject_once"})
	raw, _ := json.Marshal(map[string]string{"detail": q.Detail})
	req := RequestPermissionRequest{SessionID: s.id, Options: opts,
		ToolCall: ToolCall{ToolCallID: id, Title: q.Kind + ": " + q.Subject, RawInput: raw}}
	var resp RequestPermissionResponse
	if err := s.srv.conn.Call(ctx, "session/request_permission", req, &resp); err != nil {
		slog.Debug("acp: permission request failed", "session", s.id, "error", err)
		return tools.Deny
	}
	if resp.Outcome.Outcome != "selected" {
		return tools.Deny
	}
	switch resp.Outcome.OptionID {
	case optAllowOnce:
		return tools.AllowOnce
	case optAllowAlways:
		if q.CanAlways {
			return tools.AllowAlways
		}
	}
	return tools.Deny
}
```

Add `"encoding/json"` to `session.go`'s imports.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): permission requests and session/cancel"
```

---

### Task 7: Prompt content and slash commands

**Files:**
- Create: `internal/acp/commands.go`
- Modify: `internal/acp/server.go` (`newSession` response, `prompt` body)
- Modify: `internal/acp/session.go` (fields `prompts`, `commands`)
- Test: `internal/acp/commands_test.go`

**Interfaces:**
- Consumes: `skills.LoadPrompts([]skills.Dir) []skills.Prompt`, `skills.Prompt{Name, Description, ArgumentHint, Path, Body, Source string}`, `skills.Skill{Name, Description, ArgumentHint, Path, Source string}`, `skills.ExpandPrompt(body, args string) string`, `skills.SkillMessage` (Task 3), `config.PromptsDir()`, `(*agent.Agent).Skills()`, `.ReloadSkills()`.
- Produces: `func flatten(cwd string, blocks []ContentBlock) (string, error)`, `func promptDirs(cwd string, trusted bool) []skills.Dir`, `func buildCommands([]skills.Prompt, []skills.Skill) []AvailableCommand`, `func (s *session) reloadCommands() bool`, `func (s *session) sendCommands()`, `func (s *session) expand(text string) string`

- [ ] **Step 1: Write the failing tests**

`internal/acp/commands_test.go`:

````go
package acp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func TestFlatten(t *testing.T) {
	text := "func main() {}"
	got, err := flatten("/w", []ContentBlock{
		{Type: "text", Text: "look at"},
		{Type: "resource_link", URI: "file:///w/cmd/main.go", Name: "main.go"},
		{Type: "text", Text: "and"},
		{Type: "resource_link", URI: "file:///elsewhere/x.go"},
		{Type: "resource", Resource: &EmbeddedResource{URI: "file:///w/a.go", Text: &text}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "look at @cmd/main.go and @/elsewhere/x.go\n\na.go:\n```\nfunc main() {}\n```"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	for _, bad := range []ContentBlock{{Type: "image"}, {Type: "audio"}, {Type: "resource", Resource: &EmbeddedResource{URI: "x", Blob: strp("AAAA")}}} {
		if _, err := flatten("/w", []ContentBlock{bad}); err == nil {
			t.Errorf("%s accepted", bad.Type)
		}
	}
	if _, err := flatten("/w", nil); err == nil {
		t.Error("empty prompt accepted")
	}
}

func TestBuildCommandsPrecedence(t *testing.T) {
	got := buildCommands(
		[]skills.Prompt{{Name: "review", Description: "Review a file", ArgumentHint: "<file>"}},
		[]skills.Skill{{Name: "review", Description: "shadowed"}, {Name: "deploy", Description: "Deploy"}},
	)
	if len(got) != 2 || got[0].Name != "review" || got[0].Input.Hint != "<file>" ||
		got[1].Name != "deploy" || got[1].Input.Hint != "optional request" {
		t.Fatalf("%+v", got)
	}
}

func TestSlashTemplateExpandsAndIsAdvertised(t *testing.T) {
	work := t.TempDir()
	s := newScript(t, textTurn("ok"))
	o := testOptions(t, s) // sets XDG_CONFIG_HOME, so write the template after it
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "moca", "prompts")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "greet.md"), []byte("---\ndescription: Say hi\nargument-hint: <name>\n---\nSay hello to $ARGUMENTS."), 0o644)
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "/greet Ada")
	cmds := c.updatesOf(updCommands)
	if len(cmds) == 0 || !strings.Contains(mustJSON(cmds[0]), `"greet"`) {
		t.Fatalf("commands update: %v", cmds)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: cmds[0]})
	if body := lastUserText(t, s); !strings.Contains(body, "Say hello to Ada.") {
		t.Fatalf("template not expanded; model saw %q", body)
	}
}
````

The last test needs `mustJSON` and `lastUserText`, and `scriptServer` needs to record request bodies for `lastUserText`. Extend `script_test.go`: add a `bodies []map[string]any` field to `scriptServer` and record each body in the handler (`json.Unmarshal` of the read body, as in `internal/agent/agent_test.go`'s `newScript`). Then add:

```go
func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// lastUserText returns the last user message text of the last request.
func lastUserText(t *testing.T, s *scriptServer) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.bodies) == 0 {
		t.Fatal("no request made")
	}
	msgs, _ := s.bodies[len(s.bodies)-1]["messages"].([]any)
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i].(map[string]any)
		if m["role"] == "user" {
			return mustJSON(m["content"])
		}
	}
	return ""
}
```

Replace the handler's `io.Copy(io.Discard, r.Body)` with:

```go
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		s.bodies = append(s.bodies, m)
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'Flatten|BuildCommands|Slash' -race -count=1`
Expected: FAIL (`undefined: flatten`)

- [ ] **Step 3: Write `commands.go`**

````go
package acp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/skills"
)

// flatten turns prompt blocks into one user message: text verbatim, a
// resource_link as an @path mention (the model reads it itself, as with the
// TUI's @), embedded text resources as fenced blocks. Images, audio and
// binary resources are refused (promptCapabilities says so).
func flatten(cwd string, blocks []ContentBlock) (string, error) {
	var sb strings.Builder
	space := func() {
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), " ") && !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteByte(' ')
		}
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if strings.HasPrefix(b.Text, " ") || strings.HasPrefix(b.Text, "\n") {
				sb.WriteString(b.Text)
			} else {
				space()
				sb.WriteString(b.Text)
			}
		case "resource_link":
			space()
			sb.WriteString("@" + mentionPath(cwd, b.URI))
		case "resource":
			if b.Resource == nil || b.Resource.Text == nil {
				return "", invalidParams("binary resources are not supported")
			}
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			fmt.Fprintf(&sb, "%s:\n```\n%s\n```", mentionPath(cwd, b.Resource.URI), strings.TrimRight(*b.Resource.Text, "\n"))
		default:
			return "", invalidParams("%s content is not supported", b.Type)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", invalidParams("empty prompt")
	}
	return sb.String(), nil
}

// mentionPath: a file:// URI as a path, relative to cwd when inside it.
func mentionPath(cwd, uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := filepath.FromSlash(u.Path)
	if rel, err := filepath.Rel(cwd, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return p
}

// promptDirs mirrors the TUI: project templates only for a trusted workdir, then the global ones.
func promptDirs(cwd string, trusted bool) []skills.Dir {
	var d []skills.Dir
	if trusted {
		d = append(d, skills.Dir{Path: filepath.Join(cwd, ".moca", "prompts"), Source: "project"})
	}
	return append(d, skills.Dir{Path: config.PromptsDir(), Source: "global"})
}

// buildCommands lists prompt templates, then skills; the first owner of a
// name wins (the TUI's precedence, minus its built-ins).
func buildCommands(prompts []skills.Prompt, sk []skills.Skill) []AvailableCommand {
	seen := map[string]bool{}
	out := []AvailableCommand{}
	add := func(name, desc, hint string) {
		if seen[name] {
			return
		}
		seen[name] = true
		c := AvailableCommand{Name: name, Description: desc}
		if hint != "" {
			c.Input = &AvailableCommandInput{Hint: hint}
		}
		out = append(out, c)
	}
	for _, p := range prompts {
		add(p.Name, p.Description, p.ArgumentHint)
	}
	for _, k := range sk {
		h := k.ArgumentHint
		if h == "" {
			h = "optional request"
		}
		add(k.Name, k.Description, h)
	}
	return out
}

// reloadCommands re-reads templates and skills; it reports whether the
// advertised list changed. Called only from the prompt handler (after
// begin) and before the session is published.
func (s *session) reloadCommands() bool {
	s.agent.ReloadSkills()
	s.prompts = skills.LoadPrompts(promptDirs(s.cwd, s.trusted))
	cmds := buildCommands(s.prompts, s.agent.Skills())
	changed := !reflect.DeepEqual(cmds, s.commands)
	s.commands = cmds
	return changed
}

func (s *session) sendCommands() {
	s.send(AvailableCommandsUpdate{SessionUpdate: updCommands, AvailableCommands: s.commands})
}

// expand applies a leading /name the TUI's way: a template expands, a skill
// becomes its load-and-follow message, `//` escapes; anything else is sent
// as written (the model sees an unknown /word as text).
func (s *session) expand(text string) string {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "//") {
		return t[1:]
	}
	if !strings.HasPrefix(t, "/") {
		return text
	}
	name, args := t[1:], ""
	if i := strings.IndexFunc(name, unicode.IsSpace); i >= 0 {
		name, args = name[:i], strings.TrimSpace(name[i:])
	}
	for _, p := range s.prompts {
		if p.Name == name {
			return skills.ExpandPrompt(p.Body, args)
		}
	}
	for _, k := range s.agent.Skills() {
		if k.Name == name {
			return skills.SkillMessage(k, args)
		}
	}
	return text
}
````

- [ ] **Step 4: Wire it in**

In `session.go`, add the fields to `session` (after `trusted`):

```go
	prompts  []skills.Prompt         // prompt templates (reloaded after each prompt)
	commands []AvailableCommand      // the advertised list
```

with the `"github.com/adeotek/moca/internal/skills"` import.

In `server.go` `newSession`, replace the final `return Deferred{…}` with:

```go
	s.reloadCommands()
	return Deferred{Result: NewSessionResponse{SessionID: s.id}, Then: func() {
		s.markReady()
		s.sendCommands()
	}}, nil
```

In `server.go` `prompt`, replace everything from `var text string` up to and including `out, err := s.agent.Run(runCtx, text)` with:

```go
	text, err := flatten(s.cwd, req.Prompt)
	if err != nil {
		return nil, err
	}
	out, err := s.agent.Run(runCtx, s.expand(text))
	if s.reloadCommands() {
		s.sendCommands()
	}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): prompt content blocks and slash commands"
```

---

### Task 8: Modes and the model picker

**Files:**
- Create: `internal/acp/modes.go`
- Modify: `internal/acp/server.go` (`Request` cases, `newSession` response)
- Modify: `internal/acp/session.go` (`emit` cases)
- Test: `internal/acp/modes_test.go`

**Interfaces:**
- Consumes: `(*agent.Agent).Mode() agent.PermMode`, `.Plan() bool`, `.SetMode(agent.PermMode)`, `.SetPlan(bool)`, `.Models() []provider.Model`, `.Model() provider.Model`, `.HasCredential(string) bool`, `.SetModel(string, llm.Effort) error`; `provider.Model.Qualified() string`; events `agent.ModeChanged{Mode, Plan}`, `agent.PlanChanged{On}`; `session.idle`.
- Produces: `var sessionModes []SessionMode`, `func (s *session) modeID() string`, `func (s *session) modeState() *SessionModeState`, `func (s *session) reportMode()`, `func (s *session) muteMode(bool)`, `func (s *session) configOptions() []ConfigOption`, `func (srv *Server) setMode(json.RawMessage) (any, error)`, `func (srv *Server) setConfigOption(json.RawMessage) (any, error)`

- [ ] **Step 1: Write the failing tests**

`internal/acp/modes_test.go`:

```go
package acp

import (
	"encoding/json"
	"testing"
)

func newSessionFull(t *testing.T, c *testClient, cwd string) NewSessionResponse {
	t.Helper()
	res, err := c.call("session/new", NewSessionRequest{Cwd: cwd, MCPServers: []json.RawMessage{}})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r NewSessionResponse
	json.Unmarshal(res, &r)
	return r
}

func TestNewSessionAdvertisesModesAndModels(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	r := newSessionFull(t, c, t.TempDir())
	conform(t, "NewSessionResponse", r)
	if r.Modes == nil || r.Modes.CurrentModeID != "default" || len(r.Modes.AvailableModes) != 4 {
		t.Fatalf("modes %+v", r.Modes)
	}
	if len(r.ConfigOptions) != 1 || r.ConfigOptions[0].ID != "model" || r.ConfigOptions[0].CurrentValue != "fake/m" ||
		len(r.ConfigOptions[0].Options) != 2 {
		t.Fatalf("config options %+v", r.ConfigOptions)
	}
}

func TestModeFlagSetsInitialMode(t *testing.T) {
	o := testOptions(t, newScript(t))
	o.Mode = "plan"
	c := startClient(t, o)
	if r := newSessionFull(t, c, t.TempDir()); r.Modes.CurrentModeID != "plan" {
		t.Fatalf("mode %q", r.Modes.CurrentModeID)
	}
}

func TestSetMode(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	for _, m := range []string{"plan", "auto", "yolo", "default"} {
		if _, err := c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: m}); err != nil {
			t.Fatalf("%s: %v", m, err.Message)
		}
		ups := c.updatesOf(updCurrentMode)
		if len(ups) == 0 || ups[len(ups)-1]["currentModeId"] != m {
			t.Fatalf("after %s: %v", m, ups)
		}
	}
	if _, err := c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "turbo"}); err == nil || err.Code != codeInvalidParams {
		t.Fatalf("bogus mode: %+v", err)
	}
}

func TestSetModel(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	res, err := c.call("session/set_config_option", map[string]any{"sessionId": sid, "configId": "model", "value": "fake/m2"})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r SetConfigOptionResponse
	json.Unmarshal(res, &r)
	if r.ConfigOptions[0].CurrentValue != "fake/m2" {
		t.Fatalf("%+v", r)
	}
	for _, bad := range []map[string]any{
		{"sessionId": sid, "configId": "model", "value": "fake/nope"},
		{"sessionId": sid, "configId": "effort", "value": "high"},
	} {
		if _, err := c.call("session/set_config_option", bad); err == nil || err.Code != codeInvalidParams {
			t.Errorf("%v: %+v", bad, err)
		}
	}
}

func TestSetModeSendsOneUpdate(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "plan"})
	before := len(c.updatesOf(updCurrentMode))
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "auto"})
	ups := c.updatesOf(updCurrentMode)[before:]
	if len(ups) != 1 || ups[0]["currentModeId"] != "auto" {
		t.Fatalf("plan→auto must be one update to auto, got %v", ups)
	}
	before = len(c.updatesOf(updCurrentMode))
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "auto"})
	if n := len(c.updatesOf(updCurrentMode)) - before; n != 0 {
		t.Fatalf("an unchanged mode sent %d updates", n)
	}
}

func TestSetModeRefusedWhileRunning(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	got := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "yolo"})
		got <- err
		return selected(optReject)
	}
	c.prompt(sid, "delete x")
	if err := <-got; err == nil || err.Code != codeInvalidRequest {
		t.Fatalf("mid-run set_mode: %+v", err)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'Modes|ModeFlag|SetMode|SetModel' -race -count=1`
Expected: FAIL (`Modes` is nil; `session/set_mode` is method not found)

- [ ] **Step 3: Write `modes.go`**

```go
package acp

import (
	"encoding/json"
	"slices"

	"github.com/adeotek/moca/internal/agent"
)

// sessionModes are moca's permission modes plus the plan scope (which rides
// on default), worded as the TUI's mode badge and notices.
var sessionModes = []SessionMode{
	{ID: "default", Name: "Default", Description: "Allowlisted actions run; everything else asks"},
	{ID: "auto", Name: "Auto", Description: "Safe actions run, risky ones ask, unsure ones get a classifier's opinion"},
	{ID: "yolo", Name: "Yolo", Description: "All permission checks are off"},
	{ID: "plan", Name: "Plan", Description: "Write an implementation plan to docs/plans/; no other writes"},
}

func (s *session) modeID() string {
	if s.agent.Plan() {
		return "plan"
	}
	return string(s.agent.Mode())
}

// modeState builds the modes block of a session/new or session/load
// response and records the id the client is told.
func (s *session) modeState() *SessionModeState {
	id := s.modeID()
	s.mu.Lock()
	s.lastMode = id
	s.mu.Unlock()
	return &SessionModeState{CurrentModeID: id, AvailableModes: sessionModes}
}

// reportMode sends current_mode_update when the mode id differs from the
// last one the client saw. SetMode from plan to auto emits PlanChanged
// (mode still default) and then ModeChanged, so set_mode mutes the events
// while it runs and reports once; agent-side changes report directly.
func (s *session) reportMode() {
	id := s.modeID()
	s.mu.Lock()
	if s.modeMuted || id == s.lastMode {
		s.mu.Unlock()
		return
	}
	s.lastMode = id
	s.mu.Unlock()
	s.send(CurrentModeUpdate{SessionUpdate: updCurrentMode, CurrentModeID: id})
}

func (s *session) muteMode(on bool) {
	s.mu.Lock()
	s.modeMuted = on
	s.mu.Unlock()
}

// configOptions offers the models whose credential is available (the
// current one always) as the "model" select.
func (s *session) configOptions() []ConfigOption {
	cur := s.agent.Model().Qualified()
	var vals []ConfigOptionValue
	for _, m := range s.agent.Models() {
		q := m.Qualified()
		if q == cur || s.agent.HasCredential(q) {
			vals = append(vals, ConfigOptionValue{Value: q, Name: q})
		}
	}
	if !slices.ContainsFunc(vals, func(v ConfigOptionValue) bool { return v.Value == cur }) {
		vals = append([]ConfigOptionValue{{Value: cur, Name: cur}}, vals...)
	}
	return []ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: cur, Options: vals}}
}

func (srv *Server) setMode(params json.RawMessage) (any, error) {
	req, err := decode[SetModeRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(sessionModes, func(m SessionMode) bool { return m.ID == req.ModeID }) {
		return nil, invalidParams("unknown mode %q", req.ModeID)
	}
	return s.idle(func() (any, error) {
		s.muteMode(true)
		if req.ModeID == "plan" {
			s.agent.SetPlan(true)
		} else {
			if s.agent.Plan() {
				s.agent.SetPlan(false)
			}
			s.agent.SetMode(agent.PermMode(req.ModeID))
		}
		s.muteMode(false)
		s.reportMode()
		return struct{}{}, nil
	})
}

func (srv *Server) setConfigOption(params json.RawMessage) (any, error) {
	req, err := decode[SetConfigOptionRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	if req.ConfigID != "model" {
		return nil, invalidParams("unknown config option %q", req.ConfigID)
	}
	var value string
	if err := json.Unmarshal(req.Value, &value); err != nil {
		return nil, invalidParams("model value must be a string")
	}
	return s.idle(func() (any, error) {
		if err := s.agent.SetModel(value, ""); err != nil {
			return nil, invalidParams("%v", err)
		}
		return SetConfigOptionResponse{ConfigOptions: s.configOptions()}, nil
	})
}
```

- [ ] **Step 4: Wire it in**

In `server.go` `Request`, add these cases before the closing method-not-found return:

```go
	case "session/set_mode":
		return srv.setMode(params)
	case "session/set_config_option":
		return srv.setConfigOption(params)
```

In `newSession`, change the response to `NewSessionResponse{SessionID: s.id, Modes: s.modeState(), ConfigOptions: s.configOptions()}`.

In `session.go` `emit`, add:

```go
	case agent.ModeChanged, agent.PlanChanged:
		// nil while agent.Start applies the initial mode: the response carries
		// it. Muted while session/set_mode runs its setters (it reports once).
		if s.agent != nil {
			s.reportMode()
		}
```

In `session.go`, add these fields to `session` (in the `mu`-guarded group):

```go
	lastMode   string // the mode id the client was last told
	modeMuted  bool   // set_mode is applying setters: report once at the end
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS. If `TestSetModel`'s bad-value case passes through `SetModel` without error, check `config.SplitModel`/registry resolution in `SetModel`. An unresolvable `fake/nope` must fail, because `setModel` resolves it through the registry.

- [ ] **Step 6: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): session modes and the model config option"
```

---

### Task 9: Session list, load and history replay

**Files:**
- Create: `internal/acp/history.go`
- Modify: `internal/acp/server.go` (`Request` cases, `initialize` capabilities)
- Test: `internal/acp/history_test.go`

**Interfaces:**
- Consumes: `session.List(dir, workdir, skipID8 string, limit int) []session.Info` (`Info{Path, ID8 string; Modified time.Time; Workdir, Preview string}`; `workdir ""` = any), `session.Find(dir, ref, workdir string) (string, error)`, `session.InWorkdir(path, wd string) bool`, `session.SummaryPrefix` (Task 3), `agent.Resume(agent.StartOptions, path string) (*agent.Agent, error)`, `(*agent.Agent).History() []llm.Message`, `llm.Message{Role; Content []llm.ContentBlock}`, block types `llm.BlockText/BlockThinking/BlockToolUse/BlockToolResult`, `config.DataDir()`.
- Produces: `func sessionsDir() string`, `func (srv *Server) listSessions(json.RawMessage) (any, error)`, `func (srv *Server) loadSession(json.RawMessage) (any, error)`, `func (s *session) replay()`

- [ ] **Step 1: Write the failing tests**

`internal/acp/history_test.go`:

```go
package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListAndLoad(t *testing.T) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("x\n"), 0o644)
	s := newScript(t, toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("All good."))
	o := testOptions(t, s)
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "check a.txt please")

	res, err := c.call("session/list", ListSessionsRequest{Cwd: work})
	if err != nil {
		t.Fatal(err.Message)
	}
	var l ListSessionsResponse
	json.Unmarshal(res, &l)
	conform(t, "ListSessionsResponse", l)
	if len(l.Sessions) != 1 || l.Sessions[0].SessionID != sid || !strings.Contains(l.Sessions[0].Title, "check a.txt") {
		t.Fatalf("list %+v", l)
	}

	// A fresh process (new client, same data dir) reopens it.
	c2 := startClient(t, o)
	res, err = c2.call("session/load", LoadSessionRequest{SessionID: sid, Cwd: work, MCPServers: []json.RawMessage{}})
	if err != nil {
		t.Fatal(err.Message)
	}
	var lr LoadSessionResponse
	json.Unmarshal(res, &lr)
	conform(t, "LoadSessionResponse", lr)
	users := c2.updatesOf(updUserMessage)
	if len(users) != 1 || !strings.Contains(mustJSON(users[0]), "check a.txt please") {
		t.Fatalf("user replay %v", users)
	}
	if c2.agentText() != "All good." {
		t.Fatalf("agent replay %q", c2.agentText())
	}
	if starts, ends := c2.updatesOf(updToolCall), c2.updatesOf(updToolUpdate); len(starts) != 1 || len(ends) != 1 || ends[0]["status"] != statusCompleted {
		t.Fatalf("tool replay %v / %v", starts, ends)
	}
	// The loaded session takes prompts.
	s.mu.Lock()
	s.turns = append(s.turns, textTurn("again"))
	s.mu.Unlock()
	if stop, err := c2.prompt(sid, "more"); err != nil || stop != stopEndTurn {
		t.Fatalf("prompt after load: %q %+v", stop, err)
	}
	// The resumed conversation reaches the model: the follow-up request
	// carries the first prompt.
	s.mu.Lock()
	last := mustJSON(s.bodies[len(s.bodies)-1]["messages"])
	s.mu.Unlock()
	if !strings.Contains(last, "check a.txt please") {
		t.Fatalf("follow-up request lost the loaded history: %s", last)
	}
}

// The load response must come after the last replayed update: a client
// that renders on the response would otherwise miss the tail.
func TestLoadRespondsAfterReplay(t *testing.T) {
	work := t.TempDir()
	o := testOptions(t, newScript(t, textTurn("first answer")))
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "hello")
	c2 := startClient(t, o)
	if _, err := c2.call("session/load", LoadSessionRequest{SessionID: sid, Cwd: work, MCPServers: []json.RawMessage{}}); err != nil {
		t.Fatal(err.Message)
	}
	// call returns when the response arrives; reading is sequential, so any
	// update recorded by then was written before the response.
	if c2.agentText() != "first answer" {
		t.Fatalf("replay incomplete at response time: %q", c2.agentText())
	}
}

func TestLoadRejectsNonID8(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	for _, id := range []string{"last", "abc", "../../etc"} {
		if _, err := c.call("session/load", LoadSessionRequest{SessionID: id, Cwd: t.TempDir()}); err == nil || err.Code != codeInvalidParams {
			t.Errorf("%q: %+v", id, err)
		}
	}
}

func TestLoadFromOtherWorkdirRefused(t *testing.T) {
	work := t.TempDir()
	o := testOptions(t, newScript(t, textTurn("ok")))
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "hello")
	c2 := startClient(t, o)
	if _, err := c2.call("session/load", LoadSessionRequest{SessionID: sid, Cwd: t.TempDir()}); err == nil {
		t.Fatal("loaded a session into another workdir")
	}
}

func TestInitializeAdvertisesLoadAndList(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	res, _ := c.call("initialize", InitializeRequest{ProtocolVersion: 1})
	var r InitializeResponse
	json.Unmarshal(res, &r)
	if !r.AgentCapabilities.LoadSession || r.AgentCapabilities.SessionCapabilities.List == nil {
		t.Fatalf("%s", res)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'List|Load|Advertises' -race -count=1`
Expected: FAIL (method not found / capability false)

- [ ] **Step 3: Write `history.go`**

```go
package acp

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	msession "github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

// id8 is the only sessionId form load accepts: msession.Find also takes
// "last", which must never reach it from a client.
var id8 = regexp.MustCompile(`^[0-9a-f]{8}$`)

func sessionsDir() string { return filepath.Join(config.DataDir(), "sessions") }

func (srv *Server) listSessions(params json.RawMessage) (any, error) {
	req, err := decode[ListSessionsRequest](params)
	if err != nil {
		return nil, err
	}
	if req.Cwd != "" && !filepath.IsAbs(req.Cwd) {
		return nil, invalidParams("cwd must be an absolute path, got %q", req.Cwd)
	}
	out := ListSessionsResponse{Sessions: []SessionInfo{}}
	for _, in := range msession.List(sessionsDir(), req.Cwd, "", 100) {
		out.Sessions = append(out.Sessions, SessionInfo{SessionID: in.ID8, Cwd: in.Workdir, Title: in.Preview,
			UpdatedAt: in.Modified.UTC().Format(time.RFC3339)})
	}
	return out, nil
}

func (srv *Server) loadSession(params json.RawMessage) (any, error) {
	req, err := decode[LoadSessionRequest](params)
	if err != nil {
		return nil, err
	}
	if !id8.MatchString(req.SessionID) {
		return nil, invalidParams("sessionId must be a moca session id (8 hex chars), got %q", req.SessionID)
	}
	if s, ok := srv.get(req.SessionID); ok {
		return s.idle(func() (any, error) {
			s.replay()
			s.sendCommands()
			return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
		})
	}
	cfg, trusted, err := srv.sessionConfig(req.Cwd)
	if err != nil {
		return nil, err
	}
	path, err := msession.Find(sessionsDir(), req.SessionID, req.Cwd)
	if err != nil {
		return nil, invalidParams("%v", err)
	}
	if !msession.InWorkdir(path, req.Cwd) {
		return nil, invalidParams("session %s belongs to another directory (%s)", req.SessionID, msession.WorkdirOf(path))
	}
	s := newSession(srv, req.Cwd, trusted)
	a, err := agent.Resume(srv.startOptions(cfg, s), path)
	if err != nil {
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	s.attach(a)
	s.reloadCommands()
	srv.add(s)
	s.markReady() // the client named this id: updates may flow now
	s.replay()
	s.sendCommands()
	return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
}

// replay streams the whole conversation as the model sees it (after any
// compaction summary): user and agent text, thinking, and each tool call
// with its recorded result. ACP clients rebuild their view from it.
func (s *session) replay() {
	msgs := s.agent.History()
	results := map[string]llm.ToolResult{}
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == llm.BlockToolResult && b.ToolResult != nil {
				results[b.ToolResult.CallID] = *b.ToolResult
			}
		}
	}
	for _, m := range msgs {
		for _, b := range m.Content {
			switch {
			case b.Type == llm.BlockText && m.Role == llm.RoleUser:
				if strings.HasPrefix(strings.TrimSpace(b.Text), msession.SummaryPrefix) {
					s.send(s.chunk(updAgentMessage, "_(earlier conversation was summarized)_\n\n"))
				} else {
					s.send(s.chunk(updUserMessage, b.Text))
				}
			case b.Type == llm.BlockText:
				s.send(s.chunk(updAgentMessage, b.Text))
			case b.Type == llm.BlockThinking && b.Text != "":
				s.send(s.chunk(updThought, b.Text))
			case b.Type == llm.BlockToolUse && b.ToolCall != nil:
				id := s.openTool(b.ToolCall.ID)
				s.send(startUpdate(id, s.cwd, *b.ToolCall))
				res := tools.Result{Content: "no result recorded", IsError: true}
				if r, ok := results[b.ToolCall.ID]; ok {
					res = tools.Result{Content: r.Content, IsError: r.IsError}
				}
				s.closeTool(b.ToolCall.ID)
				s.send(endUpdate(id, s.cwd, *b.ToolCall, res))
			}
		}
		s.nextMessage()
	}
}
```

Check before relying on it: `session.WorkdirOf` exists (`cmd/moca/oneshot.go` uses it). `config.DataDir` is used by `cmd/moca/oneshot.go`.

- [ ] **Step 4: Wire it in**

In `server.go` `Request`, add:

```go
	case "session/list":
		return srv.listSessions(params)
	case "session/load":
		return srv.loadSession(params)
```

In `initialize`, set `LoadSession: true` and `SessionCapabilities: SessionCapabilities{List: &struct{}{}}` inside `AgentCapabilities`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): session/list and session/load with full history replay"
```

---

### Task 10: Request cancellation (`$/cancel_request`) and ACP error codes

ACP v1 stabilized request cancellation (`docs/protocol/v1/cancellation.mdx` at the pinned tag). The receiver of `$/cancel_request` may stop the named request's work and **must** still answer it: either with a valid result (for a prompt, `stopReason: "cancelled"`) or with error `-32800`. Cancellation also cascades. When a prompt is cancelled, the agent sends `$/cancel_request` for its own requests still pending at the client, which here means a permission prompt nobody answered. ACP also defines `-32002` (resource not found) for unknown ids, where both plans used `-32602`.

**Files:**
- Modify: `internal/acp/rpc.go` (codes, per-request contexts, `$/cancel_request` in and out)
- Modify: `internal/acp/server.go` (`lookup` → `-32002`), `internal/acp/history.go` (`session.Find` misses → `-32002`), `internal/acp/protocol.go` (`acpMethods`)
- Modify: `internal/acp/client_test.go` (record `$/cancel_request`), `internal/acp/server_test.go` (`TestUnknownMethodAndSession` code)
- Test: `internal/acp/rpc_test.go`, `internal/acp/permission_test.go`, `internal/acp/history_test.go`

**Interfaces:**
- Consumes: Task 1 `Conn`, `message`, `serveRequest`, `Call`; Task 5 `lookup`; Task 9 `loadSession`.
- Produces: constants `codeRequestCancelled = -32800`, `codeResourceNotFound = -32002`; `func notFound(format string, a ...any) error`; `Conn.inflight map[string]context.CancelFunc`; `func (c *Conn) cancelInflight(params json.RawMessage)`; `testClient.cancels []json.RawMessage`.

- [ ] **Step 1: Write the failing tests**

In `internal/acp/rpc_test.go`, add a `"wait"` case to `echoHandler.Request` (before `"deferred"`):

```go
	case "wait":
		<-ctx.Done()
		return nil, ctx.Err()
```

and append:

```go
func TestCancelRequest(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, `{"jsonrpc":"2.0","id":5,"method":"wait"}`+"\n")
	io.WriteString(in, `{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":99}}`+"\n") // unknown: ignored
	io.WriteString(in, `{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":5}}`+"\n")
	m := readMsg(t, sc)
	if m["id"] != float64(5) || m["error"].(map[string]any)["code"] != float64(codeRequestCancelled) {
		t.Fatalf("a cancelled request must answer -32800: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":"s","method":"echo","params":1}`+"\n")
	if m := readMsg(t, sc); m["id"] != "s" {
		t.Fatalf("connection unusable after a cancel: %v", m)
	}
}

func TestCallCascadesCancel(t *testing.T) {
	c, _, sc, _ := pipePeer(t, &echoHandler{})
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan error, 1)
	go func() { got <- c.Call(ctx, "client/thing", nil, nil) }()
	req := readMsg(t, sc)
	cancel()
	m := readMsg(t, sc)
	if m["method"] != "$/cancel_request" || m["params"].(map[string]any)["requestId"] != req["id"] {
		t.Fatalf("want $/cancel_request for id %v, got %v", req["id"], m)
	}
	if err := <-got; !errors.Is(err, context.Canceled) {
		t.Fatalf("Call: %v", err)
	}
}
```

In `internal/acp/client_test.go`, add the field `cancels []json.RawMessage // requestIds of $/cancel_request from the agent` to `testClient`, and a case to `read`'s switch:

```go
		case "$/cancel_request":
			var p struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			json.Unmarshal(m.Params, &p)
			c.mu.Lock()
			c.cancels = append(c.cancels, p.RequestID)
			c.mu.Unlock()
```

Append to `internal/acp/permission_test.go`:

```go
// session/cancel while the editor sits on a permission prompt: the agent
// must not wait for the answer, and must tell the client to drop the prompt.
func TestSessionCancelCascadesToPermission(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		c.notify("session/cancel", CancelNotification{SessionID: sid})
		<-release // the editor never answers
		return cancelledOutcome()
	}
	stop, err := c.prompt(sid, "delete x")
	if err != nil || stop != stopCancelled {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	c.mu.Lock()
	n := len(c.cancels)
	c.mu.Unlock()
	if n != 1 {
		t.Fatalf("want one $/cancel_request for the pending permission, got %d", n)
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("cancelled rm ran")
	}
}
```

Ordering makes this deterministic. `Call` writes `$/cancel_request` before `ask` returns `Deny`, so it precedes the prompt response on the wire, and the client reads lines in order.

In `internal/acp/server_test.go` `TestUnknownMethodAndSession`, change the unknown-session expectation from `codeInvalidParams` to `codeResourceNotFound`. Append to `internal/acp/history_test.go`:

```go
func TestLoadUnknownSessionNotFound(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	if _, err := c.call("session/load", LoadSessionRequest{SessionID: "deadbeef", Cwd: t.TempDir()}); err == nil || err.Code != codeResourceNotFound {
		t.Fatalf("%+v", err)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'CancelRequest|CallCascades|SessionCancelCascades|UnknownMethodAndSession|LoadUnknownSession' -race -count=1`
Expected: FAIL (`undefined: codeRequestCancelled`, `undefined: codeResourceNotFound`)

- [ ] **Step 3: Implement**

In `rpc.go`, extend the code constants and add the helper:

```go
	codeResourceNotFound = -32002 // ACP: e.g. an unknown sessionId
	codeRequestCancelled = -32800 // ACP: answered after $/cancel_request
```

```go
func notFound(format string, a ...any) error {
	return &RPCError{Code: codeResourceNotFound, Message: fmt.Sprintf(format, a...)}
}
```

Add `inflight map[string]context.CancelFunc // incoming request id → its cancel (guarded by mu)` to `Conn`, and initialize it in `NewConn`: `inflight: map[string]context.CancelFunc{}`.

In `dispatch`, handle the protocol notification before the request case (it never reaches the `Handler`):

```go
	switch {
	case m.Method == "$/cancel_request" && len(m.ID) == 0:
		c.cancelInflight(m.Params)
	case m.Method != "" && len(m.ID) > 0:
```

Register each request's cancel func **in `dispatch`, before the goroutine starts**. If the handler goroutine registered it, a `$/cancel_request` arriving right after the request could be processed first (notifications run inline on the read loop) and be ignored. Replace the request case:

```go
	case m.Method != "" && len(m.ID) > 0:
		reqCtx, cancel := context.WithCancel(ctx)
		key := string(bytes.TrimSpace(m.ID))
		c.mu.Lock()
		c.inflight[key] = cancel
		c.mu.Unlock()
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			defer func() {
				c.mu.Lock()
				delete(c.inflight, key)
				c.mu.Unlock()
				cancel()
			}()
			c.serveRequest(reqCtx, m)
		}()
```

Then replace the head of `serveRequest`, up to and including the error mapping (`ctx` is now the per-request context):

```go
func (c *Conn) serveRequest(ctx context.Context, m message) {
	res, err := c.h.Request(ctx, m.Method, m.Params)
	var then func()
	if d, ok := res.(Deferred); ok {
		res, then = d.Result, d.Then
	}
	out := message{ID: m.ID}
	if err != nil {
		var re *RPCError
		switch {
		case errors.As(err, &re):
		case ctx.Err() != nil && errors.Is(err, context.Canceled):
			// $/cancel_request or shutdown: ACP's "request cancelled".
			re = &RPCError{Code: codeRequestCancelled, Message: "request cancelled"}
		default:
			re = &RPCError{Code: codeInternal, Message: err.Error()}
		}
		out.Error = re
	} else if b, merr := json.Marshal(res); merr != nil {
```

(the rest of `serveRequest` is unchanged). Then add:

```go
// cancelInflight serves $/cancel_request: the named request's context is
// cancelled and its handler still answers — a result such as stopReason
// "cancelled", or -32800. Unknown and finished ids are ignored.
func (c *Conn) cancelInflight(params json.RawMessage) {
	var p struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	c.mu.Lock()
	cancel := c.inflight[string(bytes.TrimSpace(p.RequestID))]
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
```

In `Call`, replace the `ctx.Done()` branch:

```go
	case <-ctx.Done():
		// Cascade the cancellation (ACP): the client may drop its UI for this
		// request; a late answer finds no pending entry and is ignored.
		c.Notify("$/cancel_request", map[string]int64{"requestId": id})
		return ctx.Err()
```

`session/prompt` needs no change. Its run context already derives from the request context, so `$/cancel_request` on a prompt ends the run and the prompt answers `stopReason: "cancelled"`, which the spec allows.

In `server.go` `lookup`, return `notFound("unknown session %q", id)`. In `history.go` `loadSession`, the `session.Find` failure becomes `return nil, notFound("%v", err)`: after the id8 check, a `Find` error can only mean "no such session". Append `"$/cancel_request"` to `acpMethods` in `protocol.go`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): \$/cancel_request both ways, -32800 and -32002"
```

---

### Task 11: Session lifecycle: `session/close`, `session/resume`, `session/delete`

All three methods are stable in v1, and each is advertised by an empty object in `sessionCapabilities` (`close`, `resume`, `delete`).
- `session/close` must behave as `session/cancel` for running work, then free the session.
- `session/resume` is `session/load` without the history replay.
- `session/delete` removes a session that appears in `session/list`. moca already has `session.Delete(path)`, which refuses (`session.ErrInUse`) a file that a live writer holds, including this process's own open agent.

**Files:**
- Modify: `internal/acp/protocol.go` (types, capabilities, `acpMethods`), `internal/acp/session.go` (closed state, run wait group), `internal/acp/server.go` (method table, `remove`, `initialize`), `internal/acp/history.go` (`reopen`, `resumeSession`, `deleteSession`, `closeSession`)
- Test: `internal/acp/lifecycle_test.go`, `internal/acp/protocol_test.go`

**Interfaces:**
- Consumes: Task 9 `loadSession` body, `id8`, `sessionsDir`; Task 10 `notFound`; `session.Delete(path string) error`, `session.ErrInUse`; `(*agent.Agent).Close() error`.
- Produces: `ResumeSessionRequest`, `ResumeSessionResponse`, `CloseSessionRequest`, `DeleteSessionRequest`; `SessionCapabilities{List, Delete, Resume, Close *struct{}}`; `func (srv *Server) remove(id string)`; `func (s *session) shutdown()`; `func (srv *Server) reopen(id, cwd string, replay bool) (LoadSessionResponse, error)`; `closeSession`, `resumeSession`, `deleteSession` handlers.

- [ ] **Step 1: Write the failing tests**

Append to `TestProtocolShapesConform` in `protocol_test.go`:

```go
	conform(t, "InitializeResponse", InitializeResponse{ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{LoadSession: true, SessionCapabilities: SessionCapabilities{
			List: &struct{}{}, Delete: &struct{}{}, Resume: &struct{}{}, Close: &struct{}{}}},
		AuthMethods: []AuthMethod{}, AgentInfo: Implementation{Name: "moca", Version: "1"}})
	conform(t, "ResumeSessionResponse", ResumeSessionResponse{})
```

`internal/acp/lifecycle_test.go`:

```go
package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeAdvertisesLifecycle(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	res, _ := c.call("initialize", InitializeRequest{ProtocolVersion: 1})
	var r InitializeResponse
	json.Unmarshal(res, &r)
	sc := r.AgentCapabilities.SessionCapabilities
	if sc.Close == nil || sc.Resume == nil || sc.Delete == nil || sc.List == nil {
		t.Fatalf("%s", res)
	}
}

// session/close mid-prompt: the prompt ends as cancelled, the session is gone.
func TestCloseCancelsAndFrees(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	closed := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.call("session/close", CloseSessionRequest{SessionID: sid})
		closed <- err
		return cancelledOutcome()
	}
	stop, err := c.prompt(sid, "delete x")
	if err != nil || stop != stopCancelled {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if e := <-closed; e != nil {
		t.Fatalf("close: %+v", e)
	}
	if _, err := c.prompt(sid, "again"); err == nil || err.Code != codeResourceNotFound {
		t.Fatalf("prompt after close: %+v", err)
	}
	if _, err := c.call("session/close", CloseSessionRequest{SessionID: sid}); err == nil || err.Code != codeResourceNotFound {
		t.Fatalf("second close: %+v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("rm ran after close")
	}
}

func TestResumeWithoutReplay(t *testing.T) {
	work := t.TempDir()
	s := newScript(t, textTurn("first"))
	o := testOptions(t, s)
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "remember the word kumquat")
	c2 := startClient(t, o)
	res, err := c2.call("session/resume", ResumeSessionRequest{SessionID: sid, Cwd: work, MCPServers: []json.RawMessage{}})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r ResumeSessionResponse
	json.Unmarshal(res, &r)
	conform(t, "ResumeSessionResponse", r)
	if r.Modes == nil || len(r.ConfigOptions) == 0 {
		t.Fatalf("%s", res)
	}
	if n := len(c2.updatesOf(updUserMessage)) + len(c2.updatesOf(updAgentMessage)); n != 0 {
		t.Fatalf("resume replayed %d chunks", n)
	}
	s.mu.Lock()
	s.turns = append(s.turns, textTurn("kumquat"))
	s.mu.Unlock()
	if stop, err := c2.prompt(sid, "which word?"); err != nil || stop != stopEndTurn {
		t.Fatalf("prompt after resume: %q %+v", stop, err)
	}
	if !strings.Contains(lastUserText(t, s), "which word?") {
		t.Fatal("the follow-up prompt did not reach the model")
	}
	s.mu.Lock()
	last := mustJSON(s.bodies[len(s.bodies)-1])
	s.mu.Unlock()
	if !strings.Contains(last, "remember the word kumquat") {
		t.Fatal("the resumed history did not reach the model")
	}
}

// An id that is open here must not be reused for another workdir.
func TestReopenOpenSessionOtherCwd(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := c.newSession(t.TempDir())
	for _, m := range []string{"session/resume", "session/load"} {
		_, err := c.call(m, ResumeSessionRequest{SessionID: sid, Cwd: t.TempDir(), MCPServers: []json.RawMessage{}})
		if err == nil || err.Code != codeInvalidParams || !strings.Contains(err.Message, "another directory") {
			t.Fatalf("%s from another cwd: %+v", m, err)
		}
	}
}

func TestDeleteSession(t *testing.T) {
	work := t.TempDir()
	o := testOptions(t, newScript(t, textTurn("ok")))
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "hello")
	// Open and idle in this process: delete closes it first (our own writer
	// holds the file lock), then removes the file.
	if _, err := c.call("session/delete", DeleteSessionRequest{SessionID: sid}); err != nil {
		t.Fatalf("delete: %+v", err)
	}
	res, _ := c.call("session/list", ListSessionsRequest{Cwd: work})
	var l ListSessionsResponse
	json.Unmarshal(res, &l)
	if len(l.Sessions) != 0 {
		t.Fatalf("deleted session still listed: %+v", l)
	}
	if _, err := c.call("session/delete", DeleteSessionRequest{SessionID: sid}); err == nil || err.Code != codeResourceNotFound {
		t.Fatalf("second delete: %+v", err)
	}
	if _, err := c.call("session/delete", DeleteSessionRequest{SessionID: "last"}); err == nil || err.Code != codeInvalidParams {
		t.Fatalf("delete last: %+v", err)
	}
}

func TestDeleteRunningSessionRefused(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	got := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.call("session/delete", DeleteSessionRequest{SessionID: sid})
		got <- err
		return selected(optReject)
	}
	c.prompt(sid, "delete x")
	if e := <-got; e == nil || e.Code != codeInvalidRequest {
		t.Fatalf("delete mid-prompt: %+v", e)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'Lifecycle|Close|Resume|Delete|ProtocolShapes' -race -count=1`
Expected: FAIL (`undefined: CloseSessionRequest`, `unknown field Delete`)

- [ ] **Step 3: Add the types**

In `protocol.go`, replace `SessionCapabilities` and add the request/response types:

```go
// SessionCapabilities: each non-nil field advertises the method of that name.
type SessionCapabilities struct {
	List   *struct{} `json:"list,omitempty"`
	Delete *struct{} `json:"delete,omitempty"`
	Resume *struct{} `json:"resume,omitempty"`
	Close  *struct{} `json:"close,omitempty"`
}

type ResumeSessionRequest struct {
	SessionID  string            `json:"sessionId"`
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

type ResumeSessionResponse struct {
	Modes         *SessionModeState `json:"modes,omitempty"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
}

type CloseSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type DeleteSessionRequest struct {
	SessionID string `json:"sessionId"`
}
```

Append `"session/close", "session/resume", "session/delete"` to `acpMethods`.

- [ ] **Step 4: Closed state in `session.go`**

Add to `session` (in the `runMu`-guarded group):

```go
	closed bool           // session/close or delete: no new prompt may begin
	runWG  sync.WaitGroup // the running prompt, awaited by shutdown
```

In `begin`, refuse a closed session and count the run:

```go
	if s.closed {
		return nil, notFound("session %s is closed", s.id)
	}
	if s.cancel != nil {
		return nil, busy("a prompt is already running in this session")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.runWG.Add(1)
```

In `end`, call `s.runWG.Done()` inside the `if s.cancel != nil` block. In `idle`, return `notFound("session %s is closed", s.id)` first when `s.closed`. Add:

```go
// shutdown ends the session (session/close, delete): running work is
// cancelled as by session/cancel and awaited, then the agent is closed —
// its MCP servers stop and the session file's writer lock is released.
// Idempotent.
func (s *session) shutdown() {
	s.runMu.Lock()
	if s.closed {
		s.runMu.Unlock()
		return
	}
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.runMu.Unlock()
	s.runWG.Wait()
	s.agent.Close()
}
```

In `server.go`, add:

```go
func (srv *Server) remove(id string) {
	srv.mu.Lock()
	delete(srv.sessions, id)
	srv.mu.Unlock()
}
```

`closeAll` stays as it is: it runs after every handler has returned.

- [ ] **Step 5: Handlers in `history.go`**

Rename `loadSession`'s body into `reopen`, which is shared by load and resume. It differs from Task 9 in two ways: the replay is conditional, and an already-open session must belong to the same `cwd`.

```go
func (srv *Server) loadSession(params json.RawMessage) (any, error) {
	req, err := decode[LoadSessionRequest](params)
	if err != nil {
		return nil, err
	}
	return srv.reopen(req.SessionID, req.Cwd, true)
}

// resumeSession is session/load without the history replay.
func (srv *Server) resumeSession(params json.RawMessage) (any, error) {
	req, err := decode[ResumeSessionRequest](params)
	if err != nil {
		return nil, err
	}
	r, err := srv.reopen(req.SessionID, req.Cwd, false)
	if err != nil {
		return nil, err
	}
	return ResumeSessionResponse{Modes: r.Modes, ConfigOptions: r.ConfigOptions}, nil
}

// reopen serves session/load (replay) and session/resume (no replay): an
// open session is reused, a stored one resumed in its own workdir only.
func (srv *Server) reopen(id, cwd string, replay bool) (LoadSessionResponse, error) {
	if !id8.MatchString(id) {
		return LoadSessionResponse{}, invalidParams("sessionId must be a moca session id (8 hex chars), got %q", id)
	}
	if s, ok := srv.get(id); ok {
		if filepath.Clean(s.cwd) != filepath.Clean(cwd) {
			return LoadSessionResponse{}, invalidParams("session %s belongs to another directory (%s)", id, s.cwd)
		}
		r, err := s.idle(func() (any, error) {
			if replay {
				s.replay()
			}
			s.sendCommands()
			return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
		})
		if err != nil {
			return LoadSessionResponse{}, err
		}
		return r.(LoadSessionResponse), nil
	}
	cfg, trusted, err := srv.sessionConfig(cwd)
	if err != nil {
		return LoadSessionResponse{}, err
	}
	path, err := msession.Find(sessionsDir(), id, cwd)
	if err != nil {
		return LoadSessionResponse{}, notFound("%v", err)
	}
	if !msession.InWorkdir(path, cwd) {
		return LoadSessionResponse{}, invalidParams("session %s belongs to another directory (%s)", id, msession.WorkdirOf(path))
	}
	s := newSession(srv, cwd, trusted)
	a, err := agent.Resume(srv.startOptions(cfg, s), path)
	if err != nil {
		return LoadSessionResponse{}, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	s.attach(a)
	s.reloadCommands()
	srv.add(s)
	s.markReady() // the client named this id: updates may flow now
	if replay {
		s.replay()
	}
	s.sendCommands()
	return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
}

func (srv *Server) closeSession(params json.RawMessage) (any, error) {
	req, err := decode[CloseSessionRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	srv.remove(s.id)
	s.shutdown()
	slog.Info("acp session closed", "session", s.id)
	return struct{}{}, nil
}

// deleteSession removes a stored session (one session/list shows). A
// session open here is closed first — this process's writer holds the file
// lock — unless a prompt runs; one held by another moca process is refused.
func (srv *Server) deleteSession(params json.RawMessage) (any, error) {
	req, err := decode[DeleteSessionRequest](params)
	if err != nil {
		return nil, err
	}
	if !id8.MatchString(req.SessionID) {
		return nil, invalidParams("sessionId must be a moca session id (8 hex chars), got %q", req.SessionID)
	}
	if s, ok := srv.get(req.SessionID); ok {
		if _, err := s.idle(func() (any, error) { return nil, nil }); err != nil {
			return nil, err // busy: a prompt is running
		}
		srv.remove(s.id)
		s.shutdown()
	}
	path, err := msession.Find(sessionsDir(), req.SessionID, "")
	if err != nil {
		return nil, notFound("%v", err)
	}
	if err := msession.Delete(path); err != nil {
		if errors.Is(err, msession.ErrInUse) {
			return nil, busy("session " + req.SessionID + " is open in another moca process: close it there first")
		}
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	slog.Info("acp session deleted", "session", req.SessionID)
	return struct{}{}, nil
}
```

Add `"errors"` and `"log/slog"` to `history.go`'s imports. `session.Find`'s workdir argument only matters for `"last"`, which the id8 check already excludes.

- [ ] **Step 6: Wire it in**

In `server.go` `Request`, add:

```go
	case "session/close":
		return srv.closeSession(params)
	case "session/resume":
		return srv.resumeSession(params)
	case "session/delete":
		return srv.deleteSession(params)
```

In `initialize`, advertise all four: `SessionCapabilities: SessionCapabilities{List: &struct{}{}, Delete: &struct{}{}, Resume: &struct{}{}, Close: &struct{}{}}`.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): session/close, session/resume and session/delete"
```

---

### Task 12: Richer session updates: notices, compaction, session info, effort, config updates

There are five additions, all mapping events or state moca already has:

| ACP | moca source | Rule |
|---|---|---|
| `notice` | `agent.Warning`, `agent.Retry` | **only** when the client advertised `clientCapabilities.session.notices` (spec MUST). Otherwise a Warning stays a `moca: …` thought line and a Retry sends nothing. |
| `compaction_summary_chunk` + `compaction_update` | `agent.Compacted`; the summary message in `History()` | **only** with `clientCapabilities.session.compaction` (spec MUST). moca's summarizer returns the summary whole, so it is sent as one chunk (which opens the compaction) followed by `compaction_update {status: completed}`. Without the capability: a thought line, and on replay the existing "_(earlier conversation was summarized)_" chunk. |
| `session_info_update` | the first prompt's first line (the same rule as `session/list`'s title) | after each prompt: `updatedAt` always; `title` once, only for sessions created in this process (loaded and resumed ones already have one) |
| `effort` config option | `provider.Model.ClampEffort`, `Agent.SetEffort` | a `select` with category `thought_level`, offered only when the model accepts ≥ 2 levels; idle only |
| `config_option_update` | `buildConfigOptions` | sent after a prompt when the options differ from what the client last saw. Example: a `moca login` in a terminal makes another provider's models usable. `set_config_option` responses carry the options themselves. |

**Files:**
- Modify: `internal/acp/protocol.go` (types, discriminators, `InitializeRequest.ClientCapabilities`), `internal/acp/server.go` (client capabilities, prompt tail), `internal/acp/session.go` (`emit` cases, `compacted`, `sendInfo`, fields), `internal/acp/modes.go` (effort, `buildConfigOptions`, `reportConfig`, `setConfigOption`), `internal/acp/history.go` (`replayMessages`, `titled` on reopen), `internal/acp/client_test.go` (an uncredentialed provider in `testOptions`)
- Test: `internal/acp/updates_test.go`, `internal/acp/protocol_test.go`

**Interfaces:**
- Consumes: `agent.Warning{Text}`, `agent.Retry{Notice provider.RetryNotice{Attempt, Max int; Wait time.Duration}}`, `agent.Compacted{TokensBefore, TokensAfter int}` (emitted outside `a.mu`, `internal/agent/compaction.go:168`), `session.SummaryPrefix`, `llm.Efforts`, `provider.Model.ClampEffort`, `(*agent.Agent).Effort()`, `.SetEffort(llm.Effort) (llm.Effort, error)`.
- Produces: discriminators `updNotice, updCompaction, updCompactionChunk, updSessionInfo, updConfigOptions`; types `Notice, CompactionSummaryChunk, CompactionUpdate, SessionInfoUpdate, ConfigOptionUpdate, ClientCapabilities, ClientSessionCapabilities`; `func (srv *Server) clientSupports() (notices, compaction bool)`; `func summaryOf(text string) string`; `func summaryText([]llm.Message) string`; `func (s *session) compacted(summary string, before, after int)`; `func (s *session) replayMessages([]llm.Message)`; `func (s *session) sendInfo(text string)`; `func effortLevels(provider.Model) []llm.Effort`; `func effortOption(provider.Model, llm.Effort) *ConfigOption`; `func (s *session) buildConfigOptions() []ConfigOption`; `func (s *session) reportConfig()`.

- [ ] **Step 1: Write the failing tests**

Append to `TestProtocolShapesConform` in `protocol_test.go`:

```go
	title := "fix the parser"
	for _, u := range []any{
		Notice{SessionUpdate: updNotice, Severity: "warning", Title: "run stopped", Description: "details"},
		CompactionSummaryChunk{SessionUpdate: updCompactionChunk, CompactionID: "c1", Content: ContentBlock{Type: "text", Text: "summary"}},
		CompactionUpdate{SessionUpdate: updCompaction, CompactionID: "c1", Status: "completed"},
		SessionInfoUpdate{SessionUpdate: updSessionInfo, Title: &title, UpdatedAt: "2026-10-13T10:00:00Z"},
		ConfigOptionUpdate{SessionUpdate: updConfigOptions, ConfigOptions: []ConfigOption{{ID: "model", Name: "Model",
			Category: "model", Type: "select", CurrentValue: "a/b", Options: []ConfigOptionValue{{Value: "a/b", Name: "a/b"}}}}},
	} {
		conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: u})
	}
```

In `internal/acp/client_test.go` `testOptions`, add a provider whose key is unset, so its model is not offered until the env var appears. Insert into the config's `providers` object:

```
"other":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_OTHER","models":{"x":{"contextWindow":65536}}}
```

(pass `s.srv.URL` a second time to `fmt.Sprintf`), and add `t.Setenv("MOCA_T_OTHER", "")` next to `MOCA_T_KEY`. Task 8's `TestNewSessionAdvertisesModesAndModels` still sees exactly two models: `other/x` has no credential.

`internal/acp/updates_test.go`:

```go
package acp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	msession "github.com/adeotek/moca/internal/session"
)

// bareSession is a session without an agent on a Server whose conn writes
// into a buffer: for the event-mapping rules. notices/compaction stand for
// the client's advertised capabilities.
func bareSession(t *testing.T, notices, compaction bool) (*session, func() []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	srv := &Server{sessions: map[string]*session{}, notices: notices, compaction: compaction}
	srv.conn = NewConn(&buf, srv)
	s := newSession(srv, "/w", true)
	s.id = "abcd1234"
	s.markReady()
	return s, func() []map[string]any {
		var out []map[string]any
		sc := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
		for sc.Scan() {
			var m struct {
				Params SessionNotification `json:"params"`
			}
			json.Unmarshal(sc.Bytes(), &m)
			out = append(out, m.Params.Update.(map[string]any))
		}
		return out
	}
}

func TestWarningAndRetryNotices(t *testing.T) {
	s, ups := bareSession(t, true, false)
	s.emit(agent.Warning{Text: "run stopped: the same failing call repeated 3 times"})
	s.emit(agent.Retry{Notice: provider.RetryNotice{Attempt: 2, Max: 5, Wait: 1500 * time.Millisecond}})
	got := ups()
	if len(got) != 2 || got[0]["sessionUpdate"] != updNotice || got[0]["severity"] != "warning" ||
		got[1]["severity"] != "info" || !strings.Contains(got[1]["description"].(string), "attempt 2 of 5") {
		t.Fatalf("%v", got)
	}
	for _, u := range got {
		conform(t, "SessionNotification", SessionNotification{SessionID: s.id, Update: u})
	}
	// Without the capability: no notice at all (spec MUST); the warning
	// stays visible as a thought line, the retry is silent.
	s2, ups2 := bareSession(t, false, false)
	s2.emit(agent.Warning{Text: "careful"})
	s2.emit(agent.Retry{Notice: provider.RetryNotice{Attempt: 1, Max: 5}})
	got = ups2()
	if len(got) != 1 || got[0]["sessionUpdate"] != updThought || !strings.Contains(mustJSON(got[0]), "moca: careful") {
		t.Fatalf("%v", got)
	}
}

func TestCompactedUpdates(t *testing.T) {
	s, ups := bareSession(t, false, true)
	s.compacted("the summary", 90000, 12000)
	got := ups()
	if len(got) != 2 || got[0]["sessionUpdate"] != updCompactionChunk || got[1]["sessionUpdate"] != updCompaction ||
		got[1]["status"] != "completed" || got[0]["compactionId"] != got[1]["compactionId"] ||
		got[0]["content"].(map[string]any)["text"] != "the summary" {
		t.Fatalf("%v", got)
	}
	for _, u := range got {
		conform(t, "SessionNotification", SessionNotification{SessionID: s.id, Update: u})
	}
	s2, ups2 := bareSession(t, false, false)
	s2.compacted("the summary", 90000, 12000)
	if got := ups2(); len(got) != 1 || got[0]["sessionUpdate"] != updThought || !strings.Contains(mustJSON(got[0]), "90000 → 12000") {
		t.Fatalf("without the capability: %v", got)
	}
}

func TestSummaryText(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: msession.SummaryPrefix + "\nWe fixed the parser."}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "ok"}}},
	}
	if got := summaryText(msgs); got != "We fixed the parser." {
		t.Fatalf("%q", got)
	}
	if summaryText(msgs[1:]) != "" {
		t.Fatal("no summary message, no summary")
	}
}

func TestReplayCompactionNeedsCapability(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: msession.SummaryPrefix + "\nS"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}},
	}
	s, ups := bareSession(t, false, true)
	s.replayMessages(msgs)
	if got := ups(); len(got) != 3 || got[0]["sessionUpdate"] != updCompactionChunk || got[1]["sessionUpdate"] != updCompaction || got[2]["sessionUpdate"] != updAgentMessage {
		t.Fatalf("with capability: %v", got)
	}
	s2, ups2 := bareSession(t, false, false)
	s2.replayMessages(msgs)
	if got := ups2(); len(got) != 2 || !strings.Contains(mustJSON(got[0]), "earlier conversation was summarized") {
		t.Fatalf("without capability: %v", got)
	}
}

// The client's {} vs null decides: notices on, compaction off. testEmit
// (tests only) fires a warning on the new session as soon as it is ready.
func TestInitializeRecordsClientCapabilities(t *testing.T) {
	o := testOptions(t, newScript(t))
	var notices, compaction bool
	o.testEmit = func(s *session) {
		notices, compaction = s.srv.clientSupports()
		s.emit(agent.Warning{Text: "hello"})
	}
	c := startClient(t, o)
	c.call("initialize", map[string]any{"protocolVersion": 1,
		"clientCapabilities": map[string]any{"session": map[string]any{"notices": map[string]any{}, "compaction": nil}}})
	c.newSession(t.TempDir())
	deadline := time.Now().Add(5 * time.Second)
	for len(c.updatesOf(updNotice)) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n, th := len(c.updatesOf(updNotice)), len(c.updatesOf(updThought)); n != 1 || th != 0 {
		t.Fatalf("notices advertised: want 1 notice and no thought line, got %d / %d", n, th)
	}
	if !notices || compaction {
		t.Fatalf("parsed capabilities: notices %v (want true), compaction %v (want false: null)", notices, compaction)
	}
}

func TestSessionInfoAfterPrompts(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t, textTurn("a"), textTurn("b"))))
	sid := c.newSession(t.TempDir())
	c.prompt(sid, "fix the parser\nit chokes on tabs")
	c.prompt(sid, "and the lexer")
	info := c.updatesOf(updSessionInfo)
	if len(info) != 2 || info[0]["title"] != "fix the parser" || info[1]["title"] != nil {
		t.Fatalf("title once, from the first prompt's first line: %v", info)
	}
	for _, u := range info {
		if _, err := time.Parse(time.RFC3339, u["updatedAt"].(string)); err != nil {
			t.Fatalf("updatedAt: %v", u)
		}
		conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: u})
	}
}

func TestEffortOption(t *testing.T) {
	thinking := provider.Model{Provider: "p", ID: "m", ThinkingMode: "openai",
		ThinkingLevelMap: map[llm.Effort]string{llm.EffortLow: "low", llm.EffortHigh: "high"}}
	if got := effortLevels(thinking); len(got) != 2 || got[0] != llm.EffortLow || got[1] != llm.EffortHigh {
		t.Fatalf("levels %v", got)
	}
	o := effortOption(thinking, llm.EffortHigh)
	if o == nil || o.ID != "effort" || o.Category != "thought_level" || o.CurrentValue != "high" || len(o.Options) != 2 {
		t.Fatalf("%+v", o)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
		Update: ConfigOptionUpdate{SessionUpdate: updConfigOptions, ConfigOptions: []ConfigOption{*o}}})
	if effortOption(provider.Model{ThinkingMode: "none"}, llm.EffortOff) != nil {
		t.Fatal("a model without thinking has nothing to pick")
	}
}

func TestConfigOptionUpdateWhenCredentialAppears(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t, textTurn("a"), textTurn("b"))))
	sid := c.newSession(t.TempDir())
	c.prompt(sid, "one")
	if n := len(c.updatesOf(updConfigOptions)); n != 0 {
		t.Fatalf("nothing changed, yet %d config_option_update", n)
	}
	t.Setenv("MOCA_T_OTHER", "k2") // as if the user ran `moca login` in a terminal
	c.prompt(sid, "two")
	ups := c.updatesOf(updConfigOptions)
	if len(ups) != 1 || !strings.Contains(mustJSON(ups[0]), "other/x") {
		t.Fatalf("%v", ups)
	}
}
```

The test hook is `Options.testEmit func(*session)`: unexported, nil in production, added in Step 4. It runs inside `Deferred.Then`, i.e. *after* the `session/new` response, so `c.newSession` can return before it. The test therefore polls until the notice arrives and reads the captured values only afterwards. The capture happens before the notice is written, and the pipe orders the write before the client's read, so this is race-free under `-race`.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/acp -run 'ProtocolShapes|Notices|Compact|SummaryText|ReplayCompaction|ClientCapabilities|SessionInfo|EffortOption|ConfigOptionUpdate' -race -count=1`
Expected: FAIL (undefined types and functions)

- [ ] **Step 3: Types in `protocol.go`**

```go
	updNotice          = "notice"
	updCompaction      = "compaction_update"
	updCompactionChunk = "compaction_summary_chunk"
	updSessionInfo     = "session_info_update"
	updConfigOptions   = "config_option_update"
```

(in the discriminator block), and:

```go
type InitializeRequest struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
}

// ClientCapabilities: only the session extensions moca reacts to. A field
// present as {} advertises support; omitted or null does not.
type ClientCapabilities struct {
	Session *ClientSessionCapabilities `json:"session,omitempty"`
}

type ClientSessionCapabilities struct {
	Notices    *struct{} `json:"notices,omitempty"`
	Compaction *struct{} `json:"compaction,omitempty"`
}

// Notice is a live, fire-and-forget message for the user (not history).
type Notice struct {
	SessionUpdate string `json:"sessionUpdate"`
	Severity      string `json:"severity"` // info | warning | error
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
}

// CompactionSummaryChunk appends to a compaction's summary; a first-seen id
// opens an in-progress compaction.
type CompactionSummaryChunk struct {
	SessionUpdate string       `json:"sessionUpdate"`
	CompactionID  string       `json:"compactionId"`
	Content       ContentBlock `json:"content"`
}

// CompactionUpdate patches a compaction by id (here: marks it completed).
type CompactionUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	CompactionID  string `json:"compactionId"`
	Status        string `json:"status"`
}

type SessionInfoUpdate struct {
	SessionUpdate string  `json:"sessionUpdate"`
	Title         *string `json:"title,omitempty"`
	UpdatedAt     string  `json:"updatedAt,omitempty"`
}

type ConfigOptionUpdate struct {
	SessionUpdate string         `json:"sessionUpdate"`
	ConfigOptions []ConfigOption `json:"configOptions"`
}
```

- [ ] **Step 4: Client capabilities in `server.go`**

Add to `Server`: `notices, compaction bool // the client's advertised session extensions (guarded by mu)`, and `testEmit func(*session)` to `Options` (unexported; tests only). Replace `initialize`'s decode:

```go
	req, err := decode[InitializeRequest](params)
	if err != nil {
		return nil, err
	}
	if sc := req.ClientCapabilities.Session; sc != nil {
		srv.mu.Lock()
		srv.notices, srv.compaction = sc.Notices != nil, sc.Compaction != nil
		srv.mu.Unlock()
	}
```

and add:

```go
// clientSupports reports the client's notices and compaction extensions:
// ACP forbids sending those updates to a client that did not advertise them.
func (srv *Server) clientSupports() (notices, compaction bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.notices, srv.compaction
}
```

In `newSession`'s `Deferred.Then`, after `s.markReady()`, add `if srv.opts.testEmit != nil { srv.opts.testEmit(s) }`.

In `prompt`, name the expanded text and add the post-run updates:

```go
	expanded := s.expand(text)
	out, err := s.agent.Run(runCtx, expanded)
	if s.reloadCommands() {
		s.sendCommands()
	}
	s.sendInfo(expanded)
	s.reportConfig()
```

- [ ] **Step 5: Events in `session.go`**

Add fields: `titled bool` (in the `mu` group) and `lastConfig string // JSON of the config options the client last saw (guarded by mu)`. Replace the `agent.Warning` case and add two more:

```go
	case agent.Warning:
		if notices, _ := s.srv.clientSupports(); notices {
			s.send(Notice{SessionUpdate: updNotice, Severity: "warning", Title: e.Text})
		} else {
			s.send(s.chunk(updThought, "moca: "+e.Text+"\n"))
		}
	case agent.Retry:
		if notices, _ := s.srv.clientSupports(); notices {
			s.send(Notice{SessionUpdate: updNotice, Severity: "info", Title: "Retrying the model request",
				Description: fmt.Sprintf("attempt %d of %d, waiting %s", e.Notice.Attempt, e.Notice.Max, e.Notice.Wait.Round(100*time.Millisecond))})
		}
	case agent.Compacted:
		// Emitted outside a.mu (compaction.go): History is safe here.
		if s.agent != nil {
			s.compacted(summaryText(s.agent.History()), e.TokensBefore, e.TokensAfter)
		}
```

and add:

```go
// compacted reports a finished compaction. With the client's compaction
// extension: one summary chunk (moca's summarizer returns the summary whole;
// the chunk opens the compaction) and a completed update. Without it: a
// thought line (before/after <= 0: replay, sizes unknown — see replay).
func (s *session) compacted(summary string, before, after int) {
	if _, ok := s.srv.clientSupports(); ok {
		id := newMessageID()
		if summary != "" {
			s.send(CompactionSummaryChunk{SessionUpdate: updCompactionChunk, CompactionID: id,
				Content: ContentBlock{Type: "text", Text: summary}})
		}
		s.send(CompactionUpdate{SessionUpdate: updCompaction, CompactionID: id, Status: "completed"})
		return
	}
	s.send(s.chunk(updThought, fmt.Sprintf("moca: context compacted (%d → %d tokens)\n", before, after)))
}

// summaryOf strips the compaction summary message's prefix.
func summaryOf(text string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), msession.SummaryPrefix))
}

// summaryText is the current compaction summary in msgs (History() puts it
// first), or "".
func summaryText(msgs []llm.Message) string {
	for _, m := range msgs {
		if m.Role != llm.RoleUser {
			continue
		}
		for _, b := range m.Content {
			if b.Type == llm.BlockText && strings.HasPrefix(strings.TrimSpace(b.Text), msession.SummaryPrefix) {
				return summaryOf(b.Text)
			}
		}
	}
	return ""
}

// sendInfo updates the client's session metadata after a prompt: the title
// once (session/list's rule: the first line of the first user message), the
// update time always.
func (s *session) sendInfo(text string) {
	u := SessionInfoUpdate{SessionUpdate: updSessionInfo, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	s.mu.Lock()
	if !s.titled {
		s.titled = true
		line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
		if r := []rune(line); len(r) > 80 {
			line = string(r[:80])
		}
		u.Title = &line
	}
	s.mu.Unlock()
	s.send(u)
}
```

Add `"strings"`, `"time"` and `msession "github.com/adeotek/moca/internal/session"` to `session.go`'s imports. The alias is required: this package declares `type session struct`, and Go rejects a file that also imports a package named `session` ("session already declared through import of package"). Every `internal/acp` file that uses the package imports it as `msession`, including `history.go` (Task 9) and the tests.

- [ ] **Step 6: Replay in `history.go`**

Split `replay` so it can be tested without an agent, and route the summary message through the capability rule:

```go
func (s *session) replay() { s.replayMessages(s.agent.History()) }

// replayMessages streams msgs (see replay): user and agent text, thinking,
// tool calls with their results; the compaction summary as a compaction
// (client extension) or as one summarized-note chunk.
func (s *session) replayMessages(msgs []llm.Message) {
```

This is the old `replay` body without its first line (`msgs := s.agent.History()`). Its summary branch becomes:

```go
				if strings.HasPrefix(strings.TrimSpace(b.Text), msession.SummaryPrefix) {
					if _, ok := s.srv.clientSupports(); ok {
						s.compacted(summaryOf(b.Text), 0, 0)
					} else {
						s.send(s.chunk(updAgentMessage, "_(earlier conversation was summarized)_\n\n"))
					}
				} else {
```

In `reopen`, mark resumed sessions as titled: add `s.titled = true` right after `s := newSession(srv, cwd, trusted)`.

- [ ] **Step 7: Effort and config updates in `modes.go`**

Rename `configOptions` to `buildConfigOptions` and append the effort option before its `return`:

```go
	opts := []ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: cur, Options: vals}}
	if e := effortOption(s.agent.Model(), s.agent.Effort()); e != nil {
		opts = append(opts, *e)
	}
	return opts
```

Then add:

```go
// configOptions builds the options for a response and records them as what
// the client has seen.
func (s *session) configOptions() []ConfigOption {
	opts := s.buildConfigOptions()
	b, _ := json.Marshal(opts)
	s.mu.Lock()
	s.lastConfig = string(b)
	s.mu.Unlock()
	return opts
}

// reportConfig sends config_option_update when the options changed since
// the client last saw them — e.g. a `moca login` in a terminal made another
// provider's models usable. Called after each prompt.
func (s *session) reportConfig() {
	opts := s.buildConfigOptions()
	b, _ := json.Marshal(opts)
	s.mu.Lock()
	changed := string(b) != s.lastConfig
	s.lastConfig = string(b)
	s.mu.Unlock()
	if changed {
		s.send(ConfigOptionUpdate{SessionUpdate: updConfigOptions, ConfigOptions: opts})
	}
}

// effortLevels are the efforts m takes as-is, in llm.Efforts order; a model
// without thinking takes only "off".
func effortLevels(m provider.Model) []llm.Effort {
	var out []llm.Effort
	for _, e := range llm.Efforts {
		if m.ClampEffort(e) == e {
			out = append(out, e)
		}
	}
	return out
}

// effortOption is the "effort" select, or nil when m has fewer than two
// levels (nothing to pick).
func effortOption(m provider.Model, cur llm.Effort) *ConfigOption {
	lv := effortLevels(m)
	if len(lv) < 2 {
		return nil
	}
	o := ConfigOption{ID: "effort", Name: "Effort", Category: "thought_level", Type: "select", CurrentValue: string(cur)}
	for _, e := range lv {
		o.Options = append(o.Options, ConfigOptionValue{Value: string(e), Name: string(e)})
	}
	return &o
}
```

Replace `setConfigOption`'s body after the `lookup`:

```go
	var value string
	if err := json.Unmarshal(req.Value, &value); err != nil {
		return nil, invalidParams("%s value must be a string", req.ConfigID)
	}
	var apply func() error
	switch req.ConfigID {
	case "model":
		apply = func() error { return s.agent.SetModel(value, "") }
	case "effort":
		apply = func() error {
			if !slices.Contains(effortLevels(s.agent.Model()), llm.Effort(value)) {
				return fmt.Errorf("effort %q is not offered by %s", value, s.agent.Model().Qualified())
			}
			_, err := s.agent.SetEffort(llm.Effort(value))
			return err
		}
	default:
		return nil, invalidParams("unknown config option %q", req.ConfigID)
	}
	return s.idle(func() (any, error) {
		if err := apply(); err != nil {
			return nil, invalidParams("%v", err)
		}
		return SetConfigOptionResponse{ConfigOptions: s.configOptions()}, nil
	})
```

Add `"fmt"`, `"github.com/adeotek/moca/internal/llm"` and `"github.com/adeotek/moca/internal/provider"` to `modes.go`. Task 8's `TestSetModel` still passes: its `effort`/`high` case gets `-32602`, because the fake models take only `off`.

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/acp -race -count=1`
Expected: PASS. If `TestConfigOptionUpdateWhenCredentialAppears` never sees `other/x`, check that the registry lists every configured provider's models (`Agent.Models()`), not just the active provider's. If it lists only the active one, assert on the effort option instead: switch `fake/m` → `fake/m2` through `set_config_option`, and check that the next prompt sends no update (the response already carried them).

- [ ] **Step 9: Commit**

```bash
git add internal/acp
git commit -m "feat(acp): notices, compaction, session info, effort option and config updates"
```

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

## Appendix A — Merge decisions

CC = Claude Code plan `docs/superpowers/plans/2026-10-13-acp.md` (with its spec `docs/superpowers/specs/2026-10-13-acp-design.md`); HA = Hermes Agent plan `docs/plans/2026-10-13-acp-protocol.md`. "Verified" = checked against the code on 2026-10-13.

### Taken from CC (and why it beat HA)

| Topic | CC | HA | Reason |
|---|---|---|---|
| Plan form | 12 TDD tasks with full code | 12 tasks, prose steps | an executor needs the code; CC's line refs were verified |
| Replay source | `Agent.History()` | new `Agent.Transcript()` raw-entry accessor | verified: `session.Messages` keeps tool_use/tool_result blocks; `Resume` appends `Repair(…, Interrupted)` for orphaned calls; the TUI replays from `History()` (`internal/tui/app.go:1337`). No agent-core change is needed. |
| Compacted history on load | one "_(earlier conversation was summarized)_" chunk, via an exported `session.SummaryPrefix` | replays raw pre-compaction entries | matches what the model and the TUI see |
| Tool-call content | diffs for write/edit; capped text (shell tail, others head); read = summary | Summary text only, no diffs | diffs are the editor's main value; read never ships file bodies |
| `toolCallId` | `<seq>-<provider id>` | the provider's `call.ID` | verified: openai-compatible providers number `call_0…` per response, so raw ids collide across turns |
| `Outcome.Stuck` | `max_turn_requests` | `end_turn` | verified: `Stuck` is "like MaxSteps, an incomplete run"; `-p` exits `exitMaxSteps` for both |
| User echo on prompt | none (replay only) | `user_message_chunk` echo | clients render the user's own prompt; an echo would duplicate it |
| Prompt content | text + `resource_link` + embedded text resource; `embeddedContext: true` | text only | editors send `@file` as links/resources |
| `Warning` events | thought chunk `moca: …` | dropped | the user needs "run stopped: same failing call repeated" |
| Malformed line | `-32700`, id null, connection kept | log and skip | JSON-RPC 2.0 requires the error response |
| Optional features | modes, model picker, `session/list`, slash commands | modes only (list, model, commands out) | each is small, tested and schema-checked; HA excluded them for scope, not correctness |
| Schema handling | vendored pinned schema + `conform` checker in every shape test | "verified against the Python SDK" | a test-enforced schema can't drift |
| `"last"` / non-id8 on load | rejected | not considered | `session.Find` accepts `"last"`; a client must never silently resume the newest session |
| Early updates | held until the `session/new` response is written (`Deferred`, `markReady`) | not considered | the client can't route updates for an id it doesn't have yet |
| Shared helpers | `skills.SkillMessage`, `session.SummaryPrefix`, `config.ResolveMode` moved | none | one implementation for TUI, `-p` and ACP |
| Commit rule | approval once, else stage only | commit every task | AGENTS.md: "Never create a commit without the user's explicit approval" |
| Branch | `phase/8-acp` | `phase/10-acp-server` | verified: README phases end at 7, so ACP is phase 8 |
| Zed config | `agent_servers {command, args}` | an `agents.json` shape guess | matches Zed's external-agent settings |

### Taken from HA (added to CC)

| Topic | Where | Reason |
|---|---|---|
| `usage_update` after each model turn | Task 2 (type, schema gate), Task 5 (emit) | settled decision; verified that `TurnEnd` is emitted outside `a.mu`, so `Status()` is safe in `emit` |
| Saved trust decision honoured | Task 5 (`Options.Trust`), Task 9, Task 13 (`acpTrust`) | CC trusted every workdir, ignoring a "no" the user gave in the TUI. Trust is also per workdir now (one `acp` process serves many folders). |
| `--approve` flag | Task 13 | symmetry with `--no-approve` and the global flags |
| No permission timeout, documented | Global Constraints, Task 15 | settled; CC had no timeout but didn't say so |
| Concurrent-write and unknown-notification tests | Task 1 | pins the single-writer invariant under `-race` |
| Load continuity checks | Task 9 | the follow-up request carries the loaded history, and the response comes after the replay |
| Hermetic E2E gate over the built binary, with leak scan | Task 14 | catches stdout pollution, exit codes and cross-process load, which in-process tests can't. Written in Go in the `test/shipgate` style instead of HA's Python client and Hermes scratch dir. |
| ACP TCK (manual) | Task 16 | settled: manual only, results in the PR |
| v0.5.0-beta bump | Global Constraints, Task 15 | settled; CC had "unreleased" |
| `graphify update`, PR evidence | Task 16 | house rules |

### New in the merge (neither plan had it)

| Topic | Where | Reason |
|---|---|---|
| One `current_mode_update` per `set_mode` | Task 8 (`reportMode`, `muteMode`) | verified: plan → auto runs `SetPlan(false)` (emits `PlanChanged` while the mode still reads "default"), then `SetMode` (`ModeChanged`). CC sent a transient "default"; HA sent nothing for agent-side changes. |
| Trust-resolver test seeds real directories | Task 13 | verified: `TrustStore` keys are `EvalSymlinks` paths, so seeding a nonexistent dir fails |
| Gate log scan at debug level, failing on zero files | Task 14 | an empty scan must not pass silently |

### Added after the gap analysis (2026-10-13)

These were checked against the pinned `schema-v1.25.0` (`schema/v1/meta.json`, `schema.json`) and the v1 cancellation doc.

| Topic | Where | Notes |
|---|---|---|
| Schema path and tag fix | Task 2, Step 1 | upstream moved the files to `schema/v1/` (the old `schema/schema.json` returns 404), and `releases/latest` can be a crate tag (`v1.11.0`) or a v2 alpha. Both plans' download step would have failed or vendored the wrong schema. `meta.json` is now vendored too, and a test checks every method name against it. |
| `$/cancel_request` + `-32800` | Task 10 | incoming: per-request contexts; a cancelled request still answers (a prompt with `stopReason: cancelled`, others with `-32800`). Outgoing: a cancelled `Call` sends `$/cancel_request`, so `session/cancel` cascades to an unanswered permission prompt. |
| `-32002` resource not found | Task 10 | unknown session ids (was `-32602`); malformed ids stay `-32602` |
| `session/close`, `session/resume`, `session/delete` | Task 11 | close = cancel + await + `Agent.Close`; resume = load without replay; delete reuses `session.Delete` (the writer lock refuses live files). An already-open session now also has to match the requested `cwd` on load/resume. |
| `notice`, `compaction_summary_chunk` + `compaction_update` | Task 12 | client-capability-gated (spec MUST), with fallbacks. moca's summary is sent as one chunk followed by `completed`; replay maps the summary message the same way. |
| `session_info_update` | Task 12 | title once (the `session/list` rule), `updatedAt` per prompt |
| `effort` config option, `config_option_update` | Task 12 | effort levels from `ClampEffort` (offered only with ≥ 2); options re-checked after each prompt, e.g. after a `moca login` in a terminal |
| `msession` import alias | Tasks 9, 11, 12 | a compile bug carried over from the CC plan: `history.go` imported `internal/session` into a package that declares `type session` (verified with `go build`: "already declared through import of package") |

### Rejected from both

- HA's `-32000` for state errors (busy, mode mid-run): ACP reserves `-32000` for auth-required, which may trigger a client's login UI. CC uses `-32600` for busy and keeps `-32000` only for "no model/credential".
- HA's "not initialized" guard: the spec doesn't require it, and sessions don't depend on `initialize` state.
- HA's `session.Slug(first text)` slug: the session file is created in `session/new`, before any text, so the slug is `"acp"` (as in CC).

---
status: ready
batch: 3 of 4
tasks: 10-12
---

# `moca acp` — Batch 3 of 4: Protocol completeness — request cancellation, session lifecycle, richer updates

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this batch task by task in this session (the recommended mode on a usage-limited plan). superpowers:subagent-driven-development also works, at a higher cost. Steps use checkbox (`- [ ]`) syntax for tracking.

**This batch:** Tasks 10–12 of the ACP server plan — `$/cancel_request` in both directions with `-32800`/`-32002`, session/close|resume|delete, and the client-gated `notice`/compaction updates, `session_info_update`, the effort option and `config_option_update`.

**Whole feature:** `moca acp` serves the Agent Client Protocol (v1) on stdio so Zed, VS Code ACP extensions and Paseo can drive moca. It's a stdlib-only package `internal/acp` (bidirectional ndjson JSON-RPC 2.0 + ACP types + a server mapping each ACP session onto one `*agent.Agent`), wired through `moca acp` in `cmd/moca`.

**Tech Stack:** Go 1.27, stdlib only (`encoding/json`, `bufio`, `sync`, `context`); existing packages `agent`, `tools`, `skills`, `session`, `llm`, `config`, `provider`.

## Read only what this batch needs

This file is the **complete** instruction set for Tasks 10–12. The design decisions are already made and their code is written out below. To keep context and usage small:

1. **Do not open** the other plan files: `docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md`, `docs/superpowers/plans/2026-10-13-acp-batch-2-server.md`, `docs/superpowers/plans/2026-10-13-acp-batch-4-ship.md`, `docs/superpowers/plans/2026-10-13-acp-merged.md`, `docs/superpowers/plans/2026-10-13-acp.md`, `docs/plans/2026-10-13-acp-protocol.md`, or the spec `docs/superpowers/specs/2026-10-13-acp-design.md`. They are drafts or other batches; everything you need is here.
2. **Code from earlier batches is in the repo, not in other plans.** When a task's **Interfaces → Consumes** names a function or type, check it in the code (LSP `hover` / `documentSymbol` / `goToDefinition`, or `grep -n` for the symbol). Read whole files only when the task modifies them.
3. **Orientation:** if you need it, use `graphify query "<question>"` (a small scoped subgraph). Don't read `graphify-out/GRAPH_REPORT.md` or browse source trees.
4. **Tests:** run the exact per-step commands (`-run '…'`). Run the full suite once, at the end of the batch.
5. **Scope:** implement only Tasks 10–12. If a step seems to need something from a later task, it doesn't: do what the step says and note the question for the user.

## Before you start

Batch 2 must be complete. Check it before anything else; this command only reads files and runs tests:

```bash
git switch phase/8-acp
for f in server.go session.go commands.go modes.go history.go; do test -f internal/acp/$f || exit 1; done \
  && grep -q '"session/load"' internal/acp/server.go && grep -q 'usage_update' internal/acp/protocol.go \
  && go test ./internal/acp -race -count=1
```

If it fails, **stop** and report which check failed. Do not repair earlier batches from memory, and do not open the batch 2 plan to finish it yourself unless the user asks.

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

- **`$/cancel_request` arriving right behind its request.** The notification runs inline on the read loop, possibly before the request's goroutine starts. The cancel func must be registered in `dispatch`, or the cancel is silently lost and the request hangs. Pinned by `TestCancelRequest` (Task 10).
- **An editor that never advertised notices or compaction.** It must never receive a `notice`, `compaction_update` or `compaction_summary_chunk` (spec MUST); it gets the thought-line or note fallback instead. Pinned by `TestWarningAndRetryNotices`, `TestCompactedUpdates` and `TestReplayCompactionNeedsCapability` (Task 12).
- **`session/close` while a permission prompt is open.** The close must cancel the run, wait for it and free the agent without deadlocking on the unanswered prompt; later calls on the id get `-32002`. Pinned by `TestCloseCancelsAndFrees` (Task 11).
- **Deleting a session this process has open.** Our own writer holds the file lock, so `session.Delete` would report `ErrInUse`. An idle session is closed first; a running one is refused as busy. Pinned by `TestDeleteSession` and `TestDeleteRunningSessionRefused` (Task 11).
- **`session/load` or `session/resume` of an open id from a different `cwd`.** Expected: refused (`-32602`, naming the session's directory), never silently reused in the wrong workdir. Pinned by `TestReopenOpenSessionOtherCwd` (Task 11).

## Files this batch touches

| | Path |
|---|---|
| modify | `internal/acp/rpc.go` |
| modify | `internal/acp/server.go` |
| modify | `internal/acp/history.go` |
| modify | `internal/acp/protocol.go` |
| modify | `internal/acp/client_test.go` |
| modify | `internal/acp/server_test.go` |
| test | `internal/acp/rpc_test.go` |
| test | `internal/acp/permission_test.go` |
| test | `internal/acp/history_test.go` |
| modify | `internal/acp/session.go` |
| test | `internal/acp/lifecycle_test.go` |
| test | `internal/acp/protocol_test.go` |
| modify | `internal/acp/modes.go` |
| test | `internal/acp/updates_test.go` |

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

## Batch 3 completion

- [ ] Full checks: `gofmt -l .` (expect no output), `make vet`, `make test` (add `export PATH="$(mise where go)/bin:$PATH"` when `go` is not on PATH).
- [ ] Every checkbox above is ticked, or the user agreed to skip it.
- [ ] Commits: if the user approved committing at the start, every task has its commit; otherwise everything is staged and nothing committed.
- [ ] `graphify update .` (project rule after code changes).

**Stop here.** Do not start the next batch in this session. Tell the user batch 3 is done, give the commit range, and say the next file is `docs/superpowers/plans/2026-10-13-acp-batch-4-ship.md`, to be run in a fresh session (`/clear` first).

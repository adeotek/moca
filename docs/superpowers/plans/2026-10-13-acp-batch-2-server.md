---
status: ready
batch: 2 of 4
tasks: 5-9
---

# `moca acp` — Batch 2 of 4: Server — sessions, prompts, permissions, slash commands, modes, history

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this batch task by task in this session (the recommended mode on a usage-limited plan). superpowers:subagent-driven-development also works, at a higher cost. Steps use checkbox (`- [ ]`) syntax for tracking.

**This batch:** Tasks 5–9 of the ACP server plan — the ACP server over `*agent.Agent`: initialize, session/new, session/prompt with event translation and `usage_update`, permissions and cancel, prompt content and slash commands, modes and the model picker, session/list and session/load with replay.

**Whole feature:** `moca acp` serves the Agent Client Protocol (v1) on stdio so Zed, VS Code ACP extensions and Paseo can drive moca. It's a stdlib-only package `internal/acp` (bidirectional ndjson JSON-RPC 2.0 + ACP types + a server mapping each ACP session onto one `*agent.Agent`), wired through `moca acp` in `cmd/moca`.

**Tech Stack:** Go 1.27, stdlib only (`encoding/json`, `bufio`, `sync`, `context`); existing packages `agent`, `tools`, `skills`, `session`, `llm`, `config`, `provider`.

## Read only what this batch needs

This file is the **complete** instruction set for Tasks 5–9. The design decisions are already made and their code is written out below. To keep context and usage small:

1. **Do not open** the other plan files: `docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md`, `docs/superpowers/plans/2026-10-13-acp-batch-3-protocol-completeness.md`, `docs/superpowers/plans/2026-10-13-acp-batch-4-ship.md`, `docs/superpowers/plans/2026-10-13-acp-merged.md`, `docs/superpowers/plans/2026-10-13-acp.md`, `docs/plans/2026-10-13-acp-protocol.md`, or the spec `docs/superpowers/specs/2026-10-13-acp-design.md`. They are drafts or other batches; everything you need is here.
2. **Code from earlier batches is in the repo, not in other plans.** When a task's **Interfaces → Consumes** names a function or type, check it in the code (LSP `hover` / `documentSymbol` / `goToDefinition`, or `grep -n` for the symbol). Read whole files only when the task modifies them.
3. **Orientation:** if you need it, use `graphify query "<question>"` (a small scoped subgraph). Don't read `graphify-out/GRAPH_REPORT.md` or browse source trees.
4. **Tests:** run the exact per-step commands (`-run '…'`). Run the full suite once, at the end of the batch.
5. **Scope:** implement only Tasks 5–9. If a step seems to need something from a later task, it doesn't: do what the step says and note the question for the user.

## Before you start

Batch 1 must be complete. Check it before anything else; this command only reads files and runs tests:

```bash
git switch phase/8-acp
test -f internal/acp/rpc.go && test -f internal/acp/protocol.go && test -f internal/acp/toolcall.go && test -f internal/acp/testdata/meta.json \
  && grep -q 'func SkillMessage' internal/skills/prompts.go && grep -q 'func ResolveMode' internal/config/config.go \
  && go test ./internal/acp ./internal/skills ./internal/config ./internal/session ./internal/tui ./cmd/moca -race -count=1
```

If it fails, **stop** and report which check failed. Do not repair earlier batches from memory, and do not open the batch 1 plan to finish it yourself unless the user asks.

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

- **Provider tool-call ids repeat across turns.** OpenAI-compatible servers number calls `call_0`, `call_1`, … per response. Each ACP `toolCallId` must still be unique within the session, or the editor overwrites an earlier tool card. Pinned by `TestToolCallIDsUniqueAcrossTurns` (Task 5).
- **`session/load` with `"last"` or a malformed id.** `session.Find` accepts `"last"`, so ACP must reject anything that isn't an id8 rather than silently resuming the newest session. Pinned by `TestLoadRejectsNonID8` (Task 9).
- **Client disconnects while a permission request is pending.** On stdin EOF the pending call must resolve to `Deny`, the run must end and `Serve` must return without hanging. Pinned by `TestCallFailsOnEOF` (Task 1) and `TestEOFDuringPermissionDenies` (Task 6).

## Files this batch touches

| | Path |
|---|---|
| create | `internal/acp/server.go` |
| create | `internal/acp/session.go` |
| test | `internal/acp/script_test.go` |
| test | `internal/acp/client_test.go` |
| test | `internal/acp/server_test.go` |
| test | `internal/acp/permission_test.go` |
| create | `internal/acp/commands.go` |
| test | `internal/acp/commands_test.go` |
| create | `internal/acp/modes.go` |
| test | `internal/acp/modes_test.go` |
| create | `internal/acp/history.go` |
| test | `internal/acp/history_test.go` |

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

## Batch 2 completion

- [ ] Full checks: `gofmt -l .` (expect no output), `make vet`, `make test` (add `export PATH="$(mise where go)/bin:$PATH"` when `go` is not on PATH).
- [ ] Every checkbox above is ticked, or the user agreed to skip it.
- [ ] Commits: if the user approved committing at the start, every task has its commit; otherwise everything is staged and nothing committed.
- [ ] `graphify update .` (project rule after code changes).

**Stop here.** Do not start the next batch in this session. Tell the user batch 2 is done, give the commit range, and say the next file is `docs/superpowers/plans/2026-10-13-acp-batch-3-protocol-completeness.md`, to be run in a fresh session (`/clear` first).

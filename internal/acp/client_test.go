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
	t.Setenv("MOCA_T_OTHER", "")
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m","context":{"maxSteps":40},
		"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536},"m2":{"contextWindow":65536}}},
		"other":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_OTHER","models":{"x":{"contextWindow":65536}}}}}`, s.srv.URL, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	return Options{Config: cfg, Version: "test", HTTP: s.srv.Client()} // Trust nil: trusted
}

// askOptions is testOptions at the ask mode. The permission and lifecycle
// tests need a command that ASKS — they hook session/request_permission — and
// v0.7.2 made auto the product default, which runs `rm <file in the workdir>`
// without a prompt. Pinned here rather than in testOptions so the rest of the
// suite exercises the real default.
func askOptions(t *testing.T, s *scriptServer) Options {
	t.Helper()
	o := testOptions(t, s)
	o.Mode = "ask"
	return o
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
	cancels      []json.RawMessage // requestIds of $/cancel_request from the agent
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

// stop ends the session like an editor quitting: stdin EOF, then wait for
// Serve to return, which closes every agent and drops its session-file lock.
func (c *testClient) stop() {
	c.t.Helper()
	c.in.Close()
	select {
	case <-c.done:
	case <-time.After(10 * time.Second):
		c.t.Error("Serve did not return after stdin EOF")
	}
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
		case "$/cancel_request":
			var p struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			json.Unmarshal(m.Params, &p)
			c.mu.Lock()
			c.cancels = append(c.cancels, p.RequestID)
			c.mu.Unlock()
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

// eventually polls cond for up to 2 s: an update the server sent before a
// response can still be in the reader goroutine's queue when the test reads,
// so a test that must observe it cannot assume it has already landed.
func (c *testClient) eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("timed out waiting for %s", what)
	}
}

// allUpdates is a copy of every update received so far. The reader goroutine
// appends under c.mu, so a test must never range over the field itself — that
// is a data race the detector reports against whichever test happens to be
// running when it lands (it caught one in TestACPRememberTemplateNeverRuns).
func (c *testClient) allUpdates() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]any(nil), c.updates...)
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

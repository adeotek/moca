package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

func skipWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs /bin/sh")
	}
}

// shServer is a stdio "server" that is a shell one-liner.
func shServer(script string) config.MCPServer {
	return config.MCPServer{Command: "sh", Args: []string{"-c", script}}
}

// A fatal message written to stdout right before exiting must always reach
// the error: Wait used to close the stdout pipe under the reader (review
// pass 3, M1; ~5% of runs lost it).
func TestStdioStdoutFatalAlwaysSurfaces(t *testing.T) {
	skipWindows(t)
	for i := 0; i < 200; i++ {
		tr, err := startStdio(context.Background(), "x", shServer("echo 'fatal: missing token'; exit 3"), os.Environ())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = initialize(ctx, tr)
		cancel()
		if err == nil || !strings.Contains(err.Error(), "missing token") {
			t.Fatalf("run %d: stdout diagnostics lost: %v", i, err)
		}
	}
}

// A reply written just before the process exits must still be delivered.
func TestStdioReplyThenExitKeepsReply(t *testing.T) {
	skipWindows(t)
	for i := 0; i < 200; i++ {
		tr, err := startStdio(context.Background(), "x", shServer(`read l; printf '{"jsonrpc":"2.0","id":1,"result":{}}\n'; exit 0`), os.Environ())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err = tr.Call(ctx, "initialize", nil)
		cancel()
		if err != nil {
			t.Fatalf("run %d: reply lost to the exit race: %v", i, err)
		}
	}
}

// A server inheriting the stdout pipe into a grandchild must not hang the
// exit handler: the reader grace is bounded.
func TestStdioGrandchildHoldingStdoutDoesNotHangExit(t *testing.T) {
	skipWindows(t)
	tr, err := startStdio(context.Background(), "x", shServer("sleep 30 & exit 4"), os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := tr.Call(ctx, "initialize", nil); err == nil || ctx.Err() != nil {
		t.Fatalf("exit must fail the call promptly, got %v (ctx %v)", err, ctx.Err())
	}
}

func TestKeepNameWindows(t *testing.T) {
	for _, k := range []string{"Path", "SystemRoot", "USERPROFILE", "xdg_data_home"} {
		if !keepName("windows", k) {
			t.Errorf("windows must keep %s", k)
		}
	}
	if keepName("windows", "OPENAI_API_KEY") || keepName("linux", "Path") || keepName("linux", "SystemRoot") {
		t.Error("filter too loose")
	}
}

// Credentials embedded in env/header VALUES under innocuous keys are
// rewritten, not copied (pass 3, M2).
func TestImportRewritesEmbeddedSecretsInValues(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(`{"mcpServers":{
		"pg":{"command":"pg-mcp","env":{
			"DATABASE_URL":"postgres://app:hunter2@db.internal/prod",
			"DSN":"user=app password=hunter2 host=db",
			"WEBHOOK":"https://x/hook?sig=ghp_abcdefgh12345678",
			"PLAIN":"info"}},
		"web":{"type":"http","url":"https://x/mcp","headers":{"X-Custom":"Bearer abcdefgh12345"}}}}`), 0o600)
	adds, vars, _ := Plan([]Source{{"claude-code", p}}, nil, "/w")
	b, _ := json.Marshal(adds)
	for _, leak := range []string{"hunter2", "ghp_abcdefgh", "abcdefgh12345"} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("%q copied literally: %s", leak, b)
		}
	}
	if adds["pg"].Env["DATABASE_URL"] != "env:MOCA_MCP_PG_DATABASE_URL" || adds["pg"].Env["PLAIN"] != "info" || len(vars) != 4 {
		t.Fatalf("%+v %v", adds["pg"], vars)
	}
}

// A value-shaped token under an innocuous key (the value rule alone).
func TestRewriteSecretsValueRuleAlone(t *testing.T) {
	s, v := RewriteSecrets("x", config.MCPServer{Command: "c", Env: map[string]string{"NOTE": "sk-abcdefghijkl", "OK": "yes"}})
	if s.Env["NOTE"] != "env:MOCA_MCP_X_NOTE" || s.Env["OK"] != "yes" || len(v) != 1 {
		t.Fatal(s.Env, v)
	}
}

// Close must not wait for the HTTP client's whole timeout on a stalled DELETE
// (it runs under the server lock) — pass 3, M3.
func TestHTTPCloseBounded(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			select {
			case <-release:
			case <-time.After(10 * time.Second):
			}
			return
		}
		var req struct{ ID *int64 }
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Mcp-Session-Id", "s")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{}}`, *req.ID)
	}))
	defer srv.Close()
	defer close(release)
	old := closeTimeout
	closeTimeout = 200 * time.Millisecond
	defer func() { closeTimeout = old }()
	tr, _ := startHTTP("w", config.MCPServer{URL: srv.URL}, &http.Client{Timeout: 15 * time.Minute})
	if err := initialize(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	tr.Close()
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("Close took %v on a stalled DELETE", d)
	}
}

// abortingHTTPServer executes tools/call (counted per tool) and, on the first
// call of each tool, drops the SSE stream without sending the response.
func abortingHTTPServer(t *testing.T) (*httptest.Server, func(string) int) {
	var mu sync.Mutex
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			return
		}
		var req struct {
			ID     *int64
			Method string
			Params struct{ Name string }
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any = map[string]any{}
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "s")
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{
				{"name": "create_issue", "description": "Create", "inputSchema": map[string]any{"type": "object"}},
				{"name": "read_doc", "description": "Read", "inputSchema": map[string]any{"type": "object"}, "annotations": map[string]any{"readOnlyHint": true}},
			}}
		case "tools/call":
			mu.Lock()
			calls[req.Params.Name]++
			n := calls[req.Params.Name]
			mu.Unlock()
			if n == 1 { // the side effect happened; the response never arrives
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				return
			}
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": "done"}}}
		}
		resp, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, func(n string) int { mu.Lock(); defer mu.Unlock(); return calls[n] }
}

// An aborted stream is the aborted-SSE restart path (pass-2 M3) and its replay
// rule (pass 3, L1): a read-only tool is replayed on a fresh session; a
// non-read-only one is NOT (it may already have executed) and says so.
func TestHTTPAbortedStreamReplayRule(t *testing.T) {
	srv, calls := abortingHTTPServer(t)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"gh": {URL: srv.URL}}, time.Minute, ix, Options{HTTP: srv.Client()})
	defer m.Close()

	res, _, err := m.Call(context.Background(), "gh", "read_doc", nil)
	if err != nil || len(res.Content) != 1 || calls("read_doc") != 2 {
		t.Fatalf("read-only call must self-heal: %v %v (executions %d)", res, err, calls("read_doc"))
	}
	_, _, err = m.Call(context.Background(), "gh", "create_issue", json.RawMessage(`{"title":"t"}`))
	if err == nil || !strings.Contains(err.Error(), "may have executed") {
		t.Fatalf("ambiguous failure must be reported, not replayed: %v", err)
	}
	if calls("create_issue") != 1 {
		t.Fatalf("a non-read-only call executed %d times for one approval", calls("create_issue"))
	}
	// The dead transport was dropped: the next call restarts and works.
	if res, _, err := m.Call(context.Background(), "gh", "create_issue", nil); err != nil || res.Content[0].Text != "done" {
		t.Fatalf("server must restart on the next call: %v %v", res, err)
	}
}

func crashingServer(t *testing.T, mode string) (config.MCPServer, func() int) {
	f := filepath.Join(t.TempDir(), "starts")
	s := fakeServer(mode)
	s.Env["MOCA_FAKE_STARTS"] = f
	return s, func() int { b, _ := os.ReadFile(f); return len(b) }
}

// A stdio server that dies mid-call: a read-only call is retried exactly once
// (two starts, then the error); a non-read-only call is not retried.
func TestStdioCrashMidCallReplayRule(t *testing.T) {
	skipWindows(t)
	s, starts := crashingServer(t, "crashcall")
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()

	_, _, err := m.Call(context.Background(), "docs", "read_doc", nil)
	if err == nil || !strings.Contains(err.Error(), "exited") || starts() != 2 {
		t.Fatalf("read-only: one restart then the error: %v (%d starts)", err, starts())
	}
	before := starts()
	_, _, err = m.Call(context.Background(), "docs", "create_issue", nil)
	if err == nil || !strings.Contains(err.Error(), "may have executed") {
		t.Fatalf("non-read-only must not be replayed: %v", err)
	}
	if got := starts() - before; got != 1 { // the call's own (re)start only
		t.Fatalf("non-read-only call started %d servers, want 1", got)
	}
}

// A server that died while idle (nothing in flight) is restarted and the call
// goes through even for a non-read-only tool: the request never left.
func TestStdioDeadBetweenCallsRestarts(t *testing.T) {
	skipWindows(t)
	s, starts := countingServer(t)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	if _, _, err := m.Call(context.Background(), "docs", "read_doc", nil); err != nil {
		t.Fatal(err)
	}
	st := m.servers["docs"]
	st.mu.Lock()
	tr := st.cl.t.(*stdioTransport)
	st.mu.Unlock()
	tr.cmd.Process.Kill()
	<-tr.done
	if res, _, err := m.Call(context.Background(), "docs", "create_issue", nil); err != nil || len(res.Content) == 0 {
		t.Fatalf("not-sent failure must restart and replay: %v %v", res, err)
	}
	if starts() != 2 {
		t.Fatalf("starts = %d, want 2", starts())
	}
}

// A call that never answers is bounded by the call timeout, and the wedged
// server is stopped so the next use starts a fresh one.
func TestCallTimeoutStopsWedgedServer(t *testing.T) {
	s, _ := crashingServer(t, "hangcall")
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ(), CallTimeout: 300 * time.Millisecond})
	defer m.Close()
	start := time.Now()
	_, _, err := m.Call(context.Background(), "docs", "read_doc", nil)
	if err == nil || !strings.Contains(err.Error(), "did not answer within") || time.Since(start) > 5*time.Second {
		t.Fatalf("%v after %v", err, time.Since(start))
	}
	if len(m.Running()) != 0 {
		t.Fatal("a timed-out server must be stopped")
	}
}

// A server that never answers initialize is bounded by the handshake timeout.
func TestHandshakeTimeout(t *testing.T) {
	s, _ := crashingServer(t, "hanginit")
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ(), HandshakeTimeout: 300 * time.Millisecond})
	defer m.Close()
	start := time.Now()
	_, _, err := m.Call(context.Background(), "docs", "read_doc", nil)
	if err == nil || !strings.Contains(err.Error(), "no answer within") || time.Since(start) > 5*time.Second {
		t.Fatalf("%v after %v", err, time.Since(start))
	}
	// the caller's own cancellation is not a "server failure" to remember
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.Call(ctx, "docs", "read_doc", nil)
}

// tools/list that never ends is cut off instead of growing forever.
func TestListToolsRepeatedCursor(t *testing.T) {
	_, err := dial(t, fakeServer("loopcursor")).listTools(context.Background())
	if err == nil || !strings.Contains(err.Error(), "repeated cursor") {
		t.Fatalf("%v", err)
	}
}

// `search` names the servers it could not index, does not respawn a broken
// server on every query, and still serves the healthy ones (pass 3, L3).
func TestSearchReportsAndRemembersFailedServer(t *testing.T) {
	skipWindows(t)
	dir := t.TempDir()
	cnt := filepath.Join(dir, "n")
	good := fakeServer("")
	bad := shServer("echo x >> " + cnt + "; echo 'boom: bad creds' >&2; exit 1")
	servers := map[string]config.MCPServer{"good": good, "bad": bad}
	ix, _ := LoadIndex(filepath.Join(dir, "ix.json"))
	ix.Put("good", ConfigHash(good), []Tool{{Name: "read_doc", Description: "Read library documentation"}})
	m := NewManager(servers, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	p := NewTool(m, servers)
	for i := 0; i < 3; i++ {
		r := runP(p, &tools.Env{}, map[string]any{"action": "search", "query": "documentation"})
		if r.IsError || !strings.Contains(r.Content, "good/read_doc") || !strings.Contains(r.Content, "[server bad could not be indexed") ||
			!strings.Contains(r.Content, "bad creds") {
			t.Fatalf("search %d: %q", i, r.Content)
		}
	}
	if b, _ := os.ReadFile(cnt); len(b) != 2 { // "x\n": one spawn, not three
		t.Fatalf("broken server spawned %d times", len(b)/2)
	}
	// describe/call always retry (the user may have fixed the server).
	if _, err := m.Describe(context.Background(), "bad", "t"); err == nil {
		t.Fatal("describe of a broken server must fail")
	}
	if b, _ := os.ReadFile(cnt); len(b) != 4 {
		t.Fatalf("describe must retry the start: %d", len(b)/2)
	}
}

// An unwritable index is reported by search instead of silently losing
// cross-session persistence.
func TestSearchReportsUnsavedIndex(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	os.WriteFile(blocker, nil, 0o600)
	ix, _ := LoadIndex(filepath.Join(blocker, "sub", "ix.json")) // parent is a file: Save must fail
	s := fakeServer("")
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	_, notes, err := m.Search(context.Background(), "documentation", "")
	if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "could not be saved") {
		t.Fatalf("%v %v", notes, err)
	}
}

type recTransport struct{ params any }

func (r *recTransport) Call(_ context.Context, _ string, p any) (json.RawMessage, error) {
	r.params = p
	return json.RawMessage(`{"content":[]}`), nil
}
func (r *recTransport) Notify(context.Context, string, any) error { return nil }
func (r *recTransport) Close() error                              { return nil }

// Omitted, empty and explicit-null args all go out as {} (pass 3, L2).
func TestCallToolNullArgsSentAsEmptyObject(t *testing.T) {
	for _, in := range []string{"", "null", " null ", `{}`} {
		r := &recTransport{}
		(&client{t: r}).callTool(context.Background(), "x", json.RawMessage(in))
		b, _ := json.Marshal(r.params)
		if !strings.Contains(string(b), `"arguments":{}`) {
			t.Errorf("args %q → %s", in, b)
		}
	}
}

// Two writers (two moca processes) must not clobber each other's entries.
func TestIndexSaveMergesConcurrentWriters(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ix.json")
	a, _ := LoadIndex(p)
	b, _ := LoadIndex(p) // both loaded before either saved
	a.Put("alpha", "h1", []Tool{{Name: "a1"}})
	b.Put("beta", "h2", []Tool{{Name: "b1"}})
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadIndex(p)
	if _, ok := got.Valid("alpha", "h1"); !ok {
		t.Fatal("alpha lost to the second writer")
	}
	if _, ok := got.Valid("beta", "h2"); !ok {
		t.Fatal("beta missing")
	}
	// A process's own re-index of a server wins over the disk copy.
	a.Put("beta", "h3", []Tool{{Name: "b2"}})
	a.Save()
	got, _ = LoadIndex(p)
	if _, ok := got.Valid("beta", "h3"); !ok {
		t.Fatal("fresh entry must overwrite the stale one")
	}
	if left, _ := filepath.Glob(p + ".*.tmp"); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

package mcp

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
	hits, _, err := m.Search(context.Background(), "documentation", "")
	if err != nil || len(hits) != 1 || hits[0].Tool != "read_doc" || starts() != 1 {
		t.Fatal(hits, err, starts())
	}
	m.Close()

	// second "session": same index file, zero starts for search
	ix2, _ := LoadIndex(ixPath)
	m2 := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix2, Options{BaseEnv: os.Environ()})
	defer m2.Close()
	if hits, _, _ := m2.Search(context.Background(), "issue", ""); len(hits) != 1 || starts() != 1 {
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
	if _, _, err := m.Search(context.Background(), "x", "nope"); err == nil || !strings.Contains(err.Error(), "configured: docs") {
		t.Fatal(err)
	}
	if _, _, err := m.Call(context.Background(), "docs", "nope", nil); err == nil || !strings.Contains(err.Error(), "action=search") {
		t.Fatal(err)
	}
}

// A server-supplied error message containing "exited" must NOT restart the
// server: the retry predicate keys on the transport sentinels, never on error
// text (pass-1 M1 — a false restart also kills healthy in-flight siblings).
func TestServerErrorTextDoesNotRestart(t *testing.T) {
	f := filepath.Join(t.TempDir(), "starts")
	s := fakeServer("errtext")
	s.Env["MOCA_FAKE_STARTS"] = f
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	_, _, err := m.Call(context.Background(), "docs", "read_doc", nil)
	if err == nil || !strings.Contains(err.Error(), "job exited") {
		t.Fatalf("server error must surface: %v", err)
	}
	if b, _ := os.ReadFile(f); len(b) != 1 {
		t.Fatalf("server error text must not restart the server (%d starts)", len(b))
	}
}

// A 404 after a session exists is a dead session: the manager must close the
// transport, restart the server and retry exactly once (pass-2 M3/L5).
func TestHTTPRestartOnSessionExpired(t *testing.T) {
	var mu sync.Mutex
	sessions, inits := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(200)
			return
		}
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
			mu.Lock()
			sessions++
			inits++
			sid := fmt.Sprintf("sess-%d", sessions)
			mu.Unlock()
			w.Header().Set("Mcp-Session-Id", sid)
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{"name": "echo", "description": "Echo",
				"inputSchema": map[string]any{"type": "object"}, "annotations": map[string]any{"readOnlyHint": true}}}}
		case "tools/call":
			if r.Header.Get("Mcp-Session-Id") == "sess-1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}
		}
		resp, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"web": {URL: srv.URL}}, time.Minute, ix, Options{HTTP: srv.Client()})
	defer m.Close()
	res, _, err := m.Call(context.Background(), "web", "echo", nil)
	if err != nil || len(res.Content) != 1 || res.Content[0].Text != "ok" {
		t.Fatalf("expired session must self-heal: %v %v", res, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if inits != 2 {
		t.Fatalf("expected exactly one restart (2 initializes), got %d", inits)
	}
}

// Without an injected client, HTTP MCP calls must get a bounded client: the
// TUI's run context is unbounded, so a wedged SSE response would otherwise
// hang a call until the user interrupts (pass-2 M1).
func TestDefaultHTTPClientHasTimeout(t *testing.T) {
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"web": {URL: "https://x"}}, time.Minute, ix, Options{})
	if m.o.HTTP == nil || m.o.HTTP.Timeout != defaultHTTPTimeout {
		t.Fatalf("HTTP MCP calls need a bounded client: %+v", m.o.HTTP)
	}
}

// After Close, a call must fail rather than start a server nothing will stop
// (pass-2 L4).
func TestClosedManagerDoesNotRespawn(t *testing.T) {
	s, starts := countingServer(t)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	if _, _, err := m.Call(context.Background(), "docs", "read_doc", nil); err != nil {
		t.Fatal(err)
	}
	m.Close()
	if _, _, err := m.Call(context.Background(), "docs", "read_doc", nil); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("calls after Close must fail, not respawn: %v", err)
	}
	if starts() != 1 {
		t.Fatalf("no server may start after Close (%d starts)", starts())
	}
}

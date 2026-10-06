package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// headerLog records every POST the fake server receives (mutex-guarded: the
// handler goroutines and the test goroutine must not race).
type headerLog struct {
	mu   sync.Mutex
	hdrs []http.Header
}

func (h *headerLog) add(hd http.Header) {
	h.mu.Lock()
	h.hdrs = append(h.hdrs, hd)
	h.mu.Unlock()
}

func (h *headerLog) last() http.Header {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hdrs[len(h.hdrs)-1]
}

func fakeHTTPServer(t *testing.T, sse bool) (*httptest.Server, *headerLog) {
	log := &headerLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(200)
			return
		}
		log.add(r.Header.Clone())
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
	return srv, log
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
		last := hdrs.last()
		if last.Get("Mcp-Session-Id") != "sess-1" || last.Get("Authorization") != "Bearer abc" || last.Get("MCP-Protocol-Version") != "2025-06-18" {
			t.Fatalf("headers: %v", last)
		}
		tr.Close()
	}
}

// Notify must check the HTTP status: a session dropped between initialize and
// notifications/initialized surfaces as session-expired, not success
// (pass-2 L1).
func TestHTTPNotifyStatusChecked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(200)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var req struct {
			ID *int64 `json:"id"`
		}
		json.Unmarshal(b, &req)
		if req.ID == nil {
			w.WriteHeader(http.StatusNotFound) // the session is gone
			return
		}
		w.Header().Set("Mcp-Session-Id", "sess-1")
		resp, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2025-06-18"}})
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	tr, err := startHTTP("web", config.MCPServer{URL: srv.URL}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	err = initialize(context.Background(), tr)
	if err == nil || !errors.Is(err, errSessionExpired) {
		t.Fatalf("a 404 on notifications/initialized must surface as session-expired: %v", err)
	}
}

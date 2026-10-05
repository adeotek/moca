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

// errFound stops an SSE read once the response with our id has arrived.
var errFound = errors.New("found")

// httpTransport speaks MCP streamable HTTP: one POST per message, JSON or
// SSE response, Mcp-Session-Id captured from initialize and sent afterwards,
// MCP-Protocol-Version once negotiated.
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
	t.mu.Lock()
	expired := resp.StatusCode == http.StatusNotFound && t.session != ""
	t.mu.Unlock()
	if expired {
		return nil, fmt.Errorf("mcp server %s: %w", t.name, errSessionExpired)
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
		if got, ok := responseID(m.ID); ok && got == id {
			out = m
			return errFound
		}
		return nil
	}
	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if ct == "text/event-stream" {
		if err := readEvents(resp.Body, match); err != nil && !errors.Is(err, errFound) {
			return nil, fmt.Errorf("mcp server %s: %w (%w)", t.name, err, errTransportDead)
		}
	} else {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("mcp server %s: %w (%w)", t.name, err, errTransportDead)
		}
		match(string(b))
	}
	if out.ID == nil {
		// The response ended without our id (aborted SSE stream, empty body):
		// the session is no longer trustworthy — restart and retry once.
		return nil, fmt.Errorf("mcp server %s: no response for %s: %w", t.name, method, errTransportDead)
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

// Notify posts a notification (no id). The status is checked: a session
// dropped between initialize and notifications/initialized must not read as
// success.
func (t *httpTransport) Notify(ctx context.Context, method string, params any) error {
	resp, err := t.post(ctx, request{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("mcp server %s: %w", t.name, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096)) //nolint:errcheck // keep the connection reusable
	if resp.StatusCode/100 != 2 {
		t.mu.Lock()
		expired := resp.StatusCode == http.StatusNotFound && t.session != ""
		t.mu.Unlock()
		if expired {
			return fmt.Errorf("mcp server %s: %w", t.name, errSessionExpired)
		}
		return fmt.Errorf("mcp server %s: HTTP %d for %s", t.name, resp.StatusCode, method)
	}
	return nil
}

// Close ends the session (best-effort: the server may be gone).
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

// Package mcp is moca's lazy MCP proxy (§10.5): JSON-RPC over stdio and
// streamable HTTP, a persisted discovery index, lazy server lifecycle, and
// the `mcp` tool. No SDK.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// errTransportDead and errSessionExpired are the transport-dead sentinels:
// Manager.Call retries exactly once on these, never on server-supplied error
// text (a server error message containing "exited" must not restart a healthy
// server).
var (
	errTransportDead  = errors.New("MCP transport is dead")
	errSessionExpired = errors.New("session expired")
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

// responseID parses a JSON-RPC response id. moca sends int64 ids; a server
// that echoes them as strings ("12") is still understood. null, fractions
// and arbitrary strings do not parse.
func responseID(raw *json.RawMessage) (int64, bool) {
	if raw == nil {
		return 0, false
	}
	var v any
	if json.Unmarshal(*raw, &v) != nil {
		return 0, false
	}
	switch id := v.(type) {
	case float64:
		if id == math.Trunc(id) {
			return int64(id), true
		}
	case string:
		if n, err := strconv.ParseInt(id, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// transport is one in-flight-capable message pipe: Call blocks for the
// response, Notify does not.
type transport interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close() error
}

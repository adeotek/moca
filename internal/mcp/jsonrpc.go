// Package mcp is moca's lazy MCP proxy (§10.5): JSON-RPC over stdio and
// streamable HTTP, a persisted discovery index, lazy server lifecycle, and
// the `mcp` tool. No SDK.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
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

// transport is one in-flight-capable message pipe: Call blocks for the
// response, Notify does not.
type transport interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close() error
}

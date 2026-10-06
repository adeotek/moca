package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/adeotek/moca/internal/config"
)

const protocolVersion = "2025-06-18"

type Annotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Annotations Annotations     `json:"annotations"`
}

type Content struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	MimeType string          `json:"mimeType,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

type CallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError"`
}

type client struct {
	t    transport
	name string
}

// initialize performs the MCP handshake: initialize, then the initialized
// notification.
func initialize(ctx context.Context, t transport) error {
	_, err := t.Call(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "moca", "version": config.Version},
	})
	if err != nil {
		return err
	}
	return t.Notify(ctx, "notifications/initialized", nil)
}

// maxToolPages caps tools/list pagination: a server that keeps handing out
// cursors (or the same one forever) must not grow the list without bound.
const maxToolPages = 100

// listTools fetches every page of tools/list.
func (c *client) listTools(ctx context.Context) ([]Tool, error) {
	var all []Tool
	cursor := ""
	seen := map[string]bool{}
	for page := 0; ; page++ {
		if page >= maxToolPages {
			return nil, fmt.Errorf("mcp server %s: tools/list did not finish within %d pages", c.name, maxToolPages)
		}
		var params any
		if cursor != "" {
			params = map[string]string{"cursor": cursor}
		}
		raw, err := c.t.Call(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var res struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" {
			return all, nil
		}
		if seen[res.NextCursor] {
			return nil, fmt.Errorf("mcp server %s: tools/list repeated cursor %q", c.name, res.NextCursor)
		}
		seen[res.NextCursor] = true
		cursor = res.NextCursor
	}
}

func (c *client) callTool(ctx context.Context, name string, args json.RawMessage) (CallResult, error) {
	// Omitted and explicit-null args both mean "no arguments": servers that
	// validate `arguments` as an object reject null.
	if a := bytes.TrimSpace(args); len(a) == 0 || bytes.Equal(a, []byte("null")) {
		args = json.RawMessage(`{}`)
	}
	raw, err := c.t.Call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return CallResult{}, err
	}
	var r CallResult
	err = json.Unmarshal(raw, &r)
	return r, err
}

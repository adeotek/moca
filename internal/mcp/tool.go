package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// cutRunes returns s shortened to at most n bytes on a rune boundary.
func cutRunes(s string, n int) string {
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// ProxyTool is the `mcp` tool (§10.5): a fixed ~200-token schema in front of
// every configured server's tools, which are never injected into the prompt.
type ProxyTool struct {
	m       *Manager
	approve map[string][]string
	mu      sync.Mutex
	session map[string]bool // allow-always in this session: "server/tool"
}

func NewTool(m *Manager, servers map[string]config.MCPServer) *ProxyTool {
	p := &ProxyTool{m: m, approve: map[string][]string{}, session: map[string]bool{}}
	for n, s := range servers {
		p.approve[n] = s.Approve
	}
	return p
}

// Spec returns the frozen phase-2 schema, byte-identical.
func (p *ProxyTool) Spec() llm.ToolSpec { return tools.MCPSpec() }

func isTrue(b *bool) bool { return b != nil && *b }

// Allowed: a read-only, non-destructive tool runs freely; everything else
// needs the server's approve list (or "*") or an allow-always from this
// session. No name-pattern heuristics.
func (p *ProxyTool) Allowed(server, tool string, a Annotations) bool {
	if isTrue(a.ReadOnlyHint) && !isTrue(a.DestructiveHint) {
		return true
	}
	if l := p.approve[server]; slices.Contains(l, "*") || slices.Contains(l, tool) {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.session[server+"/"+tool]
}

func errResult(format string, a ...any) tools.Result {
	return tools.Result{Content: fmt.Sprintf(format, a...), IsError: true}
}

func (p *ProxyTool) Run(ctx context.Context, env *tools.Env, input json.RawMessage) tools.Result {
	var a struct {
		Action string          `json:"action"`
		Server string          `json:"server"`
		Tool   string          `json:"tool"`
		Args   json.RawMessage `json:"args"`
		Query  string          `json:"query"`
	}
	if err := json.Unmarshal(input, &a); err != nil {
		return errResult("invalid arguments: %v", err)
	}
	switch a.Action {
	case "search":
		hits, notes, err := p.m.Search(ctx, a.Query, a.Server)
		if err != nil {
			return errResult("%v", err)
		}
		var sb strings.Builder
		if len(hits) == 0 {
			sb.WriteString("[no matching MCP tools]")
			for _, n := range notes {
				sb.WriteString("\n[" + n + "]")
			}
			return tools.Result{Content: sb.String(), Summary: "search " + a.Query}
		}
		for _, h := range hits {
			d := h.Description
			if len(d) > 160 {
				d = cutRunes(d, 157) + "..."
			}
			fmt.Fprintf(&sb, "%s/%s — %s\n", h.Server, h.Tool, strings.Join(strings.Fields(d), " "))
		}
		for _, n := range notes {
			sb.WriteString("[" + n + "]\n")
		}
		sb.WriteString("[describe before calling]")
		return tools.Result{Content: sb.String(), Summary: fmt.Sprintf("search %q (%d)", a.Query, len(hits))}
	case "describe":
		if a.Server == "" || a.Tool == "" {
			return errResult("action=describe needs server and tool")
		}
		t, err := p.m.Describe(ctx, a.Server, a.Tool)
		if err != nil {
			return errResult("%v", err)
		}
		var schema []byte
		if len(t.InputSchema) > 0 {
			var v any
			json.Unmarshal(t.InputSchema, &v)
			schema, _ = json.MarshalIndent(v, "", "  ")
		}
		return tools.Result{Content: fmt.Sprintf("%s/%s\n%s\nannotations: readOnly=%v destructive=%v\ninput schema:\n%s",
			a.Server, t.Name, t.Description, isTrue(t.Annotations.ReadOnlyHint), isTrue(t.Annotations.DestructiveHint), schema),
			Summary: "describe " + a.Server + "/" + a.Tool}
	case "call":
		if env.Plan {
			return errResult("refused: plan mode disables mcp calls (search/describe are fine) — write the plan to docs/plans/<name>.md, then stop")
		}
		if a.Server == "" || a.Tool == "" {
			return errResult("action=call needs server and tool")
		}
		t, err := p.m.Describe(ctx, a.Server, a.Tool) // starts the server, gives fresh annotations
		if err != nil {
			return errResult("%v", err)
		}
		if !p.Allowed(a.Server, a.Tool, t.Annotations) {
			ans := tools.Deny
			if env.Ask != nil {
				ans = env.Ask(ctx, tools.Question{Kind: "mcp", Subject: a.Server + "/" + a.Tool, Detail: string(a.Args), CanAlways: true})
			}
			switch ans {
			case tools.Deny:
				return errResult("refused: %s/%s is not marked read-only and is not in mcp.servers.%s.approve; the user did not approve it",
					a.Server, a.Tool, a.Server)
			case tools.AllowAlways:
				p.mu.Lock()
				p.session[a.Server+"/"+a.Tool] = true
				p.mu.Unlock()
			}
		}
		res, _, err := p.m.Call(ctx, a.Server, a.Tool, a.Args)
		if err != nil {
			return errResult("%v", err)
		}
		var parts []string
		for _, c := range res.Content {
			if c.Type == "text" {
				parts = append(parts, c.Text)
			} else {
				parts = append(parts, fmt.Sprintf("[%s omitted]", c.Type))
			}
		}
		out := tools.Truncate(strings.Join(parts, "\n"), 30_000)
		return tools.Result{Content: out, IsError: res.IsError, Summary: a.Server + "/" + a.Tool}
	}
	return errResult("action must be search, describe or call")
}

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

func newProxy(t *testing.T, approve []string) *ProxyTool {
	s := fakeServer("")
	s.Approve = approve
	servers := map[string]config.MCPServer{"docs": s}
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(servers, time.Minute, ix, Options{BaseEnv: os.Environ()})
	t.Cleanup(m.Close)
	return NewTool(m, servers)
}

func runP(p *ProxyTool, env *tools.Env, args map[string]any) tools.Result {
	b, _ := json.Marshal(args)
	return p.Run(context.Background(), env, b)
}

func TestProxySpecFrozen(t *testing.T) {
	a, _ := json.Marshal(newProxy(t, nil).Spec())
	b, _ := json.Marshal(tools.MCPSpec())
	if string(a) != string(b) {
		t.Fatal("proxy must return the frozen schema")
	}
}

func TestProxySearchDescribeCall(t *testing.T) {
	p := newProxy(t, nil)
	env := &tools.Env{}
	r := runP(p, env, map[string]any{"action": "search", "query": "documentation"})
	if !strings.Contains(r.Content, "docs/read_doc — Read library documentation") {
		t.Fatal(r.Content)
	}
	r = runP(p, env, map[string]any{"action": "describe", "server": "docs", "tool": "read_doc"})
	if !strings.Contains(r.Content, `"lib"`) || !strings.Contains(r.Content, "readOnly=true") {
		t.Fatal(r.Content)
	}
	r = runP(p, env, map[string]any{"action": "call", "server": "docs", "tool": "read_doc", "args": map[string]any{"lib": "go"}})
	if r.IsError || !strings.Contains(r.Content, "read_doc(map[lib:go])") || !strings.Contains(r.Content, "[image omitted]") {
		t.Fatal(r)
	}
}

func TestProxyGating(t *testing.T) {
	p := newProxy(t, nil)
	r := runP(p, &tools.Env{}, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"})
	if !r.IsError || !strings.Contains(r.Content, "mcp.servers.docs.approve") {
		t.Fatal("non-read-only refused without asker (-p)", r.Content)
	}
	var asked []tools.Question
	env := &tools.Env{Ask: func(_ context.Context, q tools.Question) tools.Answer {
		asked = append(asked, q)
		return tools.AllowAlways
	}}
	if r := runP(p, env, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"}); r.IsError {
		t.Fatal(r.Content)
	}
	runP(p, env, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"})
	if len(asked) != 1 || asked[0].Kind != "mcp" || asked[0].Subject != "docs/create_issue" {
		t.Fatal("allow-always sticks for the session", asked)
	}
	if r := runP(newProxy(t, []string{"*"}), &tools.Env{}, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"}); r.IsError {
		t.Fatal(`approve ["*"] trusts the server`)
	}
	if r := runP(newProxy(t, nil), &tools.Env{Ask: tools.AutoAllow}, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"}); r.IsError {
		t.Fatal("yolo (AutoAllow asker) runs non-read-only tools without asking")
	}
}

func TestProxyArgErrors(t *testing.T) {
	p := newProxy(t, nil)
	if r := runP(p, &tools.Env{}, map[string]any{"action": "describe", "server": "docs"}); !r.IsError || !strings.Contains(r.Content, "tool") {
		t.Fatal(r.Content)
	}
	if r := runP(p, &tools.Env{}, map[string]any{"action": "search", "server": "zzz"}); !r.IsError || !strings.Contains(r.Content, "configured: docs") {
		t.Fatal(r.Content)
	}
}

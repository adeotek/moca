package mcp

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/tools"
)

// TestProxyPlanModeRefusesCall: plan mode is a scope, not a permission — even
// an approved or read-only tool cannot be called; search/describe stay open.
func TestProxyPlanModeRefusesCall(t *testing.T) {
	p := newProxy(t, []string{"create_issue"})
	env := &tools.Env{Ask: tools.AutoAllow, Plan: true}
	r := runP(p, env, map[string]any{"action": "call", "server": "docs", "tool": "read_doc", "args": map[string]any{"lib": "go"}})
	if !r.IsError || !strings.Contains(r.Content, "plan mode disables mcp calls") {
		t.Fatalf("plan mode must refuse call: %+v", r)
	}
	if r := runP(p, env, map[string]any{"action": "search", "query": "documentation"}); r.IsError {
		t.Fatalf("search in plan mode: %+v", r)
	}
	if r := runP(p, env, map[string]any{"action": "describe", "server": "docs", "tool": "read_doc"}); r.IsError {
		t.Fatalf("describe in plan mode: %+v", r)
	}
}

package mcp

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/tools"
)

// TestProxyPlanModeCallFollowsNormalGating: plan mode does not block mcp —
// calls run under the ordinary read-only/approve/session gating.
func TestProxyPlanModeCallFollowsNormalGating(t *testing.T) {
	p := newProxy(t, nil)
	// Read-only tool: passes with no asker while plan mode is on.
	r := runP(p, &tools.Env{Plan: true}, map[string]any{"action": "call", "server": "docs", "tool": "read_doc", "args": map[string]any{"lib": "go"}})
	if r.IsError || !strings.Contains(r.Content, "read_doc(map[lib:go])") {
		t.Fatalf("read-only call in plan mode: %+v", r)
	}
	// Non-read-only, not approved, no asker: the normal gate still refuses.
	r = runP(p, &tools.Env{Plan: true}, map[string]any{"action": "call", "server": "docs", "tool": "create_issue"})
	if !r.IsError || !strings.Contains(r.Content, "mcp.servers.docs.approve") {
		t.Fatalf("normal gating must still apply in plan mode: %+v", r)
	}
	// Search and describe stay available too.
	if r := runP(p, &tools.Env{Plan: true}, map[string]any{"action": "search", "query": "documentation"}); r.IsError {
		t.Fatalf("search in plan mode: %+v", r)
	}
	if r := runP(p, &tools.Env{Plan: true}, map[string]any{"action": "describe", "server": "docs", "tool": "read_doc"}); r.IsError {
		t.Fatalf("describe in plan mode: %+v", r)
	}
}

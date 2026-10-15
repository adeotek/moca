package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// writeProjectFile writes one project-level MCP source file.
func writeProjectFile(t *testing.T, cwd, name, body string) string {
	t.Helper()
	p := filepath.Join(cwd, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A trusted project's ecosystem files add servers; moca's user config wins a
// name both define, and the warning names the server and both files.
func TestMergeProjectEcosystemSources(t *testing.T) {
	cwd := t.TempDir()
	claude := writeProjectFile(t, cwd, ".mcp.json", `{"mcpServers":{
		"devtools": {"command":"npx","args":["-y","devtools"],"env":{"TOKEN":"${PROJECT_TOKEN}","PLAIN":"v"}},
		"github":   {"type":"http","url":"https://project.example/mcp"},
		"disabled": {"command":"x","enabled":false},
		"legacy":   {"type":"sse","url":"https://old"}}}`)
	oc := writeProjectFile(t, cwd, "opencode.json", `{"mcp":{
		"docs": {"type":"local","command":["uvx","docs-mcp"],"environment":{"KEY":"k"}},
		"devtools": {"type":"remote","url":"https://opencode.example/mcp"}}}`)
	user := map[string]config.MCPServer{"github": {URL: "https://user.example/mcp"}}
	merged, warns := MergeProject(cwd, "/home/u/config.jsonc", filepath.Join(cwd, ".moca", "config.jsonc"), user, config.MCPConfig{})

	if s := merged["devtools"]; s.Command != "npx" || s.Env["TOKEN"] != "env:PROJECT_TOKEN" || s.Env["PLAIN"] != "v" {
		t.Fatalf("devtools: %+v", s)
	}
	if s := merged["docs"]; s.Command != "uvx" || s.Env["KEY"] != "k" {
		t.Fatalf("opencode project server: %+v", s)
	}
	if s := merged["github"]; s.URL != "https://user.example/mcp" {
		t.Fatalf("the user config must win: %+v", s)
	}
	if _, ok := merged["disabled"]; ok {
		t.Fatal("a disabled server must not load")
	}
	if _, ok := merged["legacy"]; ok {
		t.Fatal("a legacy SSE server must not load")
	}
	j := strings.Join(warns, "\n")
	if !strings.Contains(j, `mcp server "github" is defined in`) ||
		!strings.Contains(j, claude) || !strings.Contains(j, "/home/u/config.jsonc") {
		t.Fatalf("github conflict warning: %v", warns)
	}
	if !strings.Contains(j, `"devtools"`) || !strings.Contains(j, oc) {
		t.Fatalf("devtools conflict warning (opencode vs claude-code): %v", warns)
	}
	if lj := strings.ToLower(j); !strings.Contains(lj, "disabled") || !strings.Contains(lj, "sse") {
		t.Fatalf("skip notes: %v", warns)
	}
}

// The project config file's mcp block is the most specific layer: it adds its
// own servers, overrides a definition it names (with a warning), and patches
// one another layer owns — the approve lists unite.
func TestMergeProjectConfigFile(t *testing.T) {
	cwd := t.TempDir()
	writeProjectFile(t, cwd, ".mcp.json", `{"mcpServers":{"shared":{"command":"from-mcp-json","args":["a"]},"patched":{"command":"ecosystem","description":"eco"}}}`)
	proj := config.MCPConfig{Servers: map[string]config.MCPServer{
		"shared":  {Command: "from-moca", Args: []string{"b"}, Description: "project"},
		"own":     {URL: "https://own/mcp"},
		"patched": {Approve: []string{"send"}}, // approve-only: how a project-scoped allow-always persists
		"patchme": {Approve: []string{"write_thing"}},
		"orphan":  {Approve: []string{"x"}},
	}}
	user := map[string]config.MCPServer{
		"github": {URL: "https://x", Approve: []string{"list_issues"}},
	}
	merged, warns := MergeProject(cwd, "/u/config.jsonc", "/p/.moca/config.jsonc", user, proj)

	if s := merged["shared"]; s.Command != "from-moca" || len(s.Args) != 1 || s.Args[0] != "b" || s.Description != "project" {
		t.Fatalf("project override: %+v", s)
	}
	if s := merged["patched"]; s.Command != "ecosystem" || s.Description != "eco" || len(s.Approve) != 1 || s.Approve[0] != "send" {
		t.Fatalf("approve-only patch over an ecosystem server: %+v", s)
	}
	if s := merged["own"]; s.URL != "https://own/mcp" {
		t.Fatalf("project server: %+v", s)
	}
	if _, ok := merged["patchme"]; ok {
		t.Fatal("a patch with no base must be skipped")
	}
	j := strings.Join(warns, "\n")
	if !strings.Contains(j, `mcp server "shared" is defined in`) || !strings.Contains(j, "the project config wins") {
		t.Fatalf("override warning: %v", warns)
	}
	if !strings.Contains(j, "orphan") || !strings.Contains(j, "no other layer defines") {
		t.Fatalf("orphan warning: %v", warns)
	}
	if strings.Contains(j, `"patched"`) {
		t.Fatalf("an approve-only patch is not a conflict: %v", warns)
	}

	// The merge is repeatable over its own output: a patch entry written for
	// a user server keeps that server and unions the approvals.
	proj2 := config.MCPConfig{Servers: map[string]config.MCPServer{"github": {Approve: []string{"create_issue", "list_issues"}}}}
	merged2, warns2 := MergeProject(cwd, "/u/config.jsonc", "/p/.moca/config.jsonc", user, proj2)
	if s := merged2["github"]; s.URL != "https://x" || len(s.Approve) != 2 || s.Approve[0] != "list_issues" {
		t.Fatalf("approve union: %+v", s)
	}
	if len(warns2) != 0 {
		t.Fatalf("a patch is not a conflict: %v", warns2)
	}
}

// A project entry that switches transports replaces the other one outright —
// a server with both command and url could never start.
func TestMergeProjectTransportSwap(t *testing.T) {
	cwd := t.TempDir()
	writeProjectFile(t, cwd, ".mcp.json", `{"mcpServers":{"s":{"command":"old","args":["-x"]}}}`)
	proj := config.MCPConfig{Servers: map[string]config.MCPServer{"s": {URL: "https://new/mcp"}}}
	merged, _ := MergeProject(cwd, "/u/config.jsonc", "/p/.moca/config.jsonc", nil, proj)
	if s := merged["s"]; s.URL != "https://new/mcp" || s.Command != "" || len(s.Args) != 0 {
		t.Fatalf("transport swap: %+v", s)
	}
}

// The user config wins a name it defines: a project entry cannot swap the
// command, args or env of a user server (it would inherit the user's
// approvals, "*" included) — it may only add approvals, and a redefinition
// warns naming both files.
func TestMergeProjectUserServerWins(t *testing.T) {
	cwd := t.TempDir()
	user := map[string]config.MCPServer{"github": {Command: "gh-mcp", Approve: []string{"*"}}}
	proj := config.MCPConfig{Servers: map[string]config.MCPServer{"github": {
		Command: "./evil", Env: map[string]string{"NODE_OPTIONS": "--require ./x"}, Approve: []string{"create_issue"},
	}}}
	merged, warns := MergeProject(cwd, "/u/config.jsonc", "/p/.moca/config.jsonc", user, proj)
	s := merged["github"]
	if s.Command != "gh-mcp" || s.Env != nil || len(s.Approve) != 2 || s.Approve[1] != "create_issue" {
		t.Fatalf("user server rewritten: %+v", s)
	}
	j := strings.Join(warns, "\n")
	if !strings.Contains(j, `"github"`) || !strings.Contains(j, "/u/config.jsonc") || !strings.Contains(j, "/p/.moca/config.jsonc") {
		t.Fatalf("conflict warning: %v", warns)
	}
}

// ProjectSources lists only the project-scoped files that exist (the same
// list `moca mcp import` discovers from a cwd).
func TestProjectSources(t *testing.T) {
	cwd := t.TempDir()
	writeProjectFile(t, cwd, ".mcp.json", `{"mcpServers":{}}`)
	writeProjectFile(t, cwd, filepath.Join(".pi", "mcp.json"), `{"mcpServers":{}}`)
	got := ProjectSources(cwd)
	if len(got) != 2 || got[0].Path != filepath.Join(cwd, ".mcp.json") || got[1].Path != filepath.Join(cwd, ".pi", "mcp.json") {
		t.Fatalf("sources: %+v", got)
	}
	if len(ProjectSources(t.TempDir())) != 0 {
		t.Fatal("no files, no sources")
	}
	// The importer discovers them too (project files right after ~/.claude.json).
	all := DiscoverSources(t.TempDir(), cwd)
	if len(all) != 2 || all[0].Path != filepath.Join(cwd, ".mcp.json") {
		t.Fatalf("discovery: %+v", all)
	}
}

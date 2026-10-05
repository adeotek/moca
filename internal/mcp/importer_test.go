package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

func TestImportPlan(t *testing.T) {
	srcs := []Source{{"claude-code", "testdata/claude.json"}, {"opencode", "testdata/opencode.jsonc"}, {"pi", "testdata/pi-mcp.json"}}
	adds, vars, notes := Plan(srcs, map[string]config.MCPServer{"context7": {}}, "/work/app")
	if _, ok := adds["old"]; ok {
		t.Fatal("legacy SSE skipped")
	}
	if _, ok := adds["off"]; ok {
		t.Fatal("disabled skipped")
	}
	if _, ok := adds["context7"]; ok {
		t.Fatal("existing kept")
	}
	gh := adds["github"]
	if gh.Env["GITHUB_PERSONAL_ACCESS_TOKEN"] != "env:MOCA_MCP_GITHUB_GITHUB_PERSONAL_ACCESS_TOKEN" || gh.Env["LOG_LEVEL"] != "info" {
		t.Fatal(gh.Env)
	}
	if adds["linear"].Headers["Authorization"] != "env:MOCA_MCP_LINEAR_AUTHORIZATION" || adds["linear"].URL == "" {
		t.Fatal(adds["linear"])
	}
	if adds["pg"].Env["DATABASE_URL"] != "env:DATABASE_URL" {
		t.Fatal("${VAR} → env:VAR", adds["pg"])
	}
	if fs := adds["fs"]; fs.Command != "npx" || len(fs.Args) != 3 || fs.Env["API_KEY"] != "env:MOCA_MCP_FS_API_KEY" {
		t.Fatal(fs)
	}
	if adds["brave"].Env["BRAVE_API_KEY"] != "env:MOCA_MCP_BRAVE_BRAVE_API_KEY" {
		t.Fatal("pi source")
	}
	for _, s := range adds {
		for _, v := range s.Env {
			if strings.Contains(v, "ghp_") || strings.Contains(v, "BSA123") || strings.Contains(v, "lin_abc") {
				t.Fatal("literal secret copied")
			}
		}
		for _, v := range s.Headers {
			if strings.Contains(v, "lin_abc") {
				t.Fatal("literal secret copied")
			}
		}
	}
	if len(vars) != 4 { // github, linear, fs, brave — the skipped duplicate pi/github adds none
		t.Fatalf("vars to export: %+v", vars)
	}
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "old") || !strings.Contains(joined, "context7") || !strings.Contains(joined, "github") {
		t.Fatal(notes)
	}
}

func TestDiscoverSources(t *testing.T) {
	home, cwd := t.TempDir(), t.TempDir()
	for _, p := range []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".pi", "agent", "mcp.json"),
		filepath.Join(home, ".agents", "mcp", "mcp.json"),
		filepath.Join(cwd, ".pi", "mcp.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	discovered := map[string]bool{}
	for _, s := range DiscoverSources(home, cwd) {
		discovered[s.Path] = true
	}
	for _, want := range []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".pi", "agent", "mcp.json"),
		filepath.Join(home, ".agents", "mcp", "mcp.json"),
		filepath.Join(cwd, ".pi", "mcp.json"),
	} {
		if !discovered[want] {
			t.Errorf("not discovered: %s (got %v)", want, discovered)
		}
	}
	if discovered[filepath.Join(home, ".config", "mcp", "mcp.json")] {
		t.Error("missing files must not be listed")
	}
}

// Pi's own file shape: stdio/http/streamable-http types, enabled:false and
// sse skipped (pi-mcp-adapter translations).
func TestParsePiShape(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pi.json")
	if err := os.WriteFile(p, []byte(`{"mcpServers":{
		"docs": {"type":"streamable-http","url":"https://x/mcp"},
		"off": {"command":"x","enabled":false},
		"old": {"type":"sse","url":"https://y"},
		"env-ref": {"command":"z","env":{"URL":"${MY_URL}"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	servers, notes, err := ParseSource(Source{"pi", p}, "/w")
	if err != nil || len(servers) != 2 || servers["docs"].URL != "https://x/mcp" {
		t.Fatal(servers, notes, err)
	}
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "off") || !strings.Contains(joined, "old") {
		t.Fatal(notes)
	}
	if _, v := RewriteSecrets("env-ref", servers["env-ref"]); len(v) != 0 {
		t.Fatal("${VAR} needs no export", v)
	}
	if s, _ := RewriteSecrets("env-ref", servers["env-ref"]); s.Env["URL"] != "env:MY_URL" {
		t.Fatal(s.Env)
	}
}

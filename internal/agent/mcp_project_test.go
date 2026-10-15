package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// startProjectMCP starts an agent over a workdir with a project .mcp.json and
// a project config file, returning the agent, the event sink and the workdir.
func startProjectMCP(t *testing.T, trusted bool, projConfig string) (*Agent, *[]Event, string) {
	t.Helper()
	s := newScript(t, textTurn("ok"))
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	work := t.TempDir()
	marker := filepath.Join(t.TempDir(), "spawned")
	// The project's own Claude Code config: two servers, one of which the
	// user config also defines (a conflict), plus a marker command nothing
	// may ever run at session start.
	mcpJSON := fmt.Sprintf(`{"mcpServers":{
		"projdocs": {"command":"sh","args":["-c","touch %s"],"description":"project docs"},
		"shared":   {"command":"sh","args":["-c","true"],"description":"from .mcp.json"}}}`, marker)
	if err := os.WriteFile(filepath.Join(work, ".mcp.json"), []byte(mcpJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if projConfig != "" {
		if err := os.MkdirAll(filepath.Join(work, ".moca"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(config.ProjectConfigFile(work), []byte(projConfig), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m",
		"mcp":{"servers":{"shared":{"command":"sh","args":["-c","true"],"description":"from the user config"}}},
		"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":65536}}}}}`,
		s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	// cmd's order: the trust decision, then the project file (only when
	// trusted), then Start.
	if trusted {
		p, err := config.LoadProject(work)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Project = p
	}
	var evs []Event
	o := StartOptions{Config: cfg, Workdir: work, Trusted: trusted, HTTP: s.srv.Client(), Slug: "t",
		ConfigPath: filepath.Join(t.TempDir(), "config.jsonc"), Emit: func(e Event) { evs = append(evs, e) }}
	a, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	if _, err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a project MCP server was spawned at session start")
	}
	return a, &evs, work
}

// A trusted project's MCP sources join the session (§10.5): its own servers
// are in the prompt roster, the user config keeps the name it defines (with a
// warning naming both files), and nothing is started eagerly.
func TestProjectMCPServersJoinTheUserLayer(t *testing.T) {
	a, evs, work := startProjectMCP(t, true, `{"mcp":{"servers":{"projcfg":{"url":"https://proj.example/mcp","description":"from the project config"}}}}`)
	servers := a.opts.Config.MCP.Servers
	if _, ok := servers["projdocs"]; !ok {
		t.Fatalf("the project .mcp.json server is missing: %v", keys(servers))
	}
	if _, ok := servers["projcfg"]; !ok {
		t.Fatalf("the project config server is missing: %v", keys(servers))
	}
	if s := servers["shared"]; s.Description != "from the user config" {
		t.Fatalf("the user config must win a name conflict: %+v", s)
	}
	sys := a.opts.System
	for _, want := range []string{"- projdocs: project docs", "- projcfg: from the project config", "- shared: from the user config"} {
		if !strings.Contains(sys, want) {
			t.Errorf("prompt roster missing %q", want)
		}
	}
	joined := warnings(*evs)
	if !strings.Contains(joined, `mcp server "shared" is defined in`) ||
		!strings.Contains(joined, filepath.Join(work, ".mcp.json")) {
		t.Fatalf("conflict warning: %v", joined)
	}
}

// Untrusted: the same workdir contributes nothing — no servers, no warnings.
func TestProjectMCPServersNeedTrust(t *testing.T) {
	a, evs, _ := startProjectMCP(t, false, `{"mcp":{"servers":{"projcfg":{"url":"https://proj.example/mcp"}}}}`)
	servers := a.opts.Config.MCP.Servers
	if len(servers) != 1 {
		t.Fatalf("only the user server may load untrusted: %v", keys(servers))
	}
	if joined := warnings(*evs); joined != "" {
		t.Fatalf("no project layer, no warnings: %v", joined)
	}
}

// A trusted project's mcp.idleTimeout applies even when the project configures
// no server of its own (§4); an untrusted workdir's never does — the TUI keeps
// a project layer in memory there (allow-always writes one), so the trust guard
// is what keeps it out. Mutation: drop the `o.Trusted &&` in prepare and the
// second case below fails.
func TestProjectMCPIdleTimeout(t *testing.T) {
	start := func(trusted bool) int {
		t.Helper()
		s := newScript(t, textTurn("ok"))
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("MOCA_T_KEY", "k")
		cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m",
			"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":65536}}}}}`,
			s.srv.URL)))
		if err != nil {
			t.Fatal(err)
		}
		cfg.Project = &config.ProjectConfig{MCP: config.MCPConfig{IdleTimeout: 45}}
		a, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), Trusted: trusted, Slug: "t", HTTP: s.srv.Client()})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { a.Close() })
		return a.opts.Config.MCP.IdleTimeout
	}
	if got := start(true); got != 45 {
		t.Fatalf("a trusted project's idleTimeout = %d, want 45", got)
	}
	if got := start(false); got != 600 { // config.Parse's default
		t.Fatalf("an untrusted workdir's project idleTimeout leaked: %d, want the user default 600", got)
	}
}

func warnings(evs []Event) string {
	var sb strings.Builder
	for _, e := range evs {
		if w, ok := e.(Warning); ok {
			sb.WriteString(w.Text + "\n")
		}
	}
	return sb.String()
}

func keys(m map[string]config.MCPServer) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProject(t *testing.T) {
	wd := t.TempDir()
	if p, err := LoadProject(wd); p != nil || err != nil {
		t.Fatalf("absent file: %v %v", p, err)
	}
	if err := os.MkdirAll(filepath.Join(wd, ".moca"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		if err := os.WriteFile(ProjectConfigFile(wd), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(`{ // project
	  "permissions": {"defaultMode": "auto", "allow": ["make *"], "deny": ["git push *"],},
	}`)
	p, err := LoadProject(wd)
	if err != nil || p.Mode() != "auto" || p.Permissions.Allow[0] != "make *" {
		t.Fatalf("%+v %v", p, err)
	}
	for src, want := range map[string]string{
		`{"model":"x/y"}`:                                 "unknown field",
		`{"permissions":{"defaultMode":"yolo"}}`:          "yolo",
		`{"permissions":{"allow":["*"]}}`:                 `"*"`,
		`{"permissions":{"allow":["**"]}}`:                "matches every command",
		`{"permissions":{"allow":["* *"]}}`:               "matches every command",
		`{"permissions":{"allow":["* rm"]}}`:              "matches every command",
		`{"permissions":{"allow":["  *"]}}`:               "matches every command",
		`{"permissions":{"auto":{"model":"openai/gpt"}}}`: "auto.model",
		`{"mcp":{"bogus":1}}`:                             "unknown field",
		`{"mcp":{"idleTimeout":-1}}`:                      "must not be negative",
		`{"mcp":{"servers":{"x":{"bogus":1}}}}`:           "unknown field",
		// A patch entry (no command, no url) is legal — it is what a
		// project-scoped allow-always persists (§10.5).
		`{"mcp":{"servers":{"x":{}}}}`:                                      "",
		`{"mcp":{"servers":{"x":{"command":"a","url":"https://b"}}}}`:       "mutually exclusive",
		`{"mcp":{"servers":{"x":{"url":"https://b","approve":["a","b"]}}}}`: "",
		`{"mcp":{"servers":{"x":{"url":"https://b","approve":["*"]}}}}`:     "blanket-approve",
		`{"mcp":{"servers":{"x":{"url":"https://b","approve":[" "]}}}}`:     "empty tool name",
		`{"mcp":{"servers":{"x":{"command":"npx","env":{"K":"env:V"}}}}}`:   "",
		`{"permissions":{"allow":["make *"]},"mcp":{"idleTimeout":120}}`:    "",
	} {
		write(src)
		_, err := LoadProject(wd)
		switch {
		case want == "" && err != nil:
			t.Errorf("%s: unexpected error %v", src, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%s: err = %v, want %q", src, err, want)
		}
	}
	for _, ok := range []string{`["make *"]`, `["git status"]`} {
		write(`{"permissions":{"allow":` + ok + `}}`)
		if _, err := LoadProject(wd); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
}

// The project file's mcp block is a full definition or a patch (§10.5): the
// parsed values survive to the caller, including a patch entry that only
// carries an approval.
func TestLoadProjectMCP(t *testing.T) {
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, ".moca"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
	  // a project's servers: one of its own, one HTTP, one approval patch
	  "permissions": {"allow": ["make *"]},
	  "mcp": {
	    "idleTimeout": 120,
	    "servers": {
	      "docs": {"command": "npx", "args": ["-y", "docs"], "env": {"TOKEN": "env:T"}, "description": "docs lookup"},
	      "web": {"url": "https://mcp.example/mcp", "headers": {"Authorization": "Bearer ${WEB_TOKEN}"}},
	      "github": {"approve": ["create_issue"]},
	    },
	  },
	}`
	if err := os.WriteFile(ProjectConfigFile(wd), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProject(wd)
	if err != nil {
		t.Fatal(err)
	}
	if p.MCP.IdleTimeout != 120 || len(p.MCP.Servers) != 3 {
		t.Fatalf("mcp block: %+v", p.MCP)
	}
	if s := p.MCP.Servers["docs"]; s.Command != "npx" || s.Env["TOKEN"] != "env:T" || s.Description != "docs lookup" {
		t.Fatalf("docs: %+v", s)
	}
	if s := p.MCP.Servers["web"]; s.URL != "https://mcp.example/mcp" || s.Headers["Authorization"] != "Bearer ${WEB_TOKEN}" {
		t.Fatalf("web: %+v", s)
	}
	if s := p.MCP.Servers["github"]; s.Command != "" || s.URL != "" || len(s.Approve) != 1 {
		t.Fatalf("patch entry: %+v", s)
	}
	if p.Permissions.Allow[0] != "make *" {
		t.Fatalf("permissions block lost: %+v", p.Permissions)
	}
}

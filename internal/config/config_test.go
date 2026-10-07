// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSpecExampleDecodesIntact(t *testing.T) {
	data, err := os.ReadFile("testdata/example.jsonc")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(data)
	if err != nil {
		t.Fatalf("§12 example must decode: %v", err)
	}
	if c.Model != "opencode-go/glm-5.3-flash" || c.ModelHard != "opencode-go/glm-5.3" {
		t.Fatalf("models: %q %q", c.Model, c.ModelHard)
	}
	if got := c.MCP.Servers["context7"].URL; got != "https://mcp.context7.com/mcp" {
		t.Fatalf("URL mangled: %q", got)
	}
	if c.MCP.Servers["context7"].Description != "library/API docs lookup" {
		t.Fatal("description lost")
	}
	if c.Providers["anthropic"].APIKey != "env:ANTHROPIC_API_KEY" {
		t.Fatal("apiKey lost")
	}
	if !slices.Contains(c.Shell.Allow, "graphify") || len(c.Shell.Allow) != 32 {
		t.Fatalf("allow list: %v", c.Shell.Allow)
	}
	if c.MCP.IdleTimeout != 600 || c.RetentionDays() != 30 {
		t.Fatal("scalars lost")
	}
	if c.Context != (ContextConfig{ReserveTokens: 16384, KeepRecentTokens: 20000, MaxSteps: 40}) {
		t.Fatalf("context: %+v", c.Context)
	}
	if c.Yolo {
		t.Fatal("example ships yolo off")
	}
	if y, _ := Parse([]byte(`{"yolo": true}`)); !y.Yolo {
		t.Fatal("yolo key decodes")
	}
}

func TestDefaultsApplied(t *testing.T) {
	c, err := Parse([]byte(`{"model":"anthropic/x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Context.ReserveTokens != 16384 || c.Context.KeepRecentTokens != 20000 || c.Context.MaxSteps != 40 {
		t.Fatalf("context defaults: %+v", c.Context)
	}
	if c.MCP.IdleTimeout != 600 || c.RetentionDays() != 30 {
		t.Fatal("scalar defaults")
	}
	if !slices.Equal(c.Shell.Allow, DefaultShellAllow) {
		t.Fatal("default allowlist")
	}
	if c.Providers["anthropic"].APIKey != "env:ANTHROPIC_API_KEY" ||
		c.Providers["openai"].APIKey != "env:OPENAI_API_KEY" ||
		c.Providers["opencode-go"].APIKey != "env:OPENCODE_API_KEY" {
		t.Fatalf("built-in provider defaults: %+v", c.Providers)
	}
}

func TestRetentionZeroMeansForever(t *testing.T) {
	c, err := Parse([]byte(`{"snapshot":{"retentionDays":0}}`))
	if err != nil || c.RetentionDays() != 0 {
		t.Fatalf("retention 0 must survive defaults: %v %d", err, c.RetentionDays())
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		`{"model":"glm-5.3"}`: "provider/model",
		`{"model":"foo/bar"}`: "unknown provider \"foo\"",
		`{"providers":{"anthropic":{"auth":"api_key","apiKey":"sk-x"}}}`:                                                                                           "env:",
		`{"providers":{"anthropic":{"auth":"magic"}}}`:                                                                                                             "auth",
		`{"providers":{"anthropic":{"auth":"oauth"}}}`:                                                                                                             "not available",
		`{"providers":{"vllm":{"auth":"api_key","apiKey":"env:K"}}}`:                                                                                               "baseUrl",
		`{"providers":{"vllm":{"baseUrl":"http://x/v1","protocol":"openai-completions","auth":"api_key","apiKey":"env:K","models":{"m":{}}}}}`:                     "contextWindow",
		`{"providers":{"vllm":{"baseUrl":"http://x/v1","protocol":"openai-completions","auth":"api_key","apiKey":"env:K","models":{"m":{"contextWindow":8192}}}}}`: "16384",
		`{"providers":{"vllm":{"baseUrl":"http://x/v1","protocol":"grpc","auth":"api_key","apiKey":"env:K"}}}`:                                                     "protocol",
		`{"modle":"x/y"}`:             "unknown field",
		`{"context":{"maxSteps":-1}}`: "maxSteps",
		`{"mcp":{"idleTimeout":-5}}`:  "mcp.idleTimeout",
	}
	for in, want := range cases {
		_, err := Parse([]byte(in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%s) err = %v, want containing %q", in, err, want)
		}
	}
}

func TestOAuthAllowedForOpenAI(t *testing.T) {
	c, err := Parse([]byte(`{"providers":{"openai":{"auth":"oauth"}}}`))
	if err != nil || c.Providers["openai"].Auth != "oauth" {
		t.Fatalf("openai oauth must parse: %v", err)
	}
}

func TestOAuthRejectedForNonOAuthProvider(t *testing.T) {
	// OAuth exists only where the policy gate permits it: `auth: "oauth"`
	// for anything else is a config error at load, not a request-time
	// failure with a runtime exit code.
	if _, err := Parse([]byte(`{"providers":{"opencode-go":{"auth":"oauth"}}}`)); err == nil || !strings.Contains(err.Error(), "no OAuth support") {
		t.Fatalf("oauth on a non-OAuth provider must be refused: %v", err)
	}
	if _, err := Parse([]byte(`{"providers":{"mycorp":{"auth":"oauth","baseUrl":"http://x","protocol":"openai-responses"}}}`)); err == nil || !strings.Contains(err.Error(), "no OAuth support") {
		t.Fatalf("oauth on a custom provider must be refused: %v", err)
	}
}

func TestSyntaxErrorReportsLineCol(t *testing.T) {
	// `x` is a genuine syntax error on line 3. (A trailing comma — e.g.
	// `"model": ,` — is legitimately blanked by the pre-pass, which moves
	// the decode error to the following `}` line; not what this test pins.)
	_, err := Parse([]byte("{\n  // c\n  \"model\": x,\n}"))
	if err == nil || !strings.Contains(err.Error(), "3:") {
		t.Fatalf("want line 3 in error, got %v", err)
	}
}

func TestSplitModel(t *testing.T) {
	p, m, err := SplitModel("openai/gpt-x/variant")
	if err != nil || p != "openai" || m != "gpt-x/variant" {
		t.Fatalf("%q %q %v", p, m, err)
	}
	for _, bad := range []string{"", "x", "/m", "p/"} {
		if _, _, err := SplitModel(bad); err == nil {
			t.Errorf("SplitModel(%q) must fail", bad)
		}
	}
}

func TestResolveEnv(t *testing.T) {
	t.Setenv("MOCA_T_KEY", "v")
	if v, err := ResolveEnv("env:MOCA_T_KEY"); err != nil || v != "v" {
		t.Fatal(v, err)
	}
	t.Setenv("MOCA_T_EMPTY", "")
	if _, err := ResolveEnv("env:MOCA_T_EMPTY"); err == nil || !strings.Contains(err.Error(), "MOCA_T_EMPTY") {
		t.Fatalf("empty var must name the variable: %v", err)
	}
	if v, err := ResolveEnv("literal"); err != nil || v != "literal" {
		t.Fatal("non-env strings pass through (headers may be literal)")
	}
}

func TestEnvRefs(t *testing.T) {
	c, _ := Parse([]byte(`{"mcp":{"servers":{"gh":{"url":"https://x","headers":{"Authorization":"env:GH_TOK"}},"fs":{"command":"x","env":{"A":"env:FS_KEY","B":"lit"}}}}}`))
	refs := EnvRefs(c)
	for _, want := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENCODE_API_KEY", "GH_TOK", "FS_KEY"} {
		if !slices.Contains(refs, want) {
			t.Errorf("EnvRefs missing %s: %v", want, refs)
		}
	}
}

func TestLoadMissingFileIsDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.jsonc"))
	if err != nil || c.Context.MaxSteps != 40 {
		t.Fatal(err)
	}
}

func TestPathsHonourXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_DATA_HOME", "/data")
	if ConfigFile() != "/cfg/moca/config.jsonc" || DataDir() != "/data/moca" {
		t.Fatal(ConfigFile(), DataDir())
	}
}

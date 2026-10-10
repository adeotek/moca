package config

import (
	"strings"
	"testing"
)

func TestWebConfigValidation(t *testing.T) {
	// Default: tavily with no key (keyless mode).
	c, err := Parse([]byte(`{"model":"anthropic/x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Web.Search.Provider != "tavily" || c.Web.Search.APIKey != "" {
		t.Fatalf("web defaults: %+v", c.Web)
	}
	bad := []struct{ cfg, want string }{
		{`{"web":{"search":{"provider":"exa"}}}`, "exa"},
		{`{"web":{"search":{"apiKey":"sk-literal"}}}`, "env:"},
		{`{"web":{"search":{"provider":"bing"}}}`, "unknown"},
		{`{"web":{"search":{"providers":"tavily"}}}`, "providers"}, // strict decode: unknown key
	}
	for _, b := range bad {
		if _, err := Parse([]byte(b.cfg)); err == nil || !strings.Contains(err.Error(), b.want) {
			t.Fatalf("%s: want error containing %q, got %v", b.cfg, b.want, err)
		}
	}
	c, err = Parse([]byte(`{"model":"anthropic/x","web":{"search":{"provider":"exa","apiKey":"env:EXA_API_KEY"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Web.Search.Provider != "exa" || c.Web.Search.APIKey != "env:EXA_API_KEY" {
		t.Fatalf("web exa decode: %+v", c.Web)
	}
}

func TestWebSearchKeyRefStrippedFromShellEnv(t *testing.T) {
	c, err := Parse([]byte(`{"model":"anthropic/x","web":{"search":{"apiKey":"env:TAVILY_API_KEY"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	refs := EnvRefs(c)
	found := false
	for _, r := range refs {
		if r == "TAVILY_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Fatalf("web.search.apiKey env ref missing from EnvRefs: %v", refs)
	}
}

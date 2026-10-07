package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendStringExistingArray(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	src := "{\n  // keep me\n  \"shell\": {\n    \"allow\": [\"go\", \"git\",], // trailing\n  },\n}\n"
	os.WriteFile(p, []byte(src), 0o600)
	if err := AppendString(p, []string{"shell", "allow"}, "python", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	want := "{\n  // keep me\n  \"shell\": {\n    \"allow\": [\"go\", \"git\", \"python\"], // trailing\n  },\n}\n"
	if string(got) != want {
		t.Fatalf("got\n%s", got)
	}
	AppendString(p, []string{"shell", "allow"}, "python", nil)
	if got2, _ := os.ReadFile(p); string(got2) != want {
		t.Fatal("idempotent")
	}
	if _, err := os.Stat(p + ".bak"); err != nil {
		t.Fatal("backup")
	}
	if _, err := Parse(got); err != nil {
		t.Fatal("result must stay valid:", err)
	}
}

func TestAppendStringMissingKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	os.WriteFile(p, []byte("{\n  \"model\": \"anthropic/claude-x\" // m\n}\n"), 0o600)
	if err := AppendString(p, []string{"shell", "allow"}, "python", []string{"go"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	c, err := Parse(got)
	if err != nil || strings.Join(c.Shell.Allow, ",") != "go,python" || c.Model != "anthropic/claude-x" || !strings.Contains(string(got), "// m") {
		t.Fatalf("%s %v", got, err)
	}
}

func TestAppendStringEmptyArrayAndNoFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "c.jsonc")
	if err := AppendString(p, []string{"mcp", "servers", "gh", "approve"}, "create_issue", nil); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), `"approve": ["create_issue"]`) {
		t.Fatal(string(b))
	}
	p2 := filepath.Join(dir, "e.jsonc")
	os.WriteFile(p2, []byte(`{"shell":{"allow":[ ]}}`), 0o600)
	AppendString(p2, []string{"shell", "allow"}, "x", nil)
	if b, _ := os.ReadFile(p2); string(b) != `{"shell":{"allow":[ "x"]}}` {
		t.Fatal(string(b))
	}
}

func TestAppendStringCommentWithComma(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		// no trailing comma; the comma lives in the comment and must survive
		"{\"shell\":{\"allow\":[\"go\" // keep, ok\n]}}": "{\"shell\":{\"allow\":[\"go\", \"x\" // keep, ok\n]}}",
		// real trailing comma followed by a comment that also has one
		"{\"shell\":{\"allow\":[\"go\", // a, b\n]}}": "{\"shell\":{\"allow\":[\"go\", \"x\" // a, b\n]}}",
		// block comment with a comma, no trailing comma
		"{\"shell\":{\"allow\":[\"go\" /* x, y */]}}": "{\"shell\":{\"allow\":[\"go\", \"x\" /* x, y */]}}",
	}
	for in, want := range cases {
		p := filepath.Join(dir, "c.jsonc")
		os.WriteFile(p, []byte(in), 0o600)
		if err := AppendString(p, []string{"shell", "allow"}, "x", nil); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(p); string(got) != want {
			t.Errorf("in  %q\ngot %q\nwant %q", in, got, want)
		}
	}
}

// A truncated config (crash mid-write, manual edit) must come back as an
// error, never a panic: the scanner is total.
func TestAppendStringTruncatedConfig(t *testing.T) {
	cases := []string{
		`{`,
		`{"shell"`,
		`{"shell":`,
		`{"shell": {`,
		`{"shell": {"allow"`,
		`{"shell": {"allow":`,
		`{"shell": {"allow": [`,
		`{"shell": {"allow": ["go"`,
		`{"shell": {"allow": ["go",`,
		`{"shell": {"allow": ["go"]`,
	}
	for _, in := range cases {
		p := filepath.Join(t.TempDir(), "c.jsonc")
		os.WriteFile(p, []byte(in), 0o600)
		if err := AppendString(p, []string{"shell", "allow"}, "x", nil); err == nil {
			t.Errorf("input %q: want an error, got nil", in)
		}
		if got, _ := os.ReadFile(p); string(got) != in {
			t.Errorf("input %q: file must be untouched", in)
		}
	}
}

// A config managed by a dotfile tool is a symlink: the edit must go through
// it, not replace it, and an existing file's mode is kept.
func TestAppendStringThroughSymlinkKeepsLinkAndMode(t *testing.T) {
	d := t.TempDir()
	real := filepath.Join(d, "real.jsonc")
	link := filepath.Join(d, "link.jsonc")
	os.WriteFile(real, []byte(`{"a":1}`), 0o644)
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := AppendString(link, []string{"shell", "allow"}, "python3", nil); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symlink was replaced by a regular file")
	}
	b, _ := os.ReadFile(real)
	if !strings.Contains(string(b), `"python3"`) {
		t.Fatalf("target not updated: %s", b)
	}
	if fi, _ := os.Stat(real); fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode changed to %v", fi.Mode().Perm())
	}
}

func TestSetObjectEntry(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	os.WriteFile(p, []byte("{\n  // c\n  \"mcp\": { \"servers\": { \"a\": {\"url\":\"https://x\"} } }\n}\n"), 0o600)
	added, err := SetObjectEntry(p, []string{"mcp", "servers"}, "b", `{"command": "y"}`)
	if err != nil || !added {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	c, err := Parse(b)
	if err != nil || c.MCP.Servers["b"].Command != "y" || c.MCP.Servers["a"].URL != "https://x" || !strings.Contains(string(b), "// c") {
		t.Fatalf("%s %v", b, err)
	}
	if added, _ := SetObjectEntry(p, []string{"mcp", "servers"}, "a", `{}`); added {
		t.Fatal("existing key untouched")
	}
	// Missing parents and a missing file both create the chain.
	p2 := filepath.Join(t.TempDir(), "new.jsonc")
	added, err = SetObjectEntry(p2, []string{"mcp", "servers"}, "gh", `{"command": "npx"}`)
	if err != nil || !added {
		t.Fatal(err)
	}
	b2, _ := os.ReadFile(p2)
	if c, err := Parse(b2); err != nil || c.MCP.Servers["gh"].Command != "npx" {
		t.Fatalf("%s %v", b2, err)
	}
	// A malformed value never reaches the file.
	if _, err := SetObjectEntry(p, []string{"mcp", "servers"}, "bad", `not json`); err == nil {
		t.Fatal("invalid JSON value must be refused")
	}
	if b3, _ := os.ReadFile(p); string(b3) != string(b) {
		t.Fatal("refused write must leave the file untouched")
	}
}

func TestSetString(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	src := `{
  // keep this comment
  "providers": {
    "openai": { "auth": "api_key", "apiKey": "env:K" }, // https://x/v1
  },
}
`
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err := SetString(p, []string{"providers", "openai"}, "auth", "oauth")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), `"auth": "oauth"`) ||
		!strings.Contains(string(b), "keep this comment") ||
		!strings.Contains(string(b), "https://x/v1") ||
		!strings.Contains(string(b), `"apiKey": "env:K"`) {
		t.Fatalf("edit lost content: %s", b)
	}
	c, err := Parse(b)
	if err != nil || c.Providers["openai"].Auth != "oauth" || c.Providers["openai"].APIKey != "env:K" {
		t.Fatalf("%v %+v", err, c.Providers["openai"])
	}
	// A missing key falls back to insertion.
	if ok, err := SetString(p, []string{"providers", "openai"}, "baseUrl", "http://y"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	// A missing path creates the chain, and a missing file is created.
	p2 := filepath.Join(t.TempDir(), "new.jsonc")
	if ok, err := SetString(p2, []string{"providers", "openai"}, "auth", "oauth"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	b2, _ := os.ReadFile(p2)
	if c, err := Parse(b2); err != nil || c.Providers["openai"].Auth != "oauth" {
		t.Fatalf("%s %v", b2, err)
	}
	// Replacing again is idempotent in place.
	if ok, err := SetString(p, []string{"providers", "openai"}, "auth", "oauth"); err != nil || !ok {
		t.Fatal(ok, err)
	}
}

// Duplicate keys resolve to the last one — the value encoding/json keeps and
// Parse reads — so the edit must land where it takes effect.
func TestSetStringDuplicateKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	if err := os.WriteFile(p, []byte(`{
  "providers": {
    "openai": { "auth": "api_key", "auth": "api_key" }
  }
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err := SetString(p, []string{"providers", "openai"}, "auth", "oauth")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	b, _ := os.ReadFile(p)
	if strings.Count(string(b), `"auth"`) != 2 {
		t.Fatalf("both duplicate keys must survive: %s", b)
	}
	c, err := Parse(b)
	if err != nil || c.Providers["openai"].Auth != "oauth" {
		t.Fatalf("the edit must land on the key Parse reads: %v %+v (%s)", err, c.Providers["openai"], b)
	}
	// Duplicate provider objects: the last object is the effective one.
	p2 := filepath.Join(t.TempDir(), "d.jsonc")
	if err := os.WriteFile(p2, []byte(`{
  "providers": {
    "openai": { "auth": "first" },
    "openai": { "auth": "last" }
  }
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, err := SetString(p2, []string{"providers", "openai"}, "auth", "oauth"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	b2, _ := os.ReadFile(p2)
	if c, err := Parse(b2); err != nil || c.Providers["openai"].Auth != "oauth" {
		t.Fatalf("%v (%s)", err, b2)
	}
}

// The function's contract is a string value: a non-string prior value is a
// broken config and must be refused, not laundered into a parseable one.
func TestSetStringRefusesNonString(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	src := `{"providers": {"openai": { "auth": 42 }}}`
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SetString(p, []string{"providers", "openai"}, "auth", "oauth"); err == nil || !strings.Contains(err.Error(), "not a string") {
		t.Fatalf("non-string prior value must be refused: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != src {
		t.Fatalf("refused write must leave the file untouched: %s", b)
	}
}

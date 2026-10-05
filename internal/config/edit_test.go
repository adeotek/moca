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

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotEnvLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantName  string
		wantValue string
		wantOK    bool
		wantErr   string
	}{
		{name: "blank", line: "  \t "},
		{name: "comment", line: "# API_KEY=nope"},
		{name: "simple", line: "API_KEY=abc123", wantName: "API_KEY", wantValue: "abc123", wantOK: true},
		{name: "spaces around", line: "  API_KEY = abc123  ", wantName: "API_KEY", wantValue: "abc123", wantOK: true},
		{name: "export", line: "export API_KEY=abc", wantName: "API_KEY", wantValue: "abc", wantOK: true},
		{name: "export prefix in name", line: "exported=1", wantName: "exported", wantValue: "1", wantOK: true},
		{name: "empty value", line: "API_KEY=", wantName: "API_KEY", wantValue: "", wantOK: true},
		{name: "inline comment", line: "API_KEY=abc # local", wantName: "API_KEY", wantValue: "abc", wantOK: true},
		{name: "hash inside value", line: "PASSWORD=a#b", wantName: "PASSWORD", wantValue: "a#b", wantOK: true},
		{name: "double quoted", line: `API_KEY="a b\tc"`, wantName: "API_KEY", wantValue: "a b\tc", wantOK: true},
		{name: "double quoted escapes", line: `JSON="{\"a\":1}\n"`, wantName: "JSON", wantValue: "{\"a\":1}\n", wantOK: true},
		{name: "single quoted literal", line: `PASSWORD='a $b \n'`, wantName: "PASSWORD", wantValue: `a $b \n`, wantOK: true},
		{name: "quoted then comment", line: `API_KEY="abc" # local`, wantName: "API_KEY", wantValue: "abc", wantOK: true},
		{name: "no equals", line: "API_KEY", wantErr: "expected KEY=VALUE"},
		{name: "invalid name", line: "1KEY=x", wantErr: "invalid variable name"},
		{name: "dash in name", line: "API-KEY=x", wantErr: "invalid variable name"},
		{name: "unterminated double quote", line: `API_KEY="abc`, wantErr: "unterminated double-quoted"},
		{name: "unterminated single quote", line: `API_KEY='abc`, wantErr: "unterminated single-quoted"},
		{name: "text after quote", line: `API_KEY="abc" trailing`, wantErr: "unexpected text after quoted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, value, ok, err := parseDotEnvLine(tc.line)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseDotEnvLine(%q) error = %v, want %q", tc.line, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDotEnvLine(%q): %v", tc.line, err)
			}
			if ok != tc.wantOK || name != tc.wantName || value != tc.wantValue {
				t.Fatalf("parseDotEnvLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.line, name, value, ok, tc.wantName, tc.wantValue, tc.wantOK)
			}
		})
	}
}

// TestLoadDotEnv covers the contract main relies on: a missing file is fine,
// the shell wins over the file, and a malformed line names the file and line.
func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if n, err := LoadDotEnv(path); err != nil || n != 0 {
		t.Fatalf("missing file: LoadDotEnv = (%d, %v), want (0, nil)", n, err)
	}

	const (
		fromFile = "MOCA_TEST_DOTENV_FROM_FILE"
		existing = "MOCA_TEST_DOTENV_EXISTING"
	)
	for _, k := range []string{fromFile, existing} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	t.Setenv(existing, "shell")

	body := "\ufeff" + // a UTF-8 BOM must not swallow the first assignment
		"# comment\n\n" +
		"export " + fromFile + "=from-file\n" +
		existing + "=from-file\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if n != 1 {
		t.Fatalf("LoadDotEnv set %d variables, want 1 (the shell's value already counts)", n)
	}
	if got := os.Getenv(fromFile); got != "from-file" {
		t.Fatalf("%s = %q, want %q", fromFile, got, "from-file")
	}
	if got := os.Getenv(existing); got != "shell" {
		t.Fatalf("%s = %q, want the shell value %q", existing, got, "shell")
	}

	if err := os.WriteFile(path, []byte("GOOD=1\nbroken line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDotEnv(path); err == nil || !strings.Contains(err.Error(), ":2:") {
		t.Fatalf("malformed line: err = %v, want a path:2 error", err)
	}
}

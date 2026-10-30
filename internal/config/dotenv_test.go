package config

import (
	"os"
	"path/filepath"
	"runtime"
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
		{name: "export alone defines nothing", line: "export"},
		{name: "export before a comment defines nothing", line: "export # just a comment"},
		{name: "empty value", line: "API_KEY=", wantName: "API_KEY", wantValue: "", wantOK: true},
		{name: "inline comment", line: "API_KEY=abc # local", wantName: "API_KEY", wantValue: "abc", wantOK: true},
		{name: "hash inside value", line: "PASSWORD=a#b", wantName: "PASSWORD", wantValue: "a#b", wantOK: true},
		{name: "value starting with a hash keeps it", line: "PASSWORD=#abc", wantName: "PASSWORD", wantValue: "#abc", wantOK: true},
		{name: "space then hash is a comment", line: "API_KEY= # nobody", wantName: "API_KEY", wantValue: "", wantOK: true},
		{name: "crlf line ending", line: "API_KEY=abc\r", wantName: "API_KEY", wantValue: "abc", wantOK: true},
		{name: "double quoted", line: `API_KEY="a b\tc"`, wantName: "API_KEY", wantValue: "a b\tc", wantOK: true},
		{name: "double quoted escapes", line: `JSON="{\"a\":1}\n"`, wantName: "JSON", wantValue: "{\"a\":1}\n", wantOK: true},
		{name: "single quoted literal", line: `PASSWORD='a $b \n'`, wantName: "PASSWORD", wantValue: `a $b \n`, wantOK: true},
		{name: "quoted then comment", line: `API_KEY="abc" # local`, wantName: "API_KEY", wantValue: "abc", wantOK: true},
		{name: "no equals", line: "API_KEY", wantErr: "expected KEY=VALUE"},
		{name: "invalid name", line: "1KEY=x", wantErr: "invalid variable name"},
		{name: "dash in name", line: "API-KEY=x", wantErr: "invalid variable name"},
		{name: "empty name", line: "=x", wantErr: "invalid variable name"},
		{name: "unterminated double quote", line: `API_KEY="abc`, wantErr: "unterminated double-quoted"},
		{name: "unterminated single quote", line: `API_KEY='abc`, wantErr: "unterminated single-quoted"},
		{name: "text after quote", line: `API_KEY="abc" trailing`, wantErr: "unexpected text after a quoted"},
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
// the shell wins over the file (an empty value included), the file's own last
// definition wins, a bad line names its path:line without costing the lines
// after it, and nothing here is fatal.
func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if rep, err := LoadDotEnv(path); err != nil || rep.Set != 0 || len(rep.Notes) != 0 {
		t.Fatalf("missing file: LoadDotEnv = (%+v, %v), want nothing set and no notes", rep, err)
	}

	const (
		fromFile = "MOCA_TEST_DOTENV_FROM_FILE"
		existing = "MOCA_TEST_DOTENV_EXISTING"
		empty    = "MOCA_TEST_DOTENV_EMPTY"
		dup      = "MOCA_TEST_DOTENV_DUP"
		after    = "MOCA_TEST_DOTENV_AFTER_BAD_LINE"
	)
	for _, k := range []string{fromFile, existing, empty, dup, after} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	t.Setenv(existing, "shell")
	t.Setenv(empty, "") // set but empty is still the shell's value

	body := "\ufeff" + // a UTF-8 BOM must not swallow the first assignment
		"# comment\n\n" +
		"export " + fromFile + "=from-file\n" +
		existing + "=from-file\n" +
		empty + "=from-file\n" +
		dup + "=first\n" +
		"broken line\n" + // everything after it must still load
		after + "=still-read\n" +
		dup + "=last\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	// fromFile, dup and after; `existing` and the empty shell value are left
	// alone.
	if rep.Set != 3 {
		t.Fatalf("LoadDotEnv set %d variables, want 3 (the shell's values already count)", rep.Set)
	}
	if got := os.Getenv(fromFile); got != "from-file" {
		t.Fatalf("%s = %q, want %q", fromFile, got, "from-file")
	}
	if got := os.Getenv(existing); got != "shell" {
		t.Fatalf("%s = %q, want the shell value %q", existing, got, "shell")
	}
	if got, set := os.LookupEnv(empty); !set || got != "" {
		t.Fatalf("%s = (%q, set=%v), want the shell's empty value kept", empty, got, set)
	}
	if got := os.Getenv(dup); got != "last" {
		t.Fatalf("%s = %q, want the file's last definition %q", dup, got, "last")
	}
	if got := os.Getenv(after); got != "still-read" {
		t.Fatalf("%s = %q: a bad line must not cost the lines after it", after, got)
	}
	if len(rep.Notes) != 1 || !strings.Contains(rep.Notes[0].Error(), ":7: expected KEY=VALUE") {
		t.Fatalf("notes = %v, want one %q note", rep.Notes, ":7: expected KEY=VALUE")
	}
	if !strings.Contains(rep.Notes[0].Error(), path) {
		t.Fatalf("note %q does not name the file", rep.Notes[0])
	}

	// The text after a closing quote is not echoed back: it may be the key
	// pasted outside its quotes, and this note reaches a terminal and a log.
	const secret = "ghp_averyrealsecret"
	if err := os.WriteFile(path, []byte("DOTENV_LEAK=\"v\" "+secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err = LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if len(rep.Notes) != 1 || strings.Contains(rep.Notes[0].Error(), secret) {
		t.Fatalf("notes = %v, want the pasted text kept out", rep.Notes)
	}

	// Nor is the text before the `=`: on a line with no key it is the value
	// itself — a connection string or a padded base64 secret pasted alone.
	for _, line := range []string{"postgres://app:hunter2@db/prod?sslmode=require", "abc+hunter2/xyz=="} {
		if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		rep, err = LoadDotEnv(path)
		if err != nil {
			t.Fatalf("LoadDotEnv: %v", err)
		}
		if len(rep.Notes) != 1 || !strings.Contains(rep.Notes[0].Error(), "invalid variable name") ||
			strings.Contains(rep.Notes[0].Error(), "hunter2") {
			t.Fatalf("%q: notes = %v, want an invalid-name note without the pasted text", line, rep.Notes)
		}
	}
}

// TestLoadDotEnvNotes covers the two non-fatal complaints: a file other users
// can read, and a name the file may not decide.
func TestLoadDotEnvNotes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows synthesises mode bits; there is nothing to report")
	}
	const modeVar = "MOCA_TEST_DOTENV_MODE"
	os.Unsetenv(modeVar)
	t.Cleanup(func() { os.Unsetenv(modeVar) })

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(modeVar+"=v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if rep.Set != 1 {
		t.Fatalf("set = %d, want 1", rep.Set)
	}
	var loose bool
	for _, n := range rep.Notes {
		loose = loose || strings.Contains(n.Error(), "chmod 600")
	}
	if !loose {
		t.Fatalf("notes = %v, want the mode complaint", rep.Notes)
	}
}

// TestLoadDotEnvReservedNames pins the path guard: the file may not decide
// where moca keeps its files. XDG_CONFIG_HOME is the case that happens in
// practice — plenty of users never export it — and the check only fires for a
// name the shell has not already set (the shell wins above it).
func TestLoadDotEnvReservedNames(t *testing.T) {
	prev, hadPrev := os.LookupEnv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv("XDG_CONFIG_HOME", prev)
			return
		}
		os.Unsetenv("XDG_CONFIG_HOME")
	})

	path := filepath.Join(t.TempDir(), ".env")
	body := "XDG_CONFIG_HOME=" + t.TempDir() + "\nMOCA_TEST_DOTENV_SIBLING=v\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("MOCA_TEST_DOTENV_SIBLING")
	t.Cleanup(func() { os.Unsetenv("MOCA_TEST_DOTENV_SIBLING") })

	rep, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if rep.Set != 1 {
		t.Fatalf("set = %d, want 1 (XDG_CONFIG_HOME is never taken from the file)", rep.Set)
	}
	if _, set := os.LookupEnv("XDG_CONFIG_HOME"); set {
		t.Fatal("a .env must not move the config dir it was read from")
	}
	var reserved bool
	for _, n := range rep.Notes {
		reserved = reserved || strings.Contains(n.Error(), "XDG_CONFIG_HOME is not applied")
	}
	if !reserved {
		t.Fatalf("notes = %v, want the reserved-name complaint", rep.Notes)
	}
	if os.Getenv("MOCA_TEST_DOTENV_SIBLING") != "v" {
		t.Fatal("the rest of the file must still apply")
	}
}

// TestReadDotEnvParsesWithoutApplying pins the read-only surface `moca config
// check` uses: the values are reported and the environment is not touched.
func TestReadDotEnvParsesWithoutApplying(t *testing.T) {
	const key = "MOCA_TEST_DOTENV_READ_ONLY"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("A=1\n"+key+"=applied?\nbad line\nB=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := ReadDotEnv(path)
	if err != nil {
		t.Fatalf("ReadDotEnv: %v", err)
	}
	if !d.Found || len(d.Vars) != 3 {
		t.Fatalf("Found=%v Vars=%+v, want 3 assignments", d.Found, d.Vars)
	}
	if d.Vars[0].Name != "A" || d.Vars[1].Name != key || d.Vars[2].Name != "B" || d.Vars[2].Line != 4 {
		t.Fatalf("Vars = %+v, want A, %s, B (line 4)", d.Vars, key)
	}
	if _, set := os.LookupEnv(key); set {
		t.Fatal("ReadDotEnv must not set anything")
	}
	if len(d.LineErr) != 1 || !strings.Contains(d.LineErr[0].Error(), ":3:") {
		t.Fatalf("LineErr = %v, want one for line 3", d.LineErr)
	}
	if d, err := ReadDotEnv(filepath.Join(t.TempDir(), "nope")); err != nil || d.Found {
		t.Fatalf("missing file: (%+v, %v), want Found=false and no error", d, err)
	}
}

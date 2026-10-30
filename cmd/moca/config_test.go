package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/permissions"
)

func runConfigCmd(t *testing.T, args ...string) (code int, out, errb string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(context.Background(), args, nil, &o, &e)
	return code, o.String(), e.String()
}

// `config sample` prints the embedded template byte for byte.
func TestConfigSamplePrintsTheTemplate(t *testing.T) {
	isolate(t)
	code, out, errb := runConfigCmd(t, "config", "sample")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb)
	}
	if out != config.Sample() {
		t.Fatalf("stdout is not the embedded sample (%d vs %d bytes)", len(out), len(config.Sample()))
	}
	if !strings.Contains(out, `"model"`) || !strings.Contains(out, "defaultMode") {
		t.Fatalf("the sample is missing its core options:\n%s", out[:min(400, len(out))])
	}
}

// A clean user config: check reports both layers and exits 0. `config check`
// runs before the config is loaded, so this also proves the dispatch order.
func TestConfigCheckClean(t *testing.T) {
	isolate(t)
	cfg := writeCfg(t, `{"model":"opencode-go/glm-5.3-flash"}`)
	code, out, errb := runConfigCmd(t, "--config", cfg, "config", "check")
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, errb, out)
	}
	if !strings.Contains(out, "ok") || !strings.Contains(out, "no issues") {
		t.Fatalf("clean check must say so:\n%s", out)
	}
}

// An unknown key is a strict-decode error, reported with the file and position.
func TestConfigCheckReportsUnknownKey(t *testing.T) {
	isolate(t)
	cfg := writeCfg(t, "{\n  \"model\": \"opencode-go/glm-5.3-flash\",\n  \"modle\": \"typo\"\n}\n")
	code, out, errb := runConfigCmd(t, "--config", cfg, "config", "check")
	if code != exitUsage {
		t.Fatalf("a broken config must exit %d, got %d (%s%s)", exitUsage, code, out, errb)
	}
	if !strings.Contains(out, "modle") || !strings.Contains(out, "3") {
		t.Fatalf("the issue must name the key and its line:\n%s", out)
	}
	if !strings.Contains(errb, "1 issue") {
		t.Fatalf("stderr must count the issues: %q", errb)
	}
}

// A missing user config is not an issue: a run falls back to defaults.
func TestConfigCheckMissingFileIsFine(t *testing.T) {
	isolate(t)
	missing := filepath.Join(t.TempDir(), "nope.jsonc")
	code, out, _ := runConfigCmd(t, "--config", missing, "config", "check")
	if code != 0 || !strings.Contains(out, "not found") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

// The project file is validated when the workdir is trusted, and reported as
// NOT READ (not skipped silently) when it is not — the §4 rule that an
// untrusted .moca/config.jsonc is never read applies to `check` too.
func TestConfigCheckProjectTrust(t *testing.T) {
	isolate(t)
	wd := t.TempDir()
	mustMkdir(t, filepath.Join(wd, ".moca"))
	proj := filepath.Join(wd, ".moca", "config.jsonc")
	os.WriteFile(proj, []byte(`{"permissions":{"defaultMode":"auto"}}`), 0o600)

	t.Chdir(wd)
	cfg := writeCfg(t, `{"model":"opencode-go/glm-5.3-flash"}`)

	code, out, _ := runConfigCmd(t, "--config", cfg, "config", "check")
	if code != 0 {
		t.Fatalf("untrusted project file must not be an error, exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "not read") || !strings.Contains(out, "not a trusted project") {
		t.Fatalf("an untrusted project file must be reported as not read:\n%s", out)
	}

	// Trust the workdir the way a run would, then the file is validated.
	ts, err := permissions.LoadTrust(filepath.Join(config.DataDir(), "trust.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.Set(wd, true); err != nil {
		t.Fatal(err)
	}
	code, out, _ = runConfigCmd(t, "--config", cfg, "config", "check")
	if code != 0 || !strings.Contains(out, "ok") {
		t.Fatalf("trusted project file must be validated and pass, exit %d:\n%s", code, out)
	}

	// A trusted project file with a project-only violation (yolo) is an issue.
	os.WriteFile(proj, []byte(`{"permissions":{"defaultMode":"yolo"}}`), 0o600)
	code, out, _ = runConfigCmd(t, "--config", cfg, "config", "check")
	if code != exitUsage || !strings.Contains(out, "yolo") {
		t.Fatalf("a project yolo must fail the check, exit %d:\n%s", code, out)
	}
}

// `config check` is the file's diagnostic surface: a bad .env line is an
// issue with its path and line, and a `.env` other users can read is reported
// as advice without failing the check.
func TestConfigCheckReportsTheDotEnv(t *testing.T) {
	isolate(t)
	cfg := writeCfg(t, `{"model":"opencode-go/glm-5.3-flash"}`)

	code, out, errb := runConfigCmd(t, "--config", cfg, "config", "check")
	if code != 0 || !strings.Contains(out, "environment file:") || !strings.Contains(out, "not found — optional") {
		t.Fatalf("a missing .env is fine, exit %d:\n%s%s", code, out, errb)
	}

	writeDotEnv(t, "# keys\nGOOD_KEY=1\n")
	code, out, _ = runConfigCmd(t, "--config", cfg, "config", "check")
	if code != 0 || !strings.Contains(out, "1 variable(s)") || !strings.Contains(out, "no issues") {
		t.Fatalf("a clean .env must pass, exit %d:\n%s", code, out)
	}

	path := writeDotEnv(t, "GOOD_KEY=1\nnot an assignment\n")
	code, out, errb = runConfigCmd(t, "--config", cfg, "config", "check")
	if code != exitUsage {
		t.Fatalf("a broken .env must fail the check, exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, path+":2: expected KEY=VALUE") {
		t.Fatalf("the issue must name the file and line:\n%s", out)
	}
	if !strings.Contains(errb, "1 issue") {
		t.Fatalf("stderr must count the issue: %q", errb)
	}
	if strings.Contains(errb, "moca: warning:") {
		// `config check` lists the file's problems itself; the startup warning
		// would print each one twice.
		t.Fatalf("the startup warning must not be repeated by `config check`: %q", errb)
	}

	// A reserved name is an issue only when a run would refuse it: with the
	// name already exported (isolate() sets XDG_CONFIG_HOME) the shell wins
	// first and a run never mentions the line, so the check must not fail on
	// it either — one file, one verdict, two surfaces.
	writeDotEnv(t, "GOOD_KEY=1\nXDG_CONFIG_HOME=/elsewhere\n")
	code, out, _ = runConfigCmd(t, "--config", cfg, "config", "check")
	if code != 0 || strings.Contains(out, "is not applied") {
		t.Fatalf("a shell-shadowed reserved line must not fail the check, exit %d:\n%s", code, out)
	}

	// With the name unset (HOME here — isolate() keeps XDG_CONFIG_HOME, which
	// is what locates the file) the run would refuse the line and warn, so the
	// check fails with it.
	home, hadHome := os.LookupEnv("HOME")
	os.Unsetenv("HOME")
	t.Cleanup(func() {
		if hadHome {
			os.Setenv("HOME", home)
		}
	})
	writeDotEnv(t, "GOOD_KEY=1\nHOME=/elsewhere\n")
	code, out, _ = runConfigCmd(t, "--config", cfg, "config", "check")
	if code != exitUsage || !strings.Contains(out, path+":2: HOME is not applied") {
		t.Fatalf("a reserved name a run refuses must fail the check with its line, exit %d:\n%s", code, out)
	}

	// A mode other users can read is advice: it does not fail the check.
	if runtime.GOOS == "windows" {
		return // Windows synthesises mode bits; there is nothing to report
	}
	if err := os.WriteFile(path, []byte("GOOD_KEY=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil { // WriteFile keeps the existing mode
		t.Fatal(err)
	}
	code, out, _ = runConfigCmd(t, "--config", cfg, "config", "check")
	if code != 0 || !strings.Contains(out, "chmod 600") || !strings.Contains(out, "no issues") {
		t.Fatalf("a loose mode is a note, not an issue, exit %d:\n%s", code, out)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

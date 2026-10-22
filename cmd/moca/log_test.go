package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

const okStream = "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n" +
	"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n" +
	"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n"

// logRunIn runs `-p prompt` against a scripted provider with XDG_STATE_HOME
// set to state; it returns the exit code and stderr.
func logRunIn(t *testing.T, state, prompt string) (int, string) {
	t.Helper()
	return logRunCfgIn(t, state, "", prompt)
}

// logRunCfgIn is logRunIn with extra JSON spliced into the config (a "log"
// block, say).
func logRunCfgIn(t *testing.T, state, extra, prompt string) (int, string) {
	t.Helper()
	isolate(t)
	t.Setenv("XDG_STATE_HOME", state)
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	srv := fakeCompletions(t, 200, okStream)
	t.Cleanup(srv.Close)
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m",`+extra+`"providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	code := run(context.Background(), []string{"--config", cfg, "-p", prompt}, nil, &out, &errb)
	return code, errb.String()
}

// logRun is logRunIn with a fresh state dir, which it also returns.
func logRun(t *testing.T, prompt string) (int, string, string) {
	t.Helper()
	state := t.TempDir()
	code, stderr := logRunIn(t, state, prompt)
	return code, stderr, state
}

// logRunCfg is logRunCfgIn with a fresh state dir, which it also returns,
// and MOCA_LOG cleared: the config key is the only level knob left.
func logRunCfg(t *testing.T, extra, prompt string) (int, string, string) {
	t.Helper()
	t.Setenv("MOCA_LOG", "")
	state := t.TempDir()
	code, stderr := logRunCfgIn(t, state, extra, prompt)
	return code, stderr, state
}

func readLog(t *testing.T, state string) string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(state, "moca", "logs", "*.log"))
	if len(m) != 1 {
		t.Fatalf("want one log file, got %v", m)
	}
	b, _ := os.ReadFile(m[0])
	return string(b)
}

func sessionID8(t *testing.T) string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_DATA_HOME"), "moca", "sessions", "*.jsonl"))
	if len(m) != 1 {
		t.Fatalf("sessions: %v", m)
	}
	base := strings.TrimSuffix(filepath.Base(m[0]), ".jsonl")
	return base[len(base)-8:]
}

func TestLogFileForOneShot(t *testing.T) {
	code, stderr, state := logRun(t, "private prompt words")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	log := readLog(t, state)
	id := "session=" + sessionID8(t)
	for _, want := range []string{"msg=start", "mode=oneshot", "level=info", "msg=session " + id,
		`msg="run end" ` + id, "outcome=done", "msg=exit code=0"} {
		if !strings.Contains(log, want) {
			t.Errorf("missing %q in:\n%s", want, log)
		}
	}
	if strings.Contains(log, "private prompt") || strings.Contains(log, "msg=request") {
		t.Fatalf("prompt leaked or debug line at info:\n%s", log)
	}
}

func TestLogDebugViaEnv(t *testing.T) {
	t.Setenv("MOCA_LOG", "debug")
	code, stderr, state := logRun(t, "hi")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if log := readLog(t, state); !strings.Contains(log, "msg=request model=m") || !strings.Contains(log, "msg=config") {
		t.Fatalf("debug lines missing:\n%s", log)
	}
}

func TestLogOffCreatesNothing(t *testing.T) {
	t.Setenv("MOCA_LOG", "off")
	if code, stderr, state := logRun(t, "hi"); code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	} else if _, err := os.Stat(filepath.Join(state, "moca")); !os.IsNotExist(err) {
		t.Fatal("MOCA_LOG=off must not create the state dir")
	}
}

// TestLogLevelFromConfig: `log.level` decides on its own — MOCA_LOG only
// overrides it for one run.
func TestLogLevelFromConfig(t *testing.T) {
	code, stderr, state := logRunCfg(t, `"log":{"level":"off"},`, "hi")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(state, "moca")); !os.IsNotExist(err) {
		t.Fatal(`log.level "off" must not create the state dir`)
	}
	code, stderr, state = logRunCfg(t, `"log":{"level":"debug"},`, "hi")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if log := readLog(t, state); !strings.Contains(log, "msg=request model=m") {
		t.Fatalf("log.level \"debug\" must add request lines:\n%s", log)
	}
}

func TestLogInvalidEnvIsUsageError(t *testing.T) {
	t.Setenv("MOCA_LOG", "loud")
	code, stderr, _ := logRun(t, "hi")
	if code != exitUsage || !strings.Contains(stderr, "MOCA_LOG") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

func TestLogOpenFailureWarns(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file") // a regular file: <it>/moca/logs cannot be created
	os.WriteFile(f, nil, 0o600)
	code, stderr := logRunIn(t, f, "hi")
	if code != 0 || strings.Count(stderr, "diagnostic log disabled") != 1 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

// TestNoLogOutputOnStderr: at debug, no record reaches the terminal — the
// TUI renders on the same stream.
func TestNoLogOutputOnStderr(t *testing.T) {
	t.Setenv("MOCA_LOG", "debug")
	_, stderr, _ := logRun(t, "hi")
	if strings.Contains(stderr, "level=") || strings.Contains(stderr, "msg=") {
		t.Fatalf("log records on stderr: %q", stderr)
	}
}

// TestLogNeverTouchesRealState: TestMain points XDG_STATE_HOME at a temp dir.
func TestLogNeverTouchesRealState(t *testing.T) {
	h, _ := os.UserHomeDir()
	if s := os.Getenv("XDG_STATE_HOME"); s == "" || strings.HasPrefix(s, filepath.Join(h, ".local")) {
		t.Fatalf("XDG_STATE_HOME=%q", s)
	}
}

// The start line is written before a trusted project file loads, so it must
// not claim a plan state; the resolved mode is logged once it is final.
func TestLogModeAfterResolution(t *testing.T) {
	_, _, state := logRunCfg(t, `"permissions":{"defaultMode":"plan"},`, "hi")
	log := readLog(t, state)
	if !strings.Contains(log, "msg=mode mode=ask plan=true") {
		t.Fatalf("resolved mode line missing:\n%s", log)
	}
	for _, l := range strings.Split(log, "\n") {
		if strings.Contains(l, "msg=start") && strings.Contains(l, "plan=") {
			t.Fatalf("start line reports plan before the project layer loads: %s", l)
		}
	}
}

// TestStartLogNoStderrUnderACP: the editor owns an external agent's stderr, and
// Zed on Windows has been reported to lose — even kill — a child over a write
// to it. A log that cannot be opened must therefore warn on stderr for the TUI
// and -p, and stay silent under ACP (where stdout is JSON-RPC and the contract
// is "diagnostics go to the log file", §9.5).
func TestStartLogNoStderrUnderACP(t *testing.T) {
	for _, tc := range []struct {
		opts     Options
		wantWarn bool
	}{
		{Options{Sub: []string{"acp"}}, false},
		{Options{}, true},
		{Options{OneShot: true}, true},
	} {
		isolate(t)
		// A state path that is a file: applog.Open cannot create <state>/logs.
		state := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(state, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XDG_STATE_HOME", state)
		t.Setenv("MOCA_LOG", "")
		cfg, err := config.Parse([]byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		closeLog, err := startLog(cfg, "/cfg/config.jsonc", tc.opts, &stderr)
		if err != nil {
			t.Fatalf("a log that cannot be opened is a warning, not an error: %v", err)
		}
		closeLog()
		if got := strings.Contains(stderr.String(), "diagnostic log disabled"); got != tc.wantWarn {
			t.Fatalf("stderr warning = %v, want %v (stderr %q)", got, tc.wantWarn, stderr.String())
		}
	}
}

// TestLogModeNamesTheRun covers the label the diagnostic log carries.
func TestLogModeNamesTheRun(t *testing.T) {
	for _, tc := range []struct {
		opts Options
		want string
	}{
		{Options{Sub: []string{"acp"}}, "acp"},
		{Options{OneShot: true}, "oneshot"},
		{Options{Execute: "docs/plans/x.md"}, "oneshot"},
		{Options{}, "tui"},
	} {
		if got := tc.opts.logMode(); got != tc.want {
			t.Fatalf("logMode(%+v) = %q, want %q", tc.opts, got, tc.want)
		}
	}
}

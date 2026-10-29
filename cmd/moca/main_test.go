package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/testenv"
)

// TestMain keeps every run() in this package away from the host's real state.
// run() opens the diagnostic log for TUI and -p runs, so MOCA_LOG would point
// that log outside the tree; testenv.Main adds the rest of the isolation the
// other test binaries already get — a temp XDG root for the config, the auth
// store, the models cache and the session dirs, plus blanked built-in
// provider keys, since an ambient key makes a provider ready and sends
// discovery at its real endpoint from a unit test.
func TestMain(m *testing.M) {
	os.Unsetenv("MOCA_LOG")
	os.Exit(testenv.Main(m))
}

// writeDotEnv drops a .env into the run's config dir — the file startup()
// reads — so a test exercises the same path a user's file takes.
func writeDotEnv(t *testing.T, body string) string {
	t.Helper()
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	path := config.DotEnvFile()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestDotEnvFeedsTheCredential pins the startup wiring end to end: a key in
// ~/.config/moca/.env is in the process environment before the config's `env:`
// reference is resolved, so the request carries it. This is the leg a unit
// test of LoadDotEnv cannot see — deleting startup() from run() breaks it,
// which is exactly how the wiring would go missing.
func TestDotEnvFeedsTheCredential(t *testing.T) {
	isolate(t)
	const keyVar, keyValue = "MOCA_T_DOTENV_KEY", "from-the-dotenv-file"
	os.Unsetenv(keyVar)
	t.Cleanup(func() { os.Unsetenv(keyVar) })
	writeDotEnv(t, "# moca's key lives here\n"+keyVar+"="+keyValue+"\n")

	var mu sync.Mutex
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auth = append(auth, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",`+
		`"auth":"api_key","apiKey":"env:`+keyVar+`","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "hi"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d stderr %q", code, errb.String())
	}
	if out.String() != "hello\n" {
		t.Fatalf("stdout = %q", out.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(auth) == 0 {
		t.Fatal("the provider was never called")
	}
	if want := "Bearer " + keyValue; auth[0] != want {
		t.Fatalf("Authorization = %q, want %q — the .env key never reached the credential", auth[0], want)
	}
	if strings.Contains(errb.String(), "warning") {
		t.Fatalf("a healthy .env must be silent: %q", errb.String())
	}
}

// TestDotEnvBadLineWarnsAndTheRunGoesOn pins both halves of the recovery: the
// broken line is named with its path and line, and the key on a later line
// still loads — one typo costs one line, not the rest of the file.
func TestDotEnvBadLineWarnsAndTheRunGoesOn(t *testing.T) {
	isolate(t)
	const keyVar = "MOCA_T_DOTENV_AFTER_BAD"
	os.Unsetenv(keyVar)
	t.Cleanup(func() { os.Unsetenv(keyVar) })
	path := writeDotEnv(t, "not an assignment\n"+keyVar+"=after-the-typo\n")

	srv := fakeCompletions(t, 200, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	defer srv.Close()
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",`+
		`"auth":"api_key","apiKey":"env:`+keyVar+`","models":{"m":{"contextWindow":32768}}}}}`)

	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "hi"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d stderr %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "moca: warning: "+path+":1: expected KEY=VALUE") {
		t.Fatalf("stderr = %q, want the path:line warning", errb.String())
	}
	if out.String() != "ok\n" {
		t.Fatalf("stdout = %q: the run must go on", out.String())
	}
}

// TestDotEnvStaysSilentUnderACP pins §9.5's contract: the editor owns stderr,
// so a broken .env is a log line there, never a write to that pipe.
func TestDotEnvStaysSilentUnderACP(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // the run's log file
	t.Setenv("MOCA_T_KEY", "k")
	writeDotEnv(t, "broken line\n")
	cfg := writeCfg(t, acpCfg)

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "acp"}, in, &out, &errb); code != exitOK {
		t.Fatalf("exit %d stderr %q", code, errb.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr = %q, want it empty under ACP", errb.String())
	}
	if !strings.Contains(out.String(), `"jsonrpc":"2.0"`) {
		t.Fatalf("stdout lost its JSON-RPC response: %q", out.String())
	}
}

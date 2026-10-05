// cmd/moca/cli_test.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/session"
)

func TestParseArgs(t *testing.T) {
	o, err := parseArgs([]string{"-p", "hi", "--model", "openai/x", "--effort", "high", "--no-approve"}, nil)
	if err != nil || !o.OneShot || o.Prompt != "hi" || o.Model != "openai/x" || o.Effort != "high" || o.Approve == nil || *o.Approve {
		t.Fatalf("%+v %v", o, err)
	}
	o, _ = parseArgs([]string{"-p", "-"}, strings.NewReader("from stdin\n"))
	if o.Prompt != "from stdin" {
		t.Fatalf("stdin prompt %q", o.Prompt)
	}
	if _, err := parseArgs([]string{"--approve", "--no-approve", "-p", "x"}, nil); err == nil {
		t.Fatal("approve flags are exclusive")
	}
	if _, err := parseArgs([]string{"--yolo", "--no-yolo", "-p", "x"}, nil); err == nil {
		t.Fatal("yolo flags are exclusive")
	}
	if _, err := parseArgs([]string{"--resume", "last", "--continue", "-p", "x"}, nil); err == nil {
		t.Fatal("resume/continue are exclusive")
	}
	o, _ = parseArgs([]string{"--no-yolo", "-p", "x"}, nil)
	if o.Yolo == nil || *o.Yolo || o.YoloOn(config.Config{Yolo: true}) {
		t.Fatal("--no-yolo overrides config")
	}
	o, _ = parseArgs([]string{"-p", "x"}, nil)
	if o.YoloOn(config.Config{}) || !o.YoloOn(config.Config{Yolo: true}) {
		t.Fatal("no flag → config decides")
	}
	if _, err := parseArgs([]string{"--effort", "med", "-p", "x"}, nil); err == nil {
		t.Fatal("bad effort is a usage error")
	}
	o, _ = parseArgs([]string{"mcp", "import"}, nil)
	if strings.Join(o.Sub, " ") != "mcp import" {
		t.Fatal(o.Sub)
	}
}

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.jsonc")
	os.WriteFile(p, []byte(body), 0o600)
	return p
}

// isolate keeps -p runs from writing sessions into the real data dir.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func fakeCompletions(t *testing.T, status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}

func TestOneShotStreamsToStdout(t *testing.T) {
	isolate(t)
	srv := fakeCompletions(t, 200, "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	code := run(context.Background(), []string{"--config", cfg, "-p", "hi"}, nil, &out, &errb)
	if code != 0 || out.String() != "hello\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "tokens 3/2") || !strings.Contains(errb.String(), "$") {
		t.Fatalf("exit summary missing: %q", errb.String())
	}
}

func TestOneShotDiscardsStdoutOnReset(t *testing.T) {
	isolate(t)
	// A clean mid-stream EOF triggers reset-and-retry. stdout already holds the
	// first attempt's text, so a pipe would see "The answer isThe answer is 42".
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if calls.Add(1) == 1 {
			io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"The answer is\"}}]}\n\n")
			return
		}
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"The answer is 42\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d stderr %q", code, errb.String())
	}
	if out.String() != "The answer is 42\n" {
		t.Fatalf("stdout %q: first attempt's text must be discarded on reset", out.String())
	}
}

func TestOneShotNonCompletionStopsExitNonZero(t *testing.T) {
	isolate(t)
	for reason, want := range map[string]string{"length": "truncated", "content_filter": "refused"} {
		srv := fakeCompletions(t, 200, "data: {\"choices\":[{\"delta\":{\"content\":\"part\"},\"finish_reason\":\""+reason+"\"}]}\n\ndata: [DONE]\n\n")
		t.Setenv("MOCA_T_KEY", "k")
		cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb)
		srv.Close()
		if code != 1 || !strings.Contains(errb.String(), want) {
			t.Errorf("%s: exit %d stderr %q — want exit 1 mentioning %q", reason, code, errb.String(), want)
		}
		if !strings.Contains(out.String(), "part") {
			t.Errorf("%s: partial text must still be printed, got %q", reason, out.String())
		}
	}
}

func TestExitCodes(t *testing.T) {
	isolate(t)
	t.Setenv("MOCA_T_KEY", "")
	cases := []struct {
		name string
		cfg  string
		args []string
		want int
	}{
		{"bare model", `{}`, []string{"-p", "x", "--model", "glm"}, 2},
		{"unknown provider", `{}`, []string{"-p", "x", "--model", "foo/bar"}, 2},
		{"bad config", `{"model": }`, []string{"-p", "x"}, 2},
		{"missing key", `{"model":"loc/m","providers":{"loc":{"baseUrl":"http://127.0.0.1:1","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`, []string{"-p", "x"}, 2},
		{"no model at all", `{}`, []string{"-p", "x"}, 2},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if got := run(context.Background(), append([]string{"--config", writeCfg(t, c.cfg)}, c.args...), nil, &out, &errb); got != c.want {
			t.Errorf("%s: exit %d want %d (stderr %q)", c.name, got, c.want, errb.String())
		}
	}
}

func TestSubcommandBeforeConfig(t *testing.T) {
	// Subcommand stubs must not trip the "no model configured" check on a
	// fresh install.
	var out, errb bytes.Buffer
	code := run(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "none.jsonc"), "login", "anthropic"}, nil, &out, &errb)
	if code != 2 || !strings.Contains(errb.String(), "login lands in a later phase") {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}

func TestProviderErrorExit1(t *testing.T) {
	isolate(t)
	srv := fakeCompletions(t, 401, `{"error":{"message":"bad key"}}`)
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errb.String(), "bad key") {
		t.Fatal(errb.String())
	}
}

func TestInterruptedExit130(t *testing.T) {
	isolate(t)
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-block
	}))
	defer srv.Close()
	defer close(block)
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { cancel() }()
	var out, errb bytes.Buffer
	if code := run(ctx, []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 130 {
		t.Fatalf("exit %d", code)
	}
}

func TestStartFailureExit1(t *testing.T) {
	isolate(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	os.WriteFile(blocked, []byte("x"), 0o644)
	t.Setenv("XDG_DATA_HOME", blocked) // MkdirAll for the data dir fails
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"http://127.0.0.1:1","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "moca:") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestOneShotToolLoopContract(t *testing.T) {
	turns := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"checking\",\"tool_calls\":[{\"index\":0,\"id\":\"c\",\"function\":{\"name\":\"ls\",\"arguments\":\"{}\"}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"All good.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n",
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, turns[min(n, len(turns)-1)])
		n++
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	code := run(context.Background(), []string{"--config", cfg, "-p", "check"}, nil, &out, &errb)
	if code != 0 || out.String() != "All good.\n" {
		t.Fatalf("code %d stdout %q", code, out.String())
	}
	if !strings.Contains(errb.String(), "▸ ls") || !strings.Contains(errb.String(), "checking") || !strings.Contains(errb.String(), "tokens ") {
		t.Fatalf("stderr %q", errb.String())
	}
}

func TestOneShotMaxStepsExit3(t *testing.T) {
	tool := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c\",\"function\":{\"name\":\"ls\",\"arguments\":\"{}\"}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n"
	final := "data: {\"choices\":[{\"delta\":{\"content\":\"stopped\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if n < 1 {
			io.WriteString(w, tool)
		} else {
			io.WriteString(w, final)
		}
		n++
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := writeCfg(t, `{"model":"loc/m","context":{"maxSteps":1},"providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 3 || out.String() != "stopped\n" {
		t.Fatalf("code %d out %q", code, out.String())
	}
}

func TestNoApproveSkipsProjectInstructions(t *testing.T) {
	var sawRules bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sawRules = strings.Contains(string(b), "PROJECT-RULE-XYZ")
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte("PROJECT-RULE-XYZ"), 0o644)
	t.Chdir(work)
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb)
	if sawRules {
		t.Fatal("default -p must not load project AGENTS.md")
	}
	run(context.Background(), []string{"--config", cfg, "--approve", "-p", "x"}, nil, &out, &errb)
	if !sawRules {
		t.Fatal("--approve loads it")
	}
	sawRules = false
	errb.Reset()
	run(context.Background(), []string{"--config", cfg, "--yolo", "-p", "x"}, nil, &out, &errb)
	if !sawRules || !strings.HasPrefix(errb.String(), "yolo mode: all permission checks are off") {
		t.Fatal("yolo trusts project resources and warns on stderr")
	}
	sawRules = true
	run(context.Background(), []string{"--config", cfg, "--yolo", "--no-approve", "-p", "x"}, nil, &out, &errb)
	if sawRules {
		t.Fatal("explicit --no-approve wins over yolo")
	}
}

func TestResumeAndContinueOneShot(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		answer := "first"
		if n > 0 && strings.Contains(string(b), "remember 42") {
			answer = "you said 42"
		}
		n++
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", answer)
	}))
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(t.TempDir())
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	run(context.Background(), []string{"--config", cfg, "-p", "remember 42"}, nil, &out, &errb)
	out.Reset()
	if code := run(context.Background(), []string{"--config", cfg, "--continue", "-p", "what did I say?"}, nil, &out, &errb); code != 0 || out.String() != "you said 42\n" {
		t.Fatalf("code %d out %q err %q", code, out.String(), errb.String())
	}
	if code := run(context.Background(), []string{"--config", cfg, "--resume", "zzzzzzzz", "-p", "x"}, nil, &out, &errb); code != 2 {
		t.Fatal("bad id → exit 2")
	}
	if code := run(context.Background(), []string{"--config", cfg, "--resume", "last", "--continue", "-p", "x"}, nil, &out, &errb); code != 2 {
		t.Fatal("exclusive flags")
	}
}

func TestResumeUnsetKeyFailsAtFirstRequest(t *testing.T) {
	isolate(t)
	t.Chdir(t.TempDir())
	// A session created while the key existed...
	t.Setenv("MOCA_T_KEY", "k")
	srv := fakeCompletions(t, 200, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "hello"}, nil, &out, &errb); code != 0 {
		t.Fatalf("seed run: %d %q", code, errb.String())
	}
	// ...and resumed after the key went away: resume itself succeeds, the
	// first request fails with the env-var error (exit 2).
	t.Setenv("MOCA_T_KEY", "")
	out.Reset()
	errb.Reset()
	if code := run(context.Background(), []string{"--config", cfg, "--resume", "last", "-p", "again"}, nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), "MOCA_T_KEY") {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}

// Review Focus 3's "`--model` can override": resume a session whose stored
// model's key is gone, overriding to a provider that has one.
func TestResumeModelOverride(t *testing.T) {
	isolate(t)
	t.Chdir(t.TempDir())
	srv := fakeCompletions(t, 200, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("MOCA_T_KEY2", "k2")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{`+
		`"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}},`+
		`"loc2":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY2","models":{"m2":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "hello"}, nil, &out, &errb); code != 0 {
		t.Fatalf("seed run: %d %q", code, errb.String())
	}
	t.Setenv("MOCA_T_KEY", "") // the stored model's key goes away
	out.Reset()
	errb.Reset()
	if code := run(context.Background(), []string{"--config", cfg, "--resume", "last", "--model", "loc2/m2", "-p", "again"}, nil, &out, &errb); code != 0 || out.String() != "ok\n" {
		t.Fatalf("override resume: code %d out %q err %q", code, out.String(), errb.String())
	}
	// The override is recorded as the newest model_change.
	dir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "moca", "sessions")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var sess string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			sess = filepath.Join(dir, e.Name())
		}
	}
	entries, err := session.ReadFile(sess)
	if err != nil {
		t.Fatal(err)
	}
	var last string
	for _, en := range entries {
		if en.Type == session.TypeModelChange {
			last = en.ModelChange.Model
		}
	}
	if last != "loc2/m2" {
		t.Fatalf("model_change %q, want loc2/m2", last)
	}
}

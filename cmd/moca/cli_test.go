// cmd/moca/cli_test.go
package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
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

func fakeCompletions(t *testing.T, status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}

func TestOneShotStreamsToStdout(t *testing.T) {
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

func TestExitCodes(t *testing.T) {
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

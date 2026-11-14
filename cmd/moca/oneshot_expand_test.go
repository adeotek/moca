package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// `moca -p` never runs the TUI's ParseInput, so a leading /name used to reach
// the model as literal text. The bug this pins: the skill exists, is listed in
// the prompt, and `-p "/review"` asks the model to follow a skill it was never
// pointed at — a silent no-op that looks like a model failure.
//
// It reads the provider's request body, not the run's output: expansion that
// happens after the request would still print the right answer.
func TestPExpandsASkillCommand(t *testing.T) {
	isolate(t)
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",`+
		`"auth":"api_key","apiKey":"env:MOCA_T_EXPAND_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	t.Setenv("MOCA_T_EXPAND_KEY", "k")

	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "/review internal/store"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("the provider was never called")
	}
	// The review skill is a built-in, extracted from this binary by Start.
	if !strings.Contains(bodies[0], "Load and follow the review skill") {
		t.Fatalf("the request did not carry the expanded skill:\n%s", bodies[0])
	}
	if !strings.Contains(bodies[0], "internal/store") {
		t.Fatalf("the arguments were dropped from the expansion:\n%s", bodies[0])
	}
	if !strings.Contains(errb.String(), "note: /review → the review skill") {
		t.Fatalf("stderr must say what expanded:\n%s", errb.String())
	}
}

// An unknown /word is prose, not a typo to reject: a message can start with a
// path, and it must reach the model exactly as typed.
func TestPLeavesAnUnknownSlashWordAlone(t *testing.T) {
	isolate(t)
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",`+
		`"auth":"api_key","apiKey":"env:MOCA_T_EXPAND_KEY2","models":{"m":{"contextWindow":32768}}}}}`)
	t.Setenv("MOCA_T_EXPAND_KEY2", "k")

	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "/etc/hosts is wrong"}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 || !strings.Contains(bodies[0], "/etc/hosts is wrong") {
		t.Fatalf("an unknown /word must reach the model as typed, got:\n%s", bodies[0])
	}
	if strings.Contains(errb.String(), "note: /") {
		t.Fatalf("nothing was expanded, so nothing may be announced: %s", errb.String())
	}
}

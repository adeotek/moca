package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// ollamaServer is a stand-in Ollama: native discovery plus a chat endpoint.
func ollamaServer(t *testing.T, params string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"name":"llama3.1:8b"}]}`)
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"capabilities": []string{"completion", "tools"}, "parameters": params})
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "unexpected credentials", 400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"local hello\"},\"finish_reason\":\"stop\"}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func ollamaStart(t *testing.T, cfgJSON string, model string, evs *[]Event) (*Agent, error) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Parse([]byte(cfgJSON))
	if err != nil {
		t.Fatal(err)
	}
	return Start(StartOptions{Config: cfg, Workdir: t.TempDir(), Model: model, Slug: "t",
		Emit: func(e Event) { *evs = append(*evs, e) }})
}

// A model reference is the whole opt-in: no key, no model list, no protocol —
// the model is discovered, a turn runs, and a guessed window is flagged.
func TestStartWithOllamaModel(t *testing.T) {
	srv := ollamaServer(t, "")
	var evs []Event
	a, err := ollamaStart(t, `{"providers":{"ollama":{"baseUrl":"`+srv.URL+`"}}}`, "ollama/llama3.1:8b", &evs)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	out, err := a.Run(context.Background(), "hi")
	if err != nil || out.Text != "local hello" {
		t.Fatalf("run: %q %v", out.Text, err)
	}
	var warned bool
	for _, e := range evs {
		if w, ok := e.(Warning); ok && strings.Contains(w.Text, "assuming 16384") && strings.Contains(w.Text, "OLLAMA_CONTEXT_LENGTH") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("a guessed context window must be flagged: %v", evs)
	}
	if got := a.Status().Window; got != 16384 {
		t.Fatalf("window %d", got)
	}
	if !a.HasCredential("ollama/llama3.1:8b") {
		t.Fatal("a local model never lacks a credential")
	}
	if err := a.RefreshModels(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOllamaReportedWindowIsQuiet(t *testing.T) {
	srv := ollamaServer(t, "num_ctx 32768")
	var evs []Event
	a, err := ollamaStart(t, `{"providers":{"ollama":{"baseUrl":"`+srv.URL+`"}}}`, "ollama/llama3.1:8b", &evs)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, e := range evs {
		if w, ok := e.(Warning); ok {
			t.Fatalf("no warning when the server reports its window: %s", w.Text)
		}
	}
	if a.Status().Window != 32768 {
		t.Fatalf("window %d", a.Status().Window)
	}
}

// A declared contextWindow fills the gap when the Modelfile states no
// num_ctx: the startup hint must stay quiet — it exists to tell the user to
// do what they already did.
func TestOllamaDeclaredWindowIsQuiet(t *testing.T) {
	srv := ollamaServer(t, "")
	var evs []Event
	cfg := `{"providers":{"ollama":{"baseUrl":"` + srv.URL + `","models":{"llama3.1:8b":{"contextWindow":32768}}}}}`
	a, err := ollamaStart(t, cfg, "ollama/llama3.1:8b", &evs)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, e := range evs {
		if w, ok := e.(Warning); ok {
			t.Fatalf("a declared window must not warn: %s", w.Text)
		}
	}
	if a.Status().Window != 32768 {
		t.Fatalf("declared window not in use: %d", a.Status().Window)
	}
}

func TestOllamaTinyWindowWarns(t *testing.T) {
	srv := ollamaServer(t, "num_ctx 4096")
	var evs []Event
	a, err := ollamaStart(t, `{"providers":{"ollama":{"baseUrl":"`+srv.URL+`"}}}`, "ollama/llama3.1:8b", &evs)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, e := range evs {
		if w, ok := e.(Warning); ok && strings.Contains(w.Text, "4096-token context window") {
			return
		}
	}
	t.Fatalf("a window below the agent minimum must warn: %v", evs)
}

// Start fails clearly — with the reason — when the default model lives on a
// server that is down or does not have it; other providers are unaffected.
func TestOllamaStartErrors(t *testing.T) {
	srv := ollamaServer(t, "")
	var evs []Event
	if _, err := ollamaStart(t, `{"providers":{"ollama":{"baseUrl":"`+srv.URL+`"}}}`, "ollama/missing:1b", &evs); err == nil || !strings.Contains(err.Error(), "ollama pull missing:1b") {
		t.Fatalf("missing model: %v", err)
	}
	down := srv.URL
	srv.Close()
	if _, err := ollamaStart(t, `{"providers":{"ollama":{"baseUrl":"`+down+`"}}}`, "ollama/llama3.1:8b", &evs); err == nil || !strings.Contains(err.Error(), "cannot reach ollama") {
		t.Fatalf("server down: %v", err)
	}
	// Configured but unused, with the server down: Start still works.
	t.Setenv("ANTHROPIC_API_KEY", "k")
	a, err := ollamaStart(t, `{"model":"anthropic/claude-haiku-4-5","providers":{"ollama":{"baseUrl":"`+down+`"}}}`, "", &evs)
	if err != nil {
		t.Fatalf("an unused, unreachable ollama must not block startup: %v", err)
	}
	a.Close()
}

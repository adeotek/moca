package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

func TestNormalizeOllamaURL(t *testing.T) {
	cases := map[string]string{
		"nas":                         "http://nas:11434/v1",
		"nas:8080":                    "http://nas:8080/v1",
		"http://nas":                  "http://nas:11434/v1",
		"http://nas:11434":            "http://nas:11434/v1",
		"http://nas:11434/":           "http://nas:11434/v1",
		"http://nas:11434/v1":         "http://nas:11434/v1",
		"http://nas:11434/v1/":        "http://nas:11434/v1",
		":11434":                      "http://127.0.0.1:11434/v1",
		"0.0.0.0":                     "http://127.0.0.1:11434/v1",
		"0.0.0.0:9999":                "http://127.0.0.1:9999/v1",
		"https://ai.example.com":      "https://ai.example.com/v1",
		"https://ai.example.com/o/v1": "https://ai.example.com/o/v1",
		"http://[::1]:11434":          "http://[::1]:11434/v1",
		"  localhost  ":               "http://localhost:11434/v1",
	}
	for in, want := range cases {
		if got := normalizeOllamaURL(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOllamaBaseURLPrecedence(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "OLLAMA_HOST" {
				return v
			}
			return ""
		}
	}
	if got := ollamaBaseURL(config.ProviderConfig{BaseURL: "http://cfg"}, env("envhost")); got != "http://cfg:11434/v1" {
		t.Fatalf("config wins: %q", got)
	}
	if got := ollamaBaseURL(config.ProviderConfig{}, env("envhost:1")); got != "http://envhost:1/v1" {
		t.Fatalf("env next: %q", got)
	}
	if got := ollamaBaseURL(config.ProviderConfig{}, env("")); got != "http://localhost:11434/v1" {
		t.Fatalf("default: %q", got)
	}
}

// fakeOllama serves the native discovery API and a chat endpoint.
type fakeOllama struct {
	*httptest.Server
	tags     []string
	show     map[string]ollamaShow
	auth     atomic.Value // last Authorization header
	chatHits atomic.Int32
}

func newFakeOllama(t *testing.T) *fakeOllama {
	t.Helper()
	f := &fakeOllama{show: map[string]ollamaShow{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		f.auth.Store(r.Header.Get("Authorization"))
		var tags ollamaTags
		for _, n := range f.tags {
			tags.Models = append(tags.Models, struct {
				Name string `json:"name"`
			}{n})
		}
		json.NewEncoder(w).Encode(tags)
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Model string }
		json.NewDecoder(r.Body).Decode(&req)
		sh, ok := f.show[req.Model]
		if !ok {
			http.Error(w, `{"error":"nope"}`, 500)
			return
		}
		json.NewEncoder(w).Encode(sh)
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		f.chatHits.Add(1)
		f.auth.Store(r.Header.Get("Authorization"))
		var body struct{ Model string }
		json.NewDecoder(r.Body).Decode(&body)
		if body.Model == "ghost:1b" {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"error":{"message":"model \"ghost:1b\" not found, try pulling it first"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func ollamaRegistry(t *testing.T, url, extra string) *Registry {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // an empty credential store
	r, err := NewRegistry(mustCfg(t, `{"providers":{"ollama":{"baseUrl":"`+url+`"`+extra+`}}}`), http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.getenv = func(string) string { return "" }
	return r
}

func TestOllamaDiscovery(t *testing.T) {
	f := newFakeOllama(t)
	f.tags = []string{"qwen3:8b", "nomic-embed-text:latest", "tinyllama:1b", "llama3.1:8b", "mystery:7b"}
	f.show["qwen3:8b"] = ollamaShow{Capabilities: []string{"completion", "tools", "thinking"}, Parameters: "num_ctx                        32768\nstop \"<|im_end|>\""}
	f.show["nomic-embed-text:latest"] = ollamaShow{Capabilities: []string{"embedding"}}
	f.show["tinyllama:1b"] = ollamaShow{Capabilities: []string{"completion"}} // no tool support
	f.show["llama3.1:8b"] = ollamaShow{Capabilities: []string{"completion", "tools"}}
	// mystery:7b: /api/show fails → kept, window assumed
	r := ollamaRegistry(t, f.URL, `,"models":{"declared:3b":{"contextWindow":65536},"llama3.1:8b":{"contextWindow":40000}}`)

	d, err := r.DiscoverOllama(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(d.Models, " "); got != "llama3.1:8b mystery:7b qwen3:8b" {
		t.Fatalf("discovered (embedding and tool-less models excluded): %s", got)
	}
	m, _, err := r.Resolve("ollama/qwen3:8b")
	if err != nil || m.ContextWindow != 32768 || m.Protocol != "openai-completions" || m.ThinkingMode != "openai" {
		t.Fatalf("qwen3: %+v %v", m, err)
	}
	if m.ThinkingLevelMap[llm.EffortOff] != "none" || m.ThinkingLevelMap[llm.EffortHigh] != "high" {
		t.Fatalf("thinking levels: %v", m.ThinkingLevelMap)
	}
	if m.MaxOutput != 8192 {
		t.Fatalf("max output is a quarter of the window: %d", m.MaxOutput)
	}
	// The server reported nothing for this one: the window is assumed.
	if m, _, _ := r.Resolve("ollama/mystery:7b"); m.ContextWindow != ollamaAssumedWindow || m.ThinkingMode != "none" {
		t.Fatalf("mystery: %+v", m)
	}
	// llama3.1:8b went unreported too, but its window is declared — a
	// decision, not a guess — so only mystery:7b is flagged as assumed.
	if got := strings.Join(d.Assumed, " "); got != "mystery:7b" {
		t.Fatalf("assumed windows: %q", got)
	}
	// A declared window overrides the discovered/assumed one; a declared-only
	// model exists although the server does not list it.
	if m, _, _ := r.Resolve("ollama/llama3.1:8b"); m.ContextWindow != 40000 {
		t.Fatalf("override: %+v", m)
	}
	if m, _, err := r.Resolve("ollama/declared:3b"); err != nil || m.ContextWindow != 65536 || m.Protocol != "openai-completions" {
		t.Fatalf("declared-only: %+v %v", m, err)
	}
	if _, _, err := r.Resolve("ollama/nomic-embed-text:latest"); err == nil {
		t.Fatal("embedding models are not offered")
	}

	// Rediscovery: a model removed from the server disappears; a new one appears.
	f.tags = []string{"qwen3:8b", "gemma3:4b"}
	f.show["gemma3:4b"] = ollamaShow{Capabilities: []string{"completion", "tools"}, Parameters: "num_ctx 20000"}
	if _, err := r.DiscoverOllama(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Resolve("ollama/mystery:7b"); err == nil {
		t.Fatal("a model gone from the server must disappear")
	}
	if m, _, err := r.Resolve("ollama/gemma3:4b"); err != nil || m.ContextWindow != 20000 {
		t.Fatalf("new model: %+v %v", m, err)
	}
	if _, _, err := r.Resolve("ollama/declared:3b"); err != nil {
		t.Fatal("declared models survive rediscovery")
	}
}

func TestOllamaKeylessAndOptionalKey(t *testing.T) {
	f := newFakeOllama(t)
	f.tags = nil
	r := ollamaRegistry(t, f.URL, "")
	if _, err := r.DiscoverOllama(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.auth.Load().(string); got != "" {
		t.Fatalf("no key configured → no Authorization header, got %q", got)
	}
	if err := r.CheckCredential("ollama/x"); err == nil || strings.Contains(err.Error(), "API key") {
		t.Fatalf("unknown model, but never a missing-key complaint: %v", err)
	}

	// With a key (a proxy in front of the server), it is sent.
	t.Setenv("OLLAMA_PROXY_KEY", "s3cret")
	r = ollamaRegistry(t, f.URL, `,"apiKey":"env:OLLAMA_PROXY_KEY"`)
	if _, err := r.DiscoverOllama(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.auth.Load().(string); got != "Bearer s3cret" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestOllamaChatWithoutKey(t *testing.T) {
	f := newFakeOllama(t)
	f.tags = []string{"llama3.1:8b"}
	f.show["llama3.1:8b"] = ollamaShow{Capabilities: []string{"completion", "tools"}, Parameters: "num_ctx 32768"}
	r := ollamaRegistry(t, f.URL, "")
	if _, err := r.DiscoverOllama(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, a, err := r.Resolve("ollama/llama3.1:8b")
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	resp, err := a.Stream(context.Background(), llm.Request{Model: "llama3.1:8b", MaxTokens: 100,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}}},
		func(e llm.Event) {
			if e.Type == llm.EventText {
				text.WriteString(e.Text)
			}
		})
	if err != nil || text.String() != "hi" || resp.Usage.Input != 7 {
		t.Fatalf("stream: %q %+v %v", text.String(), resp.Usage, err)
	}
	if got, _ := f.auth.Load().(string); got != "" {
		t.Fatalf("chat request carried %q", got)
	}
	// A model that was never pulled says how to get it.
	_, a, _ = r.Resolve("ollama/llama3.1:8b")
	_, err = a.Stream(context.Background(), llm.Request{Model: "ghost:1b", MaxTokens: 10,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}}}, func(llm.Event) {})
	if err == nil || !strings.Contains(err.Error(), "ollama pull ghost:1b") {
		t.Fatalf("404 hint: %v", err)
	}
}

func TestOllamaServerDown(t *testing.T) {
	f := newFakeOllama(t)
	url := f.URL
	f.Close() // nothing listens there now
	r := ollamaRegistry(t, url, `,"models":{"declared:3b":{}}`)
	_, err := r.DiscoverOllama(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cannot reach ollama") || !strings.Contains(err.Error(), "ollama serve") {
		t.Fatalf("discovery error: %v", err)
	}
	// Declared models still resolve; a chat request fails at once, not after
	// the retry ladder, with a message that says what to do.
	_, a, err := r.Resolve("ollama/declared:3b")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = a.Stream(context.Background(), llm.Request{Model: "declared:3b", MaxTokens: 10,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}}}, func(llm.Event) {})
	if err == nil || !strings.Contains(err.Error(), "cannot reach ollama") {
		t.Fatalf("chat error: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("an unreachable server must not be retried for long (%v)", time.Since(start))
	}
}

func TestOllamaVerify(t *testing.T) {
	f := newFakeOllama(t)
	f.tags = []string{"llama3.1:8b"}
	f.show["llama3.1:8b"] = ollamaShow{Capabilities: []string{"completion", "tools"}}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	mk := func(model string) *Registry {
		r, err := NewRegistry(mustCfg(t, `{"model":"`+model+`","providers":{"ollama":{"baseUrl":"`+f.URL+`"}}}`), http.DefaultClient, nil)
		if err != nil {
			t.Fatalf("NewRegistry must not reject a local model before discovery: %v", err)
		}
		return r
	}
	r := mk("ollama/llama3.1:8b")
	r.DiscoverOllama(context.Background())
	if err := r.Verify(); err != nil {
		t.Fatal(err)
	}
	r = mk("ollama/other:1b")
	r.DiscoverOllama(context.Background())
	if err := r.Verify(); err == nil || !strings.Contains(err.Error(), "ollama pull other:1b") {
		t.Fatalf("missing model: %v", err)
	}
	down := f.URL
	f.Close()
	r, _ = NewRegistry(mustCfg(t, `{"model":"ollama/llama3.1:8b","providers":{"ollama":{"baseUrl":"`+down+`"}}}`), http.DefaultClient, nil)
	r.DiscoverOllama(context.Background())
	if err := r.Verify(); err == nil || !strings.Contains(err.Error(), "cannot reach ollama") {
		t.Fatalf("server down: %v", err)
	}
	// Other providers' models are still verified against the catalog.
	if _, err := NewRegistry(mustCfg(t, `{"model":"anthropic/nope-9"}`), http.DefaultClient, nil); err == nil {
		t.Fatal("catalog check intact")
	}
}

func TestOllamaUnauthorizedAndWrongServer(t *testing.T) {
	status := 401
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
	defer srv.Close()
	r := ollamaRegistry(t, srv.URL, "")
	if _, err := r.DiscoverOllama(context.Background()); err == nil || !strings.Contains(err.Error(), "/login ollama") {
		t.Fatalf("401: %v", err)
	}
	status = 404
	if _, err := r.DiscoverOllama(context.Background()); err == nil || !strings.Contains(err.Error(), "is that an Ollama server") {
		t.Fatalf("404: %v", err)
	}
}

// Discovery (the TUI's /model refresh, a /clear restart) and Verify (a
// session start) can run concurrently on a live registry: the failure
// bookkeeping they share must be synchronized.
func TestOllamaDiscoveryVerifyConcurrent(t *testing.T) {
	f := newFakeOllama(t)
	url := f.URL
	f.Close() // every discovery fails, recording why
	r := ollamaRegistry(t, url, `,"models":{"declared:3b":{}}`)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = r.DiscoverOllama(context.Background()) }()
		go func() { defer wg.Done(); _ = r.Verify() }()
	}
	wg.Wait()
}

func TestNoOllamaNoProbe(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	t.Setenv("OLLAMA_HOST", srv.URL)
	r, err := NewRegistry(mustCfg(t, `{"model":"anthropic/claude-haiku-4-5"}`), http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d, err := r.DiscoverOllama(context.Background()); err != nil || len(d.Models) != 0 || hits.Load() != 0 {
		t.Fatalf("without a providers.ollama opt-in nothing is probed (hits=%d): %v", hits.Load(), err)
	}
}

// internal/provider/registry_test.go
package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

func mustCfg(t *testing.T, s string) config.Config {
	t.Helper()
	c, err := config.Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestResolveBuiltin(t *testing.T) {
	r, err := NewRegistry(mustCfg(t, `{"model":"opencode-go/glm-5.3-flash"}`), http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, a, err := r.Resolve("opencode-go/glm-5.3-flash")
	if err != nil || a == nil || m.ContextWindow == 0 {
		t.Fatal(m, err)
	}
}

func TestUnknownModel(t *testing.T) {
	_, err := NewRegistry(mustCfg(t, `{"model":"anthropic/nope-9"}`), http.DefaultClient, nil)
	if err == nil || !strings.Contains(err.Error(), "anthropic/nope-9") {
		t.Fatalf("%v", err)
	}
}

func TestBaseURLPrecedence(t *testing.T) {
	r, _ := NewRegistry(mustCfg(t, `{"providers":{"opencode-go":{"auth":"api_key","apiKey":"env:OPENCODE_API_KEY",
		"baseUrl":"http://all","baseUrls":{"anthropic-messages":"http://anth"}}}}`), http.DefaultClient, nil)
	if got := r.baseURL("opencode-go", "anthropic-messages"); got != "http://anth" {
		t.Fatal(got)
	}
	if got := r.baseURL("opencode-go", "openai-completions"); got != "http://all" {
		t.Fatal(got)
	}
	r2, _ := NewRegistry(config.Default(), http.DefaultClient, nil)
	if got := r2.baseURL("opencode-go", "openai-responses"); got != "https://opencode.ai/zen/go/v1" {
		t.Fatal(got)
	}
}

func TestCustomProviderAndLazyMissingKey(t *testing.T) {
	var gotAuth string
	srv := sseServer(t, 200, "data: [DONE]\n\n", nil, nil)
	defer srv.Close()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n\n"))
	})
	cfg := mustCfg(t, `{"model":"vllm/qwen","providers":{"vllm":{"baseUrl":"`+srv.URL+`/v1","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_T_VLLM","models":{"qwen":{"contextWindow":32768}}}}}`)
	r, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal("missing key must not fail registry construction:", err)
	}
	_, a, err := r.Resolve("vllm/qwen")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCA_T_VLLM", "")
	_, err = a.Stream(context.Background(), llm.Request{Model: "qwen", MaxTokens: 1}, func(llm.Event) {})
	if err == nil || !strings.Contains(err.Error(), "MOCA_T_VLLM") {
		t.Fatalf("missing key error must name the variable: %v", err)
	}
	t.Setenv("MOCA_T_VLLM", "sekret")
	if _, err = a.Stream(context.Background(), llm.Request{Model: "qwen", MaxTokens: 1}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sekret" {
		t.Fatal(gotAuth)
	}
}

func TestBuiltinModelNeedsProtocol(t *testing.T) {
	_, err := NewRegistry(mustCfg(t, `{"model":"opencode-go/new-model","providers":{"opencode-go":{"auth":"api_key","apiKey":"env:OPENCODE_API_KEY","models":{"new-model":{"contextWindow":200000}}}}}`), http.DefaultClient, nil)
	if err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("new model under a built-in provider must declare its protocol: %v", err)
	}
}

func TestBuiltinProviderNewModelWithModelProtocol(t *testing.T) {
	// The protocol may be declared on the model itself; the provider has none.
	r, err := NewRegistry(mustCfg(t, `{"model":"opencode-go/new-model","providers":{"opencode-go":{"auth":"api_key","apiKey":"env:OPENCODE_API_KEY","models":{"new-model":{"protocol":"openai-completions","contextWindow":200000}}}}}`), http.DefaultClient, nil)
	if err != nil {
		t.Fatalf("model-level protocol must satisfy the check: %v", err)
	}
	m, a, err := r.Resolve("opencode-go/new-model")
	if err != nil || a == nil || m.Protocol != "openai-completions" {
		t.Fatalf("%+v %v", m, err)
	}
}

func TestOverrideBuiltinModelFields(t *testing.T) {
	r, _ := NewRegistry(mustCfg(t, `{"providers":{"opencode-go":{"auth":"api_key","apiKey":"env:OPENCODE_API_KEY",
		"models":{"glm-5.3":{"contextWindow":200000}}}}}`), http.DefaultClient, nil)
	m, _, _ := r.Resolve("opencode-go/glm-5.3")
	if m.ContextWindow != 200000 || m.Cost.Input != 1.4 {
		t.Fatalf("override merges, not replaces: %+v", m)
	}
}

func TestOpenCodeGoSessionHeader(t *testing.T) {
	// The Go tier requires a stable per-conversation routing header
	// (https://opencode.ai/docs/go/#where-can-i-use-it); without it every
	// request is rejected with HTTP 400 MissingSessionID (found in the live
	// smoke, 2026-10-05).
	t.Setenv("OPENCODE_API_KEY", "K")
	var hdrs []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdrs = append(hdrs, r.Header.Clone())
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	r, err := NewRegistry(mustCfg(t, fmt.Sprintf(
		`{"model":"opencode-go/glm-5.3-flash","providers":{"opencode-go":{"baseUrls":{"openai-completions":%q}}}}`, srv.URL)),
		srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	m, a, err := r.Resolve("opencode-go/glm-5.3-flash")
	if err != nil {
		t.Fatal(err)
	}
	req := llm.Request{Model: m.ID, MaxTokens: 64, Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}}}
	for i := 0; i < 2; i++ {
		if _, err := a.Stream(context.Background(), req, func(llm.Event) {}); err != nil {
			t.Fatal(err)
		}
	}
	if len(hdrs) != 2 {
		t.Fatalf("requests: %d", len(hdrs))
	}
	s1, s2 := hdrs[0].Get("x-opencode-session"), hdrs[1].Get("x-opencode-session")
	if s1 == "" || s1 != s2 {
		t.Fatalf("session header %q vs %q: must be present and stable", s1, s2)
	}
	if ua := hdrs[0].Get("User-Agent"); ua != "moca/"+config.Version {
		t.Fatalf("user-agent %q, want moca/%s", ua, config.Version)
	}
}

func TestNoSessionHeaderOutsideOpenCodeGo(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "K")
	var hdr http.Header
	srv := sseServer(t, 200, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", nil, &hdr)
	defer srv.Close()
	r, err := NewRegistry(mustCfg(t, fmt.Sprintf(
		`{"model":"anthropic/claude-haiku-4-5","providers":{"anthropic":{"baseUrls":{"anthropic-messages":%q}}}}`, srv.URL)),
		srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	m, a, err := r.Resolve("anthropic/claude-haiku-4-5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Stream(context.Background(), llm.Request{Model: m.ID, MaxTokens: 16}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	if hdr.Get("x-opencode-session") != "" {
		t.Fatalf("OpenCode routing header must not leak to other providers: %v", hdr)
	}
}

// OAuth (Sign in with ChatGPT) is store-backed: with a token in auth.json
// and auth "oauth", requests carry the stored access token as a bearer.
func TestOAuthCredentialFromStore(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store := NewStore(filepath.Join(config.DataDir(), "auth.json"))
	if err := store.Put("openai", Token{Access: "AT-1", Expiry: time.Now().Add(time.Hour), ClientID: "oaiapp_x"}); err != nil {
		t.Fatal(err)
	}
	var hdr http.Header
	srv := sseServer(t, 200, `event: response.completed
data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":1}}}

`, nil, &hdr)
	defer srv.Close()
	cfg := mustCfg(t, fmt.Sprintf(
		`{"model":"openai/gpt-6-astra","providers":{"openai":{"auth":"oauth","baseUrls":{"openai-responses":%q}}}}`, srv.URL))
	r, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	m, a, err := r.Resolve("openai/gpt-6-astra")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Stream(context.Background(), llm.Request{Model: m.ID, MaxTokens: 16}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	if hdr.Get("Authorization") != "Bearer AT-1" {
		t.Fatalf("oauth bearer: %v", hdr)
	}
}

func TestOAuthNotLoggedIn(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := mustCfg(t, `{"model":"openai/gpt-6-astra","providers":{"openai":{"auth":"oauth"}}}`)
	r, err := NewRegistry(cfg, http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, a, err := r.Resolve("openai/gpt-6-astra")
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Stream(context.Background(), llm.Request{Model: "gpt-6-astra", MaxTokens: 16}, func(llm.Event) {})
	if err == nil || !strings.Contains(err.Error(), "moca login openai") {
		t.Fatalf("not-logged-in error must point at `moca login openai`: %v", err)
	}
}

func TestOAuthRequiresResponsesProtocol(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := mustCfg(t, `{"model":"openai/gpt-6-astra","providers":{"openai":{"auth":"oauth","models":{"gpt-6-astra":{"protocol":"openai-completions"}}}}}`)
	r, err := NewRegistry(cfg, http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Resolve("openai/gpt-6-astra"); err == nil || !strings.Contains(err.Error(), "openai-responses") {
		t.Fatalf("oauth + completions protocol must be refused: %v", err)
	}
}

type countingTransport struct {
	rt   http.RoundTripper
	hits *atomic.Int32
}

func (c countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.hits.Add(1)
	return c.rt.RoundTrip(r)
}

// CheckCredential runs on the TUI's update goroutine (/model): for an OAuth
// provider it must only confirm a login exists. An expired token is the first
// request's job to refresh, not a blocking, uncancellable network call here.
func TestCheckCredentialOAuthDoesNotTouchNetwork(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	f := newFakeAS(t)
	orig := oauthProviders["openai"]
	oauthProviders["openai"] = f.config()
	defer func() { oauthProviders["openai"] = orig }()
	var hits atomic.Int32
	hc := &http.Client{Transport: countingTransport{http.DefaultTransport, &hits}}
	cfg := mustCfg(t, `{"model":"openai/gpt-6-astra","providers":{"openai":{"auth":"oauth"}}}`)
	r, err := NewRegistry(cfg, hc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCredential("openai/gpt-6-astra"); err == nil || !strings.Contains(err.Error(), "moca login openai") {
		t.Fatalf("no stored login must point at `moca login openai`: %v", err)
	}
	store := NewStore(filepath.Join(config.DataDir(), "auth.json"))
	if err := store.Put("openai", Token{Access: "OLD", Refresh: "RT", ClientID: "oaiapp_x", Expiry: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCredential("openai/gpt-6-astra"); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 {
		t.Fatalf("CheckCredential made %d network requests", hits.Load())
	}
	if tok, _, _ := store.Get("openai"); tok.Access != "OLD" {
		t.Fatalf("CheckCredential must not rewrite the store: %+v", tok)
	}
}

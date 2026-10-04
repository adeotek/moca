// internal/provider/registry_test.go
package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"

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

func TestOverrideBuiltinModelFields(t *testing.T) {
	r, _ := NewRegistry(mustCfg(t, `{"providers":{"opencode-go":{"auth":"api_key","apiKey":"env:OPENCODE_API_KEY",
		"models":{"glm-5.3":{"contextWindow":200000}}}}}`), http.DefaultClient, nil)
	m, _, _ := r.Resolve("opencode-go/glm-5.3")
	if m.ContextWindow != 200000 || m.Cost.Input != 1.4 {
		t.Fatalf("override merges, not replaces: %+v", m)
	}
}

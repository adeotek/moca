package provider

// Tests for provider model discovery (SPECS §6, Tasks 2–4, 6, 10).

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

// blankBuiltinKeys isolates a test from ambient credentials: this host's
// environment carries the real provider keys, and a ready provider would be
// discovered against its real endpoint from a unit test.
func blankBuiltinKeys(t *testing.T) {
	t.Helper()
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENCODE_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(k, "")
	}
	// A key stored by a real /login (auth.json) would also make a provider
	// ready: point the whole config/state tree at temp dirs.
	if os.Getenv("XDG_CONFIG_HOME") == "" {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	if os.Getenv("XDG_STATE_HOME") == "" {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
	}
}

// jsonServer answers every request with body (and counts hits).
func jsonServer(t *testing.T, hits *atomic.Int32, body func(r *http.Request) (int, string)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		status, payload := body(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchAnthropicModels(t *testing.T) {
	var queries []string
	srv := jsonServer(t, nil, func(r *http.Request) (int, string) {
		if r.Header.Get("x-api-key") != "K" || r.Header.Get("anthropic-version") == "" {
			return http.StatusUnauthorized, `{}`
		}
		if got := r.URL.Query().Get("limit"); got != "1000" {
			return http.StatusBadRequest, `{}`
		}
		queries = append(queries, r.URL.RawQuery)
		if r.URL.Query().Get("after_id") == "" {
			return 200, `{"data":[
				{"id":"claude-a","lifecycle":"active","max_input_tokens":200000,"max_tokens":64000,
				 "capabilities":{"effort":{"low":{"supported":true},"high":{"supported":true},"supported":true},
				                 "thinking":{"types":{"adaptive":{"supported":true},"enabled":{"supported":true}}}}},
				{"id":"claude-old","lifecycle":"retired"}],
				"has_more":true,"last_id":"claude-a"}`
		}
		return 200, `{"data":[{"id":"claude-b","max_input_tokens":1000000,"max_tokens":128000,
			"capabilities":{"effort":{},"thinking":{"types":{"enabled":{"supported":true}}}}}],"has_more":false}`
	})
	hdr := http.Header{}
	hdr.Set("x-api-key", "K")
	hdr.Set("anthropic-version", "2023-06-01")
	models, err := fetchAnthropicModels(context.Background(), srv.Client(), srv.URL, hdr)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("want 2 active models, got %+v", models)
	}
	a := models[0]
	if a.ID != "claude-a" || a.Ctx != 200000 || a.MaxOut != 64000 || a.Mode != "adaptive" ||
		strings.Join(a.Efforts, ",") != "low,high" || a.Protocol != "anthropic-messages" {
		t.Fatalf("page-1 model: %+v", a)
	}
	if b := models[1]; b.ID != "claude-b" || b.Mode != "budget" || b.Ctx != 1000000 {
		t.Fatalf("page-2 model: %+v", b)
	}
	if len(queries) != 2 || !strings.Contains(queries[1], "after_id=claude-a") {
		t.Fatalf("pagination: %v", queries)
	}
}

func TestFetchOpenAIModels(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"object":"list","data":[
			{"id":"gpt-6-luna"},{"id":"text-embedding-3-small"},{"id":"whisper-1"},
			{"id":"dall-e-3"},{"id":"gpt-4o-realtime-preview"},{"id":"gpt-5.1","shutdown_date":"2026-10-23"}]}`
	})
	models, err := fetchOpenAIModels(context.Background(), srv.Client(), srv.URL, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gpt-6-luna" || models[0].Protocol != "openai-completions" {
		t.Fatalf("filters: %+v", models)
	}
}

func TestFetchOpenRouterModels(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[
			{"id":"vendor/model-a","context_length":400000,
			 "pricing":{"prompt":"0.000002","completion":"0.00001","input_cache_read":"0.0000002"},
			 "supported_parameters":["tools"],
			 "top_provider":{"max_completion_tokens":128000},
			 "reasoning":{"supported_efforts":["max","high","low"],"mandatory":true}},
			{"id":"vendor/image-model","context_length":32000,"pricing":{"prompt":"0.000001"},
			 "supported_parameters":["temperature"]},
			{"id":"vendor/cheap:free","pricing":{"prompt":"0"},"supported_parameters":["tools"]},
			{"id":"openrouter/auto","supported_parameters":["tools"]}]}`
	})
	models, err := fetchOpenRouterModels(context.Background(), srv.Client(), srv.URL, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Fatalf("filters (tools only, no :free/router ids): %+v", models)
	}
	m := models[0]
	if m.ID != "vendor/model-a" || m.Ctx != 400000 || m.MaxOut != 128000 {
		t.Fatalf("limits: %+v", m)
	}
	if !m.CostKnown || math.Abs(m.Cost.Input-2) > 1e-9 || math.Abs(m.Cost.Output-10) > 1e-9 || math.Abs(m.Cost.CacheRead-0.2) > 1e-9 {
		t.Fatalf("pricing per million: %+v", m.Cost)
	}
	if m.Mode != "effort-object" || !m.Mandatory || strings.Join(m.Efforts, ",") != "max,high,low" {
		t.Fatalf("reasoning: %+v", m)
	}
}

// discoveryCfg builds a registry whose only provider is a custom gateway with
// no declared models — the shape that opts into discovery.
func discoveryCfg(t *testing.T, url string) config.Config {
	t.Helper()
	blankBuiltinKeys(t)
	if os.Getenv("XDG_DATA_HOME") == "" {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	t.Setenv("MOCA_DISC_TEST_KEY", "k")
	return mustCfg(t, `{"providers":{"gw":{"baseUrl":"`+url+`","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_DISC_TEST_KEY"}}}`)
}

func TestDiscoverModelsUsesTheCache(t *testing.T) {
	var hits atomic.Int32
	srv := jsonServer(t, &hits, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"m"}]}`
	})
	reg, err := NewRegistry(discoveryCfg(t, srv.URL), srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	rep := reg.DiscoverModels(context.Background())
	if d, ok := rep.Provider("gw"); !ok || d.Err != nil || strings.Join(d.Models, ",") != "m" {
		t.Fatalf("first discovery: %+v", rep)
	}
	if hits.Load() != 1 {
		t.Fatalf("first discovery must fetch, hits=%d", hits.Load())
	}
	if _, ok := reg.models["gw/m"]; !ok {
		t.Fatal("discovered model merged")
	}
	// A second registry on the same data dir reads the fresh cache: no network.
	reg2, _ := NewRegistry(discoveryCfg(t, srv.URL), srv.Client(), nil)
	rep = reg2.DiscoverModels(context.Background())
	if hits.Load() != 1 {
		t.Fatalf("a fresh cache must not call the provider, hits=%d", hits.Load())
	}
	if d, _ := rep.Provider("gw"); !d.FromCache || len(d.Models) != 1 {
		t.Fatalf("served from cache: %+v", rep)
	}
	// Past the TTL it refetches.
	reg2.cache.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	reg2.DiscoverModels(context.Background())
	if hits.Load() != 2 {
		t.Fatalf("stale cache must refetch, hits=%d", hits.Load())
	}
}

func TestDiscoverModelsFailOpen(t *testing.T) {
	var fail atomic.Bool
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		if fail.Load() {
			return http.StatusInternalServerError, `{"error":"boom"}`
		}
		return 200, `{"data":[{"id":"m"}]}`
	})
	reg, _ := NewRegistry(discoveryCfg(t, srv.URL), srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	fail.Store(true)
	reg.cache.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	rep := reg.DiscoverModels(context.Background())
	d, _ := rep.Provider("gw")
	if d.Err == nil {
		t.Fatal("a failing provider reports its error")
	}
	if _, ok := reg.models["gw/m"]; !ok {
		t.Fatal("a failed refetch keeps the previous rows (stale-while-revalidate)")
	}
	if err := reg.Verify(); err != nil {
		t.Fatalf("discovery failures never fail a run on their own: %v", err)
	}
}

func TestMergePrecedenceKeepsCuratedMetadata(t *testing.T) {
	// The go catalog's curated glm-5.3 row must survive a thin discovered
	// payload (no window, no price) for the same id.
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"glm-5.3"},{"id":"new-model"}]}`
	})
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_GO_TEST_KEY", "k")
	cfg := mustCfg(t, `{"providers":{"opencode-go":{"baseUrl":"`+srv.URL+`","apiKey":"env:MOCA_GO_TEST_KEY"}}}`)
	reg, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reg.DiscoverModels(context.Background())
	glm, ok := reg.models["opencode-go/glm-5.3"]
	if !ok {
		t.Fatal("curated row present")
	}
	if glm.Cost.Input != 1.4 || glm.ContextWindow != 1_000_000 || glm.ThinkingMode != "openai" || !glm.CostKnown {
		t.Fatalf("curated metadata must win over a thin discovered row: %+v", glm)
	}
	if _, ok := reg.models["opencode-go/new-model"]; !ok {
		t.Fatal("a discovered-only id is added")
	}
	fresh, _ := reg.models["opencode-go/new-model"]
	if fresh.CostKnown || !fresh.WindowAssumed || fresh.ContextWindow != config.MinContextWindow {
		t.Fatalf("unknown metadata stays visibly unknown: %+v", fresh)
	}
}

func TestZenBorrowsTheGoCatalog(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"glm-5.3"},{"id":"zen-only"}]}`
	})
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_ZEN_KEY", "k")
	cfg := mustCfg(t, `{"model":"opencode-zen/glm-5.3","providers":{"opencode-zen":{"baseUrl":"`+srv.URL+`","apiKey":"env:MOCA_ZEN_KEY"}}}`)
	reg, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Verify(); err != nil {
		t.Fatal(err)
	}
	reg.DiscoverModels(context.Background())
	m, ok := reg.models["opencode-zen/glm-5.3"]
	if !ok {
		t.Fatal("seeded row present")
	}
	if m.Provider != "opencode-zen" || m.Cost.Input != 1.4 || m.ThinkingMode != "openai" || len(m.ThinkingLevelMap) == 0 {
		t.Fatalf("zen borrows the go metadata with its own provider: %+v", m)
	}
	if m := reg.models["opencode-zen/zen-only"]; m.CostKnown {
		t.Fatalf("an unseeded zen id stays unpriced: %+v", m)
	}
}

func TestActiveModelsFollowCredentials(t *testing.T) {
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("OPENCODE_API_KEY", "k")
	cfg := mustCfg(t, `{"providers":{"anthropic":{"auth":"api_key","apiKey":"env:ANTHROPIC_API_KEY"}}}`)
	reg, err := NewRegistry(cfg, http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reg.ProviderReady("opencode-zen") || reg.ProviderReady("anthropic") {
		t.Fatalf("readiness: zen=%v anthropic=%v", reg.ProviderReady("opencode-zen"), reg.ProviderReady("anthropic"))
	}
	for _, m := range reg.ActiveModels() {
		if strings.HasPrefix(m.Qualified(), "anthropic/") {
			t.Fatalf("an uncredentialed provider must not be listed: %s", m.Qualified())
		}
	}
	if len(reg.ActiveModels()) == 0 {
		t.Fatal("zen rows are active (shared OPENCODE_API_KEY)")
	}
}

func TestClearModelsCacheDropsDiscoveredRows(t *testing.T) {
	var hits atomic.Int32
	srv := jsonServer(t, &hits, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"m"}]}`
	})
	reg, _ := NewRegistry(discoveryCfg(t, srv.URL), srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	if _, ok := reg.models["gw/m"]; !ok {
		t.Fatal("discovered row present")
	}
	providers, models, err := reg.ClearModelsCache()
	if err != nil || providers != 1 || models != 1 {
		t.Fatalf("clear: %d %d %v", providers, models, err)
	}
	if _, ok := reg.models["gw/m"]; ok {
		t.Fatal("cleared rows must leave the registry")
	}
	reg.DiscoverModels(context.Background())
	if hits.Load() != 2 {
		t.Fatalf("the next discovery refetches, hits=%d", hits.Load())
	}
}

// TestAssumedWindowsFollowTheMergedRow: a provider whose payload carries no
// window must not flag a curated model as a guess — the merged row has the
// real window, and only rows that actually substituted 16384 are assumed.
func TestAssumedWindowsFollowTheMergedRow(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"glm-5.3"},{"id":"brand-new"}]}`
	})
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_GO_KEY", "k")
	cfg := mustCfg(t, `{"providers":{"opencode-go":{"baseUrl":"`+srv.URL+`","apiKey":"env:MOCA_GO_KEY"}}}`)
	reg, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := reg.DiscoverModels(context.Background()).Provider("opencode-go")
	if slices.Contains(d.Assumed, "glm-5.3") {
		t.Fatal("a curated window is not a guess")
	}
	if !slices.Contains(d.Assumed, "brand-new") {
		t.Fatalf("a discovered-only id with no window is a guess: %v", d.Assumed)
	}
}

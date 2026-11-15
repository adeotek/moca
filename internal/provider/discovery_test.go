package provider

// Tests for provider model discovery (SPECS §6, Tasks 2–4, 6, 10).

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/testenv"
)

// blankBuiltinKeys isolates a test from ambient credentials: this host's
// environment carries the real provider keys, and a ready provider would be
// discovered against its real endpoint from a unit test.
func blankBuiltinKeys(t *testing.T) {
	t.Helper()
	testenv.BlankKeys(t)
	// A key stored by a real /login (auth.json) would also make a provider
	// ready: point the whole config/state tree at temp dirs — always, since a
	// host that exports XDG_CONFIG_HOME names exactly the real store.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
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
	// The Responses API, like the curated openai rows: chat completions
	// rejects max_tokens on the reasoning models, and an OAuth login can use
	// nothing else.
	if len(models) != 1 || models[0].ID != "gpt-6-luna" || models[0].Protocol != "openai-responses" {
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
	// Always a fresh data dir: a host-exported XDG_DATA_HOME is the user's
	// real models cache, which ClearModelsCache would delete.
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_DISC_TEST_KEY", "k")
	return mustCfg(t, `{"providers":{"gw":{"baseUrl":"`+url+`","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_DISC_TEST_KEY"}}}`)
}

func TestDiscoverModelsUsesTheCache(t *testing.T) {
	var hits atomic.Int32
	srv := jsonServer(t, &hits, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"m"}]}`
	})
	cfg := discoveryCfg(t, srv.URL)
	reg, err := NewRegistry(cfg, srv.Client(), nil)
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
	reg2, _ := NewRegistry(cfg, srv.Client(), nil)
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

// TestQuirkOverlaysThePayload: the provider's payload supplies the facts, and
// a quirk only corrects what it states wrongly — it widens glm-5.3's effort
// list (the payload omits "medium") and clamps kimi-k3's output, but never
// replaces a published window or price.
func TestQuirkOverlaysThePayload(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[
			{"id":"glm-5.3","limit":{"context":200000,"output":65536},
			 "cost":{"text":{"input":{"price":1.1,"cache":{"read":0.2}},"output":{"price":3.3}}},
			 "capabilities":{"tools":{"supported":true},"reasoning":{"supported":true,"effort":["low","high","max"]}}},
			{"id":"kimi-k3","limit":{"context":800000,"output":1048576},
			 "cost":{"text":{"input":{"price":3},"output":{"price":15}}},
			 "capabilities":{"reasoning":{"supported":true,"effort":["max"]}}}]}`
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
	glm := reg.models["opencode-go/glm-5.3"]
	if glm.ContextWindow != 200000 || glm.WindowAssumed || glm.MaxOutput != 65536 {
		t.Fatalf("the payload's limits must stand: %+v", glm)
	}
	if !glm.CostKnown || glm.Cost.Input != 1.1 || glm.Cost.CacheRead != 0.2 || glm.Cost.Output != 3.3 {
		t.Fatalf("the payload's price must stand: %+v", glm.Cost)
	}
	if _, ok := glm.ThinkingLevelMap[llm.EffortMedium]; !ok {
		t.Fatalf("the quirk's effort set must apply: %v", glm.ThinkingLevelMap)
	}
	kimi := reg.models["opencode-go/kimi-k3"]
	if kimi.MaxOutput != 131072 {
		t.Fatalf("the quirk's output clamp must apply: %d", kimi.MaxOutput)
	}
	if _, ok := kimi.ThinkingLevelMap[llm.EffortOff]; !ok {
		t.Fatal("kimi-k3 can disable thinking (the quirk's set, not the payload's [max])")
	}
}

// TestZenPricedFromItsOwnPayload: zen publishes its own prices, and they are
// not the go tier's (deepseek-v4.1-flash is twice the go rate) — the payload
// wins, so a run's cost is priced on the surface it actually billed.
func TestZenPricedFromItsOwnPayload(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"glm-5.3","limit":{"context":1000000,"output":131072},
			"cost":{"text":{"input":{"price":2.5,"cache":{"read":0.5}},"output":{"price":5}}},
			"capabilities":{"reasoning":{"supported":true,"effort":["low","high","max"]}}}]}`
	})
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_ZEN_KEY", "k")
	cfg := mustCfg(t, `{"model":"opencode/glm-5.3","providers":{"opencode":{"baseUrl":"`+srv.URL+`","apiKey":"env:MOCA_ZEN_KEY"}}}`)
	reg, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reg.DiscoverModels(context.Background())
	if err := reg.Verify(); err != nil {
		t.Fatal(err)
	}
	m, ok := reg.models["opencode/glm-5.3"]
	if !ok {
		t.Fatal("the listed row is present")
	}
	if m.Provider != "opencode" || m.Cost.Input != 2.5 || m.Cost.Output != 5 || !m.CostKnown {
		t.Fatalf("zen's own price must stand: %+v", m.Cost)
	}
	if m.ThinkingMode != "openai" || len(m.ThinkingLevelMap) == 0 {
		t.Fatalf("the vendor quirk applies on zen too: %+v", m)
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
	if !reg.ProviderReady("opencode") || reg.ProviderReady("anthropic") {
		t.Fatalf("readiness: zen=%v anthropic=%v", reg.ProviderReady("opencode"), reg.ProviderReady("anthropic"))
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

// /clear-models-cache must not break the session: the active model's
// provider is re-read at once (its file recreated) and its rows never leave
// the registry meanwhile.
func TestClearModelsCacheRescansTheActiveProvider(t *testing.T) {
	var hits atomic.Int32
	srv := jsonServer(t, &hits, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"m"}]}`
	})
	reg, _ := NewRegistry(discoveryCfg(t, srv.URL), srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	rep, err := reg.ClearModelsCache(context.Background(), "gw")
	if err != nil || rep.Providers != 1 || rep.Models != 1 {
		t.Fatalf("clear: %+v %v", rep, err)
	}
	if hits.Load() != 2 {
		t.Fatalf("the active provider is re-read at once, hits=%d", hits.Load())
	}
	if rep.Rescan.Provider != "gw" || rep.Rescan.Err != nil || strings.Join(rep.Rescan.Models, ",") != "m" {
		t.Fatalf("rescan report: %+v", rep.Rescan)
	}
	if _, fresh := reg.cache.Load("gw", rep.Rescan.Base); !fresh {
		t.Fatal("the re-read recreates the provider's cache file")
	}
	if _, _, err := reg.Resolve("gw/m"); err != nil {
		t.Fatalf("the session's model must still resolve: %v", err)
	}
}

// The other providers are not re-read now: their files go, their rows stay
// (a session may be using one as its cheap or judge model), and the next
// discovery — which the deleted file can no longer serve — refetches them.
func TestClearModelsCacheKeepsOtherProvidersRows(t *testing.T) {
	var hits1, hits2 atomic.Int32
	srv1 := jsonServer(t, &hits1, func(*http.Request) (int, string) { return 200, `{"data":[{"id":"a"}]}` })
	srv2 := jsonServer(t, &hits2, func(*http.Request) (int, string) { return 200, `{"data":[{"id":"b"}]}` })
	discoveryCfg(t, srv1.URL) // env isolation
	cfg := mustCfg(t, `{"providers":{
		"gw1":{"baseUrl":"`+srv1.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_DISC_TEST_KEY"},
		"gw2":{"baseUrl":"`+srv2.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_DISC_TEST_KEY"}}}`)
	reg, _ := NewRegistry(cfg, http.DefaultClient, nil)
	reg.DiscoverModels(context.Background())
	rep, err := reg.ClearModelsCache(context.Background(), "gw1")
	if err != nil || rep.Providers != 2 {
		t.Fatalf("clear: %+v %v", rep, err)
	}
	if hits1.Load() != 2 || hits2.Load() != 1 {
		t.Fatalf("only the active provider is re-read now: gw1=%d gw2=%d", hits1.Load(), hits2.Load())
	}
	if _, _, err := reg.Resolve("gw2/b"); err != nil {
		t.Fatalf("gw2's rows stay until a refetch replaces them: %v", err)
	}
	reg.DiscoverModels(context.Background())
	if hits2.Load() != 2 {
		t.Fatalf("the next discovery refetches gw2, hits=%d", hits2.Load())
	}
}

// A re-read that fails reports why and leaves the provider's rows in place.
func TestClearModelsCacheFailedRescanKeepsRows(t *testing.T) {
	var down atomic.Bool
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		if down.Load() {
			return http.StatusBadGateway, `{}`
		}
		return 200, `{"data":[{"id":"m"}]}`
	})
	reg, _ := NewRegistry(discoveryCfg(t, srv.URL), srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	down.Store(true)
	rep, err := reg.ClearModelsCache(context.Background(), "gw")
	if err != nil || rep.Rescan.Err == nil {
		t.Fatalf("the failed re-read is reported: %+v %v", rep, err)
	}
	if _, _, err := reg.Resolve("gw/m"); err != nil {
		t.Fatalf("a failed re-read must not cost the session its model: %v", err)
	}
}

// Ollama has no cache file, but it is the active provider all the same: the
// clear re-reads the server, so a model pulled meanwhile shows up.
func TestClearModelsCacheRescansOllama(t *testing.T) {
	f := newFakeOllama(t)
	f.tags = []string{"a:1b"}
	f.show["a:1b"] = ollamaShow{Capabilities: []string{"completion", "tools"}}
	f.show["b:1b"] = ollamaShow{Capabilities: []string{"completion", "tools"}}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	r := ollamaRegistry(t, f.URL, "")
	if _, err := r.DiscoverOllama(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.tags = []string{"a:1b", "b:1b"}
	rep, err := r.ClearModelsCache(context.Background(), "ollama")
	if err != nil || rep.Rescan.Err != nil || strings.Join(rep.Rescan.Models, ",") != "a:1b,b:1b" {
		t.Fatalf("rescan: %+v %v", rep, err)
	}
	if _, _, err := r.Resolve("ollama/b:1b"); err != nil {
		t.Fatalf("the newly pulled model is resolvable: %v", err)
	}
}

// A model the registry has handed out stays resolvable when a later
// discovery stops listing it: the summarizer, the judge and /hard re-resolve
// by name mid-session, and a provider retiring the id must not strand them.
// The live row still wins when there is one.
func TestResolveKeepsAModelItHandedOut(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) { return 200, `{"data":[]}` })
	reg, _ := NewRegistry(discoveryCfg(t, srv.URL), srv.Client(), nil)
	reg.mergeDiscovered("gw", []CachedModel{{ID: "m", Ctx: 100000}, {ID: "n"}})
	if _, _, err := reg.Resolve("gw/m"); err != nil {
		t.Fatal(err)
	}
	reg.mergeDiscovered("gw", []CachedModel{{ID: "k"}})
	if _, ok := reg.models["gw/m"]; ok {
		t.Fatal("setup: the relist must drop gw/m")
	}
	if m, _, err := reg.Resolve("gw/m"); err != nil || m.ContextWindow != 100000 {
		t.Fatalf("a handed-out model must keep resolving: %+v %v", m, err)
	}
	if _, _, err := reg.Resolve("gw/n"); !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("a dropped model nobody resolved is gone: %v", err)
	}
	// The user's /model switch still sees the live catalog only.
	if err := reg.CheckCredential("gw/m"); !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("a switch to a dropped model must be refused: %v", err)
	}
	reg.mergeDiscovered("gw", []CachedModel{{ID: "m", Ctx: 200000}})
	if m, _, _ := reg.Resolve("gw/m"); m.ContextWindow != 200000 {
		t.Fatalf("the live row wins over the remembered one: %d", m.ContextWindow)
	}
}

// Verify checks the configured models at start; they stay resolvable before
// their first use too (the judge may first run an hour into a session).
func TestVerifiedModelsStayResolvable(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"m"},{"id":"h"},{"id":"j"}]}`
	})
	cfg := discoveryCfg(t, srv.URL)
	cfg.Model, cfg.ModelHard, cfg.Permissions.Auto.Model = "gw/m", "gw/h", "gw/j"
	reg, _ := NewRegistry(cfg, srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	if err := reg.Verify(); err != nil {
		t.Fatal(err)
	}
	reg.mergeDiscovered("gw", nil)
	for _, q := range []string{"gw/m", "gw/h", "gw/j"} {
		if _, _, err := reg.Resolve(q); err != nil {
			t.Fatalf("%s: a verified model must keep resolving: %v", q, err)
		}
	}
}

// TestAssumedWindowsFollowThePayload: a window the payload publishes is not a
// guess — even when the id carries a quirk — and an id with no window at all
// is flagged so the startup warning can name it.
func TestAssumedWindowsFollowThePayload(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"glm-5.3","limit":{"context":1000000,"output":131072}},{"id":"brand-new"}]}`
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
		t.Fatal("a published window is not a guess")
	}
	if !slices.Contains(d.Assumed, "brand-new") {
		t.Fatalf("a discovered-only id with no window is a guess: %v", d.Assumed)
	}
}

// TestDiscoverModelsParallelPutsKeepEveryEntry: providers are fetched in
// parallel, and each Put is a read-modify-write of one file — unserialized,
// the later rename dropped the earlier providers' entries, so the next start
// refetched them despite a fresh 24 h cache.
func TestDiscoverModelsParallelPutsKeepEveryEntry(t *testing.T) {
	var hits atomic.Int32
	srv := jsonServer(t, &hits, func(*http.Request) (int, string) {
		time.Sleep(5 * time.Millisecond)
		return 200, `{"data":[{"id":"m"}]}`
	})
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_PAR_KEY", "k")
	var b strings.Builder
	b.WriteString(`{"providers":{`)
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	for i, n := range names {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"` + n + `":{"baseUrl":"` + srv.URL + `","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_PAR_KEY"}`)
	}
	b.WriteString(`}}`)
	cfg := mustCfg(t, b.String())
	reg, _ := NewRegistry(cfg, srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	first := hits.Load()
	reg2, _ := NewRegistry(cfg, srv.Client(), nil)
	reg2.DiscoverModels(context.Background())
	if got := hits.Load() - first; got != 0 {
		t.Fatalf("every provider's entry must survive the parallel writes: %d refetched", got)
	}
}

// TestDiscoveredRowsFollowTheProviderProtocol: a custom gateway declared on
// openai-responses lists ids through the OpenAI-shaped /models, but its
// models speak the provider's protocol, not the fetcher's default.
func TestDiscoveredRowsFollowTheProviderProtocol(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) { return 200, `{"data":[{"id":"m"}]}` })
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_RESP_KEY", "k")
	cfg := mustCfg(t, `{"providers":{"gw":{"baseUrl":"`+srv.URL+`","protocol":"openai-responses","auth":"api_key","apiKey":"env:MOCA_RESP_KEY"}}}`)
	reg, _ := NewRegistry(cfg, srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	if m := reg.models["gw/m"]; m.Protocol != "openai-responses" {
		t.Fatalf("discovered row protocol: %+v", m)
	}
}

// TestDeclaredCustomListIgnoresTheCache: a custom provider's declared list is
// the contract, so rows cached before the declarations must not come back.
func TestDeclaredCustomListIgnoresTheCache(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) { return 200, `{"data":[{"id":"old"}]}` })
	cfg := discoveryCfg(t, srv.URL)
	reg, _ := NewRegistry(cfg, srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	declared := mustCfg(t, `{"providers":{"gw":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_DISC_TEST_KEY","models":{"new":{"contextWindow":32768}}}}}`)
	reg2, _ := NewRegistry(declared, srv.Client(), nil)
	reg2.DiscoverModels(context.Background())
	if _, ok := reg2.models["gw/old"]; ok {
		t.Fatal("a cached row outside the declared list must not be merged")
	}
}

// TestProtocolWithoutABaseURLFallsBack: zen serves chat completions only, so a
// model advertising just the messages surface must not resolve to a route zen
// has no base URL for.
func TestProtocolWithoutABaseURLFallsBack(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"qwen3.8-flash","limit":{"context":1000000,"output":131072},
			"api":{"messages":{"base_url":"https://inference.invalid/v1"}},
			"capabilities":{"reasoning":{"supported":true,"effort":["low","medium","xhigh"]}}}]}`
	})
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_ZEN_KEY", "k")
	cfg := mustCfg(t, `{"providers":{"opencode":{"baseUrls":{"openai-completions":"`+srv.URL+`"},"apiKey":"env:MOCA_ZEN_KEY"}}}`)
	reg, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reg.DiscoverModels(context.Background())
	m := reg.models["opencode/qwen3.8-flash"]
	if m.Protocol != "openai-completions" {
		t.Fatalf("a wire zen cannot serve must fall back to its default: %q", m.Protocol)
	}
	if m.ThinkingMode != "openai" || len(m.ThinkingLevelMap) == 0 {
		t.Fatalf("the effort list survives on the completions route: %+v", m)
	}
}

// TestMessagesOnlyModelUsesTheMessagesRoute: on the go tier the advertised
// messages surface is served, so the row takes the anthropic wire — and the
// thinking knob that wire has, not reasoning_effort.
func TestMessagesOnlyModelUsesTheMessagesRoute(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"minimax-m3","limit":{"context":1000000,"output":131072},
			"api":{"messages":{"base_url":"https://inference.invalid/v1"}},
			"capabilities":{"reasoning":{"supported":true}}}]}`
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
	m := reg.models["opencode-go/minimax-m3"]
	if m.Protocol != "anthropic-messages" || m.ThinkingMode != "budget" {
		t.Fatalf("a messages-only model takes the budget route when it publishes no effort list: %+v", m)
	}
	if _, ok := m.ThinkingLevelMap[llm.EffortOff]; !ok {
		t.Fatal("the budget route can always think less or not at all")
	}
}

// TestMessagesRouteTakesItsThinkingKnobFromThePayload: a messages-route model
// that publishes effort levels takes the ADAPTIVE knob, not the budget one —
// claude 5.5 refuses `thinking.type.enabled` outright ("use
// thinking.type.adaptive and output_config.effort", live-verified on both
// gateways 2026-11-15), while qwen3.8-flash accepts either. The level map is
// the payload's own, off excluded: adaptive thinking cannot be switched off.
func TestMessagesRouteTakesItsThinkingKnobFromThePayload(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[
			{"id":"claude-sonnet-5-5","limit":{"context":1000000,"output":128000},
			 "api":{"messages":{"base_url":"https://inference.invalid/v1"}},
			 "capabilities":{"reasoning":{"supported":true,"toggle":true,
				"effort":["low","medium","high","xhigh","max"]}}},
			{"id":"qwen-plain","limit":{"context":1000000,"output":131072},
			 "api":{"messages":{"base_url":"https://inference.invalid/v1"}},
			 "capabilities":{"reasoning":{"supported":true}}}]}`
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
	m := reg.models["opencode-go/claude-sonnet-5-5"]
	if m.Protocol != "anthropic-messages" || m.ThinkingMode != "adaptive" {
		t.Fatalf("an effort list on the messages route means the adaptive knob: %+v", m)
	}
	if _, ok := m.ThinkingLevelMap[llm.EffortOff]; ok {
		t.Fatalf("adaptive thinking cannot be disabled, so off has no key: %v", m.ThinkingLevelMap)
	}
	if got := m.ClampEffort(llm.EffortMedium); got != llm.EffortMedium {
		t.Fatalf("the payload's own levels stay reachable: medium clamps to %q", got)
	}
	if m.ClampEffort(llm.EffortOff) != llm.EffortLow {
		t.Fatalf("off must clamp up to the lowest level it can think at, got %q", m.ClampEffort(llm.EffortOff))
	}
	if !m.HistoryBound() {
		t.Fatal("an adaptive route binds the submitted history to its thinking prefix")
	}
	if q := reg.models["opencode-go/qwen-plain"]; q.ThinkingMode != "budget" {
		t.Fatalf("no effort list keeps the budget knob: %+v", q)
	}
}

func TestAnthropicEffortHonoursSupportedFalse(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"claude-x","max_input_tokens":200000,"max_tokens":64000,
			"capabilities":{"effort":{"low":{"supported":true},"max":{"supported":false}},
			"thinking":{"types":{"adaptive":{"supported":true}}}}}],"has_more":false}`
	})
	models, err := fetchAnthropicModels(context.Background(), srv.Client(), srv.URL, http.Header{})
	if err != nil || len(models) != 1 {
		t.Fatalf("%v %+v", err, models)
	}
	if got := strings.Join(models[0].Efforts, ","); got != "low" {
		t.Fatalf("an effort advertised as unsupported must not be offered: %q", got)
	}
}

// TestVerifyCitesAFailedDiscovery: with no cache and a failing list call,
// "the provider does not list it" would be a claim nothing checked.
func TestVerifyCitesAFailedDiscovery(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) { return http.StatusServiceUnavailable, `{}` })
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_DISC_TEST_KEY", "k")
	cfg := mustCfg(t, `{"model":"gw/m","providers":{"gw":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_DISC_TEST_KEY"}}}`)
	reg, _ := NewRegistry(cfg, srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	err := reg.Verify()
	if err == nil || !strings.Contains(err.Error(), "could not be fetched") {
		t.Fatalf("Verify must name the failed discovery: %v", err)
	}
}

// TestVerifyNamesAMissingCredential: discovery skips a provider with no usable
// credential, so its list was never read — the fix is the key, not a
// declaration, and "the provider does not list it" would be unchecked.
func TestVerifyNamesAMissingCredential(t *testing.T) {
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	reg, err := NewRegistry(mustCfg(t, `{"model":"anthropic/claude-nope-9"}`), http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	reg.DiscoverModels(context.Background())
	err = reg.Verify()
	if err == nil || !strings.Contains(err.Error(), "was not read") || strings.Contains(err.Error(), "does not list it") ||
		!strings.Contains(err.Error(), "anthropic/claude-nope-9") {
		t.Fatalf("Verify must name the missing credential: %v", err)
	}
}

// TestVerifyProbesTheCredentialOutsideTheLock pins the invariant that made
// Verify's pre-lock probe necessary: the credential func may re-enter the
// registry. A registered OAuth func here asks for Models() — an RLock — so a
// probe made while Verify held the write lock would block forever. The
// bounded wait turns that into a named failure instead of a hung suite.
func TestVerifyProbesTheCredentialOutsideTheLock(t *testing.T) {
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	reg, err := NewRegistry(mustCfg(t, `{"model":"openai/gpt-nope","providers":{"openai":{"auth":"oauth"}}}`), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reg.SetOAuth("openai", func(context.Context, llm.Request) (Credential, error) {
		_ = reg.Models() // re-enters the registry under RLock
		return Credential{Token: "t"}, nil
	})
	done := make(chan error, 1)
	go func() { done <- reg.Verify() }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "is not in the catalog") {
			t.Fatalf("the missing model is still named: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Verify blocked on its own lock: the credential probe must run before r.mu is taken")
	}
}

// TestRevalidateMissingBypassesAFreshCache: a model published after the list
// was cached must not be refused for a day — the configured model's provider
// is re-read past the fresh cache before Verify.
func TestRevalidateMissingBypassesAFreshCache(t *testing.T) {
	var hits atomic.Int32
	var body atomic.Value
	body.Store(`{"data":[{"id":"m"}]}`)
	srv := jsonServer(t, &hits, func(*http.Request) (int, string) { return 200, body.Load().(string) })
	cfg := discoveryCfg(t, srv.URL)
	reg, _ := NewRegistry(cfg, srv.Client(), nil)
	reg.DiscoverModels(context.Background())
	body.Store(`{"data":[{"id":"m"},{"id":"new"}]}`)
	reg2, _ := NewRegistry(cfg, srv.Client(), nil)
	reg2.DiscoverModels(context.Background())
	if hits.Load() != 1 {
		t.Fatalf("setup: the second registry reads the fresh cache, hits=%d", hits.Load())
	}
	reg2.RevalidateMissing(context.Background(), "gw/m") // known: no fetch
	if hits.Load() != 1 {
		t.Fatalf("a listed model never costs a fetch, hits=%d", hits.Load())
	}
	reg2.RevalidateMissing(context.Background(), "gw/new", "gw/other")
	if hits.Load() != 2 {
		t.Fatalf("one re-read per provider, past the fresh cache, hits=%d", hits.Load())
	}
	if _, ok := reg2.Lookup("gw/new"); !ok {
		t.Fatal("the newer model is listed after the re-read")
	}
}

// TestQuirksDoNotInventModels: a quirk describes a model, it does not prove
// the provider serves one. Only the payload's list may add a row, which is
// what keeps retired ids out of the picker.
func TestQuirksDoNotInventModels(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) { return 200, `{"data":[{"id":"glm-5.3-flash"}]}` })
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("MOCA_ZEN_KEY", "k")
	cfg := mustCfg(t, `{"providers":{"opencode":{"baseUrl":"`+srv.URL+`","apiKey":"env:MOCA_ZEN_KEY"}}}`)
	reg, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reg.DiscoverModels(context.Background())
	for _, m := range reg.Models() {
		if m.Provider == "opencode" && m.ID != "glm-5.3-flash" {
			t.Fatalf("a quirk-only id was added: %s", m.Qualified())
		}
	}
	for _, q := range []string{"opencode/kimi-k3", "opencode/deepseek-v4.1-flash", "opencode/mimo-v2.6-flash"} {
		if _, ok := reg.Lookup(q); ok {
			t.Fatalf("%s: quirked but not listed, must not exist", q)
		}
	}
	if m, ok := reg.Lookup("opencode/glm-5.3-flash"); !ok || len(m.ThinkingLevelMap) == 0 {
		t.Fatalf("the listed id keeps the vendor quirk: %+v", m)
	}
}

// TestFallbackOnlyWithoutAList: the small fallback set stands in for a list
// that was never read (no credential, or a failed first fetch), and a
// successful fetch replaces it — even an empty one, which is an answer.
func TestFallbackOnlyWithoutAList(t *testing.T) {
	blankBuiltinKeys(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	reg, err := NewRegistry(mustCfg(t, `{"providers":{"opencode-go":{"auth":"api_key","apiKey":"env:MOCA_NO_SUCH_KEY"}}}`), http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	// No credential → discovery never reads the list → the fallback rows are
	// all the provider has, and they resolve.
	var n int
	for _, m := range reg.Models() {
		if m.Provider == "opencode-go" {
			n++
			if _, _, err := reg.Resolve(m.Qualified()); err != nil {
				t.Fatalf("a fallback row must resolve: %v", err)
			}
		}
	}
	if n == 0 {
		t.Fatal("the fallback set is empty")
	}

	// A reachable provider that lists nothing: the fallback set must not
	// stand in for an answer (an empty list is a fact).
	srv := jsonServer(t, nil, func(*http.Request) (int, string) { return 200, `{"data":[]}` })
	t.Setenv("MOCA_GO_TEST_KEY", "k")
	cfg := mustCfg(t, `{"providers":{"opencode-go":{"baseUrl":"`+srv.URL+`","apiKey":"env:MOCA_GO_TEST_KEY"}}}`)
	reg2, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reg2.DiscoverModels(context.Background())
	for _, m := range reg2.Models() {
		if m.Provider == "opencode-go" {
			t.Fatalf("an empty list must not fall back: %s", m.Qualified())
		}
	}
}

// TestFetchOpenAICompatibleModelsReadsThePayload: the gateway's full shape
// lands in CachedModel — window, output, prices (cache read/write included),
// effort levels, the advertised wire — and a thin id-only payload keeps the
// old answer: unknown window, unknown price, no efforts.
func TestFetchOpenAICompatibleModelsReadsThePayload(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[
			{"id":"rich","object":"model","limit":{"context":1000000,"output":65536},
			 "cost":{"text":{"input":{"price":2,"cache":{"read":0.1,"write":2.5}},"output":{"price":10}}},
			 "capabilities":{"tools":{"supported":true},
				"reasoning":{"supported":true,"effort":["low","high","max"]}},
			 "api":{"chat-completions":{"base_url":"https://inference.invalid/v1"}}},
			{"id":"thin"},
			{"id":"embed","capabilities":{"tools":{"supported":false}}},
			{"id":"free","cost":{"text":{"input":{"price":0},"output":{"price":0}}}},
			{"id":"partial","cost":{"text":{"input":{"price":1,"cache":{"read":0.1}}}}},
			{"id":"broken","limit":{"context":"1M"},"capabilities":{"tools":{"supported":true}}},
			{"id":"broken-embed","limit":{"context":"1M"},"capabilities":{"tools":{"supported":false}}}]}`
	})
	got, err := fetchOpenAICompatibleModels(context.Background(), srv.Client(), srv.URL, http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]CachedModel{}
	for _, m := range got {
		byID[m.ID] = m
	}
	if _, ok := byID["embed"]; ok {
		t.Fatal("a tools:false model is not a model here")
	}
	if _, ok := byID["broken-embed"]; ok {
		t.Fatal("a tools:false row is dropped even when its metadata does not decode")
	}
	rich := byID["rich"]
	if rich.Ctx != 1000000 || rich.MaxOut != 65536 || rich.Protocol != "openai-completions" {
		t.Fatalf("limits/wire: %+v", rich)
	}
	if !rich.CostKnown || rich.Cost.Input != 2 || rich.Cost.Output != 10 || rich.Cost.CacheRead != 0.1 || rich.Cost.CacheWrite != 2.5 {
		t.Fatalf("cost: %+v", rich.Cost)
	}
	if !rich.Reasons || strings.Join(rich.Efforts, ",") != "low,high,max" {
		t.Fatalf("reasoning: %+v", rich)
	}
	thin := byID["thin"]
	if thin.Ctx != 0 || thin.CostKnown || thin.Reasons || len(thin.Efforts) != 0 {
		t.Fatalf("an id-only payload stays unknown: %+v", thin)
	}
	// A published zero is a price, not a gap.
	if free := byID["free"]; !free.CostKnown || free.Cost.Input != 0 {
		t.Fatalf("a free model is priced at zero: %+v", free)
	}
	// A lone input rate is not a price: moca's unit would claim the output
	// side is free, which the provider never published.
	if partial, ok := byID["partial"]; !ok || partial.CostKnown || partial.Cost != (config.CostConfig{}) {
		t.Fatalf("half a price is not a known price: %+v", partial)
	}
	// A row whose metadata does not decode still names a model: the id
	// survives with the same unknown shape a thin payload gets.
	if broken, ok := byID["broken"]; !ok || broken.Ctx != 0 || broken.CostKnown || broken.Reasons {
		t.Fatalf("an undecodable row keeps its id and nothing else: %+v", broken)
	}
}

// TestDiscoveredBudgetThinking: a budget-thinking Anthropic model publishes no
// effort list, yet it can think at every level and not at all.
func TestDiscoveredBudgetThinking(t *testing.T) {
	m := modelFromCached("anthropic", CachedModel{ID: "claude-old", Protocol: "anthropic-messages", Mode: "budget", Ctx: 200000})
	if m.ThinkingMode != "budget" {
		t.Fatalf("budget thinking survives an empty effort list: %+v", m)
	}
	if _, ok := m.ThinkingLevelMap[llm.EffortOff]; !ok {
		t.Fatal("budget thinking can be turned off")
	}
	if m.ClampEffort(llm.EffortHigh) != llm.EffortHigh {
		t.Fatalf("high effort: %v", m.ClampEffort(llm.EffortHigh))
	}
}

// TestFetchAnthropicModelsPageCap: a pagination that never ends must not spin
// past modelsMaxPages, and the truncation must be logged — an incomplete list
// is never mistaken for the full one.
func TestFetchAnthropicModelsPageCap(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"m-1"}],"has_more":true,"last_id":"m-1"}`
	})
	hdr := http.Header{}
	hdr.Set("x-api-key", "K")
	hdr.Set("anthropic-version", "2023-06-01")

	// The fixture always says has_more: the loop must stop at modelsMaxPages
	// pages, and the truncation must be logged — an incomplete list is never
	// mistaken for the full one.
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	models, err := fetchAnthropicModels(context.Background(), srv.Client(), srv.URL, hdr)
	slog.SetDefault(old)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != modelsMaxPages {
		t.Fatalf("the page cap bounds the fetch: %d models", len(models))
	}
	if !bytes.Contains(buf.Bytes(), []byte("model list truncated")) {
		t.Fatalf("the truncation must be logged, got: %s", buf.String())
	}
}

// TestVerifySuggestsNearestModels: a typo's "did you mean" is bounded to three
// candidates and stays within the typo'd model's own provider.
func TestVerifySuggestsNearestModels(t *testing.T) {
	srv := jsonServer(t, nil, func(*http.Request) (int, string) {
		return 200, `{"data":[{"id":"mod-2"},{"id":"mod-23"},{"id":"mod-24"},{"id":"mod-25"},{"id":"unrelated"}]}`
	})
	cfg := discoveryCfg(t, srv.URL)
	cfg.Model = "gw/mod-2zzz"
	reg, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	reg.DiscoverModels(context.Background())
	err = reg.Verify()
	if err == nil || !strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("a typo'd id must get a hint: %v", err)
	}
	hint := err.Error()
	if i := strings.Index(hint, "did you mean: "); i >= 0 {
		hint = hint[i+len("did you mean: "):]
	}
	if end := strings.Index(hint, " ("); end >= 0 {
		hint = hint[:end]
	}
	cands := strings.Split(hint, ", ")
	if len(cands) != 3 {
		t.Fatalf("the hint is capped at three, got: %q", cands)
	}
	for _, c := range cands {
		if !strings.HasPrefix(c, "gw/") || strings.Contains(c, "unrelated") {
			t.Fatalf("candidates stay in the provider: %q", cands)
		}
	}
}

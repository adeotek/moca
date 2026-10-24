package provider

// Provider model discovery (SPECS §6). Every registered provider publishes its
// model list on its own endpoint; this file fetches those lists, normalizes
// them into CachedModel and leaves the merging to the registry. Discovery is
// best effort and bounded: a provider that fails leaves the previous state
// (cache or catalog) in place and never fails a run.
//
// Endpoints, all verified live on 2026-10-24:
//
//	anthropic      GET {base}/v1/models?limit=1000   paginated (after_id/has_more)
//	openai         GET {base}/models                 id + shutdown_date only
//	opencode-go    GET {base}/models                 public, id only
//	opencode-zen   GET {base}/models                 public, id only
//	openrouter     GET {base}/models                 public, window/pricing/efforts
//	custom         GET {base}/models                 id only
//
// Ollama keeps its native discovery (tags + show) in ollama.go.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

const (
	modelFetchTimeout = 4 * time.Second
	maxModelsBody     = 8 << 20
	modelsMaxPages    = 5
)

// catalogSeeds borrows another provider's catalog rows when the two serve the
// same models under the same vendor prices. opencode-zen is the pay-as-you-go
// twin of the go tier: same account, same published per-token prices, so a
// discovered zen id that the go catalog already describes keeps its window,
// cost and thinking map instead of degrading to an unpriced guess.
var catalogSeeds = map[string]string{"opencode-zen": "opencode-go"}

// fetchSpec describes how to list one provider's models.
type fetchSpec struct {
	fetch func(ctx context.Context, hc *http.Client, base string, hdr http.Header) ([]CachedModel, error)
}

// fetchSpecFor returns the descriptor for a provider. Unknown names are custom
// providers: OpenAI-compatible /models, credential required when one is set.
func fetchSpecFor(name string) fetchSpec {
	switch name {
	case "anthropic":
		return fetchSpec{fetch: fetchAnthropicModels}
	case "openai":
		return fetchSpec{fetch: fetchOpenAIModels}
	case "opencode-go", "opencode-zen":
		return fetchSpec{fetch: fetchOpenAICompatibleModels}
	case "openrouter":
		return fetchSpec{fetch: fetchOpenRouterModels}
	default:
		return fetchSpec{fetch: fetchOpenAICompatibleModels}
	}
}

// defaultProtocolFor is the protocol given to a discovered model that does not
// name one. Anthropic speaks its own wire; every other provider here is an
// OpenAI-compatible gateway, and chat completions is the common denominator on
// api.openai.com (the Responses API does not serve every listed model).
func defaultProtocolFor(providerName string) string {
	if providerName == "anthropic" {
		return "anthropic-messages"
	}
	return "openai-completions"
}

// assumedWindow is the context window given to a discovered model whose
// provider does not publish one (config.MinContextWindow).
const assumedWindow = config.MinContextWindow

// modelFromCached builds a registry row from a discovered model, filling the
// gaps: unknown protocol → the provider's default, unknown window → the
// assumed one (flagged, so the UI can say so), unknown output cap → 8192,
// unknown thinking → none. It never guesses a price: CostKnown stays false
// unless the payload carried one.
func modelFromCached(pname string, cm CachedModel) Model {
	m := Model{Provider: pname, ID: cm.ID, Protocol: cm.Protocol, ContextWindow: cm.Ctx,
		MaxOutput: cm.MaxOut, Cost: cm.Cost, CostKnown: cm.CostKnown, ThinkingMode: cm.Mode}
	if m.Protocol == "" {
		m.Protocol = defaultProtocolFor(pname)
	}
	if m.ContextWindow <= 0 {
		m.ContextWindow, m.WindowAssumed = assumedWindow, true
	}
	if m.MaxOutput <= 0 {
		m.MaxOutput = 8192
	}
	if m.ThinkingMode == "" {
		m.ThinkingMode = "none"
	}
	if levels := effortMap(cm.Efforts, cm.Mandatory); len(levels) > 0 && m.ThinkingMode != "none" {
		m.ThinkingLevelMap = levels
	}
	if m.ThinkingMode != "none" && len(m.ThinkingLevelMap) == 0 {
		m.ThinkingMode = "none"
	}
	return m
}

// effortMap maps the provider's advertised effort values onto moca's levels.
// A provider that marks reasoning mandatory never gets an off key: sending
// "none"/"off" to such a model is the [1210] class of 400.
func effortMap(efforts []string, mandatory bool) map[llm.Effort]string {
	out := map[llm.Effort]string{}
	for _, e := range efforts {
		switch e {
		case "none":
			out[llm.EffortOff] = "none"
		case "minimal", "low", "medium", "high", "xhigh", "max":
			out[llm.Effort(e)] = e
		}
	}
	if mandatory {
		delete(out, llm.EffortOff)
	}
	return out
}

// DiscoveryReport is what one DiscoverModels run found.
type DiscoveryReport struct {
	Providers []ProviderModels
}

// ProviderModels is one provider's share of a discovery run.
type ProviderModels struct {
	Provider  string
	Base      string
	Models    []string
	Assumed   []string // ids whose window is a guess
	FromCache bool     // served from a fresh cache entry, no network call
	Err       error    // the fetch failed; rows may still come from a stale cache
}

// Provider returns the named provider's report.
func (d DiscoveryReport) Provider(name string) (ProviderModels, bool) {
	for _, p := range d.Providers {
		if p.Provider == name {
			return p, true
		}
	}
	return ProviderModels{}, false
}

// DiscoverModels loads every configured provider's model list: from the cache
// when it is fresh, else from the provider (keeping stale rows as the fallback
// while the fetch runs). Ollama is skipped — DiscoverOllama owns it. Providers
// are fetched in parallel; the whole run is bounded by modelFetchTimeout.
func (r *Registry) DiscoverModels(ctx context.Context) DiscoveryReport {
	// Snapshot the names: r.cfg is immutable after construction.
	var names []string
	for name := range r.cfg.Providers {
		if name == "ollama" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	res := make([]ProviderModels, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res[i] = r.discoverOne(ctx, name)
		}()
	}
	wg.Wait()
	return DiscoveryReport{Providers: res}
}

// discoverOne runs one provider's cache-then-fetch cycle.
func (r *Registry) discoverOne(ctx context.Context, name string) ProviderModels {
	spec := fetchSpecFor(name)
	proto := defaultProtocolFor(name)
	if p := r.cfg.Providers[name]; p.Protocol != "" {
		proto = p.Protocol
	}
	base := r.baseURL(name, proto)
	out := ProviderModels{Provider: name, Base: base}
	if base == "" {
		return out
	}
	if cached, fresh := r.cache.Load(name, base); len(cached) > 0 {
		r.mergeDiscovered(name, cached)
		out.FromCache = fresh
		out.Models, out.Assumed = r.discoveredIDs(name, cached)
		if fresh {
			return out
		}
	}
	// Credential gate: an unauthenticated provider is neither listed nor
	// fetched (its cache, if any, is all we can show). This also keeps unit
	// tests offline: with no key in the environment, no call is made.
	if !r.ProviderReady(name) {
		return out
	}
	// Custom providers: a declared model list IS the contract. moca cannot
	// assume an arbitrary gateway publishes /models (or publishes it in the
	// OpenAI shape), so discovery only fills in for a custom provider with no
	// declarations at all — a vLLM that lists its own models.
	if _, builtin := config.BuiltinProviders[name]; !builtin && !config.IsLocalProvider(name) &&
		len(r.cfg.Providers[name].Models) > 0 {
		return out
	}
	fctx, cancel := context.WithTimeout(ctx, modelFetchTimeout)
	defer cancel()
	got, err := spec.fetch(fctx, r.hc, base, r.discoveryHeaders(name))
	if err != nil {
		slog.Debug("model discovery failed", "provider", name, "url", base, "class", causeClass(err))
		out.Err = err
		return out
	}
	slog.Debug("model discovery", "provider", name, "models", len(got), "cached", false)
	r.mergeDiscovered(name, got)
	if err := r.cache.Put(name, base, got); err != nil {
		// The rows are in memory; a read-only data dir must not cost a run.
		slog.Debug("models cache write failed", "provider", name)
	}
	out.Models, out.Assumed = r.discoveredIDs(name, got)
	return out
}

// discoveryHeaders builds the auth (and attribution) headers a list call
// needs. A provider with no usable credential simply gets none: public
// endpoints answer anyway and the others return an error that the caller
// treats as a failed fetch.
func (r *Registry) discoveryHeaders(name string) http.Header {
	hdr := http.Header{}
	hdr.Set("User-Agent", "moca/"+config.Version)
	switch name {
	case "openrouter":
		hdr.Set("X-OpenRouter-Title", "moca")
		hdr.Set("HTTP-Referer", "https://github.com/adeotek/moca")
	}
	cred, err := r.credential(name)(context.Background(), llm.Request{})
	if err != nil {
		return hdr
	}
	if cred.Token != "" {
		if name == "anthropic" {
			hdr.Set("x-api-key", cred.Token)
			hdr.Set("anthropic-version", "2023-06-01")
		}
		hdr.Set("Authorization", "Bearer "+cred.Token)
	}
	for k, v := range cred.Headers {
		hdr.Set(k, v)
	}
	return hdr
}

// discoveredIDs lists a payload's ids and the ones whose window is a guess. A
// window declared under providers.<name>.models is a decision, not a guess, so
// it never shows up as assumed (the ollama rule, generalized).
func (r *Registry) discoveredIDs(name string, models []CachedModel) (ids, assumed []string) {
	declared := r.cfg.Providers[name].Models
	for _, m := range models {
		ids = append(ids, m.ID)
		if m.Ctx > 0 {
			continue
		}
		if o, ok := declared[m.ID]; ok && o.ContextWindow != 0 {
			continue
		}
		assumed = append(assumed, m.ID)
	}
	slices.Sort(ids)
	slices.Sort(assumed)
	return ids, assumed
}

// mergeDiscovered records a provider's discovered rows and rebuilds its slice
// of the registry under the lock.
func (r *Registry) mergeDiscovered(name string, models []CachedModel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.discovered[name] = models
	r.rebuildProviderLocked(name)
}

// addSeedsLocked clones the catalog rows of a sibling provider that serves the
// same models at the same prices (catalogSeeds). The provider name is
// rewritten, so the row is that provider's own.
func (r *Registry) addSeedsLocked(name string) {
	src, ok := catalogSeeds[name]
	if !ok {
		return
	}
	for _, m := range builtinCatalog {
		if m.Provider != src || m.ID == "" {
			continue
		}
		m.Provider, m.CostKnown = name, true
		r.models[m.Qualified()] = m
	}
}

// rebuildProviderLocked recomposes one provider's rows in precedence order:
// defaults (from the discovered row) ← catalog ← seeds ← declared config
// overrides. Walking from lowest to highest priority means a curated row's
// metadata is never replaced by a thinner provider payload.
func (r *Registry) rebuildProviderLocked(name string) {
	for q, m := range r.models {
		if m.Provider == name {
			delete(r.models, q)
		}
	}
	for _, cm := range r.discovered[name] {
		m := modelFromCached(name, cm)
		r.models[m.Qualified()] = m
	}
	r.addSeedsLocked(name)
	for _, m := range builtinCatalog {
		if m.Provider == name {
			m.CostKnown = true
			r.models[m.Qualified()] = m
		}
	}
	// Declared models and overrides win over everything discovered.
	_ = r.applyModelOverrides(name, r.cfg.Providers[name])
}

// ClearModelsCache drops the on-disk cache and every discovered row, leaving
// the catalog and declared models in place. The next discovery (or /model
// open) refetches.
func (r *Registry) ClearModelsCache() (providers, models int, err error) {
	providers, models, err = r.cache.Clear()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.discovered = map[string][]CachedModel{}
	for name := range r.cfg.Providers {
		if name == "ollama" {
			continue
		}
		r.rebuildProviderLocked(name)
	}
	return providers, models, err
}

// ProviderReady reports whether a request to this provider could be
// authenticated — the same predicate the request path applies (stored key,
// resolvable env reference, or a local provider), with no network call.
func (r *Registry) ProviderReady(name string) bool {
	if _, ok := r.cfg.Providers[name]; !ok {
		return false
	}
	_, err := r.credential(name)(context.Background(), llm.Request{})
	return err == nil
}

// ActiveModels is the catalog for a picker: rows whose provider can actually
// run. A per-provider memo keeps it to one credential lookup per provider
// rather than one per model.
func (r *Registry) ActiveModels() []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ready := make(map[string]bool, len(r.cfg.Providers))
	out := make([]Model, 0, len(r.models))
	for _, m := range r.models {
		ok, seen := ready[m.Provider]
		if !seen {
			ok = r.providerReadyLocked(m.Provider)
			ready[m.Provider] = ok
		}
		if ok {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b Model) int { return strings.Compare(a.Qualified(), b.Qualified()) })
	return out
}

// providerReadyLocked is ProviderReady without the nil-map guard; the caller
// holds mu (the credential func reads the config, which is immutable).
func (r *Registry) providerReadyLocked(name string) bool {
	if _, ok := r.cfg.Providers[name]; !ok {
		return false
	}
	_, err := r.credential(name)(context.Background(), llm.Request{})
	return err == nil
}

// fetchAnthropicModels reads /v1/models, following the after_id cursor. The
// payload is the richest of the non-OpenRouter set: it carries the real
// window, the output cap, the supported effort levels and the thinking kinds.
func fetchAnthropicModels(ctx context.Context, hc *http.Client, base string, hdr http.Header) ([]CachedModel, error) {
	type modelInfo struct {
		ID         string `json:"id"`
		Lifecycle  string `json:"lifecycle"`
		MaxInput   int    `json:"max_input_tokens"`
		MaxTokens  int    `json:"max_tokens"`
		Capability struct {
			Effort   map[string]json.RawMessage `json:"effort"`
			Thinking struct {
				Types map[string]struct {
					Supported bool `json:"supported"`
				} `json:"types"`
			} `json:"thinking"`
		} `json:"capabilities"`
	}
	type page struct {
		Data    []modelInfo `json:"data"`
		HasMore bool        `json:"has_more"`
		LastID  string      `json:"last_id"`
	}
	var out []CachedModel
	url := strings.TrimRight(base, "/") + "/v1/models?limit=1000"
	for range modelsMaxPages {
		var pg page
		if err := getJSON(ctx, hc, url, hdr, &pg); err != nil {
			return out, err
		}
		for _, m := range pg.Data {
			if m.ID == "" {
				continue
			}
			if m.Lifecycle != "" && m.Lifecycle != "active" {
				continue
			}
			cm := CachedModel{ID: m.ID, Protocol: "anthropic-messages", Ctx: m.MaxInput, MaxOut: m.MaxTokens}
			var levels []string
			for _, lvl := range []string{"minimal", "low", "medium", "high", "xhigh", "max"} {
				if _, ok := m.Capability.Effort[lvl]; ok {
					levels = append(levels, lvl)
				}
			}
			cm.Efforts = levels
			adaptive := m.Capability.Thinking.Types["adaptive"].Supported
			enabled := m.Capability.Thinking.Types["enabled"].Supported
			switch {
			case adaptive:
				cm.Mode = "adaptive"
			case enabled:
				cm.Mode = "budget"
			default:
				cm.Mode = "none"
			}
			out = append(out, cm)
		}
		if !pg.HasMore || pg.LastID == "" {
			break
		}
		url = strings.TrimRight(base, "/") + "/v1/models?limit=1000&after_id=" + pg.LastID
	}
	return out, nil
}

// openaiDenyList drops the non-chat models api.openai.com lists beside the
// chat ones (embeddings, speech, image, moderation). It is a substring list
// because the payload carries no capability signal; a new model family whose
// id matches one of these is the cost of having any filter at all.
var openaiDenyList = []string{"embedding", "tts", "whisper", "dall-e", "moderation", "image", "audio", "realtime", "transcribe"}

// fetchOpenAIModels reads /models: ids, minus non-chat models and models the
// vendor has announced a shutdown for. The payload carries no window or price.
func fetchOpenAIModels(ctx context.Context, hc *http.Client, base string, hdr http.Header) ([]CachedModel, error) {
	var list struct {
		Data []struct {
			ID           string  `json:"id"`
			ShutdownDate *string `json:"shutdown_date"`
		} `json:"data"`
	}
	if err := getJSON(ctx, hc, strings.TrimRight(base, "/")+"/models", hdr, &list); err != nil {
		return nil, err
	}
	var out []CachedModel
	for _, m := range list.Data {
		if m.ID == "" || m.ShutdownDate != nil {
			continue
		}
		if slices.ContainsFunc(openaiDenyList, func(s string) bool { return strings.Contains(strings.ToLower(m.ID), s) }) {
			continue
		}
		out = append(out, CachedModel{ID: m.ID, Protocol: "openai-completions"})
	}
	return out, nil
}

// fetchOpenAICompatibleModels reads the OpenAI-shaped /models every compatible
// gateway exposes: ids only (opencode-go, opencode-zen, custom providers).
func fetchOpenAICompatibleModels(ctx context.Context, hc *http.Client, base string, hdr http.Header) ([]CachedModel, error) {
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := getJSON(ctx, hc, strings.TrimRight(base, "/")+"/models", hdr, &list); err != nil {
		return nil, err
	}
	var out []CachedModel
	for _, m := range list.Data {
		if m.ID == "" {
			continue
		}
		out = append(out, CachedModel{ID: m.ID, Protocol: "openai-completions"})
	}
	return out, nil
}

// fetchOpenRouterModels reads the public catalog: window, output cap, per-token
// pricing, the advertised effort levels and whether reasoning is mandatory.
// Image/audio/embedding models are dropped (no `tools` parameter), as are the
// :free/:batch variants and the router ids (no fixed window, no fixed price).
func fetchOpenRouterModels(ctx context.Context, hc *http.Client, base string, hdr http.Header) ([]CachedModel, error) {
	var list struct {
		Data []struct {
			ID         string `json:"id"`
			ContextLen int    `json:"context_length"`
			// pricing values are per-token decimal STRINGS, but the object
			// also carries non-string entries (e.g. `overrides`, an array for
			// long-context tiers): decode raw and read only the four rates.
			Pricing     map[string]json.RawMessage `json:"pricing"`
			Supported   []string                   `json:"supported_parameters"`
			TopProvider struct {
				MaxCompletion int `json:"max_completion_tokens"`
			} `json:"top_provider"`
			Reasoning *struct {
				SupportedEfforts []string `json:"supported_efforts"`
				Mandatory        bool     `json:"mandatory"`
			} `json:"reasoning"`
		} `json:"data"`
	}
	if err := getJSON(ctx, hc, strings.TrimRight(base, "/")+"/models", hdr, &list); err != nil {
		return nil, err
	}
	var out []CachedModel
	for _, m := range list.Data {
		if m.ID == "" || strings.Contains(m.ID, ":") || strings.HasPrefix(m.ID, "openrouter/") {
			continue
		}
		if !slices.Contains(m.Supported, "tools") {
			continue
		}
		cm := CachedModel{ID: m.ID, Protocol: "openai-completions", Ctx: m.ContextLen, MaxOut: m.TopProvider.MaxCompletion}
		in := perMillion(rawString(m.Pricing["prompt"]))
		o := perMillion(rawString(m.Pricing["completion"]))
		if in > 0 || o > 0 {
			cm.Cost = config.CostConfig{Input: in, Output: o,
				CacheRead: perMillion(rawString(m.Pricing["input_cache_read"])), CacheWrite: perMillion(rawString(m.Pricing["input_cache_write"]))}
			cm.CostKnown = true
		}
		cm.Mode = "none"
		if m.Reasoning != nil {
			cm.Mode = "effort-object"
			cm.Efforts = m.Reasoning.SupportedEfforts
			cm.Mandatory = m.Reasoning.Mandatory
		}
		out = append(out, cm)
	}
	return out, nil
}

// rawString reads a JSON string value, or "" for anything else (null, a
// number, an object: OpenRouter mixes shapes inside `pricing`).
func rawString(r json.RawMessage) string {
	var s string
	if len(r) > 0 && json.Unmarshal(r, &s) == nil {
		return s
	}
	return ""
}

// perMillion converts OpenRouter's per-token price string to dollars per
// million tokens (moca's CostConfig unit).
func perMillion(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return f * 1e6
}

// getJSON sends one GET and decodes the bounded JSON answer.
func getJSON(ctx context.Context, hc *http.Client, url string, hdr http.Header, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header = hdr.Clone()
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return newHTTPError(resp)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxModelsBody)).Decode(out)
}

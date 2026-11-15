package provider

// Provider model discovery (SPECS §6). Every registered provider publishes its
// model list on its own endpoint; this file fetches those lists, normalizes
// them into CachedModel and leaves the merging to the registry. Discovery is
// best effort and bounded: a provider that fails leaves the previous state
// (cache or fallback) in place and never fails a run.
//
// Endpoints, all verified live on 2026-11-15:
//
//	anthropic      GET {base}/v1/models?limit=1000   paginated (after_id/has_more)
//	openai         GET {base}/models                 id + shutdown_date only
//	opencode-go    GET {base}/models                 window, output, price, efforts, api
//	opencode       GET {base}/models                 window, output, price, efforts, api
//	openrouter     GET {base}/models                 window/pricing/efforts
//	custom         GET {base}/models                 provider-shaped, ids at least
//
// Ollama keeps its native discovery (tags + show) in ollama.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
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

// modelsFetcher lists one provider's models.
type modelsFetcher func(ctx context.Context, hc *http.Client, base string, hdr http.Header) ([]CachedModel, error)

// fetcherFor returns the lister for a provider. Every name without a
// dedicated payload reader (opencode-go, opencode, custom providers) speaks
// the OpenAI-compatible /models shape.
func fetcherFor(name string) modelsFetcher {
	switch name {
	case "anthropic":
		return fetchAnthropicModels
	case "openai":
		return fetchOpenAIModels
	case "openrouter":
		return fetchOpenRouterModels
	default:
		return fetchOpenAICompatibleModels
	}
}

// defaultProtocolFor is the protocol given to a discovered model that does not
// name one. Anthropic speaks its own wire. api.openai.com rows ride the
// Responses API like the curated openai rows: it serves every chat model,
// while chat completions rejects `max_tokens` on the reasoning models and
// does not serve the Responses-only ones at all — and a subscription (OAuth)
// login can use nothing else. Every other provider here is an
// OpenAI-compatible gateway on chat completions.
func defaultProtocolFor(providerName string) string {
	switch providerName {
	case "anthropic":
		return "anthropic-messages"
	case "openai":
		return "openai-responses"
	}
	return "openai-completions"
}

// protocolFromAPI reads a gateway's advertised API surfaces and picks the wire
// moca speaks for them, most-preferred first. The block describes what the
// vendor's own SDKs use; the gateway usually serves several, so the order
// matters more than the set. A payload without one (api.openai.com, most
// compatible gateways) answers "" and the provider default applies.
func protocolFromAPI(api map[string]json.RawMessage) string {
	switch {
	case api["chat-completions"] != nil:
		return "openai-completions"
	case api["responses"] != nil:
		return "openai-responses"
	case api["messages"] != nil:
		return "anthropic-messages"
	}
	return ""
}

// assumedWindow is the context window given to a discovered model whose
// provider does not publish one (config.MinContextWindow).
const assumedWindow = config.MinContextWindow

// modelFromCached builds a registry row from a discovered model's own facts,
// filling the gaps: unknown protocol → the provider's default, unknown window
// → the assumed one (flagged, so the UI can say so), unknown output cap →
// 8192, unknown thinking → none. It never guesses a price: CostKnown stays
// false unless the payload carried one.
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
	switch {
	case m.ThinkingMode == "budget":
		// budget_tokens comes from MaxTokens and the effort, never from an
		// advertised effort list (the budget-only models publish none), and
		// a budget model can always think less or not at all: the curated
		// budget rows' level set, off included.
		m.ThinkingLevelMap = anthropicBudget
	case m.ThinkingMode == "adaptive":
		// The effort levels an adaptive model accepts are its own; the
		// curated set is the fallback for a payload that names none.
		if levels := effortMap(cm.Efforts, false); len(levels) > 0 {
			m.ThinkingLevelMap = levels
		} else {
			m.ThinkingLevelMap = anthropicAdaptive
		}
	case len(cm.Efforts) > 0:
		if m.ThinkingMode == "" {
			m.ThinkingMode = "openai"
		}
		m.ThinkingLevelMap = effortMap(cm.Efforts, cm.Mandatory)
	case m.ThinkingMode == "":
		m.ThinkingMode = "none"
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
			res[i] = r.discoverOne(ctx, name, false)
		}()
	}
	wg.Wait()
	return DiscoveryReport{Providers: res}
}

// Refresh re-reads every provider's model list and a configured Ollama
// server's, concurrently: both share ctx's deadline, and a slow Ollama server
// must not eat the cloud providers' share of it (they touch disjoint rows —
// DiscoverModels skips ollama). It returns Ollama's failure (an unreachable
// server is the one hard failure) and nil otherwise: a cloud provider that
// fails keeps its cache.
func (r *Registry) Refresh(ctx context.Context) error {
	errc := make(chan error, 1)
	go func() {
		_, err := r.DiscoverOllama(ctx)
		errc <- err
	}()
	r.DiscoverModels(ctx)
	return <-errc
}

// RevalidateMissing re-reads, past a fresh cache, the list of each named
// model's provider when the registry does not know that model. A model the
// provider published after its list was cached would otherwise be refused by
// Verify — "the provider does not list it" — for up to ModelsCacheTTL, with
// no way to refresh from a session that cannot start.
func (r *Registry) RevalidateMissing(ctx context.Context, qualified ...string) {
	seen := map[string]bool{}
	for _, q := range qualified {
		pn, _, err := config.SplitModel(q)
		if err != nil || seen[pn] || !r.listedByDiscovery(pn) {
			continue
		}
		if _, ok := r.Lookup(q); ok {
			continue
		}
		seen[pn] = true
		r.discoverOne(ctx, pn, true)
	}
}

// discoverOne runs one provider's cache-then-fetch cycle; force skips the
// fresh-cache shortcut (the cached rows still serve if the fetch fails).
func (r *Registry) discoverOne(ctx context.Context, name string, force bool) ProviderModels {
	fetch := fetcherFor(name)
	proto := defaultProtocolFor(name)
	if p := r.cfg.Providers[name]; p.Protocol != "" {
		proto = p.Protocol
	}
	base := r.baseURL(name, proto)
	out := ProviderModels{Provider: name, Base: base}
	if base == "" {
		return out
	}
	// Custom providers: a declared model list IS the contract. moca cannot
	// assume an arbitrary gateway publishes /models (or publishes it in the
	// OpenAI shape), so discovery only fills in for a custom provider with no
	// declarations at all — a vLLM that lists its own models. Checked before
	// the cache, so rows cached before the declarations do not linger.
	if !r.listedByDiscovery(name) {
		return out
	}
	if cached, fresh := r.cache.Load(name, base); len(cached) > 0 {
		r.mergeDiscovered(name, cached)
		out.FromCache = fresh
		out.Models, out.Assumed = discoveredIDs(cached), r.assumedModels(name)
		if fresh && !force {
			r.setDiscoveryErr(name, nil)
			return out
		}
	}
	// Credential gate: an unauthenticated provider is neither listed nor
	// fetched (its cache, if any, is all we can show). This also keeps unit
	// tests offline: with no key in the environment, no call is made.
	if !r.ProviderReady(name) {
		return out
	}
	fctx, cancel := context.WithTimeout(ctx, modelFetchTimeout)
	defer cancel()
	got, err := fetch(fctx, r.hc, base, r.discoveryHeaders(fctx, name))
	if err != nil {
		slog.Debug("model discovery failed", "provider", name, "url", base, "class", causeClass(err))
		out.Err = err
		r.setDiscoveryErr(name, err)
		return out
	}
	slog.Debug("model discovery", "provider", name, "models", len(got), "cached", false)
	r.setDiscoveryErr(name, nil)
	r.markListRead(name)
	r.mergeDiscovered(name, got)
	if err := r.cache.Put(name, base, got); err != nil {
		// The rows are in memory; a read-only data dir must not cost a run.
		slog.Debug("models cache write failed", "provider", name)
	}
	out.Models, out.Assumed = discoveredIDs(got), r.assumedModels(name)
	return out
}

// discoveryHeaders builds the auth (and attribution) headers a list call
// needs. A provider with no usable credential simply gets none: public
// endpoints answer anyway and the others return an error that the caller
// treats as a failed fetch. The provider extras (OpenRouter attribution, the
// go tier's session header) arrive in cred.Headers from credential(). ctx is
// the fetch's bounded context: an OAuth credential may refresh here.
func (r *Registry) discoveryHeaders(ctx context.Context, name string) http.Header {
	hdr := http.Header{}
	hdr.Set("User-Agent", "moca/"+config.Version)
	cred, err := r.credential(name)(ctx, llm.Request{})
	if err != nil {
		return hdr
	}
	if cred.Token != "" {
		// The adapter's rule (anthropic.go): an API key travels as
		// x-api-key, and Bearer is the OAuth signal — never both.
		if name == "anthropic" {
			hdr.Set("anthropic-version", "2023-06-01")
			if !cred.OAuth {
				hdr.Set("x-api-key", cred.Token)
			} else {
				hdr.Set("Authorization", "Bearer "+cred.Token)
			}
		} else {
			hdr.Set("Authorization", "Bearer "+cred.Token)
		}
	}
	for k, v := range cred.Headers {
		hdr.Set(k, v)
	}
	return hdr
}

// markListRead records that a provider's list was read successfully — an
// empty answer from a reachable provider must not fall back to the table.
func (r *Registry) markListRead(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listRead[name] = true
}

// setDiscoveryErr records why a provider's last list fetch failed (nil on
// success); Verify cites it for a configured model the catalog lacks.
func (r *Registry) setDiscoveryErr(name string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		delete(r.discoveryErr, name)
		return
	}
	r.discoveryErr[name] = err
}

// discoveredIDs lists a payload's ids.
func discoveredIDs(models []CachedModel) []string {
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	slices.Sort(ids)
	return ids
}

// assumedModels reports the provider's rows whose window is a guess. It reads
// the MERGED rows, not the payload: a provider that publishes no window still
// has correct windows for every curated or declared model, and warning about
// those was the bug this replaced (the ollama rule — "a declared window is a
// decision, not a guess" — applied to the final row instead of the payload).
func (r *Registry) assumedModels(name string) []string {
	var out []string
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		if m.Provider == name && m.WindowAssumed {
			out = append(out, m.ID)
		}
	}
	slices.Sort(out)
	return out
}

// mergeDiscovered records a provider's discovered rows and rebuilds its slice
// of the registry under the lock.
func (r *Registry) mergeDiscovered(name string, models []CachedModel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.discovered[name] = models
	if err := r.rebuildProviderLocked(name); err != nil {
		slog.Warn("provider rows rebuilt with a rejected override", "provider", name, "err", err)
	}
}

// rowFor composes one registry row from a discovered model: the provider's own
// facts, then the vendor quirk, then the wire this provider can actually
// serve. The order matters — a quirk may name a protocol the provider has no
// base URL for (a messages-only id on zen), and that falls back to the
// provider's default rather than resolving to a dead route.
func (r *Registry) rowFor(name string, cm CachedModel) Model {
	m := modelFromCached(name, cm)
	if q, ok := quirkFor(name, cm.ID); ok {
		m = applyQuirk(m, q)
	}
	// A provider-level protocol is the user's decision for every model the
	// provider serves (a custom gateway on openai-responses or
	// anthropic-messages): it wins over the fetcher's wire choice.
	if p := r.cfg.Providers[name].Protocol; p != "" {
		m.Protocol = p
	}
	if r.baseURL(name, m.Protocol) == "" {
		m.Protocol = defaultProtocolFor(name)
	}
	// A messages route has no reasoning_effort: the anthropic thinking knob is
	// its own, and which one the route takes is what the payload's effort list
	// says. A model that publishes effort levels takes the adaptive knob
	// (thinking.adaptive + output_config.effort) — verified live on both
	// gateways 2026-11-15: claude-haiku-5-5 and claude-sonnet-5-5 answer
	// `400 "thinking.type.enabled" is not supported for this model ... use
	// "thinking.type.adaptive"`, while qwen3.8-flash accepts either. A model
	// that publishes none keeps the budget knob (thinking.enabled +
	// budget_tokens), which minimax-m3 accepts and which is the only one an
	// empty effort list speaks to.
	if m.Protocol == "anthropic-messages" && m.ThinkingMode != "budget" && m.ThinkingMode != "adaptive" {
		// Adaptive cannot be switched off, so the off key a chat-completions
		// effort list may carry is dropped with it (effortMap's mandatory
		// flag does exactly that).
		if levels := effortMap(cm.Efforts, true); len(levels) > 0 {
			m.ThinkingMode, m.ThinkingLevelMap = "adaptive", levels
		} else if m.ThinkingMode == "openai" || (m.ThinkingMode == "none" && cm.Reasons) {
			m.ThinkingMode, m.ThinkingLevelMap = "budget", anthropicBudget
		}
	}
	if m.ThinkingMode != "none" && len(m.ThinkingLevelMap) == 0 {
		m.ThinkingMode = "none"
	}
	return m
}

// rebuildProviderLocked recomposes one provider's rows: the discovered list
// (each row through rowFor), the fallback rows only when no list was ever
// read, then the config's declared models and overrides. The provider's own
// payload is the model list; nothing else may add a model to it.
func (r *Registry) rebuildProviderLocked(name string) error {
	for q, m := range r.models {
		if m.Provider == name {
			delete(r.models, q)
		}
	}
	for _, cm := range r.discovered[name] {
		m := r.rowFor(name, cm)
		r.models[m.Qualified()] = m
	}
	if len(r.discovered[name]) == 0 && !r.listRead[name] {
		// No payload and no cache: a fresh install with no credential, or a
		// first fetch that failed. Keep the provider usable with the small
		// fallback set — never a stand-in for a list that was read.
		for _, m := range fallbackCatalog {
			if m.Provider == name {
				r.models[m.Qualified()] = m
			}
		}
	}
	// Declared models and overrides win over everything discovered. The
	// override is re-applied on every rebuild, and it cannot be rejected
	// here: the config is immutable and NewRegistry validated this same map
	// before it returned, so a failure is a bug in that ordering rather than
	// a user error.
	if err := r.applyModelOverrides(name, r.cfg.Providers[name]); err != nil {
		slog.Warn("declared model override not applied", "provider", name, "err", err)
		return err
	}
	return nil
}

// ClearReport is what ClearModelsCache did.
type ClearReport struct {
	Providers, Models int            // the cache files deleted, and the models they held
	Rescan            ProviderModels // the provider re-read at once (zero when none was named)
}

// ClearModelsCache deletes every provider's cache file, then re-reads rescan
// — the active model's provider — at once, so its list and its file are fresh
// before the next /model. No row leaves the registry: a session keeps the
// models it is using (a failed re-read included), and every other provider's
// list is replaced by its next discovery, which the deleted file can no longer
// serve. Ollama has no cache file but is re-read all the same when named.
func (r *Registry) ClearModelsCache(ctx context.Context, rescan string) (ClearReport, error) {
	var rep ClearReport
	var err error
	if rep.Providers, rep.Models, err = r.cache.Clear(); err != nil {
		return rep, err
	}
	if _, ok := r.cfg.Providers[rescan]; !ok {
		return rep, nil
	}
	if rescan == "ollama" {
		d, err := r.DiscoverOllama(ctx)
		rep.Rescan = ProviderModels{Provider: rescan, Base: d.Base, Models: d.Models, Assumed: d.Assumed, Err: err}
		return rep, nil
	}
	rep.Rescan = r.discoverOne(ctx, rescan, false)
	return rep, nil
}

// ProviderReady reports whether a request to this provider could be
// authenticated — CheckCredential's predicate (stored key, resolvable env
// reference, a stored subscription login, or a local provider), with no
// network call and no refresh: the TUI asks it from its update goroutine.
func (r *Registry) ProviderReady(name string) bool {
	return r.ProviderCredential(name) == nil
}

// ProviderCredential is ProviderReady's predicate with its reason: nil, or
// the error that names the fix (no key stored, not logged in).
func (r *Registry) ProviderCredential(name string) error {
	if _, ok := r.cfg.Providers[name]; !ok {
		return fmt.Errorf("unknown provider %q", name)
	}
	return r.credentialReady(name)
}

// ActiveModels is the catalog for a picker: rows whose provider can actually
// run. A per-provider memo keeps it to one credential lookup per provider
// rather than one per model, and the lookups (a store read each) run after
// the lock is released.
func (r *Registry) ActiveModels() []Model {
	all := r.Models()
	ready := make(map[string]bool, len(r.cfg.Providers))
	out := all[:0]
	for _, m := range all {
		ok, seen := ready[m.Provider]
		if !seen {
			ok = r.ProviderReady(m.Provider)
			ready[m.Provider] = ok
		}
		if ok {
			out = append(out, m)
		}
	}
	return out
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
	var pg page
	url := strings.TrimRight(base, "/") + "/v1/models?limit=1000"
	for range modelsMaxPages {
		pg = page{}
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
				if raw, ok := m.Capability.Effort[lvl]; ok && capabilitySupported(raw) {
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
	// All pages spent but the vendor says there are more: say so, so an
	// incomplete list is never mistaken for the full one (a model
	// mis-claimed as "the provider does not list it" beyond the cap).
	if pg.HasMore && pg.LastID != "" {
		slog.Warn("anthropic model list truncated", "provider", "anthropic", "pages", modelsMaxPages)
	}
	return out, nil
}

// capabilitySupported reads one capability entry: `{"supported": false}` is
// an advertised NO (sending that level is a 400), so only an explicit false
// excludes it; an entry with no flag counts by its presence.
func capabilitySupported(raw json.RawMessage) bool {
	var v struct {
		Supported *bool `json:"supported"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Supported == nil {
		return true
	}
	return *v.Supported
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
		out = append(out, CachedModel{ID: m.ID, Protocol: defaultProtocolFor("openai")})
	}
	return out, nil
}

// fetchOpenAICompatibleModels reads the OpenAI-shaped /models every compatible
// gateway exposes. The shape is the superset OpenCode's gateways publish —
// window, output cap, per-million prices (cache reads and writes included),
// the advertised effort levels and the API surfaces — and every field is
// optional, so an id-only gateway keeps working exactly as before: unknown
// window, unknown price, no effort levels.
func fetchOpenAICompatibleModels(ctx context.Context, hc *http.Client, base string, hdr http.Header) ([]CachedModel, error) {
	type limit struct {
		Context int `json:"context"`
		Input   int `json:"input"`
		Output  int `json:"output"`
	}
	type price struct {
		Price json.RawMessage `json:"price"`
		Cache struct {
			Read  json.RawMessage `json:"read"`
			Write json.RawMessage `json:"write"`
		} `json:"cache"`
	}
	type cost struct {
		Text *struct {
			Input  *price `json:"input"`
			Output *price `json:"output"`
		} `json:"text"`
	}
	type model struct {
		ID           string `json:"id"`
		Limit        limit  `json:"limit"`
		Cost         *cost  `json:"cost"`
		Capabilities *struct {
			Tools *struct {
				Supported *bool `json:"supported"`
			} `json:"tools"`
			Reasoning *struct {
				Supported *bool    `json:"supported"`
				Effort    []string `json:"effort"`
			} `json:"reasoning"`
		} `json:"capabilities"`
		API map[string]json.RawMessage `json:"api"`
	}
	var list struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := getJSON(ctx, hc, strings.TrimRight(base, "/")+"/models", hdr, &list); err != nil {
		return nil, err
	}
	var out []CachedModel
	for _, raw := range list.Data {
		var m model
		if err := json.Unmarshal(raw, &m); err != nil {
			// A row whose metadata does not decode still names a model, and an
			// unknown limit is not an absent model: keep the id with the same
			// unknown-window/no-price shape a thin payload gets. Only a row
			// with no readable id — or an advertised `tools: false`, which
			// survives in the same snippet — is dropped.
			var idOnly struct {
				ID           string `json:"id"`
				Capabilities *struct {
					Tools *struct {
						Supported *bool `json:"supported"`
					} `json:"tools"`
				} `json:"capabilities"`
			}
			if json.Unmarshal(raw, &idOnly) != nil || idOnly.ID == "" {
				slog.Debug("model row skipped", "provider", base, "err", causeClass(err))
				continue
			}
			if c := idOnly.Capabilities; c != nil && c.Tools != nil &&
				c.Tools.Supported != nil && !*c.Tools.Supported {
				slog.Debug("model row skipped", "provider", base, "err", causeClass(err))
				continue
			}
			slog.Debug("model row kept id-only", "provider", base, "id", idOnly.ID, "err", causeClass(err))
			out = append(out, CachedModel{ID: idOnly.ID})
			continue
		}
		if m.ID == "" {
			continue
		}
		// An explicit `tools: {supported: false}` is an advertised no: moca is
		// a tool-calling agent, so an embedding or image model is not a model
		// here. An absent capability is unknown, not a no.
		if c := m.Capabilities; c != nil && c.Tools != nil && c.Tools.Supported != nil && !*c.Tools.Supported {
			continue
		}
		cm := CachedModel{ID: m.ID, Protocol: protocolFromAPI(m.API),
			Ctx: m.Limit.Context, MaxOut: m.Limit.Output}
		if cm.Ctx <= 0 {
			// Some payloads publish only the input ceiling; it is the
			// conservative window when there is no separate context field.
			cm.Ctx = m.Limit.Input
		}
		if m.Cost != nil && m.Cost.Text != nil && m.Cost.Text.Input != nil {
			in := priceValue(m.Cost.Text.Input.Price)
			var outp *float64
			if m.Cost.Text.Output != nil {
				outp = priceValue(m.Cost.Text.Output.Price)
			}
			// Both rates are required for the price to count as known:
			// moca's unit carries one rate per kind, so a lone input rate
			// would claim the output side costs $0.00 — a price the provider
			// never published. A published zero is a price (a free model).
			if in != nil && outp != nil {
				cm.Cost = config.CostConfig{Input: *in, Output: *outp,
					CacheRead:  deref(priceValue(m.Cost.Text.Input.Cache.Read)),
					CacheWrite: deref(priceValue(m.Cost.Text.Input.Cache.Write))}
				cm.CostKnown = true
			}
		}
		if c := m.Capabilities; c != nil && c.Reasoning != nil {
			cm.Reasons = c.Reasoning.Supported == nil || *c.Reasoning.Supported
			cm.Efforts = c.Reasoning.Effort
		}
		out = append(out, cm)
	}
	return out, nil
}

// priceValue reads a price that is either a number or an object keyed by a
// cache TTL (OpenCode publishes cache-write prices as {"30m":0.25,"12h":0.5}).
// moca's CostConfig holds one rate per kind and a run never asks for a long
// TTL, so the cheapest tier — the short-TTL rate — is the one a run pays.
func priceValue(raw json.RawMessage) *float64 {
	if len(raw) == 0 {
		return nil
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return &f
	}
	var tiers map[string]float64
	if json.Unmarshal(raw, &tiers) == nil && len(tiers) > 0 {
		best := math.Inf(1)
		for _, v := range tiers {
			best = min(best, v)
		}
		return &best
	}
	return nil
}

// deref reads an optional JSON number, 0 when absent.
func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
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

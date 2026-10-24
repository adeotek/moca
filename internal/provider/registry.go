package provider

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

// ErrUnknownModel: the qualified model is not in the catalog or the config.
var ErrUnknownModel = errors.New("unknown model")

var defaultBaseURLs = map[string]map[string]string{
	"anthropic": {"anthropic-messages": "https://api.anthropic.com"},
	"ollama":    {"openai-completions": ollamaDefaultHost + "/v1"},
	"openai":    {"openai-responses": "https://api.openai.com/v1", "openai-completions": "https://api.openai.com/v1"},
	"opencode-go": {
		"anthropic-messages": "https://opencode.ai/zen/go",
		"openai-completions": "https://opencode.ai/zen/go/v1",
		"openai-responses":   "https://opencode.ai/zen/go/v1",
	},
	// The pay-as-you-go twin of the go tier: same account key, no
	// per-conversation routing header, chat-completions only.
	"opencode-zen": {"openai-completions": "https://opencode.ai/zen/v1"},
	"openrouter":   {"openai-completions": "https://openrouter.ai/api/v1"},
}

type Registry struct {
	cfg    config.Config
	hc     *http.Client
	notify func(RetryNotice)
	getenv func(string) string // OLLAMA_HOST; a field so tests do not touch the process env
	// mu guards models and ollamaErr: a discovery run (TUI /model, a /clear
	// restart) rewrites the ollama entries and the recorded failure while
	// the agent may resolve models and Verify may read the failure.
	mu        sync.RWMutex
	models    map[string]Model
	oauth     map[string]CredentialFunc
	sessionID string
	// cache holds the per-provider model lists (24 h); discovered is the last
	// merged list per provider, so a rebuild (a cache clear, a refresh) is
	// deterministic and never stacks duplicates.
	cache      *ModelsCache
	discovered map[string][]CachedModel
	// ollamaErr is why the last discovery failed (nil when it succeeded or
	// never ran); Verify cites it when a configured ollama model is missing.
	ollamaErr error
}

// NewRegistry builds the catalog. A nil hc means the shared default client
// (header-bounded): every request the registry makes — discovery, the
// adapters, an OAuth refresh — goes through hc, so it is never left nil.
func NewRegistry(cfg config.Config, hc *http.Client, notify func(RetryNotice)) (*Registry, error) {
	if hc == nil {
		hc = defaultHTTPClient()
	}
	r := &Registry{cfg: cfg, hc: hc, notify: notify, getenv: os.Getenv, models: map[string]Model{},
		oauth: map[string]CredentialFunc{}, sessionID: rand.Text(),
		cache: NewModelsCache(config.ModelsCacheFile(), ModelsCacheTTL), discovered: map[string][]CachedModel{}}
	for _, m := range builtinCatalog {
		m.CostKnown = true // every curated row carries authored prices
		r.models[m.Qualified()] = m
	}
	for pname, p := range cfg.Providers {
		r.addSeedsLocked(pname)
		if err := r.applyModelOverrides(pname, p); err != nil {
			return nil, err
		}
	}
	// Model existence is deliberately NOT checked here: a provider's list is
	// discovered after construction (SPECS §6), so a model absent from the
	// catalog may still exist upstream. Verify() is the gate, and it runs
	// after discovery with the discovered ids in hand.
	return r, nil
}

// applyModelOverrides merges a provider's declared models (and per-model
// overrides of catalog or discovered ones) into the registry. The caller
// holds mu when the registry is live.
func (r *Registry) applyModelOverrides(pname string, p config.ProviderConfig) error {
	local := config.IsLocalProvider(pname)
	for mid, o := range p.Models {
		q := pname + "/" + mid
		m, ok := r.models[q]
		if !ok {
			m = Model{Provider: pname, ID: mid, Protocol: p.Protocol, ThinkingMode: "none", MaxOutput: 8192}
			if local {
				// A declared local model with nothing else said: the OpenAI-
				// compatible protocol and the assumed window.
				m.Protocol, m.ContextWindow = "openai-completions", ollamaAssumedWindow
			}
		}
		if o.Protocol != "" {
			m.Protocol = o.Protocol
		}
		if m.Protocol == "" {
			return fmt.Errorf("model %q: declare its protocol under providers.%s.models.%s.protocol (%s)",
				q, pname, mid, strings.Join(config.Protocols, "|"))
		}
		if o.ContextWindow != 0 {
			m.ContextWindow, m.WindowAssumed = o.ContextWindow, false
		}
		if o.MaxOutputTokens != 0 {
			m.MaxOutput = o.MaxOutputTokens
		}
		if o.Cost != nil {
			m.Cost, m.CostKnown = *o.Cost, true
		}
		r.models[q] = m
	}
	return nil
}

// Verify checks that the configured default and hard models exist. It runs
// after discovery, which is what makes a local provider's models known.
func (r *Registry) Verify() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, q := range []string{r.cfg.Model, r.cfg.ModelHard} {
		if q == "" {
			continue
		}
		if _, ok := r.models[q]; ok {
			continue
		}
		if pn, id, err := config.SplitModel(q); err == nil && pn == "ollama" {
			if r.ollamaErr != nil {
				return fmt.Errorf("model %q: %w", q, r.ollamaErr)
			}
			return fmt.Errorf("model %q: the ollama server at %s has no model %q with tool support — pull it (ollama pull %s) or declare it under providers.ollama.models", q, r.ollamaBase(), id, id)
		}
		if hint := r.nearestModels(q); hint != "" {
			return fmt.Errorf("model %q is not in the catalog and the provider does not list it; did you mean: %s (declare any other id under providers.<name>.models)", q, hint)
		}
		return fmt.Errorf("model %q is not in the catalog; declare it under providers.<name>.models", q)
	}
	return nil
}

// nearestModels suggests up to three known ids when a configured model is
// missing: the discovered list makes a typo diagnosable ("did you mean"),
// which is the cost of deferring model validation to Verify.
func (r *Registry) nearestModels(q string) string {
	pn, id, err := config.SplitModel(q)
	if err != nil || id == "" {
		return ""
	}
	base := strings.ToLower(id)
	var cand []string
	r.mu.RLock()
	for k := range r.models {
		mid, ok := strings.CutPrefix(k, pn+"/")
		if !ok || mid == id {
			continue
		}
		low := strings.ToLower(mid)
		if strings.Contains(low, base) || strings.Contains(base, low) || commonPrefix(low, base) >= 4 {
			cand = append(cand, k)
		}
	}
	r.mu.RUnlock()
	slices.Sort(cand)
	if len(cand) > 3 {
		cand = cand[:3]
	}
	return strings.Join(cand, ", ")
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

func (r *Registry) SetOAuth(provider string, fn CredentialFunc) { r.oauth[provider] = fn }

func (r *Registry) Models() []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Model, 0, len(r.models))
	for _, m := range r.models {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b Model) int { return strings.Compare(a.Qualified(), b.Qualified()) })
	return out
}

// CheckCredential verifies, without any network call, that a model exists and
// its provider credential is usable: an env lookup for api_key providers, and
// for store-backed OAuth providers that a login is stored (a token near
// expiry is refreshed by the first request, which has a cancellable context,
// not here). A TUI model switch calls this on its update goroutine, so it
// fails fast before anything changes and never blocks on I/O.
func (r *Registry) CheckCredential(qualified string) error {
	pname, _, err := config.SplitModel(qualified)
	if err != nil {
		return err
	}
	r.mu.RLock()
	_, ok := r.models[qualified]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w %q", ErrUnknownModel, qualified)
	}
	if r.cfg.Providers[pname].Auth == "oauth" && r.oauth[pname] == nil {
		if _, ok := oauthProviders[pname]; ok {
			_, found, err := NewDefaultStore().Get(pname)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("not logged in to %s: run /login in the TUI, or moca login %s", pname, pname)
			}
			return nil
		}
	}
	_, err = r.credential(pname)(context.Background(), llm.Request{})
	return err
}

func (r *Registry) baseURL(provider, protocol string) string {
	p := r.cfg.Providers[provider]
	if provider == "ollama" {
		return r.ollamaBase()
	}
	if u := p.BaseURLs[protocol]; u != "" {
		return u
	}
	if p.BaseURL != "" {
		return p.BaseURL
	}
	return defaultBaseURLs[provider][protocol]
}

func (r *Registry) credential(provider string) CredentialFunc {
	p := r.cfg.Providers[provider]
	// OpenCode Go requires a stable per-conversation routing header
	// (https://opencode.ai/docs/go/#where-can-i-use-it) and rejects requests
	// without it (HTTP 400 MissingSessionID).
	// F-8: the header follows the *conversation*, not the process. One moca
	// process can serve several sessions (TUI /resume, ACP sessions); a single
	// process-wide id routed them all to one shard, mixing prompt-cache
	// prefixes. req.CacheKey is the session's stable cache-routing key; the
	// process id is only the fallback for a request that carries none.
	var extra map[string]string
	switch provider {
	case "opencode-go":
		extra = map[string]string{"x-opencode-session": r.sessionID}
	case "openrouter":
		// Optional app attribution (openrouter.ai rankings) plus the
		// per-conversation sticky-routing key: OpenRouter pins the upstream
		// provider per session, which is what keeps its prompt cache warm.
		extra = map[string]string{
			"X-OpenRouter-Title": "moca",
			"HTTP-Referer":       "https://github.com/adeotek/moca",
		}
	}
	withExtra := func(c Credential, req llm.Request) Credential {
		h := make(map[string]string, len(extra)+1)
		for k, v := range extra {
			h[k] = v
		}
		switch {
		case provider == "opencode-go" && req.CacheKey != "":
			h["x-opencode-session"] = req.CacheKey
		case provider == "openrouter" && req.CacheKey != "":
			h["x-session-id"] = req.CacheKey
		}
		if len(h) == 0 {
			return c
		}
		if c.Headers == nil {
			c.Headers = make(map[string]string, len(h))
		}
		for k, v := range h {
			c.Headers[k] = v
		}
		return c
	}
	if p.Auth == "oauth" {
		if fn := r.oauth[provider]; fn != nil {
			return func(ctx context.Context, req llm.Request) (Credential, error) {
				c, err := fn(ctx, req)
				if err != nil {
					return Credential{}, err
				}
				return withExtra(c, req), nil
			}
		}
		if oc, ok := oauthProviders[provider]; ok {
			// Store-backed subscription credentials: read auth.json (0600),
			// refresh under the cross-process lock near expiry.
			fn := NewDefaultStore().CredentialFor(provider, oc.refresher(r.hc))
			return func(ctx context.Context, req llm.Request) (Credential, error) {
				c, err := fn(ctx, req)
				if err != nil {
					return Credential{}, err
				}
				return withExtra(c, req), nil
			}
		}
		return func(context.Context, llm.Request) (Credential, error) {
			return Credential{}, fmt.Errorf("provider %s: OAuth is not available; set auth \"api_key\"", provider)
		}
	}
	// api_key: a key stored with /login (TUI) or `moca login <provider>` wins
	// over the config's env: reference — an explicit, per-provider credential
	// outranks an inherited environment, and /logout restores the fallback.
	// The store is read per request, so a key stored mid-session is used by
	// the next request without restarting anything.
	store := NewDefaultStore()
	return func(_ context.Context, req llm.Request) (Credential, error) {
		key, found, serr := store.GetAPIKey(provider)
		if serr == nil && found {
			return withExtra(Credential{Token: key}, req), nil
		}
		var eerr error
		if p.APIKey != "" {
			if v, err := config.ResolveEnv(p.APIKey); err == nil {
				return withExtra(Credential{Token: v}, req), nil
			} else {
				eerr = err
			}
		}
		switch {
		case serr != nil:
			// A corrupt store and no env fallback: the store error is the
			// actionable one (it says how to reset the file).
			return Credential{}, fmt.Errorf("provider %s: %w", provider, serr)
		case eerr != nil:
			return Credential{}, fmt.Errorf("provider %s: %w; store a key with /login in the TUI or moca login %s", provider, eerr, provider)
		case config.IsLocalProvider(provider):
			// A local server needs no key; one stored with /login (or an
			// apiKey env reference) is sent when the server sits behind an
			// authenticating proxy.
			return Credential{}, nil
		default:
			return Credential{}, fmt.Errorf("provider %s: no API key stored and providers.%s.apiKey is not set; add one with /login in the TUI or moca login %s", provider, provider, provider)
		}
	}
}

func (r *Registry) Resolve(qualified string) (Model, Adapter, error) {
	pname, _, err := config.SplitModel(qualified)
	if err != nil {
		return Model{}, nil, err
	}
	r.mu.RLock()
	m, ok := r.models[qualified]
	r.mu.RUnlock()
	if !ok {
		return Model{}, nil, fmt.Errorf("%w %q", ErrUnknownModel, qualified)
	}
	p, ok := r.cfg.Providers[pname]
	if !ok {
		return Model{}, nil, fmt.Errorf("unknown provider %q", pname)
	}
	if p.Auth == "oauth" {
		// The SIWC subscription route serves the Responses API only
		// (docs/specs/oauth-verification.md §2.4).
		if _, hasOAuth := oauthProviders[pname]; hasOAuth && m.Protocol != "openai-responses" {
			return Model{}, nil, fmt.Errorf("model %s: subscription OAuth is only available on the openai-responses protocol (model speaks %s)", qualified, m.Protocol)
		}
	}
	base := r.baseURL(pname, m.Protocol)
	if base == "" {
		return Model{}, nil, fmt.Errorf("provider %s has no base URL for protocol %s", pname, m.Protocol)
	}
	cred := r.credential(pname)
	var a Adapter
	switch m.Protocol {
	case "anthropic-messages":
		a = newAnthropic(m, base, cred, r.hc)
	case "openai-completions":
		a = newOpenAICompletions(m, base, cred, r.hc)
	case "openai-responses":
		a = newOpenAIResponses(m, base, cred, r.hc)
	default:
		return Model{}, nil, fmt.Errorf("model %s: unknown protocol %q", qualified, m.Protocol)
	}
	if pname == "ollama" {
		a = ollamaGuard{inner: a, base: base}
	}
	return m, WithRetry(a, DefaultRetryPolicy(r.notify)), nil
}

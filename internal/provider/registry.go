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
	"opencode":   {"openai-completions": "https://opencode.ai/zen/v1"},
	"openrouter": {"openai-completions": "https://openrouter.ai/api/v1"},
}

type Registry struct {
	cfg    config.Config
	hc     *http.Client
	notify func(RetryNotice)
	getenv func(string) string // OLLAMA_HOST; a field so tests do not touch the process env
	// mu guards models, resolved and the recorded failures: a discovery run
	// (TUI /model, a /clear-models-cache re-read) rewrites a provider's rows
	// while the agent may resolve models and Verify may read the failures.
	mu     sync.RWMutex
	models map[string]Model
	// resolved remembers every row Resolve handed out or Verify checked. The
	// summarizer, the judge and /hard re-resolve their model by name
	// mid-session; a later discovery that stops listing it (the provider
	// retired the id) must not strand them. The live row wins when present.
	resolved  map[string]Model
	oauth     map[string]CredentialFunc
	sessionID string
	// cache holds the per-provider model lists (24 h); discovered is the last
	// merged list per provider, so a rebuild (a cache clear, a refresh) is
	// deterministic and never stacks duplicates.
	cache      *ModelsCache
	discovered map[string][]CachedModel
	// discoveryErr is why a provider's last list fetch failed (absent when it
	// succeeded, was served fresh from the cache, or never ran).
	discoveryErr map[string]error
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
		cache: NewModelsCache(config.ModelsCacheDir(), ModelsCacheTTL), discovered: map[string][]CachedModel{},
		discoveryErr: map[string]error{}, resolved: map[string]Model{}}
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
	_, builtin := config.BuiltinProviders[pname]
	for mid, o := range p.Models {
		q := pname + "/" + mid
		m, ok := r.models[q]
		if !ok {
			// A declared row is a decision, priced or not (§6) — except on a
			// built-in provider, where it is most often a window/cost override
			// of an id discovery has not merged yet, whose price is unknown.
			m = Model{Provider: pname, ID: mid, Protocol: p.Protocol, ThinkingMode: "none", MaxOutput: 8192, CostKnown: !builtin}
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

// Verify checks that the configured default and hard models exist, and
// remembers them (resolved) so they outlive a later relist. It runs after
// discovery, which is what makes a local provider's models known.
//
// The credential probe is made BEFORE the lock, not under it:
// credentialReady reads the credential store (a flock'd file) and, for a
// store-backed OAuth provider near expiry, refreshes over the network, so
// holding the registry's write lock across it would stall every Resolve and
// every Status() read behind a file lock or an HTTP round trip.
func (r *Registry) Verify() error {
	cred := r.verifyCredentials()
	r.mu.Lock()
	defer r.mu.Unlock()
	// The judge's model is not a start gate (an unusable judge asks), but
	// it is remembered like the others when present.
	if m, ok := r.models[r.cfg.Permissions.Auto.Model]; ok {
		r.resolved[m.Qualified()] = m
	}
	for _, q := range []string{r.cfg.Model, r.cfg.ModelHard} {
		if q == "" {
			continue
		}
		if m, ok := r.models[q]; ok {
			r.resolved[q] = m
			continue
		}
		if pn, id, err := config.SplitModel(q); err == nil && pn == "ollama" {
			if r.ollamaErr != nil {
				return fmt.Errorf("model %q: %w", q, r.ollamaErr)
			}
			return fmt.Errorf("model %q: the ollama server at %s has no model %q with tool support — pull it (ollama pull %s) or declare it under providers.ollama.models", q, r.ollamaBase(), id, id)
		}
		if pn, _, err := config.SplitModel(q); err == nil && r.discoveryErr[pn] != nil {
			// The list could not be read: "the provider does not list it"
			// would be a claim moca never checked.
			derr := r.discoveryErr[pn]
			remediation := "retry when the provider is reachable"
			var he *HTTPError
			if errors.As(derr, &he) && (he.Status == 401 || he.Status == 403) {
				// A revoked or expired key is fixed by the key, not by retrying.
				remediation = "fix the provider's credential (update the key or log in again)"
			}
			return fmt.Errorf("model %q is not in the catalog and %s's model list could not be fetched (%v) — %s, or declare it under providers.%s.models", q, pn, derr, remediation, pn)
		}
		if pn, _, err := config.SplitModel(q); err == nil && r.listedByDiscovery(pn) {
			// Discovery skips a provider with no usable credential, so its
			// list was never read either: the credential is the fix, not a
			// declaration. The probe itself ran before the lock (it reads
			// the credential store); this branch only reads its answer.
			if cerr := cred[pn]; cerr != nil {
				return fmt.Errorf("model %q is not in the catalog, and %s's model list was not read: %w", q, pn, cerr)
			}
		}
		if hint := r.nearestModelsLocked(q); hint != "" {
			return fmt.Errorf("model %q is not in the catalog and the provider does not list it; did you mean: %s (declare any other id under providers.<name>.models)", q, hint)
		}
		return fmt.Errorf("model %q is not in the catalog; declare it under providers.<name>.models", q)
	}
	return nil
}

// verifyCredentials answers credentialReady for every provider Verify may
// cite: the provider of model/modelHard, when discovery reads its list and
// that list was read successfully. It runs WITHOUT the registry lock — the
// probe reads the credential store (a flock'd file) and can refresh a
// subscription token over the network, which must never happen under a lock
// every Resolve and Status() read contends for. A provider missing from the
// map is one the loop never asks about; a nil value is a credential that
// resolves, or a provider whose failed fetch is named as itself instead.
func (r *Registry) verifyCredentials() map[string]error {
	cred := map[string]error{}
	for _, q := range []string{r.cfg.Model, r.cfg.ModelHard} {
		pn, _, err := config.SplitModel(q)
		if err != nil || pn == "ollama" || !r.listedByDiscovery(pn) {
			continue
		}
		if _, done := cred[pn]; done {
			continue
		}
		// A model the live catalog holds is one the loop below never asks
		// about: no probe, so a start whose configured models both resolve
		// does no credential-store read at all.
		if r.hasModel(q) {
			continue
		}
		cred[pn] = nil
		// A failed fetch is cited as itself, with its own remediation:
		// probing the credential would replace that answer with a less
		// specific one.
		if r.discoveryError(pn) == nil {
			cred[pn] = r.credentialReady(pn)
		}
	}
	return cred
}

// discoveryError reads a provider's recorded list failure under the lock.
func (r *Registry) discoveryError(pname string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.discoveryErr[pname]
}

// hasModel reports whether the live catalog holds the qualified model (the
// predicate Verify's loop tests, without taking the lock for the whole walk).
func (r *Registry) hasModel(q string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.models[q]
	return ok
}

// nearestModelsLocked suggests up to three known ids when a configured model
// is missing: the discovered list makes a typo diagnosable ("did you mean"),
// which is the cost of deferring model validation to Verify. The caller holds
// mu (Verify does).
func (r *Registry) nearestModelsLocked(q string) string {
	pn, id, err := config.SplitModel(q)
	if err != nil || id == "" {
		return ""
	}
	base := strings.ToLower(id)
	var cand []string
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
	slices.Sort(cand)
	if len(cand) > 3 {
		cand = cand[:3]
	}
	return strings.Join(cand, ", ")
}

// listedByDiscovery reports whether discovery reads the provider's model list:
// every configured provider except Ollama (DiscoverOllama owns it) and a custom
// provider that declares its models (the declarations are the contract).
func (r *Registry) listedByDiscovery(pname string) bool {
	p, ok := r.cfg.Providers[pname]
	if !ok || pname == "ollama" {
		return false
	}
	_, builtin := config.BuiltinProviders[pname]
	return builtin || len(p.Models) == 0
}

// lookupLocked is the row lookup Resolve and Lookup share: the live row, else
// the row handed out before a relist dropped it (resolved). CheckCredential
// deliberately reads the live catalog only. The caller holds mu.
func (r *Registry) lookupLocked(qualified string) (Model, bool) {
	if m, ok := r.models[qualified]; ok {
		return m, true
	}
	m, ok := r.resolved[qualified]
	return m, ok
}

// Lookup returns the row a qualified model resolves to, if any.
func (r *Registry) Lookup(qualified string) (Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lookupLocked(qualified)
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
// fails fast before anything changes: no network call, and no registry lock
// held across the credential-store read it does make.
func (r *Registry) CheckCredential(qualified string) error {
	pname, _, err := config.SplitModel(qualified)
	if err != nil {
		return err
	}
	// The live catalog only: a user's switch to an id a relist retired is
	// refused (the session's own rows stay resolvable through Resolve).
	r.mu.RLock()
	_, ok := r.models[qualified]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w %q", ErrUnknownModel, qualified)
	}
	return r.credentialReady(pname)
}

// credentialReady is the network-free credential check CheckCredential and
// ProviderReady share: an env lookup or a stored key for api_key providers,
// and for store-backed OAuth providers only that a login is stored — the
// refresh (network, under the cross-process lock) is left to the first
// request, which has a cancellable context. It does read the credential store
// (one flock'd file), so a caller holding the registry lock must hoist this
// call out of it; Verify does.
func (r *Registry) credentialReady(pname string) error {
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
	_, err := r.credential(pname)(context.Background(), llm.Request{})
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
			// EnvError already names the variable; name the file it most
			// likely belongs in, since the .env is read at startup for
			// exactly this reason.
			return Credential{}, fmt.Errorf("provider %s: %w; add it to %s, or store a key with /login in the TUI or moca login %s", provider, eerr, config.DotEnvFile(), provider)
		case config.IsLocalProvider(provider):
			// A local server needs no key; one stored with /login (or an
			// apiKey env reference) is sent when the server sits behind an
			// authenticating proxy.
			return Credential{}, nil
		default:
			return Credential{}, fmt.Errorf("provider %s: no API key stored and providers.%s.apiKey is not set; add one with /login in the TUI, moca login %s, or a KEY=VALUE line in %s", provider, provider, provider, config.DotEnvFile())
		}
	}
}

func (r *Registry) Resolve(qualified string) (Model, Adapter, error) {
	pname, _, err := config.SplitModel(qualified)
	if err != nil {
		return Model{}, nil, err
	}
	m, ok := r.Lookup(qualified)
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
	r.mu.Lock()
	r.resolved[qualified] = m
	r.mu.Unlock()
	return m, WithRetry(a, DefaultRetryPolicy(r.notify)), nil
}

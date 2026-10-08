package provider

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/config"
)

// ErrUnknownModel: the qualified model is not in the catalog or the config.
var ErrUnknownModel = errors.New("unknown model")

var defaultBaseURLs = map[string]map[string]string{
	"anthropic": {"anthropic-messages": "https://api.anthropic.com"},
	"openai":    {"openai-responses": "https://api.openai.com/v1", "openai-completions": "https://api.openai.com/v1"},
	"opencode-go": {
		"anthropic-messages": "https://opencode.ai/zen/go",
		"openai-completions": "https://opencode.ai/zen/go/v1",
		"openai-responses":   "https://opencode.ai/zen/go/v1",
	},
}

type Registry struct {
	cfg       config.Config
	hc        *http.Client
	notify    func(RetryNotice)
	models    map[string]Model
	oauth     map[string]CredentialFunc
	sessionID string
}

func NewRegistry(cfg config.Config, hc *http.Client, notify func(RetryNotice)) (*Registry, error) {
	r := &Registry{cfg: cfg, hc: hc, notify: notify, models: map[string]Model{}, oauth: map[string]CredentialFunc{}, sessionID: rand.Text()}
	for _, m := range builtinCatalog {
		r.models[m.Qualified()] = m
	}
	for pname, p := range cfg.Providers {
		for mid, o := range p.Models {
			q := pname + "/" + mid
			m, ok := r.models[q]
			if !ok {
				m = Model{Provider: pname, ID: mid, Protocol: p.Protocol, ThinkingMode: "none", MaxOutput: 8192}
			}
			if o.Protocol != "" {
				m.Protocol = o.Protocol
			}
			if m.Protocol == "" {
				return nil, fmt.Errorf("model %q: declare its protocol under providers.%s.models.%s.protocol (%s)",
					q, pname, mid, strings.Join(config.Protocols, "|"))
			}
			if o.ContextWindow != 0 {
				m.ContextWindow = o.ContextWindow
			}
			if o.MaxOutputTokens != 0 {
				m.MaxOutput = o.MaxOutputTokens
			}
			if o.Cost != nil {
				m.Cost = *o.Cost
			}
			r.models[q] = m
		}
	}
	for _, q := range []string{cfg.Model, cfg.ModelHard} {
		if q == "" {
			continue
		}
		if _, ok := r.models[q]; !ok {
			return nil, fmt.Errorf("model %q is not in the catalog; declare it under providers.<name>.models", q)
		}
	}
	return r, nil
}

func (r *Registry) SetOAuth(provider string, fn CredentialFunc) { r.oauth[provider] = fn }

func (r *Registry) Models() []Model {
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
	if _, ok := r.models[qualified]; !ok {
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
	_, err = r.credential(pname)(context.Background())
	return err
}

func (r *Registry) baseURL(provider, protocol string) string {
	p := r.cfg.Providers[provider]
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
	// without it (HTTP 400 MissingSessionID). One process is one conversation
	// today; phase 3+ sessions must rebind it per conversation.
	var extra map[string]string
	if provider == "opencode-go" {
		extra = map[string]string{"x-opencode-session": r.sessionID}
	}
	withExtra := func(c Credential) Credential {
		if len(extra) == 0 {
			return c
		}
		if c.Headers == nil {
			c.Headers = map[string]string{}
		}
		for k, v := range extra {
			c.Headers[k] = v
		}
		return c
	}
	if p.Auth == "oauth" {
		if fn := r.oauth[provider]; fn != nil {
			return func(ctx context.Context) (Credential, error) {
				c, err := fn(ctx)
				if err != nil {
					return Credential{}, err
				}
				return withExtra(c), nil
			}
		}
		if oc, ok := oauthProviders[provider]; ok {
			// Store-backed subscription credentials: read auth.json (0600),
			// refresh under the cross-process lock near expiry.
			fn := NewDefaultStore().CredentialFor(provider, oc.refresher(r.hc))
			return func(ctx context.Context) (Credential, error) {
				c, err := fn(ctx)
				if err != nil {
					return Credential{}, err
				}
				return withExtra(c), nil
			}
		}
		return func(context.Context) (Credential, error) {
			return Credential{}, fmt.Errorf("provider %s: OAuth is not available; set auth \"api_key\"", provider)
		}
	}
	// api_key: a key stored with /login (TUI) or `moca login <provider>` wins
	// over the config's env: reference — an explicit, per-provider credential
	// outranks an inherited environment, and /logout restores the fallback.
	// The store is read per request, so a key stored mid-session is used by
	// the next request without restarting anything.
	store := NewDefaultStore()
	return func(context.Context) (Credential, error) {
		key, found, serr := store.GetAPIKey(provider)
		if serr == nil && found {
			return withExtra(Credential{Token: key}), nil
		}
		var eerr error
		if p.APIKey != "" {
			if v, err := config.ResolveEnv(p.APIKey); err == nil {
				return withExtra(Credential{Token: v}), nil
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
	m, ok := r.models[qualified]
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
	return m, WithRetry(a, DefaultRetryPolicy(r.notify)), nil
}

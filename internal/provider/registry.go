package provider

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/config"
)

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
	cfg    config.Config
	hc     *http.Client
	notify func(RetryNotice)
	models map[string]Model
	oauth  map[string]CredentialFunc
}

func NewRegistry(cfg config.Config, hc *http.Client, notify func(RetryNotice)) (*Registry, error) {
	r := &Registry{cfg: cfg, hc: hc, notify: notify, models: map[string]Model{}, oauth: map[string]CredentialFunc{}}
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
	if p.Auth == "oauth" {
		if fn := r.oauth[provider]; fn != nil {
			return fn
		}
		return func(context.Context) (Credential, error) {
			return Credential{}, fmt.Errorf("provider %s uses oauth: run `moca login %s`", provider, provider)
		}
	}
	return func(context.Context) (Credential, error) {
		v, err := config.ResolveEnv(p.APIKey)
		if err != nil {
			return Credential{}, fmt.Errorf("provider %s: %w", provider, err)
		}
		return Credential{Token: v}, nil
	}
}

func (r *Registry) Resolve(qualified string) (Model, Adapter, error) {
	pname, _, err := config.SplitModel(qualified)
	if err != nil {
		return Model{}, nil, err
	}
	m, ok := r.models[qualified]
	if !ok {
		return Model{}, nil, fmt.Errorf("unknown model %q", qualified)
	}
	if _, ok := r.cfg.Providers[pname]; !ok {
		return Model{}, nil, fmt.Errorf("unknown provider %q", pname)
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

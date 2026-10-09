package provider

// Ollama (https://ollama.com) — a local or LAN model server. It speaks the
// OpenAI chat-completions protocol under /v1 (the existing adapter), needs no
// API key, and has no static catalog: the models are whatever has been pulled
// onto the server, so they are discovered from its native API:
//
//	GET  /api/tags   the pulled models
//	POST /api/show   per model: capabilities and Modelfile parameters
//
// Discovery is best effort and bounded (a few seconds): a server that is down
// is not an error until a request needs it. Models declared under
// providers.ollama.models always exist and override what discovery found.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

const (
	ollamaDefaultHost = "http://localhost:11434"

	// ollamaAssumedWindow is the context window assumed when the server does
	// not report one for a model (num_ctx absent from its Modelfile). The
	// OpenAI-compatible endpoint cannot set num_ctx per request, so the real
	// window is whatever the server was started with (OLLAMA_CONTEXT_LENGTH):
	// declare providers.ollama.models.<id>.contextWindow to match it.
	ollamaAssumedWindow = config.MinContextWindow

	ollamaDiscoverTimeout = 4 * time.Second
	ollamaShowParallel    = 8
)

// ollamaBaseURL resolves the OpenAI-compatible root: the config's baseUrl,
// else OLLAMA_HOST (the variable Ollama's own CLI honors), else localhost.
func ollamaBaseURL(p config.ProviderConfig, getenv func(string) string) string {
	raw := p.BaseURL
	if raw == "" {
		raw = getenv("OLLAMA_HOST")
	}
	if raw == "" {
		raw = ollamaDefaultHost
	}
	return normalizeOllamaURL(raw)
}

// normalizeOllamaURL turns the forms people write — "nas", "nas:11434",
// "http://nas:11434", ":11434", "0.0.0.0", "https://host/ollama/v1" — into a
// full URL ending in the OpenAI-compatible root (/v1 is appended to a bare
// host). A wildcard bind address ("0.0.0.0") means "this machine".
func normalizeOllamaURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimRight(raw, "/")
	}
	host, port := u.Hostname(), u.Port()
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if port == "" && u.Scheme == "http" {
		port = "11434"
	}
	if strings.Contains(host, ":") { // IPv6 literal
		host = "[" + host + "]"
	}
	u.Host = host
	if port != "" {
		u.Host = host + ":" + port
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/v1"
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

// ollamaRoot is the native-API root: the OpenAI-compatible URL without /v1.
func ollamaRoot(base string) string { return strings.TrimSuffix(base, "/v1") }

// ollamaBase is the resolved OpenAI-compatible root for this registry.
func (r *Registry) ollamaBase() string {
	return ollamaBaseURL(r.cfg.Providers["ollama"], r.getenv)
}

type ollamaTags struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

type ollamaShow struct {
	Capabilities []string `json:"capabilities"`
	Parameters   string   `json:"parameters"`
}

var numCtxRe = regexp.MustCompile(`(?m)^\s*num_ctx\s+(\d+)`)

// Discovery reports what a discovery run found.
type Discovery struct {
	Base    string
	Models  []string
	Assumed []string       // model ids whose window is a guess (server reported none)
	Windows map[string]int // model id → context window (reported or assumed)
}

// modelFromShow builds the catalog entry for a pulled model: the zero Model
// (empty ID) for one that cannot drive an agent (an embedding model, or no
// tool support), else the model, its window and whether the server reported
// that window (false = assumed). known=false (the show call failed or the
// server predates capabilities) keeps the model without judging it.
func modelFromShow(id string, sh ollamaShow, known bool) (m Model, window int, reported bool) {
	caps := sh.Capabilities
	if known && len(caps) > 0 && (!slices.Contains(caps, "completion") || !slices.Contains(caps, "tools")) {
		return Model{}, 0, false
	}
	window = ollamaAssumedWindow
	if mm := numCtxRe.FindStringSubmatch(sh.Parameters); mm != nil {
		if n, err := strconv.Atoi(mm[1]); err == nil && n > 0 {
			window, reported = n, true
		}
	}
	m = Model{Provider: "ollama", ID: id, Protocol: "openai-completions", ContextWindow: window,
		MaxOutput: min(max(window/4, 2048), 16384), ThinkingMode: "none"}
	if slices.Contains(caps, "thinking") {
		m.ThinkingMode = "openai"
		m.ThinkingLevelMap = map[llm.Effort]string{llm.EffortOff: "none", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
	}
	return m, window, reported
}

// DiscoverOllama (re)reads the server's models into the registry. Models
// from an earlier run that are gone from the server disappear; declared ones
// stay. It returns the error of an unreachable server — whether that matters
// is the caller's call.
func (r *Registry) DiscoverOllama(ctx context.Context) (Discovery, error) {
	p, ok := r.cfg.Providers["ollama"]
	if !ok {
		return Discovery{}, nil
	}
	base := ollamaBaseURL(p, r.getenv)
	d := Discovery{Base: base, Windows: map[string]int{}}
	ctx, cancel := context.WithTimeout(ctx, ollamaDiscoverTimeout)
	defer cancel()
	hdr := http.Header{}
	if cred, err := r.credential("ollama")(ctx); err == nil && cred.Token != "" {
		hdr.Set("Authorization", "Bearer "+cred.Token)
	}
	var tags ollamaTags
	if err := r.ollamaJSON(ctx, http.MethodGet, ollamaRoot(base)+"/api/tags", hdr, nil, &tags); err != nil {
		r.ollamaErr = ollamaUnreachable(base, err)
		return d, r.ollamaErr
	}
	r.ollamaErr = nil
	type found struct {
		m       Model
		window  int
		keep    bool
		assumed bool
	}
	res := make([]found, len(tags.Models))
	var wg sync.WaitGroup
	sem := make(chan struct{}, ollamaShowParallel)
	for i, t := range tags.Models {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var sh ollamaShow
			known := r.ollamaJSON(ctx, http.MethodPost, ollamaRoot(base)+"/api/show", hdr, map[string]string{"model": t.Name}, &sh) == nil
			m, w, reported := modelFromShow(t.Name, sh, known)
			res[i] = found{m: m, window: w, keep: m.ID != "", assumed: !reported}
		}()
	}
	wg.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()
	for q, m := range r.models {
		if m.Provider == "ollama" {
			delete(r.models, q)
		}
	}
	for _, f := range res {
		if !f.keep {
			continue
		}
		r.models[f.m.Qualified()] = f.m
		d.Models = append(d.Models, f.m.ID)
		d.Windows[f.m.ID] = f.window
		if f.assumed {
			d.Assumed = append(d.Assumed, f.m.ID)
		}
	}
	// Declared models win over what the server said.
	if err := r.applyModelOverrides("ollama", p); err != nil {
		return d, err
	}
	slices.Sort(d.Models)
	return d, nil
}

// ollamaJSON sends one native-API request and decodes the JSON answer.
func (r *Registry) ollamaJSON(ctx context.Context, method, url string, hdr http.Header, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return err
	}
	req.Header = hdr.Clone()
	req.Header.Set("User-Agent", "moca/"+config.Version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return newHTTPError(resp)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}

// ollamaUnreachable words a failure to reach the server for a person: what
// was tried and what to check. The message is plain text (not wrapped), so
// the retry ladder does not spend a minute re-dialing a server that is off.
func ollamaUnreachable(base string, err error) error {
	var he *HTTPError
	switch {
	case errors.As(err, &he) && (he.Status == 401 || he.Status == 403):
		return fmt.Errorf("ollama at %s refused the request (HTTP %d) — it needs an API key: /login ollama (or providers.ollama.apiKey)", base, he.Status)
	case errors.As(err, &he):
		return fmt.Errorf("ollama at %s answered HTTP %d — is that an Ollama server? baseUrl must be its OpenAI-compatible root (…/v1)", base, he.Status)
	case errors.Is(err, context.Canceled):
		return err
	}
	cause := err.Error()
	var oe *net.OpError
	if errors.As(err, &oe) && oe.Err != nil {
		cause = oe.Err.Error()
	} else if errors.Is(err, context.DeadlineExceeded) {
		cause = "timed out"
	}
	return fmt.Errorf("cannot reach ollama at %s (%s) — is `ollama serve` running? On another machine, set providers.ollama.baseUrl or OLLAMA_HOST, and start the server with OLLAMA_HOST=0.0.0.0", base, cause)
}

// isDialFailure: the request never reached a server (refused, no route, no
// such host, dial timeout) — as opposed to a connection that broke midway.
func isDialFailure(err error) bool {
	var oe *net.OpError
	if errors.As(err, &oe) && oe.Op == "dial" {
		return true
	}
	var de *net.DNSError
	return errors.As(err, &de)
}

// ollamaGuard words the two failures a local server makes common: it is not
// running (fail at once instead of redialing for a minute) and the model has
// not been pulled.
type ollamaGuard struct {
	inner Adapter
	base  string
}

func (g ollamaGuard) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	resp, err := g.inner.Stream(ctx, req, emit)
	var he *HTTPError
	switch {
	case err == nil:
	case isDialFailure(err):
		return resp, ollamaUnreachable(g.base, err)
	case errors.As(err, &he) && he.Status == http.StatusNotFound && strings.Contains(he.Body, "not found"):
		return resp, fmt.Errorf("%w — pull it first: ollama pull %s", err, req.Model)
	case errors.As(err, &he) && strings.Contains(he.Body, "does not support tools"):
		return resp, fmt.Errorf("%w — pick a model with tool support (ollama.com/search?c=tools)", err)
	}
	return resp, err
}

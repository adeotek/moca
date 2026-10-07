package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// fakeSIWC is a minimal authorization server for the CLI: authorize redirects
// to the loopback callback with an issued client id, token returns an
// RS256-signed ID token carrying the authorize request's nonce.
func fakeSIWC(t *testing.T) provider.OAuthConfig { return fakeSIWCOpts(t, false) }

// fakeSIWCOpts builds the CLI's fake authorization server; stallToken makes
// /token never answer (until the test ends), for the timeout test.
func fakeSIWCOpts(t *testing.T, stallToken bool) provider.OAuthConfig {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var nonce string
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mu.Lock()
		nonce = q.Get("nonce")
		mu.Unlock()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code=C1&client_id=oaiapp_t&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if stallToken {
			<-t.Context().Done()
			return
		}
		mu.Lock()
		n := nonce
		mu.Unlock()
		hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"k"}`))
		pb, _ := json.Marshal(map[string]any{"iss": srv.URL, "aud": "oaiapp_t", "exp": time.Now().Add(time.Hour).Unix(),
			"nonce": n, "sub": "sub-1", "email": "u@example.com"})
		payload := base64.RawURLEncoding.EncodeToString(pb)
		sum := sha256.Sum256([]byte(hdr + "." + payload))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "AT", "refresh_token": "RT", "expires_in": 3600,
			"scope":    "openid profile email offline_access resource.invoke chatgpt.tokens.use.direct",
			"id_token": hdr + "." + payload + "." + base64.RawURLEncoding.EncodeToString(sig),
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":"k","n":%q,"e":%q}]}`,
			base64.RawURLEncoding.EncodeToString(key.N.Bytes()), base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()))
	})
	return provider.OAuthConfig{
		Provider: "openai", AuthorizeURL: srv.URL + "/authorize", TokenURL: srv.URL + "/token", JWKSURL: srv.URL + "/jwks",
		Issuer: srv.URL, Scopes: []string{"openid", "offline_access", "chatgpt.tokens.use.direct"},
		RequiredScopes: []string{"chatgpt.tokens.use.direct"}, RedirectHost: "127.0.0.1", RedirectPath: "/auth/callback",
		DynamicClientID: "dynamic_agent_client", AgentName: "moca",
	}
}

func waitFor(t *testing.T, b *lockedBuf, substr string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if strings.Contains(b.String(), substr) {
			return
		}
	}
	t.Fatalf("output never contained %q: %q", substr, b.String())
}

// `moca login openai --no-browser` where the browser callback still reaches
// the loopback listener (a local machine, or WSL): the paste prompt is armed
// but unused, and the answer to "Switch it now?" must still arrive.
func TestLoginCLISwitchesAuthAfterCallback(t *testing.T) {
	isolate(t)
	oc := fakeSIWC(t)
	old := oauthLookup
	oauthLookup = func(p string) (provider.OAuthConfig, bool) { return oc, p == "openai" }
	defer func() { oauthLookup = old }()
	cfg := writeCfg(t, `{}`)

	pr, pw := io.Pipe()
	var out, errb lockedBuf
	code := make(chan int, 1)
	go func() {
		code <- run(context.Background(), []string{"--config", cfg, "login", "openai", "--no-browser"}, pr, &out, &errb)
	}()
	waitFor(t, &out, "/authorize?")
	var authURL string
	for _, f := range strings.Fields(out.String()) {
		if strings.HasPrefix(f, "http") {
			authURL = f
			break
		}
	}
	resp, err := http.Get(authURL) // the "browser": follows the redirect to the loopback callback
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	waitFor(t, &out, "Switch it now?")
	pw.Write([]byte("y\n"))
	select {
	case c := <-code:
		if c != 0 {
			t.Fatalf("exit %d, stderr %q", c, errb.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("login never finished (the answer was swallowed?): %q", out.String())
	}
	if !strings.Contains(out.String(), "logged in to openai as u@example.com") || !strings.Contains(out.String(), "updated ") {
		t.Fatalf("stdout %q", out.String())
	}
	c, err := config.Load(cfg)
	if err != nil || c.Providers["openai"].Auth != "oauth" {
		t.Fatalf("auth not switched: %v %+v", err, c.Providers["openai"])
	}
	tok, ok, err := provider.NewStore(config.DataDir() + "/auth.json").Get("openai")
	if err != nil || !ok || tok.Access != "AT" || tok.ClientID != "oaiapp_t" {
		t.Fatalf("stored token: %+v %v %v", tok, ok, err)
	}
}

func authURLFrom(t *testing.T, out *lockedBuf) string {
	t.Helper()
	for _, f := range strings.Fields(out.String()) {
		if strings.HasPrefix(f, "http") {
			return f
		}
	}
	t.Fatalf("no authorize URL in output: %q", out.String())
	return ""
}

// A stalled token endpoint must not hang `moca login`: each login HTTP call
// is bounded by the client timeout.
func TestLoginTokenEndpointIsBounded(t *testing.T) {
	isolate(t)
	oc := fakeSIWCOpts(t, true)
	oldLookup := oauthLookup
	oauthLookup = func(p string) (provider.OAuthConfig, bool) { return oc, p == "openai" }
	defer func() { oauthLookup = oldLookup }()
	oldTimeout := loginHTTPTimeout
	loginHTTPTimeout = 100 * time.Millisecond
	defer func() { loginHTTPTimeout = oldTimeout }()
	cfg := writeCfg(t, `{}`)

	var out, errb lockedBuf
	code := make(chan int, 1)
	go func() {
		code <- run(context.Background(), []string{"--config", cfg, "login", "openai", "--no-browser"}, nil, &out, &errb)
	}()
	waitFor(t, &out, "/authorize?")
	resp, err := http.Get(authURLFrom(t, &out)) // the browser: the callback lands, then the token POST stalls
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	select {
	case c := <-code:
		if c != 1 {
			t.Fatalf("stalled token endpoint: exit %d, want 1 (%q)", c, errb.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("login hung on a stalled token endpoint: %q", out.String())
	}
	if !strings.Contains(errb.String(), "Client.Timeout") && !strings.Contains(errb.String(), "deadline exceeded") {
		t.Fatalf("stderr %q", errb.String())
	}
}

// A stalled revoke endpoint must not hang logout: the remote revocation is
// best-effort and bounded; the local registration is cleared either way.
func TestLogoutRevokeIsBounded(t *testing.T) {
	isolate(t)
	stall := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-stall // never answers until the test lets go
	}))
	t.Cleanup(func() { close(stall); srv.Close() })
	oc := provider.OAuthConfig{Provider: "openai", RevokeURL: srv.URL}
	oldLookup := oauthLookup
	oauthLookup = func(p string) (provider.OAuthConfig, bool) { return oc, p == "openai" }
	defer func() { oauthLookup = oldLookup }()
	oldTimeout := revokeTimeout
	revokeTimeout = 100 * time.Millisecond
	defer func() { revokeTimeout = oldTimeout }()
	cfg := writeCfg(t, `{}`)
	store := provider.NewStore(filepath.Join(config.DataDir(), "auth.json"))
	if err := store.Put("openai", provider.Token{Access: "a", Refresh: "r", ClientID: "c", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	var out, errb lockedBuf
	start := time.Now()
	code := run(context.Background(), []string{"--config", cfg, "logout", "openai"}, nil, &out, &errb)
	if code != 0 || time.Since(start) > 5*time.Second {
		t.Fatalf("logout exit %d after %v: %q", code, time.Since(start), errb.String())
	}
	if !strings.Contains(errb.String(), "could not revoke") {
		t.Fatalf("stderr %q", errb.String())
	}
	if _, ok, _ := store.Get("openai"); ok {
		t.Fatal("the local registration must be cleared")
	}
}

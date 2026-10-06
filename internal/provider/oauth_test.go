package provider

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

// fakeAS is a stand-in for auth.openai.com's SIWC endpoints: /authorize
// redirects to the loopback callback with a code, an issued client id and
// the state; /token checks PKCE and answers code and refresh grants;
// /jwks serves the RS256 key the ID tokens are signed with.
type fakeAS struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey

	mu        sync.Mutex
	lastAuth  url.Values
	challenge string
	nonce     string
	lastRev   url.Values

	deny               bool   // authorize redirects with error=access_denied
	badState           bool   // callback carries a wrong state
	omitPlanScope      bool   // token response omits chatgpt.tokens.use.direct
	omitRefresh        bool   // token response omits refresh_token
	omitCallbackClient bool   // callback omits client_id
	callbackClientID   string // override the callback's client_id when non-empty
	extraKey           bool   // JWKS publishes a second RSA key under kid "other"
	otherKey           *rsa.PrivateKey
	jwksExponent       string // when set, the "test" key's e is served verbatim
}

func newFakeAS(t *testing.T) *fakeAS {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAS{t: t, key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f.mu.Lock()
		f.lastAuth = q
		f.challenge = q.Get("code_challenge")
		f.nonce = q.Get("nonce")
		f.mu.Unlock()
		if q.Get("client_id") == "" {
			http.Error(w, "missing client_id", http.StatusBadRequest)
			return
		}
		if f.deny {
			http.Redirect(w, r, q.Get("redirect_uri")+"?error=access_denied&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
			return
		}
		state := q.Get("state")
		if f.badState {
			state = "EVIL"
		}
		clientID := q.Get("client_id")
		if clientID == "dynamic_agent_client" {
			clientID = "oaiapp_test" // dynamic registration issues a new id
		}
		if f.callbackClientID != "" {
			clientID = f.callbackClientID
		}
		loc := q.Get("redirect_uri") + "?code=CODE123&state=" + url.QueryEscape(state)
		if !f.omitCallbackClient {
			loc += "&client_id=" + url.QueryEscape(clientID)
		}
		http.Redirect(w, r, loc, http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") == "refresh_token" {
			switch r.Form.Get("refresh_token") {
			case "RT":
				f.writeToken(w, "AT2", "RT2", r.Form.Get("client_id"))
			case "rot_revoked":
				w.WriteHeader(400)
				w.Write([]byte(`{"error":"refresh_token_reused"}`))
			case "dead":
				w.WriteHeader(400)
				w.Write([]byte(`{"error":"invalid_grant"}`))
			case "badclient":
				w.WriteHeader(400)
				w.Write([]byte(`{"error":"invalid_client"}`))
			default:
				w.WriteHeader(400)
				w.Write([]byte(`{"error":"invalid_grant"}`))
			}
			return
		}
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		f.mu.Lock()
		ch := f.challenge
		f.mu.Unlock()
		if base64.RawURLEncoding.EncodeToString(sum[:]) != ch || r.Form.Get("code") != "CODE123" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"pkce"}`))
			return
		}
		f.writeToken(w, "AT", "RT", r.Form.Get("client_id"))
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(f.key.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.key.E)).Bytes())
		if f.jwksExponent != "" {
			e = f.jwksExponent
		}
		keys := fmt.Sprintf(`{"kty":"RSA","kid":"test","alg":"RS256","use":"sig","n":%q,"e":%q}`, n, e)
		if f.extraKey && f.otherKey != nil {
			on := base64.RawURLEncoding.EncodeToString(f.otherKey.N.Bytes())
			oe := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.otherKey.E)).Bytes())
			keys += fmt.Sprintf(`,{"kty":"RSA","kid":"other","alg":"RS256","use":"sig","n":%q,"e":%q}`, on, oe)
		}
		fmt.Fprintf(w, `{"keys":[%s]}`, keys)
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		f.mu.Lock()
		f.lastRev = r.Form
		f.mu.Unlock()
		w.WriteHeader(200)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAS) writeToken(w http.ResponseWriter, access, refresh, clientID string) {
	f.mu.Lock()
	nonce := f.nonce
	f.mu.Unlock()
	scope := "openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"
	if f.omitPlanScope {
		scope = "openid profile email offline_access resource.invoke"
	}
	body := map[string]any{
		"access_token": access, "token_type": "Bearer", "expires_in": 3600,
		"scope": scope, "id_token": f.idToken(clientID, nonce),
	}
	if !f.omitRefresh {
		body["refresh_token"] = refresh
	}
	json.NewEncoder(w).Encode(body)
}

func (f *fakeAS) idToken(clientID, nonce string) string {
	return f.signID(map[string]any{
		"iss": f.srv.URL, "aud": clientID, "exp": time.Now().Add(time.Hour).Unix(),
		"nonce": nonce, "sub": "sub-1", "email": "user@example.com",
	})
}

// enableExtraKey makes the JWKS serve a second RSA key under kid "other",
// so key selection is exercised against a rotating multi-key set.
func (f *fakeAS) enableExtraKey(t *testing.T) {
	t.Helper()
	if f.otherKey == nil {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		f.otherKey = k
	}
	f.extraKey = true
}

func (f *fakeAS) signID(claims map[string]any) string {
	return f.signIDWithKey(f.key, "test", claims)
}

func (f *fakeAS) signIDWithKey(key *rsa.PrivateKey, kid string, claims map[string]any) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"alg":"RS256","kid":%q}`, kid)))
	pb, err := json.Marshal(claims)
	if err != nil {
		f.t.Fatal(err)
	}
	payload := base64.RawURLEncoding.EncodeToString(pb)
	sum := sha256.Sum256([]byte(hdr + "." + payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		f.t.Fatal(err)
	}
	return hdr + "." + payload + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (f *fakeAS) config() OAuthConfig {
	return OAuthConfig{
		Provider:     "openai",
		AuthorizeURL: f.srv.URL + "/authorize",
		TokenURL:     f.srv.URL + "/token",
		RevokeURL:    f.srv.URL + "/revoke",
		JWKSURL:      f.srv.URL + "/jwks",
		Issuer:       f.srv.URL,
		Scopes: []string{"openid", "profile", "email", "offline_access",
			"resource.invoke", "chatgpt.tokens.use.direct"},
		RequiredScopes:  []string{"chatgpt.tokens.use.direct"},
		Resource:        "https://api.openai.com/v1",
		RedirectHost:    "127.0.0.1",
		RedirectPath:    "/auth/callback",
		DynamicClientID: "dynamic_agent_client",
		AgentName:       "moca",
		HostID:          "urn:uuid:test-host",
	}
}

func (f *fakeAS) authParams() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastAuth
}

func (f *fakeAS) revParams() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastRev
}

// browse simulates the user's browser: it visits the authorize URL and
// follows the redirect to moca's loopback callback.
func browse(u string) error {
	go func() {
		resp, err := http.Get(u)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()
	return nil
}

func runLogin(t *testing.T, f *fakeAS, saved *Token, lio LoginIO) (Token, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if lio.Out == nil {
		lio.Out = io.Discard
	}
	if lio.In == nil {
		lio.In = strings.NewReader("")
	}
	if lio.WaitBeforePaste == 0 {
		lio.WaitBeforePaste = 5 * time.Second
	}
	return Login(ctx, f.config(), saved, lio, f.srv.Client())
}

func TestLoginLocalCallback(t *testing.T) {
	f := newFakeAS(t)
	tok, err := runLogin(t, f, nil, LoginIO{OpenURL: browse})
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "AT" || tok.Refresh != "RT" || tok.ClientID != "oaiapp_test" ||
		tok.Email != "user@example.com" || tok.AccountID != "sub-1" {
		t.Fatalf("%+v", tok)
	}
	if time.Until(tok.Expiry) < 50*time.Minute {
		t.Fatal("expiry", tok.Expiry)
	}
	q := f.authParams()
	if q.Get("client_id") != "dynamic_agent_client" || q.Get("agent_name_hint") != "moca" ||
		q.Get("ext_agent_host_id") != "urn:uuid:test-host" {
		t.Fatalf("registration params: %v", q)
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" ||
		q.Get("state") == "" || q.Get("nonce") == "" || q.Get("response_type") != "code" {
		t.Fatalf("pkce params: %v", q)
	}
	if q.Get("resource") != "https://api.openai.com/v1" ||
		!strings.Contains(q.Get("scope"), "chatgpt.tokens.use.direct") {
		t.Fatalf("resource/scope: %v", q)
	}
	if !strings.HasPrefix(q.Get("redirect_uri"), "http://127.0.0.1:") ||
		!strings.HasSuffix(q.Get("redirect_uri"), "/auth/callback") {
		t.Fatalf("redirect: %v", q.Get("redirect_uri"))
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func firstURL(s string) string {
	for _, f := range strings.Fields(s) {
		if strings.HasPrefix(f, "http") {
			return f
		}
	}
	return ""
}

func TestLoginHeadlessPaste(t *testing.T) {
	f := newFakeAS(t)
	var out syncBuf
	pr, pw := io.Pipe()
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), "/authorize?") {
			if time.Now().After(deadline) {
				pw.CloseWithError(errors.New("authorize URL never printed"))
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		authURL := firstURL(out.String())
		cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := cl.Get(authURL)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		resp.Body.Close()
		pw.Write([]byte(resp.Header.Get("Location") + "\n"))
		pw.Close()
	}()
	tok, err := runLogin(t, f, nil, LoginIO{Out: &out, In: pr, Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "AT" || tok.ClientID != "oaiapp_test" {
		t.Fatalf("%+v", tok)
	}
}

func TestParsePasted(t *testing.T) {
	if c, err := parsePasted("http://127.0.0.1:5/auth/callback?code=ABC&state=S1&client_id=oaiapp_z", "S1"); err != nil || c.Code != "ABC" || c.ClientID != "oaiapp_z" {
		t.Fatal(c, err)
	}
	if _, err := parsePasted("http://127.0.0.1:5/auth/callback?code=ABC&state=EVIL", "S1"); err == nil {
		t.Fatal("state mismatch refused")
	}
	if c, _ := parsePasted("  ABC  ", "S1"); c.Code != "ABC" {
		t.Fatal("bare code")
	}
	if c, err := parsePasted("ABC#S1", "S1"); err != nil || c.Code != "ABC" {
		t.Fatal("code#state")
	}
	if _, err := parsePasted("http://x/cb?error=access_denied&state=S1", "S1"); err == nil {
		t.Fatal("denied refused")
	}
	if _, err := parsePasted("http://x/cb?state=S1", "S1"); err == nil {
		t.Fatal("missing code refused")
	}
	if _, err := parsePasted("", "S1"); err == nil {
		t.Fatal("empty refused")
	}
	// A redirect URL that lost its scheme (terminal wrap, hand copy) is
	// recovered, and the state check still applies to it.
	if c, err := parsePasted("127.0.0.1:5/auth/callback?code=ABC&state=S1&client_id=oaiapp_z", "S1"); err != nil || c.Code != "ABC" || c.ClientID != "oaiapp_z" {
		t.Fatal(c, err)
	}
	if c, err := parsePasted("localhost:5/auth/callback?code=ABC&state=S1", "S1"); err != nil || c.Code != "ABC" {
		t.Fatal(c, err)
	}
	if _, err := parsePasted("127.0.0.1:5/auth/callback?code=ABC&state=EVIL", "S1"); err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatal("recovered URL must still verify the state", err)
	}
	// Not recoverable: a URL fragment with no host is named, not exchanged.
	if _, err := parsePasted("code=ABC", "S1"); err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Fatal("scheme-less fragment refused", err)
	}
}

func TestLoginReauthReusesSavedRegistration(t *testing.T) {
	f := newFakeAS(t)
	f.omitCallbackClient = true // reauthorization callbacks may omit client_id
	saved := &Token{ClientID: "oaiapp_saved", IDToken: "hint-token", Email: "u@e.com"}
	tok, err := runLogin(t, f, saved, LoginIO{OpenURL: browse})
	if err != nil {
		t.Fatal(err)
	}
	if tok.ClientID != "oaiapp_saved" {
		t.Fatalf("%+v", tok)
	}
	q := f.authParams()
	if q.Get("client_id") != "oaiapp_saved" || q.Get("id_token_hint") != "hint-token" || q.Get("login_hint") != "u@e.com" {
		t.Fatalf("reauth params: %v", q)
	}
	if q.Get("agent_name_hint") != "" {
		t.Fatalf("reauth must omit agent_name_hint: %v", q)
	}
}

func TestLoginReauthRejectsDifferentClientID(t *testing.T) {
	f := newFakeAS(t)
	f.callbackClientID = "oaiapp_other"
	saved := &Token{ClientID: "oaiapp_saved"}
	_, err := runLogin(t, f, saved, LoginIO{OpenURL: browse})
	if err == nil || !strings.Contains(err.Error(), "different client id") {
		t.Fatal(err)
	}
}

func TestLoginRegistrationIncomplete(t *testing.T) {
	f := newFakeAS(t)
	f.omitCallbackClient = true
	_, err := runLogin(t, f, nil, LoginIO{OpenURL: browse})
	if err == nil || !strings.Contains(err.Error(), "registration incomplete") {
		t.Fatal(err)
	}
}

func TestLoginAccessDenied(t *testing.T) {
	f := newFakeAS(t)
	f.deny = true
	_, err := runLogin(t, f, nil, LoginIO{OpenURL: browse})
	if err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatal(err)
	}
}

func TestLoginStateMismatch(t *testing.T) {
	f := newFakeAS(t)
	f.badState = true
	_, err := runLogin(t, f, nil, LoginIO{OpenURL: browse})
	if err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatal(err)
	}
}

func TestLoginDeniedPlanScope(t *testing.T) {
	f := newFakeAS(t)
	f.omitPlanScope = true
	_, err := runLogin(t, f, nil, LoginIO{OpenURL: browse})
	if err == nil || !strings.Contains(err.Error(), "not granted") || !strings.Contains(err.Error(), "ChatGPT plan") {
		t.Fatal(err)
	}
}

func TestLoginNoRefreshToken(t *testing.T) {
	f := newFakeAS(t)
	f.omitRefresh = true
	_, err := runLogin(t, f, nil, LoginIO{OpenURL: browse})
	if err == nil || !strings.Contains(err.Error(), "refresh token") {
		t.Fatal(err)
	}
}

func TestRefresher(t *testing.T) {
	f := newFakeAS(t)
	r := f.config().refresher(f.srv.Client())
	tok, err := r(context.Background(), Token{Refresh: "RT", ClientID: "oaiapp_test"})
	if err != nil || tok.Access != "AT2" || tok.Refresh != "RT2" {
		t.Fatal(tok, err)
	}
	if _, err := r(context.Background(), Token{Refresh: "dead", ClientID: "oaiapp_test"}); !errors.Is(err, ErrInvalidGrant) {
		t.Fatal(err)
	}
	if _, err := r(context.Background(), Token{Refresh: "rot_revoked", ClientID: "oaiapp_test"}); !errors.Is(err, ErrInvalidGrant) {
		t.Fatal(err)
	}
	if _, err := r(context.Background(), Token{Refresh: "badclient", ClientID: "oaiapp_test"}); !errors.Is(err, ErrInvalidClient) {
		t.Fatal(err)
	}
}

func TestRevoke(t *testing.T) {
	f := newFakeAS(t)
	if err := Revoke(context.Background(), f.config(), Token{Refresh: "RT", ClientID: "oaiapp_test"}, f.srv.Client()); err != nil {
		t.Fatal(err)
	}
	q := f.revParams()
	if q.Get("token") != "RT" || q.Get("token_type_hint") != "refresh_token" || q.Get("client_id") != "oaiapp_test" {
		t.Fatalf("revoke params: %v", q)
	}
}

func TestValidateIDToken(t *testing.T) {
	f := newFakeAS(t)
	hc := f.srv.Client()
	ctx := context.Background()
	good := f.idToken("oaiapp_test", "N1")
	cl, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", good)
	if err != nil || cl.Sub != "sub-1" || cl.Email != "user@example.com" {
		t.Fatal(cl, err)
	}
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "WRONG", good); err == nil {
		t.Fatal("nonce mismatch not caught")
	}
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "other-client", "N1", good); err == nil {
		t.Fatal("audience mismatch not caught")
	}
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", good[:len(good)-4]+"AAAA"); err == nil {
		t.Fatal("bad signature not caught")
	}
	expired := f.signID(map[string]any{"iss": f.srv.URL, "aud": "oaiapp_test", "exp": time.Now().Add(-time.Hour).Unix(), "nonce": "N1", "sub": "s"})
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", expired); err == nil {
		t.Fatal("expired not caught")
	}
	wrongIss := f.signID(map[string]any{"iss": "https://evil.example", "aud": "oaiapp_test", "exp": time.Now().Add(time.Hour).Unix(), "nonce": "N1", "sub": "s"})
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", wrongIss); err == nil {
		t.Fatal("issuer mismatch not caught")
	}
}

func TestHeadlessDetectionSSH(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "10.0.0.1 22 10.0.0.2 12345")
	if !Headless() {
		t.Fatal("SSH session must be headless")
	}
}

// Two keys in the JWKS: the token's kid selects the verification key, and a
// mismatched kid never falls back to another RSA key.
func TestValidateIDTokenKidSelection(t *testing.T) {
	f := newFakeAS(t)
	f.enableExtraKey(t)
	hc := f.srv.Client()
	ctx := context.Background()
	claims := func() map[string]any {
		return map[string]any{"iss": f.srv.URL, "aud": "oaiapp_test", "exp": time.Now().Add(time.Hour).Unix(), "nonce": "N1", "sub": "s"}
	}
	// Signed by the second key, labelled with its own kid: verifies.
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", f.signIDWithKey(f.otherKey, "other", claims())); err != nil {
		t.Fatalf("second-key token must verify: %v", err)
	}
	// Signed by the first key but labelled "other": must not verify.
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", f.signIDWithKey(f.key, "other", claims())); err == nil {
		t.Fatal("kid mismatch not caught")
	}
	// An unknown kid is refused outright.
	if _, err := validateIDToken(ctx, hc, f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", f.signIDWithKey(f.key, "nope", claims())); err == nil {
		t.Fatal("unknown kid not caught")
	}
}

// A JWKS key with a nonsense public exponent is skipped, not used: the
// whole token is refused rather than verified against a bogus modulus.
func TestJWKSExponentSanity(t *testing.T) {
	f := newFakeAS(t)
	f.jwksExponent = "AA" // e = 0
	if _, err := jwksKey(context.Background(), f.srv.Client(), f.srv.URL+"/jwks", "test"); err == nil || !strings.Contains(err.Error(), "no RSA key matches") {
		t.Fatalf("bogus exponent accepted: %v", err)
	}
	if _, err := validateIDToken(context.Background(), f.srv.Client(), f.srv.URL+"/jwks", f.srv.URL, "oaiapp_test", "N1", f.idToken("oaiapp_test", "N1")); err == nil {
		t.Fatal("token verified against a zero exponent")
	}
}

// state and nonce must come from fresh entropy on every attempt: a constant
// (or empty) source must fail this, not silently degrade CSRF protection.
func TestPKCEUniqueness(t *testing.T) {
	v1, c1, s1, n1, err := pkce()
	if err != nil {
		t.Fatal(err)
	}
	v2, c2, s2, n2, err := pkce()
	if err != nil {
		t.Fatal(err)
	}
	if v1 == "" || s1 == "" || n1 == "" || s1 == s2 || n1 == n2 || v1 == v2 || c1 == c2 {
		t.Fatalf("pkce must be unique per attempt: %q/%q %q/%q %q/%q", v1, v2, s1, s2, n1, n2)
	}
	sum := sha256.Sum256([]byte(v1))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != c1 {
		t.Fatal("challenge is not S256(verifier)")
	}
}

// An entropy failure aborts the login instead of continuing with empty
// state/nonce values. The first read (the PKCE verifier) succeeds; every
// later one fails, so this pins the randText path specifically.
func TestLoginFailsOnEntropyError(t *testing.T) {
	f := newFakeAS(t)
	old := randRead
	n := 0
	randRead = func(b []byte) (int, error) {
		n++
		if n == 1 {
			return old(b)
		}
		return 0, errors.New("rng failed")
	}
	defer func() { randRead = old }()
	_, err := runLogin(t, f, nil, LoginIO{OpenURL: browse})
	if err == nil || !strings.Contains(err.Error(), "entropy") {
		t.Fatalf("entropy failure must abort the login: %v", err)
	}
}

// config.OAuthProviders (what config validation permits) and the endpoint
// table here must stay the same list: a provider that validates but has no
// flow fails at request time, and a flow that config rejects is dead code.
func TestOAuthProviderListsStayInSync(t *testing.T) {
	var fromTable []string
	for name := range oauthProviders {
		fromTable = append(fromTable, name)
	}
	slices.Sort(fromTable)
	want := slices.Clone(config.OAuthProviders)
	slices.Sort(want)
	if !slices.Equal(fromTable, want) {
		t.Fatalf("config.OAuthProviders %v != oauthProviders %v", want, fromTable)
	}
}

// A paste that arrives after the browser callback answered is ignored: the
// outcome is decided by the first response, deterministically.
func TestLoginLatePasteIgnored(t *testing.T) {
	f := newFakeAS(t)
	var out syncBuf
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), "/authorize?") {
			if time.Now().After(deadline) {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		authURL := firstURL(out.String())
		cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := cl.Get(authURL)
		if err != nil {
			return
		}
		loc := resp.Header.Get("Location")
		resp.Body.Close()
		if _, err := http.Get(loc); err != nil { // the browser callback answers first
			return
		}
		time.Sleep(50 * time.Millisecond) // let the callback result land
		pw.Write([]byte("http://127.0.0.1:5/auth/callback?code=WRONG&state=EVIL\n"))
		pw.Close()
	}()
	tok, err := runLogin(t, f, nil, LoginIO{Out: &out, In: pr, Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "AT" {
		t.Fatalf("%+v", tok)
	}
	<-done
}

// A 429/5xx from the token endpoint is an *HTTPError the retry layer
// understands (honouring Retry-After); a 400 is not retried.
func TestTokenEndpointTransientIsRetryable(t *testing.T) {
	for _, tc := range []struct {
		status int
		retry  bool
	}{{503, true}, {429, true}, {400, false}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(tc.status)
			w.Write([]byte(`{"error":"temporarily_unavailable"}`))
		}))
		_, err := OAuthConfig{TokenURL: srv.URL}.refresher(srv.Client())(context.Background(), Token{Refresh: "RT", ClientID: "c"})
		srv.Close()
		var he *HTTPError
		if !errors.As(err, &he) || he.Status != tc.status || retryable(err) != tc.retry {
			t.Fatalf("status %d: err=%v retryable=%v want %v", tc.status, err, retryable(err), tc.retry)
		}
		if tc.retry && he.RetryAfter != 7*time.Second {
			t.Fatalf("Retry-After lost: %v", he.RetryAfter)
		}
	}
}

// The line reader hands a line to whoever asks next when an earlier consumer
// walked away: nothing is left blocked on the input stealing it.
func TestLineReaderAbandonedReadKeepsLine(t *testing.T) {
	pr, pw := io.Pipe()
	lr := NewLineReader(pr)
	done := make(chan struct{})
	res := make(chan error, 1)
	go func() { _, err := lr.Next(done); res <- err }()
	time.Sleep(30 * time.Millisecond)
	close(done)
	if err := <-res; !errors.Is(err, ErrLineAbandoned) {
		t.Fatalf("abandoned Next: %v", err)
	}
	go pw.Write([]byte("y\n"))
	got := make(chan string, 1)
	go func() { s, _ := lr.Next(nil); got <- s }()
	select {
	case s := <-got:
		if s != "y\n" {
			t.Fatalf("got %q", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the line was lost to the abandoned reader")
	}
}

// The browser callback wins after the paste prompt was armed: the line typed
// next (moca login's "switch auth?" answer) must reach the next consumer.
func TestLoginCallbackWinLeavesNextLineForCaller(t *testing.T) {
	f := newFakeAS(t)
	pr, pw := io.Pipe()
	lines := NewLineReader(pr)
	open := func(u string) error { // the callback lands after the paste read started
		go func() { time.Sleep(200 * time.Millisecond); browse(u) }()
		return nil
	}
	var out syncBuf
	if _, err := runLogin(t, f, nil, LoginIO{Out: &out, Lines: lines, OpenURL: open, WaitBeforePaste: 20 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Still waiting") {
		t.Fatalf("the paste prompt was never armed; the test does not cover the leak: %q", out.String())
	}
	go pw.Write([]byte("y\n"))
	got := make(chan string, 1)
	go func() { s, _ := lines.Next(nil); got <- s }()
	select {
	case s := <-got:
		if s != "y\n" {
			t.Fatalf("got %q", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the answer line was swallowed by the paste reader")
	}
}

// Review Focus 5: the browser cannot be opened → the URL is still printed,
// the failure is named, and after WaitBeforePaste a paste completes the login.
func TestLoginOpenFailureThenPaste(t *testing.T) {
	f := newFakeAS(t)
	var out syncBuf
	pr, pw := io.Pipe()
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), "Still waiting") {
			if time.Now().After(deadline) {
				pw.CloseWithError(errors.New("paste prompt never shown"))
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := cl.Get(firstURL(out.String()))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		resp.Body.Close()
		pw.Write([]byte(resp.Header.Get("Location") + "\n"))
	}()
	noBrowser := func(string) error { return errors.New("xdg-open: not found") }
	tok, err := runLogin(t, f, nil, LoginIO{Out: &out, In: pr, OpenURL: noBrowser, WaitBeforePaste: 50 * time.Millisecond})
	if err != nil || tok.Access != "AT" {
		t.Fatalf("%+v %v", tok, err)
	}
	s := out.String()
	if !strings.Contains(s, "could not open a browser: xdg-open: not found") || !strings.Contains(s, "/authorize?") {
		t.Fatalf("URL and the open failure must be printed: %q", s)
	}
}

// The loopback page must not claim success when the login is refused.
func TestCallbackPageReportsFailure(t *testing.T) {
	for name, setup := range map[string]func(*fakeAS){
		"declined":       func(f *fakeAS) { f.deny = true },
		"state mismatch": func(f *fakeAS) { f.badState = true },
	} {
		f := newFakeAS(t)
		setup(f)
		page := make(chan string, 1)
		open := func(u string) error {
			go func() {
				resp, err := http.Get(u)
				if err != nil {
					page <- err.Error()
					return
				}
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				page <- fmt.Sprintf("%d %s", resp.StatusCode, b)
			}()
			return nil
		}
		if _, err := runLogin(t, f, nil, LoginIO{OpenURL: open}); err == nil {
			t.Fatalf("%s: login must fail", name)
		}
		select {
		case p := <-page:
			if !strings.HasPrefix(p, "400 ") || strings.Contains(p, "Login complete") {
				t.Fatalf("%s: callback page = %q", name, p)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s: no callback page", name)
		}
	}
}

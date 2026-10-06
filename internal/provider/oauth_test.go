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
	"strings"
	"sync"
	"testing"
	"time"
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
		fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":"test","alg":"RS256","use":"sig","n":%q,"e":%q}]}`, n, e)
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

func (f *fakeAS) signID(claims map[string]any) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test"}`))
	pb, err := json.Marshal(claims)
	if err != nil {
		f.t.Fatal(err)
	}
	payload := base64.RawURLEncoding.EncodeToString(pb)
	sum := sha256.Sum256([]byte(hdr + "." + payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, sum[:])
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

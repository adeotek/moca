package provider

// OAuth login for subscription providers. The only provider that ships OAuth
// is openai, via the documented "Sign in with ChatGPT" open-source token
// sharing flow (docs/specs/oauth-verification.md): dynamic client
// registration (no secret), loopback redirect on 127.0.0.1, PKCE S256,
// nonce + RS256 ID token validation against the vendor JWKS, rotating
// 30-day refresh tokens. Anthropic ships api_key only by policy.

import (
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
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// OAuthConfig carries one provider's OAuth parameters. Every value comes
// from docs/specs/oauth-verification.md (openai / Sign in with ChatGPT).
type OAuthConfig struct {
	Provider     string // for messages ("openai")
	AuthorizeURL string
	TokenURL     string
	RevokeURL    string
	JWKSURL      string
	Issuer       string // expected `iss` in the ID token
	Scopes       []string
	// RequiredScopes must all be present in the granted scopes for the
	// login to be useful (openai: chatgpt.tokens.use.direct).
	RequiredScopes []string
	Resource       string // RFC 8707 resource parameter
	RedirectHost   string // "127.0.0.1" — the flow rejects localhost
	RedirectPath   string // "/auth/callback"
	FixedPort      int    // 0 = any free port (only the port may vary)
	// DynamicClientID is the first-registration placeholder
	// (openai: "dynamic_agent_client"); the callback then issues the real
	// client id, which the store keeps per registration.
	DynamicClientID string
	AgentName       string // agent_name_hint on first registration
	HostID          string // ext_agent_host_id for this host
}

// LoginIO is the interactive surface of Login: where to print, where a
// pasted code comes from, how to open a browser, and whether the local
// callback is unreachable (SSH/container/WSL/no display).
type LoginIO struct {
	Out io.Writer
	In  io.Reader
	// Lines, when set, is the paste source instead of In. A caller that
	// prompts again after Login (moca login's "switch auth?") shares one
	// LineReader so an unanswered paste read never swallows that answer.
	Lines           *LineReader
	OpenURL         func(string) error // nil → don't try
	Headless        bool
	WaitBeforePaste time.Duration // default 120s
}

type pastedResult struct {
	Code     string
	ClientID string
	Err      error
}

// Login runs the authorization-code + PKCE flow and returns the fresh token
// (with the issued client id and validated account identity). saved is the
// stored registration for this provider, or nil for a first-time dynamic
// registration. Nothing is persisted here — the caller writes the store.
func Login(ctx context.Context, c OAuthConfig, saved *Token, lio LoginIO, hc *http.Client) (Token, error) {
	if lio.Out == nil {
		lio.Out = io.Discard
	}
	if lio.WaitBeforePaste <= 0 {
		lio.WaitBeforePaste = 120 * time.Second
	}
	verifier, challenge, state, nonce, err := pkce()
	if err != nil {
		return Token{}, err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(c.RedirectHost, strconv.Itoa(c.FixedPort)))
	if err != nil {
		return Token{}, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://%s%s", net.JoinHostPort(c.RedirectHost, strconv.Itoa(port)), c.RedirectPath)
	authURL := c.authorizeURL(redirectURI, challenge, state, nonce, saved)

	resCh := make(chan pastedResult, 1)
	done := make(chan struct{})
	defer close(done)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != c.RedirectPath {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		res := pastedResult{Code: q.Get("code"), ClientID: q.Get("client_id")}
		switch {
		case q.Get("error") != "":
			res = pastedResult{Err: fmt.Errorf("authorization was declined (%s)", q.Get("error"))}
		case q.Get("state") != state:
			res = pastedResult{Err: errors.New("state mismatch in the login callback")}
		case res.Code == "":
			res = pastedResult{Err: errors.New("the login callback carried no authorization code")}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if res.Err != nil {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "<html><body>Login failed — return to the terminal for details.</body></html>")
		} else {
			io.WriteString(w, "<html><body>Login complete — you can close this tab.</body></html>")
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case resCh <- res:
		default: // a duplicate (paste + callback): the first one wins
		}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	lines := lio.Lines
	if lines == nil && lio.In != nil {
		lines = NewLineReader(lio.In)
	}
	if lines != nil {
		go pasteReader(lio, lines, state, resCh, done)
	}

	if lio.Headless {
		fmt.Fprintf(lio.Out, "Visit this URL to sign in:\n%s\n\nPaste the code (or the full redirect URL):\n", authURL)
	} else {
		fmt.Fprintf(lio.Out, "Opening your browser to sign in...\nIf it does not open, visit:\n%s\n", authURL)
		if lio.OpenURL != nil {
			if err := lio.OpenURL(authURL); err != nil {
				fmt.Fprintf(lio.Out, "(could not open a browser: %v)\n", err)
			}
		}
	}

	var res pastedResult
	select {
	case res = <-resCh:
	case <-ctx.Done():
		return Token{}, ctx.Err()
	}
	if res.Err != nil {
		return Token{}, res.Err
	}

	// The registration leg depends on the authorize redirect carrying the
	// issued client id (docs/specs/oauth-verification.md §2.2): the
	// placeholder id is never used for token exchange, the issued id is what
	// refresh and revoke must carry later, and a reauthorization callback
	// returning a different id must not replace the saved registration. A
	// first-registration callback without the field cannot be completed —
	// refuse loudly rather than inventing one.
	clientID := res.ClientID
	switch {
	case saved == nil:
		if clientID == "" {
			return Token{}, fmt.Errorf("registration incomplete: the callback did not return an issued client id; run `moca login %s` again", c.Provider)
		}
	case clientID == "":
		clientID = saved.ClientID
	case clientID != saved.ClientID:
		return Token{}, fmt.Errorf("the callback returned a different client id than the saved registration; refusing to replace it — run `moca logout %s` first to re-register", c.Provider)
	}

	tok, err := c.exchange(ctx, hc, res.Code, clientID, redirectURI, verifier)
	if err != nil {
		return Token{}, err
	}
	claims, err := validateIDToken(ctx, hc, c.JWKSURL, c.Issuer, clientID, nonce, tok.IDToken)
	if err != nil {
		return Token{}, fmt.Errorf("verifying the login: %w", err)
	}
	if missing := missingScopes(tok.Scopes, c.RequiredScopes); len(missing) > 0 {
		return Token{}, fmt.Errorf("plan usage was not granted (%s missing): run `moca login %s` again and allow 'Use your ChatGPT plan'",
			strings.Join(missing, ", "), c.Provider)
	}
	if tok.Refresh == "" {
		return Token{}, fmt.Errorf("the login response carried no refresh token: run `moca login %s` again and approve offline access", c.Provider)
	}
	tok.ClientID = clientID
	tok.AccountID = claims.Sub
	tok.Email = claims.Email
	return tok, nil
}

// pasteReader reads one line from lines and forwards the parsed result.
// In headless mode it starts immediately (the prompt is already printed);
// otherwise it waits WaitBeforePaste for the callback first.
func pasteReader(lio LoginIO, lines *LineReader, state string, ch chan<- pastedResult, done <-chan struct{}) {
	if !lio.Headless {
		select {
		case <-time.After(lio.WaitBeforePaste):
		case <-done:
			return
		}
		fmt.Fprintln(lio.Out, "Still waiting for the browser — paste the code (or the full redirect URL) here:")
	}
	line, err := lines.Next(done)
	if err != nil && strings.TrimSpace(line) == "" {
		return // EOF, or Login finished first: the read stays for the next consumer
	}
	res, perr := parsePasted(line, state)
	if perr != nil {
		res = pastedResult{Err: perr}
	}
	// One shared channel, first result wins: a paste that arrives after the
	// browser callback answered is dropped (the buffered slot is taken), so
	// the outcome never depends on a two-channel select race.
	select {
	case ch <- res:
	default:
	}
}

func (c OAuthConfig) authorizeURL(redirectURI, challenge, state, nonce string, saved *Token) string {
	q := url.Values{}
	if saved == nil {
		q.Set("client_id", c.DynamicClientID)
		q.Set("agent_name_hint", c.AgentName)
	} else {
		q.Set("client_id", saved.ClientID)
		if saved.IDToken != "" {
			q.Set("id_token_hint", saved.IDToken)
		}
		if saved.Email != "" {
			q.Set("login_hint", saved.Email)
		}
	}
	q.Set("ext_agent_host_id", c.HostID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", strings.Join(c.Scopes, " "))
	if c.Resource != "" {
		q.Set("resource", c.Resource)
	}
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	u, err := url.Parse(c.AuthorizeURL)
	if err != nil {
		return c.AuthorizeURL
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c OAuthConfig) exchange(ctx context.Context, hc *http.Client, code, clientID, redirectURI, verifier string) (Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	}
	if c.Resource != "" {
		form.Set("resource", c.Resource)
	}
	return c.tokenRequest(ctx, hc, form)
}

// refresher returns the Refresher the store calls near expiry. The refresh
// grant carries the issued client id and the resource; scope is omitted so
// the original grant is retained. Rotating refresh tokens are replaced by
// the store's locked read → refresh → write.
func (c OAuthConfig) refresher(hc *http.Client) Refresher {
	return func(ctx context.Context, t Token) (Token, error) {
		form := url.Values{
			"grant_type":    {"refresh_token"},
			"client_id":     {t.ClientID},
			"refresh_token": {t.Refresh},
		}
		if c.Resource != "" {
			form.Set("resource", c.Resource)
		}
		return c.tokenRequest(ctx, hc, form)
	}
}

func (c OAuthConfig) tokenRequest(ctx context.Context, hc *http.Client, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return Token{}, err
	}
	return tokenFrom(resp)
}

// tokenFrom parses a token-endpoint response, mapping the terminal OAuth
// error codes to the store's sentinels (the SIWC refresh-error list).
func tokenFrom(resp *http.Response) (Token, error) {
	defer resp.Body.Close()
	var body struct {
		Access    string `json:"access_token"`
		Refresh   string `json:"refresh_token"`
		IDToken   string `json:"id_token"`
		Expires   int    `json:"expires_in"`
		Scope     string `json:"scope"`
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body)
	switch body.Error {
	case "invalid_grant", "invalid_refresh_token", "token_expired",
		"refresh_token_expired", "refresh_token_invalidated", "refresh_token_reused":
		return Token{}, ErrInvalidGrant
	case "invalid_client":
		return Token{}, ErrInvalidClient
	}
	if resp.StatusCode/100 != 2 || body.Access == "" {
		msg := body.Error
		if body.ErrorDesc != "" {
			msg = strings.TrimSpace(msg + ": " + body.ErrorDesc)
		}
		// An *HTTPError so the retry layer treats a 429/5xx from the token
		// endpoint like one from the inference endpoint.
		return Token{}, fmt.Errorf("token endpoint: %w", &HTTPError{Status: resp.StatusCode, Body: msg, RetryAfter: retryAfter(resp.Header)})
	}
	t := Token{Access: body.Access, Refresh: body.Refresh, IDToken: body.IDToken,
		Expiry: time.Now().Add(time.Duration(body.Expires) * time.Second)}
	if body.Scope != "" {
		t.Scopes = strings.Fields(body.Scope)
	}
	return t, nil
}

// Revoke ends the renewable session at the provider (logout). Best-effort:
// the caller clears the local token regardless. An empty 200 is success,
// including for an already-invalid token.
func Revoke(ctx context.Context, c OAuthConfig, t Token, hc *http.Client) error {
	if c.RevokeURL == "" || t.Refresh == "" {
		return nil
	}
	form := url.Values{"token": {t.Refresh}, "token_type_hint": {"refresh_token"}}
	if t.ClientID != "" {
		form.Set("client_id", t.ClientID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("revocation endpoint: HTTP %d", resp.StatusCode)
	}
	return nil
}

func pkce() (verifier, challenge, state, nonce string, err error) {
	b := make([]byte, 32)
	if _, err = randRead(b); err != nil {
		return
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	if state, err = randText(16); err != nil {
		return
	}
	if nonce, err = randText(16); err != nil {
		return
	}
	return
}

// randRead is crypto/rand.Read, indirected so tests can force the
// entropy-failure path: an entropy failure must abort the login, never
// continue with empty anti-replay values (state and nonce are the CSRF and
// ID-token mix-up defenses).
var randRead = rand.Read

func randText(n int) (string, error) {
	b := make([]byte, n)
	if _, err := randRead(b); err != nil {
		return "", fmt.Errorf("reading entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// parsePasted accepts a bare code, "code#state", or a redirect URL (the
// SSH/headless case: the browser lands on an unreachable loopback and the
// user copies the URL back). The state is verified in every form that
// carries one. A bare code needs no local check: it is bound server-side to
// this attempt's client id, PKCE verifier and redirect URI, and the verifier
// never leaves this process.
func parsePasted(s, wantState string) (pastedResult, error) {
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if (err != nil || u.Scheme == "" || u.Host == "") && strings.Contains(s, "code=") {
		// A redirect URL whose scheme was lost (terminal wrap, hand copy)
		// parses as a bare path or not at all; recover it so the paste
		// works instead of failing the exchange with an opaque
		// invalid_grant. The state check below still applies.
		if u2, err2 := url.Parse("http://" + strings.TrimPrefix(s, "//")); err2 == nil && u2.Host != "" && !strings.ContainsAny(u2.Host, "= ") {
			u, err = u2, nil
		}
	}
	if err == nil && u.Scheme != "" && u.Host != "" {
		q := u.Query()
		if e := q.Get("error"); e != "" {
			return pastedResult{}, fmt.Errorf("authorization was declined (%s)", e)
		}
		if q.Get("state") != wantState {
			return pastedResult{}, errors.New("state mismatch: the pasted URL is not from this login attempt")
		}
		if q.Get("code") == "" {
			return pastedResult{}, errors.New("the pasted URL carries no authorization code")
		}
		return pastedResult{Code: q.Get("code"), ClientID: q.Get("client_id")}, nil
	}
	if strings.Contains(s, "code=") {
		return pastedResult{}, errors.New("the pasted value looks like a redirect URL without its scheme; paste it with http://…, or paste just the code")
	}
	if code, st, ok := strings.Cut(s, "#"); ok {
		if st != wantState {
			return pastedResult{}, errors.New("state mismatch: the pasted value is not from this login attempt")
		}
		return pastedResult{Code: code}, nil
	}
	if s == "" {
		return pastedResult{}, errors.New("no code pasted")
	}
	return pastedResult{Code: s}, nil
}

type idClaims struct {
	Iss   string          `json:"iss"`
	Aud   json.RawMessage `json:"aud"`
	Exp   int64           `json:"exp"`
	Nonce string          `json:"nonce"`
	Sub   string          `json:"sub"`
	Email string          `json:"email"`
}

// validateIDToken verifies the RS256 signature against the provider's JWKS
// and checks issuer, audience (the issued client id), expiry and nonce.
func validateIDToken(ctx context.Context, hc *http.Client, jwksURL, issuer, clientID, nonce, idToken string) (idClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return idClaims{}, errors.New("id token is not a JWT")
	}
	hb, err := b64d(parts[0])
	if err != nil {
		return idClaims{}, fmt.Errorf("id token header: %w", err)
	}
	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(hb, &hdr); err != nil {
		return idClaims{}, fmt.Errorf("id token header: %w", err)
	}
	if hdr.Alg != "RS256" {
		return idClaims{}, fmt.Errorf("id token algorithm %q, want RS256", hdr.Alg)
	}
	sig, err := b64d(parts[2])
	if err != nil {
		return idClaims{}, fmt.Errorf("id token signature: %w", err)
	}
	key, err := jwksKey(ctx, hc, jwksURL, hdr.Kid)
	if err != nil {
		return idClaims{}, err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return idClaims{}, fmt.Errorf("id token signature does not verify: %w", err)
	}
	pb, err := b64d(parts[1])
	if err != nil {
		return idClaims{}, fmt.Errorf("id token payload: %w", err)
	}
	var cl idClaims
	if err := json.Unmarshal(pb, &cl); err != nil {
		return idClaims{}, fmt.Errorf("id token payload: %w", err)
	}
	switch {
	case cl.Iss != issuer:
		return idClaims{}, fmt.Errorf("id token issuer %q, want %q", cl.Iss, issuer)
	case !audContains(cl.Aud, clientID):
		return idClaims{}, errors.New("id token audience does not match the issued client id")
	case time.Now().Unix() >= cl.Exp:
		return idClaims{}, errors.New("id token is expired")
	case cl.Nonce != nonce:
		return idClaims{}, errors.New("id token nonce mismatch")
	case cl.Sub == "":
		return idClaims{}, errors.New("id token has no subject")
	}
	return cl, nil
}

func b64d(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

func audContains(raw json.RawMessage, want string) bool {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s == want
	}
	var l []string
	if json.Unmarshal(raw, &l) == nil {
		return slices.Contains(l, want)
	}
	return false
}

func jwksKey(ctx context.Context, hc *http.Client, jwksURL, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("jwks: HTTP %d", resp.StatusCode)
	}
	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || (kid != "" && k.Kid != kid) {
			continue
		}
		nb, err := b64d(k.N)
		if err != nil {
			continue
		}
		eb, err := b64d(k.E)
		if err != nil {
			continue
		}
		e := new(big.Int).SetBytes(eb)
		if e.Int64() <= 0 || e.Int64() > 1<<31 {
			continue
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(e.Int64())}, nil
	}
	return nil, fmt.Errorf("jwks: no RSA key matches kid %q", kid)
}

func missingScopes(granted, required []string) []string {
	var missing []string
	for _, r := range required {
		if !slices.Contains(granted, r) {
			missing = append(missing, r)
		}
	}
	return missing
}

// Headless reports whether the local callback is likely unreachable from a
// browser on this machine (SSH session, container, WSL, or no display on
// Linux): login then prints the URL and accepts a pasted code.
func Headless() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if b, err := os.ReadFile("/proc/version"); err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft") {
		return true
	}
	return runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

// OpenBrowser starts the system browser at u; the caller prints the URL as
// a fallback when this errors (no xdg-open, no display).
func OpenBrowser(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}

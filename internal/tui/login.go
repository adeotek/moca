package tui

// /login and /logout — the credential surface of the TUI (§3.5). /login picks
// a provider, then a method: a stored API key (masked entry, never echoed)
// written to auth.json (~/.config/moca/), or the subscription OAuth flow
// (openai, §6.5). At request time the stored key wins over the config's
// `env:` reference, and the store is read per request — a key stored here is
// used by the running session's next request. /logout clears whatever
// credential is stored for a provider (revoking a subscription session
// best-effort first).
//
// The whole interaction is a modal state machine like an approval prompt:
// while a login state is active, keys route to it; esc (or ctrl+c) backs out
// one level, then cancels. The flow itself runs in a tea.Cmd goroutine; its
// line-oriented output and its completion travel through the event pipe so
// they can never overtake each other (the run-event discipline).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
)

// loginOAuth is the OAuth flow entry point; a variable so tests can drive the
// TUI path against a fake authorization server without a browser.
var loginOAuth = provider.Login

// loginOAuthTimeout bounds each login HTTP call (token exchange, JWKS); the
// browser-side wait is not part of it.
const loginOAuthTimeout = 30 * time.Second

// logoutRevokeTimeout bounds the best-effort remote revocation on /logout.
const logoutRevokeTimeout = 15 * time.Second

type loginStep int

const (
	loginPickProvider loginStep = iota
	loginPickMethod
	loginEnterKey
	loginPickLogout
	loginBusy
	loginConfirmOAuth
)

// loginChoice is one row of a login picker (a provider, or an auth method).
type loginChoice struct {
	key   string // provider name, or "api_key"/"oauth"
	label string
}

// loginState is the active /login or /logout interaction.
type loginState struct {
	step     loginStep
	back     loginStep // where esc returns to inside the wizard
	items    []loginChoice
	cursor   int
	provider string
	key      []rune // API key being typed; never rendered, never logged

	// providerItems is the provider list of the picker, kept so esc from the
	// method picker can return to it.
	providerItems []loginChoice

	// OAuth flow plumbing.
	cancel context.CancelFunc
	pw     *io.PipeWriter // paste sink fed from the composer while busy

	// mu guards the progress buffer, written by the flow goroutine.
	mu      sync.Mutex
	pbuf    string   // partial output line
	pending []string // completed lines buffered when no pipe is wired (tests)
}

// shutdown cancels a running flow and releases the paste pipe, unblocking any
// write goroutine still waiting for a reader.
func (s *loginState) shutdown() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.pw != nil {
		s.pw.Close()
	}
}

// appendKey adds pasted text to the key buffer, dropping newlines and control
// bytes (a key is a single-line token; a stray newline would corrupt it).
func (s *loginState) appendKey(content string) {
	for _, r := range content {
		if r < 0x20 || r == 0x7f {
			continue
		}
		s.key = append(s.key, r)
	}
}

func (s *loginState) setList(items []loginChoice) {
	s.items, s.cursor = items, 0
}

// loginWriter adapts the flow's line-oriented output to the TUI scrollback:
// each complete line is emitted through the event pipe (or buffered for the
// done handler when tests run without one).
type loginWriter struct {
	s    *loginState
	pipe *eventPipe
}

func (w *loginWriter) Write(p []byte) (int, error) {
	s := w.s
	s.mu.Lock()
	s.pbuf += string(p)
	var lines []string
	for {
		i := strings.IndexByte(s.pbuf, '\n')
		if i < 0 {
			break
		}
		lines = append(lines, strings.TrimRight(s.pbuf[:i], "\r"))
		s.pbuf = s.pbuf[i+1:]
	}
	s.mu.Unlock()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if w.pipe != nil {
			w.pipe.send(loginProgressMsg{line: line})
			continue
		}
		s.mu.Lock()
		s.pending = append(s.pending, line)
		s.mu.Unlock()
	}
	return len(p), nil
}

// flushLoginPending prints lines buffered by loginWriter when no pipe was
// wired (unit tests); with a pipe the lines were already delivered live.
func (m *model) flushLoginPending(s *loginState) tea.Cmd {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	s.mu.Unlock()
	var cmds []tea.Cmd
	for _, line := range pending {
		cmds = append(cmds, m.printlnContent(line))
	}
	return tea.Sequence(cmds...)
}

// providerNames lists the configured providers, sorted.
func providerNames(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.Providers))
	for n := range cfg.Providers {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// providerRow labels a provider in the picker with its available methods.
func providerRow(name string) string {
	if _, oauth := provider.OAuthProvider(name); oauth {
		return name + "  — API key · subscription login"
	}
	if config.OAuthUnsupported[name] != "" {
		return name + "  — API key (subscription OAuth not permitted)"
	}
	return name + "  — API key"
}

// runLogin is `/login [provider]`.
func (m *model) runLogin(args string) tea.Cmd {
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	cfg := m.start.Config
	s := &loginState{step: loginPickProvider, back: loginPickProvider}
	for _, n := range providerNames(cfg) {
		s.providerItems = append(s.providerItems, loginChoice{key: n, label: providerRow(n)})
	}
	s.setList(s.providerItems)
	if a := strings.TrimSpace(args); a != "" {
		if _, known := cfg.Providers[a]; !known {
			return m.printlnError(fmt.Sprintf("error: unknown provider %q", a))
		}
		m.login = s
		return m.loginSelectProvider(a)
	}
	if len(s.items) == 0 {
		return m.printlnError("error: no providers configured")
	}
	m.login = s
	return nil
}

// runLogout is `/logout [provider]`: with no argument, a picker over the
// providers that actually have a stored credential.
func (m *model) runLogout(args string) tea.Cmd {
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	cfg := m.start.Config
	if a := strings.TrimSpace(args); a != "" {
		if _, known := cfg.Providers[a]; !known {
			return m.printlnError(fmt.Sprintf("error: unknown provider %q", a))
		}
		return m.logoutCmd(a)
	}
	store := provider.NewDefaultStore()
	var items []loginChoice
	for _, n := range providerNames(cfg) {
		_, hasKey, kerr := store.GetAPIKey(n)
		tok, hasTok, terr := store.Get(n)
		switch {
		case kerr != nil || terr != nil:
			// A corrupt store: the picker cannot read it, but /logout <p>
			// still resets it.
			continue
		case hasKey:
			items = append(items, loginChoice{key: n, label: n + "  — stored API key"})
		case hasTok:
			label := n + "  — stored login"
			if tok.Email != "" {
				label += " (" + tok.Email + ")"
			}
			items = append(items, loginChoice{key: n, label: label})
		}
	}
	if len(items) == 0 {
		return m.printlnMuted("nothing stored — /login first")
	}
	m.login = &loginState{step: loginPickLogout, back: loginPickLogout, items: items}
	return nil
}

// loginSelectProvider moves from the provider picker (or the /login <provider>
// shortcut) to the method picker, or straight to key entry for providers
// without an OAuth flow.
func (m *model) loginSelectProvider(name string) tea.Cmd {
	s := m.login
	s.provider = name
	if _, ok := provider.OAuthProvider(name); ok {
		s.step = loginPickMethod
		s.back = loginPickProvider
		s.setList([]loginChoice{
			{key: "api_key", label: "API key  — paste a key from the provider's console"},
			{key: "oauth", label: "Subscription login  — sign in with your ChatGPT plan in the browser"},
		})
		return nil
	}
	s.back = loginPickProvider
	return m.loginStartKey()
}

func (m *model) loginStartKey() tea.Cmd {
	s := m.login
	s.step, s.key = loginEnterKey, nil
	return nil
}

// loginChoose commits the highlighted picker row.
func (m *model) loginChoose() tea.Cmd {
	s := m.login
	if s.cursor < 0 || s.cursor >= len(s.items) {
		return nil
	}
	sel := s.items[s.cursor]
	switch s.step {
	case loginPickProvider:
		return m.loginSelectProvider(sel.key)
	case loginPickMethod:
		if sel.key == "oauth" {
			return m.loginStartOAuth()
		}
		s.back = loginPickMethod
		return m.loginStartKey()
	case loginPickLogout:
		m.login = nil
		return m.logoutCmd(sel.key)
	}
	return nil
}

// loginStartOAuth launches the subscription flow in a command goroutine. The
// credential is written by the goroutine (store.Put); progress lines and the
// completion both ride the event pipe, so the scrollback order matches the
// emission order.
func (m *model) loginStartOAuth() tea.Cmd {
	s := m.login
	p := s.provider
	oc, ok := provider.OAuthProvider(p)
	if !ok {
		m.login = nil
		return m.printlnError(fmt.Sprintf("error: no subscription login for %s", p))
	}
	store := provider.NewDefaultStore()
	saved, hasSaved, err := store.Get(p)
	if err != nil {
		m.login = nil
		return m.printlnError("error: " + err.Error())
	}
	var savedPtr *provider.Token
	if hasSaved && saved.ClientID != "" {
		savedPtr = &saved // reauthorization with the saved registration
	}
	hostID, err := store.HostID()
	if err != nil {
		m.login = nil
		return m.printlnError("error: " + err.Error())
	}
	oc.HostID = hostID
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.step = loginBusy
	pr, pw := io.Pipe()
	s.pw = pw
	pipe := m.pipe
	lio := provider.LoginIO{
		Out:      &loginWriter{s: s, pipe: pipe},
		Lines:    provider.NewLineReader(pr),
		Headless: provider.Headless(),
	}
	if !lio.Headless {
		lio.OpenURL = provider.OpenBrowser
	}
	return func() tea.Msg {
		tok, err := loginOAuth(ctx, oc, savedPtr, lio, &http.Client{Timeout: loginOAuthTimeout})
		if err == nil {
			err = store.Put(p, tok)
		}
		msg := loginDoneMsg{provider: p, email: tok.Email, err: err}
		if pipe != nil {
			pipe.send(msg) // behind every progress line this flow emitted
			return nil
		}
		return msg
	}
}

func (m *model) loginKeyInput(k tea.KeyPressMsg) tea.Cmd {
	s := m.login
	switch k.String() {
	case "esc", "ctrl+c":
		s.step, s.key = s.back, nil
		return nil
	case "enter":
		key := strings.TrimSpace(string(s.key))
		if key == "" {
			return nil
		}
		s.key = nil
		p := s.provider
		m.login = nil
		store := provider.NewDefaultStore()
		if err := store.PutAPIKey(p, key); err != nil {
			return m.printlnError("error: " + err.Error())
		}
		if m.start.Config.Providers[p].Auth == "oauth" {
			return m.printlnContent(fmt.Sprintf("stored API key for %s in %s — note: providers.%s.auth is \"oauth\"; set it to \"api_key\" to use the key", p, store.Path(), p))
		}
		return m.printlnContent(fmt.Sprintf("stored API key for %s in %s — the next request to it uses the key", p, store.Path()))
	case "backspace":
		if n := len(s.key); n > 0 {
			s.key = s.key[:n-1]
		}
		return nil
	default:
		if t := k.Text; t != "" {
			s.appendKey(t)
		}
		return nil
	}
}

// loginFlipAuth switches providers.<p>.auth to "oauth" after a successful
// subscription login (the user confirmed). The config edit is
// comment-preserving; the reload lets a later /clear start its session with
// the new mode, while the running session keeps what it was built with.
func (m *model) loginFlipAuth() tea.Cmd {
	s := m.login
	m.login = nil
	p := s.provider
	path := m.opts.ConfigPath
	if path == "" {
		path = config.ConfigFile()
	}
	if _, err := config.SetString(path, []string{"providers", p}, "auth", "oauth"); err != nil {
		return m.printlnError("error: could not update the config: " + err.Error())
	}
	note := "restart moca to use your subscription"
	if cfg, err := config.Load(path); err == nil {
		m.start.Config, m.opts.Start.Config = cfg, cfg
		note = "a /clear (or a restart) starts the next session on your subscription"
	}
	return m.printlnContent(fmt.Sprintf("set providers.%s.auth to \"oauth\" in %s — %s", p, path, note))
}

func (m *model) handleLoginDone(msg loginDoneMsg) tea.Cmd {
	s := m.login
	if s != nil && s.pw != nil {
		s.pw.Close() // release a paste write still waiting for its reader
	}
	var cmds []tea.Cmd
	if s != nil {
		cmds = append(cmds, m.flushLoginPending(s))
	}
	if msg.err != nil {
		m.login = nil
		if errors.Is(msg.err, context.Canceled) {
			cmds = append(cmds, m.printlnMuted("sign-in cancelled"))
		} else {
			cmds = append(cmds, m.printlnError("error: "+msg.err.Error()))
		}
		return tea.Sequence(cmds...)
	}
	line := "logged in to " + msg.provider
	if msg.email != "" {
		line += " as " + msg.email
	}
	cmds = append(cmds, m.printlnContent(line))
	if s == nil || m.start.Config.Providers[msg.provider].Auth == "oauth" {
		m.login = nil
		return tea.Sequence(cmds...)
	}
	s.step = loginConfirmOAuth
	return tea.Sequence(cmds...)
}

// logoutCmd revokes (best-effort, for OAuth tokens) and clears the stored
// credential off the event loop.
func (m *model) logoutCmd(p string) tea.Cmd {
	store := provider.NewDefaultStore()
	return func() tea.Msg {
		tok, hasTok, terr := store.Get(p)
		_, hasKey, _ := store.GetAPIKey(p)
		var warn error
		if terr != nil {
			warn = terr
		}
		if hasTok && tok.Refresh != "" {
			if oc, ok := provider.OAuthProvider(p); ok {
				rctx, cancel := context.WithTimeout(context.Background(), logoutRevokeTimeout)
				if rerr := provider.Revoke(rctx, oc, tok, http.DefaultClient); rerr != nil {
					warn = fmt.Errorf("could not revoke the session remotely (%v); cleared locally", rerr)
				}
				cancel()
			}
		}
		if err := store.Delete(p); err != nil {
			return logoutDoneMsg{provider: p, err: err}
		}
		return logoutDoneMsg{provider: p, had: hasTok || hasKey, warn: warn}
	}
}

func (m *model) handleLogoutDone(msg logoutDoneMsg) tea.Cmd {
	var cmds []tea.Cmd
	if msg.warn != nil {
		cmds = append(cmds, m.printlnWarn("warning: "+msg.warn.Error()))
	}
	if msg.err != nil {
		cmds = append(cmds, m.printlnError("error: "+msg.err.Error()))
		return tea.Sequence(cmds...)
	}
	if !msg.had {
		cmds = append(cmds, m.printlnMuted("no stored credential for "+msg.provider))
		return tea.Sequence(cmds...)
	}
	cmds = append(cmds, m.printlnContent("logged out of "+msg.provider))
	return tea.Sequence(cmds...)
}

// loginKey routes every key while a login state is active. esc (and ctrl+c)
// backs out one level; anything unhandled in the busy step falls through to
// the composer, so the redirect URL/code can be typed or pasted and sent.
func (m *model) loginKey(k tea.KeyPressMsg) tea.Cmd {
	s := m.login
	esc := k.String() == "esc" || k.String() == "ctrl+c"
	switch s.step {
	case loginPickProvider, loginPickMethod, loginPickLogout:
		switch {
		case esc && s.step == loginPickMethod:
			s.step, s.back = loginPickProvider, loginPickProvider
			s.setList(s.providerItems)
			return nil
		case esc:
			m.login = nil
			if s.step == loginPickLogout {
				return m.printlnMuted("logout cancelled")
			}
			return m.printlnMuted("login cancelled")
		case k.String() == "up":
			if s.cursor > 0 {
				s.cursor--
			}
			return nil
		case k.String() == "down":
			if s.cursor < len(s.items)-1 {
				s.cursor++
			}
			return nil
		case k.String() == "enter":
			return m.loginChoose()
		}
		return nil
	case loginEnterKey:
		return m.loginKeyInput(k)
	case loginConfirmOAuth:
		switch k.String() {
		case "y", "Y":
			return m.loginFlipAuth()
		case "n", "N", "esc", "ctrl+c":
			p := s.provider
			m.login = nil
			return m.printlnMuted(fmt.Sprintf("left providers.%s.auth unchanged — set \"auth\": \"oauth\" to use the subscription", p))
		}
		return nil
	case loginBusy:
		switch {
		case esc:
			if s.cancel != nil {
				s.cancel()
			}
			return nil
		case k.String() == "enter":
			text := strings.TrimSpace(m.input.Text())
			if text == "" {
				return nil
			}
			m.input.Submit()
			m.syncTextarea()
			if s.pw != nil {
				pw := s.pw
				// Non-blocking write: the flow's paste reader may not be
				// waiting yet (it starts after WaitBeforePaste in a browser
				// login); the write unblocks when it reads, or errors when
				// the flow ends and closes the pipe.
				go func() { _, _ = pw.Write([]byte(text + "\n")) }()
			}
			return m.printlnMuted("↳ handed to the sign-in flow")
		default:
			return m.forward(k)
		}
	}
	return nil
}

// loginPanel renders the active login state above the input rule.
func (m *model) loginPanel() string {
	s := m.login
	var b strings.Builder
	q := func(text string) { b.WriteString(warnFg.Render("? ") + Sanitize(text) + "\n") }
	switch s.step {
	case loginPickProvider:
		q("store a credential for which provider?  (↑/↓ · enter · esc cancels)")
		for i, it := range s.items {
			b.WriteString(choiceRow(i == s.cursor, it.label) + "\n")
		}
	case loginPickMethod:
		q(fmt.Sprintf("how should moca authenticate to %s?", s.provider))
		for i, it := range s.items {
			b.WriteString(choiceRow(i == s.cursor, it.label) + "\n")
		}
	case loginPickLogout:
		q("remove the stored credential of which provider?  (↑/↓ · enter · esc cancels)")
		for i, it := range s.items {
			b.WriteString(choiceRow(i == s.cursor, it.label) + "\n")
		}
	case loginEnterKey:
		q(fmt.Sprintf("API key for %s — enter stores it, esc goes back (it is never echoed or shown again)", s.provider))
		if len(s.key) == 0 {
			b.WriteString("  " + dim.Render("type or paste the key") + "\n")
		} else {
			b.WriteString("  " + masked(s.key) + "\n")
		}
	case loginBusy:
		q(fmt.Sprintf("waiting for the %s sign-in to complete…", s.provider))
		b.WriteString(dim.Render("  esc cancels · if the browser cannot reach this machine, paste the redirect URL or code in the input below (alt+p expands paste chips)") + "\n")
	case loginConfirmOAuth:
		q(fmt.Sprintf("logged in — switch providers.%s.auth to \"oauth\" in the config?  [y/N]", s.provider))
	}
	return strings.TrimRight(b.String(), "\n")
}

func choiceRow(active bool, label string) string {
	if active {
		return "❯ " + Sanitize(label)
	}
	return "  " + Sanitize(label)
}

// masked renders the key buffer as dots, capped so a long key cannot wrap the
// frame; only the length is visible, never the content.
func masked(key []rune) string {
	const cap = 40
	if len(key) <= cap {
		return strings.Repeat("•", len(key))
	}
	return strings.Repeat("•", cap) + fmt.Sprintf("  (%d chars)", len(key))
}

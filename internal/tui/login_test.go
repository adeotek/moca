package tui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
)

// newLoginModel builds a model with a config but no agent: /login and /logout
// never need one. The XDG dirs are isolated so the store under test is the
// temp one.
func newLoginModel(t *testing.T) *model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg, err := config.Parse([]byte(`{"model":"openai/gpt-6-astra"}`))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(cfgPath, []byte("{\n  \"model\": \"openai/gpt-6-astra\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newModel(AppOptions{Start: agent.StartOptions{Config: cfg}, ConfigPath: cfgPath, Home: "/home/u"}, nil)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// tuiCmd drives one input line through submit, like a terminal would.
func tuiCmd(t *testing.T, m *model, line string) tea.Cmd {
	t.Helper()
	m.input.Insert(line)
	m.syncTextarea()
	return m.submit()
}

// drained collects the scrollback a command prints plus every queued line
// behind it, driving the acknowledgement cycle the live renderer does.
func drained(m *model, cmd tea.Cmd) string {
	var sb strings.Builder
	sb.WriteString(printed(cmd))
	for m.outBusy {
		sb.WriteString(printed(ack(m)))
	}
	return sb.String()
}

func TestLoginStoresAPIKey(t *testing.T) {
	m := newLoginModel(t)
	tuiCmd(t, m, "/login")
	if m.login == nil || m.login.step != loginPickProvider {
		t.Fatalf("provider picker not open: %+v", m.login)
	}
	view := m.View().Content
	for _, want := range []string{"store a credential for which provider?", "anthropic", "openai", "opencode-go", "❯ anthropic"} {
		if !strings.Contains(view, want) {
			t.Fatalf("picker missing %q:\n%s", want, view)
		}
	}
	// anthropic, openai, opencode-go: two downs lands on opencode-go, which
	// has no OAuth flow and goes straight to key entry.
	m.Update(key("down"))
	m.Update(key("down"))
	m.Update(key("enter"))
	if m.login.provider != "opencode-go" || m.login.step != loginEnterKey {
		t.Fatalf("key entry not open: %+v", m.login)
	}
	// A paste into the key buffer drops newlines and never reaches the view.
	m.Update(tea.PasteMsg{Content: "sk-live-123\n"})
	if view := m.View().Content; strings.Contains(view, "sk-live-123") {
		t.Fatalf("the key leaked into the view:\n%s", view)
	}
	_, cmd := m.Update(key("enter"))
	if m.login != nil {
		t.Fatal("the wizard must close after storing")
	}
	if out := drained(m, cmd); !strings.Contains(out, "stored API key for opencode-go") {
		t.Fatalf("scrollback %q", out)
	}
	if k, ok, _ := provider.NewDefaultStore().GetAPIKey("opencode-go"); !ok || k != "sk-live-123" {
		t.Fatalf("stored key %q %v", k, ok)
	}
}

func TestLoginMarkedBackAndCancel(t *testing.T) {
	m := newLoginModel(t)
	tuiCmd(t, m, "/login openai")
	if m.login == nil || m.login.step != loginPickMethod {
		t.Fatalf("method picker not open: %+v", m.login)
	}
	if view := m.View().Content; !strings.Contains(view, "API key") || !strings.Contains(view, "Subscription login") {
		t.Fatalf("method rows:\n%s", view)
	}
	// enter on the API key row → key entry; esc walks back up the wizard.
	m.Update(key("enter"))
	if m.login.step != loginEnterKey {
		t.Fatalf("key entry: %+v", m.login)
	}
	m.Update(key("esc"))
	if m.login.step != loginPickMethod {
		t.Fatalf("esc from key entry must return to the method picker: %+v", m.login)
	}
	m.Update(key("esc"))
	if m.login.step != loginPickProvider || len(m.login.items) != 3 {
		t.Fatalf("esc from the method picker must return to the provider picker: %+v", m.login)
	}
	if _, cmd := m.Update(key("esc")); m.login != nil || !strings.Contains(drained(m, cmd), "login cancelled") {
		t.Fatalf("esc must cancel the wizard: %+v %q", m.login, drained(m, cmd))
	}
	// Enter with an empty key buffer does nothing.
	tuiCmd(t, m, "/login anthropic")
	m.Update(key("enter"))
	if m.login == nil || m.login.step != loginEnterKey {
		t.Fatalf("empty key must keep the prompt open: %+v", m.login)
	}
	m.Update(key("esc"))
	m.Update(key("esc"))
	if m.login != nil {
		t.Fatal("cancelled")
	}
}

func TestLoginRefusedWhileRunning(t *testing.T) {
	m := newLoginModel(t)
	m.running = true
	cmd := tuiCmd(t, m, "/login")
	if m.login != nil {
		t.Fatal("no wizard while a run is in progress")
	}
	if out := drained(m, cmd); !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("scrollback %q", out)
	}
}

func TestLoginOAuthFlowAndAuthFlip(t *testing.T) {
	m := newLoginModel(t)
	old := loginOAuth
	loginOAuth = func(ctx context.Context, oc provider.OAuthConfig, saved *provider.Token, lio provider.LoginIO, hc *http.Client) (provider.Token, error) {
		fmt.Fprintln(lio.Out, "Opening your browser to sign in...")
		fmt.Fprintln(lio.Out, "https://auth.example.test/authorize?state=s1")
		return provider.Token{Access: "AT", Refresh: "RT", ClientID: "cid", Email: "u@example.com", Expiry: time.Now().Add(time.Hour)}, nil
	}
	defer func() { loginOAuth = old }()

	tuiCmd(t, m, "/login openai")
	m.Update(key("down")) // the subscription row
	if m.login.cursor != 1 || m.login.items[1].key != "oauth" {
		t.Fatalf("cursor: %+v", m.login)
	}
	_, cmd := m.Update(key("enter"))
	if m.login == nil || m.login.step != loginBusy {
		t.Fatalf("flow not running: %+v", m.login)
	}
	if cmd == nil {
		t.Fatal("no flow command")
	}
	// No pipe in this model: the flow command returns the done message
	// directly, with its progress lines buffered for the done handler.
	msg := cmd()
	_, done := m.Update(msg)
	out := drained(m, done)
	for _, want := range []string{"Opening your browser", "https://auth.example.test/authorize?state=s1", "logged in to openai as u@example.com"} {
		if !strings.Contains(out, want) {
			t.Fatalf("scrollback missing %q:\n%q", want, out)
		}
	}
	if tok, ok, _ := provider.NewDefaultStore().Get("openai"); !ok || tok.Access != "AT" || tok.ClientID != "cid" {
		t.Fatalf("stored token: %+v %v", tok, ok)
	}
	if m.login == nil || m.login.step != loginConfirmOAuth {
		t.Fatalf("flip prompt not open: %+v", m.login)
	}
	// y flips the provider's auth mode; a later /clear picks the reload up.
	_, cmd = m.Update(key("y"))
	if m.login != nil {
		t.Fatal("wizard must close after the flip")
	}
	if out := drained(m, cmd); !strings.Contains(out, "set providers.openai.auth to \"oauth\"") {
		t.Fatalf("scrollback %q", out)
	}
	raw, err := os.ReadFile(m.opts.ConfigPath)
	if err != nil || !strings.Contains(string(raw), "\"oauth\"") {
		t.Fatalf("config not rewritten: %v %q", err, raw)
	}
	if got := m.start.Config.Providers["openai"].Auth; got != "oauth" {
		t.Fatalf("reloaded config auth %q", got)
	}
}

func TestLoginOAuthDeclinedFlipAndCancel(t *testing.T) {
	m := newLoginModel(t)
	old := loginOAuth
	loginOAuth = func(ctx context.Context, oc provider.OAuthConfig, saved *provider.Token, lio provider.LoginIO, hc *http.Client) (provider.Token, error) {
		return provider.Token{Access: "AT", Email: "u@example.com", Expiry: time.Now().Add(time.Hour)}, nil
	}
	defer func() { loginOAuth = old }()
	before, _ := os.ReadFile(m.opts.ConfigPath)

	tuiCmd(t, m, "/login openai")
	m.Update(key("down"))
	_, cmd := m.Update(key("enter"))
	_, done := m.Update(cmd())
	_ = drained(m, done)
	// n leaves the config untouched.
	_, cmd = m.Update(key("n"))
	if m.login != nil {
		t.Fatal("wizard must close on n")
	}
	if out := drained(m, cmd); !strings.Contains(out, "unchanged") {
		t.Fatalf("scrollback %q", out)
	}
	if after, _ := os.ReadFile(m.opts.ConfigPath); string(after) != string(before) {
		t.Fatalf("config changed on n: %q", after)
	}

	// esc during the flow cancels it.
	loginOAuth = func(ctx context.Context, oc provider.OAuthConfig, saved *provider.Token, lio provider.LoginIO, hc *http.Client) (provider.Token, error) {
		<-ctx.Done()
		return provider.Token{}, ctx.Err()
	}
	tuiCmd(t, m, "/login openai")
	m.Update(key("down"))
	_, cmd = m.Update(key("enter"))
	doneCh := make(chan tea.Msg, 1)
	go func() { doneCh <- cmd() }()
	m.Update(key("esc"))
	_, cmd = m.Update(<-doneCh)
	if m.login != nil {
		t.Fatal("wizard must close after cancel")
	}
	if out := drained(m, cmd); !strings.Contains(out, "sign-in cancelled") {
		t.Fatalf("scrollback %q", out)
	}
}

func TestLogoutClearsStoredCredential(t *testing.T) {
	m := newLoginModel(t)
	store := provider.NewDefaultStore()
	if err := store.PutAPIKey("anthropic", "sk-ant"); err != nil {
		t.Fatal(err)
	}
	// Picker with one row (only anthropic has a stored credential).
	tuiCmd(t, m, "/logout")
	if m.login == nil || m.login.step != loginPickLogout || len(m.login.items) != 1 {
		t.Fatalf("logout picker: %+v", m.login)
	}
	_, cmd := m.Update(key("enter"))
	if m.login != nil || cmd == nil {
		t.Fatal("picker must close and run the logout")
	}
	_, done := m.Update(cmd())
	if out := drained(m, done); !strings.Contains(out, "logged out of anthropic") {
		t.Fatalf("scrollback %q", out)
	}
	if _, ok, _ := store.GetAPIKey("anthropic"); ok {
		t.Fatal("stored key must be gone")
	}
	// Nothing stored: a clear note, no error.
	_, done = m.Update(m.logoutCmd("openai")())
	if out := drained(m, done); !strings.Contains(out, "no stored credential for openai") {
		t.Fatalf("scrollback %q", out)
	}
	// With nothing stored anywhere the picker is not opened.
	if cmd := tuiCmd(t, m, "/logout"); m.login != nil {
		t.Fatalf("picker opened with an empty store: %q", drained(m, cmd))
	} else if out := drained(m, cmd); !strings.Contains(out, "nothing stored") {
		t.Fatalf("scrollback %q", out)
	}
}

func TestLoginCommandParsingAndHelp(t *testing.T) {
	p, _ := ParseInput("/login opencode-go", nil)
	if p.Kind != KindCommand || p.Name != "login" || p.Args != "opencode-go" {
		t.Fatalf("parse: %+v", p)
	}
	p, _ = ParseInput("/logout", nil)
	if p.Kind != KindCommand || p.Name != "logout" {
		t.Fatalf("parse: %+v", p)
	}
	h := HelpText(nil)
	if !strings.Contains(h, "/login") || !strings.Contains(h, "/logout") {
		t.Fatalf("help:\n%s", h)
	}
}

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/session"
)

// setupEnv isolates a test's XDG dirs and blanks the built-in providers'
// default env refs, so credential marking is deterministic (an ambient
// ANTHROPIC_API_KEY must not credential a provider here).
func setupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, e := range []string{"ANTHROPIC_API_KEY", "OPENCODE_API_KEY", "OPENAI_API_KEY"} {
		t.Setenv(e, "")
	}
}

// setupCfg is the setup-mode config: a declared fake provider (two models,
// base URL never called) with model empty unless given — what a fresh
// install looks like.
func setupCfg(t *testing.T, model, apiKey string) config.Config {
	t.Helper()
	block := fmt.Sprintf(`"providers":{"fake":{"baseUrl":"http://127.0.0.1:1","protocol":"openai-completions","auth":"api_key","apiKey":%q,"models":{"m":{"contextWindow":65536},"m2":{"contextWindow":65536}}}}`, apiKey)
	j := "{" + block + "}"
	if model != "" {
		j = fmt.Sprintf(`{"model":%q,%s}`, model, block)
	}
	cfg, err := config.Parse([]byte(j))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// setupModel builds the unconfigured TUI (§3.5) with a config path that does
// not exist yet: the first-run state.
func setupModel(t *testing.T, apiKey string) (*model, string) {
	t.Helper()
	setupEnv(t)
	cfgPath := filepath.Join(t.TempDir(), "config.jsonc")
	m := newModel(AppOptions{Start: agent.StartOptions{Config: setupCfg(t, "", apiKey), Workdir: t.TempDir(), Slug: "t"}, ConfigPath: cfgPath, Home: "/home/u"}, nil)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m, cfgPath
}

// The setup notice rides the welcome print (§3.5): one print, so the
// greeting cannot be lost to the first frame's render timing.
func TestSetupNoticeRidesTheWelcome(t *testing.T) {
	m, _ := setupModel(t, "env:MOCA_SETUP_KEY")
	out := printed(m.Init())
	if !strings.Contains(out, "How can I help you today?") || !strings.Contains(out, red.Render("no provider configured")) {
		t.Fatalf("startup printed %q", out)
	}
}

// A message cannot run unconfigured: the same red notice answers it and
// nothing starts.
func TestSetupSubmitGuidesToLogin(t *testing.T) {
	m, _ := setupModel(t, "env:MOCA_SETUP_KEY")
	m.input.SetBuffer("hello")
	out, _ := simulate(m, m.submit())
	if !strings.Contains(out, "no provider configured") || !strings.Contains(out, "/login") {
		t.Fatalf("submit printed %q", out)
	}
	if m.running || m.agent != nil {
		t.Fatal("a run started while unconfigured")
	}
	if m.input.Text() != "" {
		t.Fatalf("draft not cleared: %q", m.input.Text())
	}
}

// Agent-dependent commands answer with the notice instead of doing nothing;
// /help and /exit keep working.
func TestSetupGuardsAgentCommandsButNotHelpExit(t *testing.T) {
	m, _ := setupModel(t, "env:MOCA_SETUP_KEY")
	for _, name := range []string{"compact", "cost", "effort", "hard", "yolo", "clear", "undo"} {
		out, _ := simulate(m, m.runCommand(Parsed{Kind: KindCommand, Name: name}))
		if !strings.Contains(out, "no provider configured") {
			t.Errorf("/%s printed %q", name, out)
		}
		if m.agent != nil {
			t.Fatalf("/%s started a session", name)
		}
	}
	out, _ := simulate(m, m.runCommand(Parsed{Kind: KindCommand, Name: "help"}))
	if !strings.Contains(out, "/login") {
		t.Fatalf("/help printed %q", out)
	}
	if _, quit := simulate(m, m.runCommand(Parsed{Kind: KindCommand, Name: "exit"})); !quit {
		t.Fatal("/exit must quit")
	}
}

// Picking a model starts the first session in place and saves the choice as
// the config's "model" — the next launch skips setup mode.
func TestPickModelStartsTheFirstSession(t *testing.T) {
	t.Setenv("MOCA_SETUP_KEY", "k")
	m, cfgPath := setupModel(t, "env:MOCA_SETUP_KEY")
	t.Cleanup(func() {
		if m.agent != nil {
			m.agent.Close()
		}
	})
	if out, _ := simulate(m, m.runCommand(Parsed{Kind: KindCommand, Name: "model"})); out != "" {
		t.Fatalf("picker printed %q", out)
	}
	if m.pick == nil {
		t.Fatal("no first-run picker")
	}
	idx := -1
	for i, it := range m.pick.items {
		if it.key == "fake/m" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("fake/m not listed: %v", m.pick.items)
	}
	if m.pick.cursor != idx {
		t.Fatalf("cursor on row %d, want the ready model at %d", m.pick.cursor, idx)
	}
	out, _ := simulate(m, m.pickKey(keyMsg("enter")))
	if m.agent == nil {
		t.Fatalf("no session started: %q", out)
	}
	if !strings.Contains(out, "session") || !strings.Contains(out, "fake/m") {
		t.Fatalf("start printed %q", out)
	}
	if m.status.Model != "fake/m" {
		t.Fatalf("status model %q", m.status.Model)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("model not saved: %v", err)
	}
	if !strings.Contains(string(b), `"model": "fake/m"`) {
		t.Fatalf("config now %q", b)
	}
}

// /model <id> takes the argument path, no picker.
func TestModelArgumentStartsSession(t *testing.T) {
	t.Setenv("MOCA_SETUP_KEY", "k")
	m, cfgPath := setupModel(t, "env:MOCA_SETUP_KEY")
	t.Cleanup(func() {
		if m.agent != nil {
			m.agent.Close()
		}
	})
	out, _ := simulate(m, m.runCommand(Parsed{Kind: KindCommand, Name: "model", Args: "fake/m2"}))
	if m.agent == nil || !strings.Contains(out, "fake/m2") {
		t.Fatalf("start printed %q", out)
	}
	if b, err := os.ReadFile(cfgPath); err != nil || !strings.Contains(string(b), `"model": "fake/m2"`) {
		t.Fatalf("config %q err %v", b, err)
	}
}

// Without a credential for the model's provider the start is refused with
// how to store one, and nothing is written.
func TestUncredentialedModelRefused(t *testing.T) {
	t.Setenv("MOCA_SETUP_MISSING", "")
	m, cfgPath := setupModel(t, "env:MOCA_SETUP_MISSING")
	out, _ := simulate(m, m.runCommand(Parsed{Kind: KindCommand, Name: "model", Args: "fake/m"}))
	if m.agent != nil {
		t.Fatalf("session started without a credential: %q", out)
	}
	if !strings.Contains(out, "MOCA_SETUP_MISSING") || !strings.Contains(out, "/login") {
		t.Fatalf("refusal printed %q", out)
	}
	if _, err := os.Stat(cfgPath); err == nil {
		t.Fatal("config written despite the refusal")
	}
}

// A stored key alone still says a model is missing: /model is the next step.
func TestLoginInSetupHintsAtModel(t *testing.T) {
	t.Setenv("MOCA_SETUP_KEY", "k")
	m, _ := setupModel(t, "env:MOCA_SETUP_KEY")
	if out, _ := simulate(m, m.runLogin("fake")); out != "" {
		t.Fatalf("login start printed %q", out)
	}
	if m.login == nil {
		t.Fatal("no key-entry state")
	}
	m.login.appendKey("k")
	out, _ := simulate(m, m.loginKeyInput(keyMsg("enter")))
	if !strings.Contains(out, "stored API key for fake") || !strings.Contains(out, "no model selected yet") {
		t.Fatalf("login printed %q", out)
	}
}

// /resume needs no configured model: the stored session's model wins.
func TestResumeFindsItsModelWithoutConfig(t *testing.T) {
	setupEnv(t)
	t.Setenv("MOCA_SETUP_KEY", "k")
	wd := t.TempDir()
	a, err := agent.Start(agent.StartOptions{Config: setupCfg(t, "fake/m", "env:MOCA_SETUP_KEY"), Workdir: wd, Slug: "t"})
	if err != nil {
		t.Fatal(err)
	}
	id := a.Session().ID8()
	a.Close()
	path, err := session.Find(sessionsDir(), id)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(AppOptions{Start: agent.StartOptions{Config: setupCfg(t, "", "env:MOCA_SETUP_KEY"), Workdir: wd, Slug: "t"}, Home: "/home/u"}, nil)
	t.Cleanup(func() {
		if m.agent != nil {
			m.agent.Close()
		}
	})
	if out, _ := simulate(m, m.resumeSession(path)); m.agent == nil {
		t.Fatalf("resume failed: %q", out)
	}
	if q := m.agent.Status().Model.Qualified(); q != "fake/m" {
		t.Fatalf("resumed model %q", q)
	}
}

// The status bar carries the setup state instead of an empty model pair.
func TestStatusLineCarriesTheSetupState(t *testing.T) {
	m, _ := setupModel(t, "env:MOCA_SETUP_KEY")
	_, l2 := RenderStatus(m.status, 120)
	if l2 != "no provider configured — /login" {
		t.Fatalf("status line 2 %q", l2)
	}
	if got := m.statusLine(); !strings.Contains(got, "no provider configured — /login") {
		t.Fatalf("status line %q", got)
	}
}

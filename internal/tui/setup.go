package tui

// Setup mode (§3.5): moca started with no "model" configured — a fresh
// install — so no session can be created yet. The TUI still opens: the
// scrollback carries a red notice, /login stores a credential (the wizard is
// agent-free), and /model picks a model — checked for a usable credential,
// saved as the config's "model", and turned into the first session on the
// spot. `-p` keeps the hard error: a one-shot run has no scrollback to
// explain itself in.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
)

// unconfigured reports setup mode: no agent exists because no model is
// configured (§3.5) — a session needs one.
func (m *model) unconfigured() bool { return m.agent == nil }

// noProviderNotice is the red setup line. The same text answers a message or
// an agent-dependent command while unconfigured, so the way out of the state
// is always one keystroke away.
func noProviderNotice() string {
	return red.Render("no provider configured") + " — run /login first (store an API key or sign in), then /model picks a model"
}

func (m *model) printlnNoProvider() tea.Cmd { return m.println(noProviderNotice()) }

// setupModelHint follows a successful login in the unconfigured TUI: the
// credential alone starts nothing — a model is still missing.
func (m *model) setupModelHint() tea.Cmd {
	if !m.unconfigured() {
		return nil
	}
	return m.printlnMuted("no model selected yet — /model picks one and starts the session")
}

// setupRegistry lazily builds the registry the unconfigured TUI lists /model
// choices from (there is no session yet to own one). It is cached so an
// ollama discovery stays visible to the picker, and dropped when the config
// is reloaded (loginFlipAuth); CheckCredential re-reads the credential store
// per call, so a key stored with /login shows up without rebuilding.
func (m *model) setupRegistry() (*provider.Registry, error) {
	if m.setupReg != nil {
		return m.setupReg, nil
	}
	reg, err := provider.NewRegistry(m.start.Config, http.DefaultClient, nil)
	if err != nil {
		return nil, err
	}
	m.setupReg = reg
	return reg, nil
}

// runModelSetup is /model while unconfigured: pick (or take by argument) the
// model the first session should use.
func (m *model) runModelSetup(args string) tea.Cmd {
	if cmd, refused := m.refuseRunning(); refused {
		return cmd
	}
	if a := strings.TrimSpace(args); a != "" {
		return m.startSession(a)
	}
	reg, err := m.setupRegistry()
	if err != nil {
		return m.printlnError("error: " + err.Error())
	}
	if _, local := m.start.Config.Providers["ollama"]; local {
		// Models pulled meanwhile should show up: ask the server first (a
		// few seconds at most), then open the picker.
		m.status.Transient = "reading models…"
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			_, err := reg.DiscoverOllama(ctx)
			return modelsRefreshedMsg{err: err}
		}
	}
	return m.openSetupModelPicker()
}

// openSetupModelPicker opens the first-run model picker: every catalog model,
// credential-less providers marked, the first ready one under the cursor.
func (m *model) openSetupModelPicker() tea.Cmd {
	reg, err := m.setupRegistry()
	if err != nil {
		return m.printlnError("error: " + err.Error())
	}
	items, cursor := modelPickItems(reg.Models(), "", func(q string) bool { return reg.CheckCredential(q) == nil })
	p := &pickState{
		title:      "which model should moca start with?",
		cancelNote: "no model picked — moca cannot run yet",
		onChoose:   m.startSession,
	}
	p.items, p.cursor = items, cursor
	m.openPicker(p)
	return nil
}

// startSession starts the first session from the unconfigured TUI: the
// model's credential is checked first (no network — a missing key refuses
// with how to store one), the session is created on the spot and the choice
// is saved as "model" in the config, so the next launch starts straight into
// a session. A save that fails only warns: the session still runs.
func (m *model) startSession(q string) tea.Cmd {
	reg, err := m.setupRegistry()
	if err != nil {
		return m.printlnError("error: " + err.Error())
	}
	if err := reg.CheckCredential(q); err != nil {
		return m.printlnError("error: " + err.Error())
	}
	opts := m.start
	opts.Config.Model = q
	a, err := agent.Start(opts)
	if err != nil {
		return m.printlnError("error: " + err.Error())
	}
	m.agent = a
	m.start = opts
	m.opts.Start.Config.Model = q
	m.refreshStatus()
	note := fmt.Sprintf("session %s · %s", a.Session().ID8(), q)
	path := m.opts.ConfigPath
	if path == "" {
		path = config.ConfigFile()
	}
	if _, err := config.SetString(path, nil, "model", q); err != nil {
		return tea.Sequence(m.printlnMuted(note), m.printlnWarn("warning: could not save the model to "+path+": "+err.Error()))
	}
	return tea.Sequence(m.printlnMuted(note+" — saved as the \"model\" default in "+AbbrevHome(path, m.opts.Home)), m.branchCmd())
}

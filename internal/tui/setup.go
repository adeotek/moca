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
	"log/slog"
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

// setupNotice is the setup line. When no provider is usable it is the red
// no-provider notice; once one is (a stored login or key, or one the config's
// env: reference resolves), only the model is missing. A credential store or
// registry that cannot be read is reported as itself — folding it into "no
// provider configured" would send the user to /login to hit the same error.
// The same text answers a message or an agent-dependent command while
// unconfigured, so the way out of the state is always one keystroke away.
func (m *model) setupNotice() string {
	if err := provider.NewDefaultStore().Validate(); err != nil {
		return m.th.alertStyle(m.darkBG).Render("error: " + err.Error())
	}
	reg, err := m.setupRegistry()
	if err != nil {
		return m.th.alertStyle(m.darkBG).Render("error: " + err.Error())
	}
	if anyProviderUsable(reg) {
		return m.th.mutedStyle(m.darkBG).Render("no model selected yet — /model picks one and starts the session")
	}
	return m.th.alertStyle(m.darkBG).Render("no provider configured") + " — run /login first (store an API key or sign in), then /model picks a model"
}

// anyProviderUsable is the /model picker's own test — CheckCredential, no
// network — lifted to "is there anything to pick": some provider's
// credential passes. A credential belongs to a provider, so one probe per
// provider covers all its models.
func anyProviderUsable(reg *provider.Registry) bool {
	seen := map[string]bool{}
	for _, mod := range reg.Models() {
		if seen[mod.Provider] {
			continue
		}
		seen[mod.Provider] = true
		if reg.CheckCredential(mod.Qualified()) == nil {
			return true
		}
	}
	return false
}

// printlnSetupNotice prints the setup line (the guidance, or the error that
// replaces it).
func (m *model) printlnSetupNotice() tea.Cmd { return m.println(m.setupNotice()) }

// setupModelHint follows a successful login in the unconfigured TUI: the
// credential alone starts nothing — a model is still missing, so the picker
// opens right away (a cancelled picker leaves /model for later).
func (m *model) setupModelHint() tea.Cmd {
	if !m.unconfigured() {
		return nil
	}
	return m.runModelSetup("")
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
	slog.Info("setup: first session", "model", q)
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

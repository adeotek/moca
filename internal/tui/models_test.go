package tui

// Tests for the model-list surfaces: /clear-models-cache and the active-provider
// filtering of /model (SPECS §6, §3; Review Focus 7).

import (
	"errors"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/provider"
)

// The clear re-reads the current model's provider off the loop (a network
// call), then reports the counts and what the re-read found.
func TestClearModelsCacheCommand(t *testing.T) {
	m := newAgentModel(t)
	cmd := m.runCommand(Parsed{Kind: KindCommand, Name: "clear-models-cache"})
	if cmd == nil || !strings.Contains(m.status.Transient, "fake") {
		t.Fatalf("the re-read runs off the loop, its status line naming the provider: %q", m.status.Transient)
	}
	msg, ok := cmd().(modelsClearedMsg)
	if !ok {
		t.Fatalf("want a modelsClearedMsg, got %#v", msg)
	}
	_, cmd = m.Update(msg)
	out, _ := simulate(m, cmd)
	if m.status.Transient != "" {
		t.Fatalf("the status line clears: %q", m.status.Transient)
	}
	// fake declares its models: there is no list to re-read.
	for _, want := range []string{"models cache cleared", "providers", "models", "fake has no model list to re-read"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report lacks %q: %q", want, out)
		}
	}
}

func TestClearModelsCacheReportsTheRescan(t *testing.T) {
	m := newAgentModel(t)
	_, cmd := m.Update(modelsClearedMsg{rep: provider.ClearReport{Providers: 2, Models: 40,
		Rescan: provider.ProviderModels{Provider: "openrouter", Models: []string{"a/b", "c/d"}}}})
	if out, _ := simulate(m, cmd); !strings.Contains(out, "2 providers, 40 models") || !strings.Contains(out, "openrouter re-read now (2 models)") {
		t.Fatalf("success report: %q", out)
	}
	_, cmd = m.Update(modelsClearedMsg{rep: provider.ClearReport{Providers: 1, Models: 3,
		Rescan: provider.ProviderModels{Provider: "openrouter", Err: errors.New("dial tcp: timeout")}}})
	if out, _ := simulate(m, cmd); !strings.Contains(out, "openrouter could not be re-read (dial tcp: timeout)") || !strings.Contains(out, "stay as they were") {
		t.Fatalf("a failed re-read keeps the models: %q", out)
	}
	_, cmd = m.Update(modelsClearedMsg{err: errors.New("permission denied")})
	if out, _ := simulate(m, cmd); !strings.Contains(out, "permission denied") {
		t.Fatalf("a failed delete is an error: %q", out)
	}
}

func TestClearModelsCacheRefusesMidRun(t *testing.T) {
	m := newAgentModel(t)
	m.running = true
	out := drained(m, tuiCmd(t, m, "/clear-models-cache"))
	if !strings.Contains(out, "finish or interrupt") {
		t.Fatalf("a run must refuse the clear: %q", out)
	}
}

func TestClearModelsCacheIsDiscoverable(t *testing.T) {
	// The command must be in the help table and the completion list, or it is
	// a feature only its author can find.
	if help := builtinHelp["clear-models-cache"]; !strings.Contains(help, "model lists") {
		t.Fatalf("help table: %q", help)
	}
	found := false
	for _, c := range BuiltinCommands {
		if c == "clear-models-cache" {
			found = true
		}
	}
	if !found {
		t.Fatal("not in BuiltinCommands")
	}
}

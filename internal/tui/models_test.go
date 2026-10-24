package tui

// Tests for the model-list surfaces: /clear-models-cache and the active-provider
// filtering of /model (SPECS §6, §3; Review Focus 7).

import (
	"strings"
	"testing"
)

func TestClearModelsCacheCommand(t *testing.T) {
	m := newAgentModel(t)
	out := drained(m, tuiCmd(t, m, "/clear-models-cache"))
	if !strings.Contains(out, "models cache cleared") {
		t.Fatalf("scrollback %q", out)
	}
	if !strings.Contains(out, "providers") || !strings.Contains(out, "models") {
		t.Fatalf("the report names both counts: %q", out)
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

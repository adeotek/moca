package config

// The exposure switch and the inspection budget (SPECS §4/§10.6/§11.4). The
// rules under test are the layer precedence ones: the user widens, a trusted
// project only narrows, and a project alone never enables anything.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strp(s string) string { return s }
func intp(n int) *int      { return &n }

func TestResolveCodemodeDefaults(t *testing.T) {
	on := boolp(true)
	st := ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: on}})
	if !st.On || st.Exposure != CodemodeAdditive || st.Attempts != 400 {
		t.Fatalf("defaults = %+v, want additive/400", st)
	}
	tr := strp(CodemodeTransport)
	st = ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: on, Exposure: &tr}})
	if st.Exposure != CodemodeTransport || st.Attempts != 400 {
		t.Fatalf("transport = %+v, want transport/400", st)
	}
	// An explicit budget overrides either default.
	n := intp(120)
	st = ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: on, Exposure: &tr, InspectAttempts: n}})
	if st.Attempts != 120 {
		t.Fatalf("explicit attempts = %d, want 120", st.Attempts)
	}
	// Off carries no exposure.
	st = ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: boolp(false), Exposure: &tr}})
	if st.On || st.Exposure != "" {
		t.Fatalf("off = %+v, want no exposure", st)
	}
}

func TestResolveCodemodeProjectNarrowsOnly(t *testing.T) {
	on := boolp(true)
	tr, ad := strp(CodemodeTransport), strp(CodemodeAdditive)

	// A project additive vetoes a user transport, and says so.
	st := ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on, Exposure: &tr},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{Exposure: &ad}},
	})
	if st.Exposure != CodemodeAdditive || st.Attempts != 400 || !strings.Contains(st.Note, "exposure additive") {
		t.Fatalf("project veto = %+v", st)
	}

	// A project transport alone cannot widen: it warns, like network's.
	st = ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{Exposure: &tr}},
	})
	if st.Exposure != CodemodeAdditive || !strings.Contains(st.Warning, "only your user config can") {
		t.Fatalf("project widening = %+v", st)
	}
}

func TestResolveCodemodeProjectMayLowerTheBudgetOnly(t *testing.T) {
	on := boolp(true)
	low, high := intp(80), intp(1000)

	// Both surfaces default to 400; the project lowers to 80.
	tr := strp(CodemodeTransport)
	st := ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on, Exposure: &tr},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{InspectAttempts: low}},
	})
	if st.Attempts != 80 {
		t.Fatalf("project lowering = %d, want 80", st.Attempts)
	}
	// A project cannot raise it above the user's effective budget.
	st = ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{InspectAttempts: high}},
	})
	if st.Attempts != 400 {
		t.Fatalf("project raising = %d, want the user's 400", st.Attempts)
	}
}

func TestValidateCodemodeBlock(t *testing.T) {
	on, off := boolp(true), boolp(false)
	tr, ad := strp(CodemodeTransport), strp(CodemodeAdditive)
	bad := strp("code-only")
	lo, hi := intp(InspectAttemptsMin-1), intp(InspectAttemptsMax+1)

	for _, c := range []struct {
		name string
		cfg  Config
	}{
		{"unknown exposure", Config{Codemode: CodemodeConfig{Enabled: on, Exposure: &bad}}},
		{"attempts too low", Config{Codemode: CodemodeConfig{Enabled: on, InspectAttempts: lo}}},
		{"attempts too high", Config{Codemode: CodemodeConfig{Enabled: on, InspectAttempts: hi}}},
		{"transport without enabled", Config{Codemode: CodemodeConfig{Exposure: &tr}}},
		{"transport with enabled false", Config{Codemode: CodemodeConfig{Enabled: off, Exposure: &tr}}},
	} {
		if err := c.cfg.Validate(); err == nil {
			t.Fatalf("%s was accepted", c.name)
		}
	}
	base := Default()
	for _, cm := range []CodemodeConfig{
		{Enabled: on, Exposure: &tr},
		{Enabled: on, Exposure: &ad, InspectAttempts: intp(InspectAttemptsMax)},
		{Enabled: on},
	} {
		cfg := base
		cfg.Codemode = cm
		if err := cfg.Validate(); err != nil {
			t.Fatalf("valid config rejected: %v", err)
		}
	}
}

// The project file validates its own codemode block: a repository cannot smuggle
// in a value the resolver would then misread.
func TestProjectRejectsBadCodemode(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".moca"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".moca", "config.jsonc")
	write := func(body string) error {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadProject(dir)
		return err
	}
	if err := write(`{"codemode":{"exposure":"code-only"}}`); err == nil {
		t.Fatal("project accepted an unknown exposure")
	}
	if err := write(`{"codemode":{"inspectAttempts":9999}}`); err == nil {
		t.Fatal("project accepted an out-of-range budget")
	}
	if err := write(`{"codemode":{"enabled":true,"exposure":"transport"}}`); err != nil {
		t.Fatalf("project rejected a valid block: %v", err)
	}
}

// The workspace surface (SPECS §10.6): opt-in, workspace-relative, and with an
// edit bit only the user can grant.
func TestResolveCodemodeWorkspace(t *testing.T) {
	on, yes := boolp(true), boolp(true)
	ws := strp(CodemodeWorkspace)

	st := ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: on, Exposure: &ws}})
	if !st.On || st.Exposure != CodemodeWorkspace || st.Attempts != 400 || st.WorkspaceEdits {
		t.Fatalf("workspace = %+v, want workspace/400 without edits", st)
	}
	st = ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: on, Exposure: &ws, WorkspaceEdits: yes}})
	if !st.WorkspaceEdits {
		t.Fatalf("workspaceEdits did not resolve: %+v", st)
	}
	// The bit is meaningless on the other surfaces, even if the file sets it
	// (validation refuses that, but the resolver must not depend on validation
	// having run first).
	tr := strp(CodemodeTransport)
	st = ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: on, Exposure: &tr, WorkspaceEdits: yes}})
	if st.WorkspaceEdits {
		t.Fatalf("transport resolved with edit authority: %+v", st)
	}
}

func TestWorkspaceEditsAuthorityFollowsTheUser(t *testing.T) {
	on, yes, no := boolp(true), boolp(true), boolp(false)
	ws, ad := strp(CodemodeWorkspace), strp(CodemodeAdditive)

	// A project may revoke the user's edits and must say so.
	st := ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on, Exposure: &ws, WorkspaceEdits: yes},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{WorkspaceEdits: no}},
	})
	if st.WorkspaceEdits || !strings.Contains(st.Note, "workspace edits are off in this project") {
		t.Fatalf("project revocation = %+v", st)
	}
	// A project asking for edits the user did not grant warns, never grants.
	st = ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on, Exposure: &ws},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{WorkspaceEdits: yes}},
	})
	if st.WorkspaceEdits || !strings.Contains(st.Warning, "only your user config can") {
		t.Fatalf("project widening = %+v", st)
	}
	// A project can narrow the surface back to additive, which carries no edits.
	st = ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on, Exposure: &ws, WorkspaceEdits: yes},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{Exposure: &ad}},
	})
	if st.Exposure != CodemodeAdditive || st.WorkspaceEdits || !strings.Contains(st.Note, "workspace is off in this project") {
		t.Fatalf("project narrowing = %+v", st)
	}
	// A project may not move the user to workspace.
	st = ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: on},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{Exposure: &ws}},
	})
	if st.Exposure != CodemodeAdditive || !strings.Contains(st.Warning, "workspace surface") {
		t.Fatalf("project surface widening = %+v", st)
	}
}

// The combinations a file may not state (each would resolve to a surface its
// own keys contradict).
func TestValidateWorkspaceBlock(t *testing.T) {
	on, off, yes := boolp(true), boolp(false), boolp(true)
	ws, ad := strp(CodemodeWorkspace), strp(CodemodeAdditive)
	bad := []struct {
		name string
		cm   CodemodeConfig
	}{
		{"workspace without enabled", CodemodeConfig{Exposure: &ws}},
		{"workspace with enabled false", CodemodeConfig{Enabled: off, Exposure: &ws}},
		{"workspace with network", CodemodeConfig{Enabled: on, Exposure: &ws, Network: yes}},
		{"edits without workspace", CodemodeConfig{Enabled: on, Exposure: &ad, WorkspaceEdits: yes}},
		{"edits alone", CodemodeConfig{Enabled: on, WorkspaceEdits: yes}},
	}
	for _, c := range bad {
		cfg := Default()
		cfg.Codemode = c.cm
		if err := cfg.Validate(); err == nil {
			t.Fatalf("%s was accepted", c.name)
		}
	}
	// The valid forms, including a project's bare revocation.
	good := []CodemodeConfig{
		{Enabled: on, Exposure: &ws},
		{Enabled: on, Exposure: &ws, WorkspaceEdits: yes},
		{Enabled: on, Exposure: &ws, WorkspaceEdits: boolp(false)},
	}
	for _, cm := range good {
		cfg := Default()
		cfg.Codemode = cm
		if err := cfg.Validate(); err != nil {
			t.Fatalf("valid workspace block rejected: %v", err)
		}
	}
	if err := (CodemodeConfig{WorkspaceEdits: boolp(false)}).validate("codemode"); err != nil {
		t.Fatalf("a bare project revocation was refused: %v", err)
	}
}

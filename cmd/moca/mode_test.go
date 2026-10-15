// cmd/moca/mode_test.go
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// ResolveMode resolution order (§3): an explicit --mode wins — plan maps to
// the plan scope at the default mode — else a trusted project file's
// defaultMode, else the config's resolved mode. A configured auto/yolo
// under --mode plan yields to plan (default), so it never counts as yolo
// for the trust decision.
func TestResolveMode(t *testing.T) {
	cfgAuto := config.Config{Permissions: config.PermissionsConfig{DefaultMode: "auto"}}
	cases := []struct {
		name     string
		args     []string
		cfg      config.Config
		wantMode string
		wantPlan bool
	}{
		{"--mode auto", []string{"--mode", "auto", "-p", "x"}, config.Config{}, "auto", false},
		{"--mode yolo", []string{"--mode", "yolo", "-p", "x"}, config.Config{}, "yolo", false},
		{"--mode default beats cfg auto", []string{"--mode", "default", "-p", "x"}, cfgAuto, "default", false},
		{"--mode plan", []string{"--mode", "plan", "-p", "x"}, config.Config{}, "default", true},
		{"--mode plan over cfg yolo", []string{"--mode", "plan", "-p", "x"}, config.Config{Yolo: true}, "default", true},
		{"--mode plan over cfg defaultMode auto", []string{"--mode", "plan", "-p", "x"}, cfgAuto, "default", true},
		{"--mode default over cfg yolo", []string{"--mode", "default", "-p", "x"}, config.Config{Yolo: true}, "default", false},
		{"cfg defaultMode wins", []string{"-p", "x"}, cfgAuto, "auto", false},
		{"cfg yolo flag wins", []string{"-p", "x"}, config.Config{Yolo: true}, "yolo", false},
		{"defaults to default", []string{"-p", "x"}, config.Config{}, "default", false},
	}
	for _, c := range cases {
		o, err := parseArgs(c.args, nil)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		mode, plan := o.ResolveMode(c.cfg)
		if mode != c.wantMode || plan != c.wantPlan {
			t.Errorf("%s: (%q, %v), want (%q, %v)", c.name, mode, plan, c.wantMode, c.wantPlan)
		}
	}
}

// Project receipts: a trusted project file's defaultMode joins in — but
// only when no flag overrode it, and plan still rides on default.
func TestResolveModeProjectReceipts(t *testing.T) {
	proj := &config.ProjectConfig{Permissions: config.PermissionsConfig{DefaultMode: "auto"}}
	if mode, plan := (Options{}).ResolveMode(config.Config{}); mode != "default" || plan {
		t.Errorf("no project: (%q, %v)", mode, plan)
	}
	if mode, plan := (Options{}).ResolveMode(config.Config{Project: proj}); mode != "auto" || plan {
		t.Errorf("defaultMode auto overrides cfg default: (%q, %v)", mode, plan)
	}
	if mode, _ := (Options{Mode: ptr("default")}).ResolveMode(config.Config{Project: proj}); mode != "default" {
		t.Errorf("--mode default overrides project auto: %q", mode)
	}
	if mode, plan := (Options{Mode: ptr("plan")}).ResolveMode(config.Config{Project: proj}); mode != "default" || !plan {
		t.Errorf("--mode plan over project auto: (%q, %v), want (default, true)", mode, plan)
	}
}

func ptr(s string) *string { return &s }

// The old --plan/--yolo/--no-yolo flags are gone: flag parsing refuses them
// (exit 2 at the caller) and the error names the undefined flag.
func TestRemovedModeFlags(t *testing.T) {
	for _, flag := range []string{"--plan", "--yolo", "--no-yolo"} {
		if _, err := parseArgs([]string{flag, "-p", "x"}, nil); err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Errorf("%s: want 'flag provided but not defined', got %v", flag, err)
		}
	}
}

// TestLoadProjectIgnoredWhenUntrusted: a malformed .moca/config.jsonc must
// only matter for a trusted run (§7): untrusted (-p default) starts
// normally with no project rules; the same workdir with --approve fails
// with a config error naming the file.
func TestLoadProjectIgnoredWhenUntrusted(t *testing.T) {
	srv := fakeCompletions(t, 200, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, ".moca"), 0o755); err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(work, ".moca", "config.jsonc")
	os.WriteFile(proj, []byte(`{"permissions":{"bogusKey":1}}`), 0o644)
	t.Chdir(work)
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)

	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 0 {
		t.Fatalf("untrusted must start normally: exit %d stderr %q", code, errb.String())
	}
	errb.Reset()
	if code := run(context.Background(), []string{"--config", cfg, "--approve", "-p", "x"}, nil, &out, &errb); code != exitUsage ||
		!strings.Contains(errb.String(), ".moca/config.jsonc") {
		t.Fatalf("trusted must fail with exit 2 naming the file: exit %d stderr %q", code, errb.String())
	}
}

// Plan runs at the default permission mode (rev 26): --mode plan resolves
// to (default, plan=true) whatever the config says — a configured yolo or
// project defaultMode never leaks through, so the run never counts as yolo
// for the trust decision either.
func TestPlanNeverResolvesWider(t *testing.T) {
	for _, cfg := range []config.Config{
		{Yolo: true},
		{Permissions: config.PermissionsConfig{DefaultMode: "yolo"}},
		{Permissions: config.PermissionsConfig{DefaultMode: "auto"}},
		{Project: &config.ProjectConfig{Permissions: config.PermissionsConfig{DefaultMode: "auto"}}},
	} {
		mode, plan := (Options{Mode: ptr("plan")}).ResolveMode(cfg)
		if mode != "default" || !plan {
			t.Fatalf("--mode plan over %+v → (%q, %v), want (default, true)", cfg.Permissions, mode, plan)
		}
	}
}

// A workdir needs a trust decision when it holds project resources: moca's
// own project dir/instructions and the ecosystem MCP configs a trusted
// project may load servers from (§4, §10.5).
func TestProjectResourcesTriggerTheTrustPrompt(t *testing.T) {
	if hasProjectResources(t.TempDir()) {
		t.Fatal("an empty dir needs no trust decision")
	}
	for _, name := range projectResourceNames {
		dir := t.TempDir()
		p := filepath.Join(dir, name)
		if name == ".moca" { // the project dir itself
			if err := os.MkdirAll(p, 0o700); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if !hasProjectResources(dir) {
			t.Errorf("%s must need a trust decision", name)
		}
	}
}

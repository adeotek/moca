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

func TestModeFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		cfg  config.Config
	}{
		{"--mode auto", []string{"--mode", "auto", "-p", "x"}, config.Config{}},
		{"--yolo", []string{"--yolo", "-p", "x"}, config.Config{}},
		{"--no-yolo overrides cfg yolo", []string{"--no-yolo", "-p", "x"}, config.Config{Yolo: true}},
		{"--no-yolo leaves auto", []string{"--no-yolo", "-p", "x"}, config.Config{Permissions: config.PermissionsConfig{DefaultMode: "auto"}}},
		{"cfg defaultMode wins", []string{"-p", "x"}, config.Config{Permissions: config.PermissionsConfig{DefaultMode: "auto"}}},
		{"cfg yolo flag wins", []string{"-p", "x"}, config.Config{Yolo: true}},
		{"defaults to default", []string{"-p", "x"}, config.Config{}},
		{"--mode beats cfg auto", []string{"--mode", "default", "-p", "x"}, config.Config{Permissions: config.PermissionsConfig{DefaultMode: "auto"}}},
	}
	for _, c := range cases {
		o, err := parseArgs(c.args, nil)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		got, err := o.ModeOf(c.cfg)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		want := "default"
		switch c.name {
		case "--mode auto", "--no-yolo leaves auto", "cfg defaultMode wins":
			want = "auto"
		case "--yolo", "cfg yolo flag wins":
			want = "yolo"
		}
		if got != want {
			t.Errorf("%s: mode %q, want %q", c.name, got, want)
		}
	}
	// Project receipts
	proj := &config.PermissionsConfig{DefaultMode: "auto"}
	o, _ := parseArgs([]string{"-p", "x"}, nil)
	if m, _ := o.ModeOf(config.Config{}); m != "default" {
		t.Errorf("no project: %q", m)
	}
	if m, err := (Options{}).ModeOf(config.Config{Project: proj}); err != nil || m != "auto" {
		t.Errorf("defaultMode auto overrides cfg default: %q %v", m, err)
	}
	if m, _ := (Options{Mode: ptr("default")}).ModeOf(config.Config{Project: proj}); m != "default" {
		t.Errorf("--mode default overrides project auto: %q", m)
	}
	if m, _ := (Options{Yolo: ptrT(true)}).ModeOf(config.Config{Project: proj}); m != "yolo" {
		t.Errorf("--yolo: %q", m)
	}
	// Usage errors
	for _, args := range [][]string{
		{"--mode", "bogus", "-p", "x"},
		{"--yolo", "--mode", "auto", "-p", "x"},
		{"--no-yolo", "--mode", "auto", "-p", "x"},
	} {
		if _, err := parseArgs(args, nil); err == nil || !strings.Contains(err.Error(), "mode") {
			t.Errorf("%v: want a usage error, got %v", args, err)
		}
	}
}

func ptr(s string) *string { return &s }
func ptrT(b bool) *bool    { return &b }

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

// Yolo and plan are mutually exclusive: --plan with an explicit yolo flag
// is a usage error, and a configured yolo yields to --plan (default).
func TestPlanExcludesYolo(t *testing.T) {
	for _, args := range [][]string{{"--plan", "--yolo"}, {"--plan", "--mode", "yolo"}} {
		if _, err := parseArgs(args, nil); err == nil {
			t.Fatalf("%v must be refused", args)
		}
	}
	o, err := parseArgs([]string{"--plan", "--mode", "auto"}, nil)
	if err != nil {
		t.Fatalf("--plan --mode auto: %v", err)
	}
	if m, _ := o.ModeOf(config.Config{}); m != "auto" {
		t.Fatalf("--plan --mode auto → %q", m)
	}
	o, _ = parseArgs([]string{"--plan"}, nil)
	if m, _ := o.ModeOf(config.Config{Yolo: true}); m != "default" {
		t.Fatalf("--plan over config yolo → %q, want default", m)
	}
	if m, _ := o.ModeOf(config.Config{Permissions: config.PermissionsConfig{DefaultMode: "yolo"}}); m != "default" {
		t.Fatalf("--plan over defaultMode yolo → %q, want default", m)
	}
}

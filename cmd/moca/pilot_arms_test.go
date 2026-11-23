package main

// The pilot's arms are two config files whose ONLY decoded difference is the
// `codemode` block. They shipped as byte-identical placeholders while the key
// did not exist; nothing noticed when it landed, which would have made a Stage A
// pilot compare a control against a control. This test is the notice.

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

func loadArm(t *testing.T, scenario, variant string) map[string]any {
	t.Helper()
	path := filepath.Join(repoRoot, "test", "evals", scenario, "config."+variant+".jsonc")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("%s/%s: %v", scenario, variant, err)
	}
	if cfg.Codemode.Enabled == nil && variant == "cm-on" {
		t.Fatalf("%s/%s: cm-on has no codemode block — the arm measures nothing", scenario, variant)
	}
	if cfg.Codemode.Enabled != nil && variant == "cm-off" {
		t.Fatalf("%s/%s: the control carries a codemode block", scenario, variant)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPilotArmsDifferOnlyInCodemode(t *testing.T) {
	for _, scenario := range []string{"sweep", "sweep-search"} {
		t.Run(scenario, func(t *testing.T) {
			on := loadArm(t, scenario, "cm-on")
			off := loadArm(t, scenario, "cm-off")
			if on["codemode"] == nil {
				t.Fatal("cm-on is not the treatment")
			}
			delete(on, "codemode")
			delete(off, "codemode")
			if !reflect.DeepEqual(on, off) {
				t.Fatalf("the arms differ beyond codemode:\n on: %v\n off: %v", on, off)
			}
		})
	}
}

// Every eval config names the one model the corpus is measured on: a run that
// silently measured another model would be a label nobody can compare.
func TestEvalConfigsPinTheCorpusModel(t *testing.T) {
	const want = "opencode-go/deepseek-v4.1-flash"
	dirs, err := filepath.Glob(filepath.Join(repoRoot, "test", "evals", "*", "config*.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	shared := []string{
		filepath.Join(repoRoot, "test", "evals", "config.jsonc"),
		filepath.Join(repoRoot, "test", "evals", "config.on.jsonc"),
	}
	paths := append(dirs, shared...)
	if len(paths) < 10 {
		t.Fatalf("only %d eval configs found — the glob lost the corpus", len(paths))
	}
	for _, path := range paths {
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if cfg.Model != want {
			t.Fatalf("%s: model = %q, want %q", path, cfg.Model, want)
		}
	}
}

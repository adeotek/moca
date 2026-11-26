package main

// The pilot's arms are two config files whose ONLY decoded difference is the
// `codemode` block. They shipped as byte-identical placeholders while the key
// did not exist; nothing noticed when it landed, which would have made a Stage A
// pilot compare a control against a control. This test is the notice.

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
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

// The v2 pilot is a 2x2 factorial, so its four arms have to be four DIFFERENT
// configurations whose only difference is the codemode block — and inside that
// block, exactly the two knobs the design varies. A typo that made
// additive-400 identical to additive would silently drop a cell of the design,
// so the additive arm pins the 50 it was measured with (the product default moved
// to 400 afterwards) and transport's budget has to be visible in the resolved state
// rather than implied by the file.
func TestBulkPilotArmsAreTheFactorial(t *testing.T) {
	scenario := "sweep-bulk"
	arms := []struct {
		name, exposure string
		attempts       int
		enabled        bool
	}{
		{"cm-off", "", 0, false},
		{"cm-additive", "additive", 50, true},
		{"cm-additive-400", "additive", 400, true},
		{"cm-transport", "transport", 400, true},
	}
	var base map[string]any
	for i, arm := range arms {
		cfgPath := filepath.Join(repoRoot, "test", "evals", scenario, "config."+arm.name+".jsonc")
		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("%s: %v", arm.name, err)
		}
		// What the resolver would run with, not what the file happens to omit.
		res := config.ResolveCodemode(cfg)
		if res.On != arm.enabled || res.Exposure != arm.exposure ||
			(arm.enabled && res.Attempts != arm.attempts) {
			t.Fatalf("%s resolves to %+v, want enabled=%v exposure=%q attempts=%d",
				arm.name, res, arm.enabled, arm.exposure, arm.attempts)
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		delete(m, "codemode")
		if i == 0 {
			base = m
			continue
		}
		if !reflect.DeepEqual(m, base) {
			t.Fatalf("%s differs from cm-off beyond codemode:\n %v\n %v", arm.name, m, base)
		}
	}
}

// Every eval config names its model explicitly, and only two are allowed: the
// corpus model, and the second model for the authorised model arms. A run that
// silently measured another model would be a label nobody can compare.
func TestEvalConfigsPinTheCorpusModel(t *testing.T) {
	const want = "opencode-go/deepseek-v4.1-flash"
	const second = "opencode-go/glm-5.3-flash"
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
		expect := want
		if strings.Contains(filepath.Base(path), ".glm.") {
			expect = second
		}
		if cfg.Model != expect {
			t.Fatalf("%s: model = %q, want %q", path, cfg.Model, expect)
		}
	}
}

// The changeset campaign's three arms must be three DIFFERENT configurations
// whose only difference is the codemode block, and inside it exactly the two
// knobs the design varies: exposure (off | workspace) and the edit bit. A typo
// that made the edits arm identical to the read-only arm would measure the same
// cell twice and call it a treatment.
func TestChangesetPilotArms(t *testing.T) {
	for _, scenario := range []string{"changeset-pins", "changeset-registry"} {
		t.Run(scenario, func(t *testing.T) {
			arms := []struct {
				name, exposure string
				edits          bool
			}{
				{"cm-off", "", false},
				{"cm-workspace", config.CodemodeWorkspace, false},
				{"cm-workspace-edits", config.CodemodeWorkspace, true},
			}
			var base map[string]any
			for i, arm := range arms {
				raw := loadArm(t, scenario, arm.name)
				cm, _ := raw["codemode"].(map[string]any)
				delete(raw, "codemode")
				if i == 0 {
					base = raw
					if len(cm) > 0 {
						t.Fatalf("%s: the control carries a codemode block (%v)", arm.name, cm)
					}
					continue
				}
				if !reflect.DeepEqual(raw, base) {
					t.Fatalf("%s differs beyond codemode:\n %v\n %v", arm.name, raw, base)
				}
				if cm["enabled"] != true {
					t.Fatalf("%s: enabled = %v, want true", arm.name, cm["enabled"])
				}
				if cm["exposure"] != arm.exposure {
					t.Fatalf("%s: exposure = %v, want %q", arm.name, cm["exposure"], arm.exposure)
				}
				gotEdits, _ := cm["workspaceEdits"].(bool)
				if gotEdits != arm.edits {
					t.Fatalf("%s: workspaceEdits = %v, want %v", arm.name, gotEdits, arm.edits)
				}
			}
			// And the resolver agrees with the file: the edits arm is the only
			// one that resolves edit authority.
			editsCfg, err := config.Load(filepath.Join(repoRoot, "test", "evals", scenario, "config.cm-workspace-edits.jsonc"))
			if err != nil {
				t.Fatal(err)
			}
			if st := config.ResolveCodemode(editsCfg); !st.On || st.Exposure != config.CodemodeWorkspace || !st.WorkspaceEdits {
				t.Fatalf("cm-workspace-edits resolves to %+v", st)
			}
			roCfg, err := config.Load(filepath.Join(repoRoot, "test", "evals", scenario, "config.cm-workspace.jsonc"))
			if err != nil {
				t.Fatal(err)
			}
			if st := config.ResolveCodemode(roCfg); !st.On || st.WorkspaceEdits {
				t.Fatalf("cm-workspace resolves to %+v", st)
			}
		})
	}
}

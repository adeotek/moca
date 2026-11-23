package config

// ResolveCodemode's table (SPECS §11): the user config decides, a trusted
// project can veto or declare, and a project declaration only ever warns. The
// network rows follow the same precedence but exist only while codemode is on.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boolp(b bool) *bool { return &b }

func TestResolveCodemodeEnabled(t *testing.T) {
	proj := func(v *bool) *ProjectConfig {
		return &ProjectConfig{Codemode: CodemodeConfig{Enabled: v}}
	}
	cases := []struct {
		name       string
		user       *bool
		project    *ProjectConfig
		on         bool
		warn, note bool
	}{
		{"unset, trusted unset", nil, proj(nil), false, false, false},
		{"unset, project false", nil, proj(boolp(false)), false, false, false},
		{"unset, project true", nil, proj(boolp(true)), false, true, false},
		{"user false, project true", boolp(false), proj(boolp(true)), false, true, false},
		{"user false, project false", boolp(false), proj(boolp(false)), false, false, false},
		{"user true, project unset", boolp(true), proj(nil), true, false, false},
		{"user true, project true", boolp(true), proj(boolp(true)), true, false, false},
		{"user true, project false", boolp(true), proj(boolp(false)), false, false, true},
		{"user true, no trusted project", boolp(true), nil, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := ResolveCodemode(Config{Codemode: CodemodeConfig{Enabled: c.user}, Project: c.project})
			if st.On != c.on {
				t.Fatalf("On = %v, want %v", st.On, c.on)
			}
			if (st.Warning != "") != c.warn {
				t.Fatalf("Warning = %q, want present=%v", st.Warning, c.warn)
			}
			if (st.Note != "") != c.note {
				t.Fatalf("Note = %q, want present=%v", st.Note, c.note)
			}
			if st.Network {
				t.Fatal("network is on without being configured")
			}
			if st.Warning != "" && !strings.Contains(st.Warning, "codemode") {
				t.Fatalf("warning does not name the feature: %q", st.Warning)
			}
		})
	}
}

// F20: the explicit user false must still warn, and it must say "sets it false"
// rather than "does not enable it".
func TestResolveCodemodeExplicitFalseWarns(t *testing.T) {
	st := ResolveCodemode(Config{
		Codemode: CodemodeConfig{Enabled: boolp(false)},
		Project:  &ProjectConfig{Codemode: CodemodeConfig{Enabled: boolp(true)}},
	})
	if st.Warning == "" || !strings.Contains(st.Warning, "sets it false") {
		t.Fatalf("warning = %q", st.Warning)
	}
}

func TestResolveCodemodeNetwork(t *testing.T) {
	proj := func(enabled, network *bool) *ProjectConfig {
		return &ProjectConfig{Codemode: CodemodeConfig{Enabled: enabled, Network: network}}
	}
	on := boolp(true)
	cases := []struct {
		name        string
		user        CodemodeConfig
		project     *ProjectConfig
		on, network bool
		warn, note  bool
	}{
		{"user network unset", CodemodeConfig{Enabled: on}, proj(nil, nil), true, false, false, false},
		{"user network false", CodemodeConfig{Enabled: on, Network: boolp(false)}, proj(nil, nil), true, false, false, false},
		{"user network true", CodemodeConfig{Enabled: on, Network: on}, proj(nil, nil), true, true, false, false},
		{"user network true, project true", CodemodeConfig{Enabled: on, Network: on}, proj(nil, on), true, true, false, false},
		{"user network true, project false", CodemodeConfig{Enabled: on, Network: on}, proj(nil, boolp(false)), true, false, false, true},
		{"user network unset, project true", CodemodeConfig{Enabled: on}, proj(nil, on), true, false, true, false},
		// A project network key decides nothing while the feature is off.
		{"off, project network true", CodemodeConfig{}, proj(nil, on), false, false, false, false},
		{"off by veto, project network true", CodemodeConfig{Enabled: on}, proj(boolp(false), on), false, false, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := ResolveCodemode(Config{Codemode: c.user, Project: c.project})
			if st.On != c.on || st.Network != c.network {
				t.Fatalf("On=%v Network=%v, want %v/%v", st.On, st.Network, c.on, c.network)
			}
			if (st.Warning != "") != c.warn {
				t.Fatalf("Warning = %q, want present=%v", st.Warning, c.warn)
			}
			if (st.Note != "") != c.note {
				t.Fatalf("Note = %q, want present=%v", st.Note, c.note)
			}
		})
	}
}

// The switch is a two-key block in a strict-decoded config: an unknown key
// inside it is an error in both layers, not a silently ignored typo.
func TestCodemodeBlockRejectsUnknownKeys(t *testing.T) {
	if _, err := Parse([]byte(`{"codemode":{"enabeld":true}}`)); err == nil || !strings.Contains(err.Error(), "enabeld") {
		t.Fatalf("user config: err = %v", err)
	}
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, ".moca"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ProjectConfigFile(wd), []byte(`{"codemode":{"enabeld":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(wd); err == nil || !strings.Contains(err.Error(), "enabeld") {
		t.Fatalf("project config: err = %v", err)
	}
}

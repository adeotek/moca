package agent

// Pins for PR B: `permissions.allow` is a rule list, not a list of bare names.
//
// Running it through the legacy shell.allow filter (which accepts single
// literal words only) silently dropped every entry containing a wildcard or an
// argument, so a documented `rm build/*` rule was inert: the command asked
// every time while the config claimed it was allowed.

import (
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/permissions"
)

func TestPermissionsAllowKeepsFullRules(t *testing.T) {
	cfg := config.Default()
	cfg.Project = nil
	cfg.Permissions.Allow = []string{"rm build/*", "make test CC=gcc"}

	rs, err := rules(cfg, false)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	j, err := permissions.NewJail(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := permissions.NewShell(rs, j, "linux")

	// A wildcard rule is live: the ask-every-time `rm` class is lifted for a
	// path the rule names.
	for _, cmd := range []string{"rm build/x", "rm build/sub/y"} {
		need, every, unsure, err := s.Check(cmd)
		if err != nil || len(need)+len(every)+len(unsure) > 0 {
			t.Errorf("Check(%q) asked (%v/%v/%v, err=%v); the `rm build/*` rule should cover it", cmd, need, every, unsure, err)
		}
	}
	// …and it does not cover a path it does not name.
	if need, every, unsure, err := s.Check("rm src/main.go"); err != nil || len(need)+len(every)+len(unsure) == 0 {
		t.Errorf("Check(rm src/main.go) was allowed; the rule names build/* only")
	}

	// A named rule with arguments parses too (and is what waives a
	// PATH-resolving make override, §11.4).
	if _, err := rules(cfg, false); err != nil {
		t.Fatalf("a named allow rule must parse: %v", err)
	}
}

func TestShellAllowStillFiltersBareNames(t *testing.T) {
	cfg := config.Default()
	cfg.Project = nil
	cfg.Shell.Allow = []string{"git", "rm build/*"}
	cfg.Permissions.Allow = nil

	rs, err := rules(cfg, false)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	j, err := permissions.NewJail(t.TempDir(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := permissions.NewShell(rs, j, "linux")

	// The legacy list keeps its old contract: `git` is accepted, a rule with
	// whitespace is not a command name and is dropped there (that is what
	// permissions.allow is for).
	if need, every, unsure, err := s.Check("git status"); err != nil || len(need)+len(every)+len(unsure) > 0 {
		t.Errorf("git status: %v/%v/%v err=%v, want allowed", need, every, unsure, err)
	}
	if _, every, _, err := s.Check("rm build/x"); err != nil || len(every) == 0 {
		t.Errorf("rm build/x: every=%v err=%v, want the ask-every-time class (the rule was dropped)", every, err)
	}
}

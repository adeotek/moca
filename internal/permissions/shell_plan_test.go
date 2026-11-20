package permissions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// planTestShell builds the plan-mode configuration: the default ladder's ask
// view, the workdir jail, and — when on — the plan zone installed the way the
// agent installs it.
func planTestShell(t *testing.T, on bool) (*Shell, string) {
	t.Helper()
	root := t.TempDir()
	j, err := NewJail(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := NewShell(RulesFromAllow([]string{"git", "go", "make", "ls", "tee"}), j, "linux")
	if on {
		s.SetPlanDir(filepath.Join(root, "docs", "plans"))
	}
	return s, root
}

// TestPlanZoneConfinesShellWriteChannels: with plan mode on, every target a
// redirect, tee or `time -o` would write must be a markdown file under
// docs/plans — the shell-side half of the plan confinement the review found
// missing. The commands themselves still run under the permission ladder.
func TestPlanZoneConfinesShellWriteChannels(t *testing.T) {
	s, _ := planTestShell(t, true)
	cases := []struct {
		cmd     string
		wantErr string // "" = allowed; else a substring the refusal must carry
	}{
		// Write channels outside the zone: refused, before running.
		{"echo pwned > pwned-by-shell.txt", "docs/plans/*.md"},
		{"echo pwned >> src/main.go", "docs/plans/*.md"},
		{"echo x > out.txt", "docs/plans/*.md"},
		{"echo x > docs/plans/p.txt", "docs/plans/*.md"}, // markdown only
		{"echo x > docs/plans", "docs/plans/*.md"},       // the directory is not a file
		{"echo x | tee out.txt", "docs/plans/*.md"},
		{"/usr/bin/time -o out.txt ls", "docs/plans/*.md"},         // GNU time's output file
		{"echo x > docs/plans/../outside.md", "after a directory"}, // inner .. guard, still refused
		// Write channels inside the zone: allowed (the ladder still applies).
		{"echo x > docs/plans/p.md", ""},
		{"echo x >> docs/plans/p.md", ""},
		{"echo x > docs/plans/sub/p.md", ""},
		{"echo x | tee docs/plans/p.md", ""},
		{"/usr/bin/time -o docs/plans/p.md ls", ""},
		// Non-write redirects and plain commands: untouched.
		{"echo x > /dev/null", ""},
		{"ls > /dev/null 2>&1", ""},
		{"git log", ""},
	}
	for _, c := range cases {
		_, _, _, err := s.Check(c.cmd)
		if c.wantErr != "" {
			if err == nil {
				t.Errorf("%q: want a refusal containing %q, got none", c.cmd, c.wantErr)
			} else if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%q: refusal does not contain %q: %v", c.cmd, c.wantErr, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected refusal: %v", c.cmd, err)
		}
	}
}

// TestPlanZoneOffAllowsRedirects: plan mode off is the pre-existing behaviour
// — the same target that was refused above runs (the ladder decides).
func TestPlanZoneOffAllowsRedirects(t *testing.T) {
	s, _ := planTestShell(t, false)
	if _, _, _, err := s.Check("echo x > out.txt"); err != nil {
		t.Fatalf("plan mode off: %v", err)
	}
	// And clearing the zone restores it on a shell that had it on.
	s, _ = planTestShell(t, true)
	s.SetPlanDir("")
	if _, _, _, err := s.Check("echo x > out.txt"); err != nil {
		t.Fatalf("plan zone cleared: %v", err)
	}
}

// TestPlanZoneFollowsTheAutoView: AutoView copies the zone (auto never holds
// with plan, but the copy must not be the weaker view).
func TestPlanZoneFollowsTheAutoView(t *testing.T) {
	s, _ := planTestShell(t, true)
	auto := s.AutoView(s.jail)
	if _, _, _, err := auto.Check("echo x > out.txt"); err == nil {
		t.Fatal("AutoView dropped the plan zone")
	}
}

// TestPlanZoneRefusesThroughASymlink: a path spelled inside docs/plans that
// resolves outside it is refused — the zone is decided on the resolved path.
func TestPlanZoneRefusesThroughASymlink(t *testing.T) {
	s, root := planTestShell(t, true)
	if err := os.MkdirAll(filepath.Join(root, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "secret.txt"), filepath.Join(root, "docs", "plans", "escape.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, _, err := s.Check("echo x > docs/plans/escape.md"); err == nil {
		t.Fatal("a symlink out of the plan zone was allowed")
	}
}

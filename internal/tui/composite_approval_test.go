package tui

// The composite-approval UI contract (v0.8.0): one answer teaches every
// eligible constituent, the persistence is a single config transaction, and a
// failed write still leaves the running session's already-given answer in
// effect.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

func TestCompositeAllowAlwaysPersistsAllRules(t *testing.T) {
	m := newAgentModel(t)
	fixed := time.Now()
	m.now = func() time.Time { return fixed }
	m.lastTyped = fixed.Add(-time.Second)
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{
		q: tools.Question{
			Kind: "shell", Subject: "python3, perl", Detail: "python3 -c pass && perl -e 1",
			CanAlways: true, AllowNames: []string{"python3", "perl"},
		},
		reply: reply,
	})
	_, cmd := m.Update(key("a"))
	if got := <-reply; got != tools.AllowAlways {
		t.Fatalf("answer = %v", got)
	}
	if out := printed(cmd); strings.Count(out, "always allowing") != 1 {
		t.Fatalf("want one notice, got %q", out)
	}
	pc, err := config.LoadProject(m.start.Workdir)
	if err != nil {
		t.Fatal(err)
	}
	if got := pc.Permissions.Allow; len(got) != 2 || got[0] != "python3" || got[1] != "perl" {
		t.Fatalf("project rules = %v, want both constituents", got)
	}
	// The in-memory copies carry the same list, so a /clear restart honours it.
	// (The config's default allow rows are already there; the answer appends.)
	if got := m.start.Config.Shell.Allow; len(got) < 2 || got[len(got)-2] != "python3" || got[len(got)-1] != "perl" {
		t.Fatalf("in-memory rules = %v", got)
	}
	if b, _ := os.ReadFile(config.ProjectConfigFile(m.start.Workdir)); strings.Contains(string(b), "python3, perl") {
		t.Fatalf("an inert composite rule was saved:\n%s", b)
	}
}

// A failed config write saves nothing, but the session still honours the
// answer it already got (the reply is sent before the persist).
func TestCompositeAllowAlwaysSaveFailureKeepsTheAnswer(t *testing.T) {
	m := newAgentModel(t)
	fixed := time.Now()
	m.now = func() time.Time { return fixed }
	m.lastTyped = fixed.Add(-time.Second)
	if err := os.Chmod(m.start.Workdir, 0o555); err != nil {
		t.Skipf("cannot make the workdir read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(m.start.Workdir, 0o755) })
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{
		q: tools.Question{
			Kind: "shell", Subject: "python3, perl", CanAlways: true,
			AllowNames: []string{"python3", "perl"},
		},
		reply: reply,
	})
	_, cmd := m.Update(key("a"))
	if got := <-reply; got != tools.AllowAlways {
		t.Fatalf("answer = %v", got)
	}
	if _, err := os.Stat(config.ProjectConfigFile(m.start.Workdir)); err == nil {
		t.Fatal("a read-only workdir must not gain a rules file")
	}
	if out := printed(cmd); !strings.Contains(out, "not saved") {
		t.Fatalf("want a save error notice, got %q", out)
	}
}

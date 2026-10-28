package agent

// Pin for F-5: a Start that fails after the session writer exists left the file
// open — an fd leak per failed attempt, and the session's flock held until the
// process exited (on Windows the file cannot even be deleted). The failure is
// easy to reach: an environment-referenced web key that does not resolve.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/session"
)

func countFDs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skip("/proc is not available")
	}
	return len(ents)
}

func TestFailedStartDoesNotLeakTheSessionWriter(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fd counting reads /proc")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	if os.Getenv("MOCA_T_MISSING") != "" {
		t.Skip("the missing-key canary must be unset")
	}
	cfg, err := config.Parse([]byte(`{"model":"fake/m","providers":{"fake":{"baseUrl":"http://127.0.0.1:1",
		"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":65536}}}},
		"web":{"search":{"provider":"exa","apiKey":"env:MOCA_T_MISSING"}}}`))
	if err != nil {
		t.Fatal(err)
	}

	before := countFDs(t)
	for i := range 5 {
		_, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), Emit: func(Event) {}, Slug: "t"})
		if err == nil {
			t.Fatalf("attempt %d: expected Start to fail on the unresolvable web key", i)
		}
	}
	if after := countFDs(t); after > before {
		t.Fatalf("failed Start leaked file descriptors: %d → %d over 5 attempts (want no growth)", before, after)
	}

	// The half-written session file is gone too: a header-only session in
	// /resume's list is a run that never happened.
	dir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "moca", "sessions")
	ents, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		names := make([]string, len(ents))
		for i, e := range ents {
			names[i] = e.Name()
		}
		t.Fatalf("failed Start left %d session file(s) behind: %v", len(ents), names)
	}
}

// TestDeferredSessionWaitsForTheFirstPrompt (§8): the TUI's Start keeps the
// session off the disk — a model or mode switch is buffered — and the file
// appears, complete and in order, with the first turn.
func TestDeferredSessionWaitsForTheFirstPrompt(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTest(t, s, 40, func(o *StartOptions) { o.DeferSession = true })
	if a.Session().Created() {
		t.Fatal("a deferred Start must not create the session file")
	}
	dir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "moca", "sessions")
	if ents, err := os.ReadDir(dir); err == nil && len(ents) != 0 {
		t.Fatalf("sessions dir holds %d file(s) before any prompt", len(ents))
	}
	// Pre-conversation metadata stays in memory: switching effort and mode in
	// a TUI you then quit must leave nothing to resume.
	if _, err := a.SetEffort("high"); err != nil {
		t.Fatal(err)
	}
	a.SetMode(ModeYolo)
	if a.Session().Created() {
		t.Fatal("a model/mode switch must not create the session file")
	}
	if _, err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if !a.Session().Created() {
		t.Fatal("the first prompt must create the session file")
	}
	if !strings.Contains(a.Session().Path(), a.Session().ID8()) {
		t.Fatalf("path %q does not carry the id8 %q", a.Session().Path(), a.Session().ID8())
	}
	entries, err := session.ReadFile(a.Session().Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 || entries[0].Type != session.TypeSession {
		t.Fatalf("the header must lead: %+v", entries)
	}
	var types []string
	for _, e := range entries[1:] {
		types = append(types, e.Type)
	}
	// The buffered settings land before the turn that created the file.
	if len(types) < 4 || types[0] != session.TypeModelChange || types[1] != session.TypePermissionMode || types[2] != session.TypeMessage {
		t.Fatalf("entries after the header: %v", types)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].ParentID != entries[i-1].ID {
			t.Fatalf("chain broken at entry %d: %q → %q", i, entries[i].ParentID, entries[i-1].ID)
		}
	}
}

// TestDeferredSessionSurvivesAPlanRoundTrip (§8): leaving plan mode before the
// first prompt — the shift+tab cycle's plan→yolo step, or the picker's plan
// then ask — has no envelope to retract, so it appends no off note and the
// deferred session still writes nothing.
func TestDeferredSessionSurvivesAPlanRoundTrip(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40, func(o *StartOptions) { o.DeferSession = true })
	a.SetPlan(true)
	a.SetMode(ModeYolo) // drops plan through SetPlan(false)
	a.SetPlan(true)
	a.SetPlan(false)
	if a.Session().Created() {
		t.Fatal("a plan round trip before the first prompt must not create the session file")
	}
}

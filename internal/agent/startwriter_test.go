package agent

// Pin for F-5: a Start that fails after the session writer exists left the file
// open — an fd leak per failed attempt, and the session's flock held until the
// process exited (on Windows the file cannot even be deleted). The failure is
// easy to reach: an environment-referenced web key that does not resolve.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/adeotek/moca/internal/config"
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

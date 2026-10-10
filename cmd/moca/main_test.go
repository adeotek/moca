package main

import (
	"os"
	"testing"
)

// TestMain keeps every run() in this package away from the real state dir:
// run() opens the diagnostic log for TUI and -p runs.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "moca-state-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	os.Unsetenv("MOCA_LOG")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

package main

import (
	"os"
	"testing"

	"github.com/adeotek/moca/internal/testenv"
)

// TestMain keeps every run() in this package away from the host's real state.
// run() opens the diagnostic log for TUI and -p runs, so MOCA_LOG would point
// that log outside the tree; testenv.Main adds the rest of the isolation the
// other test binaries already get — a temp XDG root for the config, the auth
// store, the models cache and the session dirs, plus blanked built-in
// provider keys, since an ambient key makes a provider ready and sends
// discovery at its real endpoint from a unit test.
func TestMain(m *testing.M) {
	os.Unsetenv("MOCA_LOG")
	os.Exit(testenv.Main(m))
}

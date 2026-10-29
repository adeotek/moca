package main

import (
	"os"
	"path/filepath"
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

// TestLoadUserDotEnv pins the startup wiring: the file is read from the
// config dir, so a key there is in the environment before config.Load runs.
func TestLoadUserDotEnv(t *testing.T) {
	const key = "MOCA_TEST_DOTENV_WIRED"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	mocaDir := filepath.Join(dir, "moca")
	if err := os.MkdirAll(mocaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mocaDir, ".env"), []byte(key+"=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadUserDotEnv(); err != nil {
		t.Fatalf("loadUserDotEnv: %v", err)
	}
	if got := os.Getenv(key); got != "from-file" {
		t.Fatalf("%s = %q, want %q", key, got, "from-file")
	}
}

// Package testenv isolates a test binary from the host's real moca state. An
// ambient provider key makes a built-in provider ready, and discovery then
// calls its real endpoint from a unit test (slow, flaky, and it changes the
// model lists the tests assert); a real config, auth store, models cache or
// session dir must never be read or written by a test either.
//
// The key list is derived from config.BuiltinProviders, so a new built-in
// provider is blanked everywhere without touching a test.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// BuiltinKeyVars lists the environment variables the built-in providers'
// default `env:` references name, sorted and deduplicated (the two opencode
// tiers share one account key).
func BuiltinKeyVars() []string {
	var out []string
	for _, ref := range config.BuiltinProviders {
		if v, ok := strings.CutPrefix(ref, "env:"); ok && v != "" {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// BlankKeys empties every built-in provider key for the test's duration.
func BlankKeys(t testing.TB) {
	t.Helper()
	for _, k := range BuiltinKeyVars() {
		t.Setenv(k, "")
	}
}

// Main is a package TestMain body: the XDG config/data/state roots point at a
// temp tree, the built-in keys are blank, and the tree is removed after the
// run. It returns the exit code for os.Exit.
func Main(m *testing.M) int {
	root, err := os.MkdirTemp("", "moca-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: setup failed: %v\n", err)
		os.Exit(1)
	}
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	os.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	os.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	for _, k := range BuiltinKeyVars() {
		os.Setenv(k, "")
	}
	code := m.Run()
	os.RemoveAll(root)
	return code
}

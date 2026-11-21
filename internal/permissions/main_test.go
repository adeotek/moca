package permissions

import (
	"errors"
	"os"
	"testing"
)

// TestMain removes the one environment dependency in this package: the PowerShell
// parse. A Windows auto shell would otherwise spawn a real parser child on the first
// Check, so the suite would be slow on a host with pwsh and different on one without.
// Tests that pin the parse path inject their own parser, and the live arm
// (psparse_live_windows_test.go) calls winParseDefault explicitly.
func TestMain(m *testing.M) {
	winParse = func(string) (parsedScript, error) {
		return parsedScript{}, errors.New("no parser in tests")
	}
	os.Exit(m.Run())
}

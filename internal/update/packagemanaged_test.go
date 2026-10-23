package update

import (
	"errors"
	"strings"
	"testing"
)

// Package-manager installs must never be replaced in place: brew keeps the
// Cellar copy plus a symlink, scoop and winget keep a manifest and a shim, and
// the next upgrade undoes (or breaks) whatever `moca update` wrote. The guard
// is pure path matching, so it is cheap to pin exhaustively.
func TestPackageManaged(t *testing.T) {
	tests := []struct {
		exe     string
		manager string // "" = not managed
		upgrade string
	}{
		{exe: `/home/toor/.local/bin/moca`},
		{exe: `/usr/local/bin/moca`},    // a manual install, not brew's
		{exe: `/home/toor/go/bin/moca`}, // go install
		{exe: ``},
		{exe: `/opt/homebrew/Cellar/moca/0.7.8/bin/moca`, manager: "Homebrew", upgrade: "brew upgrade moca"},
		{exe: `/usr/local/Cellar/moca/0.7.8/bin/moca`, manager: "Homebrew", upgrade: "brew upgrade moca"},
		{exe: `/home/linuxbrew/.linuxbrew/Cellar/moca/0.7.8/bin/moca`, manager: "Homebrew", upgrade: "brew upgrade moca"},
		{exe: `/opt/homebrew/bin/moca`, manager: "Homebrew", upgrade: "brew upgrade moca"},
		{exe: `/home/toor/scoop/apps/moca/0.7.8/moca.exe`, manager: "Scoop", upgrade: "scoop update moca"},
		{exe: `/home/toor/scoop/shims/moca.exe`, manager: "Scoop", upgrade: "scoop update moca"},
		{exe: `C:\Users\ben\AppData\Local\Microsoft\WinGet\Packages\adeotek.moca_Microsoft.Winget.Source_8wekyb3d8bbwe\moca.exe`, manager: "WinGet", upgrade: "winget upgrade adeotek.moca"},
		{exe: `C:\Users\ben\AppData\Local\Microsoft\WinGet\Links\moca.exe`, manager: "WinGet", upgrade: "winget upgrade adeotek.moca"},
		{exe: `C:\Users\Ben\AppData\Local\MICROSOFT\WinGet\Packages\x\moca.exe`, manager: "WinGet", upgrade: "winget upgrade adeotek.moca"},
	}
	for _, tc := range tests {
		err := PackageManaged(tc.exe)
		if tc.manager == "" {
			if err != nil {
				t.Errorf("%q: want no manager, got %v", tc.exe, err)
			}
			continue
		}
		var pm *PackageManagedError
		if !errors.As(err, &pm) {
			t.Errorf("%q: want a PackageManagedError naming %s, got %v", tc.exe, tc.manager, err)
			continue
		}
		if pm.Manager != tc.manager || pm.Upgrade != tc.upgrade {
			t.Errorf("%q: got %s / %s, want %s / %s", tc.exe, pm.Manager, pm.Upgrade, tc.manager, tc.upgrade)
		}
		if pm.Path != tc.exe {
			t.Errorf("%q: the error names %q", tc.exe, pm.Path)
		}
		// The message is what a user sees instead of an update: it has to
		// name the manager's own command, or it is just a refusal.
		if msg := pm.Error(); !strings.Contains(msg, tc.upgrade) || !strings.Contains(msg, "--force") {
			t.Errorf("%q: message %q must name %q and --force", tc.exe, msg, tc.upgrade)
		}
	}
}

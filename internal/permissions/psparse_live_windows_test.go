//go:build windows

package permissions

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWindowsParseLive runs the real helper against the real PowerShell. It is opt-in
// (MOCA_WINPARSE=1) so a Windows CGO/race run does not pay a child process per test,
// and it is the only place the sidecar lifecycle and the non-execution property are
// checked against an interpreter rather than a fixture.
func TestWindowsParseLive(t *testing.T) {
	if os.Getenv("MOCA_WINPARSE") != "1" {
		t.Skip("set MOCA_WINPARSE=1 to run against the real PowerShell")
	}
	defer closeWinParser()

	ps, err := winParse("Get-ChildItem -Path . -Filter *.go | Select-Object Name\nRemove-Item -Recurse -Force .\\out > log.txt")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ps.Cmds) != 3 {
		t.Fatalf("cmds = %+v, want Get-ChildItem, Select-Object, Remove-Item", ps.Cmds)
	}
	if ps.Cmds[0].Name != "Get-ChildItem" || ps.Cmds[2].Name != "Remove-Item" {
		t.Errorf("names = %q, %q", ps.Cmds[0].Name, ps.Cmds[2].Name)
	}
	if len(ps.Writes) != 1 || ps.Writes[0].Target != "log.txt" {
		t.Fatalf("writes = %+v, want log.txt", ps.Writes)
	}
	if !ps.Cmds[0].Static {
		t.Error("Get-ChildItem Static = false, want true")
	}

	// The second parse must reuse the same child: that is the whole point of the
	// sidecar (293 ms vs 67 ms per parse on PowerShell 5.1).
	psDefault.mu.Lock()
	first := 0
	if psDefault.cmd != nil && psDefault.cmd.Process != nil {
		first = psDefault.cmd.Process.Pid
	}
	psDefault.mu.Unlock()
	if _, err := winParse("Get-ChildItem"); err != nil {
		t.Fatalf("second parse: %v", err)
	}
	psDefault.mu.Lock()
	second := 0
	if psDefault.cmd != nil && psDefault.cmd.Process != nil {
		second = psDefault.cmd.Process.Pid
	}
	psDefault.mu.Unlock()
	if first == 0 || first != second {
		t.Errorf("pids = %d then %d, want one long-lived child", first, second)
	}

	// Non-execution: the parse of a destructive command must not delete anything.
	dir := t.TempDir()
	canary := filepath.Join(dir, "canary.txt")
	if err := os.WriteFile(canary, []byte("alive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := winParse(`Remove-Item -Recurse -Force "` + dir + `"`); err != nil {
		t.Fatalf("destructive parse: %v", err)
	}
	if _, err := os.Stat(canary); err != nil {
		t.Fatalf("the parse executed the command: %v", err)
	}
}

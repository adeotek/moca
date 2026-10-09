//go:build windows

package update

import (
	"fmt"
	"os"
)

// replace swaps the new binary in place of exe. Windows cannot overwrite a
// running image, but it can rename it: the old binary is moved aside
// (moca.exe.old — removed by the next update run, since it stays locked
// while this process lives), then the new one takes its place.
func replace(exe, newPath string) error {
	old := exe + ".old"
	_ = os.Remove(old) // leftover from a previous update
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("cannot move the running %s aside (close all moca instances and retry): %w", exe, err)
	}
	if err := os.Rename(newPath, exe); err != nil {
		_ = os.Rename(old, exe) // put the old one back
		return err
	}
	_ = os.Remove(old) // best effort; fails while this process runs from it
	return nil
}

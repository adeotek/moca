//go:build !windows

package update

import "os"

// replace swaps the new binary in place of exe. Both live in the same
// directory, so a rename is atomic and safe even while the old image is
// executing (the running process keeps the old inode until it exits).
func replace(exe, newPath string) error {
	return os.Rename(newPath, exe)
}

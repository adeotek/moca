package session

import (
	"errors"
	"fmt"
	"os"
)

// Delete removes a stored session file. The single-writer lock decides: a
// session another moca process holds open — the current process's own open
// session included; flock conflicts between separate descriptors even within
// one process — returns ErrInUse and nothing is removed. Deleting the file
// out from under a live writer is never what the user wants. On platforms
// without flock that rule is enforced only where the OS refuses the unlink
// itself (e.g. a Windows sharing violation).
func Delete(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		if errors.Is(err, ErrInUse) {
			return ErrInUse
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	if !lockSupported {
		// No lock was taken, and Windows refuses to unlink a file this very
		// process still holds open (no FILE_SHARE_DELETE): close it first.
		f.Close()
	}
	return os.Remove(path)
}

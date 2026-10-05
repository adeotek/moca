//go:build linux || darwin || freebsd || netbsd || openbsd

package session

import (
	"errors"
	"os"
	"syscall"
)

// lockSupported reports whether this platform enforces the single-writer rule.
const lockSupported = true

// lockFile takes a non-blocking exclusive advisory lock on f. The kernel drops
// it when the descriptor closes or the process dies, so a crashed moca never
// leaves a stale lock behind.
func lockFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return ErrInUse
		}
		return err
	}
}

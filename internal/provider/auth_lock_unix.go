//go:build !windows

package provider

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock, blocking until it is available.
// The returned func releases it. flock releases automatically when the
// process dies, so a crashed moca never wedges the store.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

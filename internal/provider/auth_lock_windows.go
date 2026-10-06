//go:build windows

package provider

import (
	"context"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx = kernel32.NewProc("LockFileEx")
	procUnlockFile = kernel32.NewProc("UnlockFileEx")
)

const (
	lockfileFailImmediately = 0x1
	lockfileExclusiveLock   = 0x2
	errorLockViolation      = syscall.Errno(33)
)

// lockFile takes an exclusive lock (LockFileEx), waiting until it is
// available or ctx is done (it polls with LOCKFILE_FAIL_IMMEDIATELY so the
// wait can be cancelled). The returned func releases it. The OS releases the
// lock when the handle closes, so a crashed moca never wedges the store.
func lockFile(ctx context.Context, path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	var ol syscall.Overlapped
	for wait := time.Millisecond; ; wait = min(wait*2, 50*time.Millisecond) {
		r, _, e := procLockFileEx.Call(f.Fd(), lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
		if r != 0 {
			break
		}
		if e != errorLockViolation {
			f.Close()
			return nil, e
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return func() {
		procUnlockFile.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
		f.Close()
	}, nil
}

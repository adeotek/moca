//go:build windows

package provider

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx = kernel32.NewProc("LockFileEx")
	procUnlockFile = kernel32.NewProc("UnlockFileEx")
)

const lockfileExclusiveLock = 0x2

// lockFile takes an exclusive lock (LockFileEx), blocking until it is
// available. The returned func releases it. The OS releases the lock when
// the handle closes, so a crashed moca never wedges the store.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	var ol syscall.Overlapped
	r, _, e := procLockFileEx.Call(f.Fd(), lockfileExclusiveLock, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		f.Close()
		return nil, e
	}
	return func() {
		procUnlockFile.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
		f.Close()
	}, nil
}

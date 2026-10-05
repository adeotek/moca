//go:build !(linux || darwin || freebsd || netbsd || openbsd)

package session

import "os"

// lockSupported is false where flock is unavailable: concurrent writers to one
// session file are then not detected.
const lockSupported = false

func lockFile(*os.File) error { return nil }

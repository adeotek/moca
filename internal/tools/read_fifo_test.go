//go:build !windows

package tools

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReadRefusesNonRegularFile(t *testing.T) {
	env, root := testEnv(t)
	fifo := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("mkfifo unavailable:", err)
	}
	r := run(t, readTool{}, env, map[string]any{"path": "pipe"})
	if !r.IsError || !strings.Contains(r.Content, "not a regular file") {
		t.Fatalf("%q", r.Content)
	}
}

//go:build !windows

package applog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenModes(t *testing.T) {
	keepDefault(t)
	dir := filepath.Join(t.TempDir(), "logs")
	c, path, err := Open(dir, LevelInfo, 30)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v", fi.Mode().Perm())
	}
}

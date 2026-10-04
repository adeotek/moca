package permissions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrustStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data", "trust.json")
	ts, err := LoadTrust(path)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "w")
	os.Mkdir(work, 0o755)
	if _, known := ts.Lookup(work); known {
		t.Fatal("empty store")
	}
	if err := ts.Set(work, true); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "lnk")
	os.Symlink(work, link)
	ts2, _ := LoadTrust(path)
	if tr, known := ts2.Lookup(link); !known || !tr {
		t.Fatal("canonical path lookup via symlink")
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}

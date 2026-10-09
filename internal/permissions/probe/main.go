package main

import (
	"fmt"
	"github.com/adeotek/moca/internal/permissions"
	"os"
	"path/filepath"
)

func main() {
	os.MkdirAll("/tmp/moca-int/inside", 0o755)
	os.RemoveAll("/tmp/moca-ext")
	os.MkdirAll("/tmp/moca-ext", 0o755)
	// existing real symlink OUT of the jail pointing to a real external file
	os.WriteFile("/tmp/moca-ext/secret.txt", []byte("ext-data"), 0o644)
	os.Symlink("/tmp/moca-ext/secret.txt", "/tmp/moca-int/link-out")
	// DANGLING symlink: points at non-existent external path
	os.Symlink("/tmp/moca-ext/newfile.txt", "/tmp/moca-int/link-dangle")
	// dangling symlink to external dir then subpath
	os.Symlink("/tmp/moca-ext2", "/tmp/moca-int/link-dir")

	j, _ := permissions.NewJail("/tmp/moca-int", nil, nil)
	for _, p := range []string{"link-out", "link-dangle", "link-dir/newfile"} {
		res, err := j.Resolve(p, true)
		if err != nil {
			fmt.Printf("resolve %-14s err=%v\n", p, err)
			continue
		}
		// simulate what guardedWrite does
		os.MkdirAll(filepath.Dir(res), 0o755)
		if err := os.WriteFile(res, []byte("pwned"), 0o644); err != nil {
			fmt.Printf("resolve %-14s ok=%s writeErr=%v\n", p, res, err)
			continue
		}
		b, _ := os.ReadFile("/tmp/moca-ext/newfile.txt")
		b2, _ := os.ReadFile("/tmp/moca-ext/secret.txt")
		fmt.Printf("resolve %-14s ok=%s ext-new=%q ext-secret=%q\n", p, res, b, b2)
	}
}

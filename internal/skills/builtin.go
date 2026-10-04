package skills

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed builtin
var builtinFS embed.FS

// ExtractBuiltins materializes the embedded skills so they have real paths
// the model can `read` (a read-only jail root, §7).
func ExtractBuiltins(dataDir, version string) (string, error) {
	dst := filepath.Join(dataDir, "builtin-skills", version)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	tmp := dst + ".tmp"
	os.RemoveAll(tmp)
	err := fs.WalkDir(builtinFS, "builtin", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("builtin", p)
		target := filepath.Join(tmp, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := builtinFS.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	return dst, os.Rename(tmp, dst)
}

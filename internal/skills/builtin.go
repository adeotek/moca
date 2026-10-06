package skills

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed builtin
var builtinFS embed.FS

// ExtractBuiltins materializes the embedded skills so they have real paths
// the model can `read` (a read-only jail root, §7). The directory is keyed
// on a content hash of the embedded files, so an edited built-in re-extracts
// on the next run, and the write-then-rename is safe against two processes
// racing on first use.
func ExtractBuiltins(dataDir string) (string, error) {
	return extractBuiltins(builtinFS, dataDir)
}

// extractBuiltins is ExtractBuiltins with an injectable filesystem. The
// directory is keyed on a content hash, so changed embedded content lands in
// a NEW directory on the next run (the old one is simply no longer
// referenced) — tests prove that with a MapFS.
func extractBuiltins(fsys fs.FS, dataDir string) (string, error) {
	h, err := builtinHash(fsys)
	if err != nil {
		return "", err
	}
	base := filepath.Join(dataDir, "builtin-skills")
	dst := filepath.Join(base, h)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(base, ".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp) // no-op after a successful rename
	err = fs.WalkDir(fsys, "builtin", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("builtin", p)
		target := filepath.Join(tmp, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		if _, serr := os.Stat(dst); serr == nil {
			return dst, nil // another process won the race
		}
		return "", err
	}
	return dst, nil
}

// builtinHash is a stable content hash of the embedded built-in skills.
func builtinHash(fsys fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(fsys, "builtin", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(b))
		h.Write(b)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

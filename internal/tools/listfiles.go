package tools

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ListFiles returns the regular files under root as slash paths relative to
// it, honoring .gitignore/.ignore the way search does (dot-entries are
// skipped), stopping after max entries. It exists for the TUI's @file
// completion: a bounded, cancellable walk that never reads file contents.
func ListFiles(ctx context.Context, root string, max int) []string {
	var out []string
	var rules []ignoreRule
	loadIgnores := func(dirAbs, rel string) {
		for _, n := range []string{".gitignore", ".ignore"} {
			if b, err := os.ReadFile(filepath.Join(dirAbs, n)); err == nil {
				rules = append(rules, parseIgnore(rel, b)...)
			}
		}
	}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return ctx.Err()
		}
		r, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel := ""
		if r != "." {
			rel = filepath.ToSlash(r)
		}
		switch {
		case p == root:
			loadIgnores(p, "")
			return nil
		case strings.HasPrefix(d.Name(), ".") || ignored(rules, rel, d.IsDir()):
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		case d.IsDir():
			loadIgnores(p, rel)
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		out = append(out, rel)
		if len(out) >= max {
			return fs.SkipAll
		}
		return nil
	})
	return out
}

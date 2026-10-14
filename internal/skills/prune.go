package skills

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// staleTmpAge is how old a `.tmp-*` extraction directory must be before it
// is treated as crash debris: a younger one may be another moca mid-extract.
const staleTmpAge = time.Hour

// Pruned is one entry `moca prune` removed (or, on a dry run, would remove).
type Pruned struct {
	Path  string
	Bytes int64 // regular-file bytes under Path (symlinks are not followed)
}

// PruneBuiltins removes everything in <dataDir>/builtin-skills except the
// directory this binary extracts to: older content hashes, the legacy
// version-keyed layout, stray files and crash-orphaned `.tmp-*` directories.
// Extraction keeps every previous directory (a newer binary lands in a new
// one) so a still-running older moca keeps reading its own; this is the
// explicit cleanup. It does not create the current directory — the next
// start extracts it. dryRun reports without removing. It attempts every
// entry and joins the errors.
func PruneBuiltins(dataDir string, dryRun bool) ([]Pruned, error) {
	return pruneBuiltins(builtinFS, dataDir, dryRun, time.Now())
}

// pruneBuiltins is PruneBuiltins with an injectable filesystem and clock.
func pruneBuiltins(fsys fs.FS, dataDir string, dryRun bool, now time.Time) ([]Pruned, error) {
	cur, err := builtinHash(fsys)
	if err != nil {
		return nil, err
	}
	base := filepath.Join(dataDir, "builtin-skills")
	ents, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Pruned
	var errs []error
	for _, e := range ents {
		if e.Name() == cur {
			continue
		}
		p := filepath.Join(base, e.Name())
		if strings.HasPrefix(e.Name(), ".tmp-") {
			if fi, err := e.Info(); err == nil && now.Sub(fi.ModTime()) < staleTmpAge {
				continue
			}
		}
		size := treeBytes(p)
		if !dryRun {
			// RemoveAll unlinks a symlink without following it.
			if err := os.RemoveAll(p); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		out = append(out, Pruned{Path: p, Bytes: size})
	}
	return out, errors.Join(errs...)
}

// treeBytes sums the regular files under p. WalkDir does not follow
// symlinks, so a link to a large tree counts as nothing.
func treeBytes(p string) int64 {
	var n int64
	filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

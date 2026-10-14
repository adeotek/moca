package skills

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// seedFS holds the standard skills moca plants in the user's skills dir on
// start (rtk, graphify). Unlike builtinFS they are NOT extracted per binary
// version: once planted they belong to the user, who can edit, replace or
// opt out of them (see SeedSkills).
//
//go:embed seed
var seedFS embed.FS

// SeedSkills creates each embedded seed skill under dir (the global skills
// dir) when nothing exists at dir/<name> yet, and returns the names it
// created. It is deliberately create-only: a skill that is already there —
// customised, an empty directory kept as an opt-out, a symlink — is left
// exactly as it is, and a newer binary never rewrites a seeded skill. Only
// the built-in skills (ExtractBuiltins) follow the binary.
func SeedSkills(dir string) ([]string, error) {
	return seedSkills(seedFS, dir)
}

// seedSkills is SeedSkills with an injectable filesystem. A failure on one
// skill does not stop the others; the errors are joined.
func seedSkills(fsys fs.FS, dir string) ([]string, error) {
	ents, err := fs.ReadDir(fsys, "seed")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var created []string
	var errs []error
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		ok, err := seedOne(fsys, e.Name(), dir)
		if err != nil {
			errs = append(errs, err)
		} else if ok {
			created = append(created, e.Name())
		}
	}
	return created, errors.Join(errs...)
}

// seedOne plants one skill and reports whether it created it. The tree is
// written to a hidden temp directory and renamed into place, so another moca
// starting at the same moment never sees a half-written skill (Discover
// ignores dot-directories, so neither does a crash leave one behind that
// could shadow the real skill).
func seedOne(fsys fs.FS, name, dir string) (bool, error) {
	dst := filepath.Join(dir, name)
	// Lstat, not Stat: a symlink is the user's arrangement and counts as
	// "exists" even when it dangles.
	if _, err := os.Lstat(dst); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	tmp, err := os.MkdirTemp(dir, ".seed-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)                      // no-op after a successful rename
	if err := os.Chmod(tmp, 0o755); err != nil { // MkdirTemp is 0700; skills are plain user files
		return false, err
	}
	root := "seed/" + name
	err = fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(tmp, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(p, root), "/")))
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
		return false, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		if _, serr := os.Lstat(dst); serr == nil {
			return false, nil // another process planted it first
		}
		return false, err
	}
	return true, nil
}

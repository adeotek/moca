package skills

import (
	"bytes"
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

// SeedUpdate is what UpdateSeedSkills did (or, with dryRun, would do) for one
// seeded skill.
type SeedUpdate struct {
	Name    string
	Created bool     // the skill did not exist and is (or would be) planted whole
	Changed []string // bundle files written, slash-separated and relative to the skill dir
	Kept    []string // files the user's copy carries that the bundle does not
	Skipped string   // why the copy was left alone ("symlink", "not a directory")
}

// UpdateSeedSkills rewrites the embedded seed skills under dir from the
// bundle — the explicit counterpart to SeedSkills' create-only rule, behind
// `moca update skills`. A bundle file that differs is replaced (atomically,
// per file), a bundle file the user deleted comes back, and a file the user
// added is kept: the skill directory is the user's, this only makes the
// bundled files follow the binary. A missing skill is planted whole. A
// symlinked skill, or a plain file where the skill directory should be, is
// reported and left alone — writing through a link would edit whatever it
// points at. With dryRun nothing is written and the result reports what would
// change.
func UpdateSeedSkills(dir string, dryRun bool) ([]SeedUpdate, error) {
	return updateSeedSkills(seedFS, dir, dryRun)
}

// updateSeedSkills is UpdateSeedSkills with an injectable filesystem. A
// failure on one skill does not stop the others; the errors are joined.
func updateSeedSkills(fsys fs.FS, dir string, dryRun bool) ([]SeedUpdate, error) {
	ents, err := fs.ReadDir(fsys, "seed")
	if err != nil {
		return nil, err
	}
	if !dryRun {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	var ups []SeedUpdate
	var errs []error
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		up, err := updateSeedOne(fsys, e.Name(), dir, dryRun)
		if err != nil {
			errs = append(errs, err)
		}
		ups = append(ups, up)
	}
	return ups, errors.Join(errs...)
}

// updateSeedOne brings one skill's bundled files up to date. Comparison is
// byte-wise; a write stages a sibling temp file and renames it, so a reader
// never sees a half-written file. Only the bundle's own names are written —
// anything else in the directory is the user's and is reported, never touched.
func updateSeedOne(fsys fs.FS, name, dir string, dryRun bool) (SeedUpdate, error) {
	up := SeedUpdate{Name: name}
	dst := filepath.Join(dir, name)
	root := "seed/" + name
	fi, err := os.Lstat(dst)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		var files []string
		if err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				files = append(files, strings.TrimPrefix(strings.TrimPrefix(p, root), "/"))
			}
			return nil
		}); err != nil {
			return up, err
		}
		up.Created, up.Changed = true, files
		if dryRun {
			return up, nil
		}
		_, err := seedOne(fsys, name, dir)
		return up, err
	case err != nil:
		return up, err
	case fi.Mode()&fs.ModeSymlink != 0:
		up.Skipped = "symlink"
		return up, nil
	case !fi.IsDir():
		up.Skipped = "not a directory"
		return up, nil
	}
	bundle := map[string]bool{}
	if err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, root), "/")
		bundle[rel] = true
		want, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		have, err := os.ReadFile(target)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err == nil && bytes.Equal(have, want) {
			return nil
		}
		up.Changed = append(up.Changed, rel)
		if dryRun {
			return nil
		}
		return writeSeedFile(target, want)
	}); err != nil {
		return up, err
	}
	if err := filepath.WalkDir(dst, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dst, p)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); !bundle[rel] {
			up.Kept = append(up.Kept, rel)
		}
		return nil
	}); err != nil {
		return up, err
	}
	return up, nil
}

// writeSeedFile writes one bundle file atomically (temp sibling + rename) so a
// concurrent reader sees either the old or the new bytes.
func writeSeedFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".seed-update-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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

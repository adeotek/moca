// Package permissions implements the path jail, shell command analysis and
// project trust. It implements interfaces declared in tools without
// importing it (§2).
package permissions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Jail struct {
	root     string
	readOnly []string
	// askWrite are user-owned roots outside the workdir where a write needs
	// the user's approval first (the global prompt templates: slash commands
	// the agent writes for the user). Resolve still refuses them; Approvable
	// hands them out so the caller can ask.
	askWrite []string
}

func NewJail(root string, readOnly, askWrite []string) (*Jail, error) {
	r, err := canonical(root)
	if err != nil {
		return nil, fmt.Errorf("jail root: %w", err)
	}
	j := &Jail{root: r}
	for _, ro := range readOnly {
		abs, err := filepath.Abs(ro)
		if err != nil {
			continue
		}
		// resolveDeep, not canonical: a read root that does not exist yet
		// (the global skills directory is routinely absent at session
		// start) must stay a root — EvalSymlinks would drop it and every
		// read under it would refuse once the directory appears.
		c, err := resolveDeep(abs)
		if err != nil {
			continue
		}
		j.readOnly = append(j.readOnly, c)
	}
	for _, aw := range askWrite {
		abs, err := filepath.Abs(aw)
		if err != nil {
			continue
		}
		c, err := resolveDeep(abs) // same reason as the read roots: may not exist yet
		if err != nil {
			continue
		}
		j.askWrite = append(j.askWrite, c)
	}
	return j, nil
}

func (j *Jail) Root() string { return j.root }

func canonical(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveDeep evaluates symlinks on the deepest existing ancestor and
// re-appends the non-existent tail (new files, new dirs). Dangling symlinks
// are followed by hand: EvalSymlinks reports ErrNotExist both for a
// component that does not exist and for a symlink whose target does not
// exist, and treating the latter as a new file would let a write follow the
// link out of the jail.
func resolveDeep(p string) (string, error) {
	const maxHops = 40
	var tail []string
	cur := p
	hops := 0
	for {
		r, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(append([]string{r}, tail...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		fi, lerr := os.Lstat(cur)
		switch {
		case lerr == nil && fi.Mode()&os.ModeSymlink != 0:
			hops++
			if hops > maxHops {
				return "", fmt.Errorf("%s: too many levels of symbolic links", p)
			}
			target, rerr := os.Readlink(cur)
			if rerr != nil {
				return "", rerr
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(cur), target)
			}
			cur = filepath.Clean(target)
			continue
		case lerr != nil && !errors.Is(lerr, fs.ErrNotExist):
			return "", lerr
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p, nil
		}
		tail = append([]string{filepath.Base(cur)}, tail...)
		cur = parent
	}
}

func within(base, p string) bool {
	if runtime.GOOS == "windows" {
		base, p = strings.ToLower(filepath.ToSlash(base)), strings.ToLower(filepath.ToSlash(p))
		return p == base || strings.HasPrefix(p, base+"/")
	}
	return p == base || strings.HasPrefix(p, base+string(filepath.Separator))
}

func (j *Jail) Resolve(path string, write bool) (string, error) {
	res, err := j.resolve(path)
	if err != nil {
		return "", err
	}
	if within(j.root, res) {
		return res, nil
	}
	if !write {
		for _, ro := range j.readOnly {
			if within(ro, res) {
				return res, nil
			}
		}
		return "", fmt.Errorf("%s is outside the workdir %s (symlinks resolved); reading is limited to the workdir and skill directories", path, j.root)
	}
	return "", fmt.Errorf("%s is outside the workdir %s (symlinks resolved); writes are confined to the workdir", path, j.root)
}

// resolve expands ~, joins relative paths to the root and resolves symlinks
// on the deepest existing ancestor (see resolveDeep).
func (j *Jail) resolve(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(j.root, path)
	}
	return resolveDeep(filepath.Clean(path))
}

// Approvable resolves a write the jail refused when the target lies under an
// ask-write root — the global prompt templates, where the user may approve a
// slash command the agent writes for them. The caller must still ask; every
// other path reports false so a refusal cannot be laundered through this.
func (j *Jail) Approvable(path string) (string, bool) {
	res, err := j.resolve(path)
	if err != nil || within(j.root, res) {
		return "", false
	}
	for _, aw := range j.askWrite {
		if within(aw, res) {
			return res, true
		}
	}
	return "", false
}

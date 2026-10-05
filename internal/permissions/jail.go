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
}

func NewJail(root string, readOnly []string) (*Jail, error) {
	r, err := canonical(root)
	if err != nil {
		return nil, fmt.Errorf("jail root: %w", err)
	}
	j := &Jail{root: r}
	for _, ro := range readOnly {
		if c, err := canonical(ro); err == nil {
			j.readOnly = append(j.readOnly, c)
		}
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
	res, err := resolveDeep(filepath.Clean(path))
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

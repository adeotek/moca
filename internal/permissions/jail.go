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
	"slices"
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
	// writable are extra read+write roots beside the workdir (auto mode adds
	// the temp roots); canonical, see WithWritable.
	writable []string
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

// WithWritable returns a copy of the jail whose read and write area also
// covers roots. Each root is made absolute and canonicalized with
// resolveDeep; empty or missing roots are skipped, and so are the
// filesystem root and the home directory or any ancestor of it (a stray
// TMPDIR=/ or TMPDIR=$HOME must not open the whole tree). The original jail
// is not modified.
func (j *Jail) WithWritable(roots ...string) *Jail {
	c := *j
	c.readOnly = slices.Clone(j.readOnly)
	c.askWrite = slices.Clone(j.askWrite)
	c.writable = slices.Clone(j.writable)
	home := ""
	if h, err := os.UserHomeDir(); err == nil {
		if hc, err := resolveDeep(filepath.Clean(h)); err == nil {
			home = hc
		}
	}
	for _, r := range roots {
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		res, err := resolveDeep(abs)
		if err != nil || filepath.Dir(res) == res || (home != "" && within(res, home)) {
			continue
		}
		if !slices.Contains(c.writable, res) {
			c.writable = append(c.writable, res)
		}
	}
	return &c
}

// TempRoots lists the temporary directories auto mode may write to:
// os.TempDir(), plus /tmp on Unix when different, plus $TMPDIR when set.
func TempRoots() []string {
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		p = filepath.Clean(p)
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	add(os.TempDir())
	if runtime.GOOS != "windows" {
		add("/tmp")
	}
	add(os.Getenv("TMPDIR"))
	return out
}

func withinAny(bases []string, p string) bool {
	for _, b := range bases {
		if within(b, p) {
			return true
		}
	}
	return false
}

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
	if within(j.root, res) || withinAny(j.writable, res) {
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
	// T-9: `~user/x` and `~+/x` are host shell expansions moca does not perform;
	// joining them to the workdir would silently address a directory literally
	// named `~user`, while the analyser refuses the same form. Refuse here too.
	if strings.HasPrefix(path, "~") && path != "~" && !strings.HasPrefix(path, "~/") {
		return "", fmt.Errorf("%s: only ~ and ~/… are expanded; moca does not resolve this form", path)
	}
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

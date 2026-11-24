package tools

// Containment for nested inspections. Every access goes through an *os.Root
// opened on the workspace, never through a resolved absolute path: os.Root
// refuses to follow a symlink out of the tree, so a symlinked ancestor is denied
// by the OS layer rather than by string comparison — which is the only kind of
// check that survives a rename between the check and the open.

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// grant is one additional root a transport session may read: the paths the
// direct `read` tool could always reach but the workspace root cannot — the
// global and built-in skills directories, the prompts directory, the session's
// spill directory and the config file. Each is opened as its own *os.Root, so a
// grant inherits the same symlink refusal the workspace has (SPECS §10.6).
type grant struct {
	path string // absolute, as admitted
	root *os.Root
}

// workspace is the root every nested read, search and ls is confined to, plus
// the typed grants a transport session adds.
type workspace struct {
	root   *os.Root
	name   string // the workspace's own path, for messages that stay inside it
	grants []grant
}

// openWorkspace opens dir as a root. It is the single place that turns a
// directory into a containment boundary. grants are absolute paths; a file is
// granted as its parent directory plus the file's own name.
func openWorkspace(dir string, grants []string) (*workspace, error) {
	if dir == "" {
		return nil, &InspectError{Class: "denied", Message: "no workspace root"}
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, &InspectError{Class: "denied", Message: "cannot open the workspace root: " + err.Error()}
	}
	w := &workspace{root: r, name: dir}
	seen := map[string]bool{}
	for _, g := range grants {
		if g == "" {
			continue
		}
		base := g
		if fi, err := os.Stat(g); err == nil && !fi.IsDir() {
			base = filepath.Dir(g)
		}
		if seen[base] {
			continue
		}
		gr, err := os.OpenRoot(base)
		if err != nil {
			continue // a grant that cannot be opened simply does not exist
		}
		seen[base] = true
		w.grants = append(w.grants, grant{path: base, root: gr})
	}
	return w, nil
}

// granted resolves an absolute path against the grants. It returns the open
// root and the path relative to it, or nil when no grant covers p. The match is
// segment-wise, so /a/bc is not covered by a grant on /a/b.
func (w *workspace) granted(p string) (*os.Root, string) {
	clean := filepath.Clean(p)
	for _, g := range w.grants {
		if clean == g.path {
			return nil, "" // the granted path itself is a directory, not a file
		}
		if strings.HasPrefix(clean, g.path+string(filepath.Separator)) {
			return g.root, strings.TrimPrefix(clean, g.path+string(filepath.Separator))
		}
	}
	return nil, ""
}

func (w *workspace) close() error {
	if w == nil {
		return nil
	}
	for _, g := range w.grants {
		g.root.Close()
	}
	if w.root == nil {
		return nil
	}
	return w.root.Close()
}

// cleanRel normalizes a guest-supplied path into a root-relative slash path.
// Absolute paths are refused rather than translated: a script gets the same
// view the workspace has, and "../" is refused here as a courtesy — os.Root
// would refuse it anyway, which is why this is not the security boundary.
func cleanRel(p string) (string, error) {
	if p == "" {
		return "", &InspectError{Class: "invalid_args", Message: "path is required"}
	}
	if strings.ContainsRune(p, 0) {
		return "", &InspectError{Class: "invalid_args", Message: "path contains a NUL byte"}
	}
	if path.IsAbs(p) || (len(p) > 1 && p[1] == ':') {
		return "", &InspectError{Class: "invalid_args", Message: "path must be relative to the workspace: " + p}
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", &InspectError{Class: "outside", Message: "path escapes the workspace: " + p}
	}
	if clean == "." {
		return ".", nil
	}
	return clean, nil
}

// openFile opens rel and requires a regular file. The mode gate runs on the
// opened descriptor, not on a path that may have changed since it was checked.
func (w *workspace) openFile(rel string) (*os.File, os.FileInfo, error) {
	return openRegular(w.root, rel, rel)
}

// openRegular opens name under root and requires a regular file; label is what
// messages call it (a workspace-relative path or a granted absolute one).
func openRegular(root *os.Root, name, label string) (*os.File, os.FileInfo, error) {
	f, err := root.Open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, &InspectError{Class: "not_found", Message: "no such file: " + label}
		}
		return nil, nil, &InspectError{Class: "denied", Message: err.Error()}
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, &InspectError{Class: "denied", Message: err.Error()}
	}
	if fi.IsDir() {
		f.Close()
		return nil, nil, &InspectError{Class: "not_regular", Message: label + " is a directory"}
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, nil, &InspectError{Class: "not_regular", Message: label + " is not a regular file"}
	}
	return f, fi, nil
}

// walk enumerates the tree under rel in lexical order, calling fn for every
// entry. A cancelled context stops it with ctx.Err() so a script that hits its
// deadline does not keep walking.
func (w *workspace) walk(ctx context.Context, rel string, skip func(rel string, d fs.DirEntry) bool, fn func(rel string, d fs.DirEntry) error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	dir, err := w.root.Open(rel)
	if err != nil {
		return &InspectError{Class: "denied", Message: err.Error()}
	}
	defer dir.Close()
	// 128 at a time: a directory with a million entries must not materialize a
	// million DirEntry values before the budget can object.
	var batch []fs.DirEntry
	for {
		batch, err = dir.ReadDir(128)
		if err != nil && err != io.EOF {
			return &InspectError{Class: "denied", Message: err.Error()}
		}
		if len(batch) == 0 {
			return nil
		}
		sort.Slice(batch, func(a, b int) bool { return batch[a].Name() < batch[b].Name() })
		for _, d := range batch {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			child := d.Name()
			if rel != "." {
				child = rel + "/" + child
			}
			if err := fn(child, d); err != nil {
				return err
			}
			if d.IsDir() && !skip(child, d) {
				if err := w.walk(ctx, child, skip, fn); err != nil {
					return err
				}
			}
		}
		if err == io.EOF {
			return nil
		}
	}
}

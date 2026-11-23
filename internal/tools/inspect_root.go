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
	"sort"
	"strings"
)

// workspace is the root every nested read, search and ls is confined to.
type workspace struct {
	root *os.Root
	name string // the workspace's own path, for messages that stay inside it
}

// openWorkspace opens dir as a root. It is the single place that turns a
// directory into a containment boundary.
func openWorkspace(dir string) (*workspace, error) {
	if dir == "" {
		return nil, &InspectError{Class: "denied", Message: "no workspace root"}
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, &InspectError{Class: "denied", Message: "cannot open the workspace root: " + err.Error()}
	}
	return &workspace{root: r, name: dir}, nil
}

func (w *workspace) close() error {
	if w == nil || w.root == nil {
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
	f, err := w.root.Open(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, &InspectError{Class: "not_found", Message: "no such file: " + rel}
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
		return nil, nil, &InspectError{Class: "not_regular", Message: rel + " is a directory"}
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, nil, &InspectError{Class: "not_regular", Message: rel + " is not a regular file"}
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

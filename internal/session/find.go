package session

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var id8re = regexp.MustCompile(`^[0-9a-f]{8}$`)

type fileInfo struct {
	path string
	mod  int64
}

func listNewestFirst(dir string) []fileInfo {
	ents, _ := os.ReadDir(dir)
	var fs []fileInfo
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			fs = append(fs, fileInfo{filepath.Join(dir, e.Name()), fi.ModTime().UnixNano()})
		}
	}
	slices.SortFunc(fs, func(a, b fileInfo) int { return cmp.Compare(b.mod, a.mod) })
	return fs
}

func idOf(path string) string {
	b := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	return b[max(0, len(b)-8):]
}

// Find resolves --resume: "last" is the newest file by mtime, anything else
// must be the 8-hex-char id suffix of a session file name.
func Find(dir, ref string) (string, error) {
	fs := listNewestFirst(dir)
	if ref == "last" {
		if len(fs) == 0 {
			return "", fmt.Errorf("no sessions in %s", dir)
		}
		return fs[0].path, nil
	}
	if !id8re.MatchString(ref) {
		return "", fmt.Errorf("session id must be 8 hex chars or \"last\", got %q", ref)
	}
	for _, f := range fs {
		if idOf(f.path) == ref {
			return f.path, nil
		}
	}
	var recent []string
	for _, f := range fs[:min(5, len(fs))] {
		recent = append(recent, idOf(f.path))
	}
	return "", fmt.Errorf("no session %s (recent: %s)", ref, strings.Join(recent, ", "))
}

// header reads only the first line of a session file (the session entry).
func header(path string) (*Header, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	line, err := r.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil || e.Session == nil {
		return nil, fmt.Errorf("%s: no session header", path)
	}
	return e.Session, nil
}

// FindForWorkdir resolves --continue: the newest session whose header workdir
// is this directory (canonical paths, the same comparison the jail uses).
func FindForWorkdir(dir, workdir string) (string, error) {
	abs, _ := filepath.Abs(workdir)
	canon, err := filepath.EvalSymlinks(abs)
	if err != nil {
		canon = abs
	}
	for _, f := range listNewestFirst(dir) {
		if h, err := header(f.path); err == nil && h.Workdir == canon {
			return f.path, nil
		}
	}
	return "", fmt.Errorf("no session for %s; start one with moca", canon)
}

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type writeTool struct{}

func (writeTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "write", Description: "Create a file or replace its entire content. New files need " +
		"no prior read; an existing file must have been read in this session and be unchanged on disk since. " +
		"Parent directories are created. Prefer edit for changes to existing files.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File path, relative to the workdir or absolute"},` +
			`"content":{"type":"string","description":"Full file content"}},` +
			`"required":["path","content"],"additionalProperties":false}`)}
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(s, "\n"), "\n") + 1
}

// guardedWrite applies the read guard, snapshot and mode preservation shared
// by write and edit.
func guardedWrite(env *Env, abs string, data []byte) error {
	if err := env.Reads.Check(abs); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(abs); err == nil {
		mode = fi.Mode().Perm()
	}
	do := func() error {
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		return os.WriteFile(abs, data, mode)
	}
	var err error
	if env.Snap != nil {
		err = env.Snap.Guard(abs, do)
	} else {
		err = do()
	}
	if err != nil {
		return err
	}
	return env.Reads.Record(abs)
}

func (writeTool) Run(ctx context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	abs, err := env.Paths.Resolve(a.Path, true)
	if err != nil {
		abs, err = askOutsideWrite(ctx, env, a.Path, err)
	}
	if err != nil {
		return errorf("%v", err)
	}
	if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
		return errorf("%s is a directory", a.Path)
	}
	prevLines, hasPrev := 0, false
	if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
		if n, err := countFileLines(abs); err == nil {
			prevLines, hasPrev = n, true
		}
	}
	if err := guardedWrite(env, abs, []byte(a.Content)); err != nil {
		return errorf("%v", err)
	}
	n := countLines(a.Content)
	if !hasPrev {
		return Result{Content: fmt.Sprintf("created %s (%d lines)", a.Path, n), Summary: fmt.Sprintf("%s [+%d]", a.Path, n)}
	}
	return Result{Content: fmt.Sprintf("wrote %s (%d lines, was %d)", a.Path, n, prevLines), Summary: fmt.Sprintf("%s [+%d −%d]", a.Path, n, prevLines)}
}

// countFileLines counts a file's lines by streaming it (a previous version
// loaded the whole file just for the count).
func countFileLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var n, size int
	var last byte
	buf := make([]byte, 64<<10)
	for {
		k, rerr := f.Read(buf)
		if k > 0 {
			size += k
			last = buf[k-1]
			n += bytes.Count(buf[:k], []byte{'\n'})
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return 0, rerr
		}
	}
	if size == 0 {
		return 0, nil
	}
	if last != '\n' {
		n++
	}
	return n, nil
}

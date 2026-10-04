package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const (
	readMaxLines   = 2000
	readMaxChars   = 50_000
	readMaxLineLen = 2000
)

type readTool struct{}

func (readTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "read", Description: "Read a text file. Returns lines as `N|content` plus a " +
		"`[lines A-B of TOTAL]` footer. At most 2000 lines or 50K chars per call; use offset/limit to page " +
		"through large files instead of re-reading them whole. Lines longer than 2000 chars are truncated. " +
		"Binary files are refused. A file must be read before write/edit may change it.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File path, relative to the workdir or absolute"},` +
			`"offset":{"type":"integer","minimum":1,"description":"1-based first line (default 1)"},` +
			`"limit":{"type":"integer","minimum":1,"description":"Max lines to return (default/max 2000)"}},` +
			`"required":["path"],"additionalProperties":false}`)}
}

func (readTool) Run(_ context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path   string `json:"path"`
		Offset *int   `json:"offset"`
		Limit  *int   `json:"limit"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	offset, limit := 1, readMaxLines
	if a.Offset != nil {
		offset = *a.Offset
	}
	if a.Limit != nil {
		limit = min(*a.Limit, readMaxLines)
	}
	if offset < 1 || limit < 1 {
		return errorf("offset and limit must be >= 1")
	}
	abs, err := env.Paths.Resolve(a.Path, false)
	if err != nil {
		return errorf("%v", err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return errorf("%v", err)
	}
	if fi.IsDir() {
		return errorf("%s is a directory; use ls", a.Path)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return errorf("%v", err)
	}
	if bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
		return errorf("%s is a binary file (%d bytes, %s); not shown", a.Path, len(data), http.DetectContentType(data))
	}
	env.Reads.Record(abs)

	text := strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := strings.Split(text, "\n")
	if text == "" {
		lines = nil
	}
	total := len(lines)
	if total == 0 {
		return Result{Content: "[empty file]", Summary: a.Path}
	}
	if offset > total {
		return errorf("offset %d is past the end (%d lines)", offset, total)
	}
	var sb strings.Builder
	last := offset - 1
	for i := offset - 1; i < total && i < offset-1+limit; i++ {
		line := lines[i]
		if len(line) > readMaxLineLen {
			line = line[:readMaxLineLen] + "[… line truncated]"
		}
		entry := fmt.Sprintf("%d|%s\n", i+1, line)
		if sb.Len()+len(entry) > readMaxChars && i > offset-1 {
			break
		}
		sb.WriteString(entry)
		last = i + 1
	}
	footer := fmt.Sprintf("[lines %d-%d of %d]", offset, last, total)
	if last < total {
		footer = fmt.Sprintf("[lines %d-%d of %d — use offset=%d to continue]", offset, last, total, last+1)
	}
	sb.WriteString(footer)
	return Result{Content: sb.String(), Summary: fmt.Sprintf("%s %d-%d/%d", a.Path, offset, last, total)}
}

package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const (
	readMaxLines     = 2000
	readDefaultLines = 400 // the window when no limit is given
	readMaxChars     = 50_000
	readMaxLineLen   = 2000
	readMaxFileSize  = 256 << 20
)

type readTool struct{}

func (readTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "read", Description: "Read a text file. Returns lines as `N|content` plus a " +
		"`[lines A-B of TOTAL]` footer. Without limit it returns at most 400 lines; limit allows up to 2000 lines " +
		"(50K chars). Page with offset/limit, or search for the lines you need, instead of re-reading whole files. " +
		"Lines longer than 2000 chars are truncated. " +
		"Binary files are refused. A file must be read before write/edit may change it.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File path, relative to the workdir or absolute"},` +
			`"offset":{"type":"integer","minimum":1,"description":"1-based first line (default 1)"},` +
			`"limit":{"type":"integer","minimum":1,"description":"Max lines to return (default 400, max 2000)"}},` +
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
	offset, limit := 1, readDefaultLines
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
	if !fi.Mode().IsRegular() {
		return errorf("%s is not a regular file (%s); not read", a.Path, fi.Mode().Type())
	}
	if fi.Size() > readMaxFileSize {
		return errorf("%s is %d bytes; too large to read in one call (limit %d MB) — use shell (head/tail) or search",
			a.Path, fi.Size(), readMaxFileSize>>20)
	}
	f, err := os.Open(abs)
	if err != nil {
		return errorf("%v", err)
	}
	defer f.Close()
	head := make([]byte, 8192)
	n, rerr := io.ReadFull(f, head)
	if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {
		return errorf("%v", rerr)
	}
	if bytes.IndexByte(head[:n], 0) >= 0 {
		return errorf("%s is a binary file (%d bytes, %s); not shown", a.Path, fi.Size(), http.DetectContentType(head[:n]))
	}
	env.Reads.Record(abs)
	if strings.HasSuffix(a.Path, "_test.go") {
		env.TestSeen = true
		env.FailingTest = ""
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return errorf("%v", err)
	}
	br := bufio.NewReaderSize(f, 64<<10)
	var sb strings.Builder
	lineNo, last, total := 0, offset-1, 0
	for {
		line, more, rerr := readLine(br, readMaxLineLen)
		if rerr != nil {
			if rerr != io.EOF {
				return errorf("%v", rerr)
			}
			break
		}
		lineNo++
		total = lineNo
		if lineNo < offset || lineNo >= offset+limit {
			continue
		}
		if more {
			line += "[… line truncated]"
		}
		entry := fmt.Sprintf("%d|%s\n", lineNo, line)
		if sb.Len()+len(entry) > readMaxChars && lineNo > offset {
			continue // keep scanning to report the real total
		}
		sb.WriteString(entry)
		last = lineNo
	}
	if total == 0 {
		return Result{Content: "[empty file]", Summary: a.Path}
	}
	if offset > total {
		return errorf("offset %d is past the end (%d lines)", offset, total)
	}
	footer := fmt.Sprintf("[lines %d-%d of %d]", offset, last, total)
	if last < total {
		footer = fmt.Sprintf("[lines %d-%d of %d — use offset=%d to continue, or search for what you need]", offset, last, total, last+1)
	}
	sb.WriteString(footer)
	return Result{Content: sb.String(), Summary: fmt.Sprintf("%s %d-%d/%d", a.Path, offset, last, total)}
}

// readLine reads one line (without its newline; a trailing \r is trimmed),
// capped at max bytes with more=true when the line continues past the cap.
// io.EOF is returned only when nothing was read.
func readLine(br *bufio.Reader, max int) (line string, more bool, err error) {
	var sb strings.Builder
	for {
		b, err := br.ReadByte()
		if err != nil {
			if sb.Len() > 0 {
				return sb.String(), more, nil
			}
			return "", false, err
		}
		if b == '\n' {
			return strings.TrimSuffix(sb.String(), "\r"), more, nil
		}
		if sb.Len() < max {
			sb.WriteByte(b)
		} else {
			more = true
		}
	}
}

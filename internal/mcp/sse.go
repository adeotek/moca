package mcp

import (
	"bufio"
	"io"
	"strings"
)

// readEvents is a minimal SSE reader for MCP streamable HTTP. It reads the
// `data:` payloads of complete events (CRLF tolerant) and hands each to fn; a
// non-nil error from fn stops the read. mcp cannot import provider (§2), and
// MCP needs no stall timeout (ctx bounds it).
func readEvents(r io.Reader, fn func(data string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 64<<20)
	var data []string
	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		d := strings.Join(data, "\n")
		data = nil
		return fn(d)
	}
	for sc.Scan() {
		line := strings.TrimSuffix(sc.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(v, " "))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	return sc.Err()
}

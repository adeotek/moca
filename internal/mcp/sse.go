package mcp

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// readEvents is a minimal SSE reader for MCP streamable HTTP. It reads the
// `data:` payloads of complete events (CRLF tolerant) and hands each to fn; a
// non-nil error from fn stops the read. mcp cannot import provider (§2), and
// MCP needs no stall timeout (ctx bounds it).
func readEvents(r io.Reader, fn func(data string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), maxMCPMessage)
	var data []string
	size := 0
	flush := func() error {
		if len(data) == 0 {
			return nil
		}
		d := strings.Join(data, "\n")
		data, size = nil, 0
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
			// F-4: an event that never ends (no blank line) accumulated
			// without limit — the scanner's per-line cap did not bound it.
			size += len(v)
			if size > maxMCPMessage {
				return fmt.Errorf("event exceeds %d MB", maxMCPMessage>>20)
			}
			data = append(data, strings.TrimPrefix(v, " "))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	return sc.Err()
}

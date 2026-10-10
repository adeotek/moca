package applog

import (
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"
)

// recordMax caps one record: a recovered panic's stack must fit.
var recordMax = 16 << 10

// cappedWriter is the file behind the handler: records are cut to
// recordMax, the file stops at max after one truncation line, and a write
// error turns it off. It never returns an error — logging must not fail a
// run.
type cappedWriter struct {
	mu  sync.Mutex
	w   io.WriteCloser
	n   int64
	max int64
	off bool
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.off {
		return len(p), nil
	}
	rec := cutRecord(p)
	if c.n+int64(len(rec)) > c.max {
		c.off = true
		fmt.Fprintf(c.w, "time=%s level=WARN msg=\"log truncated: size cap reached\"\n", time.Now().Format(time.RFC3339Nano))
		return len(p), nil
	}
	k, err := c.w.Write(rec)
	c.n += int64(k)
	if err != nil {
		c.off = true
	}
	return len(p), nil
}

func (c *cappedWriter) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.off = true
	return c.w.Close()
}

// cutRecord trims one record to recordMax bytes on a rune boundary, ending
// with a marker and the newline.
func cutRecord(p []byte) []byte {
	if len(p) <= recordMax {
		return p
	}
	const mark = " …[cut]\n"
	n := recordMax - len(mark)
	for n > 0 && !utf8.RuneStart(p[n]) {
		n--
	}
	out := make([]byte, 0, n+len(mark))
	return append(append(out, p[:n]...), mark...)
}

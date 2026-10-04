package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const stallTimeout = 90 * time.Second

var (
	ErrStall           = errors.New("stream stalled: no data for 90s")
	ErrContextOverflow = errors.New("context window exceeded")
)

type HTTPError struct {
	Status     int
	RetryAfter time.Duration
	Body       string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body) }

func newHTTPError(resp *http.Response) *HTTPError {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	e := &HTTPError{Status: resp.StatusCode, Body: string(b)}
	if s := resp.Header.Get("Retry-After"); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if f > 0 { // also rejects NaN
				e.RetryAfter = time.Duration(min(f, 3600) * float64(time.Second))
			}
		} else if t, err := http.ParseTime(s); err == nil {
			e.RetryAfter = max(time.Until(t), 0)
		}
	}
	return e
}

func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrContextOverflow) {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status == 408 || he.Status == 429 || he.Status >= 500
	}
	var ne net.Error
	return errors.Is(err, ErrStall) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.EPIPE) ||
		(errors.As(err, &ne) && ne.Timeout()) || http2Transient(err)
}

// http2Transient: net/http's bundled HTTP/2 transport keeps GOAWAY and
// stream-reset error types unexported, so they can only be matched by text.
func http2Transient(err error) bool {
	s := err.Error()
	return strings.Contains(s, "http2: server sent GOAWAY") || strings.Contains(s, "stream error: stream ID")
}

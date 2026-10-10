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
	return &HTTPError{Status: resp.StatusCode, Body: string(b), RetryAfter: retryAfter(resp.Header)}
}

// retryAfter parses a Retry-After header (delta-seconds or an HTTP date);
// zero when absent or unparseable.
func retryAfter(h http.Header) time.Duration {
	s := h.Get("Retry-After")
	if s == "" {
		return 0
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f > 0 { // also rejects NaN
			return time.Duration(min(f, 3600) * float64(time.Second))
		}
		return 0
	}
	if t, err := http.ParseTime(s); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}

func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrContextOverflow) {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		if he.Status == 408 || he.Status == 429 || he.Status >= 500 {
			return true
		}
		// The opencode-go gateway frames some upstream failures as HTTP 400
		// while labelling them server_error (the [1210] thinking-config
		// flake observed in the live ship-gate runs): retry what the server
		// itself calls a server error, bounded by the five-attempt budget.
		return he.Status == http.StatusBadRequest && strings.Contains(he.Body, `"type":"server_error"`)
	}
	var ne net.Error
	return errors.Is(err, ErrStall) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.EPIPE) ||
		(errors.As(err, &ne) && ne.Timeout()) || http2Transient(err)
}

// causeClass names a failure for the log: enough to group failures without
// their text.
func causeClass(err error) string {
	var he *HTTPError
	var ne net.Error
	switch {
	case errors.Is(err, ErrContextOverflow):
		return "overflow"
	case errors.Is(err, ErrStall):
		return "stall"
	case errors.As(err, &he):
		return fmt.Sprintf("http_%d", he.Status)
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.ECONNABORTED), errors.Is(err, syscall.EPIPE):
		return "reset"
	}
	return "other"
}

// http2Transient: net/http's bundled HTTP/2 transport keeps GOAWAY and
// stream-reset error types unexported, so they can only be matched by text.
func http2Transient(err error) bool {
	s := err.Error()
	return strings.Contains(s, "http2: server sent GOAWAY") || strings.Contains(s, "stream error: stream ID")
}

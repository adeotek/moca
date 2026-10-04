package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
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
		if n, err := strconv.Atoi(s); err == nil {
			e.RetryAfter = time.Duration(n) * time.Second
		} else if t, err := http.ParseTime(s); err == nil {
			e.RetryAfter = time.Until(t)
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
		return he.Status == 429 || he.Status >= 500
	}
	var ne net.Error
	return errors.Is(err, ErrStall) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || (errors.As(err, &ne) && ne.Timeout())
}

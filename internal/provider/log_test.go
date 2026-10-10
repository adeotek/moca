package provider

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

// logSink is a goroutine-safe buffer for captured log output.
type logSink struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *logSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// captureLog swaps the slog default for a text handler into a buffer at
// level, restoring the previous default when the test ends. Never combine
// with t.Parallel.
func captureLog(t *testing.T, level slog.Level) *logSink {
	t.Helper()
	s := &logSink{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(s, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return s
}

// seqAdapter fails with errs[i] on call i, then streams one text event.
type seqAdapter struct {
	errs []error
	n    int
}

func (s *seqAdapter) Stream(_ context.Context, _ llm.Request, emit func(llm.Event)) (llm.Response, error) {
	i := s.n
	s.n++
	if i < len(s.errs) && s.errs[i] != nil {
		return llm.Response{}, s.errs[i]
	}
	emit(llm.Event{Type: llm.EventText, Text: "x"})
	return llm.Response{Stop: llm.StopEnd, Usage: llm.Usage{Input: 7, Output: 3}}, nil
}

func logPolicy() RetryPolicy {
	return RetryPolicy{Delays: []time.Duration{time.Millisecond, time.Millisecond},
		Sleep:  func(context.Context, time.Duration) error { return nil },
		Jitter: func(d time.Duration) time.Duration { return d }}
}

func TestProviderLogsRetryAndResponse(t *testing.T) {
	log := captureLog(t, slog.LevelDebug)
	a := WithRetry(&seqAdapter{errs: []error{&HTTPError{Status: 429, Body: "slow down"}}}, logPolicy())
	req := llm.Request{Model: "m1", Messages: []llm.Message{{Role: llm.RoleUser}}}
	if _, err := a.Stream(context.Background(), req, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	out := log.String()
	for _, want := range []string{"msg=request model=m1 messages=1 tools=0", "msg=retry attempt=1 of=2", "cause=http_429",
		"msg=response model=m1", "stop=end_turn", "usage.in=7 usage.out=3", "ttfb="} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestProviderLogsFinalFailureAtInfo(t *testing.T) {
	log := captureLog(t, slog.LevelInfo)
	a := WithRetry(&seqAdapter{errs: []error{ErrContextOverflow}}, logPolicy())
	if _, err := a.Stream(context.Background(), llm.Request{Model: "m1"}, func(llm.Event) {}); err == nil {
		t.Fatal("overflow is not retried")
	}
	out := log.String()
	if !strings.Contains(out, `msg="request failed" model=m1 cause=overflow`) || strings.Contains(out, "msg=request ") {
		t.Fatalf("info shows the failure, not the debug request line:\n%s", out)
	}
}

func TestCauseClass(t *testing.T) {
	cases := map[error]string{ErrStall: "stall", ErrContextOverflow: "overflow", &HTTPError{Status: 503}: "http_503",
		context.DeadlineExceeded: "timeout", io.ErrUnexpectedEOF: "reset", errors.New("x"): "other"}
	for err, want := range cases {
		if got := causeClass(err); got != want {
			t.Errorf("causeClass(%v) = %q, want %q", err, got, want)
		}
	}
}

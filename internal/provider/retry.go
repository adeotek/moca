package provider

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

type RetryNotice struct {
	Attempt, Max int
	Wait         time.Duration
	Err          error
}

type RetryPolicy struct {
	Delays []time.Duration
	Sleep  func(ctx context.Context, d time.Duration) error
	Jitter func(d time.Duration) time.Duration
	Notify func(RetryNotice)
}

func DefaultRetryPolicy(notify func(RetryNotice)) RetryPolicy {
	return RetryPolicy{
		Delays: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second},
		Sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
		Jitter: func(d time.Duration) time.Duration { return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64())) },
		Notify: notify,
	}
}

type retrying struct {
	a Adapter
	p RetryPolicy
}

// maxRetryAfter caps how long a retry-after header can park the loop.
const maxRetryAfter = 60 * time.Second

func WithRetry(a Adapter, p RetryPolicy) Adapter { return &retrying{a, p} }

// Stream logs one request through the retry loop (SPECS §13.5): debug lines
// for the request and its response, an info line for a final failure. Every
// adapter is wrapped here, so this is the one place the wire is logged.
func (r *retrying) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	t0 := time.Now()
	var first time.Time
	slog.Debug("request", "model", req.Model, "messages", len(req.Messages), "tools", len(req.Tools))
	resp, err := r.stream(ctx, req, func(e llm.Event) {
		if first.IsZero() {
			first = time.Now()
		}
		emit(e)
	})
	switch {
	case err == nil:
		attrs := []any{"model", req.Model, "duration", time.Since(t0).Round(time.Millisecond), "stop", string(resp.Stop)}
		if !first.IsZero() {
			attrs = append(attrs, "ttfb", first.Sub(t0).Round(time.Millisecond))
		}
		attrs = append(attrs, slog.Group("usage", "in", resp.Usage.Input, "out", resp.Usage.Output,
			"cache_read", resp.Usage.CacheRead, "cache_write", resp.Usage.CacheWrite))
		slog.Debug("response", attrs...)
	case ctx.Err() == nil:
		slog.Info("request failed", "model", req.Model, "cause", causeClass(err), "error", err)
	}
	return resp, err
}

func (r *retrying) stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	attempt, midRetried := 0, false
	for {
		streamed := false
		resp, err := r.a.Stream(ctx, req, func(e llm.Event) { streamed = true; emit(e) })
		if err == nil {
			return resp, nil
		}
		if ctx.Err() != nil {
			return resp, ctx.Err()
		}
		if !retryable(err) {
			return resp, err
		}
		if streamed {
			// §3: one retry of the whole turn after a mid-stream failure, announced
			// and spaced like any other retry but outside the five-attempt budget.
			if midRetried {
				return resp, err
			}
			midRetried = true
			emit(llm.Event{Type: llm.EventReset})
			var base time.Duration
			if len(r.p.Delays) > 0 {
				base = r.p.Delays[0]
			}
			if perr := r.pause(ctx, err, base, 1, 1); perr != nil {
				return resp, perr
			}
			continue
		}
		if attempt >= len(r.p.Delays) {
			return resp, err
		}
		attempt++
		if perr := r.pause(ctx, err, r.p.Delays[attempt-1], attempt, len(r.p.Delays)); perr != nil {
			return resp, perr
		}
	}
}

// pause announces a retry and sleeps: the jittered base delay, stretched to
// honour retry-after, never beyond maxRetryAfter.
func (r *retrying) pause(ctx context.Context, cause error, base time.Duration, n, of int) error {
	wait := r.p.Jitter(base)
	var he *HTTPError
	if errors.As(cause, &he) && he.RetryAfter > wait {
		wait = he.RetryAfter
	}
	wait = min(wait, maxRetryAfter)
	slog.Info("retry", "attempt", n, "of", of, "wait", wait.Round(time.Millisecond), "cause", causeClass(cause))
	if r.p.Notify != nil {
		r.p.Notify(RetryNotice{Attempt: n, Max: of, Wait: wait, Err: cause})
	}
	return r.p.Sleep(ctx, wait)
}

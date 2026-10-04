package provider

import (
	"context"
	"errors"
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

func (r *retrying) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
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
	if r.p.Notify != nil {
		r.p.Notify(RetryNotice{Attempt: n, Max: of, Wait: wait, Err: cause})
	}
	return r.p.Sleep(ctx, wait)
}

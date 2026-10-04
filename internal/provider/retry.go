package provider

import (
	"context"
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
			if midRetried {
				return resp, err
			}
			midRetried = true
			emit(llm.Event{Type: llm.EventReset})
			continue
		}
		if attempt >= len(r.p.Delays) {
			return resp, err
		}
		wait := r.p.Jitter(r.p.Delays[attempt])
		if he, ok := err.(*HTTPError); ok && he.RetryAfter > wait {
			wait = he.RetryAfter
		}
		if wait > maxRetryAfter {
			wait = maxRetryAfter
		}
		attempt++
		if r.p.Notify != nil {
			r.p.Notify(RetryNotice{Attempt: attempt, Max: len(r.p.Delays), Wait: wait, Err: err})
		}
		if err := r.p.Sleep(ctx, wait); err != nil {
			return resp, err
		}
	}
}

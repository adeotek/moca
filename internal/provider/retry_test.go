// internal/provider/retry_test.go
package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

type scripted struct {
	steps []func(emit func(llm.Event)) error
	n     int
}

func (s *scripted) Stream(_ context.Context, _ llm.Request, emit func(llm.Event)) (llm.Response, error) {
	f := s.steps[s.n]
	s.n++
	return llm.Response{Stop: llm.StopEnd}, f(emit)
}

func testPolicy(slept *[]time.Duration, notes *[]RetryNotice) RetryPolicy {
	return RetryPolicy{
		Delays: []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second},
		Sleep:  func(_ context.Context, d time.Duration) error { *slept = append(*slept, d); return nil },
		Jitter: func(d time.Duration) time.Duration { return d },
		Notify: func(n RetryNotice) { *notes = append(*notes, n) },
	}
}

func fail(err error) func(func(llm.Event)) error { return func(func(llm.Event)) error { return err } }
func ok(func(llm.Event)) error                   { return nil }

func TestRetryBackoffThenSuccess(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	s := &scripted{steps: []func(func(llm.Event)) error{fail(&HTTPError{Status: 429}), fail(&HTTPError{Status: 503}), ok}}
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if err != nil || s.n != 3 {
		t.Fatalf("err %v calls %d", err, s.n)
	}
	if len(slept) != 2 || slept[0] != time.Second || slept[1] != 2*time.Second {
		t.Fatalf("slept %v", slept)
	}
	if notes[1].Attempt != 2 || notes[1].Max != 5 {
		t.Fatalf("notice %+v", notes[1])
	}
}

func TestRetryHonoursRetryAfter(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	s := &scripted{steps: []func(func(llm.Event)) error{fail(&HTTPError{Status: 429, RetryAfter: 7 * time.Second}), ok}}
	WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if slept[0] != 7*time.Second {
		t.Fatalf("retry-after ignored: %v", slept)
	}
}

func TestRetryGivesUpAfterFive(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	e := &HTTPError{Status: 500}
	s := &scripted{steps: []func(func(llm.Event)) error{fail(e), fail(e), fail(e), fail(e), fail(e), fail(e)}}
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if !errors.Is(err, e) || s.n != 6 {
		t.Fatalf("err %v calls %d (1 + 5 retries)", err, s.n)
	}
}

func TestNoRetryOn400(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	s := &scripted{steps: []func(func(llm.Event)) error{fail(&HTTPError{Status: 400})}}
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if err == nil || s.n != 1 {
		t.Fatal("400 must not retry")
	}
}

func TestMidStreamFailureResetsAndRetriesOnce(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	partial := func(emit func(llm.Event)) error {
		emit(llm.Event{Type: llm.EventText, Text: "par"})
		return ErrStall
	}
	s := &scripted{steps: []func(func(llm.Event)) error{partial, partial, ok}}
	var evs []llm.EventType
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(e llm.Event) { evs = append(evs, e.Type) })
	if !errors.Is(err, ErrStall) || s.n != 2 {
		t.Fatalf("second mid-stream failure must surface: err %v calls %d", err, s.n)
	}
	if len(evs) != 3 || evs[1] != llm.EventReset {
		t.Fatalf("events %v: want text, reset, text", evs)
	}
}

func TestRetryStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := DefaultRetryPolicy(nil)
	s := &scripted{steps: []func(func(llm.Event)) error{
		func(func(llm.Event)) error { cancel(); return &HTTPError{Status: 500} }, ok}}
	_, err := WithRetry(s, p).Stream(ctx, llm.Request{}, func(llm.Event) {})
	if !errors.Is(err, context.Canceled) || s.n != 1 {
		t.Fatalf("cancel during backoff: err %v calls %d", err, s.n)
	}
}

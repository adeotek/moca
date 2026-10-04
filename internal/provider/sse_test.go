// internal/provider/sse_test.go
package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type evt struct{ ev, data string }

func collect(t *testing.T, raw string) []evt {
	t.Helper()
	var got []evt
	err := readSSE(context.Background(), strings.NewReader(raw), time.Second, func(e, d string) error {
		got = append(got, evt{e, d})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSSEFramingVariants(t *testing.T) {
	want := []evt{{"a", "1"}, {"", "x\ny"}, {"b", "{}"}}
	lf := "event: a\ndata: 1\n\n: ping\n\ndata: x\ndata: y\n\nevent:b\ndata:{}\n\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	for name, raw := range map[string]string{"lf": lf, "crlf": crlf} {
		got := collect(t, raw)
		if len(got) != len(want) {
			t.Fatalf("%s: got %v", name, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s[%d]: got %v want %v", name, i, got[i], want[i])
			}
		}
	}
}

func TestSSEFinalEventWithoutBlankLine(t *testing.T) {
	if got := collect(t, "data: [DONE]"); len(got) != 1 || got[0].data != "[DONE]" {
		t.Fatalf("%v", got)
	}
}

func TestSSEStall(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	go pw.Write([]byte("data: 1\n\n"))
	err := readSSE(context.Background(), pr, 50*time.Millisecond, func(string, string) error { return nil })
	if !errors.Is(err, ErrStall) {
		t.Fatalf("want ErrStall, got %v", err)
	}
}

func TestSSECallbackErrorStops(t *testing.T) {
	boom := errors.New("boom")
	err := readSSE(context.Background(), strings.NewReader("data: 1\n\ndata: 2\n\n"), time.Second,
		func(string, string) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestRetryable(t *testing.T) {
	yes := []error{&HTTPError{Status: 429}, &HTTPError{Status: 500}, &HTTPError{Status: 529}, ErrStall, io.ErrUnexpectedEOF}
	no := []error{&HTTPError{Status: 400}, &HTTPError{Status: 401}, ErrContextOverflow, context.Canceled}
	for _, e := range yes {
		if !retryable(e) {
			t.Errorf("%v should retry", e)
		}
	}
	for _, e := range no {
		if retryable(e) {
			t.Errorf("%v should not retry", e)
		}
	}
}

func TestRetryAfterParsed(t *testing.T) {
	r := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"7"}}, Body: io.NopCloser(strings.NewReader("slow down"))}
	e := newHTTPError(r)
	if e.RetryAfter != 7*time.Second || e.Body != "slow down" {
		t.Fatalf("%+v", e)
	}
}

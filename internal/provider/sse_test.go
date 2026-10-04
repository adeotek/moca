// internal/provider/sse_test.go
package provider

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
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
	yes := []error{&HTTPError{Status: 408}, &HTTPError{Status: 429}, &HTTPError{Status: 500}, &HTTPError{Status: 529}, ErrStall, io.EOF, io.ErrUnexpectedEOF}
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

func TestRetryableTransportErrors(t *testing.T) {
	refused := &url.Error{Op: "Post", URL: "http://x", Err: &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}}
	yes := []error{
		refused,
		os.NewSyscallError("write", syscall.EPIPE),
		errors.New(`Post "https://x": http2: server sent GOAWAY and closed the connection; LastStreamID=1, ErrCode=NO_ERROR`),
		errors.New("stream error: stream ID 3; INTERNAL_ERROR; received from peer"),
	}
	for _, e := range yes {
		if !retryable(e) {
			t.Errorf("%v should retry", e)
		}
	}
	if retryable(errors.New("tls: failed to verify certificate")) {
		t.Error("unrelated errors must not retry")
	}
}

func TestRetryAfterFormats(t *testing.T) {
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	for in, want := range map[string]time.Duration{"1.5": 1500 * time.Millisecond, "-5": 0, past: 0} {
		r := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {in}}, Body: io.NopCloser(strings.NewReader(""))}
		if got := newHTTPError(r).RetryAfter; got != want {
			t.Errorf("Retry-After %q → %v, want %v", in, got, want)
		}
	}
}

func TestNonSSEOKResponseIsNotRetried(t *testing.T) {
	// A 200 JSON body (gateway error page, wrong endpoint) is not a truncated
	// stream: surfacing it at once beats burning the whole backoff schedule.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"error":"wrong endpoint"}`)
	}))
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	if err == nil || retryable(err) || !strings.Contains(err.Error(), "wrong endpoint") {
		t.Fatalf("want a non-retryable error carrying the body, got %v", err)
	}
}

func TestRetryAfterParsed(t *testing.T) {
	r := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"7"}}, Body: io.NopCloser(strings.NewReader("slow down"))}
	e := newHTTPError(r)
	if e.RetryAfter != 7*time.Second || e.Body != "slow down" {
		t.Fatalf("%+v", e)
	}
}

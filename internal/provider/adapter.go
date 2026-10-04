package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

// Adapter streams one model turn. emit receives deltas and completed tool
// calls in order; the returned Response holds the assembled message.
type Adapter interface {
	Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error)
}

type Credential struct {
	Token string
	OAuth bool // bearer token from `moca login` (phase 7)
}

type CredentialFunc func(ctx context.Context) (Credential, error)

// post sends a JSON body and returns the response on 2xx; non-2xx becomes *HTTPError.
func post(ctx context.Context, hc *http.Client, url string, hdr http.Header, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r.Header = hdr
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/event-stream")
	resp, err := hc.Do(r)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, newHTTPError(resp)
	}
	// A 200 that is not an event stream (gateway error page, wrong endpoint) is
	// not a truncated turn; surface it instead of retrying it five times.
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(strings.ToLower(ct), "text/event-stream") {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return nil, fmt.Errorf("unexpected %s response, want text/event-stream: %s", ct, b)
	}
	return resp, nil
}

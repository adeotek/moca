package provider

import (
	"net/http"
	"sync"
	"time"
)

// headerTimeout bounds the wait for provider response headers: the SSE stall
// timeout only covers the response body, so a server that accepts a request
// and then goes silent would hang an unattended run forever (seen once in
// the live ship-gate runs).
const headerTimeout = 120 * time.Second

// defaultHTTPClient is the client NewRegistry uses when its caller passes
// nil. Built once: `moca acp` makes a registry per session/new and
// authenticate, and one shared transport keeps one connection pool.
var defaultHTTPClient = sync.OnceValue(func() *http.Client { return newHTTPClient(headerTimeout) })

// newHTTPClient clones http.DefaultTransport (proxy env, dial and TLS
// timeouts) and bounds the wait for response headers.
func newHTTPClient(header time.Duration) *http.Client {
	tr, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Transport: http.DefaultTransport}
	}
	ct := tr.Clone()
	ct.ResponseHeaderTimeout = header
	return &http.Client{Transport: ct}
}

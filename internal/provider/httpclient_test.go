package provider

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultHTTPClientBoundsHeaderWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	_, err := newHTTPClient(50 * time.Millisecond).Get(srv.URL)
	if err == nil {
		t.Fatal("a server that never sends headers must time out")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("want a net timeout error, got %v", err)
	}
	// The shared default is that client at the production bound, and it is
	// one instance: every registry built without a client shares its pool.
	if tr, ok := defaultHTTPClient().Transport.(*http.Transport); !ok || tr.ResponseHeaderTimeout != headerTimeout {
		t.Fatalf("default client: %#v", defaultHTTPClient())
	}
	if defaultHTTPClient() != defaultHTTPClient() {
		t.Fatal("the default client is built once")
	}
}

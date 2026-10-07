package agent

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultHTTPClientBoundsHeaderWait(t *testing.T) {
	old := headerTimeout
	headerTimeout = 50 * time.Millisecond
	defer func() { headerTimeout = old }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	_, err := defaultHTTPClient().Get(srv.URL)
	if err == nil {
		t.Fatal("a server that never sends headers must time out")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("want a net timeout error, got %v", err)
	}
}

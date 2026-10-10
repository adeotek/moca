package fetcher

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("hi")) }))
	defer srv.Close()
	b, err := New().Get(srv.URL)
	if err != nil || string(b) != "hi" {
		t.Fatalf("%q %v", b, err)
	}
}

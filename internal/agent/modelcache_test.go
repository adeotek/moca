package agent

// Tests for /clear-models-cache at the agent level (SPECS §6): clearing the
// cache must never cost a running session the models it resolves by name.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// The summarizer and the auto-mode judge re-resolve the configured model on
// every call. A re-read that no longer lists it (the provider retired the id)
// used to make both fail with "unknown model" for the rest of the session.
func TestCheapModelSurvivesARelist(t *testing.T) {
	var list atomic.Value
	list.Store(`{"data":[{"id":"m"}]}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, list.Load().(string))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, textTurn("ok"))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	// gw declares no models: every one of them is discovered, like OpenRouter's.
	cfg, err := config.Parse([]byte(`{"model":"gw/m","providers":{"gw":{"baseUrl":"` + srv.URL + `",
		"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	a, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), HTTP: srv.Client(), Slug: "t"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })

	list.Store(`{"data":[{"id":"m2"}]}`)
	rep, err := a.ClearModelsCache(context.Background(), "gw")
	if err != nil || rep.Rescan.Err != nil || strings.Join(rep.Rescan.Models, ",") != "m2" {
		t.Fatalf("setup: the re-read must drop gw/m: %+v %v", rep, err)
	}
	if _, _, _, err := a.summarizer(); err != nil {
		t.Fatalf("compaction lost its model: %v", err)
	}
	if _, _, err := a.judgeModel(); err != nil {
		t.Fatalf("the auto-mode judge lost its model: %v", err)
	}
}

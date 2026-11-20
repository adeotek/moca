//go:build contextgate

// Package contextgate drives the built moca binary against a scripted
// provider and checks the context governor end to end. Hermetic: no key, no
// network beyond the local test server.
package contextgate

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var mocaBin = flag.String("moca", "", "path to the built moca binary")

type recorder struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (r *recorder) add(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bodies = append(r.bodies, append([]byte(nil), b...))
}

func (r *recorder) all() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]byte(nil), r.bodies...)
}

func sseText(w http.ResponseWriter, text string) {
	b, _ := json.Marshal(text)
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
}

func sseTool(w http.ResponseWriter, id, name, args string) {
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":%q,\"function\":{\"name\":%q,\"arguments\":%q}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", id, name, args)
}

// provider scripts three work turns: two reads and a final answer. Requests
// without tools are summarizer calls.
func provider(t *testing.T, rec *recorder, turns *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[{"id":"m","limit":{"context":65536,"output":4096}}]}`)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			http.Error(w, "body", http.StatusBadRequest)
			return
		}
		rec.add(body)
		var req struct {
			Tools []json.RawMessage `json:"tools"`
		}
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			w.Header().Set("Content-Type", "text/event-stream")
			sseText(w, "## Goal\ncompacted")
			return
		}
		*turns++
		w.Header().Set("Content-Type", "text/event-stream")
		switch *turns {
		case 1:
			sseTool(w, "c1", "read", `{"path":"big1.txt","limit":2000}`)
		case 2:
			sseTool(w, "c2", "read", `{"path":"big2.txt","limit":2000}`)
		default:
			sseText(w, "done")
		}
	}))
}

func writeBig(t *testing.T, dir, name string) {
	t.Helper()
	line := strings.Repeat("x", 99) + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Repeat(line, 2000)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestContextGate(t *testing.T) {
	if *mocaBin == "" {
		t.Fatal("usage: go test -tags contextgate ./test/contextgate -args -moca <binary>")
	}
	rec := &recorder{}
	turns := 0
	srv := provider(t, rec, &turns)
	defer srv.Close()

	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	work := t.TempDir()
	writeBig(t, work, "big1.txt")
	writeBig(t, work, "big2.txt")
	if err := os.MkdirAll(filepath.Join(cfgDir, "moca"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`{"model":"fake/m","context":{"maxWindowTokens":32000,"compactAtPercent":75},
		"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536,"maxOutputTokens":4096}}}}}`, srv.URL)
	if err := os.WriteFile(filepath.Join(cfgDir, "moca", "config.jsonc"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(*mocaBin, "-p", "read the files", "--approve")
	cmd.Dir = work
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+cfgDir,
		"XDG_DATA_HOME="+dataDir,
		"MOCA_T_KEY=k",
		"MOCA_T_BROKEN=",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("moca -p: %v\n%s", err, out)
	}
	if turns < 2 {
		t.Fatalf("scripted work turns not reached: %d", turns)
	}

	const ceiling = 32000 * 80 / 100
	var summaries int
	for i, body := range rec.all() {
		// The request body is JSON; chars/4 is a conservative independent
		// estimate for the gate. The agent's own estimate is not trusted here.
		if len(body)/4 > ceiling {
			t.Fatalf("request %d is %d estimated tokens, above the %d ceiling", i, len(body)/4, ceiling)
		}
		var req struct {
			Tools []json.RawMessage `json:"tools"`
		}
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			summaries++
		}
	}
	if summaries == 0 {
		t.Fatal("the compaction trigger must issue at least one summary request")
	}
	if strings.Contains(string(out), "MOCA_T_KEY") {
		t.Fatal("the provider key leaked into the run output")
	}
}

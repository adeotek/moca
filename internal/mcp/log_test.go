package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

// logSink is a goroutine-safe buffer for captured log output.
type logSink struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *logSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// captureLog swaps the slog default for a text handler into a buffer at
// level, restoring the previous default when the test ends. Never combine
// with t.Parallel.
func captureLog(t *testing.T, level slog.Level) *logSink {
	t.Helper()
	s := &logSink{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(s, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return s
}

func TestMCPLifecycleLogged(t *testing.T) {
	log := captureLog(t, slog.LevelDebug)
	s, _ := countingServer(t)
	s.Env["MOCA_LOG_PROBE_SECRET"] = "hunter2"
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, 150*time.Millisecond, ix, Options{BaseEnv: os.Environ()})
	if _, _, err := m.Call(context.Background(), "docs", "read_doc", json.RawMessage(`{"lib":"x"}`)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond) // idle stop
	m.Close()
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(log.String(), `msg="mcp server exited"`) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	out := log.String()
	for _, want := range []string{`msg="mcp server started" server=docs transport=stdio pid=`, `msg="mcp server ready" server=docs tools=`,
		`msg="mcp call" server=docs tool=read_doc`, `msg="mcp server idle stop" server=docs`, `msg="mcp server exited" server=docs`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "hunter2") || strings.Contains(out, `"lib"`) {
		t.Fatalf("env or call arguments leaked:\n%s", out)
	}
}

// TestStartFailureLogOmitsServerText: a failing server's stderr and a
// failing HTTP server's URL query reach the error text — neither may reach
// the log.
func TestStartFailureLogOmitsServerText(t *testing.T) {
	log := captureLog(t, slog.LevelDebug)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{
		"crash": {Command: "sh", Args: []string{"-c", "echo S3cretFromStderr >&2; exit 1"}},
		"web":   {URL: "http://127.0.0.1:1/x?api_key=S3cretInQuery"},
	}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	for _, name := range []string{"crash", "web"} {
		if _, err := m.Describe(context.Background(), name, "t"); err == nil {
			t.Fatalf("%s must fail to start", name)
		}
	}
	out := log.String()
	if strings.Count(out, `msg="mcp server start failed"`) != 2 {
		t.Fatalf("want two start-failure lines:\n%s", out)
	}
	if strings.Contains(out, "S3cret") {
		t.Fatalf("server text or URL query leaked:\n%s", out)
	}
}

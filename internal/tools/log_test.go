package tools

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

	"github.com/adeotek/moca/internal/llm"
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

// TestToolPanicLogsStack: the stack of a recovered tool panic reaches the
// log (the model only sees the panic value).
func TestToolPanicLogsStack(t *testing.T) {
	log := captureLog(t, slog.LevelInfo)
	NewRegistry(panicTool{}).Run(context.Background(), &Env{}, llm.ToolCall{Name: "boom", Input: json.RawMessage(`{}`)})
	out := log.String()
	if !strings.Contains(out, `level=ERROR msg="tool panic" tool=boom`) || !strings.Contains(out, "goroutine") ||
		!strings.Contains(out, "panicTool.Run") {
		t.Fatalf("stack missing:\n%s", out)
	}
}

func TestToolCallLoggedWithoutContent(t *testing.T) {
	log := captureLog(t, slog.LevelDebug)
	NewRegistry(failTool{}).Run(context.Background(), &Env{}, llm.ToolCall{Name: "fail", Input: json.RawMessage(`{"secret_arg":"hunter2"}`)})
	out := log.String()
	if !strings.Contains(out, "msg=tool name=fail") || !strings.Contains(out, "is_error=true") {
		t.Fatalf("call line missing:\n%s", out)
	}
	if strings.Contains(out, "hunter2") || strings.Contains(out, "no such file") {
		t.Fatalf("tool input/output leaked:\n%s", out)
	}
}

func TestSpillLogged(t *testing.T) {
	log := captureLog(t, slog.LevelDebug)
	Spill(&Env{SpillDir: t.TempDir(), SpillPrefix: "s-"}, "shell", "abc")
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, nil, 0o600)
	Spill(&Env{SpillDir: filepath.Join(f, "sub")}, "web", "abc")
	out := log.String()
	if !strings.Contains(out, "msg=spill kind=shell bytes=3") || !strings.Contains(out, `level=WARN msg="spill failed" kind=web`) {
		t.Fatalf("spill lines:\n%s", out)
	}
}

package tui

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
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

func TestSetupFirstSessionLogged(t *testing.T) {
	log := captureLog(t, slog.LevelInfo)
	t.Setenv("MOCA_SETUP_KEY", "k")
	m, _ := setupModel(t, "env:MOCA_SETUP_KEY")
	t.Cleanup(func() {
		if m.agent != nil {
			m.agent.Close()
		}
	})
	simulate(m, m.runCommand(Parsed{Kind: KindCommand, Name: "model", Args: "fake/m2"}))
	if out := log.String(); !strings.Contains(out, `msg="setup: first session" model=fake/m2`) {
		t.Fatalf("setup line missing:\n%s", out)
	}
}

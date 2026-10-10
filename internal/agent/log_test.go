package agent

import (
	"bytes"
	"context"
	"errors"
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

func TestAgentLogsSessionAndRunEnd(t *testing.T) {
	log := captureLog(t, slog.LevelDebug)
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTest(t, s, 40)
	if _, err := a.Run(context.Background(), "private prompt words"); err != nil {
		t.Fatal(err)
	}
	out := log.String()
	id := "session=" + a.Session().ID8()
	for _, want := range []string{"msg=session " + id, "resumed=false", `msg="run end" ` + id, "outcome=done", "steps=1", "usage.in="} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "private prompt") || strings.Contains(out, "msg=step") {
		t.Fatalf("prompt leaked, or a tool-less turn logged a step:\n%s", out)
	}
}

func TestAgentLogsNudgeAndWarning(t *testing.T) {
	log := captureLog(t, slog.LevelInfo)
	s := newScript(t, textTurn("no file"), textTurn("still no file"))
	a, _, _ := startTest(t, s, 40, planMode)
	if _, err := a.Run(context.Background(), "plan it"); err != nil {
		t.Fatal(err)
	}
	out := log.String()
	if !strings.Contains(out, "msg=nudge") || !strings.Contains(out, "kind=plan") ||
		!strings.Contains(out, "level=WARN msg=warning") || !strings.Contains(out, "without a plan file") {
		t.Fatalf("nudge/warning lines:\n%s", out)
	}
}

func TestOutcomeKind(t *testing.T) {
	cases := []struct {
		out  Outcome
		err  error
		want string
	}{
		{Outcome{}, nil, "done"},
		{Outcome{MaxSteps: true}, nil, "max_steps"},
		{Outcome{Stuck: true}, nil, "stuck"},
		{Outcome{}, context.Canceled, "interrupted"},
		{Outcome{}, errors.New("x"), "error"},
	}
	for _, c := range cases {
		if got := outcomeKind(c.out, c.err); got != c.want {
			t.Errorf("%+v %v: %q want %q", c.out, c.err, got, c.want)
		}
	}
}

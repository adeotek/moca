//go:build !windows

package tools

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRunShellBasics(t *testing.T) {
	dir := t.TempDir()
	out, err := RunShell(context.Background(), dir, os.Environ(), "pwd; echo err >&2; printf noeol; exit 3", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Output, dir) || !strings.Contains(out.Output, "err") || !strings.HasSuffix(out.Output, "noeol") || out.ExitCode != 3 {
		t.Fatalf("%+v", out)
	}
}

func TestRunShellStdinIsDevNull(t *testing.T) {
	out, _ := RunShell(context.Background(), t.TempDir(), os.Environ(), "cat; echo done", 5*time.Second)
	if strings.TrimSpace(out.Output) != "done" {
		t.Fatalf("stdin must be /dev/null, got %q", out.Output)
	}
}

func TestRunShellTimeoutKillsGroup(t *testing.T) {
	start := time.Now()
	out, _ := RunShell(context.Background(), t.TempDir(), os.Environ(), "sleep 30 & sleep 30; echo never", 300*time.Millisecond)
	if !out.TimedOut || time.Since(start) > 5*time.Second || strings.Contains(out.Output, "never") {
		t.Fatalf("%+v after %v", out, time.Since(start))
	}
}

func TestRunShellInvalidUTF8(t *testing.T) {
	out, _ := RunShell(context.Background(), t.TempDir(), os.Environ(), `printf 'a\377b'`, 5*time.Second)
	if out.Output != "a\uFFFDb" {
		t.Fatalf("%q", out.Output)
	}
}

func TestShellEnv(t *testing.T) {
	env := ShellEnv([]string{"PATH=/bin", "OPENAI_API_KEY=sk", "PAGER=less", "HOME=/h"}, []string{"OPENAI_API_KEY"})
	for _, want := range []string{"PATH=/bin", "HOME=/h", "PAGER=cat", "GIT_PAGER=cat", "GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0"} {
		if !slices.Contains(env, want) {
			t.Errorf("missing %s in %v", want, env)
		}
	}
	for _, e := range env {
		if strings.HasPrefix(e, "OPENAI_API_KEY=") || e == "PAGER=less" {
			t.Errorf("leaked %s", e)
		}
	}
}

func TestTruncate(t *testing.T) {
	var sb strings.Builder
	for i := range 10000 {
		sb.WriteString(strings.Repeat("x", 9) + string(rune('0'+i%10)) + "\n")
	}
	got := Truncate(sb.String(), 30_000)
	if len(got) > 30_100 || !strings.Contains(got, "lines omitted]") {
		t.Fatalf("len %d", len(got))
	}
	if Truncate("short", 30_000) != "short" {
		t.Fatal("no-op under cap")
	}
}

func TestTruncateLongLines(t *testing.T) {
	// One line longer than the whole budget: both ends must still be shown.
	long := strings.Repeat("x", 100_000)
	got := Truncate(long, 30_000)
	if !strings.HasPrefix(got, "xxx") || !strings.HasSuffix(got, "xxx") || !strings.Contains(got, "bytes omitted]") {
		t.Fatalf("len=%d %q…%q", len(got), got[:40], got[len(got)-40:])
	}
	if len(got) > 30_100 {
		t.Fatalf("oversized: %d", len(got))
	}
	// Normal head lines followed by one very long line: the tail must survive.
	mixed := strings.Repeat("short line\n", 40) + strings.Repeat("y", 100_000)
	got = Truncate(mixed, 30_000)
	if !strings.Contains(got, "short line") || !strings.Contains(got, "bytes omitted]") || !strings.HasSuffix(got, "yyy") {
		t.Fatalf("mixed: len=%d tail=%q", len(got), got[len(got)-20:])
	}
}

func TestRunShellBoundsCapture(t *testing.T) {
	out, err := RunShell(context.Background(), t.TempDir(), os.Environ(), "seq 1 200000", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Output, "bytes omitted]") || len(out.Output) > shellCaptureMax+100 {
		t.Fatalf("capture must be bounded: %d bytes", len(out.Output))
	}
	if !strings.HasPrefix(out.Output, "1\n") || !strings.HasSuffix(strings.TrimSpace(out.Output), "200000") {
		t.Fatalf("head/tail lost: %q…%q", out.Output[:10], out.Output[len(out.Output)-20:])
	}
}

func TestRunShellSetsidDescendantDoesNotBlock(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid not available")
	}
	start := time.Now()
	out, err := RunShell(context.Background(), t.TempDir(), os.Environ(), "setsid sleep 30 & echo hi", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatalf("Wait blocked on a straggler holding the pipe: %v", time.Since(start))
	}
	if !strings.Contains(out.Output, "hi") || out.TimedOut || out.ExitCode != 0 {
		t.Fatalf("%+v", out)
	}
}

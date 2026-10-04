//go:build !windows

package tools

import (
	"context"
	"os"
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

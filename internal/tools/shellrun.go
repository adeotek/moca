package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

type ShellOutput struct {
	Output   string
	ExitCode int
	TimedOut bool
}

var forcedEnv = []string{"PAGER=cat", "GIT_PAGER=cat", "GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0"}

func ShellEnv(base []string, strip []string) []string {
	drop := map[string]bool{"PAGER": true, "GIT_PAGER": true, "GIT_EDITOR": true, "GIT_TERMINAL_PROMPT": true}
	for _, s := range strip {
		drop[s] = true
	}
	var out []string
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if !drop[k] {
			out = append(out, kv)
		}
	}
	return append(out, forcedEnv...)
}

func RunShell(ctx context.Context, dir string, env []string, command string, timeout time.Duration) (ShellOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	name, args, err := shellCommand(command)
	if err != nil {
		return ShellOutput{}, err
	}
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, env
	// After a kill, do not wait on stragglers (setsid descendants) holding
	// the output pipe.
	cmd.WaitDelay = 2 * time.Second
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	cmd.Stdin = devnull
	w := &boundedWriter{max: shellCaptureMax}
	cmd.Stdout, cmd.Stderr = w, w
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return ShellOutput{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var werr error
	select {
	case werr = <-done:
	case <-ctx.Done():
		killProcessGroup(cmd)
		werr = <-done
	}
	out := ShellOutput{Output: strings.ToValidUTF8(w.String(), "\uFFFD")}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
	}
	var ee *exec.ExitError
	switch {
	case werr == nil:
	case errors.Is(werr, exec.ErrWaitDelay):
		// the command exited; a straggler kept the output pipe open
	case errors.As(werr, &ee):
		out.ExitCode = ee.ExitCode()
	default:
		return out, werr
	}
	if ctx.Err() != nil && !out.TimedOut {
		return out, ctx.Err() // parent cancelled (esc / Ctrl-C)
	}
	return out, nil
}

// shellCaptureMax bounds what a shell command's output can hold in memory;
// the shell tool only ever returns 30K chars, so the rest is dropped with a
// visible note (an unbounded buffer lets `yes` exhaust memory).
const shellCaptureMax = 64 << 10

// boundedWriter keeps the first half and the last half of what is written,
// up to max bytes.
type boundedWriter struct {
	head, tail []byte
	max        int
	total      int64
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.total += int64(n)
	half := w.max / 2
	if len(w.head) < half {
		k := min(half-len(w.head), n)
		w.head = append(w.head, p[:k]...)
		p = p[k:]
	}
	if len(p) > 0 {
		w.tail = append(w.tail, p...)
		if d := len(w.tail) - half; d > 0 {
			w.tail = append(w.tail[:0], w.tail[d:]...)
		}
	}
	return n, nil
}

func (w *boundedWriter) String() string {
	dropped := w.total - int64(len(w.head)+len(w.tail))
	if dropped <= 0 {
		return string(w.head) + string(w.tail)
	}
	return string(w.head) + fmt.Sprintf("[… %d bytes omitted]\n", dropped) + string(w.tail)
}

// Truncate keeps whole lines from the head and tail, ~max/2 chars each. When
// a side cannot hold even one line (output dominated by a single long line),
// it falls back to character windows snapped to rune boundaries.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	lines := strings.SplitAfter(s, "\n")
	half := max / 2
	var head, tail []string
	n := 0
	i := 0
	for ; i < len(lines) && n+len(lines[i]) <= half; i++ {
		head = append(head, lines[i])
		n += len(lines[i])
	}
	n = 0
	j := len(lines) - 1
	for ; j >= i && n+len(lines[j]) <= half; j-- {
		tail = append([]string{lines[j]}, tail...)
		n += len(lines[j])
	}
	if len(head) == 0 || len(tail) == 0 {
		h := runePrefix(s, half)
		t := runeSuffix(s, half)
		return h + fmt.Sprintf("[… %d bytes omitted]\n", len(s)-len(h)-len(t)) + t
	}
	omitted := j - i + 1
	return strings.Join(head, "") + fmt.Sprintf("[… %d lines omitted]\n", omitted) + strings.Join(tail, "")
}

// runePrefix returns the longest prefix of s that fits in n bytes without
// splitting a rune.
func runePrefix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// runeSuffix returns the longest suffix of s that fits in n bytes without
// splitting a rune.
func runeSuffix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	i := len(s) - n
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return s[i:]
}

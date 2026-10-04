package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	cmd.Stdin = devnull
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
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
	out := ShellOutput{Output: strings.ToValidUTF8(buf.String(), "\uFFFD")}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
	}
	var ee *exec.ExitError
	switch {
	case werr == nil:
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

// Truncate keeps whole lines from the head and tail, ~max/2 chars each.
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
	omitted := j - i + 1
	return strings.Join(head, "") + fmt.Sprintf("[… %d lines omitted]\n", omitted) + strings.Join(tail, "")
}

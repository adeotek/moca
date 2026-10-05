package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

const shellMaxOutput = 30_000

type shellTool struct{}

func (shellTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "shell", Description: "Run a shell command (bash -c on Unix, PowerShell on Windows). " +
		"Stateless: every call starts in the workdir, so use `cd dir && cmd` within one call. stdin is empty; " +
		"stdout and stderr are merged; output over 30K chars keeps head and tail. Commands are checked against " +
		"an allowlist; some need user approval and some are never allowed.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"command":{"type":"string","description":"The command line"},` +
			`"timeout":{"type":"integer","minimum":1,"maximum":300,"description":"Seconds (default 30)"}},` +
			`"required":["command"],"additionalProperties":false}`)}
}

func approve(ctx context.Context, env *Env, names []string, command string, canAlways bool) error {
	for _, n := range names {
		ans := Deny
		if env.Ask != nil {
			ans = env.Ask(ctx, Question{Kind: "shell", Subject: n, Detail: command, CanAlways: canAlways})
		}
		switch ans {
		case Deny:
			if canAlways {
				return fmt.Errorf("refused: `%s` is not allowlisted and the user did not approve it; ask the user to allow it or use another approach", n)
			}
			return fmt.Errorf("refused: `%s` needs explicit user approval for every call and it was not given", n)
		case AllowAlways:
			env.Commands.Allow(n)
		}
	}
	return nil
}

func (shellTool) Run(ctx context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Command string `json:"command"`
		Timeout *int   `json:"timeout"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	timeout := 30
	if a.Timeout != nil {
		timeout = *a.Timeout
	}
	if timeout < 1 || timeout > 300 {
		return errorf("timeout must be 1..300 seconds")
	}
	if strings.TrimSpace(a.Command) == "" {
		return errorf("empty command")
	}
	need, every, err := env.Commands.Check(a.Command)
	if err != nil {
		return errorf("refused: %v", err)
	}
	if err := approve(ctx, env, need, a.Command, true); err != nil {
		return errorf("%v", err)
	}
	if err := approve(ctx, env, every, a.Command, false); err != nil {
		return errorf("%v", err)
	}
	out, err := RunShell(ctx, env.Root, env.ShellEnv, a.Command, time.Duration(timeout)*time.Second)
	if err != nil {
		return errorf("%v", err)
	}
	body := Truncate(out.Output, shellMaxOutput)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	status := fmt.Sprintf("[exit %d]", out.ExitCode)
	if out.TimedOut {
		status = fmt.Sprintf("[timed out after %ds — process group killed]", timeout)
	}
	short := a.Command
	if len(short) > 60 {
		short = short[:57] + "..."
	}
	return Result{Content: body + status, IsError: out.ExitCode != 0 || out.TimedOut,
		Summary: fmt.Sprintf("%s %s", short, status), Detail: Truncate(out.Output, shellMaxOutput)}
}

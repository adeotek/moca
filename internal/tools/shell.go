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
	if env.Plan {
		return errorf("%s", planShellRefusal)
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
	if testRunCommand(a.Command) {
		switch {
		case out.TimedOut: // inconclusive — keep the state
		case testRunFailed(a.Command, out.Output, out.ExitCode, out.TimedOut):
			env.TestFailed = true
			if !env.TestSeen {
				if f := findTestFile(out.Output); f != "" {
					env.FailingTest = f
				} else if env.FailingTest == "" {
					env.FailingTest = "*_test.go"
				}
			}
		default: // a green run ends the failing state
			env.TestFailed, env.FailingTest = false, ""
		}
	}
	return Result{Content: body + status, IsError: out.ExitCode != 0 || out.TimedOut,
		Summary: fmt.Sprintf("%s %s", short, status), Detail: Truncate(out.Output, shellMaxOutput)}
}

// testRunners identify a test run from its command line (case-insensitive).
var testRunners = []string{"go test", "pytest", "cargo test", "dotnet test", "npm test", "npm run test",
	"yarn test", "jest", "vitest", "rspec", "phpunit", "mvn test", "gradle test", "make test", "rtk test"}

// testRunCommand reports whether the command line runs tests.
func testRunCommand(command string) bool {
	lc := strings.ToLower(command)
	for _, m := range testRunners {
		if strings.Contains(lc, m) {
			return true
		}
	}
	return false
}

// testRunFailed reports a failing test run: a runner marker in the command
// and a failure signal (exit != 0, or `fail` in the output — piped runs mask
// the exit code).
func testRunFailed(command, output string, exit int, timedOut bool) bool {
	return !timedOut && testRunCommand(command) &&
		(exit != 0 || strings.Contains(strings.ToLower(output), "fail"))
}

// findTestFile returns the first *_test.go file named in test output
// ("calc_test.go:17: Sum(...) = 5, want 6").
func findTestFile(output string) string {
	for i := 0; ; {
		j := strings.Index(output[i:], "_test.go")
		if j < 0 {
			return ""
		}
		j += i
		start := j
		for start > 0 && isPathByte(output[start-1]) {
			start--
		}
		if start < j {
			return output[start : j+len("_test.go")]
		}
		i = j + len("_test.go")
	}
}

func isPathByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' ||
		b == '_' || b == '-' || b == '.' || b == '/'
}

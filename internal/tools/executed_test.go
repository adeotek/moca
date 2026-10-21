package tools

import (
	"reflect"
	"strings"
	"testing"
)

func TestExecutedSegments(t *testing.T) {
	for _, c := range []struct {
		cmd  string
		want []string
	}{
		{"go test ./...", []string{"go test ./..."}},
		{"cd sub && make test CALC_STRICT=1", []string{"make test CALC_STRICT=1"}},
		// `rtk <verb>` for a command-running verb unwraps to the command it
		// ran (permissions/rtk.go's rtkRun): the evidence must carry what the
		// session actually executed — v073-cd3 measured the guard refusing a
		// true rule (`go test ./...`, run as `rtk test -- go test ./...`) and
		// the corpus judging a steered session unsteered.
		{"rtk test -- go test ./...", []string{"go test ./..."}},
		{"rtk test -- make test", []string{"make test"}},
		{"rtk test go test ./...", []string{"go test ./..."}},
		{"rtk git status", []string{"git status"}},
		{"CALC_STRICT=1 go test ./... | tee out.log", []string{"go test ./...", "tee out.log"}},
		{"   ", nil},
	} {
		if got := ExecutedSegments(c.cmd); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExecutedSegments(%q) = %#v, want %#v", c.cmd, got, c.want)
		}
	}
}

// The evidence exists only for calls that RAN: a refusal (permission, plan
// mode, guard) must not let a rule cite a command the session was not allowed
// to run, and BeginRun clears the list because a resumed session's earlier
// commands are not this process's history.
func TestRunRecordsExecutedCommands(t *testing.T) {
	env, _ := testEnv(t)
	env.Commands = &fakeCmds{}
	env.ShellEnv = []string{"PATH=" + t.TempDir()}
	reg := NewRegistry(shellTool{})

	runReg(t, reg, env, "shell", map[string]any{"command": "echo hi && echo there"})
	if got := strings.Join(env.Executed, "|"); got != "echo hi|echo there" {
		t.Fatalf("a ran command is evidence: %q", got)
	}

	env.Commands = &fakeCmds{need: []string{"echo"}} // every call asks; no asker → refused
	if r := runReg(t, reg, env, "shell", map[string]any{"command": "echo nope"}); !strings.HasPrefix(r.Content, "refused:") {
		t.Fatalf("setup: the call must be refused, got %q", r.Content)
	}
	if len(env.Executed) != 2 {
		t.Fatalf("a refusal must not add evidence: %#v", env.Executed)
	}

	env.BeginRun()
	if env.Executed != nil {
		t.Fatalf("BeginRun clears the executed list: %#v", env.Executed)
	}
}

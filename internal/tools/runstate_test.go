package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestVerifies(t *testing.T) {
	yes := []string{"go test ./...", "make vet && make test", "cd sub && go build ./...", "rtk test -- go test ./...",
		"GOFLAGS=-mod=mod go vet ./...", "npm run lint", "go run . -to f 100", "git status && cargo check", "./bin/tool --help"}
	no := []string{"cat a.go", "git diff", "ls -la | head", "grep -n foo *.go", "cd x && git status", "sed -n 1,5p a.go"}
	for _, c := range yes {
		if !verifies(c) {
			t.Errorf("verifies(%q) = false", c)
		}
	}
	for _, c := range no {
		if verifies(c) {
			t.Errorf("verifies(%q) = true", c)
		}
	}
}

// failTool always fails with the same output.
type failTool struct{}

func (failTool) Spec() llm.ToolSpec { return llm.ToolSpec{Name: "fail"} }
func (failTool) Run(context.Context, *Env, json.RawMessage) Result {
	return errorf("no such file")
}

func TestRepeatedFailureHintAndReset(t *testing.T) {
	reg := NewRegistry(failTool{})
	env := &Env{}
	call := func(in string) Result {
		return reg.Run(context.Background(), env, llm.ToolCall{Name: "fail", Input: json.RawMessage(in)})
	}
	call(`{"a": 1}`)
	if r := call(`{"a":1}`); strings.Contains(r.Content, "[hint: this exact call") {
		t.Fatalf("2nd identical failure must not hint yet: %q", r.Content)
	}
	if r := call(`{"a":1}`); !strings.Contains(r.Content, "failed 3 times") || env.MaxRepeat != 3 {
		t.Fatalf("3rd identical failure (whitespace-normalized input) must hint: %q max=%d", r.Content, env.MaxRepeat)
	}
	if r := call(`{"a":2}`); strings.Contains(r.Content, "[hint") {
		t.Fatal("a different input is a different call")
	}
	noteChange(env, "/x/a.go") // a change resets the repeat history
	if r := call(`{"a":1}`); strings.Contains(r.Content, "[hint") {
		t.Fatal("a change must reset the counts")
	}
	env.BeginRun()
	if env.MaxRepeat != 0 || env.failures != nil {
		t.Fatal("BeginRun resets")
	}
}

type panicTool struct{}

func (panicTool) Spec() llm.ToolSpec { return llm.ToolSpec{Name: "boom"} }
func (panicTool) Run(context.Context, *Env, json.RawMessage) Result {
	var s []int
	_ = s[3]
	return Result{}
}

// TestRegistryRecoversToolPanic: a bug inside one tool becomes an error
// result for that call, not a crash of the whole session (eval feature#3).
func TestRegistryRecoversToolPanic(t *testing.T) {
	r := NewRegistry(panicTool{}).Run(context.Background(), &Env{}, llm.ToolCall{Name: "boom", Input: json.RawMessage(`{}`)})
	if !r.IsError || !strings.Contains(r.Content, "internal error in boom") || !strings.Contains(r.Content, "index out of range") {
		t.Fatalf("%+v", r)
	}
}

// spillFailTool fails with output over a cap: every call spills to a fresh
// overflow file, so the result names a different path each time.
type spillFailTool struct{}

func (spillFailTool) Spec() llm.ToolSpec { return llm.ToolSpec{Name: "spillfail"} }
func (spillFailTool) Run(_ context.Context, env *Env, _ json.RawMessage) Result {
	return errorf("build failed\n%s", Spill(env, "shell", strings.Repeat("error line\n", 50)))
}

// TestRepeatedFailureIgnoresSpillPath: the overflow file name is random per
// call; the same failing call with a big output is still a repeat.
func TestRepeatedFailureIgnoresSpillPath(t *testing.T) {
	reg := NewRegistry(spillFailTool{})
	env := &Env{SpillDir: t.TempDir(), SpillPrefix: "s-"}
	var r Result
	for range 3 {
		r = reg.Run(context.Background(), env, llm.ToolCall{Name: "spillfail", Input: json.RawMessage(`{}`)})
	}
	if !strings.Contains(r.Content, "saved to ") || !strings.Contains(r.Content, "failed 3 times") || env.MaxRepeat != 3 {
		t.Fatalf("3rd identical spilling failure must hint: %q max=%d", r.Content, env.MaxRepeat)
	}
}

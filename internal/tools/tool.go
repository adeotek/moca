// Package tools defines the Tool interface and the built-in tools. It never
// imports permissions or session; those implement the small interfaces below
// structurally and agent injects them through Env.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type Result struct {
	Content string
	IsError bool
	Summary string
	Detail  string
}

type Tool interface {
	Spec() llm.ToolSpec
	Run(ctx context.Context, env *Env, input json.RawMessage) Result
}

type PathChecker interface {
	Resolve(path string, write bool) (string, error)
}

type CommandChecker interface {
	Check(command string) (needApproval, askEveryTime []string, err error)
	Allow(name string)
}

type Snapshotter interface {
	Guard(absPath string, write func() error) error
}

type Answer int

const (
	Deny Answer = iota
	AllowOnce
	AllowAlways
)

// Question is an approval request. Kind is "shell" or "mcp"; Subject the
// command name or server/tool; Detail the full command or args.
type Question struct {
	Kind, Subject, Detail string
	CanAlways             bool
}

type Asker func(ctx context.Context, q Question) Answer

// AutoAllow is the yolo-mode Asker: every approval is granted once.
func AutoAllow(context.Context, Question) Answer { return AllowOnce }

type Env struct {
	Root     string
	Paths    PathChecker
	Commands CommandChecker
	Ask      Asker
	Reads    *ReadTracker
	Snap     Snapshotter
	ShellEnv []string
	// TestSeen/TestFailed/FailingTest/Searched drive the investigation hints:
	// a failing test run sets TestFailed and FailingTest (the file name parsed
	// from its output); reading any *_test.go sets TestSeen and clears
	// FailingTest; a search call sets Searched; a green test run clears
	// TestFailed/FailingTest (shell.go/read.go/search.go).
	TestSeen    bool
	TestFailed  bool
	FailingTest string
	Searched    bool
}

type Registry struct {
	order []string
	tools map[string]Tool
}

func NewRegistry(ts ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range ts {
		r.Register(t)
	}
	return r
}

func (r *Registry) Register(t Tool) {
	name := t.Spec().Name
	if _, ok := r.tools[name]; !ok {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

func (r *Registry) Specs() []llm.ToolSpec {
	out := make([]llm.ToolSpec, len(r.order))
	for i, n := range r.order {
		out[i] = r.tools[n].Spec()
	}
	return out
}

func (r *Registry) Run(ctx context.Context, env *Env, call llm.ToolCall) Result {
	t, ok := r.tools[call.Name]
	if !ok {
		return errorf("unknown tool %q; available: %s", call.Name, strings.Join(r.order, ", "))
	}
	if !json.Valid(call.Input) {
		return errorf("invalid JSON arguments for %s (the call was probably cut off at the output limit). "+
			"Split the work into smaller calls, e.g. write a large file in parts with edit.", call.Name)
	}
	res := t.Run(ctx, env, call.Input)
	if h := investigationHint(env); h != "" {
		res.Content += h
	}
	return res
}

// investigationHint is the protocol banner appended to every tool result
// while a failing test run is unresolved: weak models fix from the failure
// text (or shell out entirely with cat/sed) without reading the failing test
// or locating the cause with search; the §14 demo requires both before the
// fix. State: shell.go (TestFailed/FailingTest, cleared by a green run),
// read.go (TestSeen/FailingTest), search.go (Searched).
func investigationHint(env *Env) string {
	if env.FailingTest != "" {
		return fmt.Sprintf("\n[hint: tests failed: (1) read the failing test file (%s) with the read tool, (2) locate the cause with the search tool, (3) only then edit]", env.FailingTest)
	}
	if env.TestFailed && !env.Searched {
		return "\n[hint: locate the cause with the search tool before editing]"
	}
	return ""
}

// investigationRefusal is the edit gate for the same protocol: while a
// failing test run's investigation is incomplete the edit tool refuses —
// weak models treat "read the file"/"locate the cause" as satisfied by
// shell equivalents (cat/sed), so hints alone lose; the guard is what binds.
func investigationRefusal(env *Env) string {
	if !env.TestFailed || (env.TestSeen && env.Searched) {
		return ""
	}
	if !env.TestSeen {
		name := env.FailingTest
		if name == "" {
			name = "*_test.go"
		}
		return fmt.Sprintf("refused: tests are failing — read the failing test file (%s) with the read tool and locate the cause with the search tool before editing", name)
	}
	return "refused: tests are failing — locate the cause with the search tool before editing"
}

func decode[T any](input json.RawMessage, v *T) *Result {
	dec := json.NewDecoder(strings.NewReader(string(input)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		r := errorf("invalid arguments: %v", err)
		return &r
	}
	return nil
}

func errorf(format string, a ...any) Result {
	return Result{Content: fmt.Sprintf(format, a...), IsError: true}
}

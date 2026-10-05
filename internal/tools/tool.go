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
	return t.Run(ctx, env, call.Input)
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

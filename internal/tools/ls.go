package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type lsTool struct{}

func (lsTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "ls", Description: "List one directory level, sorted. Directories end with `/`, " +
		"symlinks with `@`. Hidden entries only with hidden=true. Use search to find files recursively.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"Directory (default: workdir)"},` +
			`"hidden":{"type":"boolean","description":"Include dotfiles (default false)"}},` +
			`"additionalProperties":false}`)}
}

func (lsTool) Run(_ context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path   string `json:"path"`
		Hidden bool   `json:"hidden"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	if a.Path == "" {
		a.Path = "."
	}
	abs, err := env.Paths.Resolve(a.Path, false)
	if err != nil {
		return errorf("%v", err)
	}
	ents, err := os.ReadDir(abs)
	if err != nil {
		return errorf("%v", err)
	}
	var out []string
	for _, e := range ents {
		name := e.Name()
		if !a.Hidden && strings.HasPrefix(name, ".") {
			continue
		}
		switch {
		case e.Type()&os.ModeSymlink != 0:
			name += "@"
		case e.IsDir():
			name += "/"
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return Result{Content: "[empty directory]", Summary: a.Path}
	}
	more := 0
	if len(out) > 1000 {
		more, out = len(out)-1000, out[:1000]
	}
	s := strings.Join(out, "\n")
	if more > 0 {
		s += fmt.Sprintf("\n[… %d more]", more)
	}
	return Result{Content: s, Summary: fmt.Sprintf("%s (%d)", a.Path, len(out)+more)}
}

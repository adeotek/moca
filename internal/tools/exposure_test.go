package tools

// Exposure moved from "codemode is a ninth tool beside eight" to "inspection
// lives inside the script" (SPECS §10.6). The tests here pin the two things that
// can silently break: the declared list (what every request carries) and the
// fact that hiding a tool does not remove it from the dispatcher.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func declaredNames(r *Registry) []string {
	specs := r.Specs()
	out := make([]string, len(specs))
	for i, sp := range specs {
		out[i] = sp.Name
	}
	return out
}

func TestAdditiveDeclaresTheSameNine(t *testing.T) {
	r := NewRegistry(Builtins()...)
	r.Register(NewCodemodeRefusal(false))

	want := []string{"read", "write", "edit", "shell", "search", "ls", "web", "mcp", "codemode"}
	got := declaredNames(r)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("declared = %v, want %v", got, want)
	}
	for _, n := range want {
		if r.ExposureOf(n) != Direct {
			t.Fatalf("%s is not Direct in additive", n)
		}
	}
}

func TestTransportDeclaresSixAndKeepsTheDispatcher(t *testing.T) {
	r := NewRegistry(Builtins()...)
	r.Register(NewCodemodeRefusal(false))
	r.CodemodeOnly("read", "search", "ls")

	want := []string{"write", "edit", "shell", "web", "mcp", "codemode"}
	got := declaredNames(r)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("declared = %v, want %v", got, want)
	}
	// codemode is still last, and the hidden tools are still registered: the
	// bridge dispatches them, the model just cannot call them.
	if n := len(r.Names()); n != 9 {
		t.Fatalf("registry has %d names, want 9", n)
	}
	for _, n := range []string{"read", "search", "ls"} {
		if r.ExposureOf(n) != Codemode {
			t.Fatalf("%s is not Codemode after CodemodeOnly", n)
		}
	}
	if r.ExposureOf("write") != Direct {
		t.Fatal("CodemodeOnly widened to a tool it was not given")
	}
}

// A direct call to a hidden tool is refused with a message that names the path
// the model should take, and it never reaches the tool.
func TestDirectCallToAHiddenToolIsRefused(t *testing.T) {
	dir := t.TempDir()
	env := &Env{Root: dir}
	r := NewRegistry(Builtins()...)
	r.CodemodeOnly("read")
	res := r.Run(context.Background(), env, mustCall(t, "read", `{"path":"x"}`))
	if !res.IsError {
		t.Fatalf("a hidden tool ran: %+v", res)
	}
	if !strings.Contains(res.Content, "only inside a codemode script") {
		t.Fatalf("refusal does not point at the script: %q", res.Content)
	}
}

func TestCodemodeOnlyIgnoresUnknownNames(t *testing.T) {
	r := NewRegistry(Builtins()...)
	r.CodemodeOnly("nope", "read")
	if r.ExposureOf("nope") != Direct {
		t.Fatal("ExposureOf invented a tool")
	}
	if r.ExposureOf("read") != Codemode {
		t.Fatal("a known name after an unknown one was skipped")
	}
}

// CodemodeOnly is idempotent: applying the same policy twice must not double
// wrap the tool (the second wrapper would still be hidden, but the registry
// should not grow layers on every start).
func TestCodemodeOnlyIsIdempotent(t *testing.T) {
	r := NewRegistry(Builtins()...)
	r.CodemodeOnly("read")
	first := declaredNames(r)
	r.CodemodeOnly("read")
	if got := declaredNames(r); strings.Join(got, ",") != strings.Join(first, ",") {
		t.Fatalf("second application changed the declared list: %v -> %v", first, got)
	}
}

func mustCall(t *testing.T, name, input string) llm.ToolCall {
	t.Helper()
	return llm.ToolCall{Name: name, Input: json.RawMessage(input)}
}

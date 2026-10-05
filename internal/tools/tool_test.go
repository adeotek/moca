package tools

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

var update = flag.Bool("update", false, "rewrite golden files")

type echoTool struct{ name string }

func (e echoTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: e.name, Description: "echo", Schema: json.RawMessage(`{"type":"object"}`)}
}
func (e echoTool) Run(_ context.Context, _ *Env, in json.RawMessage) Result {
	var v struct {
		Msg string `json:"msg"`
	}
	if r := decode(in, &v); r != nil {
		return *r
	}
	return Result{Content: e.name + ":" + v.Msg}
}

func TestRegistryDispatch(t *testing.T) {
	r := NewRegistry(echoTool{"a"}, echoTool{"b"})
	if got := r.Run(context.Background(), &Env{}, llm.ToolCall{Name: "b", Input: json.RawMessage(`{"msg":"x"}`)}); got.Content != "b:x" {
		t.Fatal(got)
	}
	unk := r.Run(context.Background(), &Env{}, llm.ToolCall{Name: "zz", Input: json.RawMessage(`{}`)})
	if !unk.IsError || !strings.Contains(unk.Content, "read, ") && !strings.Contains(unk.Content, "a, b") {
		t.Fatalf("unknown tool must list available tools: %+v", unk)
	}
	bad := r.Run(context.Background(), &Env{}, llm.ToolCall{Name: "a", Input: json.RawMessage(`{"msg":"x`)})
	if !bad.IsError || !strings.Contains(bad.Content, "invalid JSON") || !strings.Contains(bad.Content, "smaller") {
		t.Fatalf("truncated args: %+v", bad)
	}
	r.Register(echoTool{"a"})
	if len(r.Specs()) != 2 || r.Specs()[0].Name != "a" {
		t.Fatal("Register replaces in place")
	}
}

func TestSchemasFrozen(t *testing.T) {
	got, _ := json.MarshalIndent(NewRegistry(Builtins()...).Specs(), "", "  ")
	const golden = "testdata/schemas.golden.json"
	if *update {
		os.WriteFile(golden, got, 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("tool schemas changed. They are frozen after phase 2 (DESIGN.md §4): " +
			"a change is a v2 discussion. Run with -update only during phase 2.")
	}
}

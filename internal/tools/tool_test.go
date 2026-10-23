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
		t.Fatal("tool schemas changed. They are frozen per SPECS §10: " +
			"a change is a v2 discussion (the web tool was added as rev 19). " +
			"Run with -update only for an approved change.")
	}
}

func TestAnswerString(t *testing.T) {
	for a, want := range map[Answer]string{
		Deny: "deny", AllowOnce: "once", AllowAlways: "always", AllowSession: "session",
		Answer(99): "invalid", // out of range: never panics, never reads as an answer
	} {
		if got := a.String(); got != want {
			t.Errorf("Answer(%d).String() = %q, want %q", int(a), got, want)
		}
	}
}

// TestQuestionSessionKey: a session allow is keyed by kind + subject, plus
// the command line for the exact tier. The tiers and the kinds must never
// collide — a session `git` may not answer a rule-Ask `git push`, and a shell
// subject may not answer a web host of the same text.
func TestQuestionSessionKey(t *testing.T) {
	name := Question{Kind: "shell", Subject: "git", Detail: "git status", CanAlways: true}
	exact := Question{Kind: "shell", Subject: "git", Detail: "git push origin main"}
	if name.SessionExact() || !exact.SessionExact() {
		t.Fatalf("tiers: name=%v exact=%v", name.SessionExact(), exact.SessionExact())
	}
	if name.SessionKey() == exact.SessionKey() {
		t.Fatal("the exact tier must not collide with the name tier")
	}
	// The exact key is the full command: the same line matches, another does not.
	if exact.SessionKey() != (Question{Kind: "shell", Subject: "git", Detail: "git push origin main"}).SessionKey() {
		t.Fatal("the same question must produce the same key")
	}
	if exact.SessionKey() == (Question{Kind: "shell", Subject: "git", Detail: "git push other"}).SessionKey() {
		t.Fatal("a different command line must produce a different key")
	}
	// Kinds never mix: same subject and detail, different kind.
	kinds := map[string]string{}
	for _, k := range []string{"shell", "mcp", "web", "write"} {
		key := Question{Kind: k, Subject: "x", Detail: "y"}.SessionKey()
		if prev, ok := kinds[key]; ok {
			t.Fatalf("%s and %s share a session key", prev, k)
		}
		kinds[key] = k
	}
}

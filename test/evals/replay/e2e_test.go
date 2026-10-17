//go:build replay

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// TestReplayIsDeterministic drives the built binary through one synthetic
// session (read → edit → text) twice: identical request bytes, zero prefix
// breaks, no divergence. run.sh builds first; MOCA_BIN overrides the path.
func TestReplayIsDeterministic(t *testing.T) {
	bin := os.Getenv("MOCA_BIN")
	if bin == "" {
		b, err := filepath.Abs("../../../bin/moca")
		if err != nil {
			t.Fatal(err)
		}
		bin = b
	}
	if st, err := os.Stat(bin); err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("build first (make build) — %s is not an executable", bin)
	}
	fix := t.TempDir()
	if err := os.WriteFile(filepath.Join(fix, "a.txt"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fix, "b.txt"), []byte("beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []session.Entry{
		{ID: "u1", Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleUser,
			Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "change alpha to beta in a.txt"}}}},
		{ID: "a1", Type: session.TypeMessage, Usage: &llm.Usage{Input: 10},
			Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
				{Type: llm.BlockText, Text: "reading"}}}},
		{ID: "t1", Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: "a1",
			Call: llm.ToolCall{ID: "c1", Name: "read", Input: json.RawMessage(`{"path":"a.txt"}`)}}},
		{ID: "r1", Type: session.TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "1|alpha\n[lines 1-1 of 1]"}},
		{ID: "a2", Type: session.TypeMessage, Usage: &llm.Usage{Input: 20},
			Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
				{Type: llm.BlockText, Text: "editing"}}}},
		{ID: "t2", Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: "a2",
			Call: llm.ToolCall{ID: "c2", Name: "edit",
				Input: json.RawMessage(`{"path":"a.txt","old_string":"alpha","new_string":"beta"}`)}}},
		{ID: "r2", Type: session.TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c2", Content: "changed lines 1-1; file now 1 lines"}},
		{ID: "a3", Type: session.TypeMessage, Usage: &llm.Usage{Input: 30},
			Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
				{Type: llm.BlockText, Text: "done"}}}},
	}
	turns := Turns(entries)
	scratch := t.TempDir()
	run := func(tag string) Result {
		r, err := Run(context.Background(), Options{
			Moca: bin, Fixture: fix, Prompt: PromptOf(entries),
			Dirs:          filepath.Join(scratch, tag),
			ContextWindow: 1000000, Turns: turns,
		})
		if err != nil {
			t.Fatalf("replay run: %v\noutput: %s", err, r.Output)
		}
		return r
	}
	r1, r2 := run("run1"), run("run2")
	if r1.Diverged || r2.Diverged {
		t.Fatalf("diverged: %+v / %+v", r1, r2)
	}
	if r1.PrefixBreaks != 0 || r2.PrefixBreaks != 0 {
		t.Fatalf("prefix breaks: %d / %d", r1.PrefixBreaks, r2.PrefixBreaks)
	}
	if r1.Requests != 3 || r2.Requests != 3 {
		t.Fatalf("requests: %d / %d (want 3: one per turn)", r1.Requests, r2.Requests)
	}
	if !reflect.DeepEqual(r1.Bytes, r2.Bytes) {
		t.Fatalf("bytes differ between identical runs:\n%v\n%v", r1.Bytes, r2.Bytes)
	}
}

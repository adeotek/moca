package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
)

func TestTurnsFromSession(t *testing.T) {
	entries := []session.Entry{
		{ID: "s1", Type: session.TypeSession, Session: &session.Header{}},
		{ID: "u1", Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleUser,
			Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "do it"}}}},
		{ID: "a1", Type: session.TypeMessage, Usage: &llm.Usage{Input: 10},
			Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
				{Type: llm.BlockThinking, Text: "t1"},
				{Type: llm.BlockText, Text: "working"},
			}}},
		{ID: "t1", Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: "a1",
			Call: llm.ToolCall{ID: "c1", Name: "read", Input: json.RawMessage(`{"path":"a"}`)}}},
		{ID: "t2", Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: "a1",
			Call: llm.ToolCall{ID: "c2", Name: "ls", Input: json.RawMessage(`{}`)}}},
		{ID: "r1", Type: session.TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x"}},
		{ID: "m1", Type: session.TypeCompaction, Compaction: &session.Compaction{Summary: "summary"}},
		{ID: "a2", Type: session.TypeMessage, Usage: &llm.Usage{Input: 5},
			Message: &llm.Message{Role: llm.RoleAssistant,
				Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "done"}}}},
	}
	turns := Turns(entries)
	if len(turns) != 2 {
		t.Fatalf("turns = %d, want 2 (the compaction summary is not a turn)", len(turns))
	}
	if turns[0].Text != "working" || turns[0].Thinking != "t1" || len(turns[0].Calls) != 2 {
		t.Fatalf("turn 0 = %+v", turns[0])
	}
	if turns[0].Calls[1].Name != "ls" || string(turns[0].Calls[0].Input) != `{"path":"a"}` {
		t.Fatalf("call ids/args must be verbatim: %+v", turns[0].Calls)
	}
	if turns[1].Text != "done" || len(turns[1].Calls) != 0 {
		t.Fatalf("turn 1 = %+v", turns[1])
	}
	if PromptOf(entries) != "do it" {
		t.Fatalf("prompt = %q", PromptOf(entries))
	}
	if UsesWeb(entries) {
		t.Fatal("no web call in this session")
	}
}

// TestSSERoundTrip feeds the generated stream to the real completions parser:
// the replay must reproduce text, thinking and calls exactly.
func TestSSERoundTrip(t *testing.T) {
	turn := Turn{
		Text:     "hello",
		Thinking: "think",
		Calls:    []llm.ToolCall{{ID: "c1", Name: "read", Input: json.RawMessage(`{"path":"a"}`)}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, SSE(turn))
	}))
	defer srv.Close()

	t.Setenv("MOCA_T_KEY", "k")
	cfg, err := config.Parse([]byte(fmt.Sprintf(
		`{"model":"replay/m","providers":{"replay":{"baseUrl":%q,"protocol":"openai-completions",`+
			`"auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":100000}}}}}`,
		srv.URL+"/v1")))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := provider.NewRegistry(cfg, srv.Client(), func(provider.RetryNotice) {})
	if err != nil {
		t.Fatal(err)
	}
	_, ad, err := reg.Resolve("replay/m")
	if err != nil {
		t.Fatal(err)
	}
	req := llm.Request{Model: "m", MaxTokens: 100,
		Messages: []llm.Message{{Role: llm.RoleUser,
			Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}}}
	resp, err := ad.Stream(context.Background(), req, func(llm.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if got := llm.TextOf(resp.Message); got != "hello" {
		t.Fatalf("text = %q", got)
	}
	var thinking string
	var calls []llm.ToolCall
	for _, b := range resp.Message.Content {
		switch b.Type {
		case llm.BlockThinking:
			thinking += b.Text
		case llm.BlockToolUse:
			calls = append(calls, *b.ToolCall)
		}
	}
	if thinking != "think" || len(calls) != 1 || calls[0].Name != "read" ||
		string(calls[0].Input) != `{"path":"a"}` {
		t.Fatalf("thinking=%q calls=%+v", thinking, calls)
	}
}

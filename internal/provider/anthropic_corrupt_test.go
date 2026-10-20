package provider

// Pin for F-3: the anthropic assembler indexed a map and a slice with the
// stream's own content-block index and dereferenced a per-block builder it only
// creates for tool_use. A malformed or hostile stream therefore panicked the
// run — a crashed process, not an error the retry loop can classify.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func anthropicStream(t *testing.T, events string) (llm.Response, error) {
	t.Helper()
	srv := sseServer(t, 200, events, nil, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "claude-x", ContextWindow: 200_000},
		srv.URL, func(context.Context, llm.Request) (Credential, error) { return Credential{Token: "K"}, nil }, srv.Client())
	req := llm.Request{Model: "claude-x", MaxTokens: 32, Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}}}
	return a.Stream(context.Background(), req, func(llm.Event) {})
}

func TestAnthropicCorruptStreamIsAnError(t *testing.T) {
	// An input_json_delta addressed to a text block: no builder exists for it.
	text := "event: content_block_start\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":"}}` + "\n\n"
	if _, err := anthropicStream(t, text); err == nil || !strings.Contains(err.Error(), "corruption") {
		t.Fatalf("want a corruption error, got %v", err)
	}

	// A negative index addresses no block at all.
	neg := "event: content_block_start\n" +
		`data: {"type":"content_block_start","index":-1,"content_block":{"type":"text","text":""}}` + "\n\n"
	if _, err := anthropicStream(t, neg); err == nil || !strings.Contains(err.Error(), "corruption") {
		t.Fatalf("want a corruption error for a negative index, got %v", err)
	}

	// Control: a well-formed tool_use still assembles.
	ok := "event: content_block_start\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"read"}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"x\"}"}}` + "\n\n" +
		"event: content_block_stop\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n" +
		"event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"
	res, err := anthropicStream(t, ok)
	if err != nil {
		t.Fatalf("well-formed stream failed: %v", err)
	}
	if len(res.Message.Content) != 1 || res.Message.Content[0].ToolCall == nil ||
		string(res.Message.Content[0].ToolCall.Input) != `{"path":"x"}` {
		t.Fatalf("assembled %s", fmt.Sprint(res.Message.Content))
	}
}

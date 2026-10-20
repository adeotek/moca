package provider

// The DeepSeek thinking-mode round trip: a replayed assistant turn must carry
// back the reasoning_content it produced, or the API answers 400 with "The
// `reasoning_content` in the thinking mode must be passed back to the API"
// and the run stops. The completions adapter used to drop every thinking
// block, which is exactly that request.

import (
	"encoding/json"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func deepseekModel() Model {
	for _, m := range builtinCatalog {
		if m.Qualified() == "opencode-go/deepseek-v4.1-flash" {
			return m
		}
	}
	return Model{}
}

func glmModel() Model {
	for _, m := range builtinCatalog {
		if m.Qualified() == "opencode-go/glm-5.3-flash" {
			return m
		}
	}
	return Model{}
}

// assistantMsg is one assistant turn carrying a thinking block plus text.
func assistantMsg(model, thinking, text string) llm.Message {
	blocks := []llm.ContentBlock{}
	if thinking != "" {
		blocks = append(blocks, llm.ContentBlock{Type: llm.BlockThinking, Text: thinking, Model: model, Signature: "sig"})
	}
	if text != "" {
		blocks = append(blocks, llm.ContentBlock{Type: llm.BlockText, Text: text})
	}
	return llm.Message{Role: llm.RoleAssistant, Content: blocks}
}

func completionsMsgs(t *testing.T, m Model, msgs []llm.Message) []map[string]any {
	t.Helper()
	a := &completionsAdapter{m: m}
	req := llm.Request{Model: m.ID, Messages: msgs}
	b, err := json.Marshal(a.body(req)["messages"])
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCompletionsReplaysReasoningContent(t *testing.T) {
	m := deepseekModel()
	if !m.ReplayReasoning {
		t.Fatalf("%s must replay reasoning_content (DeepSeek's thinking mode requires it)", m.Qualified())
	}
	msgs := completionsMsgs(t, m, []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "fix it"}}},
		assistantMsg(m.ID, "the test fails because X", "looking"),
	})
	if got, _ := msgs[1]["reasoning_content"].(string); got != "the test fails because X" {
		t.Fatalf("assistant turn lost its reasoning_content: %v", msgs[1])
	}
}

func TestCompletionsDropsForeignReasoning(t *testing.T) {
	m := deepseekModel()
	// Reasoning produced by another model (a /model switch mid-session) is not
	// this route's to replay, and a redacted block has no text to send.
	msgs := completionsMsgs(t, m, []llm.Message{
		assistantMsg("opencode-go/glm-5.3-flash", "another model's thoughts", "hi"),
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "secret", Model: m.ID, Redacted: true},
			{Type: llm.BlockText, Text: "hi"},
		}},
	})
	for i, msg := range msgs {
		if _, ok := msg["reasoning_content"]; ok {
			t.Fatalf("message %d replayed foreign or redacted reasoning: %v", i, msg)
		}
	}
}

func TestCompletionsDoesNotSendReasoningWhereUnwanted(t *testing.T) {
	m := glmModel()
	if m.ReplayReasoning {
		t.Fatalf("%s must not echo reasoning_content — its route never asked for it", m.Qualified())
	}
	msgs := completionsMsgs(t, m, []llm.Message{
		assistantMsg(m.ID, "thoughts", "hi"),
	})
	if _, ok := msgs[0]["reasoning_content"]; ok {
		t.Fatalf("reasoning_content sent to a route that does not take it: %v", msgs[0])
	}
}

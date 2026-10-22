package agent

// The reasoning-required route (DeepSeek's thinking mode) has two wire
// contracts, both proven live in §15 and both broken by the same mistake —
// comparing a persisted thinking block against the *bare* model id:
//
//  1. an assistant turn replayed in a later request must carry the
//     reasoning_content it produced, or the API answers
//     `400 The reasoning_content in the thinking mode must be passed back to
//     the API` and the run dies on the second turn;
//  2. after a compaction the kept tail must not be stripped of that thinking —
//     it is the same history the API demands back.
//
// These tests drive the agent, not the adapter: the bug lived in the seam
// between them (the adapter compared c.Model to req.Model, the bare id, while
// the agent persists the qualified one).

import (
	"context"
	"fmt"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// reasoningModel is a builtin catalog row that replays reasoning: the label
// and the wire shape matter, the upstream does not (the script server answers).
const reasoningModel = "opencode-go/deepseek-v4.1-flash"

// reasoningTurn is one assistant turn that thinks, then calls a tool.
func reasoningTurn(think string, calls ...[2]string) string {
	out := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"reasoning_content\":%q}}]}\n\n", think)
	return out + toolTurn(calls...)
}

// startReasoning drives an agent on a builtin ReplayReasoning row: the model
// keeps its catalog entry (so ReplayReasoning is on) and only the base URL
// points at the script server.
func startReasoning(t *testing.T, s *scriptServer) *Agent {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	cfg, err := config.Parse([]byte(fmt.Sprintf(
		`{"model":%q,"providers":{"opencode-go":{"baseUrl":%q,"apiKey":"env:MOCA_T_KEY"}}}`,
		reasoningModel, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	a, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), Mode: ModeYolo,
		HTTP: s.srv.Client(), Slug: "r"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRequestReplaysReasoningContent(t *testing.T) {
	const think = "the file must be listed first"
	s := newScript(t, reasoningTurn(think, [2]string{"ls", `{}`}), textTurn("done"))
	a := startReasoning(t, s)
	if _, err := a.Run(context.Background(), "list the workdir"); err != nil {
		t.Fatal(err)
	}
	if got := s.bodyCount(); got < 2 {
		t.Fatalf("expected a second request, got %d", got)
	}
	msgs := s.bodies[1]["messages"].([]any)
	var assistant map[string]any
	for _, m := range msgs {
		if mm := m.(map[string]any); mm["role"] == "assistant" {
			assistant = mm
		}
	}
	if assistant == nil {
		t.Fatalf("no assistant turn replayed: %v", msgs)
	}
	if got, _ := assistant["reasoning_content"].(string); got != think {
		t.Fatalf("the replayed assistant turn lost its reasoning_content (v0.7.2's echo compared the block against the bare model id): %v", assistant)
	}
	// The transcript is the other half of the contract: what the agent persists
	// is stamped with the qualified id, which is what the adapter compares to.
	entries, err := session.ReadFile(a.Session().Path())
	if err != nil {
		t.Fatal(err)
	}
	var stamped string
	for _, e := range entries {
		if e.Type != session.TypeMessage || e.Message == nil {
			continue
		}
		for _, c := range e.Message.Content {
			if c.Type == llm.BlockThinking {
				stamped = c.Model
			}
		}
	}
	if stamped != reasoningModel {
		t.Fatalf("persisted thinking block model = %q, want the qualified id %q", stamped, reasoningModel)
	}
}

// TestKeptTailThinkingFollowsTheRoute: a compaction on a reasoning-required
// route keeps the kept tail's thinking blocks; a route that does not need them
// still gets the strip (the token saving is the point there).
func TestKeptTailThinkingFollowsTheRoute(t *testing.T) {
	for _, tc := range []struct {
		model string
		keep  bool
	}{
		{reasoningModel, true},
		{"opencode-go/glm-5.3-flash", false},
	} {
		t.Run(tc.model, func(t *testing.T) {
			s := newScript(t)
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("MOCA_T_KEY", "k")
			cfg, err := config.Parse([]byte(fmt.Sprintf(
				`{"model":%q,"providers":{"opencode-go":{"baseUrl":%q,"apiKey":"env:MOCA_T_KEY"}}}`,
				tc.model, s.srv.URL)))
			if err != nil {
				t.Fatal(err)
			}
			a, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), Mode: ModeYolo,
				HTTP: s.srv.Client(), Slug: "c"})
			if err != nil {
				t.Fatal(err)
			}
			// A transcript with a compaction: user, assistant (thinking + call),
			// tool result, summary, user. The assistant turn is in the kept
			// tail (before the compaction entry).
			a.mu.Lock()
			a.entries = append(a.entries,
				session.Entry{ID: "e1", Type: session.TypeMessage, Message: userText("go")},
				session.Entry{ID: "e2", Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant,
					Content: []llm.ContentBlock{
						{Type: llm.BlockThinking, Text: "reasoned", Model: a.model.Qualified(), Signature: "sig"},
						{Type: llm.BlockText, Text: "listing"},
						{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "call_1", Name: "ls", Input: []byte(`{}`)}},
					}}},
				session.Entry{ID: "e3", Type: session.TypeToolResult, ToolResult: &llm.ToolResult{CallID: "call_1", Content: "a.txt"}},
				session.Entry{ID: "e4", Type: session.TypeCompaction, Compaction: &session.Compaction{
					FirstKeptEntryID: "e1", Summary: "listing the workdir"}},
				session.Entry{ID: "e5", Type: session.TypeMessage, Message: userText("and now?")},
			)
			a.mu.Unlock()

			var kept bool
			for _, m := range a.request(llm.ToolChoiceAuto).Messages {
				for _, c := range m.Content {
					if c.Type == llm.BlockThinking {
						kept = true
					}
				}
			}
			if kept != tc.keep {
				t.Fatalf("kept tail thinking kept = %v, want %v", kept, tc.keep)
			}
		})
	}
}

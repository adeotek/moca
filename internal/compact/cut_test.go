package compact

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func u(id, text string) Entry {
	return Entry{ID: id, Kind: KindUser, Msg: &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: text}}}}
}
func a(id, text string) Entry {
	return Entry{ID: id, Kind: KindAssistant, Msg: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: text}}}}
}
func call(id, name string) Entry {
	return Entry{ID: id, Kind: KindToolUse, Call: &llm.ToolCall{ID: id, Name: name, Input: json.RawMessage(`{}`)}}
}
func res(id, body string) Entry {
	return Entry{ID: "r" + id, Kind: KindToolResult, Result: &llm.ToolResult{CallID: id, Content: body}}
}

var k = strings.Repeat("x", 4000) // 1000 tokens

func TestFindCutKeepsRecentAtValidPoint(t *testing.T) {
	es := []Entry{u("u1", k), a("a1", k), call("c1", "read"), res("c1", k), a("a2", k), u("u2", k), a("a3", k)}
	cut, ts, ok := FindCut(es, 2500)
	if !ok || es[cut].ID != "u2" || ts != -1 {
		t.Fatalf("cut %d (%s) ts %d", cut, es[cut].ID, ts)
	}
}

func TestFindCutNeverBetweenCallAndResult(t *testing.T) {
	es := []Entry{u("u1", k), a("a1", k), call("c1", "read"), res("c1", strings.Repeat("y", 40000)), a("a2", "done")}
	cut, _, ok := FindCut(es, 1500)
	if !ok || es[cut].ID != "a2" {
		t.Fatalf("only a2 qualifies; got %s", es[cut].ID)
	}
	for i := range es {
		if es[i].Kind == KindToolResult && cut == i {
			t.Fatal("cut at a tool result")
		}
	}
}

func TestFindCutSplitTurn(t *testing.T) {
	es := []Entry{u("u0", k), a("a0", k), u("u1", "do it"), a("a1", k), call("c1", "ls"), res("c1", k), a("a2", k), call("c2", "ls"), res("c2", k), a("a3", "x")}
	cut, ts, ok := FindCut(es, 2200)
	if !ok || es[cut].Kind != KindAssistant || es[ts].ID != "u1" {
		t.Fatalf("cut %s ts %d", es[cut].ID, ts)
	}
}

func TestFindCutNothingToDo(t *testing.T) {
	if _, _, ok := FindCut([]Entry{u("u1", "hi")}, 100); ok {
		t.Fatal("single entry")
	}
	if _, _, ok := FindCut([]Entry{u("u1", "hi"), call("c", "ls"), res("c", "x")}, 100); ok {
		t.Fatal("no valid cut after index 0")
	}
	// The whole window fits keepRecent: everything is recent, there is
	// nothing older to summarize (a /compact on a new session is a no-op).
	if _, _, ok := FindCut([]Entry{u("u1", "hi"), a("a1", "yo")}, 100); ok {
		t.Fatal("whole window within keepRecent")
	}
}

package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func mustJSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestResumeRebuildsExactContext(t *testing.T) {
	s := newScript(t, textTurn("first answer"), textTurn("second answer"))
	a, work, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), "first question")
	a.SetEffort(llm.EffortLow)
	path := a.Session().Path()
	sys := a.opts.System
	a.Session().Close()

	b, err := Resume(StartOptions{Config: a.opts.Config, Workdir: "/ignored", HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	if b.opts.System != sys || b.Effort() != a.Effort() || b.opts.Env.Root != a.opts.Env.Root {
		t.Fatal("system prompt verbatim, effort restored, workdir from header")
	}
	if b.Workdir() != a.Workdir() {
		t.Fatal("workdir from header, not the resuming process")
	}
	b.Run(context.Background(), "second question")
	msgs := s.bodies[1]["messages"].([]any)
	if msgs[0].(map[string]any)["content"] != sys || len(msgs) != 4 {
		t.Fatalf("resumed request = stored system + full history + new prompt: %v", msgs)
	}
	_ = work
}

func TestResumeAfterCrashMidTool(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	// simulate a crash: assistant message + tool_use persisted, no result
	m, _ := a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant}})
	a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: m.ID, Call: llm.ToolCall{ID: "c1", Name: "shell"}}})
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := session.ReadFile(b.Session().Path())
	last := entries[len(entries)-1]
	if last.ToolResult == nil || last.ToolResult.Content != session.Interrupted {
		t.Fatalf("%+v", last)
	}
}

func TestResumeNeverRestoresYolo(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	a.SetYolo(true)
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err != nil || b.Yolo() {
		t.Fatal("resume starts with checks on unless the resuming process asks for yolo", err)
	}
	b.Session().Close()
	c, _ := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client(), Yolo: true}, path)
	if !c.Yolo() {
		t.Fatal("--yolo on resume applies")
	}
	c.Session().Close()
}

func TestResumeHonoursCompaction(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTestWith(t, s, "", "")
	u1, _ := a.append(session.Entry{Type: session.TypeMessage, Message: userText("old")})
	a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "old answer"}}}})
	u2, _ := a.append(session.Entry{Type: session.TypeMessage, Message: userText("kept")})
	a.append(session.Entry{Type: session.TypeCompaction, Compaction: &session.Compaction{Summary: "SUMMARY", FirstKeptEntryID: u2.ID}})
	_ = u1
	path := a.Session().Path()
	a.Session().Close()
	b, _ := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	b.Run(context.Background(), "new")
	body := mustJSONString(s.bodies[0])
	if !strings.Contains(body, "SUMMARY") || strings.Contains(body, "old answer") {
		t.Fatal(body)
	}
}

func TestResumeNotASession(t *testing.T) {
	if _, err := Resume(StartOptions{}, t.TempDir()+"/nope.jsonl"); err == nil {
		t.Fatal("missing file must fail")
	}
}

// A session whose workdir no longer exists must fail with a clear message
// and must not append repair entries for a resume that never became usable.
func TestResumeMissingWorkdir(t *testing.T) {
	s := newScript(t)
	a, work, _ := startTestWith(t, s, "", "")
	// Make it a crash: an unanswered tool_use that a resume would repair.
	m, _ := a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant}})
	a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: m.ID, Call: llm.ToolCall{ID: "c1", Name: "shell"}}})
	path := a.Session().Path()
	a.Session().Close()
	before, _ := session.ReadFile(path)
	if err := os.RemoveAll(work); err != nil {
		t.Fatal(err)
	}
	_, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err == nil || !strings.Contains(err.Error(), "workdir") || !strings.Contains(err.Error(), work) {
		t.Fatalf("%v", err)
	}
	after, _ := session.ReadFile(path)
	if len(after) != len(before) {
		t.Fatalf("no repair entries may be appended: before=%d after=%d", len(before), len(after))
	}
}

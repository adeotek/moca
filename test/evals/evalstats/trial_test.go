package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// trial builds a transcript the way a two-turn run leaves one.
func trial(t *testing.T, turns ...[2]string) string {
	t.Helper()
	dir := t.TempDir()
	w, err := session.Create(dir, session.Header{Workdir: "/tmp", StartedAt: time.Now()}, "trial")
	if err != nil {
		t.Fatal(err)
	}
	for _, turn := range turns {
		if _, err := w.Append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{
			Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: turn[0]}},
		}}); err != nil {
			t.Fatal(err)
		}
		if turn[1] == "" {
			continue // no answer: the "missing first answer" shape
		}
		if _, err := w.Append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{
			Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: turn[1]}},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.Path()
}

// A headless run that answers without touching a tool earns moca's action
// nudge (§14). It is a user message in the transcript but not a prompt the
// harness sent, so it must not read as a third turn -- while its own kind
// still marks it as a nudge for attribution.
func TestCheckTrialSessionIgnoresMocaNudges(t *testing.T) {
	dir := t.TempDir()
	w, err := session.Create(dir, session.Header{Workdir: "/tmp", StartedAt: time.Now()}, "nudged")
	if err != nil {
		t.Fatal(err)
	}
	appendMsg := func(role llm.Role, text, nudge string) {
		t.Helper()
		if _, err := w.Append(session.Entry{Type: session.TypeMessage, Nudge: nudge, Message: &llm.Message{
			Role: role, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: text}},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	appendMsg(llm.RoleUser, "first", "")
	appendMsg(llm.RoleAssistant, "answer one", "")
	appendMsg(llm.RoleUser, "follow up", "")
	appendMsg(llm.RoleUser, "Your reply would end this run without a single tool call. ...", "action")
	appendMsg(llm.RoleAssistant, "answer two", "")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := CheckTrialSession(w.Path(), "first", "follow up"); err != nil {
		t.Fatalf("a nudged but correct trial was refused: %v", err)
	}
}

func TestCheckTrialSessionAcceptsATwoTurnTrial(t *testing.T) {
	path := trial(t, [2]string{"first", "answer one"}, [2]string{"follow up", "answer two"})
	if err := CheckTrialSession(path, "first", "follow up"); err != nil {
		t.Fatalf("a correct two-turn transcript was refused: %v", err)
	}
}

// A resumed run's second prompt lands in the same file. Two prompts in one
// session that only carries one answer, or a prompt order that does not match
// the trial's prompts, is a harness bug that would otherwise read as a pass.
func TestCheckTrialSessionRefusals(t *testing.T) {
	cases := []struct {
		name string
		path func() string
		p1   string
		p2   string
		want string
	}{
		{"missing first answer", func() string {
			return trial(t, [2]string{"first", ""}, [2]string{"follow up", "answer two"})
		}, "first", "follow up", "missing first answer"},
		{"wrong turn boundary", func() string {
			return trial(t, [2]string{"first", "answer one"}, [2]string{"follow up", "answer two"})
		}, "first", "second", "wrong turn boundary"},
		{"wrong first prompt", func() string {
			return trial(t, [2]string{"something else", "answer one"}, [2]string{"follow up", "answer two"})
		}, "first", "follow up", "wrong turn boundary"},
		{"one turn only", func() string {
			return trial(t, [2]string{"first", "answer one"})
		}, "first", "follow up", "found 1 user prompts"},
		{"no answer after the follow-up", func() string {
			return trial(t, [2]string{"first", "answer one"}, [2]string{"follow up", ""})
		}, "first", "follow up", "turn 2 has no answer"},
		{"not a transcript", func() string {
			p := filepath.Join(t.TempDir(), "empty.jsonl")
			if err := os.WriteFile(p, []byte(""), 0o600); err != nil {
				t.Fatal(err)
			}
			return p
		}, "first", "follow up", "session header"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckTrialSession(tc.path(), tc.p1, tc.p2)
			if err == nil {
				t.Fatalf("%s: the transcript was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: error = %q, want it to mention %q", tc.name, err, tc.want)
			}
		})
	}
}

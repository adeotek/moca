package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// seedSession stores one session for workdir (with a preview message when
// text is non-empty) and returns its path.
func seedSession(t *testing.T, workdir, text string) string {
	t.Helper()
	time.Sleep(20 * time.Millisecond) // distinct mtimes, like TestFind
	canon, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := session.Create(sessionsDir(), session.Header{Workdir: canon, Provider: "fake", Model: "fake/m",
		Effort: "medium", SystemPrompt: "you are a test"}, "t")
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		msg := llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: text}}}
		if _, err := w.Append(session.Entry{Type: session.TypeMessage, Message: &msg}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.Path()
}

func openSessionPicker(t *testing.T, m *model) {
	t.Helper()
	if cmd := m.runCommand(Parsed{Kind: KindCommand, Name: "sessions"}); cmd != nil {
		settle(m)
	}
	if m.pick == nil {
		t.Fatal("the sessions picker did not open")
	}
}

func pickRow(t *testing.T, m *model, path string) {
	t.Helper()
	for i, it := range m.pick.items {
		if it.key == path {
			m.pick.cursor = i
			return
		}
	}
	t.Fatalf("row %s not in the picker", path)
}

func TestSessionsPickerListsAll(t *testing.T) {
	m := newAgentModel(t)
	curPath := m.agent.Session().Path()
	seedSession(t, m.opts.Start.Workdir, "fix the parser")
	seedSession(t, m.opts.Start.Workdir, "")
	openSessionPicker(t, m)
	if len(m.pick.items) != 3 {
		t.Fatalf("items: %d", len(m.pick.items))
	}
	if last := m.pick.items[len(m.pick.items)-1]; last.key != curPath {
		t.Fatalf("the current session (oldest) should be listed last, got %s", last.key)
	}
	var joined strings.Builder
	for _, it := range m.pick.items {
		joined.WriteString(it.label + "\n")
	}
	for _, want := range []string{m.agent.Session().ID8() + " (current)", "fix the parser", "(no messages)"} {
		if !strings.Contains(joined.String(), want) {
			t.Fatalf("labels missing %q:\n%s", want, joined.String())
		}
	}
	panel := m.pickerPanel()
	if !strings.Contains(panel, "enter switch") || !strings.Contains(panel, "ctrl+d delete") {
		t.Fatalf("panel: %q", panel)
	}
}

func TestSessionsEnterSwitches(t *testing.T) {
	m := newAgentModel(t)
	other := seedSession(t, m.opts.Start.Workdir, "earlier work")
	openSessionPicker(t, m)
	pickRow(t, m, other)
	if cmd := m.pickKey(keyMsg("enter")); cmd != nil {
		simulate(m, cmd)
	}
	if m.pick != nil {
		t.Fatal("the picker should close on enter")
	}
	if got := m.agent.Session().ID8(); got != sessionIDOf(other) {
		t.Fatalf("current session %s, want %s", got, sessionIDOf(other))
	}
}

func TestSessionsDeleteFlow(t *testing.T) {
	m := newAgentModel(t)
	other := seedSession(t, m.opts.Start.Workdir, "to be deleted")
	otherID := sessionIDOf(other)
	openSessionPicker(t, m)
	pickRow(t, m, other)

	if cmd := m.pickKey(keyMsg("ctrl+d")); cmd != nil {
		t.Fatal("ctrl+d must only start the confirmation")
	}
	if !m.pick.confirm {
		t.Fatal("no confirmation shown")
	}
	panel := m.pickerPanel()
	if !strings.Contains(panel, "delete session "+otherID+"?") || !strings.Contains(panel, "[y] delete") {
		t.Fatalf("confirm panel: %q", panel)
	}
	m.pickKey(keyMsg("n"))
	if m.pick.confirm {
		t.Fatal("n must back out of the confirmation")
	}
	if cmd := m.pickKey(keyMsg("ctrl+d")); cmd != nil || !m.pick.confirm {
		t.Fatal("ctrl+d must re-arm the confirmation")
	}

	out := printed(m.pickKey(keyMsg("y")))
	settle(m)
	if !strings.Contains(out, "deleted session "+otherID) {
		t.Fatalf("out: %q", out)
	}
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Fatal("the session file should be gone")
	}
	if m.pick == nil {
		t.Fatal("the picker should stay open after a delete")
	}
	for _, it := range m.pick.items {
		if it.key == other {
			t.Fatal("the deleted row is still listed")
		}
	}
}

func TestSessionsCurrentCannotBeDeleted(t *testing.T) {
	m := newAgentModel(t)
	curPath := m.agent.Session().Path()
	openSessionPicker(t, m)
	pickRow(t, m, curPath)
	m.pickKey(keyMsg("ctrl+d"))
	out := printed(m.pickKey(keyMsg("y")))
	settle(m)
	if !strings.Contains(out, "cannot be deleted") {
		t.Fatalf("out: %q", out)
	}
	if _, err := os.Stat(curPath); err != nil {
		t.Fatalf("the current session file must survive: %v", err)
	}
}

func TestSessionsDeleteRefusedForOpenSession(t *testing.T) {
	m := newAgentModel(t)
	// A session another process still holds open: keep the writer locked.
	canon, err := filepath.EvalSymlinks(m.opts.Start.Workdir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := session.Create(sessionsDir(), session.Header{Workdir: canon}, "t")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	msg := llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "held elsewhere"}}}
	if _, err := w.Append(session.Entry{Type: session.TypeMessage, Message: &msg}); err != nil {
		t.Fatal(err)
	}
	openSessionPicker(t, m)
	pickRow(t, m, w.Path())
	m.pickKey(keyMsg("ctrl+d"))
	out := printed(m.pickKey(keyMsg("y")))
	settle(m)
	if !strings.Contains(out, "open in another moca process") {
		t.Fatalf("out: %q", out)
	}
	if _, err := os.Stat(w.Path()); err != nil {
		t.Fatalf("the held session must survive: %v", err)
	}
}

func TestSessionsRefusedWhileRunning(t *testing.T) {
	m := newAgentModel(t)
	m.running = true
	out := printed(m.runCommand(Parsed{Kind: KindCommand, Name: "sessions"}))
	if !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("out: %q", out)
	}
	if m.pick != nil {
		t.Fatal("no picker while running")
	}
}

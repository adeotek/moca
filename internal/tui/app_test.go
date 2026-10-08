package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// newTestModel builds a model without an agent (agent-dependent commands are
// tested in agent's own tests).
func newTestModel() *model {
	m := newModel(AppOptions{Home: "/home/u"}, nil)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// keyMsg builds the message a terminal would deliver for a keystroke string,
// so tests can drive Update with a key("...") helper.
func keyMsg(s string) tea.KeyPressMsg {
	k := tea.Key{}
	body := s
	switch {
	case strings.HasPrefix(s, "shift+"):
		k.Mod, body = tea.ModShift, s[len("shift+"):]
	case strings.HasPrefix(s, "alt+"):
		k.Mod, body = tea.ModAlt, s[len("alt+"):]
	case strings.HasPrefix(s, "ctrl+"):
		k.Mod, body = tea.ModCtrl, s[len("ctrl+"):]
	}
	switch body {
	case "enter":
		k.Code = tea.KeyEnter
	case "esc":
		k.Code = tea.KeyEscape
	case "up":
		k.Code = tea.KeyUp
	case "down":
		k.Code = tea.KeyDown
	case "tab":
		k.Code = tea.KeyTab
	default:
		r := []rune(body)
		k.Code = r[len(r)-1]
		if k.Mod == 0 {
			k.Text = body // printable keys carry their text
		}
	}
	return tea.KeyPressMsg(k)
}

func key(s string) tea.Msg { return keyMsg(s) }

func TestStreamingCommitsCompletedLines(t *testing.T) {
	m := newTestModel()
	m.running = true
	_, cmd := m.Update(agentEventMsg{agent.TextDelta{Text: "line one\nline t"}})
	if cmd == nil || !strings.Contains(m.live.String(), "line t") || strings.Contains(m.live.String(), "line one") {
		t.Fatalf("completed line printed, partial kept live: %q", m.live.String())
	}
}

func TestApprovalPreservesDraft(t *testing.T) {
	m := newTestModel()
	m.input.Insert("half-typed")
	m.syncTextarea()
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python", CanAlways: true}, reply: reply})
	m.Update(key("a"))
	if got := <-reply; got != tools.AllowOnce {
		t.Fatal(got)
	}
	if m.input.Text() != "half-typed" {
		t.Fatal("draft untouched by approval keys")
	}
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "rm"}, reply: reply})
	m.Update(key("ctrl+a"))
	select {
	case <-reply:
		t.Fatal("ctrl+a is not offered for ask-every-time commands")
	default:
	}
	m.Update(key("esc"))
	if got := <-reply; got != tools.Deny {
		t.Fatal("esc on prompt = deny")
	}
}

func TestThinkingCollapsedOnTurnEnd(t *testing.T) {
	m := newTestModel()
	m.Update(agentEventMsg{agent.ThinkingDelta{Text: "secret\nreasoning"}})
	if strings.Contains(m.View().Content, "secret") {
		t.Fatal("thinking never shown expanded outside the pager")
	}
	m.Update(agentEventMsg{agent.TurnEnd{Message: llm.Message{}}})
	it, ok := m.items.Last()
	if !ok || it.Kind != "thinking" || it.Line != "⋯ #1 thinking 2 lines" {
		t.Fatal(it)
	}
}

func TestPasteDoesNotSend(t *testing.T) {
	m := newTestModel()
	m.Update(tea.PasteMsg{Content: strings.Repeat("x\n", 80)})
	if m.running || m.input.Display() != "[paste 80 lines #1]" {
		t.Fatal(m.input.Display())
	}
}

// A run that dies mid-retry must not leave the retry transient in the bar.
func TestRunDoneClearsTransient(t *testing.T) {
	m := newTestModel()
	m.status.Transient = "retry 5/5 · 16s"
	m.handleRunDone(runDoneMsg{err: context.Canceled})
	if m.status.Transient != "" {
		t.Fatal("transient stuck after a failed run")
	}
}

// The approval prompt is model-supplied text: it must be sanitized like every
// other untrusted surface, and only the first detail line is shown.
func TestApprovalPromptSanitized(t *testing.T) {
	got := approvalPrompt(tools.Question{Subject: "seq", Detail: "seq 'x\x1b[2Jy'\nsecond line"})
	if strings.ContainsRune(got, 0x1b) || !strings.Contains(got, "^[[2J") {
		t.Fatalf("unescaped control byte in prompt: %q", got)
	}
	if strings.Contains(got, "second line") {
		t.Fatal("only the first detail line is shown")
	}
}

// The scrollback opens with the title + version and the greeting.
func TestWelcomeLines(t *testing.T) {
	w := welcomeText()
	for _, want := range []string{"moca", config.Version, "How can I help you today?"} {
		if !strings.Contains(w, want) {
			t.Fatalf("welcome %q missing %q", w, want)
		}
	}
}

// /exit and its /q, /quit aliases quit (same path as ctrl+c twice). The
// submission is an echo-print-then-quit sequence, so drive it.
func TestExitCommandQuits(t *testing.T) {
	m := newAgentModel(t)
	for _, in := range []string{"/exit", "/q", "/quit"} {
		m.input.Insert(in)
		m.syncTextarea()
		cmd := m.submit()
		if cmd == nil {
			t.Fatalf("%s: no command", in)
		}
		if _, quit := simulate(m, cmd); !quit {
			t.Fatalf("%s: the submission must quit", in)
		}
	}
}

// The input area renders without a prompt and is separated from the output
// and the status bar by two full-width rules.
func TestInputRulesAndNoPrefix(t *testing.T) {
	m := newTestModel()
	v := m.View().Content
	if m.ta.Prompt != "" {
		t.Fatalf("input prompt %q", m.ta.Prompt)
	}
	if strings.Contains(v, "›") {
		t.Fatalf("input prefix leaked into the view:\n%s", v)
	}
	if rule := strings.Repeat("─", 120); strings.Count(v, rule) != 2 {
		t.Fatalf("want two full-width rules:\n%s", v)
	}
}

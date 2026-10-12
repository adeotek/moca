package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/tools"
)

// Until they answer, an approval question is pending: sel at the default
// (Allow once) and no reply sent.
func approvalPending(m *model, reply chan tools.Answer) (ok bool) {
	if m.approval != nil && m.approval.sel == 0 {
		select {
		case <-reply:
		default:
			return true
		}
	}
	return false
}

func TestApprovalPickerArrowsEnter(t *testing.T) {
	m := newTestModel()
	fixed := time.Now()
	m.now = func() time.Time { return fixed }
	m.lastTyped = fixed.Add(-2 * time.Second) // past approvalIdle
	reply := make(chan tools.Answer, 1)
	m.approval = &approvalState{approvalMsg: approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply}}
	m.Update(key("down"))
	m.Update(key("down")) // two downs: once → always → deny
	if m.approval.sel != 2 {
		t.Fatalf("two downs must land on deny: sel=%d", m.approval.sel)
	}
	m.Update(key("enter"))
	select {
	case got := <-reply:
		if got != tools.Deny {
			t.Fatalf("down twice + enter must deny: %v", got)
		}
	default:
		t.Fatal("enter did not answer the prompt")
	}
}

func TestApprovalPickerAlwaysPersists(t *testing.T) {
	m := newAgentModel(t)
	fixed := time.Now()
	m.now = func() time.Time { return fixed }
	m.lastTyped = fixed.Add(-2 * time.Second)
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	m.Update(key("down")) // "Allow always"
	_, cmd := m.Update(key("enter"))
	select {
	case got := <-reply:
		if got != tools.AllowAlways {
			t.Fatalf("picker enter must answer through the ctrl+a path: %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("enter did not answer the prompt")
	}
	if !strings.Contains(printed(cmd), "always allowing") {
		t.Fatalf("confirmation line: %q", printed(cmd))
	}
	if b, _ := os.ReadFile(m.opts.ConfigPath); !strings.Contains(string(b), `"python3"`) {
		t.Fatalf("picker enter must persist like ctrl+a: %s", b)
	}
}

func TestApprovalPickerNoAlwaysRow(t *testing.T) {
	// CanAlways=false: [once, deny] — down from once lands on deny; up from a
	// fresh prompt wraps back to deny as well.
	forward, back := false, false
	for _, down := range []bool{true, false} {
		m := newTestModel()
		fixed := time.Now()
		m.now = func() time.Time { return fixed }
		m.lastTyped = fixed.Add(-2 * time.Second)
		reply := make(chan tools.Answer, 1)
		m.approval = &approvalState{approvalMsg: approvalMsg{q: tools.Question{Kind: "shell", Subject: "x"}, reply: reply}}
		k := "up"
		if down {
			k = "down"
		}
		m.Update(key(k))
		if m.approval.sel != 1 {
			t.Fatalf("wrap-around: sel=%d after %s", m.approval.sel, k)
		}
		m.Update(key("enter"))
		select {
		case got := <-reply:
			if got != tools.Deny {
				t.Fatalf("%s+enter must deny: %v", k, got)
			}
			if down {
				forward = true
			} else {
				back = true
			}
		case <-time.After(2 * time.Second):
			t.Fatal("enter did not answer the prompt")
		}
	}
	if !forward || !back {
		t.Fatalf("not all subtests ran: forward=%v back=%v", forward, back)
	}
}

func TestApprovalPickerYieldsToDraft(t *testing.T) {
	m := newTestModel()
	fixed := time.Now()
	m.now = func() time.Time { return fixed }
	m.lastTyped = fixed.Add(-2 * time.Second)
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	m.input.SetBuffer("hello")
	m.syncTextarea()
	m.Update(key("up"))
	m.Update(key("down"))
	if m.approval.sel != 0 {
		t.Fatalf("arrows moved the selection during a draft: sel=%d", m.approval.sel)
	}
	m.Update(key("enter"))
	// With a non-empty draft, enter keeps its current meaning: it routes to
	// submit() (the draft is cleared; the panel's pending question stays
	// after it), and the approval is NOT answered.
	if m.approval == nil {
		t.Fatal("draft enter answered the approval; it must stay pending")
	}
	select {
	case a := <-reply:
		t.Fatalf("draft enter answered the prompt: %v", a)
	default:
	}
	if m.input.Buffer() != "" {
		t.Fatalf("draft enter must clear the draft: %q", m.input.Buffer())
	}
}

func TestApprovalPickerIdleGuard(t *testing.T) {
	// Empty draft but typed 100 ms ago: enter is ignored (same guard as a/d).
	m := newTestModel()
	fixed := time.Now()
	m.now = func() time.Time { return fixed } // lastTyped is exactly 100 ms before "now"
	m.lastTyped = fixed.Add(-100 * time.Millisecond)
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python3", CanAlways: true}, reply: reply})
	m.Update(key("enter"))
	if !approvalPending(m, reply) {
		t.Fatal("a fresh enter must not answer a prompt that popped up under it")
	}
}

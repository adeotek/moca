package session

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

func TestCreateAppendRead(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	w, err := Create(dir, Header{Workdir: "/w", Model: "a/b", SystemPrompt: "SYS"}, Slug("Fix the failing TestFoo in pkg/x, please!"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`/\d{4}-\d{2}-\d{2}-fix-the-failing-testfoo-in-[0-9a-f]{8}\.jsonl$`).MatchString(w.Path()) {
		t.Fatal(w.Path())
	}
	e1, _ := w.Append(Entry{Type: TypeMessage, Message: &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}})
	e2, _ := w.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "x"}})
	w.Close()
	if e2.ParentID != e1.ID || e1.ParentID == "" || e1.Time.IsZero() {
		t.Fatal("chain")
	}
	got, err := ReadFile(w.Path())
	if err != nil || len(got) != 3 || got[0].Type != TypeSession || got[0].Session.SystemPrompt != "SYS" {
		t.Fatalf("%+v %v", got, err)
	}
	fi, _ := os.Stat(w.Path())
	di, _ := os.Stat(dir)
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Fatal("modes")
	}
	w2, entries, err := Open(w.Path())
	if err != nil || len(entries) != 3 {
		t.Fatal(err)
	}
	e3, _ := w2.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "y"}})
	if e3.ParentID != e2.ID {
		t.Fatal("Open continues the chain")
	}
	_ = time.Now
}

func TestSlug(t *testing.T) {
	if Slug("") != "session" || Slug("!!!") != "session" || Slug("Héllo Wörld") != "h-llo-w-rld" {
		t.Fatal(Slug("Héllo Wörld"))
	}
}

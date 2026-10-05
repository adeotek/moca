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

func TestOpenTrimsCrashTruncatedLastLine(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	w, err := Create(dir, Header{Workdir: "/w", Model: "a/b"}, "s")
	if err != nil {
		t.Fatal(err)
	}
	w.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "one"}})
	path := w.Path()
	w.Close()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"id":"partial","type":"error","err`) // crash mid-write
	f.Close()

	w2, entries, err := Open(path)
	if err != nil || len(entries) != 2 {
		t.Fatalf("open after a truncated line: %v, %d entries", err, len(entries))
	}
	if _, err := w2.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "two"}}); err != nil {
		t.Fatal(err)
	}
	w2.Close()
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2].Error == nil || got[2].Error.Message != "two" {
		t.Fatalf("the entry appended after repair is lost: %d entries", len(got))
	}
}

func TestOpenKeepsCompleteLineWithoutNewline(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	w, err := Create(dir, Header{Workdir: "/w", Model: "a/b"}, "s")
	if err != nil {
		t.Fatal(err)
	}
	w.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "one"}})
	path := w.Path()
	w.Close()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, fi.Size()-1); err != nil { // drop the final newline only
		t.Fatal(err)
	}
	w2, entries, err := Open(path)
	if err != nil || len(entries) != 2 {
		t.Fatalf("open: %v, %d entries", err, len(entries))
	}
	if _, err := w2.Append(Entry{Type: TypeError, Error: &ErrorInfo{Message: "two"}}); err != nil {
		t.Fatal(err)
	}
	w2.Close()
	got, err := ReadFile(path)
	if err != nil || len(got) != 3 || got[2].Error.Message != "two" {
		t.Fatalf("complete last line must survive: %v, %d entries", err, len(got))
	}
}

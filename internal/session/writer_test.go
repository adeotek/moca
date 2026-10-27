package session

import (
	"errors"
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

// TestCreateLazyLeavesNoFile: a deferred session that never gets a prompt is
// an id and a path, nothing more (§8) — no file, no lock, and no sessions dir
// on a machine that never ran a prompt.
func TestCreateLazyLeavesNoFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	w, err := CreateLazy(dir, Header{Workdir: "/w", Model: "a/b"}, "s")
	if err != nil {
		t.Fatal(err)
	}
	if w.Created() {
		t.Fatal("CreateLazy must not create the file")
	}
	if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the file must not exist: %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the sessions dir must not exist yet: %v", err)
	}
	if !regexp.MustCompile(`/\d{4}-\d{2}-\d{2}-s-[0-9a-f]{8}\.jsonl$`).MatchString(w.Path()) {
		t.Fatalf("path: %q", w.Path())
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing a session that never wrote must be a no-op: %v", err)
	}
	if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Close must not create the file")
	}
}

// TestLazyWriterMaterializesOnTheFirstTurn: a model or mode switch before the
// first prompt is buffered, not written; the first message creates the file
// with the header, the buffered entry and itself, in order and chained, so the
// transcript reads the same as a prompt-first start.
func TestLazyWriterMaterializesOnTheFirstTurn(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	w, err := CreateLazy(dir, Header{Workdir: "/w", Model: "a/b", SystemPrompt: "SYS"}, "s")
	if err != nil {
		t.Fatal(err)
	}
	mc, err := w.Append(Entry{Type: TypeModelChange, ModelChange: &ModelChange{Model: "a/b", Effort: "high"}})
	if err != nil {
		t.Fatal(err)
	}
	if w.Created() {
		t.Fatal("a pre-conversation entry must not create the session")
	}
	msg, err := w.Append(Entry{Type: TypeMessage, Message: &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !w.Created() {
		t.Fatal("the first message must create the session")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Type != TypeSession || got[1].Type != TypeModelChange || got[2].Type != TypeMessage {
		t.Fatalf("entries: %+v", got)
	}
	if got[0].Session.SystemPrompt != "SYS" || got[1].ID != mc.ID || got[2].ID != msg.ID {
		t.Fatalf("ids/header: %+v", got)
	}
	if got[1].ParentID != got[0].ID || got[2].ParentID != got[1].ID {
		t.Fatalf("chain: %q → %q → %q", got[0].ID, got[1].ParentID, got[2].ParentID)
	}
	// The file the lazy writer created is a session the rest of the package
	// reads: Find locates it by its id8 and the header's workdir.
	if _, err := Find(dir, w.ID8(), "/w"); err != nil {
		t.Fatalf("Find: %v", err)
	}
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

// Two writers on one file would interleave histories, and a resume would
// repair the other process's in-flight tool calls: Open must refuse while the
// file is held, and work again once the holder closes.
func TestOpenRefusesWhileAnotherWriterHoldsTheFile(t *testing.T) {
	if !lockSupported {
		t.Skip("no advisory file locks on this platform")
	}
	w, err := Create(t.TempDir(), Header{Workdir: "/w"}, "lock")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(w.Path())
	if _, _, err := Open(w.Path()); !errors.Is(err, ErrInUse) {
		t.Fatalf("second writer must be refused: %v", err)
	}
	if after, _ := os.ReadFile(w.Path()); string(after) != string(before) {
		t.Fatal("a refused Open must not touch the file")
	}
	w.Close()
	w2, _, err := Open(w.Path())
	if err != nil {
		t.Fatalf("lock must be released by Close: %v", err)
	}
	w2.Close()
}

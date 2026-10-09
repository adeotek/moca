package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

func TestFind(t *testing.T) {
	dir := t.TempDir()
	w1, _ := Create(dir, Header{Workdir: "/a"}, "one")
	w1.Close()
	time.Sleep(20 * time.Millisecond)
	w2, _ := Create(dir, Header{Workdir: "/b"}, "two")
	w2.Close()
	if p, err := Find(dir, "last"); err != nil || p != w2.Path() {
		t.Fatal(p, err)
	}
	if p, err := Find(dir, w1.ID8()); err != nil || p != w1.Path() {
		t.Fatal(p, err)
	}
	if _, err := Find(dir, "deadbeef"); err == nil || !strings.Contains(err.Error(), w2.ID8()) {
		t.Fatal("not-found lists recent ids:", err)
	}
	if _, err := Find(dir, "../x"); err == nil {
		t.Fatal("ref must be 8 hex chars")
	}
}

func TestFindForWorkdirViaSymlink(t *testing.T) {
	dir, base := t.TempDir(), t.TempDir()
	real := filepath.Join(base, "real")
	os.Mkdir(real, 0o755)
	canon, _ := filepath.EvalSymlinks(real)
	link := filepath.Join(base, "link")
	os.Symlink(real, link)
	w, _ := Create(dir, Header{Workdir: canon}, "x")
	w.Close()
	Create(dir, Header{Workdir: "/elsewhere"}, "y")
	if p, err := FindForWorkdir(dir, link); err != nil || p != w.Path() {
		t.Fatal(p, err)
	}
}

func TestFindNoSessions(t *testing.T) {
	dir := t.TempDir()
	if _, err := Find(dir, "last"); err == nil {
		t.Fatal("empty dir")
	}
	if _, err := FindForWorkdir(dir, dir); err == nil || !strings.Contains(err.Error(), "no session for") {
		t.Fatal("no session for this workdir:", err)
	}
}

// Equal mtimes must resolve deterministically (stable sort + name order),
// not by directory-iteration chance.
func TestFindMtimeTieDeterministic(t *testing.T) {
	dir := t.TempDir()
	w1, _ := Create(dir, Header{Workdir: "/a"}, "one")
	w1.Close()
	w2, _ := Create(dir, Header{Workdir: "/b"}, "two")
	w2.Close()
	ts := time.Now()
	for _, p := range []string{w1.Path(), w2.Path()} {
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatal(err)
		}
	}
	// Stable sort keeps directory-name order among ties: "one" wins.
	p, err := Find(dir, "last")
	if err != nil || p != w1.Path() {
		t.Fatalf("mtime ties must resolve by name order: %s (%v)", p, err)
	}
}

func TestListPreviewsAndFilters(t *testing.T) {
	dir := t.TempDir()
	wd := t.TempDir()
	other := t.TempDir()
	mk := func(name, workdir string, msgs ...string) {
		var b strings.Builder
		h, _ := json.Marshal(Entry{ID: "h", Type: TypeSession, Session: &Header{Workdir: workdir}})
		b.Write(h)
		b.WriteByte('\n')
		for i, m := range msgs {
			e, _ := json.Marshal(Entry{ID: fmt.Sprint(i), Type: TypeMessage, Message: &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: m}}}})
			b.Write(e)
			b.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mk("2026-01-01-aaaaaaaa.jsonl", wd, "first line\nsecond")
	mk("2026-01-02-bbbbbbbb.jsonl", wd) // never got a message
	mk("2026-01-03-cccccccc.jsonl", other, "elsewhere")
	mk("2026-01-04-dddddddd.jsonl", wd, "[Summary of earlier conversation]\nx", "real question")
	mk("2026-01-05-eeeeeeee.jsonl", wd, "in use")
	got := List(dir, wd, "eeeeeeee", 10)
	var ids []string
	for _, g := range got {
		ids = append(ids, g.ID8+"="+g.Preview)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", ids)
	}
	have := map[string]string{got[0].ID8: got[0].Preview, got[1].ID8: got[1].Preview}
	if have["aaaaaaaa"] != "first line" || have["dddddddd"] != "real question" {
		t.Fatalf("previews: %v", have)
	}
	if len(List(dir, "", "", 10)) != 4 || len(List(dir, wd, "", 1)) != 1 {
		t.Fatal("any-workdir listing and the limit")
	}
}

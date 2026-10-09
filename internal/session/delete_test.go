package session

import (
	"errors"
	"os"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	held, err := Create(dir, Header{Workdir: "/w"}, "held")
	if err != nil {
		t.Fatal(err)
	}
	// The live writer's lock must refuse the delete, and nothing may vanish.
	if err := Delete(held.Path()); !errors.Is(err, ErrInUse) {
		t.Fatalf("locked session: %v", err)
	}
	if _, err := os.Stat(held.Path()); err != nil {
		t.Fatalf("a refused delete must leave the file: %v", err)
	}
	held.Close()
	if err := Delete(held.Path()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(held.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file still exists after delete")
	}
	if err := Delete(held.Path()); err == nil {
		t.Fatal("deleting a missing session must fail")
	}
}

func TestListAll(t *testing.T) {
	dir := t.TempDir()
	seed := func(wd string, msgs ...string) *Writer {
		t.Helper()
		w, err := Create(dir, Header{Workdir: wd}, "t")
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range msgs {
			msg := llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: m}}}
			if _, err := w.Append(Entry{Type: TypeMessage, Message: &msg}); err != nil {
				t.Fatal(err)
			}
		}
		w.Close()
		return w
	}
	a := seed("/w", "hello there")
	b := seed("/w") // never got a message
	c := seed("/other", "elsewhere")

	all, total := ListAll(dir, "/w", 10)
	if len(all) != 2 || total != 2 {
		t.Fatalf("ListAll: %d rows (%v)", len(all), all)
	}
	byID := map[string]string{}
	for _, s := range all {
		byID[s.ID8] = s.Preview
	}
	if byID[a.ID8()] != "hello there" {
		t.Fatalf("preview: %v", byID)
	}
	if _, ok := byID[b.ID8()]; !ok {
		t.Fatalf("the message-less session must be listed: %v", byID)
	}
	if _, ok := byID[c.ID8()]; ok {
		t.Fatal("another workdir's session must not be listed")
	}
	if all, total := ListAll(dir, "", 10); len(all) != 3 || total != 3 {
		t.Fatal("any-workdir ListAll")
	}
	// Past the limit the total still counts every match (in this workdir only).
	if all, total := ListAll(dir, "/w", 1); len(all) != 1 || total != 2 {
		t.Fatalf("limit: %d rows, total %d", len(all), total)
	}
	// List keeps its own contract: skipID8 honoured, message-less filtered.
	if got := List(dir, "/w", a.ID8(), 10); len(got) != 0 {
		t.Fatalf("List: %v", got)
	}
}

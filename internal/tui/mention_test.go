package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMentionQuery(t *testing.T) {
	cases := []struct {
		buf   string
		q     string
		start int
		ok    bool
	}{
		{"@", "", 0, true},
		{"look at @int", "int", 8, true},
		{"line one\n@src/a", "src/a", 9, true},
		{"mail me@example.com", "", 0, false}, // the @ must start the word
		{"done @a.go ", "", 0, false},         // the word is finished
		{"plain", "", 0, false},
		{"", "", 0, false},
	}
	for _, c := range cases {
		if q, start, ok := mentionQuery(c.buf); q != c.q || start != c.start || ok != c.ok {
			t.Errorf("mentionQuery(%q) = %q,%d,%v want %q,%d,%v", c.buf, q, start, ok, c.q, c.start, c.ok)
		}
	}
}

func TestMentionRanking(t *testing.T) {
	files := []string{"internal/tui/app.go", "app.go", "docs/app.md", "internal/tui/dropdown.go", "cmd/moca/main.go", "internal/application/x.go"}
	names := func(q string) []string {
		var out []string
		for _, it := range mentionMatches(files, q) {
			out = append(out, it.name)
		}
		return out
	}
	// basename prefix beats path substring; shallower first among equals.
	got := names("app")
	if got[0] != "app.go" || got[1] != "docs/app.md" || got[2] != "internal/tui/app.go" || got[3] != "internal/application/x.go" {
		t.Fatalf("ranking: %v", got)
	}
	if got := names("tui/dr"); len(got) != 1 || got[0] != "internal/tui/dropdown.go" {
		t.Fatalf("path fragment: %v", got)
	}
	if got := names("zzz"); len(got) != 0 {
		t.Fatalf("no match: %v", got)
	}
	if got := names(""); len(got) != len(files) || got[0] != "app.go" {
		t.Fatalf("bare @ lists shallow files first: %v", got)
	}
}

func TestMentionCompletionFlow(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"main.go", "pkg/util.go", "pkg/util_test.go", ".git/config"} {
		p := filepath.Join(dir, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, nil, 0o644)
	}
	m := newTestModel()
	m.opts.Start.Workdir = dir
	// The first @ starts the index walk (the runtime would run the returned
	// command; the test runs it by hand and delivers the result).
	typeText(m, "see @ut")
	if !m.filesLoading {
		t.Fatal("typing @ starts the index walk")
	}
	if m.dropdown() != nil {
		t.Fatal("no dropdown until the index is loaded")
	}
	m.filesLoading = false
	load := m.maybeLoadFiles()
	if load == nil {
		t.Fatal("a load command")
	}
	m.Update(load())
	d := m.dropdown()
	if d == nil || !d.mention || len(d.items) != 2 || d.items[0].name != "pkg/util.go" {
		t.Fatalf("mention items: %+v", d)
	}
	for _, it := range d.items {
		if strings.HasPrefix(it.name, ".git") {
			t.Fatal("dot-directories are not offered")
		}
	}
	// enter inserts the path (never sends) and ends the word.
	_, c := m.Update(key("enter"))
	if m.running || c != nil && printed(c) != "" {
		t.Fatalf("enter must not send while a mention list is open: %q", printed(c))
	}
	if got := m.input.Buffer(); got != "see @pkg/util.go " {
		t.Fatalf("buffer: %q", got)
	}
	if m.dropdown() != nil {
		t.Fatal("list closes once the word is finished")
	}
	// A second enter now sends (no list).
	_, c = m.Update(key("enter"))
	if m.input.Buffer() != "" {
		t.Fatalf("second enter sends: %q", m.input.Buffer())
	}
}

func TestMentionEscDismissesAndEmailDoesNot(t *testing.T) {
	m := newTestModel()
	m.files = []string{"a.go", "b.go"}
	m.input.SetBuffer("x @")
	m.syncTextarea()
	if d := m.dropdown(); d == nil || len(d.items) != 2 {
		t.Fatalf("bare @ lists files: %+v", d)
	}
	m.Update(key("esc"))
	if m.dropdown() != nil {
		t.Fatal("esc dismisses")
	}
	m.input.SetBuffer("mail me@a")
	m.syncTextarea()
	if m.dropdown() != nil {
		t.Fatal("an email address is not a mention")
	}
	// A prompt template's argument can be a mention.
	m.input.SetBuffer("/review @b")
	m.syncTextarea()
	if d := m.dropdown(); d == nil || !d.mention || d.items[0].name != "b.go" {
		t.Fatalf("mention inside a command line: %+v", d)
	}
}

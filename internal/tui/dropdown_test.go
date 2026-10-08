package tui

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func TestSlashQuery(t *testing.T) {
	cases := []struct {
		buf string
		q   string
		ok  bool
	}{
		{"", "", false},
		{"/", "", true},
		{"/lo", "lo", true},
		{"/model x", "", false},
		{"/model\tx", "", false},
		{"//escaped", "", false},
		{"/mo\nre", "", false},
		{"hello", "", false},
		{"x/lo", "", false},
	}
	for _, c := range cases {
		if q, ok := slashQuery(c.buf); q != c.q || ok != c.ok {
			t.Errorf("slashQuery(%q) = %q,%v want %q,%v", c.buf, q, ok, c.q, c.ok)
		}
	}
}

func dropNames(d *dropState) []string {
	var out []string
	for _, it := range d.items {
		out = append(out, it.name)
	}
	return out
}

func TestDropdownListsCommandsAndPrompts(t *testing.T) {
	m := newTestModel()
	m.opts.Prompts = []skills.Prompt{
		{Name: "review", ArgumentHint: "<path>", Description: "review code"},
		{Name: "model", Description: "shadowed by the builtin"},
	}
	m.input.SetBuffer("/")
	m.syncTextarea()
	d := m.dropdown()
	if d == nil {
		t.Fatal("no dropdown for /")
	}
	names := dropNames(d)
	for _, want := range []string{"model", "login", "logout", "help", "exit", "review"} {
		if !contains(names, want) {
			t.Fatalf("dropdown missing %q: %v", want, names)
		}
	}
	// The prompt shadowing a builtin name is not listed twice.
	if n := count(names, "model"); n != 1 {
		t.Fatalf("model listed %d times: %v", n, names)
	}
	// Prefix filter, case-insensitive.
	m.input.SetBuffer("/LO")
	m.syncTextarea()
	m.drop = nil
	d = m.dropdown()
	if d == nil || len(d.items) != 2 || d.items[0].name != "login" || d.items[1].name != "logout" {
		t.Fatalf("filter /LO: %+v", d)
	}
	// No match → hidden.
	m.input.SetBuffer("/zzz")
	m.syncTextarea()
	m.drop = nil
	if d := m.dropdown(); d != nil {
		t.Fatalf("no-match dropdown: %+v", d)
	}
}

func TestDropdownKeysCompleteAndSend(t *testing.T) {
	m := newTestModel()
	for _, ch := range "/lo" {
		m.Update(key(string(ch)))
	}
	d := m.dropdown()
	if d == nil || len(d.items) != 2 || d.cursor != 0 {
		t.Fatalf("dropdown after /lo: %+v", d)
	}
	m.Update(key("down"))
	if m.drop.cursor != 1 {
		t.Fatalf("down must move the cursor: %d", m.drop.cursor)
	}
	m.Update(key("up"))
	if m.drop.cursor != 0 {
		t.Fatalf("up must move back: %d", m.drop.cursor)
	}
	// tab completes the selection into the draft (no send).
	m.Update(key("tab"))
	if got := m.input.Buffer(); got != "/login " {
		t.Fatalf("tab completion: %q", got)
	}
	if m.dropdown() != nil {
		t.Fatal("the dropdown must hide once the word is complete")
	}
	if m.ta.Value() != "/login " {
		t.Fatalf("textarea out of sync: %q", m.ta.Value())
	}
	// enter on a partial word completes instead of sending.
	m.input.SetBuffer("/ex")
	m.syncTextarea()
	m.drop = nil
	m.Update(key("enter"))
	if got := m.input.Buffer(); got != "/exit " {
		t.Fatalf("enter on partial must complete: %q", got)
	}
	// enter on an exact name submits.
	m.input.SetBuffer("/help")
	m.syncTextarea()
	m.drop = nil
	m.Update(key("enter"))
	if got := m.input.Buffer(); got != "" {
		t.Fatalf("enter on an exact name must submit: %q", got)
	}
}

func TestDropdownEscDismissesUntilTheWordChanges(t *testing.T) {
	m := newTestModel()
	for _, ch := range "/lo" {
		m.Update(key(string(ch)))
	}
	if m.dropdown() == nil {
		t.Fatal("dropdown expected")
	}
	m.Update(key("esc"))
	if m.dropdown() != nil {
		t.Fatal("esc must dismiss")
	}
	// Typing more re-arms the dropdown for the new word.
	m.Update(key("g"))
	if m.dropdown() == nil {
		t.Fatal("a changed word must re-arm the dropdown")
	}
}

func TestDropdownHiddenWhenEnterWouldBeRefused(t *testing.T) {
	m := newTestModel()
	m.input.SetBuffer("/lo")
	m.syncTextarea()
	for name, set := range map[string]func(){
		"running":    func() { m.running = true },
		"compacting": func() { m.compacting = true },
		"shellBusy":  func() { m.shellBusy = true },
		"login":      func() { m.login = &loginState{} },
	} {
		set()
		if d := m.dropdown(); d != nil {
			t.Fatalf("%s: dropdown must be hidden", name)
		}
	}
}

func TestDropdownRendersCursorAndLegend(t *testing.T) {
	m := newTestModel()
	// Put the cursor on login for a deterministic first row.
	for _, ch := range "/lo" {
		m.Update(key(string(ch)))
	}
	v := m.View().Content
	for _, want := range []string{"❯ login", "logout", "tab complete", "esc dismiss"} {
		if !strings.Contains(v, want) {
			t.Fatalf("view missing %q:\n%s", want, v)
		}
	}
	// Multi-line drafts never show it.
	m.input.SetBuffer("/lo\nextra")
	m.syncTextarea()
	if strings.Contains(m.View().Content, "esc dismiss") {
		t.Fatal("multi-line draft must hide the dropdown")
	}
}

func TestClampRunes(t *testing.T) {
	if got := clampRunes("héllo wörld", 5); got != "héll…" {
		t.Fatalf("clampRunes: %q", got)
	}
	if got := clampRunes("ok", 5); got != "ok" {
		t.Fatalf("clampRunes short: %q", got)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func count(list []string, s string) int {
	n := 0
	for _, v := range list {
		if v == s {
			n++
		}
	}
	return n
}

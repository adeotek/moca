package tui

import (
	"os"
	"path/filepath"
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
	// enter on a partial word completes instead of sending (/exi: /ex is
	// ambiguous — it also prefixes /execute, which is listed first).
	m.input.SetBuffer("/exi")
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

// Skills join the dropdown tagged `skill: ` (prompt templates `prompt: `);
// a name shadowed by a built-in or a prompt is listed once — its winner —
// and tab completes a skill name like any other word.
func TestDropdownListsSkillsAndTags(t *testing.T) {
	m := newAgentModel(t)
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "moca", "skills")
	writeSkill(t, dir, "deploy", "ship it")
	writeSkill(t, dir, "review", "shadowed by the prompt template")
	writeSkill(t, dir, "model", "shadowed by the built-in")
	m.opts.Prompts = []skills.Prompt{{Name: "review", ArgumentHint: "<path>", Description: "review code"}}
	m.agent.ReloadSkills()
	m.input.SetBuffer("/")
	m.syncTextarea()
	d := m.dropdown()
	if d == nil {
		t.Fatal("no dropdown for /")
	}
	row := func(name string) (dropItem, bool) {
		for _, it := range d.items {
			if it.name == name {
				return it, true
			}
		}
		return dropItem{}, false
	}
	if it, ok := row("deploy"); !ok || it.hint != "skill: ship it" {
		t.Fatalf("skill row: %+v ok=%v", it, ok)
	}
	if it, ok := row("review"); !ok || !strings.HasPrefix(it.hint, "prompt: ") {
		t.Fatalf("the prompt must win the name and carry its tag: %+v", it)
	}
	names := dropNames(d)
	if count(names, "review") != 1 || count(names, "model") != 1 {
		t.Fatalf("shadowed names must be listed once: %v", names)
	}
	// Tab completes a skill name (prefix filter sees it like any other).
	m.input.SetBuffer("/dep")
	m.syncTextarea()
	m.drop = nil
	m.Update(key("tab"))
	if got := m.input.Buffer(); got != "/deploy " {
		t.Fatalf("tab completion: %q", got)
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

// Enter on a full command name runs that command, even when a longer sibling
// shares the prefix: the cursor lands on the exact row, so `/mode` + enter no
// longer completes to `/model` and opens the model picker (v0.7.11 — the
// pre-dropdown rule "enter on an exact name sends" was only true while the
// exact name happened to be the first match).
func TestDropdownEnterRunsTheExactName(t *testing.T) {
	m := newAgentModel(t)
	for _, c := range BuiltinCommands {
		m.input.SetBuffer("/" + c)
		m.syncTextarea()
		m.drop = nil
		d := m.dropdown()
		if d == nil {
			t.Fatalf("/%s: the dropdown must open", c)
		}
		if got := d.items[d.cursor].name; got != c {
			t.Fatalf("/%s: the cursor sits on %q, want the exact row", c, got)
		}
		if _, handled := m.dropdownKey(d, keyMsg("enter")); handled {
			t.Fatalf("/%s: enter must submit, not complete the draft (%q)", c, m.input.Buffer())
		}
	}
	// The rule itself, on a list where the exact row is not the first match —
	// the argument rows (a model id, an effort level) behave the same way.
	items := []dropItem{{name: "model"}, {name: "mode"}}
	for _, tc := range []struct {
		word string
		want int
	}{{"mode", 1}, {"MODE", 1}, {"mod", 0}, {"", 0}} {
		if got := exactCursor(items, tc.word); got != tc.want {
			t.Fatalf("exactCursor(%q) = %d, want %d", tc.word, got, tc.want)
		}
	}
	if got := exactCursor(nil, "anything"); got != 0 {
		t.Fatalf("an empty list must keep cursor 0, got %d", got)
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

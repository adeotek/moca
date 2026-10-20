package acp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func writeAutoSkillFile(t *testing.T, cwd, name string) {
	t.Helper()
	dir := filepath.Join(cwd, ".moca", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: d\nauto: true\n---\n- **a**: b (×1, 2026-10-18)\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// managedOptions is testOptions with the self-managed switch on (ACP's trust
// default makes the feature live without a flag — Decision 14).
func managedOptions(t *testing.T, s *scriptServer) Options {
	t.Helper()
	o := testOptions(t, s)
	on := true
	o.Config.Skills.SelfManaged = &on
	return o
}

func TestACPCommandsReserveRememberAlways(t *testing.T) {
	// N-10/N-17: reserved either way. With the feature off the command is still
	// listed (it is refused when typed), and a user template named `remember`
	// can never shadow it.
	off := buildCommands([]skills.Prompt{{Name: "remember", Description: "my own"}}, nil, false)
	if len(off) != 1 || off[0].Description == "my own" {
		t.Fatalf("off: a prompt template named remember must not be advertised: %+v", off)
	}
	if !strings.Contains(off[0].Description, "off") {
		t.Fatalf("off: the description must say the feature is off: %+v", off[0])
	}
	// On: reserved first, before prompts and skills.
	on := buildCommands(
		[]skills.Prompt{{Name: "remember", Description: "shadowed"}, {Name: "greet", Description: "hi"}},
		[]skills.Skill{{Name: "deploy", Description: "d"}}, true)
	if len(on) != 3 || on[0].Name != "remember" || on[0].Description != "record this as a project skill" ||
		on[0].Input == nil || on[0].Input.Hint != "rule text" {
		t.Fatalf("on: %+v", on)
	}
	if on[1].Name != "greet" || on[2].Name != "deploy" {
		t.Fatalf("order: %+v", on)
	}
}

func TestACPAutoSkillsAreNotCommands(t *testing.T) {
	got := buildCommands(nil, []skills.Skill{
		{Name: "moca-testing", Description: "learned", Auto: true},
		{Name: "deploy", Description: "hand-written"},
	}, true)
	for _, c := range got {
		if c.Name == "moca-testing" {
			t.Fatalf("an auto skill must never be advertised: %+v", got)
		}
	}
	if len(got) != 2 || got[0].Name != "remember" || got[1].Name != "deploy" {
		t.Fatalf("%+v", got)
	}
}

func TestACPTypedExcludesResources(t *testing.T) {
	body := "fix the deploy skill's staging step"
	got := textBlocks([]ContentBlock{
		{Type: "text", Text: "please"},
		{Type: "resource", Resource: &EmbeddedResource{URI: "file:///w/deploy.md", Text: &body}},
		{Type: "text", Text: "review this"},
	})
	if got != "please review this" {
		t.Fatalf("typed = %q", got)
	}
	// A resource-only prompt has no typed text at all: an attachment cannot
	// authorize a write (Decision 15).
	if got := textBlocks([]ContentBlock{{Type: "resource", Resource: &EmbeddedResource{URI: "file:///w/x.md", Text: &body}}}); got != "" {
		t.Fatalf("attachment-only typed = %q", got)
	}
}

func TestACPRememberSplitsOnAnyWhitespace(t *testing.T) {
	// A newline after the command is a separator for expand, so the typed
	// rule must be split the same way — never truncated at the first space.
	s := newScript(t, textTurn("ok"))
	c := startClient(t, managedOptions(t, s))
	sid := c.newSession(t.TempDir())
	if _, err := c.prompt(sid, "/remember\nuse make test"); err != nil {
		t.Fatalf("prompt: %v", err.Message)
	}
	if txt := lastUserText(t, s); !strings.Contains(txt, "Remember this project rule: use make test") {
		t.Fatalf("the whole typed rule must reach the run, got %s", txt)
	}
	if name, args := splitCommand("/remember\tuse  it"); name != "remember" || args != "use  it" {
		t.Fatalf("splitCommand = %q, %q", name, args)
	}
}

func TestACPRememberIntercepted(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	c := startClient(t, managedOptions(t, s))
	sid := c.newSession(t.TempDir())
	if _, err := c.prompt(sid, "/remember use -count=1"); err != nil {
		t.Fatalf("prompt: %v", err.Message)
	}
	if txt := lastUserText(t, s); !strings.Contains(txt, "Remember this project rule: use -count=1") {
		t.Fatalf("the run must carry the expansion, got %s", txt)
	}
	// Advertised while the feature is on.
	advertised := false
	for _, u := range c.updatesOf(updCommands) {
		for _, cm := range u["availableCommands"].([]any) {
			if cm.(map[string]any)["name"] == "remember" {
				advertised = true
			}
		}
	}
	if !advertised {
		t.Fatal("remember must be advertised while the feature is on")
	}
}

// Inverted by N-16/N-17: with the feature off, /remember is still a reserved
// command — it is answered with a notice (not sent to the model, not answered
// with -32603 "internal error"), and a user prompt template named `remember`
// can never shadow it.
func TestACPRememberIsReservedWhenOff(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(t.TempDir())
	stop, err := c.prompt(sid, "/remember use -count=1")
	if err != nil {
		t.Fatalf("/remember with the feature off must not be a protocol error: %v (code %d)", err.Message, err.Code)
	}
	if stop != "end_turn" {
		t.Fatalf("stop reason = %q, want end_turn", stop)
	}
	s.mu.Lock()
	n := len(s.bodies)
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("a reserved command reached the model (%d request(s))", n)
	}
	found := false
	for _, u := range c.allUpdates() {
		if strings.Contains(mustJSON(u), "self-managed skills are off") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the refusal must arrive as a message update: %v", c.allUpdates())
	}
	// It is still advertised: typing it is meaningful (a refusal the user can
	// read), so the command list must not hide it.
	advertised := func() string {
		out := ""
		for _, u := range c.updatesOf(updCommands) {
			for _, cm := range u["availableCommands"].([]any) {
				if m := cm.(map[string]any); m["name"] == "remember" {
					out, _ = m["description"].(string)
				}
			}
		}
		return out
	}
	c.eventually(t, "the advertised command list", func() bool { return advertised() != "" })
	if advertised() == "" {
		t.Fatal("remember must stay advertised with the feature off")
	}
}

// A bare /remember is still reserved: with the feature on there is no rule to
// record, so it is answered as a notice instead of reaching the model as text.
func TestACPBareRememberIsAnsweredNotForwarded(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	c := startClient(t, managedOptions(t, s))
	sid := c.newSession(t.TempDir())
	stop, err := c.prompt(sid, "/remember")
	if err != nil {
		t.Fatalf("bare /remember must not be a protocol error: %v (code %d)", err.Message, err.Code)
	}
	if stop != "end_turn" {
		t.Fatalf("stop reason = %q", stop)
	}
	s.mu.Lock()
	n := len(s.bodies)
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("bare /remember reached the model (%d request(s))", n)
	}
	found := false
	for _, u := range c.allUpdates() {
		if strings.Contains(mustJSON(u), "needs the rule to record") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the refusal must arrive as a message update: %v", c.allUpdates())
	}
}

// A user prompt template named `remember` can never shadow the command.
func TestACPRememberTemplateNeverRuns(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	opts := managedOptions(t, s)
	c := startClient(t, opts)
	cwd := t.TempDir()
	sid := c.newSession(cwd)
	stop, err := c.prompt(sid, "/remember keep the suite green")
	if err != nil {
		t.Fatalf("prompt: %v", err.Message)
	}
	if stop != "end_turn" && stop != "max_turns" {
		t.Fatalf("stop reason = %q", stop)
	}
	// The run that started is the /remember run: its typed text is the rule,
	// never a template body borrowed from a prompt named `remember`.
	s.mu.Lock()
	first := ""
	if len(s.bodies) > 0 {
		first = mustJSON(s.bodies[0])
	}
	s.mu.Unlock()
	if strings.Contains(first, "template body") {
		t.Fatal("a prompt template named remember ran")
	}
}

func TestACPStartupNoteArrivesAsAnUpdate(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	c := startClient(t, managedOptions(t, s))
	cwd := t.TempDir()
	writeAutoSkillFile(t, cwd, "moca-one")
	sid := c.newSession(cwd)
	if _, err := c.prompt(sid, "hi"); err != nil {
		t.Fatalf("prompt: %v", err.Message)
	}
	// The startup note arrives as a session update (a notice when the client
	// advertises the extension, a thought line otherwise).
	found := false
	for _, u := range c.allUpdates() {
		if strings.Contains(mustJSON(u), "self-managed skills: 1 skill") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the startup note must reach the client: %v", c.allUpdates())
	}
}

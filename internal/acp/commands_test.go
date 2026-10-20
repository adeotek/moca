package acp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func TestFlatten(t *testing.T) {
	text := "func main() {}"
	got, err := flatten("/w", []ContentBlock{
		{Type: "text", Text: "look at"},
		{Type: "resource_link", URI: "file:///w/cmd/main.go", Name: "main.go"},
		{Type: "text", Text: "and"},
		{Type: "resource_link", URI: "file:///elsewhere/x.go"},
		{Type: "resource", Resource: &EmbeddedResource{URI: "file:///w/a.go", Text: &text}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "look at @cmd/main.go and @/elsewhere/x.go\n\na.go:\n```\nfunc main() {}\n```"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	for _, bad := range []ContentBlock{{Type: "image"}, {Type: "audio"}, {Type: "resource", Resource: &EmbeddedResource{URI: "x", Blob: strp("AAAA")}}} {
		if _, err := flatten("/w", []ContentBlock{bad}); err == nil {
			t.Errorf("%s accepted", bad.Type)
		}
	}
	if _, err := flatten("/w", nil); err == nil {
		t.Error("empty prompt accepted")
	}
}

func TestBuildCommandsPrecedence(t *testing.T) {
	got := buildCommands(
		[]skills.Prompt{{Name: "review", Description: "Review a file", ArgumentHint: "<file>"}},
		[]skills.Skill{{Name: "review", Description: "shadowed"}, {Name: "deploy", Description: "Deploy"}},
		false,
	)
	if len(got) != 3 || got[0].Name != "remember" || got[1].Name != "review" || got[1].Input.Hint != "<file>" ||
		got[2].Name != "deploy" || got[2].Input.Hint != "optional request" {
		t.Fatalf("%+v", got)
	}
}

func TestSlashTemplateExpandsAndIsAdvertised(t *testing.T) {
	work := t.TempDir()
	s := newScript(t, textTurn("ok"))
	o := testOptions(t, s) // sets XDG_CONFIG_HOME, so write the template after it
	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "moca", "prompts")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "greet.md"), []byte("---\ndescription: Say hi\nargument-hint: <name>\n---\nSay hello to $ARGUMENTS."), 0o644)
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "/greet Ada")
	cmds := c.updatesOf(updCommands)
	if len(cmds) == 0 || !strings.Contains(mustJSON(cmds[0]), `"greet"`) {
		t.Fatalf("commands update: %v", cmds)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: cmds[0]})
	if body := lastUserText(t, s); !strings.Contains(body, "Say hello to Ada.") {
		t.Fatalf("template not expanded; model saw %q", body)
	}
}

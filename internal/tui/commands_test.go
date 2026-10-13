package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

// A command written during a run (the create-command workflow) becomes usable
// when the run finishes: the prompt dirs are re-read, and the dropdown and the
// parser both see it.
func TestPromptDirsReloadAtRunEnd(t *testing.T) {
	m := newTestModel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deploy.md"),
		[]byte("---\ndescription: ship it\nargument-hint: <env>\n---\nDeploy $1 now"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.opts.PromptDirs = []skills.Dir{{Path: dir, Source: "global"}}
	if _, err := ParseInput("/deploy prod", m.opts.Prompts, m.agentSkills()); err == nil {
		t.Fatal("not loaded before the run ends")
	}
	m.handleRunDone(runDoneMsg{})
	p, err := ParseInput("/deploy prod", m.opts.Prompts, m.agentSkills())
	if err != nil || p.Kind != KindPrompt || p.Text != "Deploy prod now" {
		t.Fatalf("after the run: %+v %v", p, err)
	}
	if !slices.ContainsFunc(m.commandHints(), func(d dropItem) bool { return d.name == "deploy" }) {
		t.Fatalf("the dropdown must list it: %+v", m.commandHints())
	}
	// Without dirs (callers passing literal prompts) nothing changes.
	m2 := newTestModel()
	m2.opts.Prompts = []skills.Prompt{{Name: "fixed", Body: "x"}}
	m2.handleRunDone(runDoneMsg{})
	if len(m2.opts.Prompts) != 1 || m2.opts.Prompts[0].Name != "fixed" {
		t.Fatalf("no dirs, no reload: %+v", m2.opts.Prompts)
	}
}

func TestParseInput(t *testing.T) {
	prompts := []skills.Prompt{{Name: "review", Body: "Review $1"}, {Name: "model", Body: "shadowed"}}
	cases := []struct {
		in   string
		want Parsed
	}{
		{"hello", Parsed{Kind: KindText, Text: "hello"}},
		{"/model opencode-go/glm-5.3", Parsed{Kind: KindCommand, Name: "model", Args: "opencode-go/glm-5.3"}},
		{"/hard", Parsed{Kind: KindCommand, Name: "hard"}},
		{"/review a.go", Parsed{Kind: KindPrompt, Name: "review", Args: "a.go", Text: "Review a.go"}},
		{"!go test ./...", Parsed{Kind: KindShell, Text: "go test ./..."}},
		{"!!ls", Parsed{Kind: KindShellLocal, Text: "ls"}},
		{"//etc/hosts is odd", Parsed{Kind: KindText, Text: "/etc/hosts is odd"}},
		{"  /show 7 ", Parsed{Kind: KindCommand, Name: "show", Args: "7"}},
		{"/exit", Parsed{Kind: KindCommand, Name: "exit"}},
		{"/q", Parsed{Kind: KindCommand, Name: "exit"}},
		{"/quit now", Parsed{Kind: KindCommand, Name: "exit", Args: "now"}},
	}
	for _, c := range cases {
		got, err := ParseInput(c.in, prompts, nil)
		if err != nil || got != c.want {
			t.Errorf("ParseInput(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
	// An alias never hides a prompt template that owns the name.
	owned := []skills.Prompt{{Name: "q", Body: "Question $1"}}
	if got, _ := ParseInput("/q why", owned, nil); got.Kind != KindPrompt || got.Text != "Question why" {
		t.Fatalf("a prompt named like an alias must win: %+v", got)
	}
	if _, err := ParseInput("/nope", prompts, nil); err == nil || !strings.Contains(err.Error(), "/help") {
		t.Fatal(err)
	}
	if p, _ := ParseInput("/model", prompts, nil); p.Kind != KindCommand {
		t.Fatal("built-ins win on collision")
	}
	if !strings.Contains(HelpText(prompts, nil), "/review") {
		t.Fatal("help lists prompts")
	}
	if !strings.Contains(HelpText(prompts, nil), "/exit") {
		t.Fatal("help lists /exit")
	}
}

// Skills are invokable from the picker: /name sends a message naming the
// skill's path (the agent loads the body with `read` and follows it).
// Built-ins and prompt templates win a name collision.
func TestParseInputSkills(t *testing.T) {
	sk := []skills.Skill{{Name: "rtk", Path: "/cfg/builtin/rtk/SKILL.md", Description: "compressed CLI"}}
	p, err := ParseInput("/rtk grep stuff", nil, sk)
	if err != nil || p.Kind != KindSkill || p.Name != "rtk" || p.Args != "grep stuff" {
		t.Fatalf("parse: %+v %v", p, err)
	}
	if want := "Load and follow the rtk skill (/cfg/builtin/rtk/SKILL.md) for this request: grep stuff"; p.Text != want {
		t.Fatalf("message %q, want %q", p.Text, want)
	}
	if p, _ := ParseInput("/rtk", nil, sk); p.Text != "Load and follow the rtk skill (/cfg/builtin/rtk/SKILL.md)." {
		t.Fatalf("bare invocation: %q", p.Text)
	}
	// Built-ins win over a same-named skill.
	if p, _ := ParseInput("/model x", nil, []skills.Skill{{Name: "model", Path: "/x"}}); p.Kind != KindCommand {
		t.Fatalf("a built-in must win: %+v", p)
	}
	// A prompt template wins over a same-named skill.
	prompts := []skills.Prompt{{Name: "review", Body: "Review $1"}}
	if p, _ := ParseInput("/review a.go", prompts, []skills.Skill{{Name: "review", Path: "/x"}}); p.Kind != KindPrompt {
		t.Fatalf("a prompt must win: %+v", p)
	}
	// /help lists the skills block; a name shadowed by a built-in or a
	// prompt is listed once (its winner).
	h := HelpText(nil, sk)
	if !strings.Contains(h, "skills (invoke with /name — the agent loads and follows them):") || !strings.Contains(h, "/rtk compressed CLI") {
		t.Fatalf("help must list the skill:\n%s", h)
	}
	shadowed := []skills.Skill{{Name: "model", Path: "/x"}, {Name: "review", Path: "/y"}, {Name: "rtk", Path: "/z"}}
	h = HelpText(prompts, shadowed)
	countLines := func(prefix string) int {
		n := 0
		for _, ln := range strings.Split(h, "\n") {
			if strings.HasPrefix(ln, prefix) {
				n++
			}
		}
		return n
	}
	if n := countLines("/model "); n != 1 {
		t.Fatalf("/model listed %d times:\n%s", n, h)
	}
	if n := countLines("/review "); n != 1 {
		t.Fatalf("/review listed %d times:\n%s", n, h)
	}
}

// /help lines up descriptions in one column per block, whatever the names are
// called: the built-ins and prompt templates share a column sized to the
// longest of them, the skills get their own under their heading. A fixed
// width used to push long names (`self-knowledge`, a long template) off it.
func TestHelpTextAlignsDescriptions(t *testing.T) {
	prompts := []skills.Prompt{
		{Name: "review", Description: "review a diff"},
		{Name: "a-very-long-template-name", ArgumentHint: "<x>", Description: "long"},
	}
	sk := []skills.Skill{
		{Name: "self-knowledge", Description: "about moca"},
		{Name: "rtk", Description: "compressed CLI"},
	}
	// descCol is the column the description starts in: past the name and the
	// padding after it.
	descCol := func(line string) int {
		name, rest, _ := strings.Cut(line, " ")
		return len(name) + 1 + len(rest) - len(strings.TrimLeft(rest, " "))
	}
	check := func(h string) {
		t.Helper()
		block, want, cmdCol := "commands", -1, -1
		for _, ln := range strings.Split(h, "\n") {
			switch {
			case strings.HasPrefix(ln, "skills ("):
				block, want = "skills", -1
			case strings.HasPrefix(ln, "!cmd"):
				// The shell line closes the listing in the command column.
				if col := descCol(ln); col != cmdCol {
					t.Errorf("!cmd line starts its description in column %d, want the commands' %d", col, cmdCol)
				}
			case strings.HasPrefix(ln, "/"):
				col := descCol(ln)
				if want == -1 {
					want = col
					if block == "commands" {
						cmdCol = col
					}
				} else if col != want {
					t.Errorf("%s block: %q starts its description in column %d, want %d", block, ln, col, want)
				}
			}
		}
	}
	check(HelpText(nil, nil))
	check(HelpText(prompts, sk))
	// The long template name widens the command block, not the skills block.
	h := HelpText(prompts, sk)
	if !strings.Contains(h, "/self-knowledge about moca") {
		t.Errorf("skills block must be sized to its own longest name:\n%s", h)
	}
}

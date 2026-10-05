package tui

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

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
	}
	for _, c := range cases {
		got, err := ParseInput(c.in, prompts)
		if err != nil || got != c.want {
			t.Errorf("ParseInput(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
	if _, err := ParseInput("/nope", prompts); err == nil || !strings.Contains(err.Error(), "/help") {
		t.Fatal(err)
	}
	if p, _ := ParseInput("/model", prompts); p.Kind != KindCommand {
		t.Fatal("built-ins win on collision")
	}
	if !strings.Contains(HelpText(prompts), "/review") {
		t.Fatal("help lists prompts")
	}
}

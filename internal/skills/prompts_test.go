package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrompts(t *testing.T) {
	proj, glob := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(proj, "review.md"), []byte("---\ndescription: review code\nargument-hint: <path>\n---\nReview $1 carefully. All: $ARGUMENTS"), 0o644)
	os.WriteFile(filepath.Join(glob, "review.md"), []byte("global"), 0o644)
	os.WriteFile(filepath.Join(glob, "plain.md"), []byte("No frontmatter $@"), 0o644)
	os.WriteFile(filepath.Join(glob, "skip.txt"), []byte("x"), 0o644)
	ps := LoadPrompts([]Dir{{proj, "project"}, {glob, "global"}})
	if len(ps) != 2 || ps[1].Name != "review" || ps[1].Description != "review code" || ps[0].Body != "No frontmatter $@" {
		t.Fatalf("%+v", ps)
	}
	if ps[1].Source != "project" || ps[1].ArgumentHint != "<path>" || ps[0].Source != "global" {
		t.Fatalf("sources: %+v", ps)
	}
}

func TestExpandPrompt(t *testing.T) {
	got := ExpandPrompt("Review $1 and $2; all=$ARGUMENTS; at=$@; none=$3; cost $$5", "a.go b.go")
	if got != "Review a.go and b.go; all=a.go b.go; at=a.go b.go; none=; cost $5" {
		t.Fatal(got)
	}
}

// Prompt templates written for Claude Code commands and pi load unchanged:
// unknown frontmatter keys (allowed-tools, model) are ignored, and the
// positional / $ARGUMENTS / $@ expansions match both ecosystems.
func TestEcosystemPromptsLoad(t *testing.T) {
	ps := LoadPrompts([]Dir{{Path: "testdata/prompts", Source: "global"}})
	if len(ps) != 2 {
		t.Fatalf("%+v", ps)
	}
	if ps[0].Name != "claude-review" || ps[0].Description != "Review a file for bugs" || ps[0].ArgumentHint != "[file]" {
		t.Fatalf("%+v", ps[0])
	}
	if ps[1].Name != "pi-commit" || ps[1].Description != "Write a commit message" {
		t.Fatalf("%+v", ps[1])
	}
	if got := ExpandPrompt(ps[0].Body, "main.go"); got != "Review main.go for bugs. Focus on: main.go\n" {
		t.Fatalf("%q", got)
	}
	if got := ExpandPrompt(ps[1].Body, "api fixes"); got != "Write a conventional commit message for the staged changes. Scope: api. Extra: api fixes\n" {
		t.Fatalf("%q", got)
	}
}

package skills

import (
	"os"
	"path/filepath"
	"slices"
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

func TestExpandPromptBraceForms(t *testing.T) {
	cases := []struct{ body, args, want string }{
		{"${1:+Focus on $1.}", "auth", "Focus on auth."},
		{"[${1:+Focus on $1.}]", "", "[]"},
		{"${1:-all files}", "", "all files"},
		{"${1:-all files}", "x.go", "x.go"},
		{"${2-none}|${2:-empty}", `a ""`, "|empty"}, // $2 set but empty
		{"${2+set}|${2:+nonempty}", `a ""`, "set|"},
		{"${@:2}", "a b c", "b c"},
		{"${@:2:1}", "a b c", "b"},
		{"${@:0}", "a b", "a b"}, // bash: 0 treated as 1 for the slice start (pi parity)
		{"${10}", "1 2 3 4 5 6 7 8 9 ten", "ten"},
		{"$10", "one", "one0"}, // bash semantics, not pi's
		{"${ARGUMENTS:-nothing}", "", "nothing"},
		{"${@:+args: $@}", "x y", "args: x y"},
		{"keep ${HOME} and ${FOO:-x}", "a", "keep ${HOME} and ${FOO:-x}"},
		{"unterminated ${1:+oops", "a", "unterminated ${1:+oops"},
		{`$1|$2`, `"a b" c`, "a b|c"}, // quote-aware split
		{"${1:+$1 $$}", "v", "v $"},
		{"${1:-$2}", " x", "x"},      // nested reference in the default word
		{"${1:+$@}", "$2 z", "$2 z"}, // values are not re-expanded
	}
	for _, c := range cases {
		if got := ExpandPrompt(c.body, c.args); got != c.want {
			t.Errorf("ExpandPrompt(%q, %q) = %q, want %q", c.body, c.args, got, c.want)
		}
	}
}

func TestSplitArgs(t *testing.T) {
	got := SplitArgs(`a "b c" 'd e' f""g ""`)
	want := []string{"a", "b c", "d e", "fg", ""}
	if !slices.Equal(got, want) {
		t.Fatalf("SplitArgs = %q, want %q", got, want)
	}
}

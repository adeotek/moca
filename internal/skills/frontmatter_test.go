package skills

import "testing"

func TestFrontmatter(t *testing.T) {
	src := "\ufeff---\r\nname: graphify\r\ndescription: >\r\n  Build a knowledge\r\n  graph of code.\r\nallowed-tools: Bash\r\nmetadata:\r\n  version: 2\r\n  tags: [a, b]\r\nargument-hint: \"[path]\"\r\n---\r\n# Body\r\n"
	fm, body, err := ParseFrontmatter([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if fm["name"] != "graphify" || fm["description"] != "Build a knowledge graph of code." || fm["argument-hint"] != "[path]" {
		t.Fatalf("%#v", fm)
	}
	if _, ok := fm["version"]; ok {
		t.Fatal("nested keys must not leak to top level")
	}
	if string(body) != "# Body\n" {
		t.Fatalf("%q", body)
	}
	fm, _, _ = ParseFrontmatter([]byte("---\ndescription: |\n  line one\n  line two\nname: 'x'\n---\n"))
	if fm["description"] != "line one\nline two" || fm["name"] != "x" {
		t.Fatalf("%#v", fm)
	}
	if _, _, err := ParseFrontmatter([]byte("no frontmatter")); err == nil {
		t.Fatal("missing frontmatter is an error")
	}
}

// Block scalars and quoted values, pinned by the ecosystem corpus: folded
// text collapses to one space (even across a blank line), literal keeps its
// newlines, double quotes unescape Go-style when they form a valid quoted
// string, and a doubled single quote is one quote.
func TestFrontmatterBlockAndQuoteEdges(t *testing.T) {
	fm, _, err := ParseFrontmatter([]byte("---\ndescription: >\n  one line\n\n  second paragraph\nname: x\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if fm["description"] != "one line second paragraph" {
		t.Fatalf("folded with a blank line: %q", fm["description"])
	}
	fm, _, _ = ParseFrontmatter([]byte("---\ndescription: |\n  one line\n\n  second paragraph\n---\n"))
	if fm["description"] != "one line\n\nsecond paragraph" {
		t.Fatalf("literal with a blank line: %q", fm["description"])
	}
	fm, _, _ = ParseFrontmatter([]byte("---\ndescription: \"multi \\\"quoted\\\" tail\"\n---\n"))
	if fm["description"] != `multi "quoted" tail` {
		t.Fatalf("escaped double quotes: %q", fm["description"])
	}
	fm, _, _ = ParseFrontmatter([]byte("---\ndescription: \"C:\\Users\\x\"\n---\n"))
	if fm["description"] != `C:\Users\x` {
		t.Fatalf("invalid escape falls back to raw: %q", fm["description"])
	}
	fm, _, _ = ParseFrontmatter([]byte("---\ndescription: 'it''s fine'\n---\n"))
	if fm["description"] != "it's fine" {
		t.Fatalf("single-quote escape: %q", fm["description"])
	}
}

// Block-scalar headers carry chomping/indent indicators and comments in real
// SKILL.md files; the indicator must never become the description (review
// pass 3), and a YAML ` # comment` after a plain or quoted value is dropped.
func TestFrontmatterBlockIndicatorsAndComments(t *testing.T) {
	cases := map[string]string{
		"description: >+\n  folded keep\n  second\n":   "folded keep second",
		"description: |+\n  literal keep\n  two\n":     "literal keep\ntwo",
		"description: >2\n  indented\n":                "indented",
		"description: |2-\n  a\n  b\n":                 "a\nb",
		"description: > # note\n  text\n":              "text",
		"description: plain text # trailing comment\n": "plain text",
		"description: \"quoted\" # trailing comment\n": "quoted",
		"description: 'it''s' # c\n":                   "it's",
		"description: C# is fine\n":                    "C# is fine",
		"description: \"a # b\"\n":                     "a # b",
		"description: Use when: the user asks\n":       "Use when: the user asks",
		"description: #only a comment\nname: x\n":      "",
	}
	for in, want := range cases {
		fm, _, err := ParseFrontmatter([]byte("---\n" + in + "---\nbody"))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if fm["description"] != want {
			t.Errorf("%q: description %q, want %q", in, fm["description"], want)
		}
	}
}

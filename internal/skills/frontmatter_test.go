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

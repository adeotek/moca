package tools

import "testing"

func TestIgnore(t *testing.T) {
	rules := parseIgnore("", []byte("# comment\n*.log\n/build\nnode_modules/\n!keep.log\ndocs/**/*.tmp\n\\#hash\n"))
	sub := parseIgnore("pkg", []byte("gen.go\n"))
	all := append(rules, sub...)
	cases := []struct {
		rel  string
		dir  bool
		want bool
	}{
		{"a.log", false, true}, {"x/y/a.log", false, true}, {"keep.log", false, false},
		{"build", true, true}, {"x/build", true, false},
		{"node_modules", true, true}, {"a/node_modules", true, true}, {"node_modules", false, false},
		{"docs/a/b/c.tmp", false, true}, {"docs/c.tmp", false, true}, {"other/c.tmp", false, false},
		{"#hash", false, true},
		{"pkg/gen.go", false, true}, {"pkg/deep/gen.go", false, true}, {"gen.go", false, false},
	}
	for _, c := range cases {
		if got := ignored(all, c.rel, c.dir); got != c.want {
			t.Errorf("ignored(%q, dir=%v) = %v, want %v", c.rel, c.dir, got, c.want)
		}
	}
}

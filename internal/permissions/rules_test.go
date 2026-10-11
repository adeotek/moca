package permissions

import "testing"

func lit(ws ...string) []Word {
	out := make([]Word, len(ws))
	for i, w := range ws {
		out[i] = Word{Text: w, Literal: true}
	}
	return out
}

func TestRuleMatch(t *testing.T) {
	cases := []struct {
		rule string
		cmd  []Word
		want bool
	}{
		{"rtk", lit("rtk"), true},
		{"rtk", lit("rtk", "git", "log"), true},
		{"rtk *", lit("rtk"), true},
		{"rtk rm *", lit("rtk", "rm", "-rf", "x"), true},
		{"rtk rm *", lit("rtk", "git", "rm"), false},
		{"git status", lit("git", "status"), true},
		{"git status", lit("git", "status", "-s"), false},
		{"git * --force*", lit("git", "push", "origin", "--force-with-lease"), true},
		{"go test ./...", lit("/usr/local/go/bin/go", "test", "./..."), true},
		{"npm run test:*", lit("npm", "run", "test:unit"), true},
		{"*", lit("anything", "at", "all"), true},
	}
	for _, c := range cases {
		r, err := ParseRule(c.rule)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.match(c.cmd, false); got != c.want {
			t.Errorf("%q vs %v = %v, want %v", c.rule, c.cmd, got, c.want)
		}
	}
}

func TestRuleNonLiteralIsConservative(t *testing.T) {
	r, _ := ParseRule("git push *")
	cmd := []Word{{"git", true}, {"$SUB", false}, {"origin", true}}
	if !r.match(cmd, true) {
		t.Error("deny/ask must treat $SUB as possibly `push`")
	}
	if r.match(cmd, false) {
		t.Error("allow must not treat $SUB as `push`")
	}
	any, _ := ParseRule("rm *")
	if !any.match([]Word{{"rm", true}, {"*.o", false}}, false) {
		t.Error("a standalone * matches non-literal words even for allow")
	}
}

func TestRuleCase(t *testing.T) {
	r, _ := ParseRule("git push *")
	if !r.match(lit("GIT", "push"), true) {
		t.Error("deny/ask matches the command name case-insensitively")
	}
	if r.match(lit("GIT", "push"), false) {
		t.Error("allow is case-sensitive")
	}
}

func TestParseRuleErrors(t *testing.T) {
	for _, s := range []string{"", "   "} {
		if _, err := ParseRule(s); err == nil {
			t.Errorf("ParseRule(%q) should fail", s)
		}
	}
}

func TestRulesPrecedence(t *testing.T) {
	proj, _ := ParseRuleSet("project", []string{"docker compose *"}, nil, []string{"git push *"})
	user, _ := ParseRuleSet("user", []string{"git *"}, []string{"git commit *"}, []string{"docker *"})
	r := NewRules(proj, user)
	cases := []struct {
		cmd   []Word
		want  Decision
		layer string
	}{
		{lit("git", "push", "origin"), Deny, "project"},    // project deny beats user allow
		{lit("docker", "compose", "up"), Allow, "project"}, // project allow overrides user deny
		{lit("docker", "run", "x"), Deny, "user"},
		{lit("git", "commit", "-m", "x"), Ask, "user"}, // ask beats allow within a layer
		{lit("git", "log"), Allow, "user"},
		{lit("python", "x.py"), NoMatch, ""},
	}
	for _, c := range cases {
		d, layer, _ := r.Decide(c.cmd)
		if d != c.want || layer != c.layer {
			t.Errorf("%v → %v/%q, want %v/%q", c.cmd, d, layer, c.want, c.layer)
		}
	}
	r.Learn("python")
	if d, layer, _ := r.Decide(lit("python", "x.py")); d != Allow || layer != "user" {
		t.Errorf("learned allow → %v/%q", d, layer)
	}
}

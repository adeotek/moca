package permissions

import (
	"strings"
	"testing"
	"time"
)

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
		{"go test ./...", lit("/usr/local/go/bin/go", "test", "./..."), false},
		{"rm*", lit("rm", "-rf", "x"), true},
		{"rm*", lit("rm"), true},
		{"mkfs*", lit("mkfs.ext4", "/dev/sda"), true},
		{"/usr/bin/git *", lit("/usr/bin/git", "status"), true},
		{"git *", lit("./git", "status"), false},
		{"git *", lit("/tmp/x/git", "status"), false},
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

func TestRuleConservativeHeads(t *testing.T) {
	cases := []struct {
		rule         string
		cmd          []Word
		conservative bool
		want         bool
	}{
		{"go test ./...", lit("/usr/local/go/bin/go", "test", "./..."), true, true},
		{"go test ./...", lit("/usr/local/go/bin/go", "test", "./..."), false, false},
		{"git push *", lit("./git", "push", "origin"), true, true},
		{"git push *", lit("/tmp/x/git", "push", "origin"), true, true},
		{"git push *", lit("./git", "push", "origin"), false, false},
		{"git *", lit("./git", "status"), true, true},
		{"/usr/bin/rm *", lit("rm", "-rf", "x"), true, true},
		{"/usr/bin/rm *", lit("rm", "-rf", "x"), false, false},
		{"/usr/bin/git *", lit("/usr/bin/git", "status"), false, true},
		{"/usr/bin/git *", lit("/usr/bin/git", "status"), true, true},
	}
	for _, c := range cases {
		r, err := ParseRule(c.rule)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.match(c.cmd, c.conservative); got != c.want {
			t.Errorf("%q vs %v (conservative=%v) = %v, want %v", c.rule, c.cmd, c.conservative, got, c.want)
		}
	}
}

func TestRuleNonLiteralMultiWord(t *testing.T) {
	cases := []struct {
		rule         string
		cmd          []Word
		conservative bool
		want         bool
	}{
		{"git push --force", []Word{{"git", true}, {"$A", false}}, true, true},
		{"git push --force", []Word{{"git", true}, {"$A", false}}, false, false},
		{"git push *", []Word{{"git", true}, {"$X", false}, {"origin", true}}, true, true},
		{"git push *", []Word{{"git", true}, {"$X", false}, {"origin", true}}, false, false},
		{"rm -rf *", []Word{{"rm", true}, {"$F", false}}, true, true},
		{"rm -rf *", []Word{{"rm", true}, {"$F", false}}, false, false},
		{"git status", []Word{{"git", true}, {"$A", false}}, true, true},
		{"git status", []Word{{"git", true}, {"$A", false}}, false, false},
	}
	for _, c := range cases {
		r, err := ParseRule(c.rule)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.match(c.cmd, c.conservative); got != c.want {
			t.Errorf("%q vs %v (conservative=%v) = %v, want %v", c.rule, c.cmd, c.conservative, got, c.want)
		}
	}
}

func TestLearnIsOneLiteralWord(t *testing.T) {
	for _, name := range []string{"", "*", "a b", "a\tb", "a*"} {
		r := NewRules(RuleSet{Name: "user"})
		r.Learn(name)
		for _, cmd := range [][]Word{lit("a"), lit("a", "b"), lit("a*")} {
			if d, _, _ := r.Decide(cmd); d != NoMatch {
				t.Errorf("Learn(%q) then Decide(%v) = %v, want NoMatch", name, cmd, d)
			}
		}
	}
}

func TestParseRuleCollapsesStars(t *testing.T) {
	r, err := ParseRule(strings.Repeat("* ", 10) + "a")
	if err != nil {
		t.Fatal(err)
	}
	cmd := make([]Word, 60)
	for i := range cmd {
		cmd[i] = Word{Text: "w", Literal: true}
	}
	start := time.Now()
	if r.match(cmd, false) {
		t.Error("pattern * * ... a must not match 60 words without a")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("match took %v, want under 1s", elapsed)
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

// matchWithin runs r.match and fails if it does not return within budget.
func matchWithin(t *testing.T, r Rule, cmd []Word, conservative bool, budget time.Duration) bool {
	t.Helper()
	done := make(chan bool, 1)
	go func() { done <- r.match(cmd, conservative) }()
	select {
	case got := <-done:
		return got
	case <-time.After(budget):
		t.Fatalf("match of %q against %d words did not finish within %v", r.Src, len(cmd), budget)
		return false
	}
}

func TestRuleMatchIsPolynomial(t *testing.T) {
	const budget = time.Second
	nonLits := func(n int) []Word {
		out := make([]Word, n)
		for i := range out {
			out[i] = Word{Text: "$N", Literal: false}
		}
		return out
	}
	literals := func(n int) []Word {
		out := make([]Word, n)
		for i := range out {
			out[i] = Word{Text: "w", Literal: true}
		}
		return out
	}
	cases := []struct {
		rule         string
		cmd          []Word
		conservative bool
		want         bool
	}{
		// 40 non-literal words then a literal: conservative may read $N as b or x or y.
		{"a * * b *", append(append([]Word{{"a", true}}, nonLits(40)...), Word{"z", true}), true, true},
		{"a * * b *", append(append([]Word{{"a", true}}, nonLits(40)...), Word{"z", true}), false, false},
		{"* x * y * z", append(nonLits(40), Word{"z", true}), true, true},
		{"* x * y * z", append(nonLits(40), Word{"z", true}), false, false},
		// Must explore every split of the 40 non-literal words, then fail on the
		// trailing literal (no pattern word can follow it to absorb it).
		{"* a * b * c * d * e", append(nonLits(40), Word{"q", true}), true, false},
		{"* a * b * c * d * e", append(nonLits(40), Word{"q", true}), false, false},
		// Literal-only worst cases that fail after exploring every split.
		{"* x * y * z", literals(60), true, false},
		{"* * * a", literals(200), true, false},
		{"* * * a", literals(200), false, false},
		{"a * b * c * d * e", append([]Word{{"a", true}}, literals(199)...), true, false},
		{"a * b * c * d * e", append([]Word{{"a", true}}, literals(199)...), false, false},
	}
	for _, c := range cases {
		r, err := ParseRule(c.rule)
		if err != nil {
			t.Fatal(err)
		}
		if got := matchWithin(t, r, c.cmd, c.conservative, budget); got != c.want {
			t.Errorf("%q vs %d words (conservative=%v) = %v, want %v", c.rule, len(c.cmd), c.conservative, got, c.want)
		}
	}
}

func TestRuleWideningReview(t *testing.T) {
	cases := []struct {
		rule         string
		cmd          []Word
		conservative bool
		want         bool
	}{
		// A deny/ask first word that Base reduces to a bare * matches any command.
		{"*/*", lit("git", "status"), true, true},
		{"*/*", lit("git", "status"), false, false},
		{"/*", lit("git", "status"), true, true},
		{"/*", lit("git", "status"), false, false},
		{"*/", lit("git"), true, true},
		{"*/", lit("git"), false, false},
		// Allow never takes Base: a bare-name rule does not cover a path head.
		{"git", lit("/usr/bin/git"), false, false},
		{"git", lit("/usr/bin/git"), true, true},
		{"git *", lit("C:\\bin\\git", "status"), false, false},
		{"git *", lit("C:\\bin\\git", "status"), true, true},
		{"/usr/bin/git *", lit("git", "status"), false, false},
		{"/usr/bin/git *", lit("/usr/bin/git", "status"), false, true},
	}
	for _, c := range cases {
		r, err := ParseRule(c.rule)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.match(c.cmd, c.conservative); got != c.want {
			t.Errorf("%q vs %v (conservative=%v) = %v, want %v", c.rule, c.cmd, c.conservative, got, c.want)
		}
	}
}

func TestLearnPathRecordsExactHead(t *testing.T) {
	for _, name := range []string{"./git", "/usr/bin/git"} {
		r := NewRules(RuleSet{Name: "user"})
		r.Learn(name)
		if d, _, _ := r.Decide(lit("git", "status")); d != NoMatch {
			t.Errorf("Learn(%q) then Decide(git status) = %v, want NoMatch", name, d)
		}
		if d, _, _ := r.Decide(lit("/tmp/x/git", "status")); d != NoMatch {
			t.Errorf("Learn(%q) then Decide(/tmp/x/git status) = %v, want NoMatch", name, d)
		}
		if d, layer, _ := r.Decide(lit(name, "status")); d != Allow || layer != "user" {
			t.Errorf("Learn(%q) then Decide(%s status) = %v/%q, want Allow/user", name, name, d, layer)
		}
	}
}

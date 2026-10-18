package permissions

import (
	"strings"
	"testing"
)

// The v0.7.0 §11 follow-up: the allow list names tools, and some of those
// tools run *arbitrary project code* chosen by their arguments — a Makefile
// target, a script file, an inline program. This pins the state that shipped:
// `make` is judged (a bare entry no longer covers a recipe of its choosing),
// while the interpreter shapes below still run unapproved and are listed as an
// open item in SPECS §15 — a change there has to be a deliberate policy call
// (a `-p` run has no asker, so asking means refusing, which breaks legitimate
// interpreter workflows). If one of these rows changes, update §15 with it.
func TestGenericRunnerAllowRules(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	s := NewShell(RulesFromAllow([]string{"make", "python", "python3", "node", "npm", "pytest", "docker"}), j, "linux")

	allowed := []string{"make test", "make -j4 build", "make VAR=1 check", "make fmt vet"}
	for _, cmd := range allowed {
		if need, every, unsure, err := s.Check(cmd); err != nil || len(need)+len(every)+len(unsure) > 0 {
			t.Errorf("%s: want allowed with no prompt, got need=%v every=%v unsure=%v err=%v", cmd, need, every, unsure, err)
		}
	}

	refused := map[string]string{
		"make bootstrap":        "runs a recipe from the repository's makefile",
		"make":                  "runs the makefile's default recipe",
		"make -f other.mk test": "runs a makefile of its own choosing",
		"make $TARGET":          "computed target",
	}
	for cmd, want := range refused {
		_, _, _, err := s.Check(cmd)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want a refusal containing %q, got %v", cmd, want, err)
		}
	}

	// A rule that names the recipe covers it; the bare entry does not.
	named := NewShell(NewRules(RuleSet{Name: "user", Allow: mustRules(t, "make", "make bootstrap")}), j, "linux")
	if _, _, _, err := named.Check("make bootstrap"); err != nil {
		t.Errorf("an explicit `make bootstrap` rule must cover it, got %v", err)
	}

	// Open item (SPECS §15): these still reach the tool unapproved.
	for _, cmd := range []string{"python3 -c 'print(1)'", "python3 tools/gen.py", "node -e 'x'", "npm run build", "pytest", "docker run --rm alpine sh"} {
		if need, every, unsure, err := s.Check(cmd); err != nil || len(need)+len(every)+len(unsure) > 0 {
			t.Logf("open item changed: %s now need=%v every=%v unsure=%v err=%v (update SPECS §15)", cmd, need, every, unsure, err)
		}
	}
}

func mustRules(t *testing.T, srcs ...string) []Rule {
	t.Helper()
	out := make([]Rule, 0, len(srcs))
	for _, s := range srcs {
		r, err := ParseRule(s)
		if err != nil {
			t.Fatalf("ParseRule(%q): %v", s, err)
		}
		out = append(out, r)
	}
	return out
}

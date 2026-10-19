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
		// -C/--directory outside the workdir points at a makefile of its own
		// choosing too: the value may not be a literal path elsewhere.
		"make -C ../sibling test":        "of its own choosing",
		"make --directory=/sneaky build": "of its own choosing",
		"make -C/tmp/evil build":         "of its own choosing",
		"make --dir /tmp/evil lint":      "of its own choosing",
		"cd /tmp && make -C sub test":    "new directory's choosing",
		"cd /tmp && make test":           "new directory's choosing",
	}
	for cmd, want := range refused {
		_, _, _, err := s.Check(cmd)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want a refusal containing %q, got %v", cmd, want, err)
		}
	}

	// Every target counts, and the options/overrides that hand make code of
	// their own are refused even next to a safe target.
	for cmd, want := range map[string]string{
		"make bootstrap test":               "make bootstrap",
		"make test SHELL=./evil.sh":         "chooses code make runs",
		"make 'X:=$(shell touch p)' test":   "chooses code make runs",
		"make 'X!=touch p' test":            "chooses code make runs",
		"make --eval='x:;touch pwned' test": "of its own choosing",
		"make -E 'x:' test":                 "of its own choosing",
		"make -kf other.mk test":            "of its own choosing",
		"make -$X test":                     "computed target",
		// An option that takes an argument may not be handed a computed one:
		// consuming `$DIR` unexamined would hide the directory make reads the
		// makefile from (the argument was refused before these options were
		// understood, so this is a regression pin).
		"make -C $DIR test":                  "computed argument",
		"make --dir $DIR test":               "computed argument",
		"make --directory $DIR test":         "computed argument",
		"make -I $DIR test":                  "computed argument",
		"make --in=x test --file=other.mk":   "of its own choosing",
		"make test --makefile other.mk lint": "of its own choosing",
		// Any variable a recipe runs can be overridden (this repo's own
		// `test:` runs $(GORUN)): only plain data values are covered.
		"make test GORUN=./scripts/bootstrap.sh":         "chooses code make runs",
		"make test 'GO=sh scripts/bootstrap.sh'":         "chooses code make runs",
		"make build CFLAGS=-B.":                          "chooses code make runs",
		"make test GOFLAGS=-toolexec=./evil":             "chooses code make runs",
		"MAKEFILES=evil.mk make test":                    "changed environment",
		"env MAKEFILES=evil.mk make test":                "changed environment",
		"export MAKEFLAGS=--eval=x; make test":           "changed environment",
		"MAKEFILES=evil.mk; export MAKEFILES; make test": "changed environment",
	} {
		_, _, _, err := s.Check(cmd)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want a refusal containing %q, got %v", cmd, want, err)
		}
	}
	// Option arguments (abbreviated long names included) are not targets, and
	// a plain-data override stays covered.
	for _, cmd := range []string{"make test -j 4", "make -j test", "make --jobs 8 lint", "make -C sub test",
		"make --dir sub test", "make --jo 2 test", "make -l 2.5 test", "make --load-average=2.5 test",
		"make -o build test", "make -W stale.mk lint", "make build VERSION=1.2.3", "make test V="} {
		if _, _, _, err := s.Check(cmd); err != nil {
			t.Errorf("%s: want allowed, got %v", cmd, err)
		}
	}
	// `make *` is the same rule as `make` (ParseRule): it names only the tool.
	star := NewShell(NewRules(RuleSet{Name: "user", Allow: mustRules(t, "make *")}), j, "linux")
	if _, _, _, err := star.Check("make bootstrap"); err == nil {
		t.Error("`make *` must not cover `make bootstrap`")
	}

	// A rule that names the recipe covers it; the bare entry does not.
	named := NewShell(NewRules(RuleSet{Name: "user", Allow: mustRules(t, "make", "make bootstrap")}), j, "linux")
	if _, _, _, err := named.Check("make bootstrap"); err != nil {
		t.Errorf("an explicit `make bootstrap` rule must cover it, got %v", err)
	}
	// A named rule waives only the target list: the option- and
	// environment-level checks stay unconditional (`*` rides anywhere inside
	// a named rule's own words, and must not buy its way past -f).
	if _, _, _, err := named.Check("make -f evil.mk bootstrap"); err == nil || !strings.Contains(err.Error(), "of its own choosing") {
		t.Errorf("a named rule must not cover a makefile of its own choosing, got %v", err)
	}
	if _, _, _, err := named.Check("cd /tmp && make bootstrap"); err == nil || !strings.Contains(err.Error(), "directory's choosing") {
		t.Errorf("a named rule must not cover a makefile picked by a changed directory, got %v", err)
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

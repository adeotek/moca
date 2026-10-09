package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DetectVerify finds the project's check command once at session start
// (§5): the system prompt names it, so the model verifies the way the
// project does instead of guessing. The project's own entry points win
// (Makefile targets, package.json scripts) over ecosystem defaults. It
// returns "" when nothing is recognized.
func DetectVerify(dir string) (command, source string) {
	exists := func(name string) bool { _, err := os.Stat(filepath.Join(dir, name)); return err == nil }
	for _, mf := range []string{"GNUmakefile", "makefile", "Makefile"} {
		if !exists(mf) {
			continue
		}
		t := makeTargets(filepath.Join(dir, mf))
		var parts []string
		for _, c := range []string{"vet", "lint", "check"} {
			if t[c] {
				parts = append(parts, "make "+c)
				break
			}
		}
		if t["test"] {
			parts = append(parts, "make test")
		}
		if len(parts) > 0 {
			return strings.Join(parts, " && "), mf
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(b, &pkg) == nil && len(pkg.Scripts) > 0 {
			runner := "npm"
			switch {
			case exists("pnpm-lock.yaml"):
				runner = "pnpm"
			case exists("yarn.lock"):
				runner = "yarn"
			case exists("bun.lockb"), exists("bun.lock"):
				runner = "bun"
			}
			var parts []string
			for _, s := range []string{"lint", "typecheck"} {
				if _, ok := pkg.Scripts[s]; ok {
					parts = append(parts, runner+" run "+s)
				}
			}
			if _, ok := pkg.Scripts["test"]; ok {
				parts = append(parts, runner+" test")
			}
			if len(parts) > 0 {
				return strings.Join(parts, " && "), "package.json"
			}
		}
	}
	switch {
	case exists("go.mod"):
		return "go vet ./... && go test ./...", "go.mod"
	case exists("Cargo.toml"):
		return "cargo check && cargo test", "Cargo.toml"
	case exists("pyproject.toml"), exists("pytest.ini"), exists("setup.py"), exists("tox.ini"):
		src := "pyproject.toml"
		for _, f := range []string{"pyproject.toml", "pytest.ini", "setup.py", "tox.ini"} {
			if exists(f) {
				src = f
				break
			}
		}
		if exists("ruff.toml") || fileContains(filepath.Join(dir, "pyproject.toml"), "[tool.ruff") {
			return "ruff check . && pytest", src
		}
		return "pytest", src
	case exists("gradlew"):
		return "./gradlew test", "gradlew"
	case exists("pom.xml"):
		return "mvn -q test", "pom.xml"
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.sln")); len(m) > 0 {
		return "dotnet build && dotnet test", filepath.Base(m[0])
	}
	return "", ""
}

var makeTargetRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*:([^=]|$)`)

// makeTargets lists the explicit targets of a makefile (no includes, no
// pattern rules — enough to see test/vet/lint/check).
func makeTargets(path string) map[string]bool {
	out := map[string]bool{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := makeTargetRe.FindStringSubmatch(sc.Text()); m != nil {
			out[m[1]] = true
		}
	}
	return out
}

func fileContains(path, sub string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), sub)
}

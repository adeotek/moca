package main

// Two measured numbers in the docs are hand-written, and both drifted in the
// pass that added provider model discovery: AGENTS.md counted 434 Go files
// when the tree held 436, and SPECS §15's test count was written two tests
// before the last two landed. These pins make the next drift a red test
// instead of a review catch.
//
// The `=== RUN` count itself cannot be pinned here — it includes every row a
// table-driven `t.Run` produces, so it is only knowable by running the suite.
// `counts_run_test.go` checks it under the `counts` build tag, the same
// on-demand pattern skillgate and acpgate use.

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// repoRoot is the module root: this package lives in cmd/moca.
const repoRoot = "../.."

// out runs a command in the repository root and returns its stdout. A missing
// tool is a skip, not a failure: the pin is about the docs, and a machine
// without git or the toolchain on PATH cannot measure them.
func out(t *testing.T, skipOnMissing, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = repoRoot
	b, err := cmd.Output()
	if err != nil {
		if _, isExecErr := err.(*exec.ExitError); !isExecErr {
			t.Skipf("%s not available in this environment: %v", skipOnMissing, err)
		}
		t.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return string(b)
}

// agentsClaims reads AGENTS.md's opening "N Go files" count.
var agentsFiles = regexp.MustCompile(`(\d+) Go files across (\d+) ` + "`internal/`" + ` packages`)

// specsClaims reads SPECS §15's headline "N tests across M Go packages".
var specsCounts = regexp.MustCompile(`\*\*(\d+) tests across (\d+) Go packages\*\*`)

// getInts applies one pattern to a repository file and returns the numbers it
// captured.
func getInts(t *testing.T, re *regexp.Regexp, parts ...string) []int {
	t.Helper()
	m := re.FindStringSubmatch(repoFile(t, parts...))
	if m == nil {
		t.Fatalf("%s: no %q claim to pin (the wording changed — update this pin with it)", strings.Join(parts, "/"), re.String())
	}
	nums := make([]int, 0, len(m)-1)
	for _, g := range m[1:] {
		n, err := strconv.Atoi(g)
		if err != nil {
			t.Fatalf("%s: %q is not a number: %v", strings.Join(parts, "/"), g, err)
		}
		nums = append(nums, n)
	}
	return nums
}

// TestAgentsGoFileCount keeps AGENTS.md's file count honest: it is measured
// with `git ls-files '*.go'`, so an untracked scratch file does not move it —
// only a file the change actually adds does, which is what the doc counts.
func TestAgentsGoFileCount(t *testing.T) {
	want := getInts(t, agentsFiles, "AGENTS.md")[0]
	tracked := 0
	for _, f := range strings.Split(out(t, "git", "git", "ls-files", "*.go"), "\n") {
		if strings.HasSuffix(f, ".go") {
			tracked++
		}
	}
	if tracked != want {
		t.Fatalf("AGENTS.md says %d Go files, the tree tracks %d — update the opening line of AGENTS.md", want, tracked)
	}
}

// TestSpecsPackageCount pins the package half of SPECS §15's claim. `go list`
// is the only sound way to count them: it skips a directory whose files are
// all excluded by a build tag (the hermetic gate checkers) and anything under
// testdata, which no filesystem walk can reproduce.
func TestSpecsPackageCount(t *testing.T) {
	want := getInts(t, specsCounts, "docs", "specs", "SPECS.md")[1]
	pkgs := 0
	for _, line := range strings.Split(out(t, "the Go toolchain", "go", "list", "./..."), "\n") {
		if strings.TrimSpace(line) != "" {
			pkgs++
		}
	}
	if pkgs != want {
		t.Fatalf("SPECS §15 says %d Go packages, `go list ./...` finds %d — update the claim", want, pkgs)
	}
}

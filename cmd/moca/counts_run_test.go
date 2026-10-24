//go:build counts

package main

// The "N tests" half of SPECS §15 is the `=== RUN` count — subtests
// included — so it is only knowable by running the suite, which an ordinary
// test cannot do to itself. Like the skillgate and acpgate checkers this file
// sits behind a build tag and runs on demand, after any pass that added or
// removed tests:
//
//	go test -tags counts ./cmd/moca -run TestSpecsTestCount -v
//
// It re-runs the suite with the exact command SPECS §15 documents, so the
// number it compares against is the number the doc names.

import (
	"strings"
	"testing"
)

func TestSpecsTestCount(t *testing.T) {
	want := getInts(t, specsCounts, "docs", "specs", "SPECS.md")[0]
	// No `counts` tag on the child: this test is not compiled into it, so the
	// run cannot recurse into itself.
	stdout := out(t, "the Go toolchain", "go", "test", "./...", "-count=1", "-v")
	got := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "=== RUN") {
			got++
		}
	}
	if got != want {
		t.Fatalf("SPECS §15 says %d tests, the suite ran %d `=== RUN` lines — update the claim's headline and add this pass's milestone", want, got)
	}
}

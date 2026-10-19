package calc

import (
	"os"
	"testing"
)

// TestStrict refuses to run outside the repo's own runner: the fixtures need
// CALC_STRICT=1, which only `make test` sets (see the Makefile).
func TestStrict(t *testing.T) {
	if os.Getenv("CALC_STRICT") != "1" {
		t.Fatal("the fixtures need CALC_STRICT=1 — run the suite through `make test` (see the Makefile)")
	}
	if Add(2, 2) != 4 {
		t.Fatal("Add(2, 2) != 4")
	}
}

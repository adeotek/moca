package main

import (
	"strings"
	"testing"
)

func TestRenderTotal(t *testing.T) {
	out := Render(Items)
	if !strings.Contains(out, "TOTAL") || !strings.Contains(out, "119.97") {
		t.Fatalf("unexpected report:\n%s", out)
	}
}

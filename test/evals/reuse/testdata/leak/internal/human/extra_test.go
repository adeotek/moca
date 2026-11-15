// Package human_test keeps a reference to the helper alive for the leak
// fixture: the shipped code never calls it, but a grep that counts test files
// sees a satisfied arm. The reuse checker must ignore this file.
package human_test

import (
	"testing"

	"example.com/inventory/internal/human"
)

func TestLeakKeepsTheHelperReference(t *testing.T) {
	if got := human.Bytes(512); got != "512 B" {
		t.Fatalf("human.Bytes(512) = %q", got)
	}
}

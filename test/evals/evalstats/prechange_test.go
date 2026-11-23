package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The committed goldens must match a fresh capture: this is the pin Tasks 3, 4
// and 8 compare against. A failure here means the prompt, the tool schemas or
// the direct tools' output moved — deliberately or not.
func TestPrechangeGoldensAreCurrent(t *testing.T) {
	if err := CheckPrechange(); err != nil {
		t.Fatalf("%v\n\nif the move is intended, re-capture with: go run ./test/evals/evalstats -prechange", err)
	}
}

// A capture is reproducible: two runs render identical bytes. Without this,
// the pin above would be a flake generator (the direct-output cases build
// their fixture in a temp dir).
func TestPrechangeCaptureIsDeterministic(t *testing.T) {
	a, err := CapturePrechange()
	if err != nil {
		t.Fatal(err)
	}
	b, err := CapturePrechange()
	if err != nil {
		t.Fatal(err)
	}
	for rel := range a {
		if string(a[rel]) != string(b[rel]) {
			t.Fatalf("%s differs between two captures", rel)
		}
	}
}

// The pin has to FAIL on a wrong expectation, or it proves nothing. Mutating
// one byte of a written capture is the smallest honest arm.
func TestPrechangeRejectsAWrongExpectation(t *testing.T) {
	files, err := CapturePrechange()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := writeGoldens(dir, files); err != nil {
		t.Fatal(err)
	}
	if err := checkGoldens(dir, files); err != nil {
		t.Fatalf("a fresh capture must match itself: %v", err)
	}
	// One byte in the rendered prompt: the golden is now a wrong expectation.
	mutated := append([]byte("x"), files[prechangePrompt]...)
	if err := os.WriteFile(filepath.Join(dir, prechangePrompt), mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	err = checkGoldens(dir, files)
	if err == nil {
		t.Fatal("a mutated golden passed the check")
	}
	if !strings.Contains(err.Error(), "test/evals/testdata/prechange/system-prompt.txt") ||
		!strings.Contains(err.Error(), "first difference at line 1") {
		t.Fatalf("the failure must name the file and the first difference: %v", err)
	}
	// A missing golden is a failure too — never "nothing to compare".
	if err := os.Remove(filepath.Join(dir, prechangeWire)); err != nil {
		t.Fatal(err)
	}
	if err := checkGoldens(dir, files); err == nil || !strings.Contains(err.Error(), prechangeWire) {
		t.Fatalf("a missing golden must fail and name the file: %v", err)
	}
}

// The wire capture carries every frozen builtin, in order, with a valid
// schema — that is what Task 4 compares the eighth→ninth entry against.
func TestWireToolsCaptureShape(t *testing.T) {
	specs, err := wireTools()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"read", "write", "edit", "shell", "search", "ls", "web", "mcp"}
	if len(specs) != len(want) {
		t.Fatalf("captured %d tools, want %d", len(specs), len(want))
	}
	for i, name := range want {
		if specs[i].Name != name {
			t.Fatalf("tool %d = %q, want %q (the model-facing order is a contract)", i, specs[i].Name, name)
		}
		if specs[i].Description == "" || len(specs[i].Schema) == 0 {
			t.Fatalf("tool %q has an empty description or schema", name)
		}
	}
}

// The direct-output capture must contain the pre-fix read window: a skipped
// over-budget line with later lines still printed and a contiguous footer.
// Task 3 changes exactly this case; pinning the shape here means the golden
// cannot quietly be re-captured after the fix without that being visible.
// TestDirectOutputPinsTheReadHole pins the fix that Task 3 makes to the direct
// read window: the capture must stop at the first row that does not fit, report
// the contiguous range it actually returned, and tell the caller where to
// continue. Before the fix this case skipped the long line, printed the tail
// anyway and claimed "[lines 1-30 of 30]".
func TestDirectOutputPinsTheReadHole(t *testing.T) {
	cases, err := DirectOutput()
	if err != nil {
		t.Fatal(err)
	}
	var hole prechangeCase
	for _, c := range cases {
		if c.Name == "read-long-hole" {
			hole = c
		}
	}
	if hole.Content == "" {
		t.Fatal("no read-long-hole case in the capture")
	}
	if strings.Contains(hole.Content, "25|") {
		t.Fatal("line 25 fits the budget in the capture — the fixture no longer reproduces the hole")
	}
	for _, later := range []string{"26|tail line 26", "30|tail line 30"} {
		if strings.Contains(hole.Content, later) {
			t.Fatalf("the capture printed %q after the row that did not fit: the range is not contiguous", later)
		}
	}
	if strings.Contains(hole.Content, "[lines 1-30 of 30]") {
		t.Fatal("the capture still claims the whole file was returned")
	}
	if !strings.Contains(hole.Content, "stopped at the") || !strings.Contains(hole.Content, "use offset=25 to continue") {
		t.Fatalf("the footer does not name the window and the continuation: %q", hole.Content[len(hole.Content)-100:])
	}
}

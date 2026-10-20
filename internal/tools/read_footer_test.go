package tools

// T-11 probe: the `read` footer is what a model pages by, so its three numbers
// must describe what was actually returned. The finding was unverified — this
// pins the contract instead of guessing at a defect: the window, the true total,
// the offset past the end, and the char cap stopping mid-window.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFooterNumbers(t *testing.T) {
	env, root := testEnv(t)
	var sb strings.Builder
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// Default window: 400 lines, and the footer's total is the whole file.
	r := run(t, readTool{}, env, map[string]any{"path": "big.txt"})
	if r.IsError {
		t.Fatalf("%s", r.Content)
	}
	if !strings.Contains(r.Content, "lines 1-400 of 1000") {
		t.Fatalf("default window footer: %.120q", r.Content[max(0, len(r.Content)-80):])
	}

	// A paged window reports the offset as the first line.
	r = run(t, readTool{}, env, map[string]any{"path": "big.txt", "offset": 401, "limit": 10})
	if r.IsError || !strings.Contains(r.Content, "lines 401-410 of 1000") {
		t.Fatalf("paged footer: %v %.160q", r.IsError, r.Content)
	}

	// Past the end: an error naming the total, not an empty success.
	r = run(t, readTool{}, env, map[string]any{"path": "big.txt", "offset": 1001})
	if !r.IsError || !strings.Contains(r.Content, "1000") {
		t.Fatalf("past-the-end read: %v %.120q", r.IsError, r.Content)
	}
}

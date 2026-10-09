package tools

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestListFiles(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "build/\n*.log\n")
	write("a.go", "")
	write("sub/b.go", "")
	write("sub/deep/c.go", "")
	write("build/out.bin", "")
	write("sub/x.log", "")
	write(".hidden/secret", "")
	write("sub/.gitignore", "c.go\n")
	got := ListFiles(context.Background(), root, 100)
	slices.Sort(got)
	if want := []string{"a.go", "sub/b.go"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if n := len(ListFiles(context.Background(), root, 1)); n != 1 {
		t.Fatalf("max not honored: %d", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if n := len(ListFiles(ctx, root, 100)); n != 0 {
		t.Fatalf("a cancelled walk returns what it had: %d", n)
	}
}

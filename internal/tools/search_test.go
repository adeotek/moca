package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(body), 0o644)
}

func TestSearch(t *testing.T) {
	env, root := testEnv(t)
	write(t, root, "a.go", "package a\nfunc Foo() {}\n")
	write(t, root, "sub/b.go", "// Foo here\nfunc Bar() {}\n")
	write(t, root, "sub/c.txt", "Foo text\n")
	write(t, root, ".hidden/d.go", "Foo\n")
	write(t, root, "vendor/e.go", "Foo\n")
	write(t, root, ".gitignore", "vendor/\n")
	write(t, root, "bin.dat", "Foo\x00\x01")

	r := run(t, searchTool{}, env, map[string]any{"pattern": `Foo`})
	want := "a.go:2:func Foo() {}\nsub/b.go:1:// Foo here\nsub/c.txt:1:Foo text"
	if r.Content != want {
		t.Fatalf("got\n%s\nwant\n%s", r.Content, want)
	}
	r = run(t, searchTool{}, env, map[string]any{"pattern": `Foo`, "glob": "*.go", "files_only": true})
	if r.Content != "a.go\nsub/b.go" {
		t.Fatalf("%q", r.Content)
	}
	r = run(t, searchTool{}, env, map[string]any{"pattern": `Foo`, "path": "sub", "glob": "sub/*.txt"})
	if r.Content != "sub/c.txt:1:Foo text" {
		t.Fatalf("%q", r.Content)
	}
	if r := run(t, searchTool{}, env, map[string]any{"pattern": `(?<=x)y`}); !r.IsError || !strings.Contains(r.Content, "RE2") {
		t.Fatalf("lookbehind must explain RE2: %q", r.Content)
	}
	if r := run(t, searchTool{}, env, map[string]any{"pattern": `nomatch_zzz`}); r.Content != "[no matches]" {
		t.Fatalf("%q", r.Content)
	}
}

func TestSearchCap(t *testing.T) {
	env, root := testEnv(t)
	var sb strings.Builder
	for i := range 300 {
		fmt.Fprintf(&sb, "hit %d\n", i)
	}
	write(t, root, "many.txt", sb.String())
	r := run(t, searchTool{}, env, map[string]any{"pattern": `hit`})
	if strings.Count(r.Content, "\n") != 200 || !strings.Contains(r.Content, "capped at 200") {
		t.Fatalf("cap: %d lines", strings.Count(r.Content, "\n"))
	}
}

func TestSearchAppliesAncestorIgnores(t *testing.T) {
	env, root := testEnv(t)
	write(t, root, ".gitignore", "ignored.txt\n")
	write(t, root, "sub/ignored.txt", "needle\n")
	write(t, root, "sub/kept.txt", "needle\n")

	r := run(t, searchTool{}, env, map[string]any{"pattern": "needle", "path": "sub"})
	if r.Content != "sub/kept.txt:1:needle" {
		t.Fatalf("workdir .gitignore must apply to a sub-path search: %q", r.Content)
	}
	r = run(t, searchTool{}, env, map[string]any{"pattern": "needle"})
	if r.Content != "sub/kept.txt:1:needle" {
		t.Fatalf("%q", r.Content)
	}
}

func TestSearchSkipsHugeFiles(t *testing.T) {
	env, root := testEnv(t)
	write(t, root, "small.txt", "needle\n")
	write(t, root, "big.txt", "needle\n"+strings.Repeat("filler line\n", (searchMaxFileSize/12)+2))
	r := run(t, searchTool{}, env, map[string]any{"pattern": "needle"})
	if !strings.Contains(r.Content, "small.txt:1:needle") || !strings.Contains(r.Content, "larger than 4 MB skipped]") {
		t.Fatalf("%q", r.Content)
	}
	if strings.Contains(r.Content, "big.txt:1:needle") {
		t.Fatal("huge file must be skipped, not read")
	}
}

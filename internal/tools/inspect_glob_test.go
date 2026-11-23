package tools

// tools.glob exists to make "one program over all the files" writable. The
// pilot measured the model scripting one file per call because `ls` answers one
// directory and the names were unknown (Task 7 rounds 1-3), so the contract
// here is: name the paths, respect the same ignore rules as search, and account
// for every entry against the same budget.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func globPayloadOf(t *testing.T, in *Inspection, args string) globPayload {
	t.Helper()
	v, err := in.Glob(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("glob %s: %v", args, err)
	}
	p, ok := v.(globPayload)
	if !ok {
		t.Fatalf("glob %s returned %T", args, v)
	}
	return p
}

func TestGlobMatchesPathsAcrossDirectories(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"services/api/service.yaml", "services/billing/service.yaml",
		"services/api/notes.md", "README.md", ".hidden/secret.yaml",
	} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in, ierr := NewInspection(root)
	if ierr != nil {
		t.Fatal(ierr)
	}
	defer in.Close()

	p := globPayloadOf(t, in, `{"pattern":"services/*/service.yaml"}`)
	if len(p.Paths) != 2 || p.Paths[0] != "services/api/service.yaml" || p.Paths[1] != "services/billing/service.yaml" {
		t.Fatalf("paths = %v", p.Paths)
	}
	if !p.Complete {
		t.Fatal("a small glob reported incomplete coverage")
	}

	// `**` spans any number of segments, including none.
	all := globPayloadOf(t, in, `{"pattern":"**/*.yaml"}`)
	if len(all.Paths) != 2 {
		t.Fatalf("recursive paths = %v (hidden entries stay hidden)", all.Paths)
	}
	top := globPayloadOf(t, in, `{"pattern":"*.md"}`)
	if len(top.Paths) != 1 || top.Paths[0] != "README.md" {
		t.Fatalf("top-level paths = %v", top.Paths)
	}

	// A pattern is relative to the searched directory, while the paths it
	// returns are always root-relative: the model can feed them straight back
	// into tools.read.
	sub := globPayloadOf(t, in, `{"pattern":"*/service.yaml","path":"services"}`)
	if len(sub.Paths) != 2 || sub.Paths[0] != "services/api/service.yaml" {
		t.Fatalf("scoped paths = %v", sub.Paths)
	}
}

func TestGlobHonoursIgnoreFiles(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "vendor/\n*.skip.yaml\n")
	write("keep.yaml", "x\n")
	write("drop.skip.yaml", "x\n")
	write("vendor/lib.yaml", "x\n")
	write("sub/.gitignore", "local.yaml\n")
	write("sub/app.yaml", "x\n")
	write("sub/local.yaml", "x\n")

	in, ierr := NewInspection(root)
	if ierr != nil {
		t.Fatal(ierr)
	}
	defer in.Close()
	p := globPayloadOf(t, in, `{"pattern":"**/*.yaml"}`)
	got := map[string]bool{}
	for _, path := range p.Paths {
		got[path] = true
	}
	if got["drop.skip.yaml"] || got["vendor/lib.yaml"] || got["sub/local.yaml"] {
		t.Fatalf("ignored paths survived: %v", p.Paths)
	}
	if !got["keep.yaml"] || !got["sub/app.yaml"] {
		t.Fatalf("allowed paths missing: %v", p.Paths)
	}
}

func TestGlobRefusesEscapeShapedPatterns(t *testing.T) {
	root := t.TempDir()
	in, ierr := NewInspection(root)
	if ierr != nil {
		t.Fatal(ierr)
	}
	defer in.Close()
	for _, args := range []string{
		`{"pattern":""}`,
		`{"pattern":"/etc/*"}`,
		`{"pattern":"../*"}`,
		`{"pattern":"a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q"}`,
	} {
		if _, err := in.Glob(context.Background(), json.RawMessage(args)); err == nil {
			t.Fatalf("%s was accepted", args)
		}
	}
}

// The entry budget is shared with read/search/ls: a glob that walks the whole
// tree cannot quietly spend it all and leave the rest of the script blind.
func TestGlobChargesTheEntryBudget(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 8; i++ {
		if err := os.WriteFile(filepath.Join(root, "f", string(rune('a'+i))+".yaml"), nil, 0o644); err != nil {
			if os.MkdirAll(filepath.Join(root, "f"), 0o755) != nil {
				t.Fatal("mkdir")
			}
			_ = os.WriteFile(filepath.Join(root, "f", string(rune('a'+i))+".yaml"), nil, 0o644)
		}
	}
	in, ierr := NewInspection(root)
	if ierr != nil {
		t.Fatal(ierr)
	}
	defer in.Close()
	if _, err := in.Glob(context.Background(), json.RawMessage(`{"pattern":"**"}`)); err != nil {
		t.Fatal(err)
	}
	if in.entries == 0 {
		t.Fatal("glob walked the tree without charging a single entry")
	}
}

// matchGlob is the whole pattern language; a table keeps it honest.
func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pat, path string
		want      bool
	}{
		{"services/*/service.yaml", "services/api/service.yaml", true},
		{"services/*/service.yaml", "services/api/other.yaml", false},
		{"services/*/service.yaml", "services/a/b/service.yaml", false},
		{"**/service.yaml", "service.yaml", true},
		{"**/service.yaml", "a/b/service.yaml", true},
		{"**", "a/b/c", true},
		{"a/**/d", "a/d", true},
		{"a/**/d", "a/b/c/d", true},
		{"a/**/d", "a/b/c", false},
		{"?.yaml", "a.yaml", true},
		{"?.yaml", "ab.yaml", false},
		{"[ab].yaml", "b.yaml", true},
		{"[ab].yaml", "c.yaml", false},
	}
	for _, c := range cases {
		got := matchGlob(splitPath(c.pat), splitPath(c.path))
		if got != c.want {
			t.Fatalf("matchGlob(%q, %q) = %v, want %v", c.pat, c.path, got, c.want)
		}
	}
}

func splitPath(p string) []string {
	var out []string
	cur := ""
	for _, r := range p {
		if r == '/' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

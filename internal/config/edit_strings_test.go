package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestAppendStringsAtomicAndBytePreserving pins the multi-value append behind
// a composite allow-always: every name lands in ONE read/merge/write (one
// .bak, one rename), or the file is left untouched.
func TestAppendStringsAtomicAndBytePreserving(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	src := "{\n  // keep me\n  \"shell\": {\n    \"allow\": [\"go\", \"git\",], // trailing, comma\n  },\n}\n"
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AppendStrings(p, []string{"shell", "allow"}, []string{"python3", "perl", "git", "python3"}, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	want := "{\n  // keep me\n  \"shell\": {\n    \"allow\": [\"go\", \"git\", \"python3\", \"perl\"], // trailing, comma\n  },\n}\n"
	if string(got) != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if b, _ := os.ReadFile(p + ".bak"); string(b) != src {
		t.Fatalf("backup is not the pre-edit file:\n%s", b)
	}
	if _, err := Parse(got); err != nil {
		t.Fatal("result must stay valid:", err)
	}

	// A call that adds nothing is a no-op: it must not rewrite the file.
	// (If it had written, the .bak would now hold the first result.)
	if err := AppendStrings(p, []string{"shell", "allow"}, []string{"git", "go"}, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p + ".bak"); string(b) != src {
		t.Fatal("a no-op append rewrote the file")
	}

	// String-literal commas survive as one value.
	if err := AppendStrings(p, []string{"shell", "allow"}, []string{"awk -F, '{print}'"}, nil); err != nil {
		t.Fatal(err)
	}
	c, err := Parse(mustRead(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(c.Shell.Allow, "awk -F, '{print}'") {
		t.Fatalf("value with a comma was split: %v", c.Shell.Allow)
	}
}

func TestAppendStringsEmptyIsANoOp(t *testing.T) {
	p := filepath.Join(t.TempDir(), "none.jsonc")
	if err := AppendStrings(p, []string{"shell", "allow"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("an empty append must not create the file")
	}
}

// A failure while writing leaves the original bytes untouched.
func TestAppendStringsFailureLeavesTheOriginal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.jsonc")
	src := "{\"shell\":{\"allow\":[\"go\"]}}\n"
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Skipf("cannot make the directory read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	err := AppendStrings(p, []string{"shell", "allow"}, []string{"python3", "perl"}, nil)
	if err == nil {
		if fi, statErr := os.Stat(p); statErr == nil && fi.ModTime().After(time.Now().Add(-time.Second)) {
			t.Skip("running with privileges that ignore directory permissions")
		}
		t.Fatal("want a write error in a read-only directory")
	}
	if got, _ := os.ReadFile(p); string(got) != src {
		t.Fatalf("failed append changed the file:\n%s", got)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

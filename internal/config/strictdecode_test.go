package config

// Pin for T-7: one JSON object per file. `dec.Decode` reads the first value and
// stops, so trailing content (`{} {}`, `{...} junk`) parsed as a valid config
// with the rest silently dropped — in both the user config and the project
// file, the latter a file a cloned repository controls.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRejectsTrailingContent(t *testing.T) {
	for _, src := range []string{`{} {}`, `{} junk`, "{\"model\":\"m\"} {\"model\":\"n\"}"} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("Parse(%q) accepted trailing content", src)
		}
	}
	// Control: a single object still parses.
	if _, err := Parse([]byte(`{}`)); err != nil {
		t.Fatalf("Parse({}) = %v", err)
	}
}

func TestLoadProjectRejectsTrailingContent(t *testing.T) {
	dir := t.TempDir()
	p := ProjectConfigFile(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"permissions":{"defaultMode":"auto"}} {"permissions":{"defaultMode":"yolo"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(dir); err == nil {
		t.Fatal("LoadProject accepted trailing content — a second object must not be silently dropped")
	}
}

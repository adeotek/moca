package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadInstructions(t *testing.T) {
	global, work := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(global, "AGENTS.md"), []byte("global rules"), 0o644)
	os.WriteFile(filepath.Join(work, "CLAUDE.md"), []byte("claude rules"), 0o644)
	got, _ := LoadInstructions(global, work, false)
	if len(got) != 1 || got[0].Content != "global rules" {
		t.Fatalf("untrusted: %+v", got)
	}
	got, _ = LoadInstructions(global, work, true)
	if len(got) != 2 || got[1].Content != "claude rules" {
		t.Fatalf("CLAUDE.md fallback: %+v", got)
	}
	os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte(strings.Repeat("a", 40_000)), 0o644)
	got, _ = LoadInstructions(global, work, true)
	if !got[1].Truncated || !strings.HasSuffix(got[1].Content, "[… truncated at 32K chars]") || !strings.HasSuffix(got[1].Path, "AGENTS.md") {
		t.Fatal("AGENTS.md wins over CLAUDE.md and is capped")
	}
}

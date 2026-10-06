package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// A corrupt index file must never serve a half-parsed map (pass-1 L2).
func TestLoadIndexCorruptResets(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ix.json")
	// "a" decodes before "b" fails: pre-fix this left a usable-looking entry.
	os.WriteFile(p, []byte(`{"servers":{"a":{"configHash":"h","tools":[{"name":"x"}]},"b":"bad"}}`), 0o600)
	ix, err := LoadIndex(p)
	if err != nil || len(ix.Servers) != 0 {
		t.Fatalf("%v %v", ix.Servers, err)
	}
	if _, ok := ix.Valid("a", "h"); ok {
		t.Fatal("a half-parsed entry must not be served")
	}
	os.WriteFile(p, []byte(`not json`), 0o600)
	if ix, _ := LoadIndex(p); len(ix.Servers) != 0 {
		t.Fatal("a garbage file must yield an empty index")
	}
}

func TestIndexPersistAndHash(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mcp-index.json")
	ix, _ := LoadIndex(p)
	s := config.MCPServer{Command: "npx", Args: []string{"x"}}
	h := ConfigHash(s)
	ix.Put("fs", h, []Tool{{Name: "read_file", Description: "Read a file"}})
	ix.Save()
	ix2, _ := LoadIndex(p)
	if _, ok := ix2.Valid("fs", h); !ok {
		t.Fatal("persisted")
	}
	s.Description = "changed"
	if ConfigHash(s) != h {
		t.Fatal("description does not affect the hash")
	}
	s.Args = []string{"y"}
	if _, ok := ix2.Valid("fs", ConfigHash(s)); ok {
		t.Fatal("config change invalidates")
	}
}

func TestRank(t *testing.T) {
	entries := map[string]IndexEntry{
		"github":   {Tools: []IndexTool{{Name: "create_issue", Description: "Create a GitHub issue"}, {Name: "list_issues", Description: "List issues in a repo"}, {Name: "get_file", Description: "Get file contents"}}},
		"context7": {Tools: []IndexTool{{Name: "get-library-docs", Description: "Fetch documentation for a library"}}},
	}
	hits := Rank("create issue", entries, "", 10)
	if len(hits) < 2 || hits[0].Tool != "create_issue" || hits[1].Tool != "list_issues" {
		t.Fatal(hits)
	}
	if hits := Rank("docs", entries, "github", 10); len(hits) != 0 {
		t.Fatal("server filter")
	}
	if hits := Rank("", entries, "context7", 10); len(hits) != 1 {
		t.Fatal("empty query lists")
	}
}

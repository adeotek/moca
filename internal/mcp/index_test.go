package mcp

import (
	"path/filepath"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

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

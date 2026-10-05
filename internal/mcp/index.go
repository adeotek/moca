package mcp

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/adeotek/moca/internal/config"
)

// IndexTool is one tool's discoverable facts, persisted so search works
// without starting the server.
type IndexTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Annotations Annotations `json:"annotations"`
}

type IndexEntry struct {
	ConfigHash string      `json:"configHash"`
	Tools      []IndexTool `json:"tools"`
	UpdatedAt  time.Time   `json:"updatedAt"`
}

// Index is the persisted discovery index (~/.local/share/moca/mcp-index.json):
// server name → the tool list from its last successful start. An entry is
// valid only while the server's config hash matches.
type Index struct {
	path    string
	mu      sync.Mutex
	Servers map[string]IndexEntry `json:"servers"`
}

// LoadIndex never fails on a missing or corrupt file: both mean "no index
// yet" and the servers will be indexed on demand.
func LoadIndex(path string) (*Index, error) {
	ix := &Index{path: path, Servers: map[string]IndexEntry{}}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, ix)
		if ix.Servers == nil {
			ix.Servers = map[string]IndexEntry{}
		}
	}
	return ix, nil
}

func (ix *Index) Save() error {
	ix.mu.Lock()
	b, err := json.MarshalIndent(ix, "", "  ")
	ix.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ix.path), 0o700); err != nil {
		return err
	}
	tmp := ix.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ix.path)
}

func (ix *Index) Valid(server, hash string) (IndexEntry, bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	e, ok := ix.Servers[server]
	return e, ok && e.ConfigHash == hash
}

func (ix *Index) Put(server, hash string, tools []Tool) {
	e := IndexEntry{ConfigHash: hash, UpdatedAt: time.Now()}
	for _, t := range tools {
		e.Tools = append(e.Tools, IndexTool{Name: t.Name, Description: t.Description, Annotations: t.Annotations})
	}
	ix.mu.Lock()
	ix.Servers[server] = e
	ix.mu.Unlock()
}

// ConfigHash covers exactly the fields that change a server's tool list;
// description and approve are presentation/trust knobs, so editing them does
// not force a re-index.
func ConfigHash(s config.MCPServer) string {
	b, _ := json.Marshal(struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}{Command: s.Command, Args: s.Args, Env: s.Env, URL: s.URL, Headers: s.Headers}) // encoding/json sorts map keys
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type Hit struct {
	Server, Tool, Description string
	score                     int
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// Rank scores the indexed tools against a lowercased word query: a name
// token equal to a query token scores 3, contained in the name 2, and
// contained in the description 1. Ties break by server then tool name. An
// empty query lists everything (score 0 allowed) up to limit.
func Rank(query string, entries map[string]IndexEntry, server string, limit int) []Hit {
	q := words(query)
	var hits []Hit
	for srv, e := range entries {
		if server != "" && srv != server {
			continue
		}
		for _, t := range e.Tools {
			nameWords := words(t.Name)
			name, desc := strings.ToLower(t.Name), strings.ToLower(t.Description)
			score := 0
			for _, w := range q {
				if slices.Contains(nameWords, w) {
					score += 3
				} else if strings.Contains(name, w) {
					score += 2
				}
				if strings.Contains(desc, w) {
					score++
				}
			}
			if len(q) > 0 && score == 0 {
				continue
			}
			hits = append(hits, Hit{srv, t.Name, t.Description, score})
		}
	}
	slices.SortFunc(hits, func(a, b Hit) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.Server, b.Server), cmp.Compare(a.Tool, b.Tool))
	})
	return hits[:min(limit, len(hits))]
}

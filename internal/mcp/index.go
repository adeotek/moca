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
	touched map[string]bool       // servers this process Put (they win on Save)
	Servers map[string]IndexEntry `json:"servers"`
}

// LoadIndex never fails on a missing or corrupt file: both mean "no index
// yet" and the servers will be indexed on demand. A corrupt file that parses
// a prefix must not serve a half-index, so an unmarshal error resets to empty.
func LoadIndex(path string) (*Index, error) {
	ix := &Index{path: path, Servers: map[string]IndexEntry{}}
	if b, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(b, ix) != nil {
			ix.Servers = map[string]IndexEntry{}
		}
		if ix.Servers == nil {
			ix.Servers = map[string]IndexEntry{}
		}
	}
	return ix, nil
}

// Save writes the index atomically. Other moca processes share the file, so
// it merges instead of overwriting: entries on disk for servers this process
// did not index are kept, and the temp file is unique per writer (a fixed name
// let two writers interleave on one inode).
func (ix *Index) Save() error {
	disk := &Index{Servers: map[string]IndexEntry{}}
	if b, err := os.ReadFile(ix.path); err == nil && json.Unmarshal(b, disk) != nil {
		disk.Servers = nil
	}
	ix.mu.Lock()
	merged := map[string]IndexEntry{}
	for k, e := range disk.Servers {
		merged[k] = e
	}
	for k, e := range ix.Servers {
		if _, onDisk := merged[k]; ix.touched[k] || !onDisk {
			merged[k] = e
		}
	}
	b, err := json.MarshalIndent(struct {
		Servers map[string]IndexEntry `json:"servers"`
	}{merged}, "", "  ")
	ix.mu.Unlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(ix.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, filepath.Base(ix.path)+".*.tmp") // 0600
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), ix.path); err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
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
	if ix.touched == nil {
		ix.touched = map[string]bool{}
	}
	ix.touched[server] = true
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

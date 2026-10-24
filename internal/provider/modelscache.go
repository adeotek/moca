package provider

// Per-provider model-list cache (SPECS §6). A provider's model list is fetched
// from the provider itself and kept here for ModelsCacheTTL (24 h): one file
// per provider in one directory, disposable state, metadata only (ids,
// windows, prices, effort levels — never a key, a prompt or a response). Each
// file is written atomically and belongs to one provider, so providers
// discovered in parallel — or by two moca processes (a TUI and an ACP server)
// — never rewrite each other's list and need no lock.
//
// The cache stores a NORMALIZED snapshot, not the provider payload: each
// fetcher maps its wire shape into CachedModel, so a vendor changing its JSON
// is a fetcher change, not a cache-schema migration. `version` invalidates a
// file when the normalized shape changes.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/config"
)

const (
	// ModelsCacheTTL is how long a fetched model list is considered fresh.
	ModelsCacheTTL = 24 * time.Hour
	// modelsCacheVersion is the normalized-shape version stored in each file.
	modelsCacheVersion = 1
)

// CachedModel is one provider model, normalized. An empty Protocol means "the
// provider's default"; an empty Ctx means "unknown" (the registry substitutes
// an assumed window and flags it); CostKnown false means the provider publishes
// no price, which the UI renders as `—` rather than `$0.00`.
type CachedModel struct {
	ID        string            `json:"id"`
	Protocol  string            `json:"protocol,omitempty"`
	Mode      string            `json:"mode,omitempty"` // thinking mode: "" | "none" | "effort-object" | "adaptive" | "budget"
	Ctx       int               `json:"ctx,omitempty"`
	MaxOut    int               `json:"maxOut,omitempty"`
	Cost      config.CostConfig `json:"cost,omitempty"`
	CostKnown bool              `json:"costKnown,omitempty"`
	Efforts   []string          `json:"efforts,omitempty"` // wire effort values the model accepts
	Mandatory bool              `json:"mandatory,omitempty"`
}

// modelsCacheFile is one provider's file. Provider repeats the name the file
// was written for: two names that share a file on a case-insensitive file
// system ("GW" and "gw") read as a miss, never as each other's list.
type modelsCacheFile struct {
	Version   int           `json:"version"`
	Provider  string        `json:"provider"`
	BaseURL   string        `json:"baseUrl"`
	FetchedAt time.Time     `json:"fetchedAt"`
	Models    []CachedModel `json:"models"`
}

// ModelsCache is a directory of <provider>.json files.
type ModelsCache struct {
	dir string
	ttl time.Duration
	now func() time.Time
}

func NewModelsCache(dir string, ttl time.Duration) *ModelsCache {
	return &ModelsCache{dir: dir, ttl: ttl, now: time.Now}
}

// Dir is where the cache lives (empty when caching is disabled).
func (c *ModelsCache) Dir() string {
	if c == nil {
		return ""
	}
	return c.dir
}

// Load returns the provider's cached models and whether they are still fresh.
// A stale entry is returned with fresh=false (the caller serves it while it
// refetches). A missing, corrupt, other-version, other-provider or
// other-baseURL file is a miss: no models, not fresh.
func (c *ModelsCache) Load(providerName, baseURL string) ([]CachedModel, bool) {
	if c == nil || c.dir == "" {
		return nil, false
	}
	f, err := readModelsCacheFile(c.file(providerName))
	if err != nil || f.Provider != providerName || f.BaseURL != baseURL {
		return nil, false
	}
	return f.Models, c.now().Sub(f.FetchedAt) < c.ttl
}

// Put replaces the provider's file. A failure to write is not fatal to a
// discovery run — the models are already in memory — so callers log it.
func (c *ModelsCache) Put(providerName, baseURL string, models []CachedModel) error {
	if c == nil || c.dir == "" {
		return nil
	}
	return c.write(c.file(providerName), modelsCacheFile{Version: modelsCacheVersion,
		Provider: providerName, BaseURL: baseURL, FetchedAt: c.now(), Models: models})
}

// Clear deletes every provider's file and reports what the readable ones
// held; an unreadable file is deleted too, but counts for nothing.
func (c *ModelsCache) Clear() (providers, models int, err error) {
	if c == nil || c.dir == "" {
		return 0, 0, nil
	}
	entries, err := os.ReadDir(c.dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(c.dir, e.Name())
		if f, rerr := readModelsCacheFile(path); rerr == nil {
			providers++
			models += len(f.Models)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return providers, models, err
		}
	}
	return providers, models, nil
}

// file is the provider's cache file: the name escaped to a portable file
// name, since a provider name is a config key and may hold '/', ':' or '\'.
func (c *ModelsCache) file(providerName string) string {
	return filepath.Join(c.dir, escapeFileName(providerName)+".json")
}

// escapeFileName keeps [A-Za-z0-9._-] and writes every other byte as %XX:
// the result has no path separator and no character Windows reserves.
func escapeFileName(s string) string {
	var b strings.Builder
	for i := range len(s) {
		switch ch := s[i]; {
		case 'a' <= ch && ch <= 'z', 'A' <= ch && ch <= 'Z', '0' <= ch && ch <= '9', ch == '.', ch == '_', ch == '-':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

func readModelsCacheFile(path string) (modelsCacheFile, error) {
	var f modelsCacheFile
	b, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return modelsCacheFile{}, fmt.Errorf("models cache %s: %w", path, err)
	}
	if f.Version != modelsCacheVersion {
		return modelsCacheFile{}, fmt.Errorf("models cache %s: unsupported version %d", path, f.Version)
	}
	return f, nil
}

func (c *ModelsCache) write(path string, f modelsCacheFile) error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(c.dir, ".models-cache-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // no-op after a successful rename
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

package provider

// Per-provider model-list cache (SPECS §6). A provider's model list is fetched
// from the provider itself and kept here for ModelsCacheTTL (24 h): the file
// is disposable state, holds metadata only (ids, windows, prices, effort
// levels — never a key, a prompt or a response), and is written atomically so
// two moca processes (a TUI and an ACP server) can rewrite it without a lock.
//
// The cache stores a NORMALIZED snapshot, not the provider payload: each
// fetcher maps its wire shape into CachedModel, so a vendor changing its JSON
// is a fetcher change, not a cache-schema migration. `version` invalidates the
// whole file when the normalized shape changes.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adeotek/moca/internal/config"
)

const (
	// ModelsCacheTTL is how long a fetched model list is considered fresh.
	ModelsCacheTTL = 24 * time.Hour
	// modelsCacheVersion is the normalized-shape version stored in the file.
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

type modelsCacheFile struct {
	Version int                         `json:"version"`
	Entries map[string]modelsCacheEntry `json:"entries"`
}

type modelsCacheEntry struct {
	BaseURL   string        `json:"baseUrl"`
	FetchedAt time.Time     `json:"fetchedAt"`
	Models    []CachedModel `json:"models"`
}

// ModelsCache is one JSON file keyed by provider name.
type ModelsCache struct {
	path string
	ttl  time.Duration
	now  func() time.Time
}

func NewModelsCache(path string, ttl time.Duration) *ModelsCache {
	return &ModelsCache{path: path, ttl: ttl, now: time.Now}
}

// Path is where the cache lives (empty when caching is disabled).
func (c *ModelsCache) Path() string {
	if c == nil {
		return ""
	}
	return c.path
}

// Load returns the provider's cached models and whether they are still fresh.
// A stale entry is returned with fresh=false (the caller serves it while it
// refetches). A missing, corrupt, other-version or other-baseURL entry is a
// miss: no models, not fresh.
func (c *ModelsCache) Load(providerName, baseURL string) ([]CachedModel, bool) {
	if c == nil || c.path == "" {
		return nil, false
	}
	f, err := c.read()
	if err != nil {
		return nil, false
	}
	e, ok := f.Entries[providerName]
	if !ok || e.BaseURL != baseURL {
		return nil, false
	}
	fresh := c.now().Sub(e.FetchedAt) < c.ttl
	return e.Models, fresh
}

// Put replaces one provider's entry. A failure to write is not fatal to a
// discovery run — the models are already in memory — so callers log it.
func (c *ModelsCache) Put(providerName, baseURL string, models []CachedModel) error {
	if c == nil || c.path == "" {
		return nil
	}
	f, err := c.read()
	if err != nil {
		f = modelsCacheFile{}
	}
	if f.Entries == nil {
		f.Entries = map[string]modelsCacheEntry{}
	}
	f.Version = modelsCacheVersion
	f.Entries[providerName] = modelsCacheEntry{BaseURL: baseURL, FetchedAt: c.now(), Models: models}
	return c.write(f)
}

// Clear drops every entry and reports what was dropped.
func (c *ModelsCache) Clear() (providers, models int, err error) {
	if c == nil || c.path == "" {
		return 0, 0, nil
	}
	f, rerr := c.read()
	if rerr == nil {
		for _, e := range f.Entries {
			providers++
			models += len(e.Models)
		}
	}
	if err := os.Remove(c.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, 0, err
	}
	return providers, models, nil
}

func (c *ModelsCache) read() (modelsCacheFile, error) {
	var f modelsCacheFile
	b, err := os.ReadFile(c.path)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return modelsCacheFile{}, fmt.Errorf("models cache %s: %w", c.path, err)
	}
	if f.Version != modelsCacheVersion {
		return modelsCacheFile{}, fmt.Errorf("models cache %s: unsupported version %d", c.path, f.Version)
	}
	if f.Entries == nil {
		f.Entries = map[string]modelsCacheEntry{}
	}
	return f, nil
}

func (c *ModelsCache) write(f modelsCacheFile) error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".models-cache-*")
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
	return os.Rename(name, c.path)
}

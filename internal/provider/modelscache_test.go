package provider

// Tests for the per-provider model-list cache (SPECS §6, Task 1).

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestCache(t *testing.T) (*ModelsCache, *time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	c := NewModelsCache(filepath.Join(t.TempDir(), "models-cache.json"), 24*time.Hour)
	c.now = func() time.Time { return now }
	return c, &now
}

func TestModelsCacheTTLAndBaseURL(t *testing.T) {
	c, now := newTestCache(t)
	const base = "https://gw.example/api/v1"
	if err := c.Put("gw", base, []CachedModel{{ID: "m", Ctx: 1000}}); err != nil {
		t.Fatal(err)
	}
	if models, fresh := c.Load("gw", base); !fresh || len(models) != 1 || models[0].ID != "m" {
		t.Fatalf("fresh entry: %v %v", models, fresh)
	}
	*now = now.Add(23*time.Hour + 59*time.Minute)
	if _, fresh := c.Load("gw", base); !fresh {
		t.Fatal("23h59m must still be fresh")
	}
	*now = now.Add(2 * time.Minute) // 24h01m
	models, fresh := c.Load("gw", base)
	if fresh {
		t.Fatal("past 24h must be stale")
	}
	if len(models) != 1 {
		t.Fatal("a stale entry still returns its rows (served while refetching)")
	}
	if _, fresh := c.Load("gw", "https://other.example/v1"); fresh || len(models) == 0 {
		t.Fatal("a changed base URL invalidates the entry")
	}
	if _, fresh := c.Load("other-provider", base); fresh {
		t.Fatal("entries are per provider")
	}
}

func TestModelsCacheFileShape(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Put("gw", "https://gw", []CachedModel{{ID: "m", CostKnown: true}}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(c.Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("cache file mode %o, want 600", perm)
	}
	b, err := os.ReadFile(c.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"apiKey", "Authorization", "sk-", "Bearer"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("the cache must hold metadata only, found %q", secret)
		}
	}
	// Corrupt content is a miss, not a crash.
	if err := os.WriteFile(c.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, fresh := c.Load("gw", "https://gw"); fresh {
		t.Fatal("corrupt cache must load as empty")
	}
	if err := os.WriteFile(c.Path(), []byte(`{"version":99,"entries":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, fresh := c.Load("gw", "https://gw"); fresh {
		t.Fatal("another schema version must load as empty")
	}
}

func TestModelsCacheClearAndConcurrency(t *testing.T) {
	c, _ := newTestCache(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Put("gw", "https://gw", []CachedModel{{ID: string(rune('a' + i))}})
			_, _ = c.Load("gw", "https://gw")
		}()
	}
	wg.Wait()
	providers, models, err := c.Clear()
	if err != nil || providers != 1 || models != 1 {
		t.Fatalf("clear: %d %d %v", providers, models, err)
	}
	if _, err := os.Stat(c.Path()); !os.IsNotExist(err) {
		t.Fatal("clear must remove the file")
	}
	if providers, models, err := c.Clear(); err != nil || providers != 0 || models != 0 {
		t.Fatalf("clear is idempotent: %d %d %v", providers, models, err)
	}
}

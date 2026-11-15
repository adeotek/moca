package provider

// Tests for the per-provider model-list cache (SPECS §6, Task 1).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestCache(t *testing.T) (*ModelsCache, *time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	c := NewModelsCache(filepath.Join(t.TempDir(), "models-cache"), 24*time.Hour)
	c.now = func() time.Time { return now }
	return c, &now
}

// cacheFiles lists the cache directory's file names, sorted.
func cacheFiles(t *testing.T, c *ModelsCache) []string {
	t.Helper()
	entries, err := os.ReadDir(c.Dir())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	return names
}

func TestModelsCacheTTLAndBaseURL(t *testing.T) {
	c, now := newTestCache(t)
	const base = "https://gw.example/api/v1"
	if err := c.Put("gw", base, []CachedModel{{ID: "m", Ctx: 1000}}); err != nil {
		t.Fatal(err)
	}
	if models, found, fresh := c.Load("gw", base); !found || !fresh || len(models) != 1 || models[0].ID != "m" {
		t.Fatalf("fresh entry: %v %v %v", models, found, fresh)
	}
	*now = now.Add(23*time.Hour + 59*time.Minute)
	if _, _, fresh := c.Load("gw", base); !fresh {
		t.Fatal("23h59m must still be fresh")
	}
	*now = now.Add(2 * time.Minute) // 24h01m
	models, found, fresh := c.Load("gw", base)
	if fresh {
		t.Fatal("past 24h must be stale")
	}
	if !found || len(models) != 1 {
		t.Fatal("a stale entry is still an entry, and it returns its rows (served while refetching)")
	}
	if _, found, _ := c.Load("gw", "https://other.example/v1"); found {
		t.Fatal("a changed base URL invalidates the entry")
	}
	if _, found, _ := c.Load("other-provider", base); found {
		t.Fatal("entries are per provider")
	}
}

// Each provider owns one file: a provider's write never rewrites another's
// list, and losing one file costs only that provider's refetch.
func TestModelsCacheOneFilePerProvider(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Put("anthropic", "https://a", []CachedModel{{ID: "claude-x"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Put("openrouter", "https://o", []CachedModel{{ID: "vendor/y"}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cacheFiles(t, c), ","); got != "anthropic.json,openrouter.json" {
		t.Fatalf("cache dir holds %q, want one file per provider", got)
	}
	b, err := os.ReadFile(filepath.Join(c.Dir(), "anthropic.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "vendor/y") {
		t.Fatalf("a provider's file holds only its own list: %s", b)
	}
	if err := os.Remove(filepath.Join(c.Dir(), "openrouter.json")); err != nil {
		t.Fatal(err)
	}
	if models, found, fresh := c.Load("anthropic", "https://a"); !found || !fresh || len(models) != 1 {
		t.Fatal("removing one provider's file must leave the others")
	}
}

// A provider name is a config key: it may hold a path separator, a drive
// colon or a dot-dot. The file must stay directly in the cache dir with a
// portable name (releases ship for Windows), and still round-trip.
func TestModelsCacheFileNamesAreSafe(t *testing.T) {
	c, _ := newTestCache(t)
	names := []string{"my/gw", "..", `a:b\c`, "Ünï"}
	for _, n := range names {
		if err := c.Put(n, "https://x", []CachedModel{{ID: "m-" + n}}); err != nil {
			t.Fatalf("%q: %v", n, err)
		}
	}
	files := cacheFiles(t, c)
	if len(files) != len(names) {
		t.Fatalf("want one file per name directly in the dir, got %q", files)
	}
	portable := regexp.MustCompile(`^[A-Za-z0-9._%-]+\.json$`)
	for _, f := range files {
		if !portable.MatchString(f) {
			t.Fatalf("file name %q is not portable", f)
		}
	}
	if sib := cacheFiles(t, &ModelsCache{dir: filepath.Dir(c.Dir())}); len(sib) != 1 {
		t.Fatalf("a name escaped the cache dir: %q", sib)
	}
	for _, n := range names {
		if models, found, fresh := c.Load(n, "https://x"); !found || !fresh || len(models) != 1 || models[0].ID != "m-"+n {
			t.Fatalf("%q does not round-trip: %v %v %v", n, models, found, fresh)
		}
	}
}

// The provider is recorded inside the file: on a case-insensitive file
// system "GW" and "gw" share a file, and one must never read as the other.
func TestModelsCacheFileOfAnotherProviderIsAMiss(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Put("gw", "https://x", []CachedModel{{ID: "m"}}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(c.Dir(), "gw.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Dir(), "other.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if models, found, _ := c.Load("other", "https://x"); found || len(models) != 0 {
		t.Fatalf("gw's list was read as other's: %v", models)
	}
}

func TestModelsCacheFileShape(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Put("gw", "https://gw", []CachedModel{{ID: "m", CostKnown: true}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(c.Dir(), "gw.json")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("cache file mode %o, want 600", perm)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"apiKey", "Authorization", "sk-", "Bearer"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("the cache must hold metadata only, found %q", secret)
		}
	}
	// Corrupt content is a miss, not a crash.
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, fresh := c.Load("gw", "https://gw"); found || fresh {
		t.Fatal("corrupt cache must load as a miss")
	}
	if err := os.WriteFile(path, []byte(`{"version":99,"provider":"gw","baseUrl":"https://gw","models":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, fresh := c.Load("gw", "https://gw"); found || fresh {
		t.Fatal("another schema version must load as a miss")
	}
}

// Discovery writes every provider in parallel: none may lose its entry.
func TestModelsCacheParallelProvidersAndClear(t *testing.T) {
	c, _ := newTestCache(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Put(fmt.Sprintf("p%d", i), "https://gw", []CachedModel{{ID: "m"}})
		}()
	}
	wg.Wait()
	for i := range 8 {
		if _, _, fresh := c.Load(fmt.Sprintf("p%d", i), "https://gw"); !fresh {
			t.Fatalf("p%d lost its entry to a parallel write", i)
		}
	}
	providers, models, err := c.Clear()
	if err != nil || providers != 8 || models != 8 {
		t.Fatalf("clear: %d %d %v", providers, models, err)
	}
	if left := cacheFiles(t, c); len(left) != 0 {
		t.Fatalf("clear must remove every provider's file, left %q", left)
	}
	if providers, models, err := c.Clear(); err != nil || providers != 0 || models != 0 {
		t.Fatalf("clear is idempotent: %d %d %v", providers, models, err)
	}
}

// An entry that lists no models is still an entry: "the provider lists
// nothing" and "we never asked" are different facts, and a restart must not
// turn the first into the second.
func TestModelsCacheEmptyListIsAnAnswer(t *testing.T) {
	c, now := newTestCache(t)
	for _, tc := range []struct {
		name   string
		models []CachedModel
	}{
		{"nil-then-null", nil},
		{"empty", []CachedModel{}},
	} {
		if err := c.Put("gw", "https://gw", tc.models); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		models, found, fresh := c.Load("gw", "https://gw")
		if !found || !fresh || len(models) != 0 {
			t.Fatalf("%s: an empty answer is a cached entry, found=%v fresh=%v models=%v", tc.name, found, fresh, models)
		}
		// Stale is still found: the entry says the list was read.
		*now = now.Add(25 * time.Hour)
		if _, found, fresh := c.Load("gw", "https://gw"); !found || fresh {
			t.Fatalf("%s: a stale empty answer is still an entry, found=%v fresh=%v", tc.name, found, fresh)
		}
	}
	// A provider with no file at all is the miss.
	if _, found, fresh := c.Load("other", "https://gw"); found || fresh {
		t.Fatal("a provider that was never asked is a miss")
	}
}

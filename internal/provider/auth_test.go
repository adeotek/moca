package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

func TestStoreRoundTripAndMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "data", "auth.json")
	s := NewStore(p)
	if err := s.Put("openai", Token{Access: "a", Refresh: "r", Expiry: time.Now().Add(time.Hour), ClientID: "oaiapp_x"}); err != nil {
		t.Fatal(err)
	}
	tok, ok, err := s.Get("openai")
	if err != nil || !ok || tok.Access != "a" || tok.ClientID != "oaiapp_x" {
		t.Fatal(tok, ok, err)
	}
	fi, _ := os.Stat(p)
	di, _ := os.Stat(filepath.Dir(p))
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Fatal("modes", fi.Mode(), di.Mode())
	}
	if err := s.Delete("openai"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Get("openai"); ok {
		t.Fatal("deleted")
	}
}

// A pre-existing tmp file (crash debris, another tool running as the same
// user) must not leak its mode into the published store: the write path
// forces 0600 regardless of what the tmp file had.
func TestStoreWriteForcesMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(p+".tmp", []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(p)
	if err := s.Put("openai", Token{Access: "a", Refresh: "r", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("auth.json mode %v, want 0600", fi.Mode().Perm())
	}
}

func TestConcurrentRefreshOnce(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.Put("x", Token{Access: "old", Refresh: "r1", Expiry: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	ref := func(ctx context.Context, tok Token) (Token, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		if tok.Refresh != "r1" {
			return Token{}, ErrInvalidGrant // rotated refresh token reused
		}
		return Token{Access: "new", Refresh: "r2", Expiry: time.Now().Add(time.Hour)}, nil
	}
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// separate Store values simulate separate processes sharing the file + lock
			c, err := NewStore(p).CredentialFor("x", ref)(context.Background())
			if err == nil && c.Token != "new" {
				err = errors.New("got " + c.Token)
			}
			errs[i] = err
		}()
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("refresh calls = %d", calls.Load())
	}
}

func TestRevokedAndCorrupt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.Put("x", Token{Access: "old", Refresh: "dead", Expiry: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	_, err := s.CredentialFor("x", func(context.Context, Token) (Token, error) { return Token{}, ErrInvalidGrant })(context.Background())
	if !errors.Is(err, ErrInvalidGrant) || !strings.Contains(err.Error(), "moca login x") {
		t.Fatal(err)
	}
	if _, ok, _ := s.Get("x"); !ok {
		t.Fatal("stale entry kept for logout")
	}
	os.WriteFile(p, []byte(`{"providers":{"x":{"acc`), 0o600)
	if _, _, err := s.Get("x"); err == nil || !strings.Contains(err.Error(), "moca logout") {
		t.Fatal(err)
	}
}

func TestInvalidClient(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.Put("x", Token{Access: "old", Refresh: "r", Expiry: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	_, err := s.CredentialFor("x", func(context.Context, Token) (Token, error) { return Token{}, ErrInvalidClient })(context.Background())
	if !errors.Is(err, ErrInvalidClient) || !strings.Contains(err.Error(), "moca logout x") {
		t.Fatal(err)
	}
}

func TestNotLoggedIn(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "auth.json"))
	_, err := s.CredentialFor("openai", nil)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "moca login openai") {
		t.Fatal(err)
	}
}

func TestHostIDStable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	h1, err := s.HostID()
	if err != nil || !strings.HasPrefix(h1, "urn:uuid:") {
		t.Fatal(h1, err)
	}
	h2, err := NewStore(p).HostID()
	if err != nil || h2 != h1 {
		t.Fatal(h2, h1, err)
	}
	// the host id survives provider writes and logouts
	if err := s.Put("openai", Token{Access: "a", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("openai"); err != nil {
		t.Fatal(err)
	}
	if h3, _ := NewStore(p).HostID(); h3 != h1 {
		t.Fatal("host id lost", h3)
	}
}

func TestDeleteClearsCorruptStore(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(p, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewStore(p)
	if err := s.Delete("openai"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Get("openai"); err != nil || ok {
		t.Fatal(ok, err)
	}
}

func TestRefreshKeepsExtrasWhenNotEchoed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.Put("openai", Token{Access: "old", Refresh: "r1", Expiry: time.Now().Add(-time.Second),
		ClientID: "oaiapp_1", IDToken: "idt", Email: "u@e.com", AccountID: "sub1"}); err != nil {
		t.Fatal(err)
	}
	ref := func(context.Context, Token) (Token, error) {
		// a refresh response without rotation and without echoed extras
		return Token{Access: "new", Expiry: time.Now().Add(time.Hour)}, nil
	}
	c, err := s.CredentialFor("openai", ref)(context.Background())
	if err != nil || c.Token != "new" || !c.OAuth {
		t.Fatal(c, err)
	}
	tok, _, _ := s.Get("openai")
	if tok.Refresh != "r1" || tok.ClientID != "oaiapp_1" || tok.IDToken != "idt" || tok.Email != "u@e.com" || tok.AccountID != "sub1" {
		t.Fatal("extras lost", tok)
	}
}

func TestRefreshRotates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.Put("openai", Token{Access: "old", Refresh: "r1", Expiry: time.Now().Add(-time.Second), ClientID: "oaiapp_1"}); err != nil {
		t.Fatal(err)
	}
	ref := func(ctx context.Context, tok Token) (Token, error) {
		if tok.ClientID != "oaiapp_1" || tok.Refresh != "r1" {
			return Token{}, errors.New("refresher got wrong token")
		}
		return Token{Access: "new", Refresh: "r2", IDToken: "idt2", Expiry: time.Now().Add(time.Hour)}, nil
	}
	if _, err := s.CredentialFor("openai", ref)(context.Background()); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := s.Get("openai")
	if tok.Access != "new" || tok.Refresh != "r2" || tok.IDToken != "idt2" || tok.ClientID != "oaiapp_1" {
		t.Fatal("rotation not stored", tok)
	}
}

// A caller waiting for the store lock gives up when its context is done: a
// blocking flock could not be interrupted, so Ctrl-C would hang behind another
// process's refresh.
func TestCredentialForLockWaitHonoursContext(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.Put("x", Token{Access: "a", Refresh: "r", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockFile(context.Background(), p+".lock") // another moca process, mid-refresh
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	res := make(chan error, 1)
	go func() { _, err := s.CredentialFor("x", nil)(ctx); res <- err }()
	select {
	case err := <-res:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want the context error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CredentialFor stayed blocked on the store lock after its context ended")
	}
}

// The lock is released and re-acquirable once the holder lets go.
func TestLockFileWaitsThenAcquires(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	unlock, err := lockFile(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		u, err := lockFile(context.Background(), p)
		if err == nil {
			u()
		}
		got <- err
	}()
	time.Sleep(60 * time.Millisecond)
	select {
	case <-got:
		t.Fatal("second locker must wait while the lock is held")
	default:
	}
	unlock()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second locker never acquired the released lock")
	}
}

// A stalled token endpoint cannot hold the store lock past refreshTimeout.
func TestRefreshIsTimeBounded(t *testing.T) {
	old := refreshTimeout
	refreshTimeout = 50 * time.Millisecond
	defer func() { refreshTimeout = old }()
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.Put("x", Token{Access: "old", Refresh: "r", Expiry: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	stall := func(ctx context.Context, _ Token) (Token, error) { <-ctx.Done(); return Token{}, ctx.Err() }
	start := time.Now()
	_, err := s.CredentialFor("x", stall)(context.Background())
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatalf("stalled refresh must fail at the timeout: %v after %v", err, time.Since(start))
	}
}

// A transient refresh failure inside the pre-expiry window falls back to the
// still-valid token; once the token is truly expired it is an error; a
// terminal failure never falls back.
func TestTransientRefreshFailureFallsBack(t *testing.T) {
	boom := func(context.Context, Token) (Token, error) { return Token{}, errors.New("network down") }
	s := NewStore(filepath.Join(t.TempDir(), "auth.json"))
	if err := s.Put("x", Token{Access: "still-valid", Refresh: "r", Expiry: time.Now().Add(30 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	c, err := s.CredentialFor("x", boom)(context.Background())
	if err != nil || c.Token != "still-valid" {
		t.Fatalf("valid token must serve through a transient refresh failure: %v %v", c, err)
	}
	if err := s.Put("x", Token{Access: "dead", Refresh: "r", Expiry: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CredentialFor("x", boom)(context.Background()); err == nil || !strings.Contains(err.Error(), "refreshing x token") {
		t.Fatalf("expired token + failed refresh must error: %v", err)
	}
	if err := s.Put("x", Token{Access: "still-valid", Refresh: "r", Expiry: time.Now().Add(30 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	revoked := func(context.Context, Token) (Token, error) { return Token{}, ErrInvalidGrant }
	if _, err := s.CredentialFor("x", revoked)(context.Background()); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("a revoked session must not fall back to the old token: %v", err)
	}
}

// API keys live in the same store, beside OAuth tokens; the value is
// validated (empty and control-byte keys are refused) and logout clears
// whichever credential kind the provider has.
func TestAPIKeyStore(t *testing.T) {
	p := filepath.Join(t.TempDir(), "auth.json")
	s := NewStore(p)
	if err := s.PutAPIKey("anthropic", "sk-ant-1"); err != nil {
		t.Fatal(err)
	}
	if k, ok, err := s.GetAPIKey("anthropic"); err != nil || !ok || k != "sk-ant-1" {
		t.Fatalf("round-trip: %q %v %v", k, ok, err)
	}
	if err := s.PutAPIKey("anthropic", "sk-ant-2"); err != nil {
		t.Fatal(err)
	}
	if k, _, _ := s.GetAPIKey("anthropic"); k != "sk-ant-2" {
		t.Fatalf("replace: %q", k)
	}
	for _, bad := range []string{"", "a\nb", "a\x00b", "a\x7fb"} {
		if err := s.PutAPIKey("anthropic", bad); err == nil {
			t.Fatalf("key %q must be refused", bad)
		}
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode: %v %v", fi, err)
	}
	// Both credential kinds can coexist; Delete clears both.
	if err := s.Put("anthropic", Token{Access: "a", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("anthropic"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.GetAPIKey("anthropic"); ok {
		t.Fatal("api key must be gone")
	}
	if _, ok, _ := s.Get("anthropic"); ok {
		t.Fatal("token must be gone")
	}
}

// The v0.1 store lived in the data dir; the migration moves it to the config
// dir, is a no-op when the new file exists (or nothing is to move), and
// preserves the content and the 0600 mode.
func TestMigrateLegacyStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	legacy := filepath.Join(config.DataDir(), "auth.json")
	if err := os.MkdirAll(config.DataDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"hostId":"urn:uuid:x","providers":{"openai":{"access":"AT"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyStore(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy file must be gone: %v", err)
	}
	tok, ok, err := NewDefaultStore().Get("openai")
	if err != nil || !ok || tok.Access != "AT" {
		t.Fatalf("migrated token: %+v %v %v", tok, ok, err)
	}
	if fi, err := os.Stat(config.AuthFile()); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("migrated mode: %v %v", fi, err)
	}
	if h, err := NewDefaultStore().HostID(); err != nil || h != "urn:uuid:x" {
		t.Fatalf("host id lost: %q %v", h, err)
	}
	// Second run: nothing to move.
	if err := MigrateLegacyStore(); err != nil {
		t.Fatal(err)
	}
	// A new file wins over a leftover legacy one.
	if err := os.WriteFile(legacy, []byte(`{"providers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyStore(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("the new store exists: the legacy file must be left alone")
	}
}

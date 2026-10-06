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

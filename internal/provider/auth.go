package provider

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Token is one provider registration's credentials. The store keeps one
// entry per provider (v0.1 has no multi-account picker); everything beyond
// access/refresh/expiry is provider-specific extras — for openai's Sign in
// with ChatGPT flow: the issued client id (needed for refresh and
// reauthorization), the retained ID token (id_token_hint), the account email
// (display + login_hint) and the account subject (identity check).
type Token struct {
	Access    string    `json:"access"`
	Refresh   string    `json:"refresh,omitempty"`
	Expiry    time.Time `json:"expiry"`
	AccountID string    `json:"accountId,omitempty"`
	ClientID  string    `json:"clientId,omitempty"`
	IDToken   string    `json:"idToken,omitempty"`
	Email     string    `json:"email,omitempty"`
	Scopes    []string  `json:"scopes,omitempty"`
}

// Refresher exchanges a stored token for a fresh one. It receives the whole
// token because refresh endpoints may need provider extras (e.g. the issued
// client id). It returns ErrInvalidGrant when the session is unusable and
// ErrInvalidClient when the stored registration itself was rejected.
type Refresher func(ctx context.Context, t Token) (Token, error)

var (
	// ErrInvalidGrant: the refresh token is unusable (revoked, expired,
	// rotated away) — the user must log in again.
	ErrInvalidGrant = errors.New("invalid_grant")
	// ErrInvalidClient: the stored client registration was rejected — the
	// fix is a fresh registration (logout + login), not a refresh.
	ErrInvalidClient = errors.New("invalid_client")
)

// storeFile is the on-disk shape of auth.json. hostId is this host's
// ext_agent_host_id (Sign in with ChatGPT): stable per host, opaque, not a
// credential; it survives logout so a later sign-in reuses it.
type storeFile struct {
	HostID    string           `json:"hostId,omitempty"`
	Providers map[string]Token `json:"providers"`
}

type Store struct{ path string }

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) read() (storeFile, error) {
	f := storeFile{Providers: map[string]Token{}}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("auth store %s is unreadable (%v); run 'moca logout <provider>' or delete the file", s.path, err)
	}
	if f.Providers == nil {
		f.Providers = map[string]Token{}
	}
	return f, nil
}

// write publishes the store atomically (tmp + rename, forcing 0600). The
// rename replaces the file: a symlinked auth.json is deliberately neither
// followed nor preserved — this is moca's own credential store, not a
// dotfile-managed config file (unlike config's edit helpers, which edit
// through the link).
func (s *Store) write(f storeFile) error {
	if f.Providers == nil {
		f.Providers = map[string]Token{}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	// WriteFile's mode applies only when it creates the file: a pre-existing
	// tmp (crash debris, another tool) would otherwise be published with its
	// old mode through the rename. Force it, like config's writeEdited does.
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// withLock runs fn holding the cross-process store lock (<path>.lock). Every
// read-modify-write goes through it; credential refreshes hold it across
// read → refresh → write so two processes can never race a rotating token
// (a refresh token that rotates on use would otherwise be burned).
func (s *Store) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	unlock, err := lockFile(s.path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func (s *Store) Get(p string) (Token, bool, error) {
	f, err := s.read()
	if err != nil {
		return Token{}, false, err
	}
	t, ok := f.Providers[p]
	return t, ok, nil
}

func (s *Store) Put(p string, t Token) error {
	return s.withLock(func() error {
		f, err := s.read()
		if err != nil {
			return err
		}
		f.Providers[p] = t
		return s.write(f)
	})
}

// Delete removes a provider's registration (logout). A corrupt store is
// reset rather than blocking logout; the host id is kept when readable.
func (s *Store) Delete(p string) error {
	return s.withLock(func() error {
		f, err := s.read()
		if err != nil {
			f = storeFile{Providers: map[string]Token{}}
		}
		delete(f.Providers, p)
		return s.write(f)
	})
}

// HostID returns this host's stable ext_agent_host_id, generating and
// persisting it (urn:uuid form) on first use.
func (s *Store) HostID() (string, error) {
	var id string
	err := s.withLock(func() error {
		f, err := s.read()
		if err != nil {
			return err
		}
		if f.HostID == "" {
			if f.HostID, err = newHostID(); err != nil {
				return err
			}
			if err := s.write(f); err != nil {
				return err
			}
		}
		id = f.HostID
		return nil
	})
	return id, err
}

func newHostID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// CredentialFor returns a CredentialFunc that refreshes when the access
// token has under a minute left, holding the store lock across
// read → refresh → write. The refreshed token is written before the
// credential is returned, so a crash after the exchange never loses a
// rotated refresh token.
func (s *Store) CredentialFor(p string, refresh Refresher) CredentialFunc {
	return func(ctx context.Context) (Credential, error) {
		var tok Token
		err := s.withLock(func() error {
			f, err := s.read()
			if err != nil {
				return err
			}
			t, ok := f.Providers[p]
			if !ok {
				return fmt.Errorf("not logged in to %s: run moca login %s", p, p)
			}
			if time.Until(t.Expiry) > time.Minute || refresh == nil {
				tok = t
				return nil
			}
			nt, err := refresh(ctx, t)
			if errors.Is(err, ErrInvalidGrant) {
				return fmt.Errorf("session expired for %s: run moca login %s: %w", p, p, ErrInvalidGrant)
			}
			if errors.Is(err, ErrInvalidClient) {
				return fmt.Errorf("stored client registration for %s was rejected: run moca logout %s then moca login %s: %w", p, p, p, ErrInvalidClient)
			}
			if err != nil {
				return fmt.Errorf("refreshing %s token: %w", p, err)
			}
			// A rotating provider returns a replacement refresh token; a
			// non-rotating one omits it — keep the old value then. Same for
			// the registration extras the endpoint may not echo.
			if nt.Refresh == "" {
				nt.Refresh = t.Refresh
			}
			if nt.ClientID == "" {
				nt.ClientID = t.ClientID
			}
			if nt.IDToken == "" {
				nt.IDToken = t.IDToken
			}
			if nt.Email == "" {
				nt.Email = t.Email
			}
			if nt.AccountID == "" {
				nt.AccountID = t.AccountID
			}
			f.Providers[p] = nt
			tok = nt
			return s.write(f)
		})
		if err != nil {
			return Credential{}, err
		}
		return Credential{Token: tok.Access, OAuth: true}, nil
	}
}

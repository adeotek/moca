package permissions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/flock"
)

type TrustStore struct {
	mu   sync.Mutex
	path string
	m    map[string]bool
}

func LoadTrust(path string) (*TrustStore, error) {
	t := &TrustStore{path: path, m: map[string]bool{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return t, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &t.m); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TrustStore) Lookup(dir string) (bool, bool) {
	c, err := canonical(dir)
	if err != nil {
		return false, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	v, ok := t.m[c]
	return v, ok
}

// Set records one workdir's decision. The write is a read-modify-write of a
// file two moca processes can share (a TUI answering a prompt while an ACP
// server serves the same project), so it runs under an exclusive advisory lock
// and **re-reads the file first**: writing this process's startup map over the
// file erased a decision another process saved in between — a "no" given in
// the TUI could vanish, and the ACP server (which trusts a workdir it has no
// decision for) then opened it trusted. The temp-file rename alone never
// fixed that; only merging into the current file does.
func (t *TrustStore) Set(dir string, trusted bool) error {
	c, err := canonical(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	unlock, err := flock.Lock(ctx, t.path+".lock")
	if err != nil {
		return fmt.Errorf("trust store busy: %w", err)
	}
	defer unlock()

	// Merge into what is on disk now, not into the map this process loaded at
	// startup. A missing file is an empty store; a malformed one is an error
	// (clobbering it would silently discard every other project's decision).
	cur := map[string]bool{}
	if b, err := os.ReadFile(t.path); err == nil {
		if err := json.Unmarshal(b, &cur); err != nil {
			return fmt.Errorf("trust store %s is malformed: %w", t.path, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	cur[c] = trusted
	b, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return err
	}
	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, t.path); err != nil {
		return err
	}
	t.mu.Lock()
	t.m[c] = trusted
	t.mu.Unlock()
	return nil
}

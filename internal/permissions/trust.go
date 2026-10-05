package permissions

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
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

func (t *TrustStore) Set(dir string, trusted bool) error {
	c, err := canonical(dir)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.m[c] = trusted
	b, _ := json.MarshalIndent(t.m, "", "  ")
	t.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return err
	}
	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.path)
}

package tools

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"
	"time"
)

type stamp struct {
	mod  time.Time
	size int64
	sum  [32]byte
}

// ReadTracker remembers the on-disk state of every file read (or written) in
// this session. write/edit refuse existing files that were never read or
// that changed since (§4).
type ReadTracker struct {
	mu sync.Mutex
	m  map[string]stamp
}

func NewReadTracker() *ReadTracker { return &ReadTracker{m: map[string]stamp{}} }

func stampOf(abs string) (stamp, error) {
	f, err := os.Open(abs)
	if err != nil {
		return stamp{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return stamp{}, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return stamp{}, err
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return stamp{fi.ModTime(), fi.Size(), sum}, nil
}

func (t *ReadTracker) Record(abs string) error {
	s, err := stampOf(abs)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.m[abs] = s
	t.mu.Unlock()
	return nil
}

func (t *ReadTracker) Check(abs string) error {
	cur, err := stampOf(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	t.mu.Lock()
	prev, ok := t.m[abs]
	t.mu.Unlock()
	if !ok {
		return fmt.Errorf("%s exists and has not been read in this session; read it first", abs)
	}
	if prev.size != cur.size || prev.sum != cur.sum || !prev.mod.Equal(cur.mod) {
		return fmt.Errorf("%s changed on disk since you read it; re-read it before writing", abs)
	}
	return nil
}

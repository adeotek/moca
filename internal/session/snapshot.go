package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Snapshots struct {
	mu    sync.Mutex
	w     *Writer
	dir   string
	stack []SnapshotRec
}

func NewSnapshots(w *Writer, blobDir string, prior []Entry) *Snapshots {
	s := &Snapshots{w: w, dir: blobDir}
	for _, e := range prior {
		if e.Type != TypeSnapshot {
			continue
		}
		if e.Snapshot.Blob == "undo" {
			if n := len(s.stack); n > 0 {
				s.stack = s.stack[:n-1]
			}
			continue
		}
		s.stack = append(s.stack, *e.Snapshot)
	}
	return s
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func gitClean(abs string) bool {
	dir, file := filepath.Dir(abs), filepath.Base(abs)
	if exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", "--", file).Run() != nil {
		return false
	}
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain", "--", file).Output()
	return err == nil && len(strings.TrimSpace(string(out))) == 0
}

func (s *Snapshots) Guard(abs string, write func() error) error {
	rec := SnapshotRec{Path: abs}
	prev, err := os.ReadFile(abs)
	switch {
	case err == nil && gitClean(abs):
		rec.Git = true
	case err == nil:
		rec.Blob = sum(prev)
		if fi, err := os.Stat(abs); err == nil {
			rec.Mode = uint32(fi.Mode().Perm())
		}
		if err := os.MkdirAll(s.dir, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(s.dir, rec.Blob), prev, 0o600); err != nil {
			return err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := write(); err != nil {
		return err
	}
	if now, err := os.ReadFile(abs); err == nil {
		rec.After = sum(now)
	}
	s.mu.Lock()
	s.stack = append(s.stack, rec)
	s.mu.Unlock()
	_, err = s.w.Append(Entry{Type: TypeSnapshot, Snapshot: &rec})
	return err
}

func (s *Snapshots) Undo() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.stack) == 0 {
		return "", errors.New("nothing to undo in this session (changes made via shell are never covered)")
	}
	rec := s.stack[len(s.stack)-1]
	pop := func() {
		s.stack = s.stack[:len(s.stack)-1]
		s.w.Append(Entry{Type: TypeSnapshot, Snapshot: &SnapshotRec{Path: rec.Path, Blob: "undo"}})
	}
	if rec.Git {
		pop()
		return fmt.Sprintf("%s was clean in git — restore it with: git restore -- %s", rec.Path, rec.Path), nil
	}
	cur, err := os.ReadFile(rec.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if sum(cur) != rec.After {
		return "", fmt.Errorf("%s changed since moca wrote it; undo refused", rec.Path)
	}
	if rec.Blob == "" {
		if err := os.Remove(rec.Path); err != nil {
			return "", err
		}
		pop()
		return fmt.Sprintf("removed %s (moca created it)", rec.Path), nil
	}
	b, err := os.ReadFile(filepath.Join(s.dir, rec.Blob))
	if err != nil {
		return "", fmt.Errorf("snapshot blob missing (pruned?): %w", err)
	}
	mode := fs.FileMode(0o644)
	if rec.Mode != 0 {
		mode = fs.FileMode(rec.Mode)
	}
	if err := os.WriteFile(rec.Path, b, mode); err != nil {
		return "", err
	}
	if err := os.Chmod(rec.Path, mode); err != nil {
		return "", err
	}
	pop()
	return fmt.Sprintf("restored %s", rec.Path), nil
}

func Prune(blobDir string, days int) error {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	ents, err := os.ReadDir(blobDir)
	if err != nil {
		return nil
	}
	for _, e := range ents {
		if fi, err := e.Info(); err == nil && fi.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(blobDir, e.Name()))
		}
	}
	return nil
}

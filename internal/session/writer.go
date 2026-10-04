package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Writer struct {
	mu   sync.Mutex
	f    *os.File
	path string
	id8  string
	last string
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug: first five words of the prompt, lowercased, ASCII-only, ≤32 chars.
func Slug(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) > 5 {
		words = words[:5]
	}
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.Join(words, " ")), "-"), "-")
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	if s == "" {
		return "session"
	}
	return s
}

func Create(dir string, h Header, slug string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	os.Chmod(dir, 0o700)
	if h.StartedAt.IsZero() {
		h.StartedAt = time.Now()
	}
	for range 10 {
		id8 := newID()[:8]
		p := filepath.Join(dir, fmt.Sprintf("%s-%s-%s.jsonl", h.StartedAt.Format("2006-01-02"), slug, id8))
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		w := &Writer{f: f, path: p, id8: id8}
		if _, err := w.Append(Entry{Type: TypeSession, Session: &h}); err != nil {
			f.Close()
			return nil, err
		}
		return w, nil
	}
	return nil, errors.New("could not allocate a unique session id")
}

func Open(path string) (*Writer, []Entry, error) {
	entries, err := ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}
	base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	w := &Writer{f: f, path: path, id8: base[max(0, len(base)-8):]}
	if len(entries) > 0 {
		w.last = entries[len(entries)-1].ID
	}
	return w, entries, nil
}

func (w *Writer) Append(e Entry) (Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e.ID == "" {
		e.ID = newID()
	}
	e.ParentID = w.last
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	b, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	if _, err := w.f.Write(append(b, '\n')); err != nil {
		return e, err
	}
	w.last = e.ID
	return e, nil
}

func (w *Writer) Path() string { return w.path }
func (w *Writer) ID8() string  { return w.id8 }
func (w *Writer) Close() error { return w.f.Close() }

// ReadFile tolerates a truncated last line (crash mid-write).
func ReadFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			break
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

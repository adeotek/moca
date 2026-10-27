package session

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ErrInUse means another moca process holds the session file open for
// writing: two writers would interleave histories, and a resume would "repair"
// the other process's in-flight tool calls with synthetic results.
var ErrInUse = errors.New("session is open in another moca process")

type Writer struct {
	mu   sync.Mutex
	f    *os.File
	path string
	id8  string
	last string
	// A deferred writer (CreateLazy) has no file yet: the session is an id,
	// a path and the header in memory, and the file appears with the first
	// conversational entry. Until then, metadata entries (a model or mode
	// switch from the TUI) wait in pending and are written in order behind
	// the header, so a session that never gets a prompt leaves nothing on
	// disk (§8).
	dir     string
	slug    string
	head    Entry   // the session header entry, written first on materialize
	pending []Entry // entries appended before the file existed
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

// Create makes the session file at once and writes the header entry.
func Create(dir string, h Header, slug string) (*Writer, error) {
	w, err := newWriter(dir, h, slug)
	if err != nil {
		return nil, err
	}
	if err := w.materialize(); err != nil {
		return nil, err
	}
	return w, nil
}

// CreateLazy allocates a session without touching the disk: the id, the path
// and the header stay in memory and the file is created by the first entry
// that is not pre-conversation metadata (see Append). Opening moca and
// quitting without sending a prompt therefore leaves no session behind to
// resume or clean up (§8). Path and ID8 are final from here on: a caller may
// print them before the file exists, and Created says whether it does.
func CreateLazy(dir string, h Header, slug string) (*Writer, error) {
	return newWriter(dir, h, slug)
}

// newWriter allocates the session's identity: a random id8, the file name
// built from it and the header entry. Nothing is written.
func newWriter(dir string, h Header, slug string) (*Writer, error) {
	if h.StartedAt.IsZero() {
		h.StartedAt = time.Now()
	}
	w := &Writer{dir: dir, slug: slug}
	// The header's id is settled now, not at materialize time: entries
	// buffered before the file exists chain to it, and the chain must read
	// the same whether the first prompt or a metadata entry arrived first.
	w.head = Entry{Type: TypeSession, Session: &h, ID: newID(), Time: h.StartedAt}
	w.last = w.head.ID
	w.reid()
	return w, nil
}

// reid allocates a fresh id8 and the file name that carries it.
func (w *Writer) reid() {
	w.id8 = newID()[:8]
	w.path = filepath.Join(w.dir, fmt.Sprintf("%s-%s-%s.jsonl", w.head.Session.StartedAt.Format("2006-01-02"), w.slug, w.id8))
}

// Open reopens a session file for appending and returns its entries. The
// writer lock is taken first: trimming a "partial" last line or repairing
// calls while another process is mid-write would damage its session.
func Open(path string) (*Writer, []Entry, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		if errors.Is(err, ErrInUse) {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		return nil, nil, err
	}
	entries, err := ReadFile(path)
	if err == nil {
		err = trimPartialLine(path)
	}
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	w := &Writer{f: f, path: path, id8: base[max(0, len(base)-8):]}
	if len(entries) > 0 {
		w.last = entries[len(entries)-1].ID
	}
	return w, entries, nil
}

// trimPartialLine repairs a crash-truncated trailing line: a partial line is
// dropped (appending after it would hide every later entry from ReadFile);
// a complete entry missing only its newline gets one.
func trimPartialLine(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.Size() == 0 {
		return err
	}
	lastNL, err := lastNewline(f, fi.Size())
	if err != nil {
		return err
	}
	if lastNL == fi.Size() {
		return nil
	}
	tail := make([]byte, fi.Size()-lastNL)
	if _, err := f.ReadAt(tail, lastNL); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if json.Valid(tail) {
		_, err = f.WriteAt([]byte{'\n'}, fi.Size())
		return err
	}
	return f.Truncate(lastNL)
}

// lastNewline returns the offset just past the final '\n' (0 when there is
// none, size when the file ends with one).
func lastNewline(f *os.File, size int64) (int64, error) {
	const chunk = 64 << 10
	buf := make([]byte, chunk)
	for off := size; off > 0; {
		n := int64(chunk)
		if off < n {
			n = off
		}
		off -= n
		if _, err := f.ReadAt(buf[:n], off); err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}
		if i := bytes.LastIndexByte(buf[:n], '\n'); i >= 0 {
			return off + int64(i) + 1, nil
		}
	}
	return 0, nil
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
	if w.f == nil {
		if preConversation(e.Type) { // a model/mode switch: nothing to persist yet
			w.pending = append(w.pending, e)
			w.last = e.ID
			return e, nil
		}
		if err := w.materializeLocked(); err != nil {
			return e, err
		}
	}
	if err := w.write(e); err != nil {
		return e, err
	}
	w.last = e.ID
	return e, nil
}

// preConversation is an entry type that describes the session's settings
// rather than a turn: it is buffered until a real entry arrives, so changing
// the model or the mode in a TUI you then quit does not create a session.
func preConversation(t string) bool {
	return t == TypeModelChange || t == TypePermissionMode
}

// materialize creates the session file (and its lock) and writes the header
// followed by the entries buffered before it. A file that appears anyway — a
// name collision — gets a fresh id and another try.
func (w *Writer) materialize() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.materializeLocked()
}

func (w *Writer) materializeLocked() error {
	if w.f != nil {
		return nil
	}
	if err := os.MkdirAll(w.dir, 0o700); err != nil {
		return err
	}
	os.Chmod(w.dir, 0o700)
	for range 10 {
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
		if errors.Is(err, fs.ErrExist) {
			w.reid()
			continue
		}
		if err != nil {
			return err
		}
		if err := lockFile(f); err != nil {
			f.Close()
			return err
		}
		w.f = f
		// The header leads, then whatever was buffered behind it — in the
		// order it was appended, so the parentId chain and the replay read
		// the same session a prompt-first start would have written.
		for _, e := range append([]Entry{w.head}, w.pending...) {
			if err := w.write(e); err != nil {
				f.Close()
				w.f = nil
				return err
			}
		}
		w.pending = nil
		return nil
	}
	return errors.New("could not allocate a unique session id")
}

// write marshals e and appends it; the caller holds the lock.
func (w *Writer) write(e Entry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = w.f.Write(append(b, '\n'))
	return err
}

// Path is the session file's path. For a deferred session it names a file
// that does not exist yet (Created says which).
func (w *Writer) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.path
}

func (w *Writer) ID8() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.id8
}

// Created reports whether the session file exists yet: false for a deferred
// session nobody has prompted (§8).
func (w *Writer) Created() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f != nil
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil // a deferred session that never got a prompt: nothing to close
	}
	return w.f.Close()
}

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

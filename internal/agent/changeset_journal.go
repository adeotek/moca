package agent

// The changeset journal (SPECS §10.6, plan v7 §4.7). One JSONL file per
// session, appended and fsynced before the first write and after every landed
// file: a crash between two replacements leaves a record naming exactly which
// paths were approved, which landed and which never started. Resume reads it
// (phase 5); nothing here replays or undoes a change.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/tools"
)

type journalRec struct {
	Type   string   `json:"type"` // begin | record | abort | finish
	At     string   `json:"at"`
	CallID string   `json:"callId,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	Pre    []string `json:"pre,omitempty"`
	Post   []string `json:"post,omitempty"`
	Path   string   `json:"path,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// changesetJournal is the production binding of tools.ChangesetJournal.
type changesetJournal struct {
	mu   sync.Mutex
	path string
	f    *os.File
	open bool
}

func newChangesetJournal(path string) *changesetJournal { return &changesetJournal{path: path} }

func (j *changesetJournal) begin() error {
	if j.open {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(j.path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	j.f, j.open = f, true
	return nil
}

// write appends one record and fsyncs it: a record that is not on disk is not a
// record, and the whole point is to survive the next instruction.
func (j *changesetJournal) write(rec journalRec) error {
	rec.At = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := j.f.Write(b); err != nil {
		return err
	}
	return j.f.Sync()
}

func (j *changesetJournal) closeLocked() {
	if j.open {
		_ = j.f.Close()
		j.f, j.open = nil, false
	}
}

func (j *changesetJournal) Begin(b tools.ChangesetBatch) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := j.begin(); err != nil {
		return err
	}
	rec := journalRec{Type: "begin", CallID: b.CallID, Paths: b.Paths}
	for i := range b.Paths {
		rec.Pre = append(rec.Pre, hex.EncodeToString(b.Pre[i][:]))
		rec.Post = append(rec.Post, hex.EncodeToString(b.Post[i][:]))
	}
	if err := j.write(rec); err != nil {
		j.closeLocked()
		return err
	}
	return nil
}

func (j *changesetJournal) Record(path string, pre, post []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.write(journalRec{Type: "record", Path: path, Pre: []string{sumHex(pre)}, Post: []string{sumHex(post)}})
}

func (j *changesetJournal) Abort(reason string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	err := j.write(journalRec{Type: "abort", Reason: reason})
	j.closeLocked()
	return err
}

func (j *changesetJournal) Finish() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	err := j.write(journalRec{Type: "finish"})
	j.closeLocked()
	return err
}

func sumHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

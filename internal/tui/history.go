package tui

// Persistent prompt history: one JSON line per submitted message in
// history.jsonl (data dir, 0600), filtered by workdir on load so each project
// recalls its own prompts. Control-byte entries are not persisted (they only
// ever live behind a paste chip). Everything here is best-effort: a history
// that cannot be read or written must never get in the way of sending.

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

const (
	historyKeep    = 500  // entries recalled per workdir
	historyMaxLine = 8192 // longer messages are not persisted
	historyTrimAt  = 4000 // file lines that trigger a rewrite on load
	historyTrimTo  = 2000
)

type historyRec struct {
	WD   string `json:"wd"`
	Text string `json:"t"`
}

// loadHistory returns the workdir's recent messages, oldest first, with
// consecutive duplicates collapsed. An oversized file is trimmed in place.
func loadHistory(path, workdir string) []string {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	var recs []historyRec
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		var r historyRec
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.Text != "" {
			recs = append(recs, r)
		}
	}
	f.Close()
	if len(recs) > historyTrimAt {
		recs = recs[len(recs)-historyTrimTo:]
		rewriteHistory(path, recs)
	}
	var out []string
	for _, r := range recs {
		if r.WD != workdir {
			continue
		}
		if n := len(out); n > 0 && out[n-1] == r.Text {
			continue
		}
		out = append(out, r.Text)
	}
	if len(out) > historyKeep {
		out = slices.Clone(out[len(out)-historyKeep:])
	}
	return out
}

func rewriteHistory(path string, recs []historyRec) {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	w := bufio.NewWriter(f)
	for _, r := range recs {
		b, _ := json.Marshal(r)
		w.Write(append(b, '\n'))
	}
	if w.Flush() != nil || f.Close() != nil || os.Rename(tmp, path) != nil {
		os.Remove(tmp)
	}
}

// appendHistory records one submitted message.
func appendHistory(path, workdir, text string) {
	if path == "" || text == "" || len(text) > historyMaxLine || hasControl(text) {
		return
	}
	b, err := json.Marshal(historyRec{WD: workdir, Text: text})
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(b, '\n'))
}

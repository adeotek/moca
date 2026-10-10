// Package applog is moca's diagnostic log (SPECS §13.5): one logfmt file per
// process under <state>/logs, metadata only — never conversation content.
// Only cmd/moca imports it; every other package logs through the standard
// log/slog default logger, so no internal import edge exists.
package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Level int

const (
	LevelOff Level = iota
	LevelInfo
	LevelDebug
)

// ParseLevel reads a log.level / MOCA_LOG value ("" is the info default).
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "off":
		return LevelOff, nil
	}
	return LevelOff, fmt.Errorf("log level %q unknown (want info|debug|off)", s)
}

// Discard installs a handler that drops every record. cmd/moca calls it
// first: slog's built-in default writes to stderr, which would land on the
// TUI's screen or in test output before Open.
func Discard() { slog.SetDefault(slog.New(slog.DiscardHandler)) }

// fileMax caps one log file; a var so tests can shrink it.
var fileMax int64 = 10 << 20

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// Open prunes dir, creates this process's log file and installs it as the
// slog default. LevelOff touches nothing. On error the default handler is
// left as it was.
func Open(dir string, level Level, retentionDays int) (io.Closer, string, error) {
	if level == LevelOff {
		return nopCloser{}, "", nil
	}
	now := time.Now()
	prune(dir, retentionDays, now)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%d.log", now.Format("2006-01-02"), os.Getpid()))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, "", err
	}
	w := &cappedWriter{w: f, max: fileMax}
	lv := slog.LevelInfo
	if level == LevelDebug {
		lv = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lv, ReplaceAttr: replaceAttr})))
	return w, path, nil
}

// prune removes *.log files in dir older than days (days<=0 keeps all).
func prune(dir string, days int, now time.Time) {
	if days <= 0 {
		return
	}
	cutoff := now.AddDate(0, 0, -days)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !e.Type().IsRegular() || filepath.Ext(e.Name()) != ".log" {
			continue
		}
		if fi, err := e.Info(); err == nil && fi.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

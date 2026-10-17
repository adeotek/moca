package tools

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Stateful-file guard (§10): a shell command that opens a SQLite database
// whose -wal/-journal sidecar is present can destroy that sidecar — SQLite
// deletes a WAL it cannot parse when it opens the database, and a hot
// rollback journal is gone once it is applied. The working-style rules say
// copy first; weak models obey refusals, not prose (the §14 ship-gate
// lesson), so the shell tool refuses the open until the run has copied the
// database or its sidecar.
//
// Precision matters as much as the refusal: a syntactically valid WAL is
// normal recovery (SQLite replays it, nothing is lost), so only a sidecar
// SQLite cannot parse is refused — plus any rollback journal, whose rollback
// removes the file.

// sqliteValueFlags are the sqlite3(1) options that take a value: the word
// after them is not the database operand.
var sqliteValueFlags = map[string]bool{
	"-cmd": true, "-init": true, "-separator": true, "-nullvalue": true,
	"-newline": true, "-mmap": true, "-pagecache": true, "-vfs": true,
}

// sqliteSidecarGuard inspects a shell command and reports a refusal for the
// first SQLite open that would consume a sidecar SQLite cannot parse, plus
// the source paths of every byte copy the command makes. The shell tool
// marks those copied only after the command succeeds, so a failed copy
// cannot unlock an open later in the run.
//
// The analysis is static and best-effort: a command the parser rejects, or
// one whose operands are dynamic, fails open (the permission analyser and
// the shell itself still own those errors).
func sqliteSidecarGuard(env *Env, command string) (refusal string, copies []string) {
	f, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return "", nil
	}
	copied := map[string]bool{}
	syntax.Walk(f, func(node syntax.Node) bool {
		if refusal != "" {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		words := make([]guardWord, 0, len(call.Args))
		for _, w := range call.Args {
			words = append(words, guardWordOf(w))
		}
		if !words[0].literal {
			return true
		}
		args := words[1:]
		switch filepath.Base(words[0].lit) {
		case "cp":
			// A byte copy: the source is what later opens may treat as safe.
			if src, ok := firstOperand(args, nil); ok && src != "" {
				abs := resolveGuardPath(env.Root, src)
				if !copied[abs] {
					copied[abs] = true
					copies = append(copies, abs)
				}
			}
		case "sqlite3":
			if hasWord(args, "-readonly") {
				return true // a read-only open cannot delete the sidecar
			}
			db, ok := firstOperand(args, sqliteValueFlags)
			if !ok || db == "" || db == ":memory:" || db == "-" || strings.HasPrefix(db, "file:") {
				return true
			}
			abs := resolveGuardPath(env.Root, db)
			if side := unparseableSidecar(abs); side != "" && !copied[abs] && !copied[side] && !env.copiedPath(abs) && !env.copiedPath(side) {
				refusal = fmt.Sprintf("refused: `sqlite3 %s` would consume `%s` — SQLite deletes a wal/journal sidecar "+
					"it cannot parse when it opens the database, destroying the evidence. Copy the stateful files "+
					"first, e.g. `cp %s %s.bak && cp %s %s.bak-wal`, then work on the copies.",
					db, filepath.Base(side), db, db, filepath.Base(side), db)
			}
		}
		return true
	})
	return refusal, copies
}

type guardWord struct {
	lit     string
	literal bool
}

func guardWordOf(w *syntax.Word) guardWord {
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, dp := range p.Parts {
				l, ok := dp.(*syntax.Lit)
				if !ok {
					return guardWord{literal: false}
				}
				sb.WriteString(l.Value)
			}
		default:
			return guardWord{literal: false}
		}
	}
	return guardWord{lit: sb.String(), literal: true}
}

// firstOperand returns the first non-flag argument, skipping options listed
// in valueFlags together with their value. ok is false when a dynamic word
// sits before the operand: the path cannot be resolved statically.
func firstOperand(args []guardWord, valueFlags map[string]bool) (string, bool) {
	skip := false
	for _, w := range args {
		if skip {
			skip = false
			continue
		}
		if !w.literal {
			return "", false
		}
		if strings.HasPrefix(w.lit, "-") && w.lit != "-" {
			if valueFlags[w.lit] {
				skip = true
			}
			continue
		}
		return w.lit, true
	}
	return "", true
}

func hasWord(args []guardWord, want string) bool {
	for _, w := range args {
		if w.literal && w.lit == want {
			return true
		}
	}
	return false
}

func resolveGuardPath(root, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, p)
}

// unparseableSidecar returns the path of a -wal/-journal sidecar of db that
// SQLite would consume on open: any rollback journal (its rollback deletes
// the file), or a WAL without SQLite's header magic (SQLite cannot parse it
// and deletes it).
func unparseableSidecar(db string) string {
	if _, err := os.Stat(db + "-journal"); err == nil {
		return db + "-journal"
	}
	wal := db + "-wal"
	if _, err := os.Stat(wal); err != nil {
		return ""
	}
	if walHeaderOK(wal) {
		return "" // a valid WAL is normal recovery: SQLite replays it
	}
	return wal
}

// walHeaderOK reports SQLite's WAL magic (0x377f0682/0x377f0683, either
// byte order). Short or unreadable files are not a valid WAL.
func walHeaderOK(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var h [4]byte
	if _, err := io.ReadFull(f, h[:]); err != nil {
		return false
	}
	m := binary.BigEndian.Uint32(h[:])
	return m == 0x377f0682 || m == 0x377f0683
}

//go:build !windows

package tools

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sidecarDir builds a workdir with the sidecar shapes the guard must tell
// apart: an unparseable WAL, a valid WAL, a hot rollback journal, and a
// database with no sidecar at all.
func sidecarDir(t *testing.T) (*Env, string) {
	t.Helper()
	env, root := testEnv(t)
	env.Commands = &fakeCmds{}
	env.ShellEnv = os.Environ()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wal := make([]byte, 8)
	binary.BigEndian.PutUint32(wal, 0x377f0682) // SQLite WAL magic
	write("main.db", []byte("not really sqlite\n"))
	write("main.db-wal", []byte("xcrypted-wal")) // not a WAL header: SQLite would delete it
	write("ok.db", []byte("not really sqlite\n"))
	write("ok.db-wal", wal) // a valid WAL is normal recovery
	write("hot.db", []byte("not really sqlite\n"))
	write("hot.db-journal", []byte("rollback pages"))
	write("fresh.db", []byte("not really sqlite\n"))
	return env, root
}

func TestSQLiteSidecarGuard(t *testing.T) {
	env, root := sidecarDir(t)
	mainDB := filepath.Join(root, "main.db")
	cases := []struct {
		name   string
		cmd    string
		refuse bool
	}{
		{"unparseable wal", "sqlite3 main.db .tables", true},
		{"bench shape: read then open in one command", "xxd main.db-wal && sqlite3 main.db .tables", true},
		{"quoted path", `sqlite3 "main.db" .tables`, true},
		{"dynamic query after the operand", `sqlite3 main.db "$Q"`, true},
		{"value-taking option", "sqlite3 -cmd '.timeout 500' main.db .tables", true},
		{"valid wal is normal recovery", "sqlite3 ok.db .tables", false},
		{"no sidecar", "sqlite3 fresh.db .tables", false},
		{"hot rollback journal", "sqlite3 hot.db .tables", true},
		{"readonly open cannot delete it", "sqlite3 -readonly main.db .tables", false},
		{"in-memory", "sqlite3 :memory: 'select 1'", false},
		{"not a call word", "echo sqlite3 main.db", false},
		{"dynamic operand", `sqlite3 "$DB" .tables`, false},
		{"copy earlier in the same command", "cp main.db main.db.bak && sqlite3 main.db .tables", false},
		{"unrelated command", "cat main.db", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refusal, _ := sqliteSidecarGuard(env, tc.cmd)
			if got := refusal != ""; got != tc.refuse {
				t.Fatalf("%q: refusal=%v, want %v (%s)", tc.cmd, got, tc.refuse, refusal)
			}
			if tc.refuse && (!strings.Contains(refusal, "cp ") || !strings.Contains(refusal, "sidecar")) {
				t.Fatalf("refusal must name the fix and the hazard: %s", refusal)
			}
		})
	}
	// The same-command copy is reported for marking, and a copy made earlier
	// in the run unlocks the open.
	if _, copies := sqliteSidecarGuard(env, "cp main.db main.db.bak"); len(copies) != 1 || copies[0] != mainDB {
		t.Fatalf("copies = %v, want [%s]", copies, mainDB)
	}
	env.markCopied(mainDB)
	if refusal, _ := sqliteSidecarGuard(env, "sqlite3 main.db .tables"); refusal != "" {
		t.Fatalf("a copied database must be openable: %s", refusal)
	}
}

func TestShellToolRefusesStatefulOpen(t *testing.T) {
	env, _ := sidecarDir(t)
	r := run(t, shellTool{}, env, map[string]any{"command": "sqlite3 main.db .tables"})
	if !r.IsError || !strings.Contains(r.Content, "refused") || !strings.Contains(r.Content, "sidecar") {
		t.Fatalf("want a stateful-file refusal, got: %s", r.Content)
	}
	if !strings.Contains(r.Content, "cp main.db main.db.bak") {
		t.Fatalf("refusal must name the copy command: %s", r.Content)
	}
}

func TestShellToolMarksCopiedOnlyOnSuccess(t *testing.T) {
	env, root := sidecarDir(t)
	if r := run(t, shellTool{}, env, map[string]any{"command": "cp missing.db copy.bak"}); !r.IsError {
		t.Fatal("the failed cp should be an error result")
	}
	if env.copiedPath(filepath.Join(root, "missing.db")) {
		t.Fatal("a failed copy must not unlock an open")
	}
	if r := run(t, shellTool{}, env, map[string]any{"command": "cp main.db main.db.bak"}); r.IsError {
		t.Fatalf("cp should succeed: %s", r.Content)
	}
	if !env.copiedPath(filepath.Join(root, "main.db")) {
		t.Fatal("a successful copy must unlock the database")
	}
	if r := run(t, shellTool{}, env, map[string]any{"command": "sqlite3 main.db .tables"}); strings.Contains(r.Content, "refused") {
		t.Fatalf("copied database must not be refused: %s", r.Content)
	}
	env.BeginRun()
	if env.copiedPath(filepath.Join(root, "main.db")) {
		t.Fatal("BeginRun must reset the copied state (per-run discipline)")
	}
}

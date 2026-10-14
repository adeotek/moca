package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pruneEnv points the data dir at a temp dir holding two leftovers (an older
// content hash and a legacy version-keyed directory) and returns the
// builtin-skills dir.
func pruneEnv(t *testing.T) (base string) {
	t.Helper()
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	base = filepath.Join(data, "moca", "builtin-skills")
	for rel, body := range map[string]string{
		"aaaaaaaaaaaa/rtk/SKILL.md": "old",
		"0.0.0-dev/rtk/SKILL.md":    "legacy",
	} {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

func execPrune(t *testing.T, args ...string) (code int, out, errb string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(context.Background(), append([]string{"prune"}, args...), nil, &o, &e)
	return code, o.String(), e.String()
}

func TestRunPruneRemovesOlderBuiltinDirs(t *testing.T) {
	base := pruneEnv(t)
	code, out, errb := execPrune(t)
	if code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	for _, want := range []string{"removed " + filepath.Join(base, "aaaaaaaaaaaa"), "removed " + filepath.Join(base, "0.0.0-dev"), "pruned 2 entries"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	if ents, _ := os.ReadDir(base); len(ents) != 0 {
		t.Fatalf("leftovers survived: %v", ents)
	}
}

func TestRunPruneDryRunRemovesNothing(t *testing.T) {
	base := pruneEnv(t)
	code, out, errb := execPrune(t, "--dry-run")
	if code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	for _, want := range []string{"would remove " + filepath.Join(base, "aaaaaaaaaaaa"), "would remove 2 entries"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	if ents, _ := os.ReadDir(base); len(ents) != 2 {
		t.Fatalf("dry run must remove nothing: %v", ents)
	}
}

func TestRunPruneNothingToDo(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	code, out, _ := execPrune(t)
	if code != exitOK || !strings.Contains(out, "nothing to prune") {
		t.Fatalf("exit %d, stdout: %s", code, out)
	}
}

func TestRunPruneRejectsUnknownArgs(t *testing.T) {
	base := pruneEnv(t)
	for _, args := range [][]string{{"--bogus"}, {"extra"}} {
		code, _, errb := execPrune(t, args...)
		if code != exitUsage || !strings.Contains(errb, "usage: moca prune [--dry-run]") {
			t.Errorf("%v: exit %d, stderr: %s", args, code, errb)
		}
	}
	if ents, _ := os.ReadDir(base); len(ents) != 2 {
		t.Fatalf("a rejected invocation must remove nothing: %v", ents)
	}
}

// Like `update`, prune must not depend on a parseable config: a broken
// config may be exactly why the user is cleaning up.
func TestRunPruneIgnoresBrokenConfig(t *testing.T) {
	pruneEnv(t)
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "moca")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "config.jsonc"), []byte("{ this is not json"), 0o644)
	if code, _, errb := execPrune(t); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
}

func TestUsageListsPrune(t *testing.T) {
	var out bytes.Buffer
	printUsage(&out)
	if !strings.Contains(out.String(), "moca prune") {
		t.Fatalf("usage does not mention prune:\n%s", out.String())
	}
}

func TestFormatBytes(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 1536: "1.5 KiB", 5 << 20: "5.0 MiB"} {
		if got := formatBytes(in); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

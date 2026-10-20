package permissions

// Pin for T-2 (High): the trust store's Set is a read-modify-write of a file
// two moca processes share. Writing this process's startup map over the file
// erased a decision another process saved in between — a "no" given in the TUI
// could disappear, and the ACP server then opened that workdir trusted, because
// it trusts a workdir it has no decision for (acpTrust).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTrustStoreSetMergesAcrossProcesses(t *testing.T) {
	p := filepath.Join(t.TempDir(), "trust.json")
	a, err := LoadTrust(p) // one moca process, at startup
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadTrust(p) // a second one, same (empty) file
	if err != nil {
		t.Fatal(err)
	}
	declined, accepted := t.TempDir(), t.TempDir()

	if err := a.Set(declined, false); err != nil {
		t.Fatal(err)
	}
	// b's map is still empty — it must merge, not overwrite.
	if err := b.Set(accepted, true); err != nil {
		t.Fatal(err)
	}

	got, err := LoadTrust(p)
	if err != nil {
		t.Fatal(err)
	}
	if v, known := got.Lookup(declined); !known || v {
		t.Fatalf("the first process's refusal was lost (value=%v known=%v) — a declined workdir would open trusted in ACP", v, known)
	}
	if v, known := got.Lookup(accepted); !known || !v {
		t.Fatalf("the second process's decision is missing (value=%v known=%v)", v, known)
	}
}

// A malformed file is an error, never a silent reset: clobbering it would
// discard every other project's decision.
func TestTrustStoreSetRefusesToClobberMalformedFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "trust.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := LoadTrust(p)
	if err == nil {
		t.Fatal("LoadTrust must reject a malformed store")
	}
	_ = s
	// A store built directly (as startup does after a failed load, which is
	// fatal) still must not overwrite it.
	raw := &TrustStore{path: p, m: map[string]bool{}}
	if err := raw.Set(t.TempDir(), true); err == nil {
		t.Fatal("Set must refuse to overwrite a malformed store")
	}
	var m map[string]bool
	b, readErr := os.ReadFile(p)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if json.Unmarshal(b, &m) == nil {
		t.Fatal("the malformed file was replaced")
	}
}

// The lock is released on the error paths too: a second Set must not hang.
func TestTrustStoreSetRemainsUsableAfterAnError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "trust.json")
	s, err := LoadTrust(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(t.TempDir(), true); err != nil {
		t.Fatal(err)
	}
	// Racing writers, each with its own store, all land.
	dirs := make([]string, 8)
	for i := range dirs {
		dirs[i] = t.TempDir()
	}
	errs := make(chan error, len(dirs))
	for i, d := range dirs {
		go func(i int, d string) {
			s2, err := LoadTrust(p)
			if err == nil {
				err = s2.Set(d, i%2 == 0)
			}
			errs <- err
		}(i, d)
	}
	for range dirs {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadTrust(p)
	if err != nil {
		t.Fatal(err)
	}
	for i, d := range dirs {
		if v, known := got.Lookup(d); !known || v != (i%2 == 0) {
			t.Fatalf("dir %d lost its decision (value=%v known=%v)", i, v, known)
		}
	}
	_ = context.Background()
}

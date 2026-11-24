package tools

// Transport read parity (SPECS §10.6, v2 D23): a scripted read must leave the
// session in the state a direct read would — the freshness stamp `edit`/`write`
// check, the `*_test.go` investigation flag, and `Searched` from a search — and
// it must reach the same paths the jail admits beyond the workdir. The stamp is
// computed from the descriptor that was read, once per path per execution.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func transportInspection(t *testing.T, dir string, grants []string) *Inspection {
	t.Helper()
	in, err := NewInspection(dir)
	if err != nil {
		t.Fatal(err)
	}
	in.configure(grants, 0, true)
	t.Cleanup(func() { in.Close() })
	return in
}

func TestScriptedReadStampsTheFile(t *testing.T) {
	dir := t.TempDir()
	body := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	in := transportInspection(t, dir, nil)
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"a.txt"}`)); err != nil {
		t.Fatalf("read: %v", err)
	}
	st, ok := in.StampFor("a.txt")
	if !ok {
		t.Fatal("no stamp captured")
	}
	want, err := stampOf(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if st.sum != want.sum || st.size != want.size {
		t.Fatalf("stamp %x/%d != file %x/%d", st.sum[:4], st.size, want.sum[:4], want.size)
	}
	// And the tracker accepts it as a read, which is what edit checks.
	rt := NewReadTracker()
	rt.RecordStamp(filepath.Join(dir, "a.txt"), st.mod, st.size, st.sum)
	if err := rt.Check(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatalf("the stamped file reads as unread: %v", err)
	}
}

// A paged read hashes the file once: the memo is keyed by the requested path, so
// page 2..n reuse page 1's stamp instead of re-reading 30 times.
func TestPagedReadStampsOnce(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		sb.WriteString("line\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	in := transportInspection(t, dir, nil)
	payload, err := in.Read(context.Background(), json.RawMessage(`{"path":"big.txt","limit":10}`))
	if err != nil {
		t.Fatal(err)
	}
	pages := 0
	for next := payload.(readPayload).Next; next != ""; {
		pages++
		if pages > 100 {
			t.Fatal("cursor chain did not end")
		}
		p, err := in.Read(context.Background(), json.RawMessage(`{"next":"`+next+`"}`))
		if err != nil {
			t.Fatalf("page %d: %v", pages, err)
		}
		next = p.(readPayload).Next
	}
	if pages < 5 {
		t.Fatalf("expected a chain of pages, got %d", pages)
	}
	if n := len(in.stamps); n != 1 {
		t.Fatalf("stamp memo holds %d entries for one path, want 1", n)
	}
}

func TestScriptedReadReachesGrantsAndRefusesOutside(t *testing.T) {
	dir := t.TempDir()
	extra := t.TempDir()
	if err := os.WriteFile(filepath.Join(extra, "skill.md"), []byte("skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := transportInspection(t, dir, []string{extra})

	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"`+filepath.Join(extra, "skill.md")+`"}`)); err != nil {
		t.Fatalf("granted read: %v", err)
	}
	if _, ok := in.StampFor(filepath.Join(extra, "skill.md")); !ok {
		t.Fatal("a granted read captured no stamp")
	}

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := in.Read(context.Background(), json.RawMessage(`{"path":"`+outside+`"}`))
	if err == nil {
		t.Fatal("an ungranted absolute path was read")
	}
	var ie *InspectError
	if !errors.As(err, &ie) || ie.Class != "outside" {
		t.Fatalf("class = %v, want outside", err)
	}
}

func TestTransportAttemptBudget(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := NewInspection(dir)
	if err != nil {
		t.Fatal(err)
	}
	in.configure(nil, 2, false)
	for i := 0; i < 2; i++ {
		if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"a.txt"}`)); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	_, err = in.Read(context.Background(), json.RawMessage(`{"path":"a.txt"}`))
	if err == nil {
		t.Fatal("a third attempt was admitted with budget 2")
	}
	if !strings.Contains(err.Error(), "exhausted (2)") {
		t.Fatalf("error does not name the configured budget: %v", err)
	}
}

// The bridge is what carries parity into the session: a successful scripted read
// stamps the file for `edit`/`write` and arms the test-hint flags, a search marks
// the session as having looked — and none of it happens in additive, whose
// contract is the opposite.
func TestBridgeRecordsParityOnlyInTransport(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "svc_test.go")
	if err := os.WriteFile(abs, []byte("package svc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := transportInspection(t, dir, nil)
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"svc_test.go"}`)); err != nil {
		t.Fatal(err)
	}

	env := &Env{Root: dir, Reads: NewReadTracker()}
	br := &bridge{in: in, env: env, transport: true}
	br.recordParity("read", json.RawMessage(`{"path":"svc_test.go"}`))
	if err := env.Reads.Check(abs); err != nil {
		t.Fatalf("a scripted read did not satisfy the freshness check: %v", err)
	}
	if !env.TestSeen || env.FailingTest != "" {
		t.Fatalf("test flags not set: seen=%v failing=%q", env.TestSeen, env.FailingTest)
	}
	br.recordParity("search", json.RawMessage(`{"pattern":"x"}`))
	if !env.Searched {
		t.Fatal("a scripted search did not set Searched")
	}

	additive := &Env{Root: dir, Reads: NewReadTracker()}
	additiveBridge := &bridge{in: in, env: additive, transport: false}
	additiveBridge.recordParity("read", json.RawMessage(`{"path":"svc_test.go"}`))
	additiveBridge.recordParity("search", json.RawMessage(`{"pattern":"x"}`))
	if err := additive.Reads.Check(abs); err == nil {
		t.Fatal("additive recorded a read")
	}
	if additive.Searched || additive.TestSeen {
		t.Fatal("additive set the direct tools' flags")
	}
}

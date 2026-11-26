package tools

// Grant scope (SPECS §10.6): external grants are directory boundaries that only
// the transport surface receives, and a file is never a grant. The regression
// this file pins: a grant on moca's own config file used to be widened to its
// parent directory, so a script could read auth.json — provider credentials and
// plaintext API keys — beside it. That path was reachable from additive too,
// because grants were applied whenever a budget was set.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// holderInspection builds the Inspection exactly as a session does, through the
// holder, so the surface (not a hand-set field) decides what a script may reach.
func holderInspection(t *testing.T, dir string, transport bool, grants ...string) *Inspection {
	t.Helper()
	h := &CodemodeHolder{Attempts: 400, Transport: transport, Grants: grants}
	in := h.inspection(dir)
	t.Cleanup(func() { in.Close() })
	return in
}

func readAbs(t *testing.T, in *Inspection, abs string) error {
	t.Helper()
	args, err := json.Marshal(map[string]string{"path": abs})
	if err != nil {
		t.Fatal(err)
	}
	_, err = in.Read(context.Background(), args)
	return err
}

func wantOutside(t *testing.T, err error, what string) {
	t.Helper()
	var ie *InspectError
	if !errors.As(err, &ie) || ie.Class != "outside" {
		t.Fatalf("%s: err = %v, want an outside refusal", what, err)
	}
}

// A config-shaped fixture: the granted file, the credential store beside it, and
// the workspace the session runs in.
func grantFixture(t *testing.T) (work, cfgDir, cfgFile, authFile string) {
	t.Helper()
	work = t.TempDir()
	cfgDir = t.TempDir()
	cfgFile = filepath.Join(cfgDir, "config.jsonc")
	authFile = filepath.Join(cfgDir, "auth.json")
	if err := os.WriteFile(cfgFile, []byte("{\"model\":\"fake/m\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authFile, []byte(`{"apiKeys":{"fake":"sk-must-not-leave-the-host"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return work, cfgDir, cfgFile, authFile
}

// The P0 regression: the config file is a jail read root, and granting it must
// not hand a transport script the directory it lives in.
func TestAFileGrantIsNotWidenedToItsDirectory(t *testing.T) {
	work, _, cfgFile, authFile := grantFixture(t)
	for _, transport := range []bool{true, false} {
		in := holderInspection(t, work, transport, cfgFile)
		wantOutside(t, readAbs(t, in, authFile),
			"a sibling of the granted file")
		wantOutside(t, readAbs(t, in, cfgFile),
			"the granted file itself (a file is not a grant)")
	}
}

// Additive keeps its budget and gets no external roots at all, whatever the
// holder was handed: the grant list is a transport right.
func TestAdditiveHasNoExternalGrants(t *testing.T) {
	work, cfgDir, cfgFile, authFile := grantFixture(t)
	prompts := filepath.Join(cfgDir, "prompts")
	if err := os.MkdirAll(prompts, 0o700); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(prompts, "probe.md")
	if err := os.WriteFile(probe, []byte("PROMPT-CANARY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	in := holderInspection(t, work, false, cfgFile, prompts, cfgDir)
	for _, abs := range []string{authFile, probe, cfgFile} {
		wantOutside(t, readAbs(t, in, abs), "additive read of "+abs)
	}
}

// Transport still reaches a granted directory and everything inside it — the
// prompts and skills roots exist so a script can consult reference material.
func TestTransportReadsInsideADirectoryGrant(t *testing.T) {
	work, cfgDir, _, _ := grantFixture(t)
	prompts := filepath.Join(cfgDir, "prompts")
	if err := os.MkdirAll(prompts, 0o700); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(prompts, "probe.md")
	if err := os.WriteFile(probe, []byte("PROMPT-CANARY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	in := holderInspection(t, work, true, prompts)
	args, err := json.Marshal(map[string]string{"path": probe})
	if err != nil {
		t.Fatal(err)
	}
	v, err := in.Read(context.Background(), args)
	if err != nil {
		t.Fatalf("transport read of a granted directory failed: %v", err)
	}
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "PROMPT-CANARY") {
		t.Fatalf("granted read did not return the file: %s", body)
	}
}

// A directory grant covers its own subtree and nothing beside it.
func TestDirectoryGrantDoesNotCoverSiblings(t *testing.T) {
	work := t.TempDir()
	granted := t.TempDir()
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(granted, "in.txt"), []byte("IN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(other, "out.txt")
	if err := os.WriteFile(outside, []byte("OUT\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	in := holderInspection(t, work, true, granted)
	if err := readAbs(t, in, filepath.Join(granted, "in.txt")); err != nil {
		t.Fatalf("inside a directory grant must read: %v", err)
	}
	wantOutside(t, readAbs(t, in, outside), "a sibling directory")
	// The grant's own path is a directory, not a file.
	wantOutside(t, readAbs(t, in, granted), "the grant directory itself")
}

// A grant that is missing when the inspection first touches the filesystem
// grants nothing: the grant set is resolved once, at that first use, and is not
// retried into existence afterwards — nor is its parent opened on its behalf.
func TestMissingGrantGrantsNothing(t *testing.T) {
	work, cfgDir, _, _ := grantFixture(t)
	if err := os.WriteFile(filepath.Join(work, "seed.txt"), []byte("SEED\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(cfgDir, "not-there")
	in := holderInspection(t, work, true, missing)
	// The first read is what opens the workspace (and resolves the grants). It
	// is workspace-relative: absolute paths are the grant path, by definition.
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"seed.txt"}`)); err != nil {
		t.Fatalf("workspace read: %v", err)
	}
	if err := os.MkdirAll(missing, 0o700); err != nil {
		t.Fatal(err)
	}
	late := filepath.Join(missing, "late.txt")
	if err := os.WriteFile(late, []byte("LATE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantOutside(t, readAbs(t, in, late), "a directory created after the grant set was resolved")
}

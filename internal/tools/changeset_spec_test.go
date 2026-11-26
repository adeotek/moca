package tools

// The edits variant's spec and inventory (plan v7 §4.12). The description is
// frozen: it is what every request in a writable workspace session carries, and
// its own 768 B cap is the one that moved for it (the 576/640 caps and both
// read-only goldens are untouched).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/codemode"
)

const editsDescription = "Workspace JS. Await tools.read({path}|{next})\u2192{lines:[{n,text}]}, " +
	"tools.search({pattern})\u2192{matches:[{path,line,text}]}, " +
	"tools.ls({path})\u2192{entries:[{name,kind}]}, " +
	"tools.glob({pattern})\u2192{paths}. " +
	"Await tools.stageEdit({path,edits:[{old,new}]})\u2192{staged,path,edits} to stage exact edits " +
	"the host applies after this program returns; read the file first; staged is not written; " +
	"sha256(text)\u2192hex is available. " +
	"Return the answer; a bare expression is null."

func TestCodemodeWorkspaceEditsSpec(t *testing.T) {
	spec := codemodeSpecEdits()
	if spec.Name != "codemode" {
		t.Fatalf("name = %q", spec.Name)
	}
	if spec.Description != editsDescription {
		t.Fatalf("description drifted from the frozen text:\n%q", spec.Description)
	}
	if n := len(spec.Description) + 1; n > 768 {
		t.Fatalf("the edits description is %d B with its newline, over the 768 B cap", n)
	}
	// The table and the description must agree in both directions: a method the
	// table can dispatch is named, and a named method exists.
	caps := workspaceCapabilities(true)
	for name := range caps {
		if !strings.Contains(spec.Description, "tools."+name) {
			t.Fatalf("%s is dispatchable but not named in the description", name)
		}
	}
	for _, name := range []string{"read", "search", "ls", "glob", "stageEdit"} {
		if _, ok := caps[name]; !ok {
			t.Fatalf("%s is named but not dispatchable", name)
		}
	}
	// No other surface acquires the clause.
	if strings.Contains(codemodeSpec(false).Description, "stageEdit") ||
		strings.Contains(codemodeSpec(true).Description, "stageEdit") {
		t.Fatal("a read-only spec advertises stageEdit")
	}
	if _, ok := workspaceCapabilities(false)["stageEdit"]; ok {
		t.Fatal("the read-only table carries stageEdit")
	}
}

func TestStageEditDispatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x := 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &bridge{in: mustInspection(dir), caps: workspaceCapabilities(true),
		cs: NewChangeset(ChangesetPolicy{Edits: true, Trusted: true, Mode: "yolo"}, dir)}
	defer b.cs.Close()
	out, cerr := b.dispatch(context.Background(), "stageEdit", json.RawMessage(`{"path":"a.go","edits":[{"old":"x := 1","new":"x := 2"}]}`))
	if cerr != nil {
		t.Fatalf("dispatch: %v", cerr)
	}
	if !strings.Contains(string(out), `"staged":true`) {
		t.Fatalf("result = %s", out)
	}
	// A mismatch is correctable and travels as a typed error.
	_, cerr = b.dispatch(context.Background(), "stageEdit", json.RawMessage(`{"path":"a.go","edits":[{"old":"nope","new":"x"}]}`))
	if cerr == nil || cerr.Class != "mismatch" {
		t.Fatalf("mismatch class = %v", cerr)
	}
	// A read-only table cannot dispatch it at all.
	ro := &bridge{in: mustInspection(dir), caps: workspaceCapabilities(false)}
	_, cerr = ro.dispatch(context.Background(), "stageEdit", json.RawMessage(`{"path":"a.go","edits":[{"old":"x := 1","new":"x := 2"}]}`))
	if cerr == nil || !strings.Contains(cerr.Message, "no such tool") {
		t.Fatalf("read-only dispatch = %v", cerr)
	}
}

// The hook itself: a program that staged a file has its batch applied after it
// returns, and the provider sees the guest text plus the receipt.
func TestCodmodeRunAppliesAStagedBatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x := 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	journal := &fakeJournal{}
	env := &Env{Root: dir, Snap: &csFakeSnap{}, Journal: journal, Reads: NewReadTracker()}
	holder := &CodemodeHolder{
		Open:   func() (CodemodeEngine, error) { return &stagingEngine{t: t}, nil },
		Caps:   codemode.ExecCaps{Edits: true},
		Policy: ChangesetPolicy{Edits: true, Trusted: true, Mode: "yolo"},
	}
	res := NewCodemode(holder).Run(context.Background(), env, json.RawMessage(`{"code":"1"}`))
	if res.IsError {
		t.Fatalf("run: %s", res.Content)
	}
	if !strings.Contains(res.Content, "[changeset applied]") {
		t.Fatalf("content = %q", res.Content)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "a.go"))
	if string(got) != "x := 2\n" {
		t.Fatalf("disk = %q", got)
	}
	if !journal.finished || len(journal.records) != 1 {
		t.Fatalf("journal = %+v", journal)
	}
}

// stagingEngine stages one file before returning, the way a guest script would.
type stagingEngine struct{ t *testing.T }

func (s *stagingEngine) NewPrelude(codemode.ExecCaps) string { return "// prelude" }

func (s *stagingEngine) Execute(ctx context.Context, caps codemode.ExecCaps, src string, dispatch codemode.DispatchFunc, timeout time.Duration) (codemode.Result, *codemode.CallError) {
	if _, cerr := dispatch(ctx, "stageEdit", json.RawMessage(`{"path":"a.go","edits":[{"old":"x := 1","new":"x := 2"}]}`)); cerr != nil {
		s.t.Fatalf("stage: %v", cerr)
	}
	return codemode.Result{Value: json.RawMessage(`"done"`)}, nil
}

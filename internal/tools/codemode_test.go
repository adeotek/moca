package tools

// The codemode adapter's contract: what the model can reach, what it cannot,
// and what happens to the result when the script fails.

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/codemode"
)

// fakeEngine records the capabilities it was given, so a test can prove the
// prelude and the execution saw the same value (R38) without a reactor.
type fakeEngine struct {
	preludeCaps codemode.ExecCaps
	execCaps    codemode.ExecCaps
	preludes    int
	executes    int
	src         string
	result      codemode.Result
	err         *codemode.CallError
	dispatch    codemode.DispatchFunc
}

func (f *fakeEngine) NewPrelude(caps codemode.ExecCaps) string {
	f.preludeCaps = caps
	f.preludes++
	return "// prelude"
}

func (f *fakeEngine) Execute(ctx context.Context, caps codemode.ExecCaps, src string, dispatch codemode.DispatchFunc, timeout time.Duration) (codemode.Result, *codemode.CallError) {
	f.execCaps, f.src, f.dispatch, f.executes = caps, src, dispatch, f.executes+1
	return f.result, f.err
}

func newEnv(t *testing.T) (*Env, string) {
	t.Helper()
	dir := t.TempDir()
	return &Env{Root: dir, Paths: rootChecker{dir}, Reads: NewReadTracker()}, dir
}

// ---- the spec ------------------------------------------------------------

// The whole spec is the F17 form and it has a hard size budget: every byte is
// sent on every request and a description edit resets the prompt cache.
func TestCodemodeSpec(t *testing.T) {
	const golden = "testdata/codemode-schema.golden.json"
	spec := codemodeSpec()
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw = append(raw, '\n')
	if *update {
		if err := os.WriteFile(golden, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("no golden (run with -update): %v", err)
	}
	if string(raw) != string(want) {
		t.Fatalf("spec changed:\n got %s\nwant %s", raw, want)
	}
	if n := len(raw); n > 512 {
		t.Fatalf("whole spec is %d bytes, budget is 512", n)
	}
	if spec.Name != "codemode" {
		t.Fatalf("name = %q", spec.Name)
	}
}

// The refusal-only factory ships the same bytes: a session without codemode must
// not see a different tool list.
func TestCodemodeRefusalHasTheSameSpec(t *testing.T) {
	a, _ := json.Marshal(NewCodemodeRefusal().Spec())
	b, _ := json.Marshal(codemodeSpec())
	if string(a) != string(b) {
		t.Fatalf("refusal spec differs:\n%s\n%s", a, b)
	}
}

// ---- the capability table ------------------------------------------------

func TestCodemodeCapabilityTableIsWorkspaceOnly(t *testing.T) {
	caps := workspaceCapabilities()
	want := map[string]bool{"read": true, "search": true, "ls": true}
	if len(caps) != len(want) {
		t.Fatalf("table has %d names, want %d", len(caps), len(want))
	}
	for name, c := range caps {
		if !want[name] {
			t.Fatalf("unexpected capability %q", name)
		}
		if c.name != name {
			t.Fatalf("%s carries name %q", name, c.name)
		}
		if c.network {
			t.Fatalf("%s is a network capability in v1", name)
		}
	}
	for _, forbidden := range []string{"write", "edit", "shell", "mcp", "web", "codemode"} {
		if _, ok := caps[forbidden]; ok {
			t.Fatalf("the table exposes %q", forbidden)
		}
	}
}

// A guest frame for a name outside the table is refused by the host, not
// ignored, and it never reaches a tool.
func TestCodemodeDispatchRefusesUnknownNames(t *testing.T) {
	env, _ := newEnv(t)
	b := &bridge{in: mustInspection(env.Root), caps: workspaceCapabilities()}
	defer b.in.Close()
	for _, name := range []string{"write", "shell", "mcp__x", "codemode"} {
		_, cerr := b.dispatch(context.Background(), name, json.RawMessage(`{}`))
		if cerr == nil || cerr.Class != "refused" {
			t.Fatalf("%s => %v, want refused", name, cerr)
		}
	}
	if b.calls != 0 {
		t.Fatalf("a refused name counted as a call: %d", b.calls)
	}
}

// ---- admission -----------------------------------------------------------

func TestCodemodeNilAdmitAdmits(t *testing.T) {
	env, _ := newEnv(t)
	eng := &fakeEngine{result: codemode.Result{Value: json.RawMessage(`42`)}}
	tool := NewCodemode(&CodemodeHolder{Engine: eng})
	res := tool.Run(context.Background(), env, json.RawMessage(`{"code":"1+1"}`))
	if res.IsError || res.Steered {
		t.Fatalf("nil Admit refused the call: %+v", res)
	}
	if eng.executes != 1 {
		t.Fatalf("execute calls = %d", eng.executes)
	}
}

func TestCodemodeNotAdmittedDoesNotRun(t *testing.T) {
	env, _ := newEnv(t)
	env.Admit = func() bool { return false }
	eng := &fakeEngine{}
	res := NewCodemode(&CodemodeHolder{Engine: eng}).Run(context.Background(), env, json.RawMessage(`{"code":"1+1"}`))
	if !res.Steered {
		t.Fatal("a refused admission did not set Steered")
	}
	if eng.executes != 0 {
		t.Fatalf("the engine ran while steering was pending (%d)", eng.executes)
	}
}

// Admission can be lost while the script runs: the result must say so even when
// the script succeeded, because the agent has to deliver the steering message.
func TestCodemodeSteeredAfterExecute(t *testing.T) {
	env, _ := newEnv(t)
	calls := 0
	env.Admit = func() bool { calls++; return calls == 1 }
	eng := &fakeEngine{result: codemode.Result{Value: json.RawMessage(`1`)}}
	res := NewCodemode(&CodemodeHolder{Engine: eng}).Run(context.Background(), env, json.RawMessage(`{"code":"1"}`))
	if !res.Steered {
		t.Fatal("admission lost mid-call was not reported")
	}
	if eng.executes != 1 {
		t.Fatalf("execute calls = %d", eng.executes)
	}
}

// ---- results -------------------------------------------------------------

// A timeout after text() keeps the accepted output: the guest's epilogue is
// lost, not what it already printed.
func TestCodemodeTimeoutKeepsAcceptedText(t *testing.T) {
	env, _ := newEnv(t)
	eng := &fakeEngine{
		result: codemode.Result{Texts: []string{"partial result"}},
		err:    &codemode.CallError{Class: "timeout", Message: "the script exceeded its deadline"},
	}
	res := NewCodemode(&CodemodeHolder{Engine: eng}).Run(context.Background(), env, json.RawMessage(`{"code":"while(1){}"}`))
	if !res.IsError {
		t.Fatal("a timeout was reported as success")
	}
	if !strings.Contains(res.Content, "failed: timeout") || !strings.Contains(res.Content, "partial result") {
		t.Fatalf("content lost the status or the text: %q", res.Content)
	}
	if !strings.Contains(res.Content, "[codemode: 0 calls]") {
		t.Fatalf("a failed call must carry the ledger: %q", res.Content)
	}
}

func TestCodemodeClampsLongOutput(t *testing.T) {
	env, _ := newEnv(t)
	var texts []string
	for i := 0; i < 400; i++ {
		texts = append(texts, strings.Repeat("x", 100))
	}
	eng := &fakeEngine{result: codemode.Result{Texts: texts, Value: json.RawMessage(`{"k":"` + strings.Repeat("v", 5000) + `"}`)}}
	res := NewCodemode(&CodemodeHolder{Engine: eng}).Run(context.Background(), env, json.RawMessage(`{"code":"1"}`))
	if n := utf8.RuneCountInString(res.Content); n > codemodeResultMax+len("…[truncated]") {
		t.Fatalf("result is %d runes, cap is %d", n, codemodeResultMax)
	}
	if !strings.Contains(res.Content, "truncated") {
		t.Fatal("a clamped result does not say it was clamped")
	}
}

// A clean run that read everything says nothing about budgets it did not touch.
func TestCodemodeLedgerOnlyWhenItMatters(t *testing.T) {
	env, _ := newEnv(t)
	clean := &fakeEngine{result: codemode.Result{Value: json.RawMessage(`1`)}}
	res := NewCodemode(&CodemodeHolder{Engine: clean}).Run(context.Background(), env, json.RawMessage(`{"code":"1"}`))
	if strings.Contains(res.Content, "[codemode:") {
		t.Fatalf("a clean run carried a ledger: %q", res.Content)
	}
	failed := &fakeEngine{err: &codemode.CallError{Class: "script", Message: "boom"}}
	res = NewCodemode(&CodemodeHolder{Engine: failed}).Run(context.Background(), env, json.RawMessage(`{"code":"1"}`))
	if !strings.Contains(res.Content, "[codemode:") {
		t.Fatalf("a failed run has no ledger: %q", res.Content)
	}
}

// ---- the container -------------------------------------------------------

// The nested path asks nothing, spills nothing and records no read: a script's
// inspection is not the model's inspection, and read-then-direct-edit must still
// require its own direct read.
func TestCodemodeAsksNothingSpillsNothingAndRecordsNoRead(t *testing.T) {
	env, dir := newEnv(t)
	abs := writeFile(t, dir, "a.txt", "hello\n")
	env.SpillDir = t.TempDir()
	env.Ask = func(context.Context, Question) Answer {
		t.Fatal("a nested call asked for approval")
		return Deny
	}
	eng := &fakeEngine{result: codemode.Result{Value: json.RawMessage(`1`)}}
	NewCodemode(&CodemodeHolder{Engine: eng}).Run(context.Background(), env, json.RawMessage(`{"code":"1"}`))

	// The bridge the adapter builds is the real dispatch path; exercise it
	// directly so the assertions are about that code and not a stub.
	b := &bridge{in: mustInspection(env.Root), caps: workspaceCapabilities()}
	defer b.in.Close()
	if _, cerr := b.dispatch(context.Background(), "read", json.RawMessage(`{"path":"a.txt"}`)); cerr != nil {
		t.Fatalf("dispatch: %v", cerr)
	}
	if err := env.Reads.Check(abs); err == nil {
		t.Fatal("a nested read recorded a direct read — a following edit would be allowed")
	}
	entries, err := os.ReadDir(env.SpillDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the nested path spilled %d files", len(entries))
	}
}

// ---- R38 -----------------------------------------------------------------

// The same ExecCaps value must reach NewPrelude and Execute: the prelude the
// model was shown and the capabilities it gets are one decision, not two.
func TestCodemodeSameCapsForPreludeAndExecute(t *testing.T) {
	env, _ := newEnv(t)
	eng := &fakeEngine{result: codemode.Result{Value: json.RawMessage(`1`)}}
	caps := codemode.ExecCaps{NetworkPrelude: true, Network: false}
	holder := &CodemodeHolder{Engine: eng, Caps: caps}
	tool := NewCodemode(holder)
	if p := holder.Prelude(); p == "" {
		t.Fatal("no prelude")
	}
	tool.Run(context.Background(), env, json.RawMessage(`{"code":"1"}`))
	if eng.preludes != 1 || eng.executes != 1 {
		t.Fatalf("preludes=%d executes=%d", eng.preludes, eng.executes)
	}
	if eng.preludeCaps != eng.execCaps {
		t.Fatalf("caps drifted: prelude %+v, execute %+v", eng.preludeCaps, eng.execCaps)
	}
	if eng.preludeCaps != caps {
		t.Fatalf("caps = %+v, want %+v", eng.preludeCaps, caps)
	}
	// Memoized: a second call renders nothing new and cannot drift.
	if holder.Prelude() != eng.NewPrelude(caps) {
		t.Fatal("the memoized prelude differs from a fresh render")
	}
	if eng.preludes != 2 {
		t.Fatalf("preludes = %d", eng.preludes)
	}
}

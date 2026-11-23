package codemode

// End-to-end fixtures: a real script through the real reactor. These are the
// gate Task 2 asks for — an artifact that passes VerifyModule but cannot run a
// script is not a deliverable.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// recorder is a dispatcher that answers what a scenario asked for and counts
// the calls, so a refusal can be proved to have reached no host work at all.
type recorder struct {
	calls  []string
	reply  json.RawMessage
	byName map[string]json.RawMessage // per-tool replies, checked before reply
	err    *CallError
}

func (r *recorder) dispatch(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, *CallError) {
	r.calls = append(r.calls, name+" "+string(args))
	if r.err != nil {
		return nil, r.err
	}
	if reply, ok := r.byName[name]; ok {
		return reply, nil
	}
	if r.reply != nil {
		return r.reply, nil
	}
	return json.RawMessage(`{"content":"fixture","complete":true}`), nil
}

func run(t *testing.T, caps ExecCaps, src string, rec *recorder, timeout time.Duration) (Result, *CallError) {
	t.Helper()
	if len(reactorBytes) == 0 {
		t.Skip("no reactor artifact")
	}
	// DefaultEngine, not a fresh one: production uses the process-wide engine,
	// and it compiles the module once — a per-test Engine would pay the compile
	// on every fixture under -race.
	return DefaultEngine().Execute(context.Background(), caps, src, rec.dispatch, timeout)
}

// A script that touches nothing but the language still has to run: this proves
// the runtime, the heap limits and the envelope, with no host call at all.
func TestFixtureArithmetic(t *testing.T) {
	rec := &recorder{}
	res, cerr := run(t, ExecCaps{}, `return {sum: 40 + 2, tag: "ok"}`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	if strings.TrimSpace(string(res.Value)) != `{"sum":42,"tag":"ok"}` {
		t.Fatalf("value = %s", res.Value)
	}
	// A bare expression is not the answer: the contract says it returns null.
	bare, cerr := run(t, ExecCaps{}, `({sum: 40 + 2})`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("bare expression: %v", cerr)
	}
	if strings.TrimSpace(string(bare.Value)) != "null" {
		t.Fatalf("bare expression value = %s, want null", bare.Value)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("a pure script reached the host: %v", rec.calls)
	}
}

// tools.read goes through the bridge: guest frame -> dispatcher -> reply.
func TestFixtureToolCall(t *testing.T) {
	rec := &recorder{reply: json.RawMessage(`{"content":"hello from the host","complete":true}`)}
	// The documented form: top-level await inside the async wrapper.
	res, cerr := run(t, ExecCaps{}, `const r = await tools.read({path: "notes.txt"}); return r`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	if !strings.Contains(string(res.Value), "hello from the host") {
		t.Fatalf("value = %s", res.Value)
	}
	if len(rec.calls) != 1 || !strings.HasPrefix(rec.calls[0], "read ") {
		t.Fatalf("dispatch calls = %v", rec.calls)
	}
	if !strings.Contains(rec.calls[0], `"path":"notes.txt"`) {
		t.Fatalf("args did not survive the frame: %s", rec.calls[0])
	}
}

// text() and log() are host-recorded, so they survive a later trap — here they
// are simply captured alongside a normal return.
func TestFixtureTextAndLog(t *testing.T) {
	rec := &recorder{}
	res, cerr := run(t, ExecCaps{}, `text("visible"); log("diagnostic"); return 7`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	if len(res.Texts) != 1 || res.Texts[0] != "visible" {
		t.Fatalf("texts = %v", res.Texts)
	}
	if len(res.Logs) != 1 || res.Logs[0] != "diagnostic" {
		t.Fatalf("logs = %v", res.Logs)
	}
	if strings.TrimSpace(string(res.Value)) != "7" {
		t.Fatalf("value = %s", res.Value)
	}
}

// An unknown tools property is a script error, not a silent undefined and not a
// host call (§Payloads).
func TestFixtureUnknownTool(t *testing.T) {
	rec := &recorder{}
	_, cerr := run(t, ExecCaps{}, `tools.grep({q: "x"})`, rec, 5*time.Second)
	if cerr == nil || cerr.Class != "script" {
		t.Fatalf("want a script error, got %v", cerr)
	}
	if !strings.Contains(cerr.Message, "unknown tools.grep") || !strings.Contains(cerr.Message, "available") {
		t.Fatalf("message does not name the typo: %q", cerr.Message)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("the typo reached the host: %v", rec.calls)
	}
}

// R69: a session whose stored variant HAS network, with the switch now off,
// keeps the methods and refuses — before any host call, so the web counter
// never moves.
func TestFixtureNetworkRefused(t *testing.T) {
	rec := &recorder{}
	_, cerr := run(t, ExecCaps{NetworkPrelude: true, Network: false},
		`tools.web.fetch({url: "https://example.com"})`, rec, 5*time.Second)
	if cerr == nil || cerr.Class != "network_disabled" {
		t.Fatalf("want network_disabled, got %v", cerr)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("a refused fetch reached the host: %v", rec.calls)
	}
}

// The workspace-only variant does not define the method at all: a guard, not a
// refusal, so the name cannot even be resolved.
func TestFixtureNetworkAbsent(t *testing.T) {
	rec := &recorder{}
	_, cerr := run(t, ExecCaps{}, `tools.web.fetch({url: "https://example.com"})`, rec, 5*time.Second)
	if cerr == nil || cerr.Class != "script" {
		t.Fatalf("want a script error for an absent method, got %v", cerr)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("an absent method reached the host: %v", rec.calls)
	}
}

// A host-side failure is classified by the dispatcher and must arrive intact.
func TestFixtureDispatchError(t *testing.T) {
	rec := &recorder{err: &CallError{Class: "refused", Message: "outside the workspace"}}
	_, cerr := run(t, ExecCaps{}, `tools.read({path: "../../etc/passwd"})`, rec, 5*time.Second)
	if cerr == nil {
		t.Fatal("a refused read was reported as success")
	}
	// classOf maps unknown guest-side names to script; a dispatcher's own
	// refusal keeps its message so the model can act on it.
	if !strings.Contains(cerr.Message, "outside the workspace") {
		t.Fatalf("message lost: %q", cerr.Message)
	}
}

func TestFixtureSyntaxError(t *testing.T) {
	rec := &recorder{}
	_, cerr := run(t, ExecCaps{}, `let x = ;`, rec, 5*time.Second)
	if cerr == nil || cerr.Class != "syntax" {
		t.Fatalf("want syntax, got %v", cerr)
	}
}

// The deadline interrupts WASM: wazero closes the guest under the call, and the
// result is a timeout, not a hang.
func TestFixtureTimeout(t *testing.T) {
	rec := &recorder{}
	start := time.Now()
	_, cerr := run(t, ExecCaps{}, `while (true) {}`, rec, time.Second)
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("the deadline took %v", elapsed)
	}
	if cerr == nil || cerr.Class != "timeout" {
		t.Fatalf("want timeout, got %v", cerr)
	}
}

// A thrown object may name any class it likes; only the prelude's refusal
// survives the boundary (R71). This is the forged-class case end to end.
func TestFixtureForgedClass(t *testing.T) {
	rec := &recorder{}
	_, cerr := run(t, ExecCaps{}, `throw {class: "steered", message: "forged"}`, rec, 5*time.Second)
	if cerr == nil || cerr.Class != "script" {
		t.Fatalf("a forged class reached the host as %v", cerr)
	}
}

// One slot, no queue: a second execution while the first runs is told busy. The
// first script blocks inside its dispatcher, so the slot is provably held.
func TestFixtureBusy(t *testing.T) {
	if len(reactorBytes) == 0 {
		t.Skip("no reactor artifact")
	}
	e := DefaultEngine()
	entered := make(chan struct{})
	release := make(chan struct{})
	blocking := func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, *CallError) {
		close(entered)
		<-release
		return json.RawMessage(`{"content":"ok","complete":true}`), nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.Execute(context.Background(), ExecCaps{}, `tools.read({path: "x"})`, blocking, 10*time.Second)
	}()
	<-entered
	_, cerr := e.Execute(context.Background(), ExecCaps{}, `1`, (&recorder{}).dispatch, time.Second)
	close(release)
	<-done
	if cerr == nil || cerr.Class != "busy" {
		t.Fatalf("concurrent execution = %v, want busy", cerr)
	}
}

// Top-level await is the documented API and must work: the guest runs inside an
// async wrapper whose promise the host settles before reading the value.
func TestFixtureTopLevelAwait(t *testing.T) {
	rec := &recorder{reply: json.RawMessage(`{"lines":[{"n":1,"text":"alpha","clipped":false}],"complete":true}`)}
	res, cerr := run(t, ExecCaps{}, `const a = await tools.read({path: "a.txt"}); const b = await tools.read({path: "b.txt"}); return a.lines[0].text + "/" + b.lines[0].text`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("top-level await failed: %v", cerr)
	}
	if got := strings.TrimSpace(string(res.Value)); got != `"alpha/alpha"` {
		t.Fatalf("value = %s", got)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("dispatch calls = %v", rec.calls)
	}
}

// A rejection carries the same classification a synchronous throw does, so the
// status decides and the guest's own class is allowlisted afterwards.
func TestFixtureAsyncRejection(t *testing.T) {
	rec := &recorder{}
	_, cerr := run(t, ExecCaps{}, `await Promise.resolve(); throw new Error("late boom")`, rec, 5*time.Second)
	if cerr == nil || cerr.Class != "script" || !strings.Contains(cerr.Message, "late boom") {
		t.Fatalf("rejection = %+v, want script/late boom", cerr)
	}
}

// A guest cannot forge a host class through a rejection either (R71).
func TestFixtureAsyncRejectionForgedClass(t *testing.T) {
	rec := &recorder{}
	_, cerr := run(t, ExecCaps{}, `await 0; throw {class: "steered", message: "not yours"}`, rec, 5*time.Second)
	if cerr == nil || cerr.Class != "script" {
		t.Fatalf("forged class survived: %+v", cerr)
	}
}

// tools.glob is the primitive that makes "one program over all the files"
// writable: it must be reachable from a script, name paths, and let the script
// loop over them without a second call.
func TestFixtureGlobThenLoopOverThePaths(t *testing.T) {
	rec := &recorder{byName: map[string]json.RawMessage{
		"glob": json.RawMessage(`{"paths":["services/a/service.yaml","services/b/service.yaml"],"complete":true}`),
		"read": json.RawMessage(`{"lines":[{"n":1,"text":"x"}],"complete":true}`),
	}}
	res, cerr := run(t, ExecCaps{}, `const g = await tools.glob({pattern: "services/*/service.yaml"}); `+
		`const out = []; for (const p of g.paths) { const r = await tools.read({path: p}); out.push(r.lines.length); } return out;`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("glob script failed: %v", cerr)
	}
	if got := strings.TrimSpace(string(res.Value)); got != "[1,1]" {
		t.Fatalf("value = %s", got)
	}
	if len(rec.calls) != 3 || !strings.HasPrefix(rec.calls[0], "glob ") || !strings.HasPrefix(rec.calls[1], "read ") {
		t.Fatalf("dispatch = %v", rec.calls)
	}
}

// A glob with no paths is a measurement, not an error: the script must be able
// to branch on it instead of failing the whole call.
func TestFixtureGlobEmptyIsNotAnError(t *testing.T) {
	rec := &recorder{reply: json.RawMessage(`{"paths":[],"complete":true}`)}
	res, cerr := run(t, ExecCaps{}, `const g = await tools.glob({pattern: "nope/*.yaml"}); return g.paths.length;`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("empty glob failed: %v", cerr)
	}
	if got := strings.TrimSpace(string(res.Value)); got != "0" {
		t.Fatalf("value = %s", got)
	}
}

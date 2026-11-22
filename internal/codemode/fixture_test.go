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
	calls []string
	reply json.RawMessage
	err   *CallError
}

func (r *recorder) dispatch(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, *CallError) {
	r.calls = append(r.calls, name+" "+string(args))
	if r.err != nil {
		return nil, r.err
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
	res, cerr := run(t, ExecCaps{}, `({sum: 40 + 2, tag: "ok"})`, rec, 5*time.Second)
	if cerr != nil {
		t.Fatalf("unexpected error: %v", cerr)
	}
	if strings.TrimSpace(string(res.Value)) != `{"sum":42,"tag":"ok"}` {
		t.Fatalf("value = %s", res.Value)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("a pure script reached the host: %v", rec.calls)
	}
}

// tools.read goes through the bridge: guest frame -> dispatcher -> reply.
func TestFixtureToolCall(t *testing.T) {
	rec := &recorder{reply: json.RawMessage(`{"content":"hello from the host","complete":true}`)}
	res, cerr := run(t, ExecCaps{}, `tools.read({path: "notes.txt"})`, rec, 5*time.Second)
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
	res, cerr := run(t, ExecCaps{}, `text("visible"); log("diagnostic"); 7`, rec, 5*time.Second)
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

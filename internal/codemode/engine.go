package codemode

// The engine: a process-wide wazero runtime and compiled reactor, a fresh guest
// instance and one execution slot per script (plan v13, §Lifecycle and F18).
//
// The Go API here is the leaf package's whole surface, defined in Task 2 so
// Task 4 does not have to invent it: CallError, DispatchFunc, Result, ExecCaps,
// plus DefaultEngine/NewPrelude/Execute.

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// ErrNoReactor means this build has no reactor artifact. The shipped binary
// embeds reactor/qjs-moca-reactor.wasm (reactor/BUILD.md); until that artifact
// exists the feature must fail loudly instead of pretending to run a script.
var ErrNoReactor = errors.New("codemode: the reactor WASM is not embedded in this build (see internal/codemode/reactor/BUILD.md)")

// The deadline band. The model may set 1–30 s (the tool schema); a non-positive
// value falls back to the default, and execDeadline clamps before the task
// starts (R25/R54).
const (
	DefaultTimeout = 10 * time.Second
	MinTimeout     = 1 * time.Second
	MaxTimeout     = 30 * time.Second
)

// CallError is a classified guest failure. Class is one of the §Failure ABI
// classes; Line is the guest line when the runtime reported one.
type CallError struct {
	Class   string `json:"class"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

func (e *CallError) Error() string {
	if e == nil {
		return ""
	}
	if e.Line > 0 {
		return fmt.Sprintf("%s (line %d): %s", e.Class, e.Line, e.Message)
	}
	return e.Class + ": " + e.Message
}

// DispatchFunc answers one inspection or web frame. The engine never maps a
// guest name onto a tool itself: the adapter decides what exists, so the
// capability table lives in one place.
type DispatchFunc func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, *CallError)

// Result is the accepted output of one script. Texts and Logs are host-owned
// copies taken as the script ran, so a later trap loses only the epilogue.
type Result struct {
	Value   json.RawMessage
	Texts   []string
	Logs    []string
	Steered bool
}

// ExecCaps is the immutable per-execution capability handoff (R24/R39). The
// same value reaches NewPrelude and Execute:
//
//	NetworkPrelude  the session header's stored variant — does the guest get
//	                tools.web.* at all? (D10: the wire matches the prompt)
//	Network         is nested web admitted for THIS execution? (R69: the header
//	                variant AND the current effective switch)
//
// A resumed network-header session with the switch now off is {true,false}:
// the methods exist and refuse with network_disabled. A workspace-only session
// is {false,false} and never sees them.
type ExecCaps struct {
	NetworkPrelude bool
	Network        bool
	// Edits admits tools.stageEdit: the workspace surface's writable half. It is
	// off unless the session declared workspace + workspaceEdits, and the prelude
	// defines the method only when it is on (the same cut-the-source rule the web
	// block follows).
	Edits bool
}

// Engine owns the process-wide runtime, the compiled module and the slot.
// Creating it compiles nothing and opens no workspace root.
type Engine struct {
	mu       sync.Mutex
	rt       wazero.Runtime
	compiled wazero.CompiledModule
	busy     bool
}

var (
	defaultOnce   sync.Once
	defaultEngine *Engine
)

// DefaultEngine returns the process-wide handle. WASM initialization stays
// lazy: the first Execute compiles the module.
func DefaultEngine() *Engine {
	defaultOnce.Do(func() { defaultEngine = &Engine{} })
	return defaultEngine
}

// reactorBytes is the embedded artifact: quickjs-ng v0.9.0 plus moca_host.c,
// built by reactor/build.sh (wasi-sdk 29.0, both pins verified). VerifyModule
// checks it against abi.json at first use and in TestReactorIsEmbedded.
//
//go:embed reactor/qjs-moca-reactor.wasm
var reactorBytes []byte

// NewPrelude renders the guest prelude for one execution's capabilities.
func (e *Engine) NewPrelude(caps ExecCaps) string { return renderPrelude(caps) }

// execDeadline clamps the model's timeout to the documented band at task start
// (R25): the guest never runs past it, whatever a later host call does.
func execDeadline(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if timeout < MinTimeout {
		timeout = MinTimeout
	}
	if timeout > MaxTimeout {
		timeout = MaxTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

// Execute runs one script. The slot is non-blocking: a second caller gets
// `busy` rather than a queue, because a script that waits is indistinguishable
// from one that hangs.
func (e *Engine) Execute(ctx context.Context, caps ExecCaps, src string, dispatch DispatchFunc, timeout time.Duration) (Result, *CallError) {
	if len(reactorBytes) == 0 {
		return Result{}, &CallError{Class: "crash", Message: ErrNoReactor.Error()}
	}
	if dispatch == nil {
		return Result{}, &CallError{Class: "script", Message: "no dispatcher"}
	}
	e.mu.Lock()
	if e.busy {
		e.mu.Unlock()
		return Result{}, &CallError{Class: "busy", Message: "another script is running"}
	}
	e.busy = true
	e.mu.Unlock()
	// Release the slot only after all host work has unwound: a dispatch already
	// in flight may still return, and a second script must not see the process
	// mid-call (§Bridge).
	defer func() {
		e.mu.Lock()
		e.busy = false
		e.mu.Unlock()
	}()

	// Compile and instantiate under the CALLER's context, not the script's
	// deadline: the first execution in a process pays for the module compile,
	// and charging that to a 1 s script budget would fail a script that runs in
	// milliseconds.
	inst, cerr := e.instance(ctx)
	if cerr != nil {
		return Result{}, cerr
	}
	defer inst.close(context.Background())

	// The deadline starts when the SCRIPT does: the first execution in a process
	// pays for the module compile, and charging that to a 1 s script budget would
	// fail a script that runs in milliseconds.
	cctx, cancel := execDeadline(ctx, timeout)
	defer cancel()
	return inst.run(cctx, caps, src, dispatch)
}

// instance compiles the module once and creates one guest for this script.
func (e *Engine) instance(ctx context.Context) (*instance, *CallError) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.rt == nil {
		rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
			WithMemoryLimitPages(uint32(TheABI.Limits.LinearMemoryPages)).
			WithCloseOnContextDone(true))
		// WASI is instantiated for the handful of calls the reactor imports
		// (clock, fd_write for its own diagnostics, random). Its stdio is
		// discarded: a guest's output reaches the model through the bridge, never
		// through our stdout.
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
			rt.Close(ctx)
			return nil, &CallError{Class: "crash", Message: "instantiate wasi: " + err.Error()}
		}
		if err := registerHost(ctx, rt); err != nil {
			rt.Close(ctx)
			return nil, &CallError{Class: "crash", Message: err.Error()}
		}
		if err := VerifyModule(reactorBytes); err != nil {
			rt.Close(ctx)
			return nil, &CallError{Class: "crash", Message: err.Error()}
		}
		compiled, err := rt.CompileModule(ctx, reactorBytes)
		if err != nil {
			rt.Close(ctx)
			return nil, &CallError{Class: "crash", Message: "compile reactor: " + err.Error()}
		}
		e.rt, e.compiled = rt, compiled
	}
	// A reactor is initialized, never started: _initialize runs, _start must not.
	mod, err := e.rt.InstantiateModule(ctx, e.compiled, wazero.NewModuleConfig().
		WithStartFunctions("_initialize").
		WithStdout(io.Discard).WithStderr(io.Discard))
	if err != nil {
		return nil, &CallError{Class: "crash", Message: "instantiate reactor: " + err.Error()}
	}
	inst := &instance{mod: mod}
	if cerr := inst.init(ctx); cerr != nil {
		inst.close(ctx)
		return nil, cerr
	}
	return inst, nil
}

// instance is one guest: its exports plus the scratch buffer qjs_run writes the
// envelope into.
type instance struct {
	mod     api.Module
	mem     api.Memory
	qjsRun  api.Function
	allocFn api.Function
	freeFn  api.Function
	outPtr  uint32
	outCap  uint32
}

func (in *instance) init(ctx context.Context) *CallError {
	in.mem = in.mod.Memory()
	in.qjsRun = in.mod.ExportedFunction("qjs_run")
	in.allocFn = in.mod.ExportedFunction("moca_alloc")
	in.freeFn = in.mod.ExportedFunction("moca_free")
	initFn := in.mod.ExportedFunction("qjs_init")
	for name, fn := range map[string]api.Function{
		"qjs_run": in.qjsRun, "moca_alloc": in.allocFn, "qjs_init": initFn,
		"qjs_destroy": in.mod.ExportedFunction("qjs_destroy"),
	} {
		if fn == nil {
			return &CallError{Class: "crash", Message: "reactor has no " + name}
		}
	}
	results, err := initFn.Call(ctx, uint64(TheABI.Limits.JSHeapBytes), uint64(TheABI.Limits.JSStackBytes))
	if err != nil {
		return &CallError{Class: "crash", Message: "qjs_init: " + err.Error()}
	}
	if len(results) != 1 || int32(results[0]) != 0 {
		return &CallError{Class: "crash", Message: "qjs_init refused the heap/stack limits"}
	}
	cap := uint64(TheABI.Limits.ResponseBytes)
	res, err := in.allocFn.Call(ctx, cap)
	if err != nil || len(res) != 1 {
		return &CallError{Class: "crash", Message: "moca_alloc for the result buffer failed"}
	}
	in.outPtr, in.outCap = uint32(res[0]), uint32(cap)
	return nil
}

func (in *instance) close(ctx context.Context) {
	if in.mod != nil {
		_ = in.mod.Close(ctx)
	}
}

// run copies the source in, calls qjs_run and decodes the envelope. The host
// keeps the accepted events, so a trap still returns what the script already
// printed (§Failure ABI).
func (in *instance) run(ctx context.Context, caps ExecCaps, src string, dispatch DispatchFunc) (Result, *CallError) {
	if len(src) > TheABI.Limits.RequestBytes {
		return Result{}, &CallError{Class: "limit", Message: "script is larger than 64 KiB"}
	}
	cctx, st := withHost(ctx, dispatch)
	cctx = withMemory(cctx, in.mem)

	// The prelude is prepended on the Go side: the guest evaluates one script,
	// and the model's own source keeps its own line numbering from there on.
	full := renderPrelude(caps) + "\n" + src
	srcPtr, cerr := in.write(cctx, []byte(full))
	if cerr != nil {
		return Result{}, cerr
	}
	defer in.free(cctx, srcPtr)
	lenPtr, cerr := in.alloc(cctx, 4)
	if cerr != nil {
		return Result{}, cerr
	}
	defer in.free(cctx, lenPtr)

	res, err := in.qjsRun.Call(cctx, uint64(srcPtr), uint64(len(full)),
		uint64(in.outPtr), uint64(in.outCap), uint64(lenPtr))
	out := Result{Texts: st.texts, Logs: st.logs}
	if err != nil {
		// A trap or a deadline: wasm is gone, but the host already owns what
		// the script emitted.
		if cctx.Err() != nil {
			return out, &CallError{Class: "timeout", Message: "the script exceeded its deadline"}
		}
		return out, &CallError{Class: "crash", Message: err.Error()}
	}
	status := int32(res[0])
	n := readU32(in, lenPtr)
	if int(n) > TheABI.Limits.ResponseBytes {
		return out, &CallError{Class: "crash", Message: "the reactor reported an oversized envelope"}
	}
	raw, ok := in.mem.Read(in.outPtr, n)
	if !ok {
		return out, &CallError{Class: "crash", Message: "the reactor returned an unreadable envelope"}
	}
	out.Value = nil
	res2, cerr := decodeEnvelope(status, raw)
	res2.Texts, res2.Logs = out.Texts, out.Logs
	res2.Steered = st.steer.Load()
	if cerr != nil {
		return res2, cerr
	}
	return res2, nil
}

// write copies b into guest memory through moca_alloc, so the guest owns the
// copy before it can observe it change.
func (in *instance) write(ctx context.Context, b []byte) (uint32, *CallError) {
	if len(b) == 0 {
		b = []byte{0} // a zero-length allocation is not a pointer the guest can use
	}
	ptr, cerr := in.alloc(ctx, uint64(len(b)))
	if cerr != nil {
		return 0, cerr
	}
	if !in.mem.Write(ptr, b) {
		return 0, &CallError{Class: "crash", Message: "script buffer is outside guest memory"}
	}
	return ptr, nil
}

func (in *instance) alloc(ctx context.Context, n uint64) (uint32, *CallError) {
	res, err := in.allocFn.Call(ctx, n)
	if err != nil || len(res) != 1 {
		return 0, &CallError{Class: "oom", Message: "moca_alloc failed"}
	}
	return uint32(res[0]), nil
}

func (in *instance) free(ctx context.Context, ptr uint32) {
	if in.freeFn != nil {
		_, _ = in.freeFn.Call(ctx, uint64(ptr))
	}
}

func readU32(in *instance, ptr uint32) uint32 {
	b, ok := in.mem.Read(ptr, 4)
	if !ok {
		return 0
	}
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// envelope is the reactor's epilogue: the script's value, the accepted events
// and a classified error, all as the guest wrote them.
type envelope struct {
	Value json.RawMessage `json:"value"`
	Texts []string        `json:"texts"`
	Logs  []string        `json:"logs"`
	Error *guestError     `json:"error"`
}

type guestError struct {
	Class   string `json:"class"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
}

// classOf maps a guest-thrown class onto a CallError class. It is the fixed
// allowlist R71 demands: only the class the PRELUDE raises for a capability
// refusal survives; every other thrown object — including one shaped like it —
// is `script`, so a script can never render its own "failed: steered"/"cancelled"
// and no guest string can reach class-keyed host behaviour.
func classOf(class string) string {
	switch class {
	case "network_disabled":
		return class
	case "limit":
		return "limit"
	default:
		return "script"
	}
}

// classFromStatus settles the class of a guest failure. The STATUS decides the
// classes only the host can know (syntax, oom, limit); the guest-named class is
// consulted last and always through the allowlist, so a script that throws
// {class:"syntax"} on a runtime error still reads as `script`.
func classFromStatus(status int32, guestNamed string) string {
	switch status {
	case 1:
		return "syntax"
	case 3:
		return "oom"
	case 4, 5:
		return "limit"
	}
	return classOf(guestNamed)
}

// decodeEnvelope turns qjs_run's status and buffer into the Go result. A status
// with no epilogue still returns a classified error.
func decodeEnvelope(status int32, raw []byte) (Result, *CallError) {
	var out Result
	if len(raw) > 0 {
		var env envelope
		if err := json.Unmarshal(raw, &env); err == nil {
			out.Value, out.Texts, out.Logs = env.Value, env.Texts, env.Logs
			if env.Error != nil {
				return out, &CallError{Class: classFromStatus(status, env.Error.Class), Message: env.Error.Message, Line: env.Error.Line}
			}
		}
	}
	if status == 0 {
		return out, nil
	}
	return out, &CallError{Class: statusClass(status), Message: "the script failed"}
}

// statusClass maps qjs_run's status codes. 4 (job limit) and 5 (output too
// large) are both `limit`; anything unknown is `crash`, never a silent success.
func statusClass(status int32) string {
	switch status {
	case 1:
		return "syntax"
	case 2:
		return "script"
	case 3:
		return "oom"
	case 4, 5:
		return "limit"
	default:
		return "crash"
	}
}

// registerHost installs the single import the reactor is allowed: moca.invoke.
// The dispatcher is taken from the CALL context, never from a process global,
// because the host module is registered once per runtime while executions
// differ.
func registerHost(ctx context.Context, rt wazero.Runtime) error {
	_, err := rt.NewHostModuleBuilder("moca").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, reqPtr, reqLen, resPtr, resCap uint32) int32 {
			return hostInvoke(ctx, reqPtr, reqLen, resPtr, resCap)
		}).
		Export("invoke").
		Instantiate(ctx)
	return err
}

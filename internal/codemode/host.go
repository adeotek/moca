package codemode

// The host half of the bridge: one import (`moca.invoke`), the frame decoder,
// and the bounded response encoder. The dispatcher and the accepted-event
// collector travel in the call context, never in a process global — the host
// module is registered once per runtime while executions differ (§Lifecycle).

import (
	"bytes"
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"
)

// hostState is one execution's bridge state.
type hostState struct {
	dispatch DispatchFunc
	steer    atomic.Bool
	texts    []string
	logs     []string
}

type hostKey struct{}

// withHost attaches the bridge state to the execution context.
func withHost(ctx context.Context, dispatch DispatchFunc) (context.Context, *hostState) {
	st := &hostState{dispatch: dispatch}
	return context.WithValue(ctx, hostKey{}, st), st
}

func hostOf(ctx context.Context) *hostState {
	st, _ := ctx.Value(hostKey{}).(*hostState)
	return st
}

// MarkSteered records that admission refused during this execution. The adapter
// calls it from inside Env.Admit; Result.Steered is read from here, never from
// guest text or a guest-named class (D3/R71).
func MarkSteered(ctx context.Context) {
	if st := hostOf(ctx); st != nil {
		st.steer.Store(true)
	}
}

// steered reports the host-set flag for this execution.
func steered(ctx context.Context) bool {
	if st := hostOf(ctx); st != nil {
		return st.steer.Load()
	}
	return false
}

// hostClass normalises a dispatcher-raised class. An unknown one becomes
// `script` rather than escaping into the result as noise.
func hostClass(class string) string {
	switch class {
	case "invalid_args", "invalid_cursor_args", "not_found", "not_regular",
		"outside", "denied", "changed", "limit", "cancelled", "refused",
		"script", "network_disabled":
		return class
	default:
		return "script"
	}
}

// frame is the guest→host message the prelude writes.
type frame struct {
	Op   string          `json:"op"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// reply is what the host sends back. Only one of Value/Error is meaningful.
type reply struct {
	Value json.RawMessage `json:"value,omitempty"`
	Error *CallError      `json:"error,omitempty"`
}

// hostInvoke is the `moca.invoke` import body. It returns the encoded reply's
// length, -1 when the reply does not fit the guest's buffer, or -2 on a host
// error — the three results the ABI documents.
func hostInvoke(ctx context.Context, reqPtr, reqLen, resPtr, resCap uint32) int32 {
	st := hostOf(ctx)
	mem := memoryOf(ctx)
	if st == nil || st.dispatch == nil || mem == nil {
		return -2
	}
	if reqLen == 0 || int(reqLen) > TheABI.Limits.RequestBytes ||
		int(resCap) < 1 || int(resCap) > TheABI.Limits.ResponseBytes {
		return -2
	}
	raw, ok := mem.Read(reqPtr, reqLen)
	if !ok {
		return -2
	}
	// The guest's buffer can change under us; copy before decoding.
	var f frame
	if err := json.Unmarshal(append([]byte(nil), raw...), &f); err != nil {
		return -2
	}

	var out reply
	switch f.Op {
	case "inspect":
		args := f.Args
		if len(args) == 0 || string(args) == "null" {
			args = json.RawMessage("{}")
		}
		value, cerr := st.dispatch(ctx, f.Name, args)
		if cerr != nil {
			// The dispatcher is HOST code: its class is trusted and only
			// normalised against the classes the adapter defines. R71's
			// allowlist governs what a GUEST may throw (classFromStatus), not
			// what the host reports — otherwise every typed inspection error
			// would decay to `script`.
			out.Error = &CallError{Class: hostClass(cerr.Class), Message: cerr.Message, Line: cerr.Line}
		} else {
			out.Value = value
		}
	case "text", "log":
		var s string
		if len(f.Args) > 0 && string(f.Args) != "null" {
			if err := json.Unmarshal(f.Args, &s); err != nil {
				return -2
			}
		}
		if f.Op == "log" {
			// ≤200 records × ≤1 KiB: past that the host stops recording rather
			// than grow without bound (§Failure ABI).
			if len(s) > 1024 {
				s = s[:1024]
			}
			if len(st.logs) < 200 {
				st.logs = append(st.logs, s)
			}
		} else if len(st.texts) < 200 {
			st.texts = append(st.texts, s)
		}
		out.Value = json.RawMessage("null")
	default:
		return -2
	}

	// Encode into a bounded buffer: no marshal-then-check on an unbounded slice.
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(out); err != nil {
		return -2
	}
	enc := bytes.TrimRight(b.Bytes(), "\n")
	if len(enc) > int(resCap) {
		return -1
	}
	if !mem.Write(resPtr, enc) {
		return -2
	}
	return int32(len(enc))
}

// memoryKey carries the guest's linear memory for the duration of one call, so
// the import body can reach it without a global.
type memoryKey struct{}

func withMemory(ctx context.Context, mem api.Memory) context.Context {
	return context.WithValue(ctx, memoryKey{}, mem)
}

func memoryOf(ctx context.Context) api.Memory {
	m, _ := ctx.Value(memoryKey{}).(api.Memory)
	return m
}

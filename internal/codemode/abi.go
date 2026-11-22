// Package codemode runs model-authored JavaScript in a QuickJS-NG reactor
// compiled to WASM and executed by wazero (SPECS §2). It is a leaf: stdlib and
// wazero only, never an internal/... import — the bridge to the agent lives in
// internal/tools (the adapter) and the agent (the dispatcher).
package codemode

// ABI pinned in abi.json and verified against the built reactor. The reactor
// is a shipped artifact, not a runtime download: the build recipe lives in
// reactor/BUILD.md and every fact it depends on is data here, so a rebuild that
// changes an export, an import or a limit fails a test instead of failing at
// runtime in front of a user.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"context"

	"github.com/tetratelabs/wazero"
)

//go:embed abi.json
var abiJSON []byte

// ABI is the parsed contract. Import names are `<module>.<name>`.
type ABI struct {
	Exports          []string          `json:"exports"`
	RequiredImports  []string          `json:"required_imports"`
	AllowedImports   []string          `json:"allowed_imports"`
	ForbiddenImports []string          `json:"forbidden_imports"`
	Limits           Limits            `json:"limits"`
	RunStatus        map[string]string `json:"run_status"`
}

// Limits are the byte budgets the guest is created with. The linear-memory
// page count is the wazero runtime knob; the rest are passed to qjs_init.
type Limits struct {
	LinearMemoryBytes int `json:"linear_memory_bytes"`
	LinearMemoryPages int `json:"linear_memory_pages"`
	JSHeapBytes       int `json:"js_heap_bytes"`
	LinearStackBytes  int `json:"linear_stack_bytes"`
	JSStackBytes      int `json:"js_stack_bytes"`
	RequestBytes      int `json:"request_bytes"`
	ResponseBytes     int `json:"response_bytes"`
}

// TheABI is the contract every build is measured against.
var TheABI = mustABI()

func mustABI() ABI {
	var a ABI
	if err := json.Unmarshal(abiJSON, &a); err != nil {
		panic("codemode: abi.json: " + err.Error())
	}
	if err := a.validate(); err != nil {
		panic("codemode: abi.json: " + err.Error())
	}
	return a
}

// validate rejects a contract that contradicts itself: an import both allowed
// and forbidden, an export set without the calls the engine makes, or limits
// where the declared page count cannot hold the declared linear memory.
func (a ABI) validate() error {
	need := []string{"memory", "qjs_init", "qjs_run", "qjs_destroy", "moca_alloc", "moca_free"}
	have := map[string]bool{}
	for _, e := range a.Exports {
		have[e] = true
	}
	for _, e := range need {
		if !have[e] {
			return fmt.Errorf("export %q is missing (the engine calls it)", e)
		}
	}
	allowed := map[string]bool{}
	for _, i := range a.AllowedImports {
		allowed[i] = true
	}
	for _, i := range a.ForbiddenImports {
		if allowed[i] {
			return fmt.Errorf("import %q is both allowed and forbidden", i)
		}
	}
	for _, i := range a.RequiredImports {
		if !allowed[i] && !strings.HasPrefix(i, "moca.") {
			return fmt.Errorf("required import %q is not in the allowlist", i)
		}
	}
	if got := a.Limits.LinearMemoryPages * 65536; got != a.Limits.LinearMemoryBytes {
		return fmt.Errorf("limits: %d pages is %d bytes, not the declared %d",
			a.Limits.LinearMemoryPages, got, a.Limits.LinearMemoryBytes)
	}
	if a.Limits.ResponseBytes <= a.Limits.RequestBytes {
		return fmt.Errorf("limits: response (%d) must exceed request (%d)", a.Limits.ResponseBytes, a.Limits.RequestBytes)
	}
	return nil
}

// VerifyModule checks a built reactor against the ABI: refusing a missing
// export, an import outside the allowlist, and any import the contract
// explicitly forbids. It decodes the module rather than instantiating it, so it
// costs one parse and needs no runtime.
//
// This is the gate Task 2 names: `path_*`, `fd_prestat_*`, `sock_*` and
// `poll_oneoff` must never appear, because the guest has no filesystem and no
// sockets — the bridge is the only authority.
func VerifyModule(wasm []byte) error {
	ctx := context.Background()
	cfg := wazero.NewRuntimeConfig().WithMemoryLimitPages(uint32(TheABI.Limits.LinearMemoryPages))
	rt := wazero.NewRuntimeWithConfig(ctx, cfg)
	defer rt.Close(ctx)

	compiled, err := rt.CompileModule(ctx, wasm)
	if err != nil {
		return fmt.Errorf("reactor: %w", err)
	}
	defer compiled.Close(ctx)

	exports := map[string]bool{}
	for _, f := range compiled.ExportedFunctions() {
		for _, name := range f.ExportNames() {
			exports[name] = true
		}
	}
	for name := range compiled.ExportedMemories() {
		exports[name] = true
	}
	var missing []string
	for _, want := range TheABI.Exports {
		if !exports[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("reactor is missing export(s): %s", strings.Join(missing, ", "))
	}

	imports, err := wasmImports(wasm)
	if err != nil {
		return fmt.Errorf("reactor: %w", err)
	}
	allowed := map[string]bool{}
	for _, i := range TheABI.AllowedImports {
		allowed[i] = true
	}
	for _, i := range TheABI.RequiredImports {
		allowed[i] = true
	}
	seen := map[string]bool{}
	for _, imp := range imports {
		name := imp.Module + "." + imp.Name
		seen[name] = true
		if !allowed[name] {
			return fmt.Errorf("reactor imports %q, which is not in abi.json's allowlist", name)
		}
	}
	for _, req := range TheABI.RequiredImports {
		if !seen[req] {
			return fmt.Errorf("reactor does not import %q — the guest could not reach the host", req)
		}
	}
	return nil
}

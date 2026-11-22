package codemode

import (
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"
)

// The deadline is clamped at task start, not trusted from the model: a
// non-positive value is the documented 10 s default, and anything outside the
// schema's 1–30 s band is pulled back into it (R25/R54).
func TestExecDeadlineClamps(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want time.Duration
	}{
		{0, DefaultTimeout},
		{-5 * time.Second, DefaultTimeout},
		{500 * time.Millisecond, MinTimeout},
		{1 * time.Second, MinTimeout},
		{7 * time.Second, 7 * time.Second},
		{30 * time.Second, MaxTimeout},
		{10 * time.Minute, MaxTimeout},
	}
	for _, tc := range cases {
		ctx, cancel := execDeadline(context.Background(), tc.in)
		deadline, ok := ctx.Deadline()
		cancel()
		if !ok {
			t.Fatalf("%v: no deadline", tc.in)
		}
		if got := time.Until(deadline); got > tc.want || got < tc.want-time.Second {
			t.Fatalf("%v: deadline ≈ %v, want %v", tc.in, got, tc.want)
		}
	}
}

// The prelude's SHAPE follows the stored variant and its ADMISSION follows the
// current switch (R69). All four states must render, and no placeholder may
// survive: a leftover __NET_LIVE__ would be a syntax error inside the guest.
func TestPreludeVariants(t *testing.T) {
	cases := []struct {
		caps     ExecCaps
		wantWeb  bool
		wantLive bool
	}{
		{ExecCaps{false, false}, false, false},
		{ExecCaps{false, true}, false, false}, // impossible per R69, but must still render safely
		{ExecCaps{true, false}, true, false},  // resumed network session, switch now off
		{ExecCaps{true, true}, true, true},
	}
	for _, tc := range cases {
		src := PreludeSource(tc.caps)
		for _, leftover := range []string{"__NET_SHAPE__", "__NET_LIVE__", "__REQ_MAX__", "__RES_MAX__"} {
			if strings.Contains(src, leftover) {
				t.Fatalf("%+v: placeholder %s survived", tc.caps, leftover)
			}
		}
		hasWeb := strings.Contains(src, `defineProperty(tools, "web"`)
		if hasWeb != tc.wantWeb {
			t.Fatalf("%+v: web block = %v, want %v", tc.caps, hasWeb, tc.wantWeb)
		}
		if tc.wantLive && !strings.Contains(src, "var NET_LIVE = true") {
			t.Fatalf("%+v: the admission flag is not true", tc.caps)
		}
		if !tc.wantLive && !strings.Contains(src, "var NET_LIVE = false") {
			t.Fatalf("%+v: the admission flag is not false", tc.caps)
		}
	}
	// A workspace-only guest must not be able to PRODUCE the refusal: the
	// throwing code is cut from its source, so the class cannot exist there
	// even if the model guessed the name. (The file's header comments still
	// mention it — that text never reaches the guest.)
	if strings.Contains(PreludeSource(ExecCaps{}), `throw { class: "network_disabled"`) {
		t.Fatal("a workspace-only prelude can still throw network_disabled")
	}
	if !strings.Contains(PreludeSource(ExecCaps{true, false}), `throw { class: "network_disabled"`) {
		t.Fatal("a network-shaped prelude must carry the refusal path")
	}
}

// R71: the class a guest names never crosses the boundary wholesale. Only the
// prelude's capability refusal survives; everything else is `script`, so a
// script cannot forge `steered`, `cancelled` or any class-keyed behaviour.
func TestClassAllowlist(t *testing.T) {
	for _, forged := range []string{"steered", "cancelled", "timeout", "busy", "oom", "crash", "refused", "syntax", ""} {
		if got := classOf(forged); got != "script" {
			t.Fatalf("class %q mapped to %q, want script", forged, got)
		}
	}
	if got := classOf("network_disabled"); got != "network_disabled" {
		t.Fatalf("network_disabled mapped to %q", got)
	}
	if got := classOf("limit"); got != "limit" {
		t.Fatalf("limit mapped to %q", got)
	}
}

func TestStatusClass(t *testing.T) {
	want := map[int32]string{1: "syntax", 2: "script", 3: "oom", 4: "limit", 5: "limit", 6: "crash", 99: "crash"}
	for status, class := range want {
		if got := statusClass(status); got != class {
			t.Fatalf("status %d = %q, want %q", status, got, class)
		}
	}
}

// The envelope is the reactor's whole answer: value, events and a classified
// error. A status with no epilogue still yields a class, and an unknown class
// falls back to script.
func TestDecodeEnvelope(t *testing.T) {
	out, cerr := decodeEnvelope(0, []byte(`{"value":{"complete":true},"texts":["a"],"logs":["b"]}`))
	if cerr != nil || string(out.Value) != `{"complete":true}` || len(out.Texts) != 1 || len(out.Logs) != 1 {
		t.Fatalf("ok envelope = %+v / %v", out, cerr)
	}
	out, cerr = decodeEnvelope(2, []byte(`{"error":{"class":"network_disabled","message":"off","line":3}}`))
	if cerr == nil || cerr.Class != "network_disabled" || cerr.Line != 3 {
		t.Fatalf("refusal envelope = %v", cerr)
	}
	if !strings.Contains(cerr.Error(), "line 3") {
		t.Fatalf("the line must be in the message: %q", cerr.Error())
	}
	out, cerr = decodeEnvelope(2, []byte(`{"error":{"class":"steered","message":"forged"}}`))
	if cerr == nil || cerr.Class != "script" {
		t.Fatalf("a forged class must read as script, got %v", cerr)
	}
	if _, cerr = decodeEnvelope(1, nil); cerr == nil || cerr.Class != "syntax" {
		t.Fatalf("status without an epilogue = %v", cerr)
	}
	if _, cerr = decodeEnvelope(0, nil); cerr != nil {
		t.Fatalf("an empty ok epilogue is not an error: %v", cerr)
	}
	_ = out
}

// Without the artifact the engine refuses loudly and leaves the slot free: a
// second call must not be told `busy` because the first one failed.
func TestExecuteWithoutReactor(t *testing.T) {
	saved := reactorBytes
	reactorBytes = nil
	defer func() { reactorBytes = saved }()

	e := &Engine{}
	dispatch := func(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, *CallError) {
		return json.RawMessage(`null`), nil
	}
	_, cerr := e.Execute(context.Background(), ExecCaps{}, "1+1", dispatch, time.Second)
	if cerr == nil || !errors.Is(ErrNoReactor, ErrNoReactor) || cerr.Class != "crash" {
		t.Fatalf("want a crash naming the missing reactor, got %v", cerr)
	}
	if !strings.Contains(cerr.Message, "reactor") {
		t.Fatalf("the message must name the reactor: %q", cerr.Message)
	}
	if _, cerr = e.Execute(context.Background(), ExecCaps{}, "1+1", dispatch, time.Second); cerr.Class == "busy" {
		t.Fatal("a failed execution left the slot busy")
	}
}

// VerifyModule is the artifact gate: garbage is refused, and the contract it
// checks is the one in abi.json.
func TestVerifyModuleRejectsGarbage(t *testing.T) {
	if err := VerifyModule([]byte("not wasm")); err == nil {
		t.Fatal("garbage was accepted as a reactor")
	}
	if err := VerifyModule(nil); err == nil {
		t.Fatal("an empty module was accepted")
	}
}

// The contract is data, and a contradiction in it must fail here rather than
// in a user's session: the six exports, the allowlist split and the limits.
func TestABIContract(t *testing.T) {
	if err := TheABI.validate(); err != nil {
		t.Fatalf("abi.json is self-contradictory: %v", err)
	}
	want := map[string]bool{"memory": true, "qjs_init": true, "qjs_run": true, "qjs_destroy": true, "moca_alloc": true, "moca_free": true}
	for _, e := range TheABI.Exports {
		if !want[e] {
			t.Fatalf("abi.json exports %q, which the engine does not know", e)
		}
		delete(want, e)
	}
	if len(want) != 0 {
		t.Fatalf("abi.json is missing exports: %v", want)
	}
	if TheABI.Limits.LinearMemoryPages*65536 != 32<<20 {
		t.Fatalf("linear memory = %d pages, want 512 (32 MiB)", TheABI.Limits.LinearMemoryPages)
	}
	if TheABI.Limits.JSHeapBytes != 12<<20 || TheABI.Limits.JSStackBytes != 1<<20 {
		t.Fatalf("guest limits = %d/%d, want 12 MiB heap / 1 MiB stack", TheABI.Limits.JSHeapBytes, TheABI.Limits.JSStackBytes)
	}
	// The forbidden list is the security property: the guest has no filesystem
	// and no sockets, so these imports must never be allowed.
	allowed := map[string]bool{}
	for _, i := range TheABI.AllowedImports {
		allowed[i] = true
	}
	for _, bad := range TheABI.ForbiddenImports {
		if allowed[bad] {
			t.Fatalf("forbidden import %q is allowlisted", bad)
		}
	}
	for _, prefix := range []string{"path_", "sock_", "poll_oneoff", "fd_prestat_"} {
		found := false
		for _, f := range TheABI.ForbiddenImports {
			if strings.Contains(f, prefix) {
				found = true
			}
		}
		if !found {
			t.Fatalf("abi.json does not forbid the %s* imports", prefix)
		}
	}
}

// The reactor is a build artifact, not a download: until reactor/BUILD.md has
// been run there is nothing to embed, and this test says so instead of passing
// silently. Once the file exists, it must be embedded — the shipped binary
// carries the interpreter.
func TestReactorIsEmbedded(t *testing.T) {
	const path = "reactor/qjs-moca-reactor.wasm"
	_, err := os.Stat(path)
	if err != nil {
		t.Skipf("no reactor artifact yet (%s): build it per reactor/BUILD.md", path)
	}
	if len(reactorBytes) == 0 {
		t.Fatalf("%s exists but is not embedded — add the //go:embed line in engine.go", path)
	}
	if err := VerifyModule(reactorBytes); err != nil {
		t.Fatalf("the embedded reactor violates abi.json: %v", err)
	}
}

// The leaf rule (§Bridge): this package is stdlib + wazero only. A dependency
// on any internal/... package would make the reactor's contract depend on the
// agent and destroy the layering Task 4 relies on — including through a test.
func TestPackageIsALeaf(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "github.com/adeotek/moca/internal/"
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, "\"")
			if strings.HasPrefix(path, prefix) {
				t.Fatalf("%s imports %s — internal/codemode must stay a leaf", name, path)
			}
		}
	}
}

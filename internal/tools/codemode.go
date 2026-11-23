package tools

// The codemode adapter: the ninth tool. It owns the bridge (the capability map
// the guest may reach, the inspection root, the accepted text/log capture and
// the ledger), and nothing else — the engine, the reactor and the containment
// live in their own packages.
//
// Two factories, one schema:
//   - NewCodemode(holder) runs scripts.
//   - NewCodemodeRefusal() is the same tool with no engine and no root: it
//     refuses every call. A session that is not allowed to use codemode still
//     gets the exact spec, so the tool list does not shift under the model.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/codemode"
	"github.com/adeotek/moca/internal/llm"
)

const (
	// codemodeResultMax bounds the whole outer result, in runes.
	codemodeResultMax = 2000
	// codemodeNoticeReserve is kept free for the status line and the ledger,
	// so a truncated value never eats the reason.
	codemodeNoticeReserve = 512
	codemodeMaxSecond     = 30
)

// CodemodeEngine is the slice of the engine the adapter uses. It is an
// interface so the adapter can be tested without a reactor, and so a session
// can hold a refusal-only tool with a nil engine.
type CodemodeEngine interface {
	NewPrelude(caps codemode.ExecCaps) string
	Execute(ctx context.Context, caps codemode.ExecCaps, src string, dispatch codemode.DispatchFunc, timeout time.Duration) (codemode.Result, *codemode.CallError)
}

// CodemodeHolder is the {engine, caps} pair for one effective policy. It is
// built once from the session header, so the SAME ExecCaps value reaches
// NewPrelude and Execute — the prelude the model was promised and the
// capabilities it actually gets cannot drift (R47/R38).
type CodemodeHolder struct {
	// Open constructs the engine on first use. It is lazy because creating the
	// handle compiles no module and opens no root, and because a session whose
	// artifact is missing must fail its first call, not its start.
	Open func() (CodemodeEngine, error)
	Caps codemode.ExecCaps

	engine  CodemodeEngine
	openErr error
	prelude string
	built   bool
}

// Engine resolves the engine once, memoizing both the success and the error.
func (h *CodemodeHolder) Engine() (CodemodeEngine, error) {
	if h.engine == nil && h.openErr == nil {
		if h.Open == nil {
			h.openErr = errors.New("codemode: no engine")
		} else {
			h.engine, h.openErr = h.Open()
		}
	}
	return h.engine, h.openErr
}

// Prelude renders the prelude for this policy exactly once. It is the only
// caller of NewPrelude, so the prelude the model was shown and the capabilities
// Execute receives come from one immutable ExecCaps value (R38).
func (h *CodemodeHolder) Prelude() string {
	if !h.built {
		h.built = true
		if eng, err := h.Engine(); err == nil {
			h.prelude = eng.NewPrelude(h.Caps)
		}
	}
	return h.prelude
}

type codemodeTool struct {
	holder *CodemodeHolder
	// variant is the stored prelude variant for a refusal-only tool, which has
	// no holder to read it from: a resumed session whose frozen prompt promises
	// codemode still sees the spec that matches that prompt.
	variant bool
}

// NewCodemode returns a codemode tool that runs scripts with holder's engine and
// capability set.
func NewCodemode(holder *CodemodeHolder) Tool { return codemodeTool{holder: holder} }

// NewCodemodeRefusal returns the same tool with no engine: every call is
// refused, nothing is opened, and the spec matches the stored prompt variant.
func NewCodemodeRefusal(networkPrelude bool) Tool {
	return codemodeTool{variant: networkPrelude}
}

// capability is one entry of the guest's table. The table is the whole
// security boundary on the workspace side: a name that is not in it cannot be
// dispatched, whatever the script passes.
type capability struct {
	name    string
	network bool
	call    func(ctx context.Context, b *bridge, args json.RawMessage) (json.RawMessage, *codemode.CallError)
}

// bridge is one execution's workspace view plus its capture and ledger.
type bridge struct {
	in      *Inspection
	caps    map[string]capability
	texts   []string
	logs    []string
	calls   int
	gaps    int
	reasons []string
}

// idiomEchoLead opens the result-side idiom reminder. It is matched, never
// repeated: a result that already carries it is not appended twice.
const idiomEchoLead = "hint:"

// codemodeSpec is the tool's whole spec in the F17 form: name, description and
// schema together, because a description edit resets the prompt cache for every
// enabled session. Both specs are frozen and measured (541 B workspace, 627 B
// network against the 576/640 budgets); the schema is identical in both
// variants, so only the description moves. The workspace budget moved from 512
// to 576 when the API gained `glob`: the cap exists to bound what every request
// carries, and a fourth call's shape is worth its bytes (SPECS §15).
//
// The workspace description names each call's result shape. Stage A's pilot
// measured the cost of leaving it implicit: the one trial that scripted the
// manifests carried the cheapest history of its six, and the trial that had to
// guess `lines`/`entries` spent seven script attempts discovering them before
// failing. A shape is part of the API, so it belongs where the model reads the
// API -- three calls, three shapes, still under the byte budget.
//
// The network variant keeps its call list without shapes on purpose: its web
// payloads are v1.1's contract (Task 9) and documenting a shape that does not
// exist yet would be worse than naming the call.
func codemodeSpec(network bool) llm.ToolSpec {
	desc := "Read-only workspace JS. Await tools.read({path}|{next})\u2192{lines:[{n,text}]}, " +
		"tools.search({pattern})\u2192{matches:[{path,line,text}]}, " +
		"tools.ls({path})\u2192{entries:[{name,kind}]}, " +
		"tools.glob({pattern})\u2192{paths}. " +
		"Return the answer; a bare expression is null."
	if network {
		// Same shapes, plus the two web calls. Their payloads are v1.1's
		// contract, so the call is named and the shape is not.
		desc = "Read-only workspace JS. Await tools.read({path}|{next})\u2192{lines:[{n,text}]}, " +
			"tools.search({pattern})\u2192{matches:[{path,line,text}]}, " +
			"tools.ls({path})\u2192{entries:[{name,kind}]}, " +
			"tools.glob({pattern})\u2192{paths}; with network enabled also await " +
			"tools.web.fetch({url}) and tools.web.search({query}). " +
			"Return the answer; a bare expression is null."
	}
	return llm.ToolSpec{
		Name:        "codemode",
		Description: desc,
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"code":{"type":"string"},` +
			`"timeoutSeconds":{"type":"integer","minimum":1,"maximum":30,` +
			`"description":"Guest deadline; default 10s. Host I/O may finish later."}},` +
			`"required":["code"],"additionalProperties":false}`),
	}
}

func (t codemodeTool) Spec() llm.ToolSpec {
	v := t.variant
	if t.holder != nil {
		v = t.holder.Caps.NetworkPrelude
	}
	return codemodeSpec(v)
}

// Run runs one script. Admission is checked before the call and again after it
// returns: a run that was admitted may still lose its annotation while the
// script is executing, and the agent needs to know that before it summarises.
func (t codemodeTool) Run(ctx context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Code           string `json:"code"`
		TimeoutSeconds *int   `json:"timeoutSeconds"`
	}
	if r := decodeArgs(input, &a); r != nil {
		return Result{Content: "codemode: " + r.Message, IsError: true, Summary: "codemode (invalid)"}
	}
	admit := env.Admit
	if admit == nil {
		admit = func() bool { return true }
	}
	var eng CodemodeEngine
	if t.holder != nil {
		eng, _ = t.holder.Engine()
	}
	if eng == nil {
		return Result{
			Content: "codemode is not enabled in this session; use read, search and ls directly.",
			IsError: true,
			Summary: "codemode (refused)",
		}
	}
	if !admit() {
		return Result{Content: "steering pending: codemode was not run", Summary: "codemode (steered)", Steered: true}
	}
	// One script per user turn: a second call in the same turn is refused
	// without running anything. This is the mechanism, not a nudge -- the pilot
	// showed a hint arriving after the second script cannot prevent it, so the
	// budget has to be enforced before the script, and the message has to say
	// what to do instead.

	// The bridge is per call: its root, budgets and cursors are the script's,
	// and a second call starts clean.
	br := &bridge{in: mustInspection(env.Root), caps: workspaceCapabilities()}
	defer br.in.Close()

	res, cerr := eng.Execute(ctx, t.holder.Caps, a.Code, br.dispatch, timeoutOf(a.TimeoutSeconds))
	steered := !admit() || res.Steered

	content, isErr := br.render(res, cerr)
	return Result{Content: content, IsError: isErr, Summary: br.summary(cerr), Steered: steered}
}

// mustInspection builds the execution's inspection. It defers the failure to
// the first dispatch: a script that never inspects anything (1+1) must not fail
// because the root could not be opened.
func mustInspection(root string) *Inspection {
	in, err := NewInspection(root)
	if err != nil {
		// NewInspection only rejects an empty root; dispatch reports it then.
		return &Inspection{dir: "", cursors: map[string]cursor{}}
	}
	return in
}

// workspaceCapabilities is v1's table: the four workspace primitives, no
// network row. A caller that is not admitted network never gets a table with
// one, so there is nothing to filter at dispatch time.
func workspaceCapabilities() map[string]capability {
	return map[string]capability{
		"read":   {name: "read", network: false, call: callRead},
		"search": {name: "search", network: false, call: callSearch},
		"ls":     {name: "ls", network: false, call: callLS},
		"glob":   {name: "glob", network: false, call: callGlob},
	}
}

func callRead(ctx context.Context, b *bridge, args json.RawMessage) (json.RawMessage, *codemode.CallError) {
	v, err := b.in.Read(ctx, args)
	return encodeResult(b, v, err)
}

func callSearch(ctx context.Context, b *bridge, args json.RawMessage) (json.RawMessage, *codemode.CallError) {
	v, err := b.in.Search(ctx, args)
	return encodeResult(b, v, err)
}

func callLS(ctx context.Context, b *bridge, args json.RawMessage) (json.RawMessage, *codemode.CallError) {
	v, err := b.in.LS(ctx, args)
	return encodeResult(b, v, err)
}

func callGlob(ctx context.Context, b *bridge, args json.RawMessage) (json.RawMessage, *codemode.CallError) {
	v, err := b.in.Glob(ctx, args)
	return encodeResult(b, v, err)
}

// dispatch answers one guest frame. An unknown name is a host-side refusal, not
// a guest-side throw: the script asked for something that does not exist in
// this session, and the model needs to see that.
func (b *bridge) dispatch(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, *codemode.CallError) {
	cap, ok := b.caps[name]
	if !ok {
		return nil, &codemode.CallError{Class: "refused", Message: "no such tool in this session: " + name}
	}
	b.calls++
	out, cerr := cap.call(ctx, b, args)
	if cerr != nil {
		return nil, cerr
	}
	return out, nil
}

// encodeResult marshals a payload, notes its coverage for the ledger and turns
// a typed inspection failure into the class the engine reports.
func encodeResult(b *bridge, v any, err error) (json.RawMessage, *codemode.CallError) {
	if err != nil {
		return nil, callErrorOf(err)
	}
	b.noteGaps(v)
	raw, jerr := json.Marshal(v)
	if jerr != nil {
		return nil, &codemode.CallError{Class: "crash", Message: "payload is not encodable: " + jerr.Error()}
	}
	return raw, nil
}

func callErrorOf(err error) *codemode.CallError {
	var ie *InspectError
	if errors.As(err, &ie) {
		return &codemode.CallError{Class: ie.Class, Message: ie.Message}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &codemode.CallError{Class: "cancelled", Message: "the script was cancelled"}
	}
	return &codemode.CallError{Class: "script", Message: err.Error()}
}

func timeoutOf(seconds *int) time.Duration {
	if seconds == nil {
		return 0 // the engine applies its own default and clamps
	}
	s := *seconds
	if s < 1 {
		s = 1
	}
	if s > codemodeMaxSecond {
		s = codemodeMaxSecond
	}
	return time.Duration(s) * time.Second
}

// render builds the outer result: the accepted text, the final value and, when
// the call failed or coverage has gaps, the ledger. Accepted output is kept
// even when the script failed — a timeout after text() loses only the epilogue.
func (b *bridge) render(res codemode.Result, cerr *codemode.CallError) (string, bool) {
	var sb strings.Builder
	for _, t := range res.Texts {
		sb.WriteString(t)
		sb.WriteString("\n")
	}
	body := strings.TrimRight(sb.String(), "\n")
	if cerr != nil {
		head := "failed: " + cerr.Class
		if cerr.Message != "" {
			head += ": " + cerr.Message
		}
		if body != "" {
			body = head + "\n" + body
		} else {
			body = head
		}
	} else if v := strings.TrimSpace(string(res.Value)); v != "" && v != "null" {
		if body != "" {
			body += "\n"
		}
		body += v
	}

	ledger := b.ledger(cerr != nil)
	if ledger != "" {
		body = strings.TrimRight(body, "\n")
		if body != "" {
			body += "\n"
		}
		body += ledger
	}
	if body == "" {
		body = "(no output)"
	}
	return clampRunes(body, codemodeResultMax), cerr != nil
}

// ledger appears iff the call failed or coverage has gaps: a clean run that read
// everything says nothing about budgets it did not touch.
func (b *bridge) ledger(failed bool) string {
	if !failed && b.gaps == 0 {
		return ""
	}
	line := fmt.Sprintf("[codemode: %d calls", b.calls)
	if len(b.reasons) > 0 {
		line += "; gaps: " + strings.Join(b.reasons, ", ")
	}
	line += "]"
	return clampRunes(line, 200)
}

func (b *bridge) summary(cerr *codemode.CallError) string {
	if cerr != nil {
		return "codemode failed: " + cerr.Class
	}
	return fmt.Sprintf("codemode %d calls", b.calls)
}

// clampRunes truncates on a rune boundary and says so, so a truncated result is
// never mistaken for a complete one.
func clampRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…[truncated]"
}

// resetCoverage folds one call's coverage into the execution ledger.
func (b *bridge) noteGaps(payload any) {
	// Payloads carry their own reasons; the bridge keeps the execution-wide
	// picture for the ledger line.
	type coverage struct {
		Complete bool     `json:"complete"`
		Reasons  []string `json:"reasons"`
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var c coverage
	if json.Unmarshal(raw, &c) != nil || c.Complete {
		return
	}
	b.gaps += len(c.Reasons)
	for _, r := range c.Reasons {
		if len(b.reasons) < 5 {
			b.reasons = append(b.reasons, r)
		}
	}
}

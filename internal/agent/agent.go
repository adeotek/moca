package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

const wrapUpText = "You have reached the step limit for this run. Do not call tools. " +
	"Summarize what you did, what remains, and how to continue."

type Options struct {
	Config    config.Config
	Providers *provider.Registry
	Tools     *tools.Registry
	Env       *tools.Env
	Session   *session.Writer
	Snapshots *session.Snapshots
	System    string
	Model     string
	Effort    llm.Effort
	Emit      func(Event)
	Prior     []session.Entry
}

type Outcome struct {
	Text     string
	MaxSteps bool
}

type Agent struct {
	opts    Options
	model   provider.Model
	adapter provider.Adapter
	effort  llm.Effort
	usage   llm.Usage
	cost    float64
	yolo    bool
	hard    *savedModel
	// mu guards the fields the TUI reads while a run is in progress from
	// another goroutine: the steering queue, the transcript mirror, the
	// usage/cost totals and the estimate anchor (Status/ContextTokens/
	// TakeSteering). Writers: append, turn, Steer, Compact. Everything else
	// is called between runs only.
	mu            sync.Mutex
	entries       []session.Entry // full transcript (in-memory mirror)
	steer         []string
	anchorTokens  int
	anchorEntries int
	anchorValid   bool
	// lastOverflow is the error behind the most recent errOverflow turn
	// (provider overflow, or a length stop with the context full); the
	// compact-and-retry surfaces it when recovery is impossible.
	lastOverflow error
	strict       struct {
		paths tools.PathChecker
		cmds  tools.CommandChecker
		ask   tools.Asker
	}
}

func New(o Options) (*Agent, error) {
	a := &Agent{opts: o, entries: o.Prior}
	a.strict.paths, a.strict.cmds, a.strict.ask = o.Env.Paths, o.Env.Commands, o.Env.Ask
	if err := a.setModel(o.Model, o.Effort); err != nil {
		return nil, err
	}
	for _, e := range o.Prior {
		if e.Usage != nil {
			a.usage = a.usage.Add(*e.Usage)
			a.cost += e.Cost
		}
	}
	return a, nil
}

func (a *Agent) setModel(q string, e llm.Effort) error {
	m, ad, err := a.opts.Providers.Resolve(q)
	if err != nil {
		return err
	}
	if e == "" {
		e = m.DefaultEffort()
	}
	a.model, a.adapter, a.effort = m, ad, m.ClampEffort(e)
	return nil
}

func (a *Agent) Model() provider.Model        { return a.model }
func (a *Agent) Effort() llm.Effort           { return a.effort }
func (a *Agent) Totals() (llm.Usage, float64) { return a.usage, a.cost }
func (a *Agent) Session() *session.Writer     { return a.opts.Session }

// Workdir is the session's jail root — the workdir the session was started
// in; a resumed session keeps its original one.
func (a *Agent) Workdir() string { return a.opts.Env.Root }
func (a *Agent) emit(e Event) {
	if a.opts.Emit != nil {
		a.opts.Emit(e)
	}
}

func (a *Agent) append(e session.Entry) (session.Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, err := a.opts.Session.Append(e)
	if err == nil {
		a.entries = append(a.entries, e)
	}
	return e, err
}

func userText(s string) *llm.Message {
	return &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: s}}}
}

func (a *Agent) request(choice llm.ToolChoice) llm.Request {
	// Snapshot the transcript under the lock: the TUI may append a `!` note
	// from another goroutine while a run is in progress.
	a.mu.Lock()
	msgs := session.Messages(a.entries)
	a.mu.Unlock()
	return llm.Request{
		Model:      a.model.ID,
		System:     a.opts.System,
		Messages:   TransformHistory(msgs, a.model.Qualified()),
		Tools:      a.opts.Tools.Specs(),
		ToolChoice: choice,
		MaxTokens:  a.model.MaxTokens(a.opts.Config.Context.ReserveTokens),
		Effort:     a.effort,
	}
}

// turn streams one model response and persists it. It returns the response
// and the persisted tool calls. A context overflow (or a length stop with the
// input within reserve of the window) becomes errOverflow and nothing is
// persisted: Run compacts once and repeats the turn (§6).
func (a *Agent) turn(ctx context.Context, choice llm.ToolChoice) (llm.Response, []llm.ToolCall, error) {
	resp, err := a.adapter.Stream(ctx, a.request(choice), func(e llm.Event) {
		switch e.Type {
		case llm.EventText:
			a.emit(TextDelta{e.Text})
		case llm.EventThinking:
			a.emit(ThinkingDelta{e.Text})
		case llm.EventReset:
			a.emit(StreamReset{})
		}
	})
	if errors.Is(err, provider.ErrContextOverflow) ||
		(err == nil && resp.Stop == llm.StopLength &&
			resp.Usage.Input+resp.Usage.CacheRead+resp.Usage.CacheWrite >= a.model.ContextWindow-a.Budget().Reserve) {
		a.lastOverflow = err
		if a.lastOverflow == nil {
			a.lastOverflow = fmt.Errorf("response hit the length limit with the context full (%d input tokens)",
				resp.Usage.Input+resp.Usage.CacheRead+resp.Usage.CacheWrite)
		}
		a.emit(StreamReset{})
		return resp, nil, errOverflow
	}
	if err != nil {
		a.append(session.Entry{Type: session.TypeError, Error: &session.ErrorInfo{Message: err.Error()}})
		return resp, nil, err
	}
	var body []llm.ContentBlock
	var calls []llm.ToolCall
	for _, c := range resp.Message.Content {
		if c.Type == llm.BlockToolUse {
			calls = append(calls, *c.ToolCall)
		} else {
			if c.Type == llm.BlockThinking {
				// Adapters stamp the bare id; the agent owns qualification (§3).
				c.Model = a.model.Qualified()
			}
			body = append(body, c)
		}
	}
	cost := a.model.CostOf(resp.Usage)
	a.mu.Lock()
	a.usage, a.cost = a.usage.Add(resp.Usage), a.cost+cost
	a.mu.Unlock()
	u := resp.Usage
	msgEntry, err := a.append(session.Entry{Type: session.TypeMessage, Model: a.model.Qualified(), Usage: &u, Cost: cost,
		Message: &llm.Message{Role: llm.RoleAssistant, Content: body}})
	if err != nil {
		return resp, nil, err
	}
	for _, c := range calls {
		pc := c
		if !json.Valid(pc.Input) {
			// A call cut off mid-JSON cannot be persisted as-is (json.Marshal
			// fails on the invalid RawMessage); store an empty object — Run
			// still gives it the "cut off / invalid JSON" error result
			// without executing it (§6).
			pc.Input = json.RawMessage("{}")
		}
		if _, err := a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: msgEntry.ID, Call: pc}}); err != nil {
			return resp, nil, err
		}
	}
	// Anchor the §6 estimate after everything this turn appended: UsageTokens
	// covers the request plus this response (text and calls).
	// A provider that reports no usage (an endpoint ignoring include_usage)
	// cannot anchor anything: leave the chars/4 fallback in charge.
	a.mu.Lock()
	n := compact.UsageTokens(resp.Usage)
	a.anchorTokens, a.anchorEntries, a.anchorValid = n, len(a.entries), n > 0
	a.mu.Unlock()
	a.emit(TurnEnd{Message: resp.Message, Usage: resp.Usage, Cost: cost, Stop: resp.Stop})
	return resp, calls, nil
}

func (a *Agent) result(call llm.ToolCall, r tools.Result) error {
	_, err := a.append(session.Entry{Type: session.TypeToolResult,
		ToolResult: &llm.ToolResult{CallID: call.ID, Content: r.Content, IsError: r.IsError}})
	a.emit(ToolEnd{Call: call, Result: r})
	return err
}

// abort appends synthetic results for every unanswered call and reports them
// like real tool ends.
func (a *Agent) abort(reason string) {
	a.mu.Lock()
	entries := append([]session.Entry(nil), a.entries...)
	a.mu.Unlock()
	for _, fix := range session.Repair(entries, reason) {
		if _, err := a.append(fix); err != nil || fix.ToolResult == nil {
			continue
		}
		call := llm.ToolCall{ID: fix.ToolResult.CallID}
		for _, e := range entries {
			if e.Type == session.TypeToolUse && e.ToolUse != nil && e.ToolUse.Call.ID == call.ID {
				call = e.ToolUse.Call
				break
			}
		}
		a.emit(ToolEnd{Call: call, Result: tools.Result{Content: fix.ToolResult.Content, IsError: fix.ToolResult.IsError}})
	}
}

// turnWithRecovery runs one turn; on overflow it compacts once and retries.
// A failed compaction, or a second overflow, surfaces the original error.
func (a *Agent) turnWithRecovery(ctx context.Context, choice llm.ToolChoice) (llm.Response, []llm.ToolCall, error) {
	resp, calls, err := a.turn(ctx, choice)
	if !errors.Is(err, errOverflow) {
		return resp, calls, err
	}
	orig := a.lastOverflow
	if cerr := a.Compact(ctx); cerr != nil {
		return resp, nil, orig
	}
	resp, calls, err = a.turn(ctx, choice)
	if errors.Is(err, errOverflow) {
		return resp, nil, orig
	}
	return resp, calls, err
}

func (a *Agent) Run(ctx context.Context, prompt string) (Outcome, error) {
	if err := a.maybeCompact(ctx); err != nil {
		return Outcome{}, err
	}
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(prompt)}); err != nil {
		return Outcome{}, err
	}
	for step := 0; ; step++ {
		if step == a.opts.Config.Context.MaxSteps {
			a.append(session.Entry{Type: session.TypeMessage, Message: userText(wrapUpText)})
			resp, calls, err := a.turnWithRecovery(ctx, llm.ToolChoiceNone)
			if err != nil {
				return Outcome{}, err
			}
			if len(calls) > 0 {
				// The model ignored tool_choice: none; every call still gets
				// exactly one result (§10).
				a.abort("the wrap-up turn must not call tools; this call was not executed")
			}
			return Outcome{Text: llm.TextOf(resp.Message), MaxSteps: true}, nil
		}
		resp, calls, err := a.turnWithRecovery(ctx, llm.ToolChoiceAuto)
		if err != nil {
			return Outcome{}, err
		}
		if len(calls) == 0 {
			queued, err := a.applySteering()
			if err != nil {
				return Outcome{}, err
			}
			if queued {
				continue
			}
			return Outcome{Text: llm.TextOf(resp.Message)}, nil
		}
		for _, call := range calls {
			if ctx.Err() != nil {
				a.abort(session.AbortedByUser)
				return Outcome{}, ctx.Err()
			}
			a.emit(ToolStart{Call: call})
			var r tools.Result
			if resp.Stop == llm.StopLength {
				r = tools.Result{IsError: true, Content: fmt.Sprintf("your output was cut off at the token limit, so this %s call "+
					"was not executed. Split the work into smaller calls (e.g. write a short file, then extend it with edit).", call.Name)}
			} else {
				r = a.opts.Tools.Run(ctx, a.opts.Env, call)
			}
			if err := a.result(call, r); err != nil {
				return Outcome{}, err
			}
		}
		if ctx.Err() != nil {
			a.abort(session.AbortedByUser)
			return Outcome{}, ctx.Err()
		}
		// Steering lands after the complete tool batch (never between a call
		// and its result — no provider accepts that, §11).
		if _, err := a.applySteering(); err != nil {
			return Outcome{}, err
		}
		if err := a.maybeCompact(ctx); err != nil {
			return Outcome{}, err
		}
	}
}

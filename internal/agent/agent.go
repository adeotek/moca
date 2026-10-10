package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/mcp"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

const wrapUpText = "You have reached the step limit for this run. Do not call tools. " +
	"Summarize what you did, what remains, and how to continue."

// emptyNudge answers a turn with neither text nor tool calls.
const emptyNudge = "Your last reply was empty. Continue the task where you left off, or give your final answer."

// stuckText is the wrap-up of a run stopped for repeating a failing call.
const stuckText = "This run is stopped: the same tool call failed with the same result several times. Do not call tools. " +
	"Explain to the user what you were trying to do, what is blocking you, and what they could do about it."

// verifyNudge is persisted and answered when a run that changed code is
// about to end without a build/test since (§14): one bounded nudge.
func verifyNudge(file string) string {
	return "You changed " + file + " but have not built or tested since the last change. " +
		"Run the project's checks now (build, tests, or running the program), or state why verification is not possible, then give your final answer."
}

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
	// MCP is the lazy MCP-server manager when servers are configured
	// (start.go); nil otherwise.
	MCP *mcp.Manager
}

type Outcome struct {
	Text     string
	MaxSteps bool
	// Stuck: the run was stopped after the same call failed identically
	// tools.RepeatStopAt times (§14) — like MaxSteps, an incomplete run.
	Stuck bool
}

type Agent struct {
	opts    Options
	model   provider.Model
	adapter provider.Adapter
	effort  llm.Effort
	usage   llm.Usage
	cost    float64
	yolo    bool
	plan    bool
	steps   int // steps taken by the current run (run-end log line)
	// doPlan/doPlanRel: the plan file a RunPlan run executes (abs, display).
	doPlan, doPlanRel string
	hard              *savedModel
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

// cacheKey is the session's prompt-cache routing key (openai
// prompt_cache_key): stable for the session, so every request lands on the
// shard that holds its prefix.
func (a *Agent) cacheKey() string {
	if a.opts.Session == nil {
		return ""
	}
	return "moca-" + a.opts.Session.ID8()
}

// Close releases the agent's resources: the MCP servers it may have started
// (stopping their processes) and the session writer. Both front ends call it
// at shutdown.
func (a *Agent) Close() error {
	if a.opts.MCP != nil {
		a.opts.MCP.Close()
	}
	return a.opts.Session.Close()
}

// Workdir is the session's jail root — the workdir the session was started
// in; a resumed session keeps its original one.
func (a *Agent) Workdir() string { return a.opts.Env.Root }
func (a *Agent) emit(e Event) {
	switch e := e.(type) {
	case Warning:
		a.logger().Warn("warning", "text", e.Text)
	case Compacted:
		a.logger().Info("compacted", "before", e.TokensBefore, "after", e.TokensAfter)
	}
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
		Messages:   withPlanEnvelope(TransformHistory(msgs, a.model.Qualified()), a.plan),
		Tools:      a.opts.Tools.Specs(),
		ToolChoice: choice,
		CacheKey:   a.cacheKey(),
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
// A failed compaction, or a second overflow, surfaces the original error;
// a cancelled one surfaces the cancellation (esc must read as an interrupt).
func (a *Agent) turnWithRecovery(ctx context.Context, choice llm.ToolChoice) (llm.Response, []llm.ToolCall, error) {
	resp, calls, err := a.turn(ctx, choice)
	if !errors.Is(err, errOverflow) {
		return resp, calls, err
	}
	a.logger().Info("context overflow: compacting and retrying the turn")
	orig := a.lastOverflow
	if cerr := a.Compact(ctx); cerr != nil {
		if ctx.Err() != nil || errors.Is(cerr, context.Canceled) {
			return resp, nil, cerr
		}
		return resp, nil, orig
	}
	resp, calls, err = a.turn(ctx, choice)
	if errors.Is(err, errOverflow) {
		return resp, nil, orig
	}
	return resp, calls, err
}

// wrapUp ends a run with one tool-less turn answering text (the step limit,
// a stuck run).
func (a *Agent) wrapUp(ctx context.Context, text string) (Outcome, error) {
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(text)}); err != nil {
		return Outcome{}, err
	}
	resp, calls, err := a.turnWithRecovery(ctx, llm.ToolChoiceNone)
	if err != nil {
		return Outcome{}, err
	}
	if len(calls) > 0 {
		// The model ignored tool_choice: none; every call still gets
		// exactly one result (§10).
		a.abort("the wrap-up turn must not call tools; this call was not executed")
	}
	a.warnPlanMissing()
	return Outcome{Text: llm.TextOf(resp.Message)}, nil
}

// Run runs one prompt to completion (§14) and logs its end.
func (a *Agent) Run(ctx context.Context, prompt string) (Outcome, error) {
	t0 := time.Now()
	u0, c0 := a.Totals()
	a.steps = 0
	out, err := a.run(ctx, prompt)
	u, c := a.Totals()
	attrs := []any{"outcome", outcomeKind(out, err), "steps", a.steps, "duration", time.Since(t0).Round(time.Millisecond),
		slog.Group("usage", "in", u.Input-u0.Input, "out", u.Output-u0.Output,
			"cache_read", u.CacheRead-u0.CacheRead, "cache_write", u.CacheWrite-u0.CacheWrite),
		"cost", fmt.Sprintf("%.4f", c-c0)}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	a.logger().Info("run end", attrs...)
	return out, err
}

func (a *Agent) run(ctx context.Context, prompt string) (Outcome, error) {
	// Per-run tool state (plan deliverable, changes awaiting verification,
	// identical-failure counts): tools/runstate.go.
	a.opts.Env.BeginRun()
	nudged, verifyNudged, reserved, doNudged, emptyNudged := false, false, false, false, false
	if err := a.maybeCompact(ctx); err != nil {
		return Outcome{}, err
	}
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(prompt)}); err != nil {
		return Outcome{}, err
	}
	for step := 0; ; step++ {
		maxSteps := a.opts.Config.Context.MaxSteps
		if step == maxSteps {
			out, err := a.wrapUp(ctx, wrapUpText)
			out.MaxSteps = true
			return out, err
		}
		if a.plan && !reserved && !a.opts.Env.PlanWrote && maxSteps >= 4 && step == maxSteps-2 {
			// Two steps before the tool-less wrap-up: a long investigation
			// must still land its deliverable (§14).
			reserved = true
			if err := a.nudge("plan_reserve", planReserveText); err != nil {
				return Outcome{}, err
			}
		}
		resp, calls, err := a.turnWithRecovery(ctx, llm.ToolChoiceAuto)
		if err != nil {
			return Outcome{}, err
		}
		a.steps = step + 1
		if len(calls) > 0 {
			names := make([]string, len(calls))
			for i, c := range calls {
				names[i] = c.Name
			}
			a.logger().Debug("step", "n", step+1, "tools", names)
		}
		if len(calls) == 0 {
			queued, err := a.applySteering()
			if err != nil {
				return Outcome{}, err
			}
			if queued {
				continue
			}
			if strings.TrimSpace(llm.TextOf(resp.Message)) == "" {
				// No text and no calls: a provider/model glitch, not an
				// answer — one bounded nudge, then a warning (§14).
				if !emptyNudged {
					emptyNudged = true
					if err := a.nudge("empty", emptyNudge); err != nil {
						return Outcome{}, err
					}
					continue
				}
				a.emit(Warning{"the model ended the run with an empty reply"})
			}
			if a.plan && !a.opts.Env.PlanWrote {
				if !nudged {
					// One bounded retry: a plan run must end with its file.
					nudged = true
					if err := a.nudge("plan", planNudge); err != nil {
						return Outcome{}, err
					}
					continue
				}
				a.warnPlanMissing()
			}
			if !a.plan && !verifyNudged && a.opts.Env.Unverified != "" {
				// One bounded nudge: changed code ends verified or with a
				// stated reason (§14).
				verifyNudged = true
				if err := a.nudge("verify", verifyNudge(a.opts.Env.Unverified)); err != nil {
					return Outcome{}, err
				}
				continue
			}
			if !doNudged {
				doNudged = true
				if ok, err := a.nudgeDoPlan(); err != nil {
					return Outcome{}, err
				} else if ok {
					continue
				}
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
		if a.opts.Env.MaxRepeat >= tools.RepeatStopAt {
			a.emit(Warning{fmt.Sprintf("run stopped: the same failing call repeated %d times", a.opts.Env.MaxRepeat)})
			out, err := a.wrapUp(ctx, stuckText)
			out.Stuck = true
			return out, err
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

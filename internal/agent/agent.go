package agent

import (
	"context"
	"fmt"
	"strings"

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
	entries []session.Entry // full transcript (in-memory mirror)
	usage   llm.Usage
	cost    float64
	yolo    bool
	strict  struct {
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
func (a *Agent) emit(e Event) {
	if a.opts.Emit != nil {
		a.opts.Emit(e)
	}
}

func (a *Agent) append(e session.Entry) (session.Entry, error) {
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
	return llm.Request{
		Model:      a.model.ID,
		System:     a.opts.System,
		Messages:   session.Messages(a.entries),
		Tools:      a.opts.Tools.Specs(),
		ToolChoice: choice,
		MaxTokens:  a.model.MaxTokens(a.opts.Config.Context.ReserveTokens),
		Effort:     a.effort,
	}
}

// turn streams one model response and persists it. It returns the response
// and the persisted tool calls.
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
			body = append(body, c)
		}
	}
	cost := a.model.CostOf(resp.Usage)
	a.usage, a.cost = a.usage.Add(resp.Usage), a.cost+cost
	u := resp.Usage
	msgEntry, err := a.append(session.Entry{Type: session.TypeMessage, Model: a.model.Qualified(), Usage: &u, Cost: cost,
		Message: &llm.Message{Role: llm.RoleAssistant, Content: body}})
	if err != nil {
		return resp, nil, err
	}
	for _, c := range calls {
		if _, err := a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: msgEntry.ID, Call: c}}); err != nil {
			return resp, nil, err
		}
	}
	a.emit(TurnEnd{Message: resp.Message, Usage: resp.Usage, Cost: cost, Stop: resp.Stop})
	return resp, calls, nil
}

func textOf(m llm.Message) string {
	var sb strings.Builder
	for _, c := range m.Content {
		if c.Type == llm.BlockText {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

func (a *Agent) result(call llm.ToolCall, r tools.Result) error {
	_, err := a.append(session.Entry{Type: session.TypeToolResult,
		ToolResult: &llm.ToolResult{CallID: call.ID, Content: r.Content, IsError: r.IsError}})
	a.emit(ToolEnd{Call: call, Result: r})
	return err
}

// abort appends synthetic results for every unanswered call.
func (a *Agent) abort(reason string) {
	for _, fix := range session.Repair(a.entries, reason) {
		a.append(fix)
	}
}

func (a *Agent) Run(ctx context.Context, prompt string) (Outcome, error) {
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(prompt)}); err != nil {
		return Outcome{}, err
	}
	for step := 0; ; step++ {
		if step == a.opts.Config.Context.MaxSteps {
			a.append(session.Entry{Type: session.TypeMessage, Message: userText(wrapUpText)})
			resp, _, err := a.turn(ctx, llm.ToolChoiceNone)
			if err != nil {
				return Outcome{}, err
			}
			return Outcome{Text: textOf(resp.Message), MaxSteps: true}, nil
		}
		resp, calls, err := a.turn(ctx, llm.ToolChoiceAuto)
		if err != nil {
			return Outcome{}, err
		}
		if len(calls) == 0 {
			return Outcome{Text: textOf(resp.Message)}, nil
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
	}
}

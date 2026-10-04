package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type completionsAdapter struct {
	m    Model
	url  string
	cred CredentialFunc
	hc   *http.Client
}

func newOpenAICompletions(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter {
	return &completionsAdapter{m: m, url: strings.TrimRight(baseURL, "/") + "/chat/completions", cred: cred, hc: hc}
}

func (a *completionsAdapter) body(req llm.Request) map[string]any {
	var msgs []map[string]any
	if req.System != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		var text strings.Builder
		var calls []map[string]any
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockText:
				text.WriteString(c.Text)
			case llm.BlockToolUse:
				calls = append(calls, map[string]any{"id": c.ToolCall.ID, "type": "function",
					"function": map[string]any{"name": c.ToolCall.Name, "arguments": string(c.ToolCall.Input)}})
			case llm.BlockToolResult:
				msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": c.ToolResult.CallID, "content": c.ToolResult.Content})
			}
		}
		if text.Len() == 0 && calls == nil {
			continue
		}
		msg := map[string]any{"role": string(m.Role), "content": text.String()}
		if calls != nil {
			msg["tool_calls"] = calls
		}
		msgs = append(msgs, msg)
	}
	b := map[string]any{"model": req.Model, "messages": msgs, "stream": true, "max_tokens": req.MaxTokens,
		"stream_options": map[string]any{"include_usage": true}}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"type": "function", "function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": t.Schema}}
		}
		b["tools"] = tools
		if req.ToolChoice != "" {
			b["tool_choice"] = string(req.ToolChoice)
		}
	}
	if a.m.ThinkingMode == "openai" {
		if v := a.m.ThinkingLevelMap[req.Effort]; v != "" {
			b["reasoning_effort"] = v
		}
	}
	return b
}

type completionsChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		Prompt     int `json:"prompt_tokens"`
		Completion int `json:"completion_tokens"`
		Details    struct {
			Cached int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *completionsAdapter) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	cred, err := a.cred(ctx)
	if err != nil {
		return llm.Response{}, err
	}
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+cred.Token)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := post(ctx, a.hc, a.url, hdr, a.body(req))
	if err != nil {
		return llm.Response{}, openaiOverflow(err)
	}
	defer resp.Body.Close()

	var thinking, text strings.Builder
	type pending struct {
		call *llm.ToolCall
		args strings.Builder
	}
	calls := map[int]*pending{}
	out := llm.Response{Stop: llm.StopEnd}
	err = readSSE(ctx, resp.Body, stallTimeout, func(_, data string) error {
		if data == "[DONE]" {
			return nil
		}
		var ch completionsChunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return fmt.Errorf("openai-completions: bad chunk: %w", err)
		}
		if ch.Error != nil {
			return fmt.Errorf("openai-completions: %s", ch.Error.Message)
		}
		if u := ch.Usage; u != nil {
			out.Usage = llm.Usage{Input: u.Prompt - u.Details.Cached, CacheRead: u.Details.Cached, Output: u.Completion}
		}
		for _, c := range ch.Choices {
			if r := c.Delta.ReasoningContent + c.Delta.Reasoning; r != "" {
				thinking.WriteString(r)
				emit(llm.Event{Type: llm.EventThinking, Text: r})
			}
			if c.Delta.Content != "" {
				text.WriteString(c.Delta.Content)
				emit(llm.Event{Type: llm.EventText, Text: c.Delta.Content})
			}
			for _, tc := range c.Delta.ToolCalls {
				p := calls[tc.Index]
				if p == nil {
					p = &pending{call: &llm.ToolCall{}}
					calls[tc.Index] = p
				}
				if tc.ID != "" {
					p.call.ID = tc.ID
				}
				if tc.Function.Name != "" {
					p.call.Name = tc.Function.Name
				}
				p.args.WriteString(tc.Function.Arguments)
			}
			switch c.FinishReason {
			case "tool_calls":
				out.Stop = llm.StopToolUse
			case "length":
				out.Stop = llm.StopLength
			case "content_filter":
				out.Stop = llm.StopRefusal
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	out.Message.Role = llm.RoleAssistant
	if thinking.Len() > 0 {
		out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockThinking, Text: thinking.String(), Model: req.Model})
	}
	if text.Len() > 0 {
		out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockText, Text: text.String()})
	}
	idx := make([]int, 0, len(calls))
	for i := range calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		p := calls[i]
		in := p.args.String()
		if in == "" {
			in = "{}"
		}
		p.call.Input = json.RawMessage(in)
		emit(llm.Event{Type: llm.EventToolCall, ToolCall: p.call})
		out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: p.call})
	}
	if len(calls) > 0 {
		out.Stop = llm.StopToolUse // some servers send finish_reason "stop" with tool calls
	}
	return out, nil
}

func openaiOverflow(err error) error {
	var he *HTTPError
	if errors.As(err, &he) && he.Status == 400 &&
		(strings.Contains(he.Body, "context_length_exceeded") || strings.Contains(he.Body, "maximum context length")) {
		return fmt.Errorf("%w: %s", ErrContextOverflow, he.Body)
	}
	return err
}

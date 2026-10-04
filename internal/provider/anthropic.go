package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type anthropicAdapter struct {
	m    Model
	url  string
	cred CredentialFunc
	hc   *http.Client
}

func newAnthropic(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter {
	return &anthropicAdapter{m: m, url: strings.TrimRight(baseURL, "/") + "/v1/messages", cred: cred, hc: hc}
}

var ephemeral = map[string]string{"type": "ephemeral"}

func (a *anthropicAdapter) body(req llm.Request) map[string]any {
	cache := !req.NoCacheWrite
	b := map[string]any{"model": req.Model, "max_tokens": req.MaxTokens, "stream": true}
	if req.System != "" {
		sys := map[string]any{"type": "text", "text": req.System}
		if cache {
			sys["cache_control"] = ephemeral
		}
		b["system"] = []any{sys}
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Schema}
		}
		if cache {
			tools[len(tools)-1]["cache_control"] = ephemeral
		}
		b["tools"] = tools
		if req.ToolChoice != "" {
			b["tool_choice"] = map[string]string{"type": string(req.ToolChoice)}
		}
	}
	msgs := make([]map[string]any, len(req.Messages))
	for i, m := range req.Messages {
		blocks := make([]map[string]any, 0, len(m.Content))
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockText:
				blocks = append(blocks, map[string]any{"type": "text", "text": c.Text})
			case llm.BlockThinking:
				if c.Redacted {
					blocks = append(blocks, map[string]any{"type": "redacted_thinking", "data": c.Signature})
					break
				}
				blocks = append(blocks, map[string]any{"type": "thinking", "thinking": c.Text, "signature": c.Signature})
			case llm.BlockToolUse:
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": c.ToolCall.ID, "name": c.ToolCall.Name, "input": c.ToolCall.Input})
			case llm.BlockToolResult:
				blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": c.ToolResult.CallID,
					"content": c.ToolResult.Content, "is_error": c.ToolResult.IsError})
			}
		}
		if cache && i == len(req.Messages)-1 && len(blocks) > 0 {
			blocks[len(blocks)-1]["cache_control"] = ephemeral
		}
		msgs[i] = map[string]any{"role": string(m.Role), "content": blocks}
	}
	b["messages"] = msgs
	switch a.m.ThinkingMode {
	case "adaptive":
		b["thinking"] = map[string]any{"type": "adaptive", "display": "summarized"}
		b["output_config"] = map[string]any{"effort": a.m.ThinkingLevelMap[req.Effort]}
	case "budget":
		if req.Effort != llm.EffortOff {
			b["thinking"] = map[string]any{"type": "enabled", "budget_tokens": a.m.BudgetTokens(req.MaxTokens)}
		}
	}
	return b
}

type anthropicEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message struct {
		Usage anthropicUsage `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
		Text string `json:"text"`
		Data string `json:"data"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		Signature   string `json:"signature"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage anthropicUsage `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicUsage struct {
	Input      int `json:"input_tokens"`
	Output     int `json:"output_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
}

func (a *anthropicAdapter) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	cred, err := a.cred(ctx)
	if err != nil {
		return llm.Response{}, err
	}
	hdr := http.Header{}
	hdr.Set("anthropic-version", "2023-06-01")
	if cred.OAuth {
		hdr.Set("Authorization", "Bearer "+cred.Token)
	} else {
		hdr.Set("x-api-key", cred.Token)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := post(ctx, a.hc, a.url, hdr, a.body(req))
	if err != nil {
		return llm.Response{}, anthropicOverflow(err)
	}
	defer resp.Body.Close()

	var out llm.Response
	out.Message.Role = llm.RoleAssistant
	blocks := map[int]*llm.ContentBlock{}
	partial := map[int]*strings.Builder{}
	var order []int
	err = readSSE(ctx, resp.Body, stallTimeout, func(_, data string) error {
		var ev anthropicEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return fmt.Errorf("anthropic: bad event: %w", err)
		}
		switch ev.Type {
		case "message_start":
			u := ev.Message.Usage
			out.Usage = llm.Usage{Input: u.Input, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Output: u.Output}
		case "content_block_start":
			cb := &llm.ContentBlock{}
			switch ev.ContentBlock.Type {
			case "text":
				cb.Type = llm.BlockText
			case "thinking":
				cb.Type, cb.Model = llm.BlockThinking, req.Model
			case "redacted_thinking":
				// The opaque payload arrives complete in `data`; keep it in
				// Signature so the block replays verbatim (never as thinking).
				cb.Type, cb.Model, cb.Redacted, cb.Signature = llm.BlockThinking, req.Model, true, ev.ContentBlock.Data
				cb.Text = "[Reasoning redacted]"
			case "tool_use":
				cb.Type = llm.BlockToolUse
				cb.ToolCall = &llm.ToolCall{ID: ev.ContentBlock.ID, Name: ev.ContentBlock.Name}
				partial[ev.Index] = &strings.Builder{}
			default:
				return nil
			}
			blocks[ev.Index] = cb
			order = append(order, ev.Index)
		case "content_block_delta":
			cb := blocks[ev.Index]
			if cb == nil {
				return nil
			}
			switch ev.Delta.Type {
			case "text_delta":
				cb.Text += ev.Delta.Text
				emit(llm.Event{Type: llm.EventText, Text: ev.Delta.Text})
			case "thinking_delta":
				cb.Text += ev.Delta.Thinking
				emit(llm.Event{Type: llm.EventThinking, Text: ev.Delta.Thinking})
			case "signature_delta":
				cb.Signature += ev.Delta.Signature
			case "input_json_delta":
				partial[ev.Index].WriteString(ev.Delta.PartialJSON)
			}
		case "content_block_stop":
			if cb := blocks[ev.Index]; cb != nil && cb.Type == llm.BlockToolUse {
				in := partial[ev.Index].String()
				if in == "" {
					in = "{}"
				}
				cb.ToolCall.Input = json.RawMessage(in)
				emit(llm.Event{Type: llm.EventToolCall, ToolCall: cb.ToolCall})
			}
		case "message_delta":
			out.Usage.Output = ev.Usage.Output
			out.Stop = anthropicStop(ev.Delta.StopReason)
		case "error":
			if ev.Error.Type == "overloaded_error" {
				return &HTTPError{Status: 529, Body: ev.Error.Message}
			}
			return fmt.Errorf("anthropic: %s: %s", ev.Error.Type, ev.Error.Message)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	for _, i := range order {
		out.Message.Content = append(out.Message.Content, *blocks[i])
	}
	return out, nil
}

func anthropicStop(s string) llm.StopReason {
	switch s {
	case "tool_use":
		return llm.StopToolUse
	case "max_tokens", "model_context_window_exceeded":
		return llm.StopLength
	case "refusal":
		return llm.StopRefusal
	}
	return llm.StopEnd
}

func anthropicOverflow(err error) error {
	var he *HTTPError
	if errors.As(err, &he) && he.Status == 400 && strings.Contains(he.Body, "prompt is too long") {
		return fmt.Errorf("%w: %s", ErrContextOverflow, he.Body)
	}
	return err
}

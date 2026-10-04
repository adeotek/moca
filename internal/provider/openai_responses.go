package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type responsesAdapter struct {
	m    Model
	url  string
	cred CredentialFunc
	hc   *http.Client
}

func newOpenAIResponses(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter {
	return &responsesAdapter{m: m, url: strings.TrimRight(baseURL, "/") + "/responses", cred: cred, hc: hc}
}

func (a *responsesAdapter) body(req llm.Request) map[string]any {
	var input []map[string]any
	for _, m := range req.Messages {
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockText:
				typ := "input_text"
				if m.Role == llm.RoleAssistant {
					typ = "output_text"
				}
				input = append(input, map[string]any{"type": "message", "role": string(m.Role),
					"content": []map[string]any{{"type": typ, "text": c.Text}}})
			case llm.BlockThinking:
				if c.Signature == "" {
					continue // cannot replay reasoning without its encrypted payload
				}
				input = append(input, map[string]any{"type": "reasoning", "id": c.ThinkingID,
					"encrypted_content": c.Signature, "summary": []any{}})
			case llm.BlockToolUse:
				input = append(input, map[string]any{"type": "function_call", "call_id": c.ToolCall.ID,
					"name": c.ToolCall.Name, "arguments": string(c.ToolCall.Input)})
			case llm.BlockToolResult:
				input = append(input, map[string]any{"type": "function_call_output",
					"call_id": c.ToolResult.CallID, "output": c.ToolResult.Content})
			}
		}
	}
	b := map[string]any{"model": req.Model, "input": input, "stream": true, "store": false,
		"max_output_tokens": req.MaxTokens, "include": []string{"reasoning.encrypted_content"}}
	if req.System != "" {
		b["instructions"] = req.System
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"type": "function", "name": t.Name, "description": t.Description, "parameters": t.Schema}
		}
		b["tools"] = tools
		if req.ToolChoice != "" {
			b["tool_choice"] = string(req.ToolChoice)
		}
	}
	if a.m.ThinkingMode == "openai" {
		if v := a.m.ThinkingLevelMap[req.Effort]; v != "" {
			b["reasoning"] = map[string]any{"effort": v, "summary": "auto"}
		}
	}
	return b
}

type responsesEvent struct {
	Type   string `json:"type"`
	Delta  string `json:"delta"`
	ItemID string `json:"item_id"`
	Item   struct {
		Type             string `json:"type"`
		ID               string `json:"id"`
		CallID           string `json:"call_id"`
		Name             string `json:"name"`
		Arguments        string `json:"arguments"`
		EncryptedContent string `json:"encrypted_content"`
		Summary          []struct {
			Text string `json:"text"`
		} `json:"summary"`
	} `json:"item"`
	Response struct {
		Status            string `json:"status"`
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Usage struct {
			Input   int `json:"input_tokens"`
			Output  int `json:"output_tokens"`
			Details struct {
				Cached int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (a *responsesAdapter) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
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

	out := llm.Response{Stop: llm.StopEnd}
	out.Message.Role = llm.RoleAssistant
	var text strings.Builder
	reasoning := map[string]*strings.Builder{} // per reasoning item: streamed summary deltas
	flushText := func() {
		if text.Len() > 0 {
			out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockText, Text: text.String()})
			text.Reset()
		}
	}
	hasCalls := false
	done := false
	err = readSSE(ctx, resp.Body, stallTimeout, func(_, data string) error {
		var ev responsesEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return fmt.Errorf("openai-responses: bad event: %w", err)
		}
		switch ev.Type {
		case "response.output_text.delta":
			text.WriteString(ev.Delta)
			emit(llm.Event{Type: llm.EventText, Text: ev.Delta})
		case "response.reasoning_summary_text.delta":
			if ev.ItemID != "" {
				b := reasoning[ev.ItemID]
				if b == nil {
					b = &strings.Builder{}
					reasoning[ev.ItemID] = b
				}
				b.WriteString(ev.Delta)
			}
			emit(llm.Event{Type: llm.EventThinking, Text: ev.Delta})
		case "response.output_item.done":
			switch ev.Item.Type {
			case "reasoning":
				flushText()
				var sum []string
				for _, s := range ev.Item.Summary {
					sum = append(sum, s.Text)
				}
				summary := strings.Join(sum, "\n")
				if summary == "" {
					// The final item may omit the summary; keep the streamed text.
					if b := reasoning[ev.Item.ID]; b != nil {
						summary = b.String()
					}
				}
				delete(reasoning, ev.Item.ID)
				out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockThinking,
					Text: summary, ThinkingID: ev.Item.ID, Signature: ev.Item.EncryptedContent, Model: req.Model})
			case "function_call":
				flushText()
				args := ev.Item.Arguments
				if args == "" {
					args = "{}"
				}
				call := &llm.ToolCall{ID: ev.Item.CallID, Name: ev.Item.Name, Input: json.RawMessage(args)}
				emit(llm.Event{Type: llm.EventToolCall, ToolCall: call})
				out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: call})
				hasCalls = true
			case "message":
				flushText()
			}
		case "response.completed", "response.incomplete":
			done = true
			u := ev.Response.Usage
			out.Usage = llm.Usage{Input: u.Input - u.Details.Cached, CacheRead: u.Details.Cached, Output: u.Output}
			if ev.Response.IncompleteDetails.Reason == "max_output_tokens" {
				out.Stop = llm.StopLength
			}
		case "response.failed":
			if e := ev.Response.Error; e != nil {
				if e.Code == "context_length_exceeded" {
					return fmt.Errorf("%w: %s", ErrContextOverflow, e.Message)
				}
				if e.Code == "server_error" || e.Code == "rate_limit_exceeded" {
					return &HTTPError{Status: 500, Body: e.Message}
				}
				return fmt.Errorf("openai-responses: %s: %s", e.Code, e.Message)
			}
			return fmt.Errorf("openai-responses: response failed")
		case "error":
			return fmt.Errorf("openai-responses: %s: %s", ev.Code, ev.Message)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	if !done {
		// A clean EOF without response.completed/incomplete is a truncated turn.
		return out, io.ErrUnexpectedEOF
	}
	flushText()
	if hasCalls && out.Stop == llm.StopEnd {
		out.Stop = llm.StopToolUse
	}
	return out, nil
}

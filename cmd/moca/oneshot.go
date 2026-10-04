package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
)

func exitFor(ctx context.Context, err error) int {
	var ee *config.EnvError
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		return exitInterrupted
	case errors.As(err, &ee):
		return exitUsage
	default:
		return exitRuntime
	}
}

func runOneShot(ctx context.Context, o Options, cfg config.Config, stdout, stderr io.Writer) int {
	reg, err := provider.NewRegistry(cfg, http.DefaultClient, func(n provider.RetryNotice) {
		fmt.Fprintf(stderr, "retry %d/%d · %s (%v)\n", n.Attempt, n.Max, n.Wait.Round(100_000_000), n.Err)
	})
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	m, adapter, err := reg.Resolve(cfg.Model)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	effort := m.DefaultEffort()
	if o.Effort != "" {
		e, _ := llm.ParseEffort(o.Effort)
		effort = m.ClampEffort(e)
	}
	req := llm.Request{
		Model:     m.ID,
		System:    "You are moca, a coding agent.",
		MaxTokens: m.MaxTokens(cfg.Context.ReserveTokens),
		Effort:    effort,
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: o.Prompt}}}},
	}
	// stdout is the final message only (§12.5). Text is held back until the
	// stream ends: a mid-stream retry replays the whole response, and bytes
	// already written to a pipe cannot be taken back. The retry notice itself
	// goes to stderr via the registry's notify callback.
	var text strings.Builder
	resp, err := adapter.Stream(ctx, req, func(e llm.Event) {
		switch e.Type {
		case llm.EventText:
			text.WriteString(e.Text)
		case llm.EventReset:
			text.Reset()
		}
	})
	io.WriteString(stdout, text.String()) // on failure: whatever the last attempt produced
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitFor(ctx, err)
	}
	io.WriteString(stdout, "\n")
	u := resp.Usage
	fmt.Fprintf(stderr, "tokens %d/%d · $%.4f\n", u.Input+u.CacheRead+u.CacheWrite, u.Output, m.CostOf(u))
	switch resp.Stop {
	case llm.StopLength:
		fmt.Fprintln(stderr, "moca: response truncated (token limit reached)")
		return exitRuntime
	case llm.StopRefusal:
		fmt.Fprintln(stderr, "moca: model refused to answer")
		return exitRuntime
	}
	return exitOK
}

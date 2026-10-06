package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
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

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return s
}

// resolveResume maps --resume <id8|last> / --continue to a session file path;
// "" means a fresh session.
func resolveResume(o Options, wd string) (string, error) {
	if o.Resume == "" && !o.Continue {
		return "", nil
	}
	dir := filepath.Join(config.DataDir(), "sessions")
	if o.Continue {
		return session.FindForWorkdir(dir, wd)
	}
	return session.Find(dir, o.Resume)
}

func runOneShot(ctx context.Context, o Options, cfg config.Config, stdout, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	resumePath, err := resolveResume(o, wd)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	var turnText strings.Builder
	var lastStop llm.StopReason
	emit := func(e agent.Event) {
		switch e := e.(type) {
		case agent.TextDelta:
			turnText.WriteString(e.Text)
		case agent.StreamReset:
			turnText.Reset()
		case agent.TurnEnd:
			lastStop = e.Stop
			if e.Stop == llm.StopToolUse && turnText.Len() > 0 {
				fmt.Fprintln(stderr, "  "+strings.ReplaceAll(strings.TrimSpace(turnText.String()), "\n", "\n  "))
			}
			turnText.Reset()
		case agent.ToolEnd:
			if e.Result.IsError {
				fmt.Fprintf(stderr, "✗ %s %s\n", e.Call.Name, firstLine(e.Result.Content))
			} else {
				fmt.Fprintf(stderr, "▸ %s %s\n", e.Call.Name, e.Result.Summary)
			}
		case agent.Retry:
			fmt.Fprintf(stderr, "retry %d/%d · %s\n", e.Notice.Attempt, e.Notice.Max, e.Notice.Wait.Round(1e8))
		case agent.Warning:
			fmt.Fprintf(stderr, "warning: %s\n", e.Text)
		}
	}
	yolo := o.YoloOn(cfg)
	trusted := yolo
	if o.Approve != nil {
		trusted = *o.Approve
	}
	if yolo {
		fmt.Fprintln(stderr, "yolo mode: all permission checks are off")
	}
	var a *agent.Agent
	so := agent.StartOptions{Config: cfg, Workdir: wd, Effort: o.Effort, Model: o.Model,
		Trusted: trusted, Yolo: yolo, Emit: emit, Slug: session.Slug(o.Prompt)}
	if resumePath != "" {
		a, err = agent.Resume(so, resumePath)
	} else {
		a, err = agent.Start(so)
	}
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		var se *agent.StartError
		if errors.As(err, &se) {
			return exitRuntime
		}
		return exitUsage
	}
	defer a.Close()
	out, err := a.Run(ctx, o.Prompt)
	u, cost := a.Totals()
	defer fmt.Fprintf(stderr, "tokens %d/%d · $%.4f\n", u.Input+u.CacheRead+u.CacheWrite, u.Output, cost)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitFor(ctx, err)
	}
	fmt.Fprintln(stdout, out.Text)
	if out.MaxSteps {
		return exitMaxSteps
	}
	// Phase-1 contract kept: a final turn cut off at the token limit or
	// refused by the model is not a success (the partial text is printed).
	switch lastStop {
	case llm.StopLength:
		fmt.Fprintln(stderr, "moca: response truncated (token limit reached)")
		return exitRuntime
	case llm.StopRefusal:
		fmt.Fprintln(stderr, "moca: model refused to answer")
		return exitRuntime
	}
	return exitOK
}

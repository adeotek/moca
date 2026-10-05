package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tui"
)

func hasProjectResources(dir string) bool {
	for _, n := range []string{".moca", "AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

// decideTrust resolves project trust (§7): an explicit flag wins; else yolo
// trusts; else the saved decision; else, when the workdir has project
// resources, ask once (on ctx — a signal during the prompt cancels startup
// quietly) and save the answer.
func decideTrust(ctx context.Context, o Options, wd string, yolo bool) (bool, error) {
	if o.Approve != nil {
		return *o.Approve, nil
	}
	if yolo {
		return true, nil // §7.5: yolo trusts project resources unless --no-approve is explicit
	}
	ts, err := permissions.LoadTrust(filepath.Join(config.DataDir(), "trust.json"))
	if err != nil {
		return false, err
	}
	if v, known := ts.Lookup(wd); known {
		return v, nil
	}
	if !hasProjectResources(wd) {
		return false, nil
	}
	ans, err := tui.RunTrustPrompt(ctx, wd)
	if err != nil {
		return false, err
	}
	return ans, ts.Set(wd, ans)
}

// runTUI opens the interactive TUI (§11); prompt templates come from the
// project dir only when trusted, then the global config dir.
func runTUI(ctx context.Context, o Options, cfg config.Config, cfgPath string, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	yolo := o.YoloOn(cfg)
	trusted, err := decideTrust(ctx, o, wd, yolo)
	if err != nil {
		if !errors.Is(err, context.Canceled) { // ctrl+c at the trust prompt: quiet 130
			fmt.Fprintln(stderr, "moca:", err)
		}
		return exitFor(ctx, err)
	}
	var pdirs []skills.Dir
	if trusted {
		pdirs = append(pdirs, skills.Dir{Path: filepath.Join(wd, ".moca", "prompts"), Source: "project"})
	}
	pdirs = append(pdirs, skills.Dir{Path: filepath.Join(config.ConfigDir(), "prompts"), Source: "global"})
	home, _ := os.UserHomeDir()
	err = tui.Run(ctx, tui.AppOptions{
		Start:      agent.StartOptions{Config: cfg, Workdir: wd, Effort: o.Effort, Trusted: trusted, Yolo: yolo, Slug: "tui"},
		ConfigPath: cfgPath, Prompts: skills.LoadPrompts(pdirs), Home: home,
	})
	if err != nil {
		if ctx.Err() == nil { // SIGINT/SIGTERM: no message, just 130
			fmt.Fprintln(stderr, "moca:", err)
		}
		return exitFor(ctx, err)
	}
	return exitOK
}

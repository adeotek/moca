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

// projectResourceNames are the project-level files that make a workdir need a
// trust decision: moca's own project dir/instructions plus the ecosystem MCP
// configs a trusted project may load servers from (§4, §10.5).
var projectResourceNames = []string{
	".moca",
	"AGENTS.md",
	"CLAUDE.md",
	".mcp.json",
	"opencode.json",
	"opencode.jsonc",
	filepath.Join(".pi", "mcp.json"),
}

func hasProjectResources(dir string) bool {
	for _, n := range projectResourceNames {
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

// modeWithoutProject resolves the mode ignoring the project layer: the
// trust decision (yolo trusts, §7.5) must not depend on a file the workdir
// may or may not be allowed to contribute.
func modeWithoutProject(o Options, cfg config.Config) string {
	cfg.Project = nil
	mode, _ := o.ResolveMode(cfg)
	return mode
}

// themeFor resolves the run's TUI theme (§3.5): the built-in palette, the
// user config's `tui.theme` overrides, then a trusted project's (the project
// layer wins, §4). Colors only — nothing else in the TUI reads config.
func themeFor(cfg config.Config) tui.Theme {
	th := tui.DefaultTheme().WithOverrides(cfg.TUI.Theme)
	if cfg.Project != nil {
		th = th.WithOverrides(cfg.Project.TUI.Theme)
	}
	return th
}

// runTUI opens the interactive TUI (§11); prompt templates come from the
// project dir only when trusted, then the global config dir.
func runTUI(ctx context.Context, o Options, cfg config.Config, cfgPath string, stderr io.Writer) int {
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
	trustMode := modeWithoutProject(o, cfg)
	trusted, err := decideTrust(ctx, o, wd, trustMode == "yolo")
	if err != nil {
		if !errors.Is(err, context.Canceled) { // ctrl+c at the trust prompt: quiet 130
			fmt.Fprintln(stderr, "moca:", err)
		}
		return exitFor(ctx, err)
	}
	// Trusted: the project file's rules and defaultMode join in; a project
	// file that does not parse is a config error (exit 2) — but only for a
	// workdir the user chose to trust.
	if trusted {
		p, err := config.LoadProject(wd)
		if err != nil {
			fmt.Fprintln(stderr, "moca:", err)
			return exitUsage
		}
		cfg.Project = p
	}
	mode, plan := o.ResolveMode(cfg)
	var pdirs []skills.Dir
	if trusted {
		pdirs = append(pdirs, skills.Dir{Path: filepath.Join(wd, ".moca", "prompts"), Source: "project"})
	}
	promptsDir := config.PromptsDir()
	pdirs = append(pdirs, skills.Dir{Path: promptsDir, Source: "global"})
	home, _ := os.UserHomeDir()
	err = tui.Run(ctx, tui.AppOptions{
		Start:      agent.StartOptions{Config: cfg, Workdir: wd, Effort: o.Effort, Model: o.Model, Trusted: trusted, Mode: agent.PermMode(mode), Plan: plan, Slug: "tui", ConfigPath: cfgPath},
		ConfigPath: cfgPath, Prompts: skills.LoadPrompts(pdirs), PromptDirs: pdirs, Home: home,
		ResumePath:  resumePath,
		HintsPath:   filepath.Join(config.DataDir(), "hints.json"),
		HistoryPath: filepath.Join(config.DataDir(), "history.jsonl"),
		Theme:       themeFor(cfg),
	})
	if err != nil {
		if ctx.Err() == nil { // SIGINT/SIGTERM: no message, just 130
			fmt.Fprintln(stderr, "moca:", err)
		}
		return exitFor(ctx, err)
	}
	return exitOK
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/acp"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/permissions"
)

const acpUsage = "usage: moca acp [--model <provider/model>] [--effort <level>] [--mode default|auto|yolo|plan] [--approve | --no-approve]"

// runACP serves the Agent Client Protocol on stdin/stdout (`moca acp`):
// editors launch it as a child process. stdout carries JSON-RPC only;
// diagnostics go to the log file. Trust is decided per workdir by acpTrust.
func runACP(ctx context.Context, o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("moca acp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	model := fs.String("model", o.Model, "provider/model")
	effort := fs.String("effort", o.Effort, "effort level")
	mode := fs.String("mode", "", "permission mode")
	approve := fs.Bool("approve", false, "trust project resources")
	noApprove := fs.Bool("no-approve", false, "do not trust project resources")
	if err := fs.Parse(o.Sub[1:]); err != nil || fs.NArg() > 0 {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stdout, acpUsage)
			return exitOK
		}
		fmt.Fprintln(stderr, "moca:", acpUsage)
		return exitUsage
	}
	if *mode == "" && o.Mode != nil {
		*mode = *o.Mode
	}
	if *mode != "" && !slices.Contains(cliModes, *mode) {
		fmt.Fprintf(stderr, "moca: --mode must be one of %s, got %q\n", strings.Join(cliModes, ", "), *mode)
		return exitUsage
	}
	if *effort != "" {
		if _, err := llm.ParseEffort(*effort); err != nil {
			fmt.Fprintln(stderr, "moca:", err)
			return exitUsage
		}
	}
	if *approve && *noApprove {
		fmt.Fprintln(stderr, "moca: --approve and --no-approve are mutually exclusive")
		return exitUsage
	}
	explicit := o.Approve // global flags before `acp`
	if *approve || *noApprove {
		v := *approve
		explicit = &v
	}
	// yolo trusts (§7.5); decided without the project layer, as the TUI does.
	m, _ := config.ResolveMode(*mode, cfg)
	trust := acpTrust(explicit, m == "yolo", filepath.Join(config.DataDir(), "trust.json"))
	closeLog, err := startLog(cfg, cfgPath, o, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	defer closeLog()
	err = acp.Serve(ctx, stdin, stdout, acp.Options{Config: cfg, ConfigPath: cfgPath, Model: *model, Effort: *effort,
		Mode: *mode, Trust: trust, Version: config.Version})
	if err != nil && ctx.Err() == nil {
		// The log, never stderr: the editor owns that pipe, and on Windows a
		// write to it has been reported to take the child down with it
		// (SPECS §9.5). The exit code is the contract a wrapper reads.
		slog.Error("acp: serve stopped", "err", err)
		return exitRuntime
	}
	return exitOK
}

// acpTrust resolves project trust per workdir: decideTrust (tui.go) without
// its interactive prompt — an ACP server has no terminal to ask on. An
// explicit flag wins; else yolo trusts; else a saved decision is honoured
// (a "no" given in the TUI stays a no); else trusted, since the editor
// opening the folder is the trust act. Nothing is written to trust.json.
func acpTrust(approve *bool, yolo bool, trustPath string) func(cwd string) (bool, error) {
	return func(cwd string) (bool, error) {
		if approve != nil {
			return *approve, nil
		}
		if yolo {
			return true, nil
		}
		ts, err := permissions.LoadTrust(trustPath)
		if err != nil {
			return false, err
		}
		if v, known := ts.Lookup(cwd); known {
			return v, nil
		}
		return true, nil
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
)

func main() {
	// Credentials moved to ~/.config/moca/auth.json; a v0.1 store in the data
	// dir is moved once, before anything reads it. A failure leaves the old
	// file in place (nothing is lost) and only warns. (In main, not run(), so
	// tests never touch a real user store.)
	if err := provider.MigrateLegacyStore(); err != nil {
		fmt.Fprintln(os.Stderr, "moca: warning: could not move the old auth store into the config dir:", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parseArgs(args, stdin)
	if err != nil {
		if errors.Is(err, errHelp) {
			printUsage(stdout)
			return exitOK
		}
		fmt.Fprintln(stderr, "moca:", err)
		var ue usageError
		if errors.As(err, &ue) {
			return exitUsage
		}
		return exitRuntime
	}
	if o.Version {
		fmt.Fprintln(stdout, "moca", config.Version)
		return exitOK
	}
	if len(o.Sub) > 0 {
		// `update` runs before the config is loaded: updating the binary
		// must not depend on a parseable config (that may be exactly what
		// is broken).
		if o.Sub[0] == "update" {
			return runUpdate(ctx, o, stdout, stderr)
		}
		// mcp/login/logout subcommands need config but no model.
		path := o.ConfigPath
		if path == "" {
			path = config.ConfigFile()
		}
		cfg, err := config.Load(path)
		if err != nil {
			fmt.Fprintln(stderr, "moca:", err)
			return exitUsage
		}
		switch o.Sub[0] {
		case "mcp":
			return runMCP(ctx, o, cfg, path, stdin, stdout, stderr)
		case "login":
			return runLogin(ctx, o, cfg, path, stdin, stdout, stderr)
		case "logout":
			return runLogout(ctx, o, cfg, stdout, stderr)
		}
		fmt.Fprintf(stderr, "moca: unknown command %q\n", o.Sub[0])
		return exitUsage
	}
	path := o.ConfigPath
	if path == "" {
		path = config.ConfigFile()
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	if o.Model != "" {
		cfg.Model = o.Model
		if err := cfg.Validate(); err != nil {
			fmt.Fprintln(stderr, "moca:", err)
			return exitUsage
		}
	}
	if cfg.Model == "" && o.OneShot {
		// A one-shot run needs a model up front; the TUI (no -p) instead
		// opens in setup mode and offers /login + /model (§3.5).
		fmt.Fprintf(stderr, "moca: no model configured — set \"model\" in %s or pass --model provider/model\n", path)
		return exitUsage
	}
	if !o.OneShot {
		return runTUI(ctx, o, cfg, path, stderr)
	}
	return runOneShot(ctx, o, cfg, stdout, stderr)
}

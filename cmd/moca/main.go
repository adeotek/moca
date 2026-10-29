package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adeotek/moca/internal/applog"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// bootResult is what startup() found: the variables the user .env contributed
// and every diagnostic collected while booting.
type bootResult struct {
	DotEnvVars int
	Notes      []error
}

// startup does the work that has to precede every command — the optional
// ~/.config/moca/.env, then the one-time move of a v0.1 credential store — and
// returns the diagnostics instead of printing them: the channel depends on the
// mode, which only parseArgs decides (under ACP stderr belongs to the editor,
// §9.5). It runs first in run(), not in main(), so the whole boot path is
// testable.
func startup() bootResult {
	rep, err := config.LoadDotEnv(config.DotEnvFile())
	var r bootResult
	r.DotEnvVars, r.Notes = rep.Set, rep.Notes
	if err != nil {
		r.Notes = append(r.Notes, fmt.Errorf("could not read the user .env: %w", err))
	}
	// Credentials moved to ~/.config/moca/auth.json. A failure leaves the old
	// file in place (nothing is lost) and only warns.
	if err := provider.MigrateLegacyStore(); err != nil {
		r.Notes = append(r.Notes, fmt.Errorf("could not move the old auth store into the config dir: %w", err))
	}
	return r
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	applog.Discard() // before anything can log: slog's default writes to stderr (the TUI's screen)
	boot := startup()
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
	// `config` reports the file itself (see runConfigCheck), so it would only
	// print the same thing twice.
	reportHere := o.logMode() != "acp" && !(len(o.Sub) > 0 && o.Sub[0] == "config")
	if reportHere {
		// ACP's stderr belongs to the editor (§9.5); every other mode owns it.
		// Printed before the config is loaded, so a config error cannot hide a
		// broken .env line behind it. startLog repeats them into the log.
		for _, n := range boot.Notes {
			fmt.Fprintln(stderr, "moca: warning:", n)
		}
	}
	if len(o.Sub) > 0 {
		// `update` runs before the config is loaded: updating the binary
		// must not depend on a parseable config (that may be exactly what
		// is broken).
		if o.Sub[0] == "update" {
			return runUpdate(ctx, o, stdout, stderr)
		}
		// `prune` is cleanup of the data dir: like `update`, it must work
		// when the config is what is broken.
		if o.Sub[0] == "prune" {
			return runPrune(o, stdout, stderr)
		}
		// `config` reads (and prints) configuration: `check` must work when
		// the config is exactly what is broken, and `sample` needs none.
		if o.Sub[0] == "config" {
			return runConfig(o, stdout, stderr)
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
		case "acp":
			return runACP(ctx, o, cfg, path, stdin, stdout, stderr, boot)
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
		// F-1: after the override, before Validate. `--model ollama/qwen3` is
		// the opt-in for a local provider that has no config entry, and
		// validating first rejected the very model the user asked for.
		if pn, _, err := config.SplitModel(cfg.Model); err == nil {
			cfg.UseProvider(pn)
		}
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
	closeLog, err := startLog(cfg, path, o, stderr, boot)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	defer closeLog()
	t0 := time.Now()
	var code int
	if o.OneShot {
		code = runOneShot(ctx, o, cfg, stdout, stderr)
	} else {
		code = runTUI(ctx, o, cfg, path, stderr)
	}
	slog.Info("exit", "code", code, "duration", time.Since(t0).Round(time.Millisecond))
	return code
}

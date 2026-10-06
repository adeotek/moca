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
)

func main() {
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
	if cfg.Model == "" {
		fmt.Fprintf(stderr, "moca: no model configured — set \"model\" in %s or pass --model provider/model\n", path)
		return exitUsage
	}
	if !o.OneShot {
		return runTUI(ctx, o, cfg, path, stderr)
	}
	return runOneShot(ctx, o, cfg, stdout, stderr)
}

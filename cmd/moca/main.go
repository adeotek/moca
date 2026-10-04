package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/adeotek/moca/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parseArgs(args, stdin)
	if err != nil {
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
	if len(o.Sub) > 0 {
		fmt.Fprintf(stderr, "moca: %s lands in a later phase\n", o.Sub[0])
		return exitUsage
	}
	if !o.OneShot {
		fmt.Fprintln(stderr, "moca: TUI lands in phase 3; use -p")
		return exitUsage
	}
	return runOneShot(ctx, o, cfg, stdout, stderr)
}

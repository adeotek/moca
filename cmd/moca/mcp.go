package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/mcp"
)

// runMCP dispatches `moca mcp import [--yes]` and `moca mcp index`. Neither
// needs a model: they run before the model check in run().
func runMCP(ctx context.Context, o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(o.Sub) < 2 {
		fmt.Fprintln(stderr, "usage: moca mcp import [--yes] | moca mcp index")
		return exitUsage
	}
	switch o.Sub[1] {
	case "import":
		return runMCPImport(o, cfg, cfgPath, stdin, stdout)
	case "index":
		return runMCPIndex(ctx, cfg, stdout, stderr)
	}
	fmt.Fprintf(stderr, "moca: unknown mcp subcommand %q\n", o.Sub[1])
	return exitUsage
}

func runMCPImport(o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout io.Writer) int {
	yes := slices.Contains(o.Sub[2:], "--yes")
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stdout, "moca:", err)
		return exitRuntime
	}
	home, _ := os.UserHomeDir()
	srcs := mcp.DiscoverSources(home, wd)
	if len(srcs) == 0 {
		fmt.Fprintln(stdout, "no Claude Code, OpenCode or Pi MCP configs found")
		return exitOK
	}
	adds, vars, notes := mcp.Plan(srcs, cfg.MCP.Servers, wd)
	names := slices.Sorted(maps.Keys(adds))
	if len(names) == 0 {
		fmt.Fprintln(stdout, "nothing to import")
	} else {
		fmt.Fprintf(stdout, "will add %d server(s) to %s:\n\n", len(names), cfgPath)
		for _, name := range names {
			b, _ := json.MarshalIndent(adds[name], "  ", "  ")
			fmt.Fprintf(stdout, "  %s:\n  %s\n\n", name, b)
		}
		if len(vars) > 0 {
			fmt.Fprintln(stdout, "secrets were replaced by env: references — export these before the servers can start:")
			for _, v := range vars {
				fmt.Fprintf(stdout, "  export %s=<value of %s from the %s config>\n", v.Name, v.Key, v.Tool)
			}
			fmt.Fprintln(stdout)
		}
	}
	if len(notes) > 0 {
		fmt.Fprintln(stdout, "notes:")
		for _, n := range notes {
			fmt.Fprintln(stdout, "  "+n)
		}
	}
	if len(names) == 0 {
		return exitOK
	}
	if !yes && !confirm(stdin, fmt.Sprintf("write %d server(s) to %s? [y/N] ", len(names), cfgPath), stdout) {
		fmt.Fprintln(stdout, "aborted")
		return exitOK
	}
	for _, name := range names {
		b, err := json.Marshal(adds[name])
		if err != nil {
			fmt.Fprintln(stdout, "moca:", err)
			return exitRuntime
		}
		added, err := config.SetObjectEntry(cfgPath, []string{"mcp", "servers"}, name, string(b))
		if err != nil {
			fmt.Fprintf(stdout, "moca: %s: %v\n", name, err)
			return exitRuntime
		}
		if !added {
			fmt.Fprintf(stdout, "  %s: already present, kept\n", name)
		}
	}
	fmt.Fprintf(stdout, "wrote %d server(s) to %s\n", len(names), cfgPath)
	return exitOK
}

func confirm(stdin io.Reader, prompt string, stdout io.Writer) bool {
	fmt.Fprint(stdout, prompt)
	sc := bufio.NewScanner(stdin)
	if !sc.Scan() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(sc.Text())) {
	case "y", "yes":
		return true
	}
	return false
}

func runMCPIndex(ctx context.Context, cfg config.Config, stdout, stderr io.Writer) int {
	if len(cfg.MCP.Servers) == 0 {
		fmt.Fprintln(stdout, "no MCP servers configured")
		return exitOK
	}
	ix, _ := mcp.LoadIndex(filepath.Join(config.DataDir(), "mcp-index.json"))
	mgr := mcp.NewManager(cfg.MCP.Servers, time.Duration(cfg.MCP.IdleTimeout)*time.Second, ix, mcp.Options{BaseEnv: os.Environ()})
	defer mgr.Close()
	counts, errs := mgr.IndexAll(ctx)
	code := exitOK
	for _, name := range mgr.Servers() {
		if err := errs[name]; err != nil {
			fmt.Fprintf(stderr, "moca: %s: %v\n", name, err)
			code = exitRuntime
			continue
		}
		fmt.Fprintf(stdout, "%s: %d tools\n", name, counts[name])
	}
	return code
}

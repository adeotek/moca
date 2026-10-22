package main

import (
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/applog"
	"github.com/adeotek/moca/internal/config"
)

// startLog opens the diagnostic log for a TUI, -p or ACP run (SPECS §13.5).
// MOCA_LOG overrides log.level for this run; an invalid value is a usage
// error. A log that cannot be opened only warns: the run goes on without it —
// except under ACP, where the warning is dropped (see below).
// The returned func closes the file.
func startLog(cfg config.Config, cfgPath string, o Options, stderr io.Writer) (func(), error) {
	name := cfg.Log.Level
	if v := os.Getenv("MOCA_LOG"); v != "" {
		name = v
	}
	level, err := applog.ParseLevel(name)
	if err != nil {
		return nil, usageError{fmt.Errorf("MOCA_LOG: %w", err)}
	}
	c, _, err := applog.Open(filepath.Join(config.StateDir(), "logs"), level, cfg.RetentionDays())
	if err != nil {
		if o.logMode() != "acp" {
			// An external agent's stderr is not a diagnostic channel: the
			// editor owns the pipe, and Zed on Windows has been reported to
			// lose — even kill — a child over a write to it. The full contract
			// is §9.5's: stdout carries JSON-RPC, diagnostics go to the log.
			fmt.Fprintln(stderr, "moca: warning: diagnostic log disabled:", err)
		}
		return func() {}, nil
	}
	if level == applog.LevelOff {
		return func() {}, nil
	}
	slog.Info("start", "version", config.Version, "os", runtime.GOOS, "arch", runtime.GOARCH,
		"mode", o.logMode(), "config", cfgPath, "level", strings.ToLower(strings.TrimSpace(name)))
	slog.Debug("config", "model", cfg.Model, "providers", slices.Sorted(maps.Keys(cfg.Providers)),
		"mcp", slices.Sorted(maps.Keys(cfg.MCP.Servers)), "web", cfg.Web.Search.Provider)
	return func() { c.Close() }, nil
}

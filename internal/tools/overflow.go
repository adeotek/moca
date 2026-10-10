package tools

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Overflow (§10): shell, search and web outputs over their model-facing cap
// are cut, but the full text is kept on disk and the result names the file —
// the model reads or searches that file instead of re-running the command, so
// the caps stay small without losing anything. Env.SpillDir is a read root
// of the jail (agent/start.go); files are named <SpillPrefix><kind>-*.txt and
// pruned with the snapshot retention.

// Spill saves full and returns the note naming the file, or "" when spilling
// is off (no SpillDir) or fails — the cut output then stands alone.
func Spill(env *Env, kind, full string) string {
	if env.SpillDir == "" {
		return ""
	}
	fail := func(err error) string {
		slog.Warn("spill failed", "kind", kind, "error", err)
		return ""
	}
	if err := os.MkdirAll(env.SpillDir, 0o700); err != nil {
		return fail(err)
	}
	f, err := os.CreateTemp(env.SpillDir, env.SpillPrefix+kind+"-*.txt")
	if err != nil {
		return fail(err)
	}
	_, werr := f.WriteString(full)
	if cerr := f.Close(); werr != nil || cerr != nil {
		os.Remove(f.Name())
		return fail(errors.Join(werr, cerr))
	}
	slog.Debug("spill", "kind", kind, "bytes", len(full))
	lines := strings.Count(full, "\n")
	if full != "" && !strings.HasSuffix(full, "\n") {
		lines++
	}
	return fmt.Sprintf("[full output (%d lines, %s) saved to %s — read it with offset/limit or search it; do not re-run the command to see it]",
		lines, webSize(len(full)), f.Name())
}

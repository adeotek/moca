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

// Spill saves full and returns the note naming the file. A nil error with an
// empty note means spilling is off (no SpillDir); a non-nil error means the
// note is empty and the cut output stands alone.
func Spill(env *Env, kind, full string) (string, error) {
	if env.SpillDir == "" {
		return "", nil
	}
	fail := func(err error) (string, error) {
		slog.Warn("spill failed", "kind", kind, "error", err)
		return "", err
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
	// T-4: not "the full output" — the capture keeps head and tail up to 64K
	// and marks what it dropped, so the file can be missing a middle. The
	// description and this note now say what the file actually holds.
	return fmt.Sprintf("[output saved to %s — read it with offset/limit or search it; do not re-run the command to see it (%d lines, %s; the capture keeps head and tail)]",
		f.Name(), lines, webSize(len(full))), nil
}

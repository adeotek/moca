package main

import (
	"fmt"
	"io"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/skills"
)

// runPrune implements `moca prune [--dry-run]`: it removes what previous
// versions left in the data dir. Today that is the built-in skill
// extractions — each binary version extracts to its own content-hashed
// directory and keeps the older ones so a still-running moca can finish
// reading its own — plus the legacy version-keyed layout and crashed
// extractions. Sessions, snapshots, history, trust and everything under the
// config dir (seeded skills included) are never touched. Like `update` it
// needs no config.
func runPrune(o Options, stdout, stderr io.Writer) int {
	dry := false
	for _, a := range o.Sub[1:] {
		if a != "--dry-run" {
			fmt.Fprintln(stderr, "moca: usage: moca prune [--dry-run]")
			return exitUsage
		}
		dry = true
	}
	removed, err := skills.PruneBuiltins(config.DataDir(), dry)
	verb, done := "removed", "pruned"
	if dry {
		verb, done = "would remove", "would remove"
	}
	var total int64
	for _, p := range removed {
		fmt.Fprintf(stdout, "%s %s (%s)\n", verb, p.Path, formatBytes(p.Bytes))
		total += p.Bytes
	}
	switch {
	case len(removed) == 0 && err == nil:
		fmt.Fprintln(stdout, "nothing to prune")
	case len(removed) > 0:
		noun := "entries"
		if len(removed) == 1 {
			noun = "entry"
		}
		fmt.Fprintf(stdout, "%s %d %s (%s)", done, len(removed), noun, formatBytes(total))
		if dry {
			fmt.Fprint(stdout, " — run without --dry-run to delete")
		}
		fmt.Fprintln(stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "moca: prune:", err)
		return exitRuntime
	}
	return exitOK
}

// formatBytes renders a size for humans: 1023 B, 1.5 KiB, 5.0 MiB, 2.0 GiB.
func formatBytes(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	case n < 1<<30:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
}

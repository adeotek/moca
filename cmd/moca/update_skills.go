package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/skills"
)

// runUpdateSkills implements `moca update skills [--check]`: it rewrites the
// seeded standard skills (rtk, graphify) in the global skills dir from the
// binary's embedded bundle. Startup seeding is create-only — once planted a
// skill belongs to the user, and a newer binary never overwrites it — so this
// command is the explicit refresh. Bundled files follow the binary; a file the
// user added is kept, and a symlinked skill is reported instead of written
// through. Like `moca update`, it needs no config (the config dir is XDG-only).
func runUpdateSkills(check bool, stdout, stderr io.Writer) int {
	dir := filepath.Join(config.ConfigDir(), "skills")
	ups, err := skills.UpdateSeedSkills(dir, check)
	fmt.Fprintf(stdout, "skills dir: %s\n", dir)
	for _, u := range ups {
		switch {
		case u.Skipped != "":
			fmt.Fprintf(stdout, "%s: skipped (%s — left as is)", u.Name, u.Skipped)
		case u.Created:
			if check {
				fmt.Fprintf(stdout, "%s: would be created (%d %s)", u.Name, len(u.Changed), plural(len(u.Changed), "file", "files"))
			} else {
				fmt.Fprintf(stdout, "%s: created (%d %s)", u.Name, len(u.Changed), plural(len(u.Changed), "file", "files"))
			}
		case len(u.Changed) == 0:
			fmt.Fprintf(stdout, "%s: up to date", u.Name)
		default:
			verb := "updated"
			if check {
				verb = "would update"
			}
			fmt.Fprintf(stdout, "%s: %s %d %s (%s)", u.Name, verb, len(u.Changed), plural(len(u.Changed), "file", "files"), strings.Join(u.Changed, ", "))
		}
		if u.Skipped == "" && !u.Created && len(u.Kept) > 0 {
			fmt.Fprintf(stdout, " — kept %d extra %s (%s)", len(u.Kept), plural(len(u.Kept), "file", "files"), strings.Join(u.Kept, ", "))
		}
		fmt.Fprintln(stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "moca: update skills:", err)
		return exitRuntime
	}
	return exitOK
}

// plural picks the singular or plural noun for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/skills"
)

// runSkills implements `moca skills [list]` and `moca skills fork <name>`.
//
// The listing exists because resolution order is invisible otherwise: project,
// then global, then built-in, with the first name winning (SPECS §3.5). A user
// who wants their own `review` has no way to discover the built-in exists or
// that a copy shadows it, and the built-in is what the fork is for.
//
// Fork copies the built-in into the global dir so the copy is the user's: the
// bundled one follows the binary and would rewrite an edit on the next update,
// which is the trap the note in every built-in SKILL.md warns about. Nothing
// outside the skills dirs is read or written, and an existing skill is never
// overwritten.
func runSkills(args []string, stdout, stderr io.Writer) int {
	action := "list"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "list":
		return listSkills(stdout, stderr)
	case "fork":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "moca: usage: moca skills fork <name>")
			return exitUsage
		}
		builtinDir, err := skills.ExtractBuiltins(config.DataDir())
		if err != nil {
			fmt.Fprintln(stderr, "moca: skills:", err)
			return exitRuntime
		}
		dir := filepath.Join(config.ConfigDir(), "skills")
		dst, err := skills.Fork(builtinDir, dir, args[1])
		if err != nil {
			fmt.Fprintln(stderr, "moca: skills fork:", err)
			fmt.Fprintln(stderr, "moca: usage: moca skills fork <name>")
			return exitUsage
		}
		fmt.Fprintf(stdout, "forked %s -> %s\n", args[1], dst)
		fmt.Fprintf(stdout, "the copy shadows the built-in from the next session; edit %s\n", filepath.Join(dst, "SKILL.md"))
		return exitOK
	case "help", "-h", "--help":
		fmt.Fprint(stdout, skillsUsage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "moca: unknown skills command %q\n%s", action, skillsUsage)
		return exitUsage
	}
}

const skillsUsage = `usage: moca skills [list]           list the skills discovery would load, in resolution order
       moca skills fork <name>      copy a built-in skill into your own skills dir, to customise it

A built-in skill follows the binary: editing it in the data dir does not survive
an update. Fork it instead, and the copy (in the global skills dir, or
<repo>/.moca/skills for a team) shadows the built-in by name.
`

// listSkills prints what discovery would load, in resolution order. The project
// dir belongs to the working directory: `moca skills` is a read-only listing, so
// it shows what a trusted session there would see; the trust decision still
// gates whether the agent loads it.
func listSkills(stdout, stderr io.Writer) int {
	builtinDir, err := skills.ExtractBuiltins(config.DataDir())
	if err != nil {
		fmt.Fprintln(stderr, "moca: skills:", err)
		return exitRuntime
	}
	var dirs []skills.Dir
	if wd, err := os.Getwd(); err == nil {
		if fi, err := os.Stat(filepath.Join(wd, ".moca", "skills")); err == nil && fi.IsDir() {
			dirs = append(dirs, skills.Dir{Path: filepath.Join(wd, ".moca", "skills"), Source: "project"})
		}
	}
	dirs = append(dirs,
		skills.Dir{Path: filepath.Join(config.ConfigDir(), "skills"), Source: "global"},
		skills.Dir{Path: builtinDir, Source: "builtin"})

	sk, errs := skills.Discover(dirs)
	for _, e := range errs {
		fmt.Fprintln(stderr, "moca: skills:", e)
	}
	if len(sk) == 0 {
		fmt.Fprintln(stdout, "no skills: none in .moca/skills, the global skills dir or the built-in set")
		return exitOK
	}

	nw, sw := len("NAME"), len("SOURCE")
	for _, k := range sk {
		nw = max(nw, len(k.Name))
		sw = max(sw, len(k.Source))
	}
	fmt.Fprintln(stdout, "skills (resolution order: project, global, built-in — the first name wins)")
	fmt.Fprintf(stdout, "%-*s  %-*s  %s\n", nw, "NAME", sw, "SOURCE", "DESCRIPTION")
	builtins := 0
	for _, k := range sk {
		if k.Source == "builtin" {
			builtins++
		}
		desc := strings.Join(strings.Fields(k.Description), " ")
		fmt.Fprintf(stdout, "%-*s  %-*s  %s\n", nw, k.Name, sw, k.Source, desc)
	}
	if builtins > 0 {
		fmt.Fprintf(stdout, "built-in skills follow the binary: `moca skills fork <name>` copies one into %s, where the copy wins\n",
			filepath.Join(config.ConfigDir(), "skills"))
	}
	return exitOK
}

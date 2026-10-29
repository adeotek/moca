package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/permissions"
)

// runConfig implements `moca config check` and `moca config sample` (SPECS §3).
//
// It is dispatched before the config is loaded, on purpose: `check` must work
// when the config is exactly what is broken, and `sample` needs no config at
// all.
func runConfig(o Options, stdout, stderr io.Writer) int {
	if len(o.Sub) < 2 {
		fmt.Fprintln(stderr, "usage: moca config check | moca config sample")
		return exitUsage
	}
	if len(o.Sub) > 2 {
		fmt.Fprintf(stderr, "moca: `config %s` takes no arguments\n", o.Sub[1])
		return exitUsage
	}
	switch o.Sub[1] {
	case "sample":
		fmt.Fprint(stdout, config.Sample())
		return exitOK
	case "check":
		return runConfigCheck(o, stdout, stderr)
	}
	fmt.Fprintf(stderr, "moca: unknown config command %q (want `check` or `sample`)\n", o.Sub[1])
	return exitUsage
}

// runConfigCheck validates the user config and the project one, and reports
// every issue it finds. The project file is only read for a trusted workdir —
// the same rule a run follows (§4: an untrusted .moca/config.jsonc is never
// read, so `check` must not read it either); an untrusted one is reported as
// not read rather than silently skipped.
func runConfigCheck(o Options, stdout, stderr io.Writer) int {
	issues := 0
	report := func(ok bool, format string, args ...any) {
		if !ok {
			issues++
		}
		fmt.Fprintf(stdout, format+"\n", args...)
	}

	userPath := o.ConfigPath
	if userPath == "" {
		userPath = config.ConfigFile()
	}
	fmt.Fprintln(stdout, "user config:")
	switch _, err := os.Stat(userPath); {
	case errors.Is(err, fs.ErrNotExist):
		fmt.Fprintf(stdout, "  %s: not found — built-in defaults apply\n", userPath)
	case err != nil:
		report(false, "  %s: %v", userPath, err)
	default:
		if _, err := config.Load(userPath); err != nil {
			report(false, "  %s: %v", userPath, err)
		} else {
			fmt.Fprintf(stdout, "  %s: ok\n", userPath)
		}
	}

	// The .env beside the config is read at startup and its env: references
	// are what a provider key resolves against, so a bad line there has to be
	// diagnosable without watching a TUI's first frame.
	fmt.Fprintln(stdout, "environment file:")
	envPath := config.DotEnvFile()
	d, envErr := config.ReadDotEnv(envPath)
	switch {
	case envErr != nil:
		report(false, "  %s: %v", envPath, envErr)
	case !d.Found:
		fmt.Fprintf(stdout, "  %s: not found — optional (env: references then need the shell)\n", envPath)
	default:
		for _, e := range d.LineErr {
			report(false, "  %v", e)
		}
		if d.ModeNote != nil {
			// Advice, not a defect: the file loads either way.
			fmt.Fprintf(stdout, "  note: %v\n", d.ModeNote)
		}
		fmt.Fprintf(stdout, "  %s: %d variable(s)\n", envPath, len(d.Vars))
	}

	wd, err := os.Getwd()
	if err != nil {
		report(false, "project config: cannot resolve the working directory: %v", err)
	} else {
		projPath := config.ProjectConfigFile(wd)
		fmt.Fprintln(stdout, "project config:")
		switch _, statErr := os.Stat(projPath); {
		case errors.Is(statErr, fs.ErrNotExist):
			fmt.Fprintf(stdout, "  %s: not present\n", projPath)
		case statErr != nil:
			report(false, "  %s: %v", projPath, statErr)
		case !dirTrusted(wd):
			fmt.Fprintf(stdout, "  %s: present but not read — %s is not a trusted project\n", projPath, wd)
			fmt.Fprintln(stdout, "  (a run asks once; trust it there, then re-run this check)")
		default:
			if _, err := config.LoadProject(wd); err != nil {
				report(false, "  %s: %v", projPath, err)
			} else {
				fmt.Fprintf(stdout, "  %s: ok\n", projPath)
			}
		}
	}

	if issues > 0 {
		fmt.Fprintf(stderr, "moca config check: %d issue(s)\n", issues)
		return exitUsage
	}
	fmt.Fprintln(stdout, "no issues")
	return exitOK
}

// dirTrusted reports whether the workdir is a trusted project, using the same
// store a run consults. A store that cannot be read is not trust.
func dirTrusted(dir string) bool {
	ts, err := permissions.LoadTrust(filepath.Join(config.DataDir(), "trust.json"))
	if err != nil || ts == nil {
		return false
	}
	v, known := ts.Lookup(dir)
	return known && v
}

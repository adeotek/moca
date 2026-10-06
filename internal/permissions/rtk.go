package permissions

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// rtk subcommand classes, captured from `rtk --help` and `rtk <sub> --help`
// (rtk 0.51.0, 2026-10-06, linuxbrew install), plus execution probes:
// `rtk test echo x` and `rtk test -- echo x` both run the command directly;
// `rtk err`, `rtk summary` and `rtk run` take the command as the first
// positional word; `rtk run -c` and `--shell` take a shell command string;
// `rtk test` (bare) errors with "command is required"; `rtk find … -exec …`
// forwards execution. Re-check when bumping the rtk version the built-in
// skill documents.
//
// The analyser's contract (§7/§10): `rtk <cmd>` is unwrapped like env/time —
// the wrapped command passes the same analysis — and rtk itself must be
// allowlisted. The two maps below are the special classes; for every other
// subcommand the sub word itself is the wrapped command name (`rtk git …` →
// git, `rtk docker …` → docker, `rtk tsc …` → tsc), including unknown words —
// a typo or a subcommand newer than this table fails closed as a command
// that must be allowlisted.

var (
	// rtkSelf: rtk runs no user-named program — either its own machinery or a
	// fixed read-only utility used with flags and paths only (the same trust
	// the analyser gives those utilities when run directly). Allowed as
	// plain `rtk`.
	rtkSelf = map[string]bool{
		// rtk's own machinery (stats, config, hooks, history analysis).
		"gain": true, "init": true, "config": true, "help": true, "telemetry": true,
		"trust": true, "untrust": true, "verify": true, "learn": true, "discover": true,
		"session": true, "cc-economics": true, "hook": true, "hook-audit": true,
		"recall": true, "rewrite": true, "pipe": true,
		// fixed read-only utilities — flags and paths only.
		"read": true, "ls": true, "tree": true, "find": true, "grep": true, "rg": true,
		"ast-grep": true, "json": true, "deps": true, "diff": true, "log": true,
		"wc": true, "env": true, "smart": true,
	}

	// rtkRun: the first positional word (or the word after `--`) is a command
	// to execute — `rtk test go test ./...`, `rtk err npm …`, `rtk summary …`,
	// `rtk run ./tool …`, `rtk proxy curl …`.
	rtkRun = map[string]bool{"test": true, "err": true, "summary": true, "proxy": true, "run": true}
)

// rtkTarget consumes the words after `rtk` at args[i] and returns the index
// of the word rtk will run. rtk's own flags are value-less and only accepted
// before the subcommand (-v/--verbose, --ultra-compact, --skip-env,
// -h/--help, -V/--version), so leading flag words are skipped. self=true
// means rtk runs nothing user-named. Fails closed: a non-literal word, a
// shell command string, or an unknown option refuses the whole command.
func rtkTarget(args []*syntax.Word, i int) (next int, self bool, err error) {
	sub, j, err := rtkSub(args, i)
	if err != nil {
		return 0, false, err
	}
	if j < 0 {
		return 0, true, nil
	}
	switch {
	case rtkSelf[sub]:
		return 0, true, nil
	case rtkRun[sub]:
		return rtkRunTarget(args, j, sub)
	}
	return j, false, nil // the sub word is itself the wrapped command name
}

// rtkSub returns rtk's subcommand word (the first non-flag word after rtk
// and its leading flags), or -1 when there is none.
func rtkSub(args []*syntax.Word, i int) (sub string, j int, err error) {
	for j = i + 1; j < len(args); j++ {
		w, ok := wordLit(args[j])
		if !ok {
			return "", 0, errNonLiteral
		}
		if !strings.HasPrefix(w, "-") {
			return w, j, nil
		}
	}
	return "", -1, nil
}

// rtkRunTarget finds the command word after a runner subcommand (`test`,
// `err`, `summary`, `proxy`, `run`). rtk's value-less flags are skipped, `--`
// ends option parsing, and a command-string option (`-c`/`--command` on run,
// `--shell`) or an unknown option refuses the whole command — the analyser
// cannot see inside a shell string, and guessing at an unknown option could
// misplace the command word. self=true means nothing was left to run (rtk
// itself errors out, e.g. "command is required").
func rtkRunTarget(args []*syntax.Word, i int, sub string) (int, bool, error) {
	for j := i + 1; j < len(args); {
		w, ok := wordLit(args[j])
		if !ok {
			return 0, false, errNonLiteral
		}
		switch {
		case w == "--":
			if j+1 < len(args) {
				return j + 1, false, nil
			}
			return 0, true, nil
		case w == "--ultra-compact" || w == "--skip-env" || w == "-h" || w == "--help":
			j++
		case rtkShellString(sub, w):
			return 0, false, fmt.Errorf("cannot analyse `rtk %s %s`: it runs a shell command string; run the command without the option", sub, w)
		case strings.HasPrefix(w, "-"):
			return 0, false, fmt.Errorf("cannot analyse `rtk %s` option %q; run the command without the option", sub, w)
		default:
			return j, false, nil
		}
	}
	return 0, true, nil
}

// rtkShellString reports whether w is an option that takes a shell command
// string instead of argv: `-c`/`--command` on `rtk run`, `--shell` on the
// runners that have it.
func rtkShellString(sub, w string) bool {
	if sub != "proxy" && (w == "--shell" || strings.HasPrefix(w, "--shell=")) {
		return true
	}
	if sub != "run" {
		return false
	}
	return strings.HasPrefix(w, "-c") || w == "--command" || strings.HasPrefix(w, "--command=")
}

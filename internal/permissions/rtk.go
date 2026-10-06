package permissions

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

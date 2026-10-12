package permissions

import (
	"path/filepath"
	"strings"
)

// Auto mode (task 5): a deterministic risk table decides what runs without
// asking. It runs after hard-deny/refused and after the layered rules — an
// explicit rule always decides first. Anything the table cannot judge as
// clearly read-only, clearly confined or clearly dangerous is "unsure" and
// goes to the classifier. The table never widens beyond its rows: a
// path-qualified or case-variant head, a non-literal word, or a tool
// missing from the table is never allowed.

// autoVerdict is the table's answer for one command. The zero value is
// autoUnsure, so a missed branch fails toward the classifier.
type autoVerdict int

const (
	autoUnsure autoVerdict = iota
	autoAllow
	autoAsk // every time, never learnable
)

// and combines two verdicts: ask beats unsure beats allow.
func (a autoVerdict) and(b autoVerdict) autoVerdict {
	switch {
	case a == autoAsk || b == autoAsk:
		return autoAsk
	case a == autoUnsure || b == autoUnsure:
		return autoUnsure
	}
	return autoAllow
}

// pathClass is where a literal path operand lands.
type pathClass int

const (
	pathOut  pathClass = iota // outside the confinement, an ancestor of the workdir, unresolvable, or relative after a cd
	pathOK                    // strictly inside the workdir or a writable temp root
	pathRoot                  // exactly the workdir or a writable root (never an ancestor: that is pathOut)
	pathGit                   // any path with a .git component, before or after resolution
)

// pathFunc classifies a literal path for a read (write=false) or a write.
type pathFunc func(p string, write bool) pathClass

// readOK: a read may touch anything confined, .git and the roots included.
func readOK(c pathClass) bool { return c != pathOut }

// alwaysAskNames: remote access, process/system control and system package
// managers. Every invocation asks.
var alwaysAskNames = map[string]bool{
	"ssh": true, "scp": true, "rsync": true, "sftp": true, "nc": true, "ncat": true, "socat": true,
	"kill": true, "pkill": true, "killall": true, "systemctl": true, "crontab": true,
	"brew": true, "dnf": true, "yum": true,
	// additions beyond the brief's table (stricter only: they never allow)
	"netcat": true, "telnet": true, "ftp": true,
	"shutdown": true, "reboot": true, "halt": true, "poweroff": true, "service": true, "launchctl": true,
	"pacman": true, "zypper": true, "snap": true, "flatpak": true,
	// fetch-and-run: download a package and execute it
	"npx": true, "bunx": true, "uvx": true,
}

// readOnlyCmds: tools that only read (their writing/executing options are
// handled in readOnlyVerdict).
var readOnlyCmds = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true, "wc": true, "grep": true, "rg": true,
	"sort": true, "uniq": true, "diff": true, "which": true, "jq": true, "stat": true, "file": true,
	"tree": true, "du": true, "df": true, "basename": true, "dirname": true, "realpath": true,
	"date": true, "find": true,
}

// confinedCmds: mutators allowed when every operand is literal and confined.
// A guarded operand that is the workdir or a writable root itself asks
// every time; an operand with a .git component asks for all of them.
var confinedCmds = map[string]rootGuard{
	"rm": guardAll, "rmdir": guardAll, "chmod": guardAll, "mv": guardSources,
	"cp": guardNone, "ln": guardNone, "mkdir": guardNone, "touch": guardNone,
}

type rootGuard int

const (
	guardNone rootGuard = iota
	guardAll
	guardSources // every operand but the destination (mv)
)

// autoClassify judges one command (ws[0] is the command name) against the
// risk table. conf classifies literal path operands; it already accounts for
// a cd earlier in the command (relative paths are then pathOut).
func autoClassify(ws []Word, conf pathFunc) autoVerdict {
	head := ws[0].Text
	lower := strings.ToLower(filepath.Base(head))
	v := toolVerdict(ws, lower, conf)
	if v == autoAllow && (head != lower || anyNonLiteral(ws[1:])) {
		// Only the bare, exact-case name is the table's tool: `./ls` is
		// whatever the agent wrote there, and a non-literal word has no
		// knowable value.
		v = autoUnsure
	}
	return v
}

func anyNonLiteral(ws []Word) bool {
	for _, w := range ws {
		if !w.Literal {
			return true
		}
	}
	return false
}

func toolVerdict(ws []Word, lower string, conf pathFunc) autoVerdict {
	switch {
	case isInterpreter(lower):
		return interpVerdict(ws, lower, conf)
	case alwaysAskNames[lower] || strings.HasPrefix(lower, "apt"):
		return autoAsk
	case lower == "pipx" && askIfAnyWord(ws, "run") == autoAsk:
		return autoAsk // fetch-and-run
	case strings.HasPrefix(lower, "pip"):
		return pipVerdict(ws, 1, conf)
	}
	switch lower {
	case "git":
		return gitVerdict(ws, conf)
	case "curl":
		return curlVerdict(ws, conf)
	case "wget":
		return wgetVerdict(ws, conf)
	case "sed":
		return sedVerdict(ws, conf)
	case "tar":
		return tarVerdict(ws, conf)
	case "unzip":
		return unzipVerdict(ws, conf)
	case "go":
		return goVerdict(ws, conf)
	case "make":
		for _, f := range optValues(ws, 1, "f", []string{"--file", "--makefile"}) {
			if !f.Literal || f.Text == "-" {
				return autoAsk // a makefile read from stdin is code from a pipe
			}
		}
		if hasOpt(ws, 1, "E", "--eval") {
			return autoUnsure
		}
		return readCheck(ws, 1, conf)
	case "pytest", "py.test":
		return buildWithWrites(ws, 1, conf, "", []string{"--basetemp", "--junitxml", "--junit-xml", "--result-log", "--report-log"})
	case "cargo":
		return cargoVerdict(ws, conf)
	case "mise":
		if len(ws) > 1 && ws[1].Text == "run" {
			return readCheck(ws, 2, conf)
		}
		return autoUnsure
	case "npm", "yarn", "pnpm":
		return npmVerdict(ws, lower, conf)
	case "docker", "podman":
		return askIfAnyWord(ws, "run", "push")
	case "kubectl":
		return askIfAnyWord(ws, "apply", "delete")
	case "terraform", "tofu":
		return askIfAnyWord(ws, "apply", "destroy")
	case "rtk":
		return rtkVerdict(ws, conf)
	case "tee":
		// Addition beyond the brief's table: the Unix walk already refuses
		// a tee target outside the confinement (as a redirect), Windows has
		// no such walk — so the table places targets itself; .git asks.
		_, args := splitArgs(ws, 1)
		for _, a := range args {
			if !a.Literal {
				return autoAsk // a value the table cannot place asks (on Unix the walk refuses it)
			}
			if c := conf(a.Text, true); c == pathGit || c == pathOut {
				return autoAsk
			}
		}
		return autoAllow
	case "chmod":
		if shortFlag(ws, 1, 'R') || hasOpt(ws, 1, "", "--recursive") {
			return autoAsk
		}
	}
	if g, ok := confinedCmds[lower]; ok {
		if _, ok := specMutators[lower]; !ok {
			return mutatorVerdict(ws, 1, conf, g, false) // positional-only (rm, rmdir, chmod)
		}
		v, p, paths := specMutatorVerdict(ws, lower, conf, g)
		recursive := p.has("-r", "-R", "-a", "--recursive", "--archive")
		if v == autoAllow && (lower == "mv" || (lower == "cp" && recursive)) && anyRoot(paths, conf) {
			// A tree merged into the workdir root may carry a .git (`cp -r
			// evil/. .`): the classifier judges it.
			return autoUnsure
		}
		return v
	}
	if readOnlyCmds[lower] {
		return readOnlyVerdict(ws, lower, conf)
	}
	return autoUnsure
}

// askIfAnyWord asks when any literal word is one of names (the risky
// subcommands of tools with no allow form): over-matching only turns an
// unsure into an ask.
func askIfAnyWord(ws []Word, names ...string) autoVerdict {
	for _, w := range ws[1:] {
		for _, n := range names {
			if w.Text == n {
				return autoAsk
			}
		}
	}
	return autoUnsure
}

// readOnlyVerdict: the read-only tools, minus their writing or executing
// options.
func readOnlyVerdict(ws []Word, lower string, conf pathFunc) autoVerdict {
	v := autoAllow
	switch lower {
	case "find":
		for _, w := range ws[1:] {
			switch t := w.Text; {
			case t == "-exec" || t == "-execdir" || t == "-ok" || t == "-okdir" || t == "-delete" ||
				t == "-fls" || strings.HasPrefix(t, "-fprint"):
				return autoAsk
			}
		}
	case "rg":
		// --pre and --hostname-bin run a program; --pre-glob only filters.
		if hasOpt(ws, 1, "", "--pre", "--hostname-bin") {
			return autoAsk
		}
	case "sort":
		if hasOpt(ws, 1, "", "--compress-program") {
			return autoAsk
		}
		v = writeTargets(optValues(ws, 1, "o", []string{"--output"}), conf, false).
			and(writeTargets(optValues(ws, 1, "T", []string{"--temporary-directory"}), conf, true))
	case "tree":
		if shortFlag(ws, 1, 'R') {
			return autoUnsure
		}
		v = writeTargets(optValues(ws, 1, "o", nil), conf, false)
	case "uniq":
		_, args := splitArgs(ws, 1)
		if len(args) > 1 {
			v = writeTargets(args[1:], conf, false)
		}
	case "date":
		if shortFlag(ws, 1, 's') || hasOpt(ws, 1, "", "--set") {
			return autoAsk
		}
	case "file":
		if shortFlag(ws, 1, 'C') || hasOpt(ws, 1, "", "--compile") {
			return autoUnsure
		}
	}
	return v.and(readCheck(ws, 1, conf))
}

// rtkVerdict: rtk's own read-only subcommands get the read check on their
// operands; for the runner and tool forms the wrapped command is classified
// on its own by unwrap, so rtk itself adds nothing (addition beyond the
// brief's table, see the task report).
func rtkVerdict(ws []Word, conf pathFunc) autoVerdict {
	i := 1
	for i < len(ws) && strings.HasPrefix(ws[i].Text, "-") {
		i++
	}
	if i < len(ws) && rtkSelf[strings.ToLower(ws[i].Text)] {
		return readCheck(ws, i+1, conf)
	}
	return autoAllow
}

// anyRoot reports whether a literal path is the workdir or a writable root
// itself.
func anyRoot(paths []Word, conf pathFunc) bool {
	for _, a := range paths {
		if a.Literal && conf(a.Text, true) == pathRoot {
			return true
		}
	}
	return false
}

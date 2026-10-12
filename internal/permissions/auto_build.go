package permissions

import (
	"slices"
	"strings"
)

// Auto risk table: project build/test tools, package managers and
// interpreters. Every parser fails toward unsure (cannot place a word) or
// ask (a risky form is present); none of them can turn an unknown form into
// an allow.

var (
	goWriteFlags = map[string]bool{"o": true, "coverprofile": true, "cpuprofile": true, "memprofile": true,
		"blockprofile": true, "mutexprofile": true, "trace": true, "outputdir": true}
	goExecFlags = map[string]bool{"exec": true, "toolexec": true, "vettool": true, "overlay": true}
)

func goVerdict(ws []Word, conf pathFunc) autoVerdict {
	if len(ws) < 2 {
		return autoUnsure
	}
	from := 2
	switch ws[1].Text {
	case "build", "test", "vet", "fmt", "list", "version":
	case "env":
		if hasOpt(ws, 2, "wu") {
			return autoAsk // persistent go env writes
		}
	case "mod":
		if len(ws) < 3 || (ws[2].Text != "tidy" && ws[2].Text != "download") {
			return autoUnsure
		}
		from = 3
	default:
		return autoUnsure
	}
	var writes []Word
	for i := from; i < len(ws); i++ {
		t := ws[i].Text
		if !ws[i].Literal || !strings.HasPrefix(t, "-") || t == "-" {
			continue
		}
		if t == "--" {
			break
		}
		name, val, eq := strings.Cut(strings.TrimLeft(t, "-"), "=")
		name = strings.TrimPrefix(name, "test.") // -test.coverprofile after -args
		switch {
		case goExecFlags[name]:
			return autoUnsure // runs a program the table cannot see
		case goWriteFlags[name] && eq:
			writes = append(writes, Word{Text: val, Literal: true})
		case goWriteFlags[name] && i+1 < len(ws):
			i++
			writes = append(writes, ws[i])
		}
	}
	v := writeTargets(writes, conf, true)
	if ws[1].Text == "fmt" { // rewrites the named packages' files
		_, args := splitArgs(ws, from)
		v = v.and(mutatorArgs(args, conf, guardNone, false))
	}
	return v.and(readCheck(ws, from, conf))
}

func cargoVerdict(ws []Word, conf pathFunc) autoVerdict {
	if len(ws) < 2 {
		return autoUnsure
	}
	switch ws[1].Text {
	case "publish", "yank", "owner", "login", "logout":
		return autoAsk
	case "build", "test", "check":
		if shortFlag(ws, 2, 'Z') || hasOpt(ws, 2, "", "--config") {
			return autoUnsure
		}
		return buildWithWrites(ws, 2, conf, "", []string{"--target-dir", "--artifact-dir", "--out-dir"})
	}
	return autoUnsure
}

// npmVerdict: publish and global installs ask (for yarn/pnpm too, an
// addition); `npm test` and `npm run …` are project scripts.
func npmVerdict(ws []Word, lower string, conf pathFunc) autoVerdict {
	for i, w := range ws[1:] {
		switch w.Text {
		case "publish", "unpublish", "-g", "--global", "--location=global", "dlx":
			return autoAsk
		case "--location":
			if i+2 < len(ws) && ws[i+2].Text == "global" {
				return autoAsk
			}
		}
	}
	if lower == "yarn" && len(ws) > 1 && ws[1].Text == "global" {
		return autoAsk
	}
	if lower != "npm" || len(ws) < 2 {
		return autoUnsure
	}
	switch ws[1].Text {
	case "exec", "x":
		return autoAsk // fetch-and-run
	case "test", "t", "tst", "run", "run-script":
		// npm options (before --) can swap the script shell, inject node
		// options or load another npmrc: only a few are known harmless.
		opts, _ := splitArgs(ws, 2)
		for _, o := range opts {
			if !slices.Contains(npmSafeOpts, o.Text) {
				return autoUnsure
			}
		}
		return readCheck(ws, 2, conf)
	}
	return autoUnsure
}

var npmSafeOpts = []string{"--silent", "-s", "--quiet", "-q", "--if-present", "--loglevel=silent", "--no-color"}

// pipVerdict: `pip install` asks unless --target names a confined
// directory (then it is unsure, like npm install).
func pipVerdict(ws []Word, from int, conf pathFunc) autoVerdict {
	install := false
	for _, w := range ws[from:] {
		if w.Text == "install" {
			install = true
		}
	}
	if !install {
		return autoUnsure
	}
	ts := optValues(ws, from, "t", []string{"--target"})
	if len(ts) > 0 && writeTargets(ts, conf, true) == autoAllow &&
		!hasOpt(ws, from, "", "--prefix", "--root", "--user") {
		return autoUnsure
	}
	return autoAsk
}

// Interpreters (ruling 2).

type interpSpec struct {
	code      string   // short letters running inline code, stdin or a REPL → ask
	script    string   // short letters naming a script or module to run → unsure
	value     string   // short letters taking a value (rest of cluster or next word)
	attached  string   // short letters taking the rest of the cluster only
	codeLong  []string // long options running inline code → ask
	scriptLng []string // long options running project files → unsure
	valueLong []string // long options taking the next word
	plus      bool     // +o style options (shells)
}

var (
	shellSpec = interpSpec{code: "cis", value: "oO", valueLong: []string{"--rcfile", "--init-file"}, plus: true}
	interps   = map[string]interpSpec{
		"sh": shellSpec, "bash": shellSpec, "zsh": shellSpec, "dash": shellSpec, "ksh": shellSpec,
		"fish": {code: "ciC", value: "dDo", codeLong: []string{"--command", "--init-command"}},
		"node": {code: "epi", value: "rC", codeLong: []string{"--eval", "--print", "--interactive"},
			scriptLng: []string{"--test", "--run"},
			valueLong: []string{"--require", "--import", "--loader", "--experimental-loader", "--input-type", "--conditions", "--title"}},
		"bun":  {code: "ep", value: "r", codeLong: []string{"--eval", "--print"}, valueLong: []string{"--preload", "--cwd", "--config", "--env-file"}},
		"perl": {code: "eE", value: "IMm", attached: "ix0lCdDV"},
		"ruby": {code: "e", value: "IrCE", attached: "ixF0KTW", valueLong: []string{"--encoding", "--external-encoding", "--internal-encoding", "--enable", "--disable", "--dump"}},
		"php":  {code: "rRBEFa", script: "fS", value: "cdzt"},
	}
	pythonSpec = interpSpec{code: "ci", script: "m", value: "WXQ", valueLong: []string{"--check-hash-based-pycs"}}
	luaSpec    = interpSpec{code: "ei", value: "l"}
)

func isInterpreter(l string) bool {
	_, ok := interps[l]
	return ok || l == "deno" || strings.HasPrefix(l, "python") || strings.HasPrefix(l, "lua")
}

// interpVerdict: inline code, stdin or a REPL asks every time; a script or
// module operand is unsure (the classifier judges the script).
func interpVerdict(ws []Word, lower string, conf pathFunc) autoVerdict {
	if lower == "deno" {
		_, args := splitArgs(ws, 1)
		if len(args) == 0 || args[0].Text == "eval" || args[0].Text == "repl" {
			return autoAsk
		}
		return autoUnsure
	}
	spec, ok := interps[lower]
	switch {
	case ok:
	case strings.HasPrefix(lower, "python"):
		spec = pythonSpec
	default:
		spec = luaSpec
	}
	for i := 1; i < len(ws); i++ {
		w := ws[i]
		t := w.Text
		if !w.Literal {
			return autoUnsure
		}
		switch {
		case t == "-":
			return autoAsk
		case t == "--":
			if i+1 < len(ws) {
				return autoUnsure
			}
			return autoAsk
		case strings.HasPrefix(t, "--"):
			name, _, eq := strings.Cut(t, "=")
			switch {
			case slices.Contains(spec.codeLong, name):
				return autoAsk
			case slices.Contains(spec.scriptLng, name):
				return autoUnsure
			case slices.Contains(spec.valueLong, name) && !eq:
				i++
			}
		case len(t) > 1 && (t[0] == '-' || (spec.plus && t[0] == '+')):
			for k := 1; k < len(t); k++ {
				ch := t[k]
				switch {
				case strings.IndexByte(spec.code, ch) >= 0:
					return autoAsk
				case strings.IndexByte(spec.script, ch) >= 0:
					if spec.script == "m" { // python -m
						mod, next := t[k+1:], i+1
						if mod == "" && i+1 < len(ws) {
							mod, next = ws[i+1].Text, i+2
						}
						if mod == "pip" {
							return pipVerdict(ws, next, conf)
						}
					}
					return autoUnsure
				case strings.IndexByte(spec.attached, ch) >= 0:
				case strings.IndexByte(spec.value, ch) >= 0:
					if k == len(t)-1 {
						i++
					}
				default:
					continue
				}
				break
			}
		default:
			return autoUnsure // a script file operand
		}
	}
	return autoAsk // no script: reads stdin or opens a REPL
}

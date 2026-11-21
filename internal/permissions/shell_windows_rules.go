package permissions

import (
	"fmt"
	"regexp"
	"strings"
)

// winSplit ends one statement and starts the next. A newline belongs in the
// class: PowerShell treats CR, LF and CRLF as statement separators, so leaving
// them out made every line after the first an *argument* of the first — a
// block whose head was a builtin or an allowlisted name was cleared whole.
var winSplit = regexp.MustCompile("&&|\\|\\||[;|\r\n]")

// winBlockComment strips a <# … #> span, PowerShell's block comment.
var winBlockComment = regexp.MustCompile(`(?s)<#.*?#>`)

// winCommentFree drops block comments and comment-only lines before the
// command is split, keeping the line boundary (a comment ends where the
// statement would have). A comment is not a statement and must not become an
// approval prompt for a command name like "#". Only WHOLE-line comments are
// stripped: an inline `#` is left alone, because the word model cannot tell it
// from a `#` inside a quoted string, and truncating a string can only hide an
// operand — an unterminated quote reads as non-literal, which fails closed.
func winCommentFree(command string) string {
	if !strings.ContainsRune(command, '#') {
		return command
	}
	command = winBlockComment.ReplaceAllString(command, "")
	var b strings.Builder
	for _, line := range strings.Split(command, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			b.WriteByte('\n')
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// winBlockOnly reports a segment that is nothing but block delimiters. Under
// the newline split every statement inside a block is its own segment, so a
// lone `{` or `}` runs nothing and must not prompt under a name like "}".
func winBlockOnly(fields []string) bool {
	for _, f := range fields {
		switch strings.Trim(f, `"'`) {
		case "{", "}", "(", ")":
		default:
			return false
		}
	}
	return true
}

var winAsk = map[string]bool{"remove-item": true, "rm": true, "ri": true, "del": true, "erase": true, "rd": true, "rmdir": true}

// Auto mode on Windows (the Unix risk table on best-effort words).
var (
	// winAutoNonLit: characters that make a PowerShell argument's value
	// unknowable for the table — wildcards, arrays, redirects, drive /
	// provider / stream / -Param:value colons, ~ (home and 8.3 short
	// names), and the typographic quotes and dashes PowerShell accepts.
	winAutoNonLit = "*?[],:<>&~!‘’‚‛“”„–—―"
	// winAlwaysAsk: code runners and system control with no Unix spelling
	// in the table — every invocation asks. `&` and `.` are PowerShell's call
	// and dot-source operators: `& $cmd` runs whatever the variable holds.
	winAlwaysAsk = map[string]bool{
		"&": true, ".": true,
		"iex": true, "invoke-expression": true, "invoke-command": true, "icm": true,
		"start-process": true, "saps": true, "start": true, "invoke-item": true, "ii": true,
		"cmd": true, "powershell": true, "pwsh": true, "wscript": true, "cscript": true, "mshta": true,
		"rundll32": true, "regsvr32": true, "reg": true, "sc": true, "net": true, "schtasks": true,
		"taskkill": true, "stop-process": true, "spps": true, "stop-computer": true, "restart-computer": true,
		"set-executionpolicy": true, "winget": true, "choco": true, "scoop": true,
		// PowerShell statement keywords and code runners with no Unix spelling:
		// the table cannot see what a block or a compiled type does, and the
		// classifier receives only text, so these ask every time.
		"for": true, "foreach": true, "while": true, "do": true, "switch": true,
		"start-job": true, "start-threadjob": true, "add-type": true,
		"register-objectevent": true, "register-engineevent": true, "register-scheduledjob": true,
		"install-module": true, "save-module": true, "import-module": true, "save-help": true,
		"get-credential": true, "new-service": true, "set-service": true, "stop-service": true,
		"new-itemproperty": true, "set-itemproperty": true, "remove-itemproperty": true,
		"rename-itemproperty": true, "copy-itemproperty": true, "move-itemproperty": true,
		// Destructive file content and machine state the table has no operand
		// model for: Clear-Content truncates a file and Set-Item writes any
		// provider (file, registry, env), so neither may reach the classifier.
		"clear-content": true, "clc": true, "clear-item": true, "cli": true,
		"set-item": true, "si": true,
		// Disk, boot, firewall and ACL tools. `format` and `diskpart` destroy
		// data irrecoverably and have no row; `format-volume` is hard-denied
		// already, the rest ask every time rather than being judged by a model.
		"format": true, "diskpart": true, "clear-disk": true, "initialize-disk": true,
		"bcdedit": true, "netsh": true, "wmic": true,
		"icacls": true, "cacls": true, "takeown": true,
		"unblock-file": true, "set-authenticodesignature": true,
		"invoke-webrequest": true, "iwr": true, "invoke-restmethod": true, "irm": true,
	}
	// winCd: commands after which the cwd is no longer the workdir.
	winCd = map[string]bool{"cd": true, "chdir": true, "sl": true, "set-location": true,
		"pushd": true, "push-location": true, "popd": true, "pop-location": true}
	// winDefines: commands that change how later names or values resolve,
	// tainting the rest of the command even when a rule allowed them.
	winDefines = map[string]bool{".": true, "&": true, "function": true, "filter": true,
		"set-alias": true, "sal": true, "new-alias": true, "nal": true, "set-variable": true, "sv": true,
		"new-variable": true, "nv": true, "set-item": true, "si": true, "new-item": true, "ni": true,
		"import-module": true, "ipmo": true, "set": true}
)

// winPathCheck narrows the Unix path check to what the best-effort Windows
// words can place: plain relative paths only. A rooted path (`\x`, `/x`),
// a drive or provider path (no ':' reaches here — it is non-literal), and a
// component with a trailing dot or space (Windows drops them: `.git.` is
// .git; a read of Go's `...` pattern excepted) are outside; separators are normalized for the shared guards.
func winPathCheck(inner pathFunc) pathFunc {
	return func(p string, write bool) pathClass {
		// Judge the value, not its quoted spelling: `'..\x'` is a traversal and
		// `'.moca/skills/x/SKILL.md'` is a skills write.
		p = strings.Trim(p, `"'`)
		if p == "" || strings.HasPrefix(p, `\`) || strings.HasPrefix(p, "/") || strings.ContainsAny(p, ":~") {
			return pathOut
		}
		for _, c := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
			if c == "." || c == ".." || (c == "..." && !write) { // `./...`: Go's package pattern, read only
				continue
			}
			if strings.TrimRight(c, ". ") != c {
				return pathOut
			}
		}
		return inner(strings.ReplaceAll(p, `\`, "/"), write)
	}
}

// checkWindows is best-effort (no PowerShell parser in v1, §7): the first
// token of every ;/|/&&/|| segment is checked, plus — for `rtk` — the
// wrapped command word on the same best-effort basis: no quoting model and
// no option tables, so a later `--shell <cmd-string>` shape is only
// approximated, never analysed (review F1/M4). Documented as weaker than
// Unix.
// winSkillWriters are the cmdlets whose literal argument names a file they
// write. The Windows parser is a word model by design (§7: no PowerShell
// parser, no quoting model), so the skills-dir test is textual on the
// normalised path rather than a filesystem resolution — the same class of
// check the Unix walk does, at the same (pattern-level) strength.
var winSkillWriters = map[string]bool{
	"set-content": true, "add-content": true, "out-file": true, "new-item": true,
	"set-item": true, "copy-item": true, "move-item": true, "rename-item": true,
	"tee-object": true,
}

// winSkillPath reports whether a normalised Windows path names the project's
// skills dir. Backslashes fold to slashes and case is ignored, as the
// filesystem does; a bare SKILL.md under a skills/ component counts too.
func winSkillPath(p string) bool {
	f := strings.ToLower(strings.ReplaceAll(strings.Trim(p, `"'`), `\`, "/"))
	if strings.Contains(f, ".moca/skills") {
		return true
	}
	return strings.HasSuffix(f, "skill.md") && strings.Contains(f, "skills/")
}

func (s *Shell) checkWindows(command string) ([]string, []string, []string, error) {
	// Auto mode asks PowerShell's own parser first: it knows the quoting, the
	// parameter binding and the redirections the word model below guesses at.
	// Any failure — no interpreter, a broken sidecar, a script it could not parse
	// — falls through to the word model, never to an allow.
	if s.auto {
		parse := s.winParse
		if parse == nil {
			parse = winParse
		}
		if parsed, err := parse(command); err == nil {
			return s.checkParsed(parsed)
		}
	}
	var v verdictAcc
	var st cmdState
	nonLit := "$`\"'(){}@%"
	if s.auto {
		nonLit += winAutoNonLit
	}
	classify := func(fields []string) error {
		ws := make([]Word, len(fields))
		for i, f := range fields {
			// A word that is one quoted string is literal (winQuoted): its quotes
			// stay in Text so a quoted `-Path` is not read as a parameter, while
			// winPathCheck and textForms see the value between them.
			ws[i] = Word{Text: f, Literal: i == 0 || !strings.ContainsAny(f, nonLit) || winQuoted(f)}
		}
		ws[0].Text = winName(fields[0])
		// Skill writes: the cmdlets that write a path never touch the
		// skills dir, same rule as the Unix walk (owner: the tool-side
		// guard, which re-validates on load).
		if winSkillWriters[ws[0].Text] {
			for _, w := range ws[1:] {
				if !w.Literal || strings.HasPrefix(strings.Trim(w.Text, `"'`), "-") {
					continue
				}
				if winSkillPath(w.Text) {
					return errSkillWrite
				}
			}
		}
		asked := len(v.need) + len(v.every) + len(v.unsure)
		if err := s.classify(ws, &v, st); err != nil {
			return err
		}
		// Auto mode carries state across segments like the Unix walk: a cd
		// makes every later relative path unplaceable, and anything the
		// table did not clear (it may define a function or an alias, set a
		// variable, dot-source a script) taints what follows.
		if winCd[ws[0].Text] {
			st.cwdUnknown, st.tainted = true, true
		}
		if len(v.need)+len(v.every)+len(v.unsure) > asked || winDefines[ws[0].Text] {
			st.tainted = true
		}
		return nil
	}
	for _, seg := range winSplit.Split(winCommentFree(command), -1) {
		fields := strings.Fields(seg)
		if len(fields) == 0 || winBlockOnly(fields) {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(fields[0]), ".exe")
		if s.auto && strings.ContainsAny(seg, "<>") {
			// No redirect model on Windows: a segment that redirects is
			// never cleared by the table (builtins included), and it
			// taints what follows.
			v.add(&v.unsure, winName(fields[0]))
			st.tainted = true
		}
		if err := classify(fields); err != nil {
			return nil, nil, nil, err
		}
		if name != "rtk" {
			continue
		}
		// rtk unwrap, best-effort: find the subcommand word (skipping rtk's
		// value-less flags), then mirror the Unix rule — for a runner the
		// first positional word is the wrapped command name; for the tool
		// class the sub word itself is; for rtkSelf nothing more is run.
		j := 1
		for j < len(fields) && strings.HasPrefix(fields[j], "-") {
			j++
		}
		if j >= len(fields) {
			continue
		}
		sub := winName(fields[j])
		switch {
		case rtkSelf[sub]:
		case rtkRun[sub]:
			j++ // runner flags first, then the `--` separator
			for j < len(fields) && strings.HasPrefix(fields[j], "-") && fields[j] != "--" {
				j++
			}
			if j < len(fields) && fields[j] == "--" {
				j++
				if j < len(fields) && strings.HasPrefix(fields[j], "-") {
					return nil, nil, nil, fmt.Errorf("cannot analyse `rtk %s -- %s`: the word after `--` is option-shaped; run the command without the option", sub, fields[j])
				}
			}
			if j < len(fields) {
				if err := classify(fields[j:]); err != nil {
					return nil, nil, nil, err
				}
			}
		default:
			if err := classify(fields[j:]); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	return v.need, v.every, v.unsure, nil
}

// checkParsed judges a script that PowerShell parsed for us, with the same table,
// rules and state machine as the word model: the difference is that every command
// arrives with its parameters and values separated, and every file redirection is
// listed rather than guessed at from a `<`/`>` in the text.
//
// A command the parser marked not static still goes through the table: an
// unknowable operand is left non-literal, so `Where-Object { $_.Name }` stays a
// filter while `Get-Content $env:X` cannot be cleared (the operand the read needs is
// exactly the one nobody can place). A scriptblock's own commands are separate
// CommandAsts, so `& $x` inside one is judged on its own and asks.
func (s *Shell) checkParsed(ps parsedScript) ([]string, []string, []string, error) {
	var v verdictAcc
	var st cmdState
	for _, c := range ps.Cmds {
		ws := psWords(c)
		if !ws[0].Literal {
			// `& $cmd`: neither the command nor its arguments are knowable.
			v.add(&v.every, ws[0].Text)
			st.tainted = true
			continue
		}
		if winSkillWriters[ws[0].Text] {
			for _, w := range ws[1:] {
				if !w.Literal || strings.HasPrefix(w.Text, "-") {
					continue
				}
				if winSkillPath(w.Text) {
					return nil, nil, nil, errSkillWrite
				}
			}
		}
		asked := len(v.need) + len(v.every) + len(v.unsure)
		if err := s.classify(ws, &v, st); err != nil {
			return nil, nil, nil, err
		}
		// Same state machine as the text path: a cd invalidates every relative
		// path after it, and anything that was not cleared — or that defines a
		// name — taints what follows.
		if winCd[ws[0].Text] {
			st.cwdUnknown, st.tainted = true, true
		}
		if len(v.need)+len(v.every)+len(v.unsure) > asked || winDefines[ws[0].Text] {
			st.tainted = true
		}
	}
	// Redirections are writes. They are confined exactly like the Unix ones
	// (checkTargetText: the jail, `..`, a cd, and the plan zone) because the
	// analyser can see them at all only since the parse.
	for _, w := range ps.Writes {
		target := strings.TrimSpace(strings.Trim(w.Target, `"'`))
		if target == "" {
			continue
		}
		if winSkillPath(target) {
			return nil, nil, nil, errSkillWrite
		}
		if err := s.checkTargetText(target, st.cwdUnknown); err != nil {
			return nil, nil, nil, err
		}
	}
	return v.need, v.every, v.unsure, nil
}

// psWords turns a parsed command into the word slice the table takes: the name, then
// each parameter and its value (or the value alone), with an unknowable element left
// non-literal. A quoted value is literal — the parse knows it is a string.
func psWords(c psCmd) []Word {
	name := winName(c.Name)
	ws := make([]Word, 0, len(c.Args)+1)
	ws = append(ws, Word{Text: name, Literal: psPlainName(name)})
	for _, a := range c.Args {
		if a.Param != "" {
			ws = append(ws, Word{Text: "-" + strings.ToLower(a.Param), Literal: true})
			if a.Value != nil {
				ws = append(ws, Word{Text: *a.Value, Literal: true})
			}
			continue
		}
		if a.Value != nil {
			ws = append(ws, Word{Text: *a.Value, Literal: true})
			continue
		}
		ws = append(ws, Word{Text: "", Literal: false})
	}
	return ws
}

// psPlainName reports a command name the parse resolved to a bare word: anything
// carrying an expansion is a name nobody can look up in the table.
func psPlainName(name string) bool {
	if name == "" {
		return false
	}
	return !strings.ContainsAny(name, "$`(){}@%")
}

// winName normalizes a best-effort token: lowercase, surrounding quotes
// trimmed, `.exe` stripped.
func winName(tok string) string {
	return strings.TrimSuffix(strings.Trim(strings.ToLower(tok), `'"`), ".exe")
}

// winQuoted reports a word PowerShell passes through unchanged: the whole word
// is one quoted string, so the quotes are its delimiters and its value is the
// text between them. SINGLE quotes never evaluate anything in PowerShell (the
// only escape is a doubled ”, which is rejected here as an interior quote), so
// `'$(rm x)'` and `'{"a":1}'` are data. DOUBLE quotes interpolate, so the word
// stays non-literal when it holds `$` or a backtick — the two things a
// double-quoted string resolves.
//
// Treating these as literal is what stops a quoted write target from dodging the
// path checks (`'..\x'`) and a quoted path from dodging the skills guard
// (`Set-Content '.moca/skills/x/SKILL.md'`), which the word model previously
// read as "unknowable" and handed to the classifier.
func winQuoted(f string) bool {
	if len(f) < 2 {
		return false
	}
	q := f[0]
	if q != '\'' && q != '"' || f[len(f)-1] != q {
		return false
	}
	inner := f[1 : len(f)-1]
	if strings.ContainsRune(inner, rune(q)) {
		return false
	}
	if q == '\'' {
		return true
	}
	return !strings.ContainsAny(inner, "$`")
}

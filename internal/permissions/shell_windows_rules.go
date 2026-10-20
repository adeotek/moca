package permissions

import (
	"fmt"
	"regexp"
	"strings"
)

var winSplit = regexp.MustCompile(`&&|\|\||[;|]`)

var winAsk = map[string]bool{"remove-item": true, "rm": true, "ri": true, "del": true, "erase": true, "rd": true, "rmdir": true}

// Auto mode on Windows (the Unix risk table on best-effort words).
var (
	// winAutoNonLit: characters that make a PowerShell argument's value
	// unknowable for the table — wildcards, arrays, redirects, drive /
	// provider / stream / -Param:value colons, ~ (home and 8.3 short
	// names), and the typographic quotes and dashes PowerShell accepts.
	winAutoNonLit = "*?[],:<>&~!‘’‚‛“”„–—―"
	// winAlwaysAsk: code runners and system control with no Unix spelling
	// in the table — every invocation asks.
	winAlwaysAsk = map[string]bool{
		"iex": true, "invoke-expression": true, "invoke-command": true, "icm": true,
		"start-process": true, "saps": true, "start": true, "invoke-item": true, "ii": true,
		"cmd": true, "powershell": true, "pwsh": true, "wscript": true, "cscript": true, "mshta": true,
		"rundll32": true, "regsvr32": true, "reg": true, "sc": true, "net": true, "schtasks": true,
		"taskkill": true, "stop-process": true, "spps": true, "stop-computer": true, "restart-computer": true,
		"set-executionpolicy": true, "winget": true, "choco": true, "scoop": true,
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
	var v verdictAcc
	var st cmdState
	nonLit := "$`\"'(){}@%"
	if s.auto {
		nonLit += winAutoNonLit
	}
	classify := func(fields []string) error {
		ws := make([]Word, len(fields))
		for i, f := range fields {
			ws[i] = Word{Text: f, Literal: i == 0 || !strings.ContainsAny(f, nonLit)}
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
	for _, seg := range winSplit.Split(command, -1) {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
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

// winName normalizes a best-effort token: lowercase, surrounding quotes
// trimmed, `.exe` stripped.
func winName(tok string) string {
	return strings.TrimSuffix(strings.Trim(strings.ToLower(tok), `'"`), ".exe")
}

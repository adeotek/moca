package permissions

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

var hardDeny = map[string]bool{"sudo": true, "su": true, "doas": true, "dd": true, "shred": true, "chown": true}

var builtinsOK = map[string]bool{"cd": true, "pwd": true, "echo": true, "printf": true, "test": true,
	"[": true, "true": true, "false": true, "exit": true}

var refused = map[string]bool{"eval": true, "source": true, ".": true, "exec": true}

var askEvery = map[string]bool{"rm": true}

// errNonLiteral refuses any command-name position whose runtime value cannot
// be known exactly (expansions, escapes, globs): fail closed, never guess.
var errNonLiteral = errors.New("non-literal command name refused (e.g. $CMD); write the command out")

// Shell analyses commands for the model-facing shell tool (§7). Not a
// sandbox: an allowlisted interpreter can do anything the user can.
type Shell struct {
	rules *Rules
	auto  bool // wired in a later task; false = default mode
	jail  *Jail
	goos  string
}

func NewShell(rules *Rules, jail *Jail, goos string) *Shell {
	if rules == nil {
		rules = NewRules()
	}
	return &Shell{rules: rules, jail: jail, goos: goos}
}

// RulesFromAllow builds a single "user" layer whose allow rules are the given
// command names (names ParseRule rejects are skipped).
func RulesFromAllow(names []string) *Rules {
	rs := RuleSet{Name: "user"}
	for _, n := range names {
		if r, err := ParseRule(n); err == nil {
			rs.Allow = append(rs.Allow, r)
		}
	}
	return NewRules(rs)
}

// Allow records an allow-always answer. Hard-deny and ask-every-time names
// are never learned: an allow rule would otherwise lift the ask-every class.
func (s *Shell) Allow(name string) {
	lower := strings.ToLower(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	if hardDeny[lower] || strings.HasPrefix(lower, "mkfs") || askEvery[lower] || winAsk[lower] {
		return
	}
	s.rules.Learn(name)
	if lower2 := strings.ToLower(name); lower2 != name {
		s.rules.Learn(lower2)
	}
}

type verdictAcc struct {
	need, every, unsure []string
}

func (v *verdictAcc) add(list *[]string, name string) {
	if !slices.Contains(*list, name) {
		*list = append(*list, name)
	}
}

// shortCmd renders at most the first three words of a command for an error
// message, so a heredoc is never dumped.
func shortCmd(ws []Word) string {
	var parts []string
	for i, w := range ws {
		if i == 3 {
			break
		}
		if w.Literal {
			parts = append(parts, w.Text)
		} else {
			parts = append(parts, "?")
		}
	}
	return strings.Join(parts, " ")
}

// classify judges one command (ws = its words from the classified index to
// the end of the call). Hard-deny and refused come first and no rule can
// override them; then the layered rules; then the built-in classes.
func (s *Shell) classify(ws []Word, v *verdictAcc) error {
	win := s.goos == "windows"
	head := ws[0].Text
	var base string
	if win {
		base = baseName(head)
	} else {
		base = filepath.Base(head)
	}
	// The safety classes match case-insensitively: on a case-insensitive
	// filesystem (macOS, Windows) `SUDO` and `RM` resolve to the real
	// binaries, and must not slip past hard-deny / ask-every-time.
	lower := strings.ToLower(base)
	switch {
	case hardDeny[lower] || strings.HasPrefix(lower, "mkfs") || (win && lower == "format-volume"):
		return fmt.Errorf("%s is never allowed (hard-deny)", base)
	case !win && refused[lower]:
		return fmt.Errorf("%s is refused: it runs code the analyser cannot see", base)
	}
	switch d, layer, src := s.rules.Decide(ws); d {
	case Deny:
		return fmt.Errorf("`%s` is denied by the permissions rule %q (%s config)", shortCmd(ws), src, layer)
	case Ask:
		v.add(&v.every, base)
		return nil
	case Allow:
		return nil
	}
	// A path-qualified head is its own allow-always identity: `./tool` is not
	// `tool`, so the prompt shows (and learns) the full head text.
	name := base
	if strings.ContainsAny(head, `/\`) {
		name = head
	}
	switch {
	case builtinsOK[base] || (win && builtinsOK[lower]):
	case askEvery[lower] || (win && winAsk[lower]):
		v.add(&v.every, base)
	default:
		v.add(&v.need, name)
	}
	return nil
}

// toWords converts call arguments to analyser words.
func toWords(args []*syntax.Word) []Word {
	ws := make([]Word, len(args))
	for i, a := range args {
		t, ok := wordLit(a)
		ws[i] = Word{Text: t, Literal: ok}
	}
	return ws
}

// literalSafe reports whether a word part's runtime value is exactly its
// source text: no backslash escapes (unquoted words decode them; inside
// double quotes they are escapes) and, unquoted only, no glob
// metacharacters or brace expansion (`{rm,x}`, `{1..3}`), which bash expands
// before the command runs.
func literalSafe(v string, unquoted bool) bool {
	if strings.Contains(v, "\\") {
		return false
	}
	if unquoted && strings.ContainsAny(v, "*?") {
		return false
	}
	if unquoted && strings.Contains(v, "[") && strings.Contains(v, "]") {
		return false
	}
	if unquoted && strings.Contains(v, "{") && strings.Contains(v, "}") &&
		(strings.Contains(v, ",") || strings.Contains(v, "..")) {
		return false
	}
	return true
}

// wordLit returns the literal value of a word made only of literal and
// quoted-literal parts. Words whose runtime value cannot be known exactly
// (expansions, escapes, globs, ANSI-C strings) report false so callers fail
// closed.
func wordLit(w *syntax.Word) (string, bool) {
	var sb, unq strings.Builder // unq: the unquoted text only, across all parts
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			if !literalSafe(x.Value, true) {
				return "", false
			}
			sb.WriteString(x.Value)
			unq.WriteString(x.Value)
		case *syntax.SglQuoted:
			if x.Dollar {
				return "", false // $'…' decodes escapes
			}
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			if x.Dollar {
				return "", false // $"…" locale quoting
			}
			for _, q := range x.Parts {
				l, ok := q.(*syntax.Lit)
				if !ok {
					return "", false
				}
				if !literalSafe(l.Value, false) {
					return "", false
				}
				sb.WriteString(l.Value)
			}
		default:
			return "", false
		}
	}
	// Brace and bracket patterns can span parts — `{"rm",x}` is the unquoted
	// text `{,x}` around a quoted word, and bash still expands it — so they
	// are judged on the word's whole unquoted text, not part by part.
	if !patternSafe(unq.String()) {
		return "", false
	}
	return sb.String(), true
}

// patternSafe reports whether the unquoted text of a word contains no brace
// expansion (`{a,b}`, `{1..3}`) or bracket glob (`[ab]`).
func patternSafe(u string) bool {
	if strings.Contains(u, "{") && strings.Contains(u, "}") &&
		(strings.Contains(u, ",") || strings.Contains(u, "..")) {
		return false
	}
	return !(strings.Contains(u, "[") && strings.Contains(u, "]"))
}

// optSpec describes a wrapper command's options so unwrap can find the
// wrapped command without guessing.
type optSpec struct {
	bare     map[string]bool // options that take no argument
	arg      map[string]bool // options whose argument is the next word
	prefixed []string        // long options accepting --name=value
	assigns  bool            // consume NAME=VALUE words (env)
	dashDash bool            // a literal -- ends the options
	numeric  bool            // accept a bare numeric flag, e.g. nice -10
}

var (
	envSpec = optSpec{
		bare: map[string]bool{"-i": true, "--ignore-environment": true, "-0": true, "--null": true,
			"-v": true, "--debug": true, "--help": true, "--version": true},
		arg:      map[string]bool{"-u": true, "--unset": true, "-C": true, "--chdir": true},
		prefixed: []string{"--unset", "--chdir"},
		assigns:  true,
		dashDash: true,
	}
	timeSpec = optSpec{
		bare: map[string]bool{"-p": true, "--portability": true, "-a": true, "--append": true,
			"-v": true, "--verbose": true, "--help": true, "--version": true},
		arg:      map[string]bool{"-f": true, "--format": true, "-o": true, "--output": true},
		prefixed: []string{"--format", "--output"},
	}
	niceSpec = optSpec{
		bare:     map[string]bool{"--help": true, "--version": true},
		arg:      map[string]bool{"-n": true, "--adjustment": true},
		prefixed: []string{"--adjustment"},
		numeric:  true,
	}
	timeoutSpec = optSpec{
		bare: map[string]bool{"-f": true, "--foreground": true, "-p": true, "--preserve-status": true,
			"-v": true, "--verbose": true, "--help": true, "--version": true},
		arg:      map[string]bool{"-k": true, "--kill-after": true, "-s": true, "--signal": true},
		prefixed: []string{"--kill-after", "--signal"},
	}
	nohupSpec = optSpec{bare: map[string]bool{"--help": true, "--version": true}}
)

// skipWrapperOpts consumes a wrapper's options and returns the index of the
// wrapped command name, or -1 when the wrapper has no wrapped command. It
// fails closed: an option it cannot model refuses the whole command.
func skipWrapperOpts(args []*syntax.Word, i int, spec optSpec, wrapper string) (int, error) {
	j := i + 1
	for j < len(args) {
		a, ok := wordLit(args[j])
		if !ok {
			return -1, fmt.Errorf("cannot analyse a non-literal %s argument; write the command out", wrapper)
		}
		if spec.dashDash && a == "--" {
			return j + 1, nil
		}
		if !strings.HasPrefix(a, "-") {
			if spec.assigns && strings.Contains(a, "=") {
				j++
				continue
			}
			return j, nil
		}
		if spec.numeric && numericFlag(a) {
			j++
			continue
		}
		if spec.bare[a] {
			j++
			continue
		}
		if spec.arg[a] {
			if j+1 >= len(args) {
				return -1, fmt.Errorf("cannot analyse `%s %s` without its argument", wrapper, a)
			}
			j += 2
			continue
		}
		matched := false
		for _, p := range spec.prefixed {
			if strings.HasPrefix(a, p+"=") {
				j++
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		return -1, fmt.Errorf("cannot analyse `%s` option %q; run the wrapped command without the wrapper", wrapper, a)
	}
	return -1, nil
}

// skipWrapper consumes one transparent wrapper at args[i] — env, time,
// nice, timeout, nohup, command — and returns the index of the wrapped
// command name (-1 when there is none, e.g. `env -i` or `command -v cmd`).
func skipWrapper(args []*syntax.Word, i int) (int, error) {
	name, _ := wordLit(args[i]) // callers switch on the same names, lowercased
	switch strings.ToLower(filepath.Base(name)) {
	case "env":
		return skipWrapperOpts(args, i, envSpec, "env")
	case "time":
		return skipWrapperOpts(args, i, timeSpec, "time")
	case "nice":
		return skipWrapperOpts(args, i, niceSpec, "nice")
	case "timeout":
		j, err := skipWrapperOpts(args, i, timeoutSpec, "timeout")
		if err != nil || j < 0 || j >= len(args) {
			return j, err
		}
		if _, ok := wordLit(args[j]); !ok {
			return -1, fmt.Errorf("cannot analyse a non-literal `timeout` duration")
		}
		return j + 1, nil // the duration word
	case "nohup":
		return skipWrapperOpts(args, i, nohupSpec, "nohup")
	case "command":
		return skipCommand(args, i)
	}
	return -1, nil // unreachable: callers switch on the same names
}

func numericFlag(a string) bool {
	if len(a) < 2 || a[0] != '-' {
		return false
	}
	for _, c := range a[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// skipCommand handles `command [-pVv] cmd`; -v/-V only looks a command up.
func skipCommand(args []*syntax.Word, i int) (int, error) {
	j := i + 1
	for j < len(args) {
		a, ok := wordLit(args[j])
		if !ok {
			return -1, fmt.Errorf("cannot analyse a non-literal `command` argument")
		}
		if a == "--" {
			return j + 1, nil
		}
		if !strings.HasPrefix(a, "-") {
			return j, nil
		}
		flags := strings.TrimPrefix(a, "-")
		if flags == "" || strings.Trim(flags, "pVv") != "" {
			return -1, fmt.Errorf("cannot analyse `command` option %q", a)
		}
		if strings.ContainsAny(flags, "vV") {
			return -1, nil // lookup only
		}
		j++
	}
	return -1, nil
}

// unwrap walks transparent wrappers and rtk, returning the indexes of every
// word that must be classified as a command name. Transparent wrappers (env,
// time, timeout, nice, nohup, command) are not classified themselves — they
// only relocate the command; rtk is classified AND unwrapped, so `rtk git …`
// classifies both `rtk` and `git`, and `rtk proxy rm x` reaches `rm`.
// Wrapper names match case-insensitively: a case variant (`RTK proxy …`)
// must not skip the analysis behind one allow-always click (review F2/M3).
func unwrap(args []*syntax.Word) ([]int, error) {
	var out []int
	i := 0
	for i < len(args) {
		name, ok := wordLit(args[i])
		if !ok {
			return append(out, i), nil // a non-literal name is refused by Check
		}
		switch strings.ToLower(filepath.Base(name)) {
		case "rtk":
			out = append(out, i)
			next, self, err := rtkTarget(args, i)
			if err != nil {
				return nil, err
			}
			if self {
				return out, nil
			}
			i = next
			continue
		case "env", "time", "nohup", "nice", "timeout", "command":
			next, err := skipWrapper(args, i)
			if err != nil {
				return nil, err
			}
			if next < 0 {
				return out, nil
			}
			i = next
			continue
		}
		return append(out, i), nil
	}
	return out, nil
}

func (s *Shell) checkTarget(w *syntax.Word, cwdUnknown bool) error {
	t, ok := wordLit(w)
	if !ok {
		return fmt.Errorf("non-literal redirect/tee target refused")
	}
	if t == "/dev/null" || t == "/dev/stdout" || t == "/dev/stderr" {
		return nil
	}
	if cwdUnknown && !filepath.IsAbs(t) && t != "~" && !strings.HasPrefix(t, "~/") {
		return fmt.Errorf("relative redirect target %q after a cd/pushd/popd cannot be resolved; use an absolute path or run the cd separately", t)
	}
	if _, err := s.jail.Resolve(t, true); err != nil {
		return err
	}
	return nil
}

func (s *Shell) Check(command string) (need, every, unsure []string, err error) {
	if s.goos == "windows" {
		return s.checkWindows(command)
	}
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot parse command (refused, no best-effort guessing): %v", err)
	}
	var v verdictAcc
	var cwdUnknown bool
	var firstErr error
	fail := func(e error) {
		if firstErr == nil {
			firstErr = e
		}
	}
	syntax.Walk(f, func(n syntax.Node) bool {
		if firstErr != nil {
			return false
		}
		switch x := n.(type) {
		case *syntax.Stmt:
			for _, r := range x.Redirs {
				switch r.Op {
				case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll, syntax.ClbOut, syntax.RdrInOut:
					if err := s.checkTarget(r.Word, cwdUnknown); err != nil {
						fail(err)
					}
				case syntax.DplOut:
					if t, ok := wordLit(r.Word); !ok || (t != "-" && strings.Trim(t, "0123456789") != "") {
						if err := s.checkTarget(r.Word, cwdUnknown); err != nil {
							fail(err)
						}
					}
				}
			}
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				return true // pure assignment
			}
			idxs, uerr := unwrap(x.Args)
			if uerr != nil {
				fail(uerr)
				return false
			}
			ws := toWords(x.Args)
			for _, idx := range idxs {
				if !ws[idx].Literal {
					fail(errNonLiteral)
					return false
				}
				if err := s.classify(ws[idx:], &v); err != nil {
					fail(err)
					return false
				}
				switch filepath.Base(ws[idx].Text) {
				case "cd", "pushd", "popd":
					// Relative redirect targets are resolved against the jail
					// root, which is only correct while the cwd is unchanged;
					// after a cd only absolute targets can be checked.
					cwdUnknown = true
				case "tee":
					for _, a := range x.Args[idx+1:] {
						if l, ok := wordLit(a); ok && strings.HasPrefix(l, "-") {
							continue
						}
						if err := s.checkTarget(a, cwdUnknown); err != nil {
							fail(err)
						}
					}
				}
			}
		}
		return true
	})
	if firstErr != nil {
		return nil, nil, nil, firstErr
	}
	return v.need, v.every, v.unsure, nil
}

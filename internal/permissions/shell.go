package permissions

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

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
	auto  bool // auto mode: the risk table (auto.go) replaces "needs approval"
	jail  *Jail
	goos  string
}

// AutoView returns the auto-mode view of s: the same Rules (an allow-always
// learned in either view applies in both), the jail j (normally
// s.jail.WithWritable(TempRoots()...)), and the risk table instead of the
// default "needs approval" class.
func (s *Shell) AutoView(j *Jail) *Shell {
	return &Shell{rules: s.rules, auto: true, jail: j, goos: s.goos}
}

func NewShell(rules *Rules, jail *Jail, goos string) *Shell {
	if rules == nil {
		rules = NewRules()
	}
	return &Shell{rules: rules, jail: jail, goos: goos}
}

// learnable reports whether name may enter an allow list as a plain command
// name: one literal word (what Rules.Learn accepts) that is not hard-deny,
// mkfs*, or an ask-every-time class (an allow rule would lift that ask).
// winToo adds the Windows ask names.
func learnable(name string, winToo bool) bool {
	if name == "" || strings.Contains(name, "*") || strings.IndexFunc(name, unicode.IsSpace) >= 0 {
		return false
	}
	lower := strings.ToLower(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	if hardDeny[lower] || strings.HasPrefix(lower, "mkfs") || askEvery[lower] || (winToo && winAsk[lower]) {
		return false
	}
	return true
}

// LegacyAllowNames filters the legacy shell.allow list down to the names that
// are accepted: single literal command names only, never a wildcard and never
// a hard-deny or ask-every-time name (on every OS, so a shared config cannot
// differ by platform).
func LegacyAllowNames(names []string) []string {
	var out []string
	for _, n := range names {
		if learnable(n, true) {
			out = append(out, n)
		}
	}
	return out
}

// RulesFromAllow builds a single "user" layer whose allow rules are the
// accepted legacy command names (see LegacyAllowNames).
func RulesFromAllow(names []string) *Rules {
	rs := RuleSet{Name: "user"}
	for _, n := range LegacyAllowNames(names) {
		if r, err := ParseRule(n); err == nil {
			rs.Allow = append(rs.Allow, r)
		}
	}
	return NewRules(rs)
}

// Learnable reports whether Allow would record name (tools.LearnableCommands:
// the asker offers allow-always only then).
func (s *Shell) Learnable(name string) bool { return learnable(name, s.goos == "windows") }

// Allow records an allow-always answer, subject to the same predicate.
func (s *Shell) Allow(name string) {
	if !s.Learnable(name) {
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

// cmdState is what the walk knows when it reaches a command: whether a
// cd/pushd/popd (or env -C) made the cwd unknown, and — auto mode only —
// whether something before it may have changed how a command name or its
// arguments resolve (an assignment, export/declare, printf -v, a cd to an
// unconfined or unknown directory). A tainted command is never allowed by
// the risk table; it goes to the classifier.
type cmdState struct {
	cwdUnknown bool
	tainted    bool
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
func (s *Shell) classify(ws []Word, v *verdictAcc, st cmdState) error {
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
	// A path-qualified head is its own identity (ruling 14): `./tool` is not
	// `tool`, so prompts show (and learn) the full head text. That holds for
	// rule-Ask entries in both modes and for every auto-mode entry; only the
	// default-mode ask-every-time class (askEvery/winAsk) shows the base name
	// (`/bin/rm x` → `rm`, as TestShellRules expects).
	name := base
	pathHead := strings.ContainsAny(head, `/\`)
	if pathHead {
		name = head
	}
	switch d, layer, src := s.rules.Decide(ws); d {
	case Deny:
		return fmt.Errorf("`%s` is denied by the permissions rule %q (%s config)", shortCmd(ws), src, layer)
	case Ask:
		v.add(&v.every, name)
		return nil
	case Allow:
		return nil
	}
	// A builtin is only a builtin by its bare name: `./echo` runs whatever
	// file is there.
	builtin := !pathHead && (builtinsOK[base] || (win && builtinsOK[lower]))
	if s.auto {
		switch {
		case builtin:
		default:
			// Windows runs the same table on its best-effort words, behind
			// winPathCheck (plain relative paths only) and with the
			// PowerShell removal aliases judged as rm.
			conf, tws := s.pathCheck(st.cwdUnknown), ws
			if win {
				conf = winPathCheck(conf)
				if !pathHead && winAsk[lower] {
					tws = append([]Word{{Text: "rm", Literal: true}}, ws[1:]...)
				}
			}
			verdict := autoClassify(tws, conf)
			if verdict == autoAllow && st.tainted {
				verdict = autoUnsure
			}
			// rm runs without a prompt only on literal paths the table
			// placed inside the workdir or a temp root; anything it cannot
			// place (a glob, a variable, a cd before it) asks every time
			// and never reaches the classifier.
			if verdict == autoUnsure && (askEvery[lower] || (win && winAsk[lower])) {
				verdict = autoAsk
			}
			if win && winAlwaysAsk[lower] {
				verdict = autoAsk
			}
			switch verdict {
			case autoAllow:
			case autoAsk:
				v.add(&v.every, name)
			default:
				v.add(&v.unsure, name)
			}
		}
		return nil
	}
	switch {
	case builtin:
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
	return s.checkTargetText(t, cwdUnknown)
}

func (s *Shell) checkTargetText(t string, cwdUnknown bool) error {
	if t == "/dev/null" || t == "/dev/stdout" || t == "/dev/stderr" {
		return nil
	}
	if strings.HasPrefix(t, "~") && t != "~" && !strings.HasPrefix(t, "~/") {
		return fmt.Errorf("redirect target %q: ~user, ~+ and ~- are not resolved; write the path out", t)
	}
	if innerDotDot(t) {
		return fmt.Errorf("redirect target %q has .. after a directory (a symlink would be followed first); write the path without it", t)
	}
	if cwdUnknown && !filepath.IsAbs(t) && t != "~" && !strings.HasPrefix(t, "~/") {
		return fmt.Errorf("relative redirect target %q after a cd/pushd/popd cannot be resolved; use an absolute path or run the cd separately", t)
	}
	if _, err := s.jail.Resolve(t, true); err != nil {
		return err
	}
	return nil
}

// pathCheck classifies literal path operands for the auto table against
// s.jail (the workdir plus its writable roots). After a cd a relative path
// cannot be placed; `~user`, `~+` and `~-` are never resolved.
func (s *Shell) pathCheck(cwdUnknown bool) pathFunc {
	// The guards compare case-folded: on a case-insensitive filesystem
	// `.GIT` is .git (stricter on Linux, where it is merely a sibling).
	fold := strings.ToLower
	root := s.jail.Root()
	return func(p string, write bool) pathClass {
		home := p == "~" || strings.HasPrefix(p, "~/")
		if strings.HasPrefix(p, "~") && !home {
			return pathOut
		}
		if innerDotDot(p) {
			return pathOut
		}
		if cwdUnknown && !filepath.IsAbs(p) && !home {
			return pathOut
		}
		res, err := s.jail.Resolve(p, write)
		if err != nil {
			return pathOut // outside, and every ancestor of the workdir
		}
		roots := append([]string{root}, s.jail.writable...)
		for _, r := range roots {
			if fold(res) == fold(r) {
				return pathRoot // the workdir or a writable root itself
			}
		}
		for _, r := range roots {
			if within(fold(res), fold(r)) {
				// An ancestor of the workdir (or of another writable root)
				// that lies inside a writable root: treated as outside.
				return pathOut
			}
		}
		if gitComponent(p) || gitComponent(res) {
			return pathGit
		}
		return pathOK
	}
}

// gitComponent reports whether any path component of p is `.git`
// (case-folded; trailing dots/spaces ignored, as Windows does). The match is
// the whole component: `.gitignore` is not one.
func gitComponent(p string) bool {
	for _, c := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.EqualFold(strings.TrimRight(c, ". "), ".git") {
			return true
		}
	}
	return false
}

// gitTarget reports whether a write target names .git or lands in it after
// resolution (a symlink to .git).
func (s *Shell) gitTarget(t string) bool {
	if gitComponent(t) {
		return true
	}
	res, err := s.jail.Resolve(t, true)
	return err == nil && gitComponent(res)
}

// headName is the identity a command head is shown under (ruling 14): the
// full text when path-qualified, else the base name.
func headName(t string) string {
	if strings.ContainsAny(t, `/\`) {
		return t
	}
	return filepath.Base(t)
}

// stmtLabel names a statement for an every-time ask about one of its
// redirects: the command it runs, or "redirect".
func stmtLabel(x *syntax.Stmt) string {
	if call, ok := x.Cmd.(*syntax.CallExpr); ok && len(call.Args) > 0 {
		if idxs, err := unwrap(call.Args); err == nil && len(idxs) > 0 {
			if t, ok := wordLit(call.Args[idxs[len(idxs)-1]]); ok {
				return headName(t)
			}
		}
	}
	return "redirect"
}

// innerDotDot reports whether p has a `..` after a named component
// (`link/..`). The jail cleans paths lexically, but the kernel applies `..`
// after following a symlink — `escape/../x` lands beside the symlink's
// target, not in the workdir — so such paths are not placed. A leading
// `../` is fine: it applies to the canonical workdir.
func innerDotDot(p string) bool {
	named := false
	for _, c := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		switch c {
		case "..":
			if named {
				return true
			}
		case ".":
		default:
			named = true
		}
	}
	return false
}

// cdConfined reports whether a cd goes to one literal directory inside the
// confinement. pushd/popd (a directory stack), a bare cd (home), `cd -`
// and a non-literal target do not.
func (s *Shell) cdConfined(ws []Word, cwdUnknown bool) bool {
	if strings.ToLower(filepath.Base(ws[0].Text)) != "cd" {
		return false
	}
	_, args := splitArgs(ws, 1)
	if len(args) != 1 || !args[0].Literal || args[0].Text == "-" {
		return false
	}
	return s.pathCheck(cwdUnknown)(args[0].Text, false) != pathOut
}

// checkWrapperOutputs checks every output file of a `time` wrapper (GNU
// time writes the last `-o FILE` / `--output[=]FILE`) as a redirect target.
// The wrapper words are walked with timeSpec, so option values (`-f FMT`,
// an earlier `-o`) never end the scan early. ws[:last idx] holds the
// wrapper words; unwrap already validated their options. In auto mode a
// target inside .git asks every time.
func (s *Shell) checkWrapperOutputs(ws []Word, idxs []int, cwdUnknown bool, v *verdictAcc) error {
	end := len(ws)
	if len(idxs) > 0 {
		end = idxs[len(idxs)-1]
	}
	for i := 0; i < end; i++ {
		if strings.ToLower(filepath.Base(ws[i].Text)) != "time" {
			continue
		}
	walk:
		for j := i + 1; j < end; j++ {
			t := ws[j].Text
			target, has := "", false
			switch {
			case t == "-o" || t == "--output":
				if j+1 < len(ws) {
					target, has = ws[j+1].Text, true
				}
				j++
			case strings.HasPrefix(t, "--output="):
				target, has = strings.TrimPrefix(t, "--output="), true
			case timeSpec.arg[t]:
				j++ // -f/--format FMT
			case strings.HasPrefix(t, "-"):
			default:
				break walk // the wrapped command
			}
			if !has {
				continue
			}
			if err := s.checkTargetText(target, cwdUnknown); err != nil {
				return err
			}
			if s.auto && s.gitTarget(target) {
				v.add(&v.every, headName(ws[i].Text))
			}
		}
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
	var cwdUnknown, tainted bool
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
				check := false
				switch r.Op {
				case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll, syntax.ClbOut, syntax.RdrInOut:
					check = true
				case syntax.DplOut:
					t, ok := wordLit(r.Word)
					check = !ok || (t != "-" && strings.Trim(t, "0123456789") != "")
				}
				if !check {
					continue
				}
				if err := s.checkTarget(r.Word, cwdUnknown); err != nil {
					fail(err)
				} else if t, _ := wordLit(r.Word); s.auto && s.gitTarget(t) {
					v.add(&v.every, stmtLabel(x)) // .git/config, hooks: code that git runs later
				}
			}
		case *syntax.DeclClause:
			tainted = true // export/declare/local/readonly/typeset: PATH and friends
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				tainted = true // a pure assignment (PATH=…) changes later commands
				return true
			}
			idxs, uerr := unwrap(x.Args)
			if uerr != nil {
				fail(uerr)
				return false
			}
			ws := toWords(x.Args)
			if err := s.checkWrapperOutputs(ws, idxs, cwdUnknown, &v); err != nil {
				fail(err)
				return false
			}
			for _, idx := range idxs {
				if !ws[idx].Literal {
					fail(errNonLiteral)
					return false
				}
				st := cmdState{cwdUnknown: cwdUnknown, tainted: tainted || len(x.Assigns) > 0}
				for _, pw := range ws[:idx] { // wrapper words: env NAME=VALUE, env -C DIR
					if !strings.HasPrefix(pw.Text, "-") && strings.Contains(pw.Text, "=") {
						st.tainted = true
					}
					if pw.Text == "-C" || strings.HasPrefix(pw.Text, "--chdir") {
						st.cwdUnknown, st.tainted = true, true
					}
				}
				if err := s.classify(ws[idx:], &v, st); err != nil {
					fail(err)
					return false
				}
				switch filepath.Base(ws[idx].Text) {
				case "printf":
					if hasOpt(ws, idx+1, "v") {
						tainted = true // printf -v PATH …
					}
				case "cd", "pushd", "popd":
					// Relative redirect targets are resolved against the jail
					// root, which is only correct while the cwd is unchanged;
					// after a cd only absolute targets can be checked. In auto
					// mode a cd anywhere but a confined directory also taints
					// what follows (a bare `ls` would read it).
					if s.auto && !s.cdConfined(ws[idx:], cwdUnknown) {
						tainted = true
					}
					cwdUnknown = true
				case "tee":
					for _, a := range x.Args[idx+1:] {
						if l, ok := wordLit(a); ok && strings.HasPrefix(l, "-") {
							continue
						}
						if err := s.checkTarget(a, cwdUnknown); err != nil {
							fail(err)
						} else if t, _ := wordLit(a); s.auto && s.gitTarget(t) {
							v.add(&v.every, headName(ws[idx].Text))
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

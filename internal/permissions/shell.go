package permissions

import (
	"errors"
	"fmt"
	"os"
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
	// skillDir is the project's resolved skills dir: a write target under it
	// is refused (the skill guard in tools validates those writes, and the
	// shell cannot). "" = no check — the feature is off.
	skillDir string
}

// SetSkillDir installs the skill-dir refusal (C1). It belongs to the feature
// being on: a resumed session whose switch went off does not call it.
func (s *Shell) SetSkillDir(dir string) { s.skillDir = dir }

// AutoView returns the auto-mode view of s: the same Rules (an allow-always
// learned in either view applies in both), the jail j (normally
// s.jail.WithWritable(TempRoots()...)), and the risk table instead of the
// default "needs approval" class.
func (s *Shell) AutoView(j *Jail) *Shell {
	// skillDir travels with the copy: auto mode swaps in this view
	// (agent.New), and losing the field would silently drop the refusal in
	// the mode that runs the most shell commands.
	return &Shell{rules: s.rules, auto: true, jail: j, goos: s.goos, skillDir: s.skillDir}
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
	// cwdOut: a cd whose target left the confinement (an unconfined or
	// non-literal cd, a bare `cd`, pushd/popd without an argument). make then
	// reads ITS makefile from the new directory — refused by the generic
	// runner check; a confined cd keeps it inside the workdir's own tree.
	cwdOut bool
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
		// A bare-name allow rule covers a tool's common forms, not arbitrary
		// code its arguments pick out of the repository: an injected "run
		// `make <target>`" must not ride the `make` entry (v0.7.0 §11
		// follow-up — the probe is TestGenericRunnerAllowRules).
		if genericRunners[lower] {
			if err := s.genericRunnerRefusal(base, ws, st, s.rules.AllowsNamed(ws)); err != nil {
				return err
			}
		}
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

// skillWriteRefusal carries no "refused:" prefix: the shell tool adds it to
// every Check error.
const skillWriteRefusal = "skill files are written with write/edit (the skill guard validates them), not the shell — use write or edit on .moca/skills/<dir>/SKILL.md"

// errSkillWrite is the skill-dir refusal as an error. It reports Internal():
// it is the shell side of the skill guard — moca's own procedure, not a
// project fact — so the tools layer must not let it arm the skill hint (the
// guard's own refusals never do, H2/H3). Structural: tools does not import
// this package.
var errSkillWrite error = internalErr(skillWriteRefusal)

type internalErr string

func (e internalErr) Error() string { return string(e) }

// Internal marks a refusal that comes from moca's own guards.
func (internalErr) Internal() bool { return true }

// genericRunners are allowlisted tools whose *arguments* choose code to run:
// the allow rule names the tool, not whatever its arguments hand it. For
// `make` the target is a recipe from the repository's own Makefile — exactly
// the shape a poisoned checkout would use ("run `make <target>` before
// testing"), and a bare `make` allow entry would run it without a prompt.
var genericRunners = map[string]bool{"make": true}

// safeMakeTargets are the recipe names a bare `make` allow rule covers: the
// workflow targets a repository's own tooling uses (build/check/read shapes).
var safeMakeTargets = map[string]bool{
	"all": true, "build": true, "test": true, "tests": true, "check": true,
	"fmt": true, "vet": true, "lint": true, "cover": true, "coverage": true,
	"tidy": true, "unit": true, "verify": true,
}

// safeMakeTargetList is safeMakeTargets for the refusal texts, sorted (one
// source: the messages cannot drift from the map).
var safeMakeTargetList = func() string {
	names := make([]string, 0, len(safeMakeTargets))
	for n := range safeMakeTargets {
		names = append(names, n)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}()

// makeCodeVars are variables whose command-line override picks the code make
// runs regardless of what the makefile says (the recipe shell, extra
// makefiles, nested flags).
var makeCodeVars = map[string]bool{"SHELL": true, ".SHELLFLAGS": true, "MAKEFILES": true,
	"MAKEFLAGS": true, "MFLAGS": true, "MAKE": true, "MAKESHELL": true}

// makeArgLong are make's long options whose argument may be the next word;
// makeCountLong take an optional numeric next word. Matched with longIn, so
// abbreviations count (`--dir sub` is --directory: make resolves a unique
// prefix and refuses an ambiguous one).
var (
	makeArgLong   = []string{"--directory", "--include-dir", "--old-file", "--assume-old", "--new-file", "--assume-new", "--what-if"}
	makeCountLong = []string{"--jobs", "--load-average", "--max-load"}
)

// plainMakeValue reports whether a command-line override's value is inert
// data — a word, number or version (`VAR=1`, `VERSION=1.2.3`). Any variable a
// recipe runs as a command can be overridden (`make test GORUN=./x.sh` runs
// ./x.sh in this repo's own Makefile), so paths, command strings, flags
// (`CFLAGS=-B.`) and expansions are code, not data.
func plainMakeValue(v string) bool {
	if strings.HasPrefix(v, "-") {
		return false
	}
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._,+-", c)) {
			return false
		}
	}
	return true
}

// genericRunnerRefusal fails closed on a generic runner driven by arguments,
// unless an allow rule named that code itself (`make bootstrap` in
// permissions.allow covers it; `make` alone does not — §11's rule that a rule
// covers what it names). EVERY target is checked (make runs them all), and
// the options and overrides that hand make code of their own — another
// makefile, --eval, -C/--directory outside the workdir (the value would run a
// different directory's makefile), SHELL=…, an immediate or shell
// assignment, a value that is not plain data, an option argument that is
// computed rather than written out (`-C $DIR` would otherwise be skipped
// unexamined) — are refused, as is a changed environment (envChanged: a
// prefix assignment, `env NAME=…` or an earlier export — make reads
// MAKEFILES and MAKEFLAGS from it, and every recipe inherits it).
func (s *Shell) genericRunnerRefusal(base string, ws []Word, st cmdState, named bool) error {
	if st.tainted {
		return fmt.Errorf("`%s` with a changed environment (a NAME=value prefix, env, or an earlier assignment or export) can choose the code it runs — pass plain overrides as arguments (`%s <target> NAME=value`), or add an explicit permissions.allow rule", base, base)
	}
	if st.cwdOut {
		// A cd (or pushd/popd) that left the confinement selects the directory
		// whose makefile the runner reads — a makefile of its own choosing
		// that no allowed target list covers. The safe-target names say
		// nothing about the recipe; the directory is the pick. A confined cd
		// (still inside the workdir's own tree) does not count.
		return fmt.Errorf("`%s` after a directory change (an earlier cd, or the wrapper's -C) reads a makefile of the new directory's choosing — run %s from the workdir, or add an explicit permissions.allow rule", base, base)
	}
	// A rule that names the target waives only the safe-target-list check
	// below (the author wrote those exact words after the tool's name). The
	// option-, override- and environment-level checks stay unconditional: `*`
	// rides anywhere inside a named rule's own words, and `make -f evil.mk
	// bootstrap` under an explicit `make bootstrap` entry must still refuse.
	chdirOutside := func(t string) error {
		return fmt.Errorf("`%s` with %s points a makefile of its own choosing (outside the workdir) — run %s against the workdir's makefile, or add an explicit permissions.allow rule", base, t, base)
	}
	dirOutside := func(val string) bool {
		if val == "" || st.cwdUnknown {
			return true
		}
		res, err := s.jail.Resolve(val, true)
		return err != nil || !within(s.jail.Root(), res)
	}
	ownMakefile := func(t string) error {
		return fmt.Errorf("`%s` with %s runs a makefile of its own choosing — add an explicit permissions.allow rule if you want it", base, t)
	}
	// computedArg: an option that takes an argument may not be handed a
	// computed word — `-C $DIR` changes directory to a path nothing here can
	// see, and the argument would otherwise be skipped unexamined.
	computedArg := func(t string) error {
		return fmt.Errorf("`%s` with %s and a computed argument (expansion or glob) is refused — write the argument out, or add an explicit permissions.allow rule", base, t)
	}
	// count is a numeric make argument (`-j 4`, `-l 2.5`).
	count := func(s string) bool {
		return s != "" && strings.Trim(s, "0123456789.") == "" &&
			strings.ContainsFunc(s, func(r rune) bool { return r >= '0' && r <= '9' })
	}
	var targets []string
	for i := 1; i < len(ws); i++ {
		w := ws[i]
		t := w.Text
		if !w.Literal {
			return fmt.Errorf("`%s` with a computed target or option (expansion or glob) is refused — write it out, or add an explicit permissions.allow rule", base)
		}
		switch {
		case t == "--":
			continue
		case strings.HasPrefix(t, "--"):
			name, val, hasVal := strings.Cut(t, "=")
			if longIs(name, "--file") || longIs(name, "--makefile") || longIs(name, "--eval") {
				return ownMakefile(t)
			}
			switch {
			case hasVal:
				if longIs(name, "--directory") && dirOutside(val) {
					return chdirOutside(t)
				}
			case longIs(name, "--directory"):
				if i+1 >= len(ws) || !ws[i+1].Literal {
					return computedArg(t)
				}
				if dirOutside(ws[i+1].Text) {
					return chdirOutside(t)
				}
				i++ // the option's argument, not a target
			case longIn(name, makeArgLong):
				if i+1 < len(ws) && !ws[i+1].Literal {
					return computedArg(t)
				}
				i++ // the option's argument, not a target
			case longIn(name, makeCountLong):
				if i+1 < len(ws) && ws[i+1].Literal && count(ws[i+1].Text) {
					i++ // make takes a numeric next word as the count
				}
			}
		case strings.HasPrefix(t, "-") && len(t) > 1:
			cluster := t[1:]
			for k := 0; k < len(cluster); k++ {
				c, rest := cluster[k], cluster[k+1:]
				if c == 'f' || c == 'E' {
					return ownMakefile(t)
				}
				if strings.IndexByte("CIoW", c) >= 0 {
					if rest == "" && i+1 < len(ws) {
						if !ws[i+1].Literal {
							return computedArg(t)
						}
						if c == 'C' && dirOutside(ws[i+1].Text) {
							return chdirOutside(t)
						}
						i++ // -C dir: the next word is the argument
					} else if c == 'C' && dirOutside(rest) {
						return chdirOutside(t) // -Cdir: the value rides the cluster
					}
					break
				}
				if c == 'j' || c == 'l' {
					if rest == "" && i+1 < len(ws) && ws[i+1].Literal && count(ws[i+1].Text) {
						i++
					}
					if rest == "" || count(rest) {
						break
					}
				}
			}
		case strings.Contains(t, "="):
			// VAR=value: not a recipe selector, but an override may still pick
			// the code make runs.
			// GNU make strips whitespace around the variable name, so
			// `MAKEFILES =x` sets MAKEFILES: trim it before the exact-name
			// lookup, or the override falls through to plainMakeValue and
			// names another makefile.
			name, val, _ := strings.Cut(t, "=")
			op := name
			name = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(name), ":+?!"))
			if makeCodeVars[name] || strings.Contains(op, "!") || !plainMakeValue(val) {
				return fmt.Errorf("`%s` with the override %s chooses code make runs — add an explicit permissions.allow rule if you want it", base, t)
			}
		default:
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		if !named {
			return fmt.Errorf("`%s` with no target runs the makefile's default recipe — name one of: %s", base, safeMakeTargetList)
		}
		return nil
	}
	for _, target := range targets {
		if !safeMakeTargets[target] && !named {
			return fmt.Errorf("`%s %s` runs a recipe from the repository's makefile — allowed recipes are: %s; add an explicit permissions.allow rule to run others", base, target, safeMakeTargetList)
		}
	}
	return nil
}

// skillTarget reports whether a literal write target lands under the project's
// skills dir after resolution. Pattern-level only: it knows nothing about
// approval and is not a sandbox (an interpreter can still write there).
func (s *Shell) skillTarget(t string) bool {
	if s.skillDir == "" || t == "" {
		return false
	}
	res, err := s.jail.Resolve(t, true)
	if err != nil {
		return false
	}
	if caseFolds(s.goos) {
		// A case-variant spelling (.moca/Skills/…) resolves into the real
		// skills dir on a case-folding filesystem: the byte-exact prefix
		// test inside within would let it past this refusal. Compare
		// component by component (EqualFold per whole component — a fixed
		// byte-length slice cut a multibyte fold equivalent in half).
		return foldWithin(res, s.skillDir)
	}
	return within(s.skillDir, res)
}

// skillOperand refuses a known writer's literal operand that lands under the
// skills dir: a `--opt=PATH` value is checked like a bare operand
// (`cp --target-directory=.moca/skills/x`), and after a cd — where a relative
// path cannot be placed against the workdir — a relative operand naming a
// SKILL.md or a skills component is refused on its spelling alone
// (`cd .moca/skills && cp x moca-a/SKILL.md`, `cd .moca && cp -r x skills/y`).
// That spelling also matches a project's own skills/ folder, so its refusal
// says how to get past it, like the after-cd redirect refusal does.
func (s *Shell) skillOperand(t string, cwdUnknown, afterDash bool) error {
	if s.skillDir == "" {
		return nil
	}
	// After `--` every word is an operand, including a dash-leading one
	// (`cp -- -skill dest` reads the file named -skill).
	if !afterDash {
		if strings.HasPrefix(t, "--") {
			_, v, ok := strings.Cut(t, "=")
			if !ok {
				return nil
			}
			t = v
		} else if strings.HasPrefix(t, "-") {
			return nil
		}
	}
	if t == "" {
		return nil
	}
	if cwdUnknown && !filepath.IsAbs(t) && t != "~" && !strings.HasPrefix(t, "~/") {
		for _, c := range pathComps(t) {
			if strings.EqualFold(c, "SKILL.md") || strings.EqualFold(c, "skills") {
				return internalErr(fmt.Sprintf("relative path %q after a cd/pushd/popd cannot be placed and may be in .moca/skills — use a path from the workdir, or run the cd separately (%s)", t, skillWriteRefusal))
			}
		}
		return nil
	}
	if s.skillTarget(t) {
		return errSkillWrite
	}
	return nil
}

// wrapperDirOutside reports whether a wrapper's -C/--chdir argument cannot be
// placed inside the workdir. An empty argument (missing, or a computed word the
// walk cannot read) counts as outside: fail closed, like a cd that leaves the
// confinement.
func (s *Shell) wrapperDirOutside(dir string) bool {
	if dir == "" || dir == "-" {
		return true
	}
	res, err := s.jail.Resolve(dir, true)
	if err != nil {
		return true
	}
	return !within(s.jail.Root(), res)
}

// caseFolds reports where the filesystem folds file-name case (darwin,
// windows) plus one-off child-process probes that force it (the same env var
// the tools package honors): the shell's skill-write refusal (C1) must see a
// case-variant skills spelling as the skills dir there. goos is the Shell's
// own view, set by the runtime at construction.
func caseFolds(goos string) bool {
	return goos == "darwin" || goos == "windows" || os.Getenv("MOCA_CASEFOLD_FS") == "1"
}

// foldWithin reports whether paths p == base or lies under base, EqualFold
// per path component (both arguments are absolute, symlink-resolved).
func foldWithin(p, base string) bool {
	pc, bc := pathComps(p), pathComps(base)
	if len(pc) < len(bc) {
		return false
	}
	for i, c := range bc {
		if !strings.EqualFold(pc[i], c) {
			return false
		}
	}
	return true
}

func pathComps(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '\\' })
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
			if s.skillTarget(target) {
				return errSkillWrite
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
	var cwdUnknown, tainted, cwdOutside bool
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
				if t, ok := wordLit(r.Word); ok && s.skillTarget(t) {
					fail(errSkillWrite)
				} else if err := s.checkTarget(r.Word, cwdUnknown); err != nil {
					fail(err)
				} else if s.auto && s.gitTarget(t) {
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
				st := cmdState{cwdUnknown: cwdUnknown, tainted: tainted || len(x.Assigns) > 0, cwdOut: cwdOutside}
				for k := 0; k < idx; k++ { // wrapper words: env NAME=VALUE, env -C DIR
					pw := ws[k]
					if !strings.HasPrefix(pw.Text, "-") && strings.Contains(pw.Text, "=") {
						st.tainted = true
					}
					dir, hasDir := "", false
					switch {
					case pw.Text == "-C" || pw.Text == "--chdir":
						hasDir = true
						if k+1 < idx && ws[k+1].Literal {
							dir = ws[k+1].Text
						}
					case strings.HasPrefix(pw.Text, "--chdir="):
						hasDir, dir = true, strings.TrimPrefix(pw.Text, "--chdir=")
					}
					if hasDir {
						// The wrapper runs the command in DIR: every operand
						// of that command resolves there (a relative one can
						// no longer be placed), and a directory that does not
						// resolve inside the workdir is an outside cwd — the
						// same rule an earlier cd has.
						st.cwdUnknown, st.tainted = true, true
						if s.wrapperDirOutside(dir) {
							st.cwdOut = true
						}
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
					confined := s.cdConfined(ws[idx:], cwdUnknown)
					if s.auto && !confined {
						tainted = true
					}
					if !confined {
						cwdOutside = true
					}
					cwdUnknown = true
				case "cp", "mv", "install", "ln", "truncate", "rm", "patch", "rsync":
					afterDash := false
					for _, a := range x.Args[idx+1:] {
						if t, ok := wordLit(a); ok {
							if t == "--" {
								afterDash = true
								continue
							}
							if err := s.skillOperand(t, st.cwdUnknown, afterDash); err != nil {
								fail(err)
							}
						}
					}
				case "sed":
					if hasOpt(ws, idx+1, "i", "--in-place") {
						afterDash := false
						for _, a := range x.Args[idx+1:] {
							if t, ok := wordLit(a); ok {
								if t == "--" {
									afterDash = true
									continue
								}
								if err := s.skillOperand(t, st.cwdUnknown, afterDash); err != nil {
									fail(err)
								}
							}
						}
					}
				case "tee":
					afterDash := false
					for _, a := range x.Args[idx+1:] {
						t, ok := wordLit(a)
						if ok && t == "--" {
							afterDash = true
							continue
						}
						if ok && !afterDash && strings.HasPrefix(t, "-") {
							continue
						}
						if s.skillTarget(t) {
							fail(errSkillWrite)
						} else if err := s.checkTarget(a, st.cwdUnknown); err != nil {
							fail(err)
						} else if s.auto && s.gitTarget(t) {
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

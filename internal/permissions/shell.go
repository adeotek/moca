package permissions

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/syntax"
)

var hardDeny = map[string]bool{"sudo": true, "su": true, "doas": true, "dd": true, "shred": true, "chown": true}

var builtinsOK = map[string]bool{"cd": true, "pwd": true, "echo": true, "printf": true, "test": true,
	"[": true, "true": true, "false": true, "exit": true}

var refused = map[string]bool{"eval": true, "source": true, ".": true, "exec": true}

var askEvery = map[string]bool{"rm": true}

// Shell analyses commands for the model-facing shell tool (§7). Not a
// sandbox: an allowlisted interpreter can do anything the user can.
type Shell struct {
	mu    sync.Mutex
	allow map[string]bool
	jail  *Jail
	goos  string
}

func NewShell(allow []string, jail *Jail, goos string) *Shell {
	s := &Shell{allow: map[string]bool{}, jail: jail, goos: goos}
	for _, a := range allow {
		s.allow[a] = true
	}
	return s
}

func (s *Shell) Allow(name string) {
	s.mu.Lock()
	s.allow[strings.ToLower(name)] = true
	s.allow[name] = true
	s.mu.Unlock()
}

func (s *Shell) allowed(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.allow[name]
}

type verdictAcc struct {
	need, every []string
}

func (v *verdictAcc) add(list *[]string, name string) {
	if !slices.Contains(*list, name) {
		*list = append(*list, name)
	}
}

func (s *Shell) classify(name string, v *verdictAcc) error {
	base := filepath.Base(name)
	switch {
	case hardDeny[base] || strings.HasPrefix(base, "mkfs"):
		return fmt.Errorf("%s is never allowed (hard-deny)", base)
	case refused[base]:
		return fmt.Errorf("%s is refused: it runs code the analyser cannot see", base)
	case builtinsOK[base]:
	case askEvery[base]:
		v.add(&v.every, base)
	case !s.allowed(base):
		v.add(&v.need, base)
	}
	return nil
}

// literalSafe reports whether a word part's runtime value is exactly its
// source text: no backslash escapes (unquoted words decode them; inside
// double quotes they are escapes) and, unquoted only, no glob
// metacharacters, which bash expands before the command runs.
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
	return true
}

// wordLit returns the literal value of a word made only of literal and
// quoted-literal parts. Words whose runtime value cannot be known exactly
// (expansions, escapes, globs, ANSI-C strings) report false so callers fail
// closed.
func wordLit(w *syntax.Word) (string, bool) {
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			if !literalSafe(x.Value, true) {
				return "", false
			}
			sb.WriteString(x.Value)
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
	return sb.String(), true
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

// skipWrapper consumes a wrapper's options and returns the index of the
// wrapped command name, or -1 when the wrapper has no wrapped command. It
// fails closed: an option it cannot model refuses the whole command.
func skipWrapper(args []*syntax.Word, i int, spec optSpec, wrapper string) (int, error) {
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

// unwrap skips wrapper commands and their options, returning the index of
// the wrapped command name in args (or -1 if none).
func unwrap(args []*syntax.Word) (int, error) {
	i := 0
	for i < len(args) {
		name, ok := wordLit(args[i])
		if !ok {
			return i, nil // a non-literal command name is refused by Check
		}
		var (
			j   int
			err error
		)
		switch filepath.Base(name) {
		case "env":
			j, err = skipWrapper(args, i, envSpec, "env")
		case "time":
			j, err = skipWrapper(args, i, timeSpec, "time")
		case "nice":
			j, err = skipWrapper(args, i, niceSpec, "nice")
		case "timeout":
			if j, err = skipWrapper(args, i, timeoutSpec, "timeout"); err == nil && j >= 0 && j < len(args) {
				if _, ok := wordLit(args[j]); !ok {
					return -1, fmt.Errorf("cannot analyse a non-literal `timeout` duration")
				}
				j++ // the duration word
			}
		case "nohup":
			j, err = skipWrapper(args, i, nohupSpec, "nohup")
		case "command":
			j, err = skipCommand(args, i)
		default:
			return i, nil
		}
		if err != nil {
			return -1, err
		}
		if j < 0 {
			return -1, nil
		}
		i = j
	}
	return -1, nil
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

func (s *Shell) Check(command string) ([]string, []string, error) {
	if s.goos == "windows" {
		return s.checkWindows(command)
	}
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse command (refused, no best-effort guessing): %v", err)
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
			idx, uerr := unwrap(x.Args)
			if uerr != nil {
				fail(uerr)
				return false
			}
			if idx < 0 {
				return true // wrappers are transparent; nothing wrapped (or `command -v`)
			}
			name, ok := wordLit(x.Args[idx])
			if !ok {
				fail(fmt.Errorf("non-literal command name refused (e.g. $CMD); write the command out"))
				return false
			}
			if err := s.classify(name, &v); err != nil {
				fail(err)
				return false
			}
			switch filepath.Base(name) {
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
		return true
	})
	if firstErr != nil {
		return nil, nil, firstErr
	}
	return v.need, v.every, nil
}

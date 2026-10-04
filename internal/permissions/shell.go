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

// wordLit returns the literal value of a word made only of literal and
// quoted-literal parts.
func wordLit(w *syntax.Word) (string, bool) {
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(x.Value)
		case *syntax.SglQuoted:
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, q := range x.Parts {
				l, ok := q.(*syntax.Lit)
				if !ok {
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

// unwrap skips wrapper commands and their options/arguments, returning the
// index of the wrapped command name in args (or -1 if none).
func unwrap(args []*syntax.Word) (int, error) {
	i := 0
	for i < len(args) {
		name, ok := wordLit(args[i])
		if !ok {
			return i, nil
		}
		switch filepath.Base(name) {
		case "env":
			i++
			for i < len(args) {
				a, _ := wordLit(args[i])
				if strings.HasPrefix(a, "-") || strings.Contains(a, "=") {
					i++
					continue
				}
				break
			}
		case "time", "nohup":
			i++
			for i < len(args) {
				if a, _ := wordLit(args[i]); strings.HasPrefix(a, "-") {
					i++
					continue
				}
				break
			}
		case "nice":
			i++
			for i < len(args) {
				a, _ := wordLit(args[i])
				if a == "-n" {
					i += 2
					continue
				}
				if strings.HasPrefix(a, "-") {
					i++
					continue
				}
				break
			}
		case "timeout":
			i++
			for i < len(args) {
				a, _ := wordLit(args[i])
				if a == "-s" || a == "-k" || a == "--signal" || a == "--kill-after" {
					i += 2
					continue
				}
				if strings.HasPrefix(a, "-") {
					i++
					continue
				}
				break
			}
			i++ // the duration
		case "command":
			if i+1 < len(args) {
				if a, _ := wordLit(args[i+1]); a == "-v" || a == "-V" {
					return -1, nil // lookup only
				}
			}
			i++
		default:
			return i, nil
		}
	}
	return -1, nil
}

func (s *Shell) checkTarget(w *syntax.Word) error {
	t, ok := wordLit(w)
	if !ok {
		return fmt.Errorf("non-literal redirect/tee target refused")
	}
	if t == "/dev/null" || t == "/dev/stdout" || t == "/dev/stderr" {
		return nil
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
				case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll, syntax.ClbOut:
					if err := s.checkTarget(r.Word); err != nil {
						fail(err)
					}
				case syntax.DplOut:
					if t, ok := wordLit(r.Word); !ok || (t != "-" && strings.Trim(t, "0123456789") != "") {
						if err := s.checkTarget(r.Word); err != nil {
							fail(err)
						}
					}
				}
			}
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				return true // pure assignment
			}
			idx, _ := unwrap(x.Args)
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
			if filepath.Base(name) == "tee" {
				for _, a := range x.Args[idx+1:] {
					if l, ok := wordLit(a); ok && strings.HasPrefix(l, "-") {
						continue
					}
					if err := s.checkTarget(a); err != nil {
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

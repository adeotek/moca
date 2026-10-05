package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// span is a JSON value's byte range in the (standardized) document.
type span struct{ start, end int } // end exclusive

type scanner struct {
	b []byte
	i int
}

func (s *scanner) ws() {
	for s.i < len(s.b) && strings.IndexByte(" \t\r\n", s.b[s.i]) >= 0 {
		s.i++
	}
}

func (s *scanner) str() (string, error) {
	start := s.i
	s.i++
	for s.i < len(s.b) {
		switch s.b[s.i] {
		case '\\':
			s.i += 2
			continue
		case '"':
			s.i++
			var v string
			err := json.Unmarshal(s.b[start:s.i], &v)
			return v, err
		}
		s.i++
	}
	return "", errors.New("unterminated string")
}

// value skips one value; if it is an object and path is non-empty it
// descends, returning the span of the deepest existing value on path and
// how many path elements were matched.
func (s *scanner) value(path []string) (span, int, error) {
	s.ws()
	start := s.i
	if s.i >= len(s.b) {
		return span{}, 0, errors.New("unexpected end")
	}
	switch s.b[s.i] {
	case '{':
		s.i++
		best, depth := span{start, -1}, 0
		for {
			s.ws()
			if s.b[s.i] == '}' {
				s.i++
				if depth == 0 {
					best.end = s.i
				}
				return best, depth, nil
			}
			if s.b[s.i] == ',' {
				s.i++
				continue
			}
			k, err := s.str()
			if err != nil {
				return span{}, 0, err
			}
			s.ws()
			s.i++ // ':'
			if len(path) > 0 && k == path[0] {
				sp, d, err := s.value(path[1:])
				if err != nil {
					return span{}, 0, err
				}
				best, depth = sp, d+1
			} else if _, _, err := s.value(nil); err != nil {
				return span{}, 0, err
			}
		}
	case '[':
		s.i++
		for {
			s.ws()
			if s.b[s.i] == ']' {
				s.i++
				return span{start, s.i}, 0, nil
			}
			if s.b[s.i] == ',' {
				s.i++
				continue
			}
			if _, _, err := s.value(nil); err != nil {
				return span{}, 0, err
			}
		}
	case '"':
		_, err := s.str()
		return span{start, s.i}, 0, err
	default:
		for s.i < len(s.b) && strings.IndexByte(",}] \t\r\n", s.b[s.i]) < 0 {
			s.i++
		}
		return span{start, s.i}, 0, nil
	}
}

func nested(keys []string, arr []string) string {
	b, _ := json.Marshal(arr)
	v := strings.ReplaceAll(string(b), `","`, `", "`)
	for i := len(keys) - 1; i >= 1; i-- {
		k, _ := json.Marshal(keys[i])
		v = fmt.Sprintf("{ %s: %s }", k, v)
	}
	k, _ := json.Marshal(keys[0])
	return fmt.Sprintf("%s: %s", k, v)
}

func concat(parts ...[]byte) []byte { return slices.Concat(parts...) }

// rawComma returns the index of the first ',' in gap that is outside a
// comment, or -1. gap holds only whitespace, comments and commas.
func rawComma(gap []byte) int {
	for i := 0; i < len(gap); i++ {
		switch {
		case gap[i] == ',':
			return i
		case gap[i] == '/' && i+1 < len(gap) && gap[i+1] == '/':
			for i < len(gap) && gap[i] != '\n' {
				i++
			}
		case gap[i] == '/' && i+1 < len(gap) && gap[i+1] == '*':
			end := strings.Index(string(gap[i+2:]), "*/")
			if end < 0 {
				return -1
			}
			i += 2 + end + 1
		}
	}
	return -1
}

// AppendString appends value to the string array at keyPath, creating the
// file and any missing intermediate objects as needed ("allow-always"
// persistence, §7). Comments, formatting and everything else stay
// byte-identical; the file is written atomically after a .bak copy, and a
// value already present is a no-op.
func AppendString(path string, keyPath []string, value string, init []string) error {
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("{\n  "+nested(keyPath, slices.Concat(init, []string{value}))+"\n}\n"), 0o600)
	}
	if err != nil {
		return err
	}
	std, err := Standardize(src)
	if err != nil {
		return err
	}
	sc := &scanner{b: std}
	sp, depth, err := sc.value(keyPath)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var out []byte
	if depth == len(keyPath) {
		var cur []string
		if err := json.Unmarshal(std[sp.start:sp.end], &cur); err != nil {
			return fmt.Errorf("%s: %s is not a string array", path, strings.Join(keyPath, "."))
		}
		if slices.Contains(cur, value) {
			return nil
		}
		q, _ := json.Marshal(value)
		closeAt := sp.end - 1 // index of ']'
		last := closeAt - 1   // last non-space before ']' (comments are spaces in std)
		for last > sp.start && strings.IndexByte(" \t\r\n", std[last]) >= 0 {
			last--
		}
		// A trailing comma sits in the RAW gap between the last element and ']'.
		// Standardize blanked it to a space in std, so it is invisible there — and
		// `last` skips whitespace, so it never points at it either way. Look for the
		// comma in src (byte indexes match: Standardize preserves length), skipping
		// comments: a ',' inside `// a, b` is the user's text, not a trailing comma.
		gapComma := rawComma(src[last+1 : closeAt])
		switch {
		case len(cur) == 0:
			out = concat(src[:closeAt], q, src[closeAt:])
		case gapComma >= 0:
			// trailing comma: insert after the last element, drop the old comma,
			// keep whatever else follows it (whitespace, comments) up to ']'.
			commaAt := last + 1 + gapComma
			out = concat(src[:last+1], []byte(", "+string(q)), src[commaAt+1:])
		default:
			out = concat(src[:last+1], []byte(", "+string(q)), src[last+1:])
		}
	} else {
		if std[sp.start] != '{' {
			return fmt.Errorf("%s: %s is not an object", path, strings.Join(keyPath[:depth], "."))
		}
		ins := "\n  " + nested(keyPath[depth:], slices.Concat(init, []string{value}))
		if strings.TrimSpace(string(std[sp.start+1:sp.end-1])) != "" {
			ins += ","
		}
		out = concat(src[:sp.start+1], []byte(ins), src[sp.start+1:])
	}
	if err := os.WriteFile(path+".bak", src, 0o600); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

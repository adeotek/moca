package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Prompt is a slash-command template: <dir>/<name>.md, frontmatter optional
// (description, argument-hint). First dir wins on a name collision (§10).
type Prompt struct {
	Name, Description, ArgumentHint, Path, Body, Source string
}

func LoadPrompts(dirs []Dir) []Prompt {
	seen := map[string]bool{}
	var out []Prompt
	for _, d := range dirs {
		ents, err := os.ReadDir(d.Path)
		if err != nil {
			continue
		}
		for _, e := range ents {
			name, ok := strings.CutSuffix(e.Name(), ".md")
			if !ok || e.IsDir() || seen[name] {
				continue
			}
			p := filepath.Join(d.Path, e.Name())
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			pr := Prompt{Name: name, Path: p, Source: d.Source, Body: string(data)}
			if fm, body, err := ParseFrontmatter(data); err == nil {
				pr.Description, pr.ArgumentHint, pr.Body = fm["description"], fm["argument-hint"], string(body)
			}
			seen[name] = true
			out = append(out, pr)
		}
	}
	slices.SortFunc(out, func(a, b Prompt) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// ExpandPrompt substitutes template references with the slash command's
// arguments (bash parameter-expansion subset, a superset of pi's): $1..$9,
// $@ / $ARGUMENTS (the raw argument string), $$ (a literal $), ${N},
// ${X:-w} ${X-w} ${X:+w} ${X+w} (X = N, @ or ARGUMENTS; w may reference
// arguments, one level), ${@:N} and ${@:N:L}. Arguments split like a shell
// (quotes group, "" is an empty argument); values are never re-expanded.
// Anything else — ${HOME}, an unterminated ${ — is kept verbatim.
func ExpandPrompt(body, args string) string {
	return expand(body, args, SplitArgs(args), true)
}

func expand(s, raw string, argv []string, braces bool) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' || i+1 >= len(s) {
			sb.WriteByte(s[i])
			continue
		}
		rest := s[i+1:]
		switch {
		case rest[0] == '$':
			sb.WriteByte('$')
			i++
		case rest[0] == '@':
			sb.WriteString(raw)
			i++
		case strings.HasPrefix(rest, "ARGUMENTS"):
			sb.WriteString(raw)
			i += len("ARGUMENTS")
		case rest[0] >= '1' && rest[0] <= '9':
			sb.WriteString(arg(argv, int(rest[0]-'0')))
			i++
		case rest[0] == '{' && braces:
			if end := closingBrace(rest); end > 0 {
				if out, ok := braceExpr(rest[1:end], raw, argv); ok {
					sb.WriteString(out)
					i += end + 1
					continue
				}
			}
			sb.WriteByte('$')
		default:
			sb.WriteByte('$')
		}
	}
	return sb.String()
}

// closingBrace returns the index of the } matching rest[0] == '{' (counting
// nested ${ pairs), or -1.
func closingBrace(rest string) int {
	depth := 0
	for i := 0; i < len(rest); i++ {
		switch {
		case rest[i] == '$' && i+1 < len(rest) && rest[i+1] == '$':
			i++ // literal $$ — cannot open a brace
		case rest[i] == '$' && i+1 < len(rest) && rest[i+1] == '{':
			depth++
			i++
		case rest[i] == '{' && i == 0:
			depth++
		case rest[i] == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

var braceName = regexp.MustCompile(`^(@|ARGUMENTS|[1-9][0-9]*)`)

func braceExpr(in, raw string, argv []string) (string, bool) {
	name := braceName.FindString(in)
	if name == "" {
		return "", false
	}
	val, set := raw, len(argv) > 0
	if name != "@" && name != "ARGUMENTS" {
		n, err := strconv.Atoi(name)
		if err != nil {
			return "", false
		}
		set = n <= len(argv)
		val = arg(argv, n)
	}
	op := in[len(name):]
	switch {
	case op == "":
		return val, true
	case name == "@" && len(op) > 1 && op[0] == ':' && op[1] >= '0' && op[1] <= '9':
		return slice(argv, op[1:])
	case strings.HasPrefix(op, ":-"):
		if val == "" {
			return expand(op[2:], raw, argv, false), true
		}
		return val, true
	case strings.HasPrefix(op, ":+"):
		if val != "" {
			return expand(op[2:], raw, argv, false), true
		}
		return "", true
	case op[0] == '-':
		if !set {
			return expand(op[1:], raw, argv, false), true
		}
		return val, true
	case op[0] == '+':
		if set {
			return expand(op[1:], raw, argv, false), true
		}
		return "", true
	}
	return "", false
}

// arg returns the n-th (1-based) argument, or "" when absent.
func arg(argv []string, n int) string {
	if n >= 1 && n <= len(argv) {
		return argv[n-1]
	}
	return ""
}

// slice implements ${@:N[:L]}: arguments from N (clamped to >= 1), at most L
// of them, space-joined. False on a malformed spec.
func slice(argv []string, spec string) (string, bool) {
	startS, lenS, hasLen := strings.Cut(spec, ":")
	start, err := strconv.Atoi(startS)
	if err != nil {
		return "", false
	}
	start = max(start, 1)
	end := len(argv)
	if hasLen {
		l, err := strconv.Atoi(lenS)
		if err != nil || l < 0 {
			return "", false
		}
		end = min(start-1+l, len(argv))
	}
	if start-1 >= end {
		return "", true
	}
	return strings.Join(argv[start-1:end], " "), true
}

// SplitArgs splits a slash command's argument string like a shell: whitespace
// separates, '…' and "…" group, and a quote pair with nothing in it still
// yields an (empty) argument. Unterminated quotes take the rest of the line.
func SplitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	inArg := false
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote, inArg = c, true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inArg {
				out = append(out, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteByte(c)
			inArg = true
		}
	}
	if inArg {
		out = append(out, cur.String())
	}
	return out
}

package skills

import (
	"bytes"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// blockHeader matches a YAML block-scalar header: `|` or `>` with optional
// chomping (`+`/`-`) and indentation (digit) indicators in either order and an
// optional trailing comment (`>+`, `|2-`, `> # note`).
var blockHeader = regexp.MustCompile(`^[|>][+\-0-9]*\s*(#.*)?$`)

// ParseFrontmatter reads the YAML-subset frontmatter used by SKILL.md and
// prompt templates across pi / claude-code / opencode. Only top-level
// scalar keys are returned; nested maps and lists are skipped.
func ParseFrontmatter(data []byte) (map[string]string, []byte, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, nil, errors.New("no frontmatter (file must start with ---)")
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil, nil, errors.New("unterminated frontmatter")
	}
	head := text[4 : 4+end]
	body := strings.TrimPrefix(text[4+end+4:], "\n")
	lines := strings.Split(head, "\n")
	fm := map[string]string{}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if l == "" || strings.HasPrefix(l, "#") || l[0] == ' ' || l[0] == '\t' || strings.HasPrefix(l, "- ") {
			continue
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if blockHeader.MatchString(v) {
			var block []string
			for i+1 < len(lines) && (lines[i+1] == "" || lines[i+1][0] == ' ' || lines[i+1][0] == '\t') {
				i++
				block = append(block, strings.TrimSpace(lines[i]))
			}
			sep := "\n"
			if v[0] == '>' {
				// Folded: every line break — including a paragraph break
				// (a blank line) — collapses to a single space; `|` keeps
				// its newlines.
				sep = " "
				block = slices.DeleteFunc(block, func(l string) bool { return l == "" })
			}
			fm[k] = strings.TrimSpace(strings.Join(block, sep))
			continue
		}
		if v == "" || v[0] == '#' {
			continue // nested map or list follows, or a comment-only value
		}
		fm[k] = scalar(v)
	}
	return fm, []byte(body), nil
}

// scalar decodes a single-line scalar value: quoted values are unquoted (a
// well-formed double-quoted value unescapes Go-style, a doubled single quote
// is one quote) and a trailing ` # comment` after a quoted value, or inside a
// plain one, is dropped as YAML does.
func scalar(v string) string {
	if end := closingQuote(v); end > 0 {
		if rest := strings.TrimSpace(v[end+1:]); rest == "" || strings.HasPrefix(rest, "#") {
			q := v[:end+1]
			if q[0] == '"' {
				if uq, err := strconv.Unquote(q); err == nil {
					return uq
				}
				return q[1 : len(q)-1] // not a Go-style quoted string; keep as written
			}
			return strings.ReplaceAll(q[1:len(q)-1], "''", "'")
		}
	}
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1] // quoted but not a lone string: keep the old tolerant strip
	}
	if v[0] != '"' && v[0] != '\'' {
		if i := strings.Index(v, " #"); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
	}
	return v
}

// closingQuote returns the index of the quote closing the quoted scalar that
// starts v, or -1 when v is not quoted or never closes.
func closingQuote(v string) int {
	if len(v) < 2 || (v[0] != '"' && v[0] != '\'') {
		return -1
	}
	for i := 1; i < len(v); i++ {
		switch {
		case v[0] == '"' && v[i] == '\\':
			i++
		case v[i] == v[0] && v[0] == '\'' && i+1 < len(v) && v[i+1] == '\'':
			i++ // '' is an escaped quote
		case v[i] == v[0]:
			return i
		}
	}
	return -1
}

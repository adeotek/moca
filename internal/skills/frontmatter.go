package skills

import (
	"bytes"
	"errors"
	"slices"
	"strconv"
	"strings"
)

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
		if v == "|" || v == ">" || v == "|-" || v == ">-" {
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
		if v == "" {
			continue // nested map or list follows
		}
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			if uq, err := strconv.Unquote(v); err == nil {
				v = uq
			} else {
				v = v[1 : len(v)-1] // not a Go-style quoted string; keep as written
			}
		} else if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
			v = strings.ReplaceAll(v[1:len(v)-1], "''", "'")
		}
		fm[k] = v
	}
	return fm, []byte(body), nil
}

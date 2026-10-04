package tools

import (
	"path"
	"regexp"
	"strings"
)

type ignoreRule struct {
	base    string // slash path of the dir holding the ignore file, "" = root
	re      *regexp.Regexp
	neg     bool
	dirOnly bool
}

// parseIgnore implements the gitignore subset rg honours: comments, blank
// lines, ! negation, trailing / (dirs only), leading or inner / (anchored to
// base), * ? [..] and ** globs, and \ escapes for a leading # or !.
func parseIgnore(base string, data []byte) []ignoreRule {
	var rules []ignoreRule
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := ignoreRule{base: base}
		if strings.HasPrefix(line, "!") {
			r.neg, line = true, line[1:]
		} else if strings.HasPrefix(line, `\`) {
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly, line = true, strings.TrimSuffix(line, "/")
		}
		anchored := strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		expr := globToRegexp(line)
		if !anchored {
			expr = `(?:.*/)?` + expr
		}
		re, err := regexp.Compile("^" + expr + "$")
		if err != nil {
			continue
		}
		r.re = re
		rules = append(rules, r)
	}
	return rules
}

func globToRegexp(g string) string {
	var sb strings.Builder
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch {
		case strings.HasPrefix(g[i:], "**/"):
			sb.WriteString(`(?:.*/)?`)
			i += 2
		case strings.HasPrefix(g[i:], "/**") && i+3 == len(g):
			sb.WriteString(`/.*`)
			i += 2
		case strings.HasPrefix(g[i:], "**"):
			sb.WriteString(`.*`)
			i++
		case c == '*':
			sb.WriteString(`[^/]*`)
		case c == '?':
			sb.WriteString(`[^/]`)
		case c == '[':
			end := strings.IndexByte(g[i:], ']')
			if end < 0 {
				sb.WriteString(`\[`)
				continue
			}
			cls := g[i+1 : i+end]
			if strings.HasPrefix(cls, "!") {
				cls = "^" + cls[1:]
			}
			sb.WriteString("[" + cls + "]")
			i += end
		case c == '\\' && i+1 < len(g):
			i++
			sb.WriteString(regexp.QuoteMeta(string(g[i])))
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return sb.String()
}

func ignored(rules []ignoreRule, rel string, isDir bool) bool {
	out := false
	for _, r := range rules {
		sub := rel
		if r.base != "" {
			if !strings.HasPrefix(rel, r.base+"/") {
				continue
			}
			sub = rel[len(r.base)+1:]
		}
		if r.dirOnly && !isDir {
			continue
		}
		if r.re.MatchString(sub) {
			out = !r.neg
		}
	}
	return out
}

func joinRel(dir, name string) string {
	if dir == "" || dir == "." {
		return name
	}
	return path.Join(dir, name)
}

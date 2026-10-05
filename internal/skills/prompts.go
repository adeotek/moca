package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

var promptVar = regexp.MustCompile(`\$(\$|ARGUMENTS|@|[1-9])`)

// ExpandPrompt substitutes $ARGUMENTS / $@ (all args), $1..$9 (whitespace-split)
// and $$ (a literal $).
func ExpandPrompt(body, args string) string {
	fields := strings.Fields(args)
	return promptVar.ReplaceAllStringFunc(body, func(m string) string {
		switch m {
		case "$$":
			return "$"
		case "$ARGUMENTS", "$@":
			return args
		}
		i := int(m[1] - '1')
		if i < len(fields) {
			return fields[i]
		}
		return ""
	})
}

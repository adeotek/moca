// Package skills loads Agent Skills (SKILL.md), embedded built-in skills and
// project instructions (AGENTS.md). Only name + description + absolute path
// enter the system prompt; bodies are loaded by the model with `read` (§9).
package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Skill struct {
	Name, Description, ArgumentHint, Path, Source string
}

type Dir struct{ Path, Source string }

func Discover(dirs []Dir) ([]Skill, []error) {
	seen := map[string]bool{}
	var out []Skill
	var errs []error
	for _, d := range dirs {
		ents, err := os.ReadDir(d.Path)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.Type()&fs.ModeSymlink != 0 {
				errs = append(errs, fmt.Errorf("%s: symlinked skill directory skipped (bodies load through the read jail; move the skill in or copy it)",
					filepath.Join(d.Path, e.Name())))
				continue
			}
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(d.Path, e.Name(), "SKILL.md")
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			fm, _, err := ParseFrontmatter(data)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", p, err))
				continue
			}
			name := fm["name"]
			if name == "" {
				name = e.Name()
			}
			if strings.TrimSpace(fm["description"]) == "" {
				errs = append(errs, fmt.Errorf("%s: missing description; skill skipped", p))
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			abs, _ := filepath.Abs(p)
			out = append(out, Skill{Name: name, Description: fm["description"], ArgumentHint: fm["argument-hint"], Path: abs, Source: d.Source})
		}
	}
	slices.SortFunc(out, func(a, b Skill) int { return strings.Compare(a.Name, b.Name) })
	return out, errs
}

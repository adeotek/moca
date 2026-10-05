package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const (
	searchCap         = 200
	searchMaxFileSize = 4 << 20
)

type searchTool struct{}

func (searchTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "search", Description: "Search file contents recursively with a regular expression " +
		"(Go RE2 syntax: no lookahead/lookbehind or backreferences; use (?i) for case-insensitive). " +
		"Respects .gitignore/.ignore and skips hidden and binary files. Returns `path:line:text`, " +
		"or only paths with files_only. At most 200 hits.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"pattern":{"type":"string","description":"RE2 regular expression"},` +
			`"path":{"type":"string","description":"Directory or file to search (default: workdir)"},` +
			`"glob":{"type":"string","description":"Filename filter, e.g. *.go or src/*.ts"},` +
			`"files_only":{"type":"boolean","description":"List matching files only"}},` +
			`"required":["pattern"],"additionalProperties":false}`)}
}

func (searchTool) Run(ctx context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Pattern   string `json:"pattern"`
		Path      string `json:"path"`
		Glob      string `json:"glob"`
		FilesOnly bool   `json:"files_only"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		return errorf("invalid pattern (Go RE2 syntax — no lookaround or backreferences): %v", err)
	}
	if a.Path == "" {
		a.Path = "."
	}
	start, err := env.Paths.Resolve(a.Path, false)
	if err != nil {
		return errorf("%v", err)
	}
	var hits []string
	capped := false
	skippedLarge := 0
	var rules []ignoreRule
	loadIgnores := func(dirAbs, rel string) {
		for _, n := range []string{".gitignore", ".ignore"} {
			if b, err := os.ReadFile(filepath.Join(dirAbs, n)); err == nil {
				rules = append(rules, parseIgnore(rel, b)...)
			}
		}
	}
	// gitignore semantics: ignore files apply from the workdir down, deeper
	// files overriding. Rules are keyed by paths relative to base; load the
	// ancestors of start first, the walk loads start itself and below.
	base := start
	if r, err := filepath.Rel(env.Root, start); err == nil && !strings.HasPrefix(r, "..") {
		base = env.Root
	}
	relBase := func(dir string) string {
		r, err := filepath.Rel(base, dir)
		if err != nil || r == "." {
			return ""
		}
		return filepath.ToSlash(r)
	}
	for cur := env.Root; base != start && cur != start; {
		loadIgnores(cur, relBase(cur))
		r, err := filepath.Rel(cur, start)
		if err != nil {
			break
		}
		cur = filepath.Join(cur, strings.SplitN(filepath.ToSlash(r), "/", 2)[0])
	}
	// display paths relative to the workdir when inside it
	display := func(abs string) string {
		if r, err := filepath.Rel(env.Root, abs); err == nil && !strings.HasPrefix(r, "..") {
			return filepath.ToSlash(r)
		}
		return abs
	}
	err = filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return ctx.Err()
		}
		rel := relBase(p)
		switch {
		case p == start:
			if d.IsDir() {
				loadIgnores(p, rel)
			}
		case strings.HasPrefix(d.Name(), ".") || ignored(rules, rel, d.IsDir()):
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		case d.IsDir():
			loadIgnores(p, rel)
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		disp := display(p)
		if a.Glob != "" {
			target := path.Base(disp)
			if strings.Contains(a.Glob, "/") {
				target = disp
			}
			if ok, _ := path.Match(a.Glob, target); !ok {
				return nil
			}
		}
		if fi, err := d.Info(); err != nil {
			return nil
		} else if fi.Size() > searchMaxFileSize {
			skippedLarge++
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil || bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
			return nil
		}
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 64<<10), 4<<20)
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			if !re.MatchString(line) {
				continue
			}
			if len(hits) == searchCap {
				capped = true
				return fs.SkipAll
			}
			if a.FilesOnly {
				hits = append(hits, disp)
				break
			}
			if len(line) > 300 {
				line = line[:300] + "…"
			}
			hits = append(hits, fmt.Sprintf("%s:%d:%s", disp, n, line))
		}
		return nil
	})
	if err != nil {
		return errorf("%v", err)
	}
	if len(hits) == 0 {
		msg := "[no matches]"
		if skippedLarge > 0 {
			msg += fmt.Sprintf("\n[… %d files larger than %d MB skipped]", skippedLarge, searchMaxFileSize>>20)
		}
		return Result{Content: msg, Summary: a.Pattern + " (0)"}
	}
	out := strings.Join(hits, "\n")
	if capped {
		out += "\n[… results capped at 200; narrow the pattern or path]"
	}
	if skippedLarge > 0 {
		out += fmt.Sprintf("\n[… %d files larger than %d MB skipped]", skippedLarge, searchMaxFileSize>>20)
	}
	return Result{Content: out, Summary: fmt.Sprintf("%s (%d)", a.Pattern, len(hits))}
}

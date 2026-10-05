package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type editTool struct{}

func (editTool) Spec() llm.ToolSpec {
	return llm.ToolSpec{Name: "edit", Description: "Replace old_string with new_string in a file that was read in " +
		"this session. old_string must match exactly once (include surrounding lines to disambiguate) unless " +
		"replace_all is set. If no exact match exists, one whitespace-insensitive match is accepted and " +
		"new_string is re-indented to fit. Copy text without read's `N|` prefixes. Returns a unified diff.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"File path"},` +
			`"old_string":{"type":"string","description":"Exact text to replace (non-empty)"},` +
			`"new_string":{"type":"string","description":"Replacement text"},` +
			`"replace_all":{"type":"boolean","description":"Replace every exact occurrence (default false)"}},` +
			`"required":["path","old_string","new_string"],"additionalProperties":false}`)}
}

func (editTool) Run(_ context.Context, env *Env, input json.RawMessage) Result {
	var a struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if r := decode(input, &a); r != nil {
		return *r
	}
	abs, err := env.Paths.Resolve(a.Path, true)
	if err != nil {
		return errorf("%v", err)
	}
	orig, err := os.ReadFile(abs)
	if err != nil {
		return errorf("%v (edit changes existing files; use write to create one)", err)
	}
	if err := env.Reads.Check(abs); err != nil {
		return errorf("%v", err)
	}
	out, hunks, err := applyEdit(orig, a.OldString, a.NewString, a.ReplaceAll)
	if err != nil {
		return errorf("%s: %v", a.Path, err)
	}
	if err := guardedWrite(env, abs, out); err != nil {
		return errorf("%v", err)
	}
	norm := func(b []byte) []string {
		return splitLines(strings.ReplaceAll(strings.TrimPrefix(string(b), "\ufeff"), "\r\n", "\n"))
	}
	oL, nL := norm(orig), norm(out)
	diff := formatDiff(a.Path, oL, nL, hunks)
	adds, dels := 0, 0
	for _, l := range strings.Split(diff, "\n")[2:] { // skip the ---/+++ header
		switch {
		case strings.HasPrefix(l, "+"):
			adds++
		case strings.HasPrefix(l, "-"):
			dels++
		}
	}
	var ranges []string
	shift := 0
	for _, h := range hunks {
		start := h.Line + shift
		ranges = append(ranges, fmt.Sprintf("%d-%d", start, start+max(len(h.New), 1)-1))
		shift += len(h.New) - len(h.Old)
	}
	content := fmt.Sprintf("%schanged lines %s; file now %d lines", diff, strings.Join(ranges, ", "), len(nL))
	return Result{Content: content, Summary: fmt.Sprintf("%s [+%d −%d]", a.Path, adds, dels), Detail: diff}
}

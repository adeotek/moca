package tools

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type hunk struct {
	Line     int // 1-based start line in the original
	Old, New []string
}

var bom = []byte{0xEF, 0xBB, 0xBF}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

var lineNumPrefix = regexp.MustCompile(`(?m)^\s*\d+\|`)

// applyEdit implements DESIGN.md §5 on an LF-normalized, BOM-stripped view
// and restores the file's dominant line ending and BOM on output.
func applyEdit(content []byte, oldS, newS string, replaceAll bool) ([]byte, []hunk, error) {
	if oldS == "" {
		return nil, nil, errors.New("old_string is empty; to create or overwrite a whole file use write")
	}
	if oldS == newS {
		return nil, nil, errors.New("old_string and new_string are identical; nothing to change")
	}
	hasBOM := bytes.HasPrefix(content, bom)
	content = bytes.TrimPrefix(content, bom)
	if !utf8.Valid(content) {
		return nil, nil, errors.New("file is not valid UTF-8; edit refused")
	}
	crlf := bytes.Count(content, []byte("\r\n"))
	lf := bytes.Count(content, []byte("\n")) - crlf
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	oldS = strings.ReplaceAll(oldS, "\r\n", "\n")
	newS = strings.ReplaceAll(newS, "\r\n", "\n")

	var out string
	var hunks []hunk
	n := strings.Count(text, oldS)
	switch {
	case n == 1 || (n > 1 && replaceAll):
		out, hunks = replaceExact(text, oldS, newS)
	case n > 1:
		return nil, nil, fmt.Errorf("old_string has %d matches (lines %s); add surrounding context to make it unique, or set replace_all", n, joinInts(matchLines(text, oldS)))
	case replaceAll:
		return nil, nil, errors.New("old_string not found (replace_all only matches exactly); re-read the file and copy the text verbatim")
	default:
		var err error
		out, hunks, err = replaceFuzzy(text, oldS, newS)
		if err != nil {
			return nil, nil, err
		}
	}
	if crlf > lf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	res := []byte(out)
	if hasBOM {
		res = append(append([]byte{}, bom...), res...)
	}
	return res, hunks, nil
}

func lineOf(text string, byteOff int) int { return strings.Count(text[:byteOff], "\n") + 1 }

func matchLines(text, s string) []int {
	var ls []int
	for off := 0; ; {
		i := strings.Index(text[off:], s)
		if i < 0 {
			return ls
		}
		ls = append(ls, lineOf(text, off+i))
		off += i + len(s)
	}
}

func joinInts(xs []int) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = fmt.Sprint(x)
	}
	return strings.Join(s, ", ")
}

// replaceExact replaces every exact occurrence; each hunk spans the whole
// lines touched by one occurrence.
func replaceExact(text, oldS, newS string) (string, []hunk) {
	var sb strings.Builder
	var hunks []hunk
	off := 0
	for {
		i := strings.Index(text[off:], oldS)
		if i < 0 {
			break
		}
		start, end := off+i, off+i+len(oldS)
		ls := strings.LastIndex(text[:start], "\n") + 1
		le := len(text)
		if j := strings.IndexByte(text[end:], '\n'); j >= 0 {
			le = end + j
		}
		hunks = append(hunks, hunk{
			Line: lineOf(text, ls),
			Old:  strings.Split(text[ls:le], "\n"),
			New:  strings.Split(text[ls:start]+newS+text[end:le], "\n"),
		})
		sb.WriteString(text[off:start])
		sb.WriteString(newS)
		off = end
	}
	sb.WriteString(text[off:])
	return sb.String(), hunks
}

func indentOf(s string) string { return s[:len(s)-len(strings.TrimLeft(s, " \t"))] }

func replaceFuzzy(text, oldS, newS string) (string, []hunk, error) {
	fileLines := strings.Split(text, "\n")
	oldLines := splitLines(oldS)
	norm := func(s string) string { return strings.TrimSpace(s) }
	var starts []int
	for i := 0; i+len(oldLines) <= len(fileLines); i++ {
		ok := true
		for j, ol := range oldLines {
			if norm(fileLines[i+j]) != norm(ol) {
				ok = false
				break
			}
		}
		if ok {
			starts = append(starts, i)
		}
	}
	if len(starts) == 0 {
		msg := "old_string not found (exact or whitespace-insensitive); re-read the relevant lines and retry with the exact text"
		if lineNumPrefix.MatchString(oldS) {
			msg += "; old_string looks like it includes read's `N|` line-number prefixes — drop them"
		}
		return "", nil, errors.New(msg)
	}
	if len(starts) > 1 {
		ls := make([]int, len(starts))
		for k, s := range starts {
			ls[k] = s + 1
		}
		return "", nil, fmt.Errorf("old_string has no exact match and %d whitespace-insensitive matches (lines %s); add more context", len(starts), joinInts(ls))
	}
	at := starts[0]
	// indentation delta between old_string's first non-blank line and the file line it matched
	var oldInd, fileInd string
	for j, ol := range oldLines {
		if strings.TrimSpace(ol) != "" {
			oldInd, fileInd = indentOf(ol), indentOf(fileLines[at+j])
			break
		}
	}
	var repl []string
	for _, nl := range splitLines(newS) {
		if strings.TrimSpace(nl) == "" {
			repl = append(repl, "")
			continue
		}
		ind := indentOf(nl)
		switch {
		case strings.HasPrefix(ind, oldInd):
			ind = fileInd + ind[len(oldInd):]
		case strings.HasPrefix(oldInd, ind):
			removed := oldInd[len(ind):]
			if strings.HasSuffix(fileInd, removed) {
				ind = fileInd[:len(fileInd)-len(removed)]
			} else {
				ind = fileInd[:max(0, len(fileInd)-len(removed))]
			}
		}
		repl = append(repl, ind+strings.TrimLeft(nl, " \t"))
	}
	orig := fileLines[at : at+len(oldLines)]
	out := append(append(append([]string{}, fileLines[:at]...), repl...), fileLines[at+len(oldLines):]...)
	return strings.Join(out, "\n"), []hunk{{Line: at + 1, Old: orig, New: repl}}, nil
}

// formatDiff renders unified-diff hunks with 3 lines of context, computed
// from the original and new full line slices. Common leading/trailing lines
// inside a hunk become context.
func formatDiff(path string, orig, upd []string, hunks []hunk) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", path, path)
	shift := 0
	for _, h := range hunks {
		o, n := h.Old, h.New
		pre := 0
		for pre < len(o) && pre < len(n) && o[pre] == n[pre] {
			pre++
		}
		suf := 0
		for suf < len(o)-pre && suf < len(n)-pre && o[len(o)-1-suf] == n[len(n)-1-suf] {
			suf++
		}
		oStart := h.Line - 1 + pre // 0-based first changed orig line
		oDel := o[pre : len(o)-suf]
		nAdd := n[pre : len(n)-suf]
		ctxA := max(0, oStart-3)
		ctxB := min(len(orig), oStart+len(oDel)+3)
		nStart := ctxA + shift
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", ctxA+1, ctxB-ctxA, nStart+1, ctxB-ctxA-len(oDel)+len(nAdd))
		for _, l := range orig[ctxA:oStart] {
			sb.WriteString(" " + l + "\n")
		}
		for _, l := range oDel {
			sb.WriteString("-" + l + "\n")
		}
		for _, l := range nAdd {
			sb.WriteString("+" + l + "\n")
		}
		for _, l := range orig[oStart+len(oDel) : ctxB] {
			sb.WriteString(" " + l + "\n")
		}
		shift += len(nAdd) - len(oDel)
	}
	return sb.String()
}

package tui

// `@path` file mentions: typing `@` plus a path fragment as the last word of
// the draft opens the dropdown with the workdir's files (gitignore-aware),
// `tab`/`enter` insert the path. The text is sent as written — the model
// reads the file itself with its windowed `read`, so a mention costs no
// tokens until it matters.

import (
	"context"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/tools"
)

const (
	maxMentionFiles = 5000 // files indexed per workdir
	maxMentionRows  = 50   // matches kept for the dropdown
)

type filesLoadedMsg struct {
	files []string
	gen   int
}

// invalidateFiles drops the @mention index (and any walk in flight) so the
// next mention re-indexes.
func (m *model) invalidateFiles() {
	m.files, m.filesLoading = nil, false
	m.filesGen++
}

// mentionQuery returns the fragment after the `@` of the draft's last word
// and the byte offset of that `@`; ok=false when the last word is not a
// mention (the `@` must start the word, so an email address never opens it).
func mentionQuery(buf string) (query string, start int, ok bool) {
	i := strings.LastIndexAny(buf, " \t\n") + 1
	if i >= len(buf) || buf[i] != '@' {
		return "", 0, false
	}
	return buf[i+1:], i, true
}

// maybeLoadFiles starts the workdir walk the first time a mention is typed
// (and again after the files may have changed): no walk for anyone who never
// types `@`.
func (m *model) maybeLoadFiles() tea.Cmd {
	if m.files != nil || m.filesLoading {
		return nil
	}
	if _, _, ok := mentionQuery(m.input.Buffer()); !ok {
		return nil
	}
	m.filesLoading = true
	dir, gen := m.opts.Start.Workdir, m.filesGen
	return func() tea.Msg {
		return filesLoadedMsg{gen: gen, files: append([]string{}, tools.ListFiles(context.Background(), dir, maxMentionFiles)...)}
	}
}

// rankFile scores a path against the fragment (lower is better, -1 = no
// match): basename prefix, then a path-segment prefix, then any substring.
func rankFile(path, ql string) int {
	pl := strings.ToLower(path)
	if ql == "" {
		return 3
	}
	base := pl[strings.LastIndexByte(pl, '/')+1:]
	switch {
	case strings.HasPrefix(base, ql):
		return 0
	case strings.HasPrefix(pl, ql) || strings.Contains(pl, "/"+ql):
		return 1
	case strings.Contains(pl, ql):
		return 2
	}
	return -1
}

// mentionMatches ranks the indexed files against the fragment; among equals
// the shallower, shorter path wins.
func mentionMatches(files []string, q string) []dropItem {
	ql := strings.ToLower(q)
	type scored struct {
		path        string
		rank, depth int
	}
	var hits []scored
	for _, f := range files {
		if r := rankFile(f, ql); r >= 0 {
			hits = append(hits, scored{f, r, strings.Count(f, "/")})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.depth != b.depth {
			return a.depth < b.depth
		}
		if len(a.path) != len(b.path) {
			return len(a.path) < len(b.path)
		}
		return a.path < b.path
	})
	if len(hits) > maxMentionRows {
		hits = hits[:maxMentionRows]
	}
	out := make([]dropItem, len(hits))
	for i, h := range hits {
		out[i] = dropItem{name: h.path}
	}
	return out
}

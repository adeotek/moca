package permissions

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// Word is one argv word of a simple command as the analyser sees it:
// Literal=false when its runtime value cannot be known (expansions, globs).
type Word struct {
	Text    string
	Literal bool
}

// Rule is one permissions pattern (SPECS §11): words separated by spaces; a
// standalone * matches zero or more words, a * inside a word any run of
// characters in that word. One word without * is a command-name rule
// (`rtk` ≡ `rtk *`).
type Rule struct {
	Src   string
	words []string
}

func ParseRule(s string) (Rule, error) {
	raw := strings.Fields(s)
	if len(raw) == 0 {
		return Rule{}, errors.New("empty permissions rule")
	}
	// Consecutive standalone * words are equivalent to one; collapsing them
	// bounds the backtracking in matchWords.
	ws := make([]string, 0, len(raw))
	for _, w := range raw {
		if w == "*" && len(ws) > 0 && ws[len(ws)-1] == "*" {
			continue
		}
		ws = append(ws, w)
	}
	if len(ws) == 1 && ws[0] != "*" {
		ws = append(ws, "*")
	}
	return Rule{Src: s, words: ws}, nil
}

// baseName is the command name of a head word: the last path element, with
// both separators honoured so a Windows-style path is handled on any host.
func baseName(s string) string {
	return filepath.Base(strings.ReplaceAll(s, `\`, "/"))
}

// match reports whether the rule covers cmd. conservative (deny/ask) compares
// the command name case-insensitively after stripping any directory from both
// the head word and the rule's first word, and treats non-literal words as
// possibly matching. Allow compares the head text exactly (so /tmp/x/git does
// not satisfy `git *`) and matches non-literal words only with a standalone *.
func (r Rule) match(cmd []Word, conservative bool) bool {
	if len(cmd) == 0 {
		return false
	}
	head := cmd[0]
	first := r.words[0]
	if conservative {
		head.Text = strings.ToLower(baseName(head.Text))
		first = strings.ToLower(baseName(first))
	}
	return matchWords(append([]string{first}, r.words[1:]...), append([]Word{head}, cmd[1:]...), conservative)
}

// matchWords matches pattern words against command words. A standalone *
// matches zero or more command words. In conservative mode a non-literal
// command word may expand to zero or more words, so it may be skipped or may
// be matched against a pattern word while staying available for more.
func matchWords(pat []string, ws []Word, conservative bool) bool {
	m := &matcher{pat: pat, ws: ws, conservative: conservative}
	m.memo = make([]uint8, (len(pat)+1)*(len(ws)+1))
	return m.match(0, 0)
}

// matcher memoises matchWords on (pattern index, command-word index). Every
// call leaves the same state space, so each state is evaluated at most once
// and matching is polynomial in len(pat)*len(ws)^2 instead of exponential in
// the number of stars and non-literal words.
type matcher struct {
	pat          []string
	ws           []Word
	conservative bool
	memo         []uint8 // 0 unknown, 1 false, 2 true
}

func (m *matcher) match(pi, wi int) bool {
	idx := pi*(len(m.ws)+1) + wi
	if v := m.memo[idx]; v != 0 {
		return v == 2
	}
	res := m.step(pi, wi)
	if res {
		m.memo[idx] = 2
	} else {
		m.memo[idx] = 1
	}
	return res
}

func (m *matcher) step(pi, wi int) bool {
	pat, ws := m.pat[pi:], m.ws[wi:]
	if len(pat) == 0 {
		if len(ws) == 0 {
			return true
		}
		// Only a possibly-empty expansion can remain past an exhausted pattern.
		return m.conservative && !ws[0].Literal && m.match(pi, wi+1)
	}
	if pat[0] == "*" {
		for i := 0; i <= len(ws); i++ {
			if m.match(pi+1, wi+i) {
				return true
			}
		}
		return false
	}
	if len(ws) == 0 {
		return false
	}
	if ws[0].Literal {
		return globWord(pat[0], ws[0].Text) && m.match(pi+1, wi+1)
	}
	if !m.conservative {
		return false
	}
	// Expands to nothing, or its first expanded word matches pat[0] and it may
	// expand to further words.
	return m.match(pi, wi+1) || m.match(pi+1, wi)
}

// globWord matches one word against a pattern where only * is special.
func globWord(p, s string) bool {
	parts := strings.Split(p, "*")
	if len(parts) == 1 {
		return p == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(s, mid)
		if i < 0 {
			return false
		}
		s = s[i+len(mid):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

type Decision int

const (
	NoMatch Decision = iota
	Allow
	Ask
	Deny
)

type RuleSet struct {
	Name             string
	Allow, Ask, Deny []Rule
}

func ParseRuleSet(name string, allow, ask, deny []string) (RuleSet, error) {
	rs := RuleSet{Name: name}
	for _, l := range []struct {
		src []string
		dst *[]Rule
		key string
	}{{allow, &rs.Allow, "allow"}, {ask, &rs.Ask, "ask"}, {deny, &rs.Deny, "deny"}} {
		for _, s := range l.src {
			r, err := ParseRule(s)
			if err != nil {
				return RuleSet{}, fmt.Errorf("permissions.%s: %w", l.key, err)
			}
			*l.dst = append(*l.dst, r)
		}
	}
	return rs, nil
}

func (rs RuleSet) decide(ws []Word) (Decision, string) {
	for _, r := range rs.Deny {
		if r.match(ws, true) {
			return Deny, r.Src
		}
	}
	for _, r := range rs.Ask {
		if r.match(ws, true) {
			return Ask, r.Src
		}
	}
	for _, r := range rs.Allow {
		if r.match(ws, false) {
			return Allow, r.Src
		}
	}
	return NoMatch, ""
}

// Rules is the layered rule store shared by every Shell view of a session:
// the first layer that matches decides (project before user).
type Rules struct {
	mu     sync.Mutex
	layers []RuleSet
}

func NewRules(layers ...RuleSet) *Rules {
	if len(layers) == 0 {
		layers = []RuleSet{{Name: "user"}}
	}
	return &Rules{layers: layers}
}

func (r *Rules) Decide(ws []Word) (Decision, string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.layers {
		if d, src := l.decide(ws); d != NoMatch {
			return d, l.Name, src
		}
	}
	return NoMatch, "", ""
}

// Learn records an allow-always answer for name in the user layer. name is
// one literal command word; anything else (empty, whitespace, a wildcard) is
// ignored rather than widened into a broader rule.
func (r *Rules) Learn(name string) {
	if name == "" || strings.Contains(name, "*") || strings.IndexFunc(name, unicode.IsSpace) >= 0 {
		return
	}
	rule, err := ParseRule(name)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	last := &r.layers[len(r.layers)-1]
	last.Allow = append(last.Allow, rule)
}

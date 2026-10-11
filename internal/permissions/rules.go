package permissions

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
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
	ws := strings.Fields(s)
	if len(ws) == 0 {
		return Rule{}, errors.New("empty permissions rule")
	}
	if len(ws) == 1 && !strings.Contains(ws[0], "*") {
		ws = append(ws, "*")
	}
	return Rule{Src: s, words: ws}, nil
}

// match reports whether the rule covers cmd. conservative (deny/ask) treats
// a non-literal word as possibly matching any pattern word and compares the
// command name case-insensitively; allow does neither.
func (r Rule) match(cmd []Word, conservative bool) bool {
	if len(cmd) == 0 {
		return false
	}
	head := cmd[0]
	head.Text = filepath.Base(head.Text)
	if conservative {
		head.Text = strings.ToLower(head.Text)
	}
	first := r.words[0]
	if conservative {
		first = strings.ToLower(first)
	}
	return matchWords(append([]string{first}, r.words[1:]...), append([]Word{head}, cmd[1:]...), conservative)
}

func matchWords(pat []string, ws []Word, conservative bool) bool {
	if len(pat) == 0 {
		return len(ws) == 0
	}
	if pat[0] == "*" {
		for i := 0; i <= len(ws); i++ {
			if matchWords(pat[1:], ws[i:], conservative) {
				return true
			}
		}
		return false
	}
	if len(ws) == 0 {
		return false
	}
	if ws[0].Literal {
		if !globWord(pat[0], ws[0].Text) {
			return false
		}
	} else if !conservative {
		return false
	}
	return matchWords(pat[1:], ws[1:], conservative)
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

// Learn records an allow-always answer for name in the user layer.
func (r *Rules) Learn(name string) {
	rule, err := ParseRule(name)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	last := &r.layers[len(r.layers)-1]
	last.Allow = append(last.Allow, rule)
}

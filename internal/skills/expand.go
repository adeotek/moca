package skills

import (
	"strings"
	"unicode"
)

// Expansion kinds reported by ExpandCommand.
const (
	ExpandNone     = ""
	ExpandSkill    = "skill"
	ExpandTemplate = "template"
	ExpandEscape   = "escape"
	// ExpandRemember is the reserved name every headless surface forwards to
	// Agent.Remember. It is a kind rather than a caller-side special case so a
	// user prompt template named `remember` cannot shadow it on any surface.
	ExpandRemember = "remember"
)

// RememberCommand is the reserved command name: /remember is the user's own
// rule, never a template or a skill (SPECS §3, §12).
const RememberCommand = "remember"

// ExpandCommand applies a leading `/name` to the skills and prompt templates a
// run has loaded, for the surfaces that do not run the TUI's ParseInput:
// `moca -p` and an ACP `session/prompt`. Both need the same rule, and a second
// copy of it drifted once already, so it lives here.
//
// The rules, in order:
//   - `//word` is the escape: the message is `word`, minus one slash.
//   - `/remember args` is reserved on every surface: kind ExpandRemember, and
//     the caller hands the rule to Agent.Remember (the TUI's own command).
//   - `/name args` where name is a prompt template expands the template with
//     args substituted (ExpandPrompt).
//   - `/name args` where name is a hand-written skill becomes the load-and-
//     follow message (SkillMessage). An **auto** skill is not invocable by
//     name — it rides the artifacts, not the switch (SPECS §12).
//   - anything else is returned as written, never as an error: a message can
//     legitimately start with a slash (`/etc/hosts is wrong`), and an unknown
//     word is prose, not a typo the tool can correct for you.
//
// The TUI's own built-in commands (`/model`, `/goal`, …) are not expanded:
// they act on the running interface, which a headless run does not have.
func ExpandCommand(text string, prompts []Prompt, sk []Skill) (out, name, kind string) {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "//") {
		word := t[2:]
		word = word[:cutWord(word)]
		return t[1:], word, ExpandEscape
	}
	if !strings.HasPrefix(t, "/") {
		return text, "", ExpandNone
	}
	var args string
	name, args = SplitCommand(t)
	if name == RememberCommand {
		return text, name, ExpandRemember
	}
	for _, p := range prompts {
		if p.Name == name {
			return ExpandPrompt(p.Body, args), name, ExpandTemplate
		}
	}
	for _, k := range sk {
		if k.Auto || k.Name != name {
			continue
		}
		return SkillMessage(k, args), name, ExpandSkill
	}
	return text, "", ExpandNone
}

// SplitCommand splits a trimmed "/name args" at the first whitespace of any
// kind — a tab or a newline after the name counts. The TUI, `-p`, ACP and the
// escape all split the same way, or `/review<TAB>src` expands on one surface
// and not on another (SPECS §3).
func SplitCommand(t string) (name, args string) {
	name = strings.TrimPrefix(t, "/")
	if i := strings.IndexFunc(name, unicode.IsSpace); i >= 0 {
		name, args = name[:i], strings.TrimSpace(name[i:])
	}
	return name, args
}

// cutWord is the index of the first space or newline in s (len(s) when there
// is none); the escape case names the word it unescaped.
func cutWord(s string) int {
	if i := strings.IndexAny(s, " \n\t"); i >= 0 {
		return i
	}
	return len(s)
}

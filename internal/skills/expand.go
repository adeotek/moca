package skills

import "strings"

// Expansion kinds reported by ExpandCommand.
const (
	ExpandNone     = ""
	ExpandSkill    = "skill"
	ExpandTemplate = "template"
	ExpandEscape   = "escape"
)

// ExpandCommand applies a leading `/name` to the skills and prompt templates a
// run has loaded, for the surfaces that do not run the TUI's ParseInput:
// `moca -p` and an ACP `session/prompt`. Both need the same rule, and a second
// copy of it drifted once already, so it lives here.
//
// The rules, in order:
//   - `//word` is the escape: the message is `word`, minus one slash.
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
	name, args, _ := strings.Cut(t[1:], " ")
	args = strings.TrimSpace(args)
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

// cutWord is the index of the first space or newline in s (len(s) when there
// is none); the escape case names the word it unescaped.
func cutWord(s string) int {
	if i := strings.IndexAny(s, " \n\t"); i >= 0 {
		return i
	}
	return len(s)
}

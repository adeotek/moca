package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/skills"
)

type InputKind int

const (
	KindText InputKind = iota
	KindCommand
	KindPrompt
	KindShell
	KindShellLocal
	// KindSkill is a skill invoked from the picker or by name (/rtk args):
	// the submission is a message telling the agent to load and follow the
	// skill's body (no new mechanism — the body loads via read).
	KindSkill
)

// Parsed is the classified input line: a message, a built-in command, an
// expanded prompt template, a skill invocation, or a shell command (`!` =
// to the model, `!!` = local only).
type Parsed struct {
	Kind             InputKind
	Name, Args, Text string
}

// BuiltinCommands win over prompt templates on a name collision (§10).
var BuiltinCommands = []string{"model", "effort", "hard", "mode", "yolo", "plan", "execute", "clear", "resume", "sessions", "compact", "cost", "undo", "copy", "show", "login", "logout", "help", "exit"}

// builtinAliases resolve to their built-in unless a prompt template already
// owns the name (`/q`, `/quit` → `/exit`) — only the full built-in names are
// reserved.
var builtinAliases = map[string]string{"q": "exit", "quit": "exit"}

var builtinHelp = map[string]string{
	"model":    "[provider/model]  pick a model or switch (forfeits prompt cache)",
	"effort":   "[level]  show or set effort (off|minimal|low|medium|high|xhigh|max)",
	"hard":     "toggle modelHard + high effort",
	"mode":     "[default|auto|yolo|plan]  show the permission mode or set it (shift+tab cycles default→auto→plan→yolo; between runs only)",
	"yolo":     "toggle yolo mode: ALL permission checks off (between runs only)",
	"plan":     "[request]  toggle plan mode (runs write an implementation plan to docs/plans/, change nothing else); with a request, plan it now",
	"execute":  "<plan.md> [instructions]  execute an implementation plan step by step, ticking its - [ ] boxes",
	"clear":    "start a new session (the old one stays resumable)",
	"resume":   "[id]  pick an earlier session of this directory to continue",
	"sessions": "list, switch or delete this directory's sessions",
	"compact":  "summarize older context now",
	"cost":     "session token and cost detail",
	"undo":     "revert the last write/edit of this session",
	"copy":     "copy the last assistant message (OSC 52)",
	"show":     "<n>  open item #n in the pager",
	"login":    "[provider]  store an API key or sign in to a subscription provider",
	"logout":   "[provider]  remove a stored login or API key",
	"help":     "this help",
	"exit":     "quit (/q, /quit — same as ctrl+c twice)",
}

// ParseInput classifies one input line. Rules: `!!x` → local shell, `!x` →
// shell, `/name args` → built-in command, prompt template, or skill (unknown
// → error), a leading `//` escapes to text, anything else is a message.
func ParseInput(s string, prompts []skills.Prompt, sk []skills.Skill) (Parsed, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "!!"):
		return Parsed{Kind: KindShellLocal, Text: strings.TrimSpace(s[2:])}, nil
	case strings.HasPrefix(s, "!"):
		return Parsed{Kind: KindShell, Text: strings.TrimSpace(s[1:])}, nil
	case strings.HasPrefix(s, "//"):
		return Parsed{Kind: KindText, Text: s[1:]}, nil
	case strings.HasPrefix(s, "/"):
		name, args, _ := strings.Cut(s[1:], " ")
		args = strings.TrimSpace(args)
		if t := builtinAliases[name]; t != "" && !slices.ContainsFunc(prompts, func(p skills.Prompt) bool { return p.Name == name }) {
			name = t
		}
		if slices.Contains(BuiltinCommands, name) {
			return Parsed{Kind: KindCommand, Name: name, Args: args}, nil
		}
		for _, p := range prompts {
			if p.Name == name {
				return Parsed{Kind: KindPrompt, Name: name, Args: args, Text: skills.ExpandPrompt(p.Body, args)}, nil
			}
		}
		for _, k := range sk {
			if k.Name == name {
				return Parsed{Kind: KindSkill, Name: name, Args: args, Text: skillMessage(k, args)}, nil
			}
		}
		return Parsed{}, fmt.Errorf("unknown command /%s (try /help)", name)
	}
	return Parsed{Kind: KindText, Text: s}, nil
}

// skillMessage is what a /skill submission sends: the model loads the
// body with read (the message names the path) and follows it.
func skillMessage(s skills.Skill, args string) string {
	if args == "" {
		return fmt.Sprintf("Load and follow the %s skill (%s).", s.Name, s.Path)
	}
	return fmt.Sprintf("Load and follow the %s skill (%s) for this request: %s", s.Name, s.Path, args)
}

// promptNamed reports whether a prompt template owns the name (built-in
// commands > prompts > skills: a shadowed name is listed once, resolved to
// its winner).
func promptNamed(prompts []skills.Prompt, name string) bool {
	return slices.ContainsFunc(prompts, func(p skills.Prompt) bool { return p.Name == name })
}

// agentSkills is the agent's current skill list (nil without an agent —
// setup mode has nothing to invoke).
func (m *model) agentSkills() []skills.Skill {
	if m.agent == nil {
		return nil
	}
	return m.agent.Skills()
}

// HelpText is the /help listing. Descriptions line up in one column per block
// — built-ins with the prompt templates, then the skills under their own
// heading — sized to the longest name in the block, since a template or skill
// can be named anything (a hard-coded width broke on `self-knowledge`).
func HelpText(prompts []skills.Prompt, sk []skills.Skill) string {
	var listedPrompts []skills.Prompt
	for _, p := range prompts {
		if !slices.Contains(BuiltinCommands, p.Name) {
			listedPrompts = append(listedPrompts, p)
		}
	}
	var listedSkills []skills.Skill
	for _, k := range sk {
		if !slices.Contains(BuiltinCommands, k.Name) && !promptNamed(prompts, k.Name) {
			listedSkills = append(listedSkills, k)
		}
	}

	w := 0
	for _, c := range BuiltinCommands {
		w = max(w, utf8.RuneCountInString(c))
	}
	for _, p := range listedPrompts {
		w = max(w, utf8.RuneCountInString(p.Name))
	}

	var sb strings.Builder
	row := func(width int, name, desc string) { fmt.Fprintf(&sb, "%-*s %s\n", width+1, "/"+name, desc) }
	for _, c := range BuiltinCommands {
		row(w, c, builtinHelp[c])
	}
	for _, p := range listedPrompts {
		// Join, don't "%s %s": an empty argument hint would otherwise leave a
		// double space and push the description a column right of the rest.
		row(w, p.Name, strings.TrimSpace(p.ArgumentHint+" "+p.Description))
	}
	if len(listedSkills) > 0 {
		sw := 0
		for _, k := range listedSkills {
			sw = max(sw, utf8.RuneCountInString(k.Name))
		}
		sb.WriteString("skills (invoke with /name — the agent loads and follows them):\n")
		for _, k := range listedSkills {
			row(sw, k.Name, k.Description)
		}
	}
	fmt.Fprintf(&sb, "%-*s run cmd, output goes to the model   !!cmd  run cmd locally only\n", w+1, "!cmd")
	sb.WriteString("type / for the command dropdown — ↑/↓ pick · tab completes · enter on an exact name sends\n")
	sb.WriteString("enter send · shift+enter newline · esc interrupt · shift+tab cycle mode (default/auto/plan/yolo) · ctrl+t cycle effort · ctrl+o pager (latest item) · alt+t read the latest thinking · alt+p paste chips · ctrl+r search history · ctrl+c×2 quit")
	return sb.String()
}

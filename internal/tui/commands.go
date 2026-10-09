package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/skills"
)

type InputKind int

const (
	KindText InputKind = iota
	KindCommand
	KindPrompt
	KindShell
	KindShellLocal
)

// Parsed is the classified input line: a message, a built-in command, an
// expanded prompt template, or a shell command (`!` = to the model, `!!` =
// local only).
type Parsed struct {
	Kind             InputKind
	Name, Args, Text string
}

// BuiltinCommands win over prompt templates on a name collision (§10).
var BuiltinCommands = []string{"model", "effort", "hard", "yolo", "clear", "resume", "sessions", "compact", "cost", "undo", "copy", "show", "login", "logout", "help", "exit"}

// builtinAliases resolve to their built-in unless a prompt template already
// owns the name (`/q`, `/quit` → `/exit`) — only the full built-in names are
// reserved.
var builtinAliases = map[string]string{"q": "exit", "quit": "exit"}

var builtinHelp = map[string]string{
	"model":    "[provider/model]  pick a model or switch (forfeits prompt cache)",
	"effort":   "[level]  show or set effort (off|minimal|low|medium|high|xhigh|max)",
	"hard":     "toggle modelHard + high effort",
	"yolo":     "toggle yolo mode: ALL permission checks off (between runs only)",
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
// shell, `/name args` → built-in command or prompt template (unknown → error),
// a leading `//` escapes to text, anything else is a message.
func ParseInput(s string, prompts []skills.Prompt) (Parsed, error) {
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
		return Parsed{}, fmt.Errorf("unknown command /%s (try /help)", name)
	}
	return Parsed{Kind: KindText, Text: s}, nil
}

func HelpText(prompts []skills.Prompt) string {
	var sb strings.Builder
	for _, c := range BuiltinCommands {
		fmt.Fprintf(&sb, "/%-8s %s\n", c, builtinHelp[c])
	}
	for _, p := range prompts {
		if slices.Contains(BuiltinCommands, p.Name) {
			continue
		}
		fmt.Fprintf(&sb, "/%-8s %s %s\n", p.Name, p.ArgumentHint, p.Description)
	}
	sb.WriteString("!cmd     run cmd, output goes to the model   !!cmd  run cmd locally only\n")
	sb.WriteString("type / for the command dropdown — ↑/↓ pick · tab completes · enter on an exact name sends\n")
	sb.WriteString("enter send · shift+enter newline · esc interrupt · ctrl+o pager (latest item) · alt+t read the latest thinking · alt+p paste chips · ctrl+r search history · ctrl+c×2 quit")
	return sb.String()
}

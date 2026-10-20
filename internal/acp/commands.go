package acp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/skills"
)

// flatten turns prompt blocks into one user message: text verbatim, a
// resource_link as an @path mention (the model reads it itself, as with the
// TUI's @), embedded text resources as fenced blocks. Images, audio and
// binary resources are refused (promptCapabilities says so).
func flatten(cwd string, blocks []ContentBlock) (string, error) {
	var sb strings.Builder
	space := func() {
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), " ") && !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteByte(' ')
		}
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if strings.HasPrefix(b.Text, " ") || strings.HasPrefix(b.Text, "\n") {
				sb.WriteString(b.Text)
			} else {
				space()
				sb.WriteString(b.Text)
			}
		case "resource_link":
			space()
			sb.WriteString("@" + mentionPath(cwd, b.URI))
		case "resource":
			if b.Resource == nil || b.Resource.Text == nil {
				return "", invalidParams("binary resources are not supported")
			}
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			fmt.Fprintf(&sb, "%s:\n```\n%s\n```", mentionPath(cwd, b.Resource.URI), strings.TrimRight(*b.Resource.Text, "\n"))
		default:
			return "", invalidParams("%s content is not supported", b.Type)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", invalidParams("empty prompt")
	}
	return sb.String(), nil
}

// textBlocks joins the prompt's `text` blocks the way flatten joins
// everything: the part the user typed. Attached `resource`/`resource_link`
// blocks never count (Decision 15 — an attachment cannot authorize a write).
func textBlocks(blocks []ContentBlock) string {
	var sb strings.Builder
	space := func() {
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), " ") && !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteByte(' ')
		}
	}
	for _, b := range blocks {
		if b.Type != "text" {
			continue
		}
		if strings.HasPrefix(b.Text, " ") || strings.HasPrefix(b.Text, "\n") {
			sb.WriteString(b.Text)
		} else {
			space()
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

// mentionPath: a file:// URI as a path, relative to cwd when inside it.
func mentionPath(cwd, uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := filepath.FromSlash(u.Path)
	if rel, err := filepath.Rel(cwd, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return p
}

// promptDirs mirrors the TUI: project templates only for a trusted workdir, then the global ones.
func promptDirs(cwd string, trusted bool) []skills.Dir {
	var d []skills.Dir
	if trusted {
		d = append(d, skills.Dir{Path: filepath.Join(cwd, ".moca", "prompts"), Source: "project"})
	}
	return append(d, skills.Dir{Path: config.PromptsDir(), Source: "global"})
}

// buildCommands lists prompt templates, then hand-written skills; the first
// owner of a name wins (the TUI's precedence, minus its other built-ins).
// remember reserves ACP's first built-in, only while the feature is on for the
// session (SPECS §9.5); auto skills are never advertised as commands.
func buildCommands(prompts []skills.Prompt, sk []skills.Skill, remember bool) []AvailableCommand {
	seen := map[string]bool{}
	out := []AvailableCommand{}
	add := func(name, desc, hint string) {
		if seen[name] {
			return
		}
		seen[name] = true
		c := AvailableCommand{Name: name, Description: desc}
		if hint != "" {
			c.Input = &AvailableCommandInput{Hint: hint}
		}
		out = append(out, c)
	}
	// N-10/N-17: reserved always, and added first so no prompt or skill named
	// `remember` can shadow it. It is advertised whether or not the feature is
	// on because typing it always does something the user can see: with the
	// feature off it is a refusal, never a template.
	desc := "record this as a project skill"
	if !remember {
		desc = "record this as a project skill (self-managed skills are off)"
	}
	add("remember", desc, "rule text")
	for _, p := range prompts {
		add(p.Name, p.Description, p.ArgumentHint)
	}
	for _, k := range sk {
		if k.Auto {
			continue
		}
		h := k.ArgumentHint
		if h == "" {
			h = "optional request"
		}
		add(k.Name, k.Description, h)
	}
	return out
}

// reloadCommands re-reads templates and skills; it reports whether the
// advertised list changed. Called only from the prompt handler (after
// begin) and before the session is published. It holds cmdMu: the agent's
// skill list and the command list are shared with a response's follow-up on
// another goroutine.
func (s *session) reloadCommands() bool {
	s.cmdMu.Lock()
	defer s.cmdMu.Unlock()
	s.agent.ReloadSkills()
	prompts := skills.LoadPrompts(promptDirs(s.cwd, s.trusted))
	cmds := buildCommands(prompts, s.agent.Skills(), s.agent.SelfManagedSkills())
	changed := !reflect.DeepEqual(cmds, s.commands)
	s.prompts, s.commands = prompts, cmds
	return changed
}

// sendCommands advertises the current command list. The list is taken under
// cmdMu and sent outside it — reloadCommands replaces the slice, never
// mutates it, so the value stays valid for the write.
func (s *session) sendCommands() {
	s.cmdMu.Lock()
	cmds := s.commands
	s.cmdMu.Unlock()
	s.send(AvailableCommandsUpdate{SessionUpdate: updCommands, AvailableCommands: cmds})
}

// splitCommand splits a trimmed "/name args" at the first whitespace of any
// kind (a newline after the name counts): the one split expand and the
// typed-text extraction share, so they cannot disagree on what was typed.
func splitCommand(t string) (name, args string) {
	name = strings.TrimPrefix(t, "/")
	if i := strings.IndexFunc(name, unicode.IsSpace); i >= 0 {
		name, args = name[:i], strings.TrimSpace(name[i:])
	}
	return name, args
}

// expand applies a leading /name the TUI's way: a template expands, a skill
// becomes its load-and-follow message, `//` escapes; anything else is sent
// as written (the model sees an unknown /word as text). The second return
// marks a /remember expansion: it starts a run of its own (Agent.Remember),
// which a plain string return could not (L5).
func (s *session) expand(text string) (string, bool) {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "//") {
		return t[1:], false
	}
	if !strings.HasPrefix(t, "/") {
		return text, false
	}
	name, args := splitCommand(t)
	if name == "remember" {
		// N-10/N-17: reserved on ACP unconditionally. It used to fall through
		// when the feature was off or no rule was typed, so a user prompt named
		// `remember` ran instead — or the raw "/remember" went to the model.
		// Agent.Remember owns every refusal (empty rule, plan, feature off) and
		// returns them as UserRefusal.
		return agent.RememberRequest(args), true
	}
	s.cmdMu.Lock()
	prompts, sk := s.prompts, s.agent.Skills()
	s.cmdMu.Unlock()
	for _, p := range prompts {
		if p.Name == name {
			return skills.ExpandPrompt(p.Body, args), false
		}
	}
	for _, k := range sk {
		if k.Auto {
			continue
		}
		if k.Name == name {
			return skills.SkillMessage(k, args), false
		}
	}
	return text, false
}

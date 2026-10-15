package acp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"

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

// buildCommands lists prompt templates, then skills; the first owner of a
// name wins (the TUI's precedence, minus its built-ins).
func buildCommands(prompts []skills.Prompt, sk []skills.Skill) []AvailableCommand {
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
	for _, p := range prompts {
		add(p.Name, p.Description, p.ArgumentHint)
	}
	for _, k := range sk {
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
	cmds := buildCommands(prompts, s.agent.Skills())
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

// expand applies a leading /name the TUI's way: a template expands, a skill
// becomes its load-and-follow message, `//` escapes; anything else is sent
// as written (the model sees an unknown /word as text).
func (s *session) expand(text string) string {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "//") {
		return t[1:]
	}
	if !strings.HasPrefix(t, "/") {
		return text
	}
	name, args := t[1:], ""
	if i := strings.IndexFunc(name, unicode.IsSpace); i >= 0 {
		name, args = name[:i], strings.TrimSpace(name[i:])
	}
	s.cmdMu.Lock()
	prompts, sk := s.prompts, s.agent.Skills()
	s.cmdMu.Unlock()
	for _, p := range prompts {
		if p.Name == name {
			return skills.ExpandPrompt(p.Body, args)
		}
	}
	for _, k := range sk {
		if k.Name == name {
			return skills.SkillMessage(k, args)
		}
	}
	return text
}

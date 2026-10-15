package mcp

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/adeotek/moca/internal/config"
)

// ProjectSources are the ecosystem config files a *project* may hold MCP
// servers in: Claude Code's `.mcp.json`, OpenCode's project config and Pi's
// project file (all in the `mcpServers` / `mcp` shapes ParseSource
// understands). Only existing files are returned.
func ProjectSources(cwd string) []Source {
	cands := []Source{
		{"claude-code", filepath.Join(cwd, ".mcp.json")},
		{"opencode", filepath.Join(cwd, "opencode.json")},
		{"opencode", filepath.Join(cwd, "opencode.jsonc")},
		{"pi", filepath.Join(cwd, ".pi", "mcp.json")},
	}
	var out []Source
	for _, c := range cands {
		if _, err := os.Stat(c.Path); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// MergeProject layers a trusted workdir's project MCP servers over the user
// config's (§10.5): the ecosystem project files first, then the project config
// file's `mcp.servers`, which may also *patch* a server another layer defines —
// a fields-set-in-the-patch-override merge with the approve lists united, so a
// project-scoped allow-always persists as a short entry beside the definition
// the user already has. The user config always wins a name it defines, and
// every conflict is reported as a warning naming the server and both files
// (userSrc/projSrc are the display names of those two config files). A project
// entry with neither command nor url that no other layer knows is skipped with
// a warning: it names a server nothing can start.
func MergeProject(cwd, userSrc, projSrc string, user map[string]config.MCPServer, proj config.MCPConfig) (map[string]config.MCPServer, []string) {
	eff := make(map[string]config.MCPServer, len(user))
	origin := make(map[string]string, len(user))
	for n, s := range user {
		eff[n], origin[n] = s, userSrc
	}
	var warns []string
	warn := func(format string, a ...any) { warns = append(warns, fmt.Sprintf(format, a...)) }

	for _, src := range ProjectSources(cwd) {
		servers, notes, err := ParseSource(src, cwd)
		for _, n := range notes {
			warn("%s: %s", src.Path, n)
		}
		if err != nil {
			warn("could not read %s: %v", src.Path, err)
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(servers)) {
			if prev, ok := origin[name]; ok {
				warn("mcp server %q is defined in %s and %s — using %s", name, src.Path, prev, prev)
				continue
			}
			eff[name], origin[name] = envRefs(servers[name]), src.Path
		}
	}

	for _, name := range slices.Sorted(maps.Keys(proj.Servers)) {
		ps := proj.Servers[name]
		cur, known := eff[name]
		if !known {
			if ps.Command == "" && ps.URL == "" {
				warn("mcp.servers.%s in %s: no command or url and no other layer defines %q — skipped", name, projSrc, name)
				continue
			}
			eff[name], origin[name] = ps, projSrc
			continue
		}
		if ps.Command != "" || ps.URL != "" {
			warn("mcp server %q is defined in %s and %s — the project config wins", name, origin[name], projSrc)
		}
		eff[name] = patchServer(cur, ps)
	}
	return eff, warns
}

// patchServer applies a project entry over the server it names: the fields the
// patch sets win (a transport replaces the other one outright), and the approve
// lists unite — an approval never takes another approval away.
func patchServer(base, patch config.MCPServer) config.MCPServer {
	out := base
	switch {
	case patch.URL != "":
		out.URL, out.Command, out.Args = patch.URL, "", nil
	case patch.Command != "":
		out.Command, out.URL = patch.Command, ""
	}
	if patch.Args != nil && out.Command != "" {
		out.Args = patch.Args
	}
	if patch.Env != nil {
		out.Env = patch.Env
	}
	if patch.Headers != nil {
		out.Headers = patch.Headers
	}
	if patch.Description != "" {
		out.Description = patch.Description
	}
	out.Approve = unite(base.Approve, patch.Approve)
	return out
}

// unite appends b's entries to a, keeping a's order and dropping duplicates.
func unite(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	out := slices.Clone(a)
	for _, v := range b {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

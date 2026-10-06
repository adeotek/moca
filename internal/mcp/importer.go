package mcp

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/config"
)

// Source is one host config file `moca mcp import` can read.
type Source struct{ Tool, Path string }

// EnvVar is one secret that was replaced by an env: reference; the user must
// export Name before the server can start.
type EnvVar struct {
	Name, Server, Field, Key string
	Tool                     string // source host config (for the export hint)
}

// DiscoverSources lists the existing Claude Code / OpenCode / Pi MCP config
// files. Pi paths come from pi-mcp-adapter's documented file layout: the
// shared user-global files and Pi's own (all the Claude-compatible
// `mcpServers` shape).
func DiscoverSources(home, cwd string) []Source {
	cands := []Source{
		{"claude-code", filepath.Join(home, ".claude.json")},
		{"claude-code", filepath.Join(cwd, ".mcp.json")},
		{"opencode", filepath.Join(home, ".config", "opencode", "opencode.json")},
		{"opencode", filepath.Join(home, ".config", "opencode", "opencode.jsonc")},
		{"pi", filepath.Join(home, ".config", "mcp", "mcp.json")},
		{"pi", filepath.Join(home, ".agents", "mcp.json")},
		{"pi", filepath.Join(home, ".agents", "mcp", "mcp.json")},
		{"pi", filepath.Join(home, ".pi", "agent", "mcp.json")},
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

type claudeServer struct {
	Type     string            `json:"type"`
	Command  string            `json:"command"`
	Args     []string          `json:"args"`
	Env      map[string]string `json:"env"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
	Enabled  *bool             `json:"enabled"`  // pi: enabled:false → disabled
	Disabled *bool             `json:"disabled"` // adapter override
}

type opencodeServer struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Enabled     *bool             `json:"enabled"`
}

func readJSONC(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	std, err := config.Standardize(b)
	if err != nil {
		return err
	}
	return json.Unmarshal(std, v)
}

// ParseSource returns the servers a source file defines, plus notes for what
// was skipped and why.
func ParseSource(src Source, cwd string) (map[string]config.MCPServer, []string, error) {
	out := map[string]config.MCPServer{}
	var notes []string
	addClaude := func(m map[string]claudeServer) {
		for n, s := range m {
			switch {
			case s.Type == "sse":
				notes = append(notes, fmt.Sprintf("skipped %s (%s): legacy SSE transport is not supported", n, src.Tool))
			case (s.Enabled != nil && !*s.Enabled) || (s.Disabled != nil && *s.Disabled):
				notes = append(notes, fmt.Sprintf("skipped %s (%s): disabled", n, src.Tool))
			case s.URL != "":
				out[n] = config.MCPServer{URL: s.URL, Headers: s.Headers}
			case s.Command != "":
				out[n] = config.MCPServer{Command: s.Command, Args: s.Args, Env: s.Env}
			}
		}
	}
	switch src.Tool {
	case "claude-code", "pi":
		var f struct {
			MCPServers map[string]claudeServer `json:"mcpServers"`
			Projects   map[string]struct {
				MCPServers map[string]claudeServer `json:"mcpServers"`
			} `json:"projects"`
		}
		if err := readJSONC(src.Path, &f); err != nil {
			return nil, nil, err
		}
		addClaude(f.MCPServers)
		if p, ok := f.Projects[cwd]; ok {
			addClaude(p.MCPServers)
		}
	case "opencode":
		var f struct {
			MCP map[string]opencodeServer `json:"mcp"`
		}
		if err := readJSONC(src.Path, &f); err != nil {
			return nil, nil, err
		}
		for n, s := range f.MCP {
			if s.Enabled != nil && !*s.Enabled {
				notes = append(notes, fmt.Sprintf("skipped %s (opencode): disabled", n))
				continue
			}
			switch {
			case s.Type == "remote":
				out[n] = config.MCPServer{URL: s.URL, Headers: s.Headers}
			case len(s.Command) > 0:
				out[n] = config.MCPServer{Command: s.Command[0], Args: s.Command[1:], Env: s.Environment}
			}
		}
	}
	return out, notes, nil
}

var (
	secretKey   = regexp.MustCompile(`(?i)key|token|secret|password|auth`)
	secretValue = regexp.MustCompile(`^(?:(?:Bearer|Basic) \S+|(?:sk-|ghp_|gho_|github_pat_|glpat-|xox[abp]-|AKIA)\S+)`)
	varRef      = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)
	nonAlnum    = regexp.MustCompile(`[^A-Z0-9]+`)
	// userinfo matches URL credentials like https://user:token@host/mcp.
	userinfo = regexp.MustCompile(`://[^/@\s:]+:[^/@\s]+@`)
	// tokenish matches credential-shaped tokens anywhere in a value, with a
	// length floor so benign strings ("sk-1") do not trip it: the same
	// prefixes as secretValue plus scheme-style header values.
	// passwordKV matches key=value connection-string secrets ("password=…").
	passwordKV = regexp.MustCompile(`(?i)\b(?:password|passwd|pwd)\s*=\s*\S+`)
	tokenish   = regexp.MustCompile(`(?:(?:sk-|ghp_|gho_|github_pat_|glpat-|xox[abp]-|AKIA)[A-Za-z0-9_-]{8,}|(?:Bearer|Basic)\s+\S{8,})`)
)

// embeddedSecret reports a credential inside an env/header value whose key
// gave nothing away: URL userinfo (postgres://user:pw@host), a
// credential-shaped token or Bearer/Basic scheme anywhere in the value, or a
// password=… pair (DSNs, connection strings).
func embeddedSecret(v string) bool {
	return userinfo.MatchString(v) || tokenish.MatchString(v) || passwordKV.MatchString(v)
}

// literalSecretFields lists the fields of a server carrying a
// credential-shaped literal that RewriteSecrets does not rewrite (it only
// touches env/headers): command, args, or URL userinfo. Such a server is
// skipped by Plan rather than imported with the secret copied verbatim.
func literalSecretFields(s config.MCPServer) []string {
	var fields []string
	if tokenish.MatchString(s.Command) {
		fields = append(fields, "command")
	}
	for i, a := range s.Args {
		if tokenish.MatchString(a) {
			fields = append(fields, fmt.Sprintf("args[%d]", i))
		}
	}
	if s.URL != "" && (userinfo.MatchString(s.URL) || tokenish.MatchString(s.URL)) {
		fields = append(fields, "url")
	}
	return fields
}

func envName(server, key string) string {
	return "MOCA_MCP_" + strings.Trim(nonAlnum.ReplaceAllString(strings.ToUpper(server), "_"), "_") + "_" +
		strings.Trim(nonAlnum.ReplaceAllString(strings.ToUpper(key), "_"), "_")
}

// RewriteSecrets replaces credential-looking env/header values with env:
// references; secrets are never copied literally. `${VAR}`/`$VAR` whole-value references
// become env:VAR (nothing to export); a secret-looking key or value becomes
// env:MOCA_MCP_<SERVER>_<KEY> and is reported for export.
func RewriteSecrets(server string, s config.MCPServer) (config.MCPServer, []EnvVar) {
	var vars []EnvVar
	rewrite := func(field string, m map[string]string) map[string]string {
		if m == nil {
			return nil
		}
		out := map[string]string{}
		for k, v := range m {
			switch {
			case strings.HasPrefix(v, "env:"):
				out[k] = v
			case varRef.MatchString(v):
				out[k] = "env:" + varRef.FindStringSubmatch(v)[1]
			case secretKey.MatchString(k) || secretValue.MatchString(v) || embeddedSecret(v):
				name := envName(server, k)
				out[k] = "env:" + name
				vars = append(vars, EnvVar{Name: name, Server: server, Field: field, Key: k})
			default:
				out[k] = v
			}
		}
		return out
	}
	s.Env = rewrite("env", s.Env)
	s.Headers = rewrite("headers", s.Headers)
	return s, vars
}

// Plan merges every source into the servers to add: first source wins on a
// name conflict, names already in moca's config are skipped, servers whose
// command line or URL embeds a literal secret are skipped (never imported
// with the secret copied), and so are servers whose env/headers keys would
// collide on one export variable name. The secret rewrite runs on the
// winners only.
func Plan(sources []Source, existing map[string]config.MCPServer, cwd string) (map[string]config.MCPServer, []EnvVar, []string) {
	adds := map[string]config.MCPServer{}
	var vars []EnvVar
	var notes []string
	origin := map[string]string{}
	usedVars := map[string]bool{}
	for _, src := range sources {
		servers, n, err := ParseSource(src, cwd)
		notes = append(notes, n...)
		if err != nil {
			notes = append(notes, fmt.Sprintf("could not read %s: %v", src.Path, err))
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(servers)) { // stable winners and notes
			s := servers[name]
			if _, ok := existing[name]; ok {
				notes = append(notes, fmt.Sprintf("skipped %s: already configured", name))
				continue
			}
			if o, ok := origin[name]; ok {
				notes = append(notes, fmt.Sprintf("skipped %s from %s: already taken from %s", name, src.Tool, o))
				continue
			}
			if fields := literalSecretFields(s); len(fields) > 0 {
				notes = append(notes, fmt.Sprintf("skipped %s: a literal secret appears in %s — replace it with an env: reference (or ${VAR}) in the source config first", name, strings.Join(fields, ", ")))
				continue
			}
			s2, v := RewriteSecrets(name, s)
			seen := map[string]bool{}
			dup := ""
			for _, e := range v {
				if seen[e.Name] || usedVars[e.Name] {
					dup = e.Name
					break
				}
				seen[e.Name] = true
			}
			if dup != "" {
				notes = append(notes, fmt.Sprintf("skipped %s: two secrets map to the export variable %s — rename one key so the names stay unique", name, dup))
				continue
			}
			for i := range v {
				v[i].Tool = src.Tool
				usedVars[v[i].Name] = true
			}
			adds[name], origin[name] = s2, src.Tool
			vars = append(vars, v...)
		}
	}
	return adds, vars, notes
}

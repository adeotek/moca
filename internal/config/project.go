package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ProjectConfigFile is the per-project config file (SPECS §4). It is read only
// for a trusted workdir (cmd decides) and may hold a permissions object and an
// mcp block: a cloned repository must not be able to point providers, keys or
// the classifier model anywhere.
func ProjectConfigFile(workdir string) string {
	return filepath.Join(workdir, ".moca", "config.jsonc")
}

// ProjectConfig is a trusted project file's content (SPECS §4): the same
// `permissions` block as the user config, plus an `mcp` block whose servers
// extend the user layer's (§10.5) — a project-scoped server, or a project's
// approval of one the user configured.
type ProjectConfig struct {
	Permissions PermissionsConfig `json:"permissions"`
	MCP         MCPConfig         `json:"mcp"`
}

// Mode is the project layer's default permission mode ("" when unset).
func (p *ProjectConfig) Mode() string {
	if p == nil {
		return ""
	}
	return p.Permissions.DefaultMode
}

// LoadProject reads the project file; nil, nil when absent.
func LoadProject(workdir string) (*ProjectConfig, error) {
	path := ProjectConfigFile(workdir)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	std, err := Standardize(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var f ProjectConfig
	dec := json.NewDecoder(bytes.NewReader(std))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, withPosition(data, err))
	}
	p := f.Permissions
	if err := p.validate("permissions"); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if p.DefaultMode == "yolo" {
		return nil, fmt.Errorf("%s: a project file cannot set defaultMode \"yolo\" (use --mode yolo or /yolo)", path)
	}
	if i := slices.IndexFunc(p.Allow, allowsEverything); i >= 0 {
		return nil, fmt.Errorf("%s: a project file cannot allow %q (matches every command)", path, p.Allow[i])
	}
	if p.Auto.Model != "" {
		return nil, fmt.Errorf("%s: permissions.auto.model is a user setting, not a project one", path)
	}
	if err := f.validateMCP(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &f, nil
}

// validateMCP checks the project file's mcp block. A server entry is either a
// full definition (command xor url) or a patch of a server another layer
// defines — an entry with neither is how moca persists a project-scoped
// allow-always (§10.5) — but never both transports, and it may not
// blanket-approve a server: a cloned repository must not be able to hand the
// agent every tool of a server it also controls.
func (f ProjectConfig) validateMCP() error {
	if f.MCP.IdleTimeout < 0 {
		return fmt.Errorf("mcp.idleTimeout must not be negative")
	}
	for name, s := range f.MCP.Servers {
		if s.Command != "" && s.URL != "" {
			return fmt.Errorf("mcp.servers.%s: command (stdio) and url (streamable HTTP) are mutually exclusive", name)
		}
		for i, t := range s.Approve {
			switch {
			case strings.TrimSpace(t) == "":
				return fmt.Errorf("mcp.servers.%s.approve[%d]: empty tool name", name, i)
			case t == "*":
				return fmt.Errorf("mcp.servers.%s.approve: a project file cannot blanket-approve \"*\" — list the tools it needs", name)
			}
		}
	}
	return nil
}

// allowsEverything reports whether an allow rule's first word is made only of
// '*' (e.g. "*", "**", "* rm"), so it would match any command.
func allowsEverything(rule string) bool {
	f := strings.Fields(rule)
	return len(f) > 0 && strings.Trim(f[0], "*") == ""
}

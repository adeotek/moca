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

// ProjectConfigFile is the per-project permissions file (SPECS §4). It is
// read only for a trusted workdir (cmd decides) and may hold nothing but a
// permissions object: a cloned repository must not be able to point
// providers, keys or the classifier model anywhere.
func ProjectConfigFile(workdir string) string {
	return filepath.Join(workdir, ".moca", "config.jsonc")
}

// LoadProject reads the project permissions file; nil, nil when absent.
func LoadProject(workdir string) (*PermissionsConfig, error) {
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
	var f struct {
		Permissions PermissionsConfig `json:"permissions"`
	}
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
		return nil, fmt.Errorf("%s: a project file cannot set defaultMode \"yolo\" (use --yolo or /yolo)", path)
	}
	if i := slices.IndexFunc(p.Allow, allowsEverything); i >= 0 {
		return nil, fmt.Errorf("%s: a project file cannot allow %q (matches every command)", path, p.Allow[i])
	}
	if p.Auto.Model != "" {
		return nil, fmt.Errorf("%s: permissions.auto.model is a user setting, not a project one", path)
	}
	return &p, nil
}

// allowsEverything reports whether an allow rule's first word is made only of
// '*' (e.g. "*", "**", "* rm"), so it would match any command.
func allowsEverything(rule string) bool {
	f := strings.Fields(rule)
	return len(f) > 0 && strings.Trim(f[0], "*") == ""
}

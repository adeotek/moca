package config

import (
	"os"
	"path/filepath"
)

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// ConfigDir is ~/.config/moca (or $XDG_CONFIG_HOME/moca).
func ConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "moca")
	}
	return filepath.Join(home(), ".config", "moca")
}

// DataDir is ~/.local/share/moca (or $XDG_DATA_HOME/moca): sessions,
// snapshots, auth.json, trust.json, mcp-index.json, builtin-skills.
func DataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "moca")
	}
	return filepath.Join(home(), ".local", "share", "moca")
}

func ConfigFile() string { return filepath.Join(ConfigDir(), "config.jsonc") }

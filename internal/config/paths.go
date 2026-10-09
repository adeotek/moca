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
// snapshots, trust.json, mcp-index.json, builtin-skills.
func DataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "moca")
	}
	return filepath.Join(home(), ".local", "share", "moca")
}

// ConfigFile is ~/.config/moca/config.jsonc.
func ConfigFile() string { return filepath.Join(ConfigDir(), "config.jsonc") }

// PromptsDir is ~/.config/moca/prompts: the user's prompt templates — one
// `<name>.md` per slash command. The first TUI run seeds the starter
// create-command template here (skills.SeedUserPrompts) and the jail allows
// the agent to read them and to write them with the user's approval
// (permissions.NewJail's ask-write root).
func PromptsDir() string { return filepath.Join(ConfigDir(), "prompts") }

// AuthFile is ~/.config/moca/auth.json: the credential store — API keys
// stored with /login (TUI) or `moca login <provider>`, and OAuth tokens
// (file 0600). It lives beside the config file so one directory holds
// everything the user manages; a v0.1 store in the data dir is migrated
// once (provider.MigrateLegacyStore).
func AuthFile() string { return filepath.Join(ConfigDir(), "auth.json") }

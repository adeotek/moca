package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DotEnvFile is ~/.config/moca/.env: optional environment variables loaded
// when moca starts (API keys, MCP server settings) — the file a user manages
// beside config.jsonc and auth.json.
func DotEnvFile() string { return filepath.Join(ConfigDir(), ".env") }

// LoadDotEnv reads the dotenv file at path and sets every variable it defines
// that is not already present in the process environment. A missing file is
// not an error (the file is optional); a variable already set — by the shell,
// a wrapper script or a previous call — keeps its value, so the real
// environment always wins. The returned count is the number of variables set.
func LoadDotEnv(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	text := strings.TrimPrefix(string(b), "\ufeff") // tolerate a UTF-8 BOM
	set := 0
	for i, line := range strings.Split(text, "\n") {
		name, value, ok, err := parseDotEnvLine(line)
		if err != nil {
			return set, fmt.Errorf("%s:%d: %w", path, i+1, err)
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(name); exists {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return set, fmt.Errorf("%s:%d: %w", path, i+1, err)
		}
		set++
	}
	return set, nil
}

// parseDotEnvLine parses one dotenv line: blank lines and `#` comments are
// skipped; `export ` is accepted; the value may be bare (an unquoted ` #`
// starts a trailing comment) or single-/double-quoted (double quotes honour
// \n, \r, \t, \\ and \"; single quotes are literal). It returns ok=false for
// a line that defines nothing.
func parseDotEnvLine(line string) (name, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	if rest, found := strings.CutPrefix(line, "export"); found && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
		line = strings.TrimSpace(rest)
	}
	key, raw, found := strings.Cut(line, "=")
	if !found {
		return "", "", false, errors.New("expected KEY=VALUE")
	}
	key = strings.TrimSpace(key)
	if !validEnvName(key) {
		return "", "", false, fmt.Errorf("invalid variable name %q", key)
	}
	value, err = parseDotEnvValue(raw)
	if err != nil {
		return "", "", false, err
	}
	return key, value, true, nil
}

// parseDotEnvValue parses the text after the first `=`: bare, single-quoted
// or double-quoted, with only whitespace or a comment allowed after a closing
// quote.
func parseDotEnvValue(raw string) (string, error) {
	raw = strings.TrimLeft(raw, " \t")
	if raw == "" {
		return "", nil
	}
	switch raw[0] {
	case '"':
		var b strings.Builder
		for i := 1; i < len(raw); i++ {
			switch raw[i] {
			case '\\':
				if i+1 >= len(raw) {
					return "", errors.New("unterminated escape in double-quoted value")
				}
				i++
				switch raw[i] {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				case '"', '\\':
					b.WriteByte(raw[i])
				default:
					b.WriteByte('\\')
					b.WriteByte(raw[i])
				}
			case '"':
				if err := onlyTrailingComment(raw[i+1:]); err != nil {
					return "", err
				}
				return b.String(), nil
			default:
				b.WriteByte(raw[i])
			}
		}
		return "", errors.New("unterminated double-quoted value")
	case '\'':
		end := strings.IndexByte(raw[1:], '\'')
		if end < 0 {
			return "", errors.New("unterminated single-quoted value")
		}
		if err := onlyTrailingComment(raw[end+2:]); err != nil {
			return "", err
		}
		return raw[1 : end+1], nil
	default:
		return strings.TrimSpace(stripTrailingComment(raw)), nil
	}
}

// onlyTrailingComment refuses anything but whitespace or a comment after a
// quoted value: `KEY="a" b` is a mistake, not a value with a suffix.
func onlyTrailingComment(rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.HasPrefix(rest, "#") {
		return nil
	}
	return fmt.Errorf("unexpected text after quoted value: %q", rest)
}

// stripTrailingComment drops a `#` comment from an unquoted value. A `#` is a
// comment only at the start or after whitespace, so a value like `a#b` keeps
// its hash.
func stripTrailingComment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}
		if i == 0 || s[i-1] == ' ' || s[i-1] == '\t' {
			return s[:i]
		}
	}
	return s
}

// validEnvName accepts the portable shell spelling: a letter or underscore
// followed by letters, digits and underscores.
func validEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

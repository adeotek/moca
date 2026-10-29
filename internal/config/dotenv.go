package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DotEnvFile is ~/.config/moca/.env: optional environment variables loaded
// when moca starts (API keys, MCP server settings) — the file a user manages
// beside config.jsonc and auth.json.
func DotEnvFile() string { return filepath.Join(ConfigDir(), ".env") }

// DotEnvVar is one assignment from the file, in the order its name first
// appears. A name defined twice keeps the last value and that value's line —
// the file's own last definition wins, as it does in every other dotenv
// implementation.
type DotEnvVar struct {
	Name  string
	Value string
	Line  int
}

// DotEnv is a parsed file. Found is false when there is no file — it is
// optional, so that is not a problem. A malformed line never stops the parse:
// the lines around it are still read, because dropping the rest of the file
// turns one typo into "the key is missing" with nothing pointing at the typo.
type DotEnv struct {
	Found   bool
	Vars    []DotEnvVar
	LineErr []error // malformed lines, each prefixed path:line
	// ModeNote warns that the file is readable by users other than its owner,
	// which matters because it holds the values env: references name. It is
	// advice, not a defect: the file still loads.
	ModeNote error
}

// DotEnvReport is what LoadDotEnv did: how many variables reached the process
// environment, and what the user should be told — the file's malformed lines
// and over-readable mode, plus the assignments that were not applied.
type DotEnvReport struct {
	Set   int
	Notes []error
}

// ReadDotEnv parses the dotenv file at path without touching the environment
// (`moca config check` reads through here). A missing file is not an error.
func ReadDotEnv(path string) (DotEnv, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DotEnv{}, nil
		}
		return DotEnv{}, err
	}
	d := DotEnv{Found: true}
	// The file holds the values env: references resolve to, so on a
	// multi-user host a group/world-readable one leaks them. Windows has no
	// such mode to report (Go synthesises 0666 there), so the check is Unix-only.
	if fi, err := os.Stat(path); err == nil && runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		d.ModeNote = fmt.Errorf("%s: mode %#o makes it readable by others — chmod 600 it (it holds the keys env: references name)", path, fi.Mode().Perm())
	}
	text := strings.TrimPrefix(string(b), "\ufeff") // tolerate a UTF-8 BOM
	index := map[string]int{}
	for i, line := range strings.Split(text, "\n") {
		name, value, ok, err := parseDotEnvLine(line)
		if err != nil {
			d.LineErr = append(d.LineErr, fmt.Errorf("%s:%d: %w", path, i+1, err))
			continue
		}
		if !ok {
			continue
		}
		if j, seen := index[name]; seen {
			d.Vars[j].Value, d.Vars[j].Line = value, i+1
			continue
		}
		index[name] = len(d.Vars)
		d.Vars = append(d.Vars, DotEnvVar{Name: name, Value: value, Line: i + 1})
	}
	return d, nil
}

// LoadDotEnv reads the dotenv file at path and sets every variable it defines
// that is not already present in the process environment. A missing file is not
// an error (the file is optional); a variable already set — by the shell, a
// wrapper script or a previous load — keeps its value, so the real environment
// always wins, an empty one included. Nothing here is fatal: a bad line and an
// assignment the OS refuses are reported in the returned notes and the rest are
// still applied.
func LoadDotEnv(path string) (DotEnvReport, error) {
	d, err := ReadDotEnv(path)
	if err != nil {
		return DotEnvReport{}, err
	}
	rep := DotEnvReport{Notes: d.LineErr}
	if d.ModeNote != nil {
		rep.Notes = append(rep.Notes, d.ModeNote)
	}
	for _, v := range d.Vars {
		if _, exists := os.LookupEnv(v.Name); exists {
			continue
		}
		if dotEnvReserved(v.Name) {
			rep.Notes = append(rep.Notes, fmt.Errorf("%s:%d: %s is not applied — it decides where moca keeps its files, so it cannot come from the file itself", path, v.Line, v.Name))
			continue
		}
		if err := os.Setenv(v.Name, v.Value); err != nil {
			rep.Notes = append(rep.Notes, fmt.Errorf("%s:%d: %s: %w", path, v.Line, v.Name, err))
			continue
		}
		rep.Set++
	}
	return rep, nil
}

// dotEnvReserved reports whether name decides where moca keeps its files. Such
// a variable cannot come from the file it was read from: applying it mid-boot
// would move the config directory — and auth.json with it — away from the file
// that asked for the move, so the next start would read a different file.
func dotEnvReserved(name string) bool {
	switch name {
	case "HOME", "XDG_CONFIG_HOME":
		return true
	}
	return false
}

// parseDotEnvLine parses one dotenv line: blank lines and `#` comments are
// skipped (an `export` before a comment too); `export ` is accepted; the value
// may be bare (an unquoted `#` after whitespace starts a trailing comment) or
// single-/double-quoted (double quotes honour \n, \r, \t, \\ and \"; single
// quotes are literal). It returns ok=false for a line that defines nothing.
func parseDotEnvLine(line string) (name, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	if rest, found := strings.CutPrefix(line, "export"); found && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
		line = strings.TrimSpace(rest)
		if line == "" || strings.HasPrefix(line, "#") {
			return "", "", false, nil // `export` marks an assignment; on its own it defines nothing
		}
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

// parseDotEnvValue parses the text after the first `=`: bare, single-quoted or
// double-quoted, with only whitespace or a comment allowed after a closing
// quote.
func parseDotEnvValue(raw string) (string, error) {
	quoted := strings.TrimLeft(raw, " \t")
	switch {
	case quoted == "":
		return "", nil
	case quoted[0] == '"':
		return unquoteDotEnvDouble(quoted)
	case quoted[0] == '\'':
		return unquoteDotEnvSingle(quoted)
	}
	return strings.TrimSpace(stripTrailingComment(raw)), nil
}

// unquoteDotEnvDouble parses a double-quoted value, honouring \n, \r, \t, \\
// and \"; any other escaped byte keeps its backslash.
func unquoteDotEnvDouble(s string) (string, error) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 >= len(s) {
				return "", errors.New("unterminated escape in double-quoted value")
			}
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '"', '\\':
				b.WriteByte(s[i])
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		case '"':
			if err := onlyTrailingComment(s[i+1:]); err != nil {
				return "", err
			}
			return b.String(), nil
		default:
			b.WriteByte(s[i])
		}
	}
	return "", errors.New("unterminated double-quoted value")
}

// unquoteDotEnvSingle parses a single-quoted value: everything up to the next
// quote is taken literally, so a $ or a backslash needs no escaping.
func unquoteDotEnvSingle(s string) (string, error) {
	end := strings.IndexByte(s[1:], '\'')
	if end < 0 {
		return "", errors.New("unterminated single-quoted value")
	}
	if err := onlyTrailingComment(s[end+2:]); err != nil {
		return "", err
	}
	return s[1 : end+1], nil
}

// onlyTrailingComment refuses anything but whitespace or a comment after a
// quoted value: `KEY="a" b` is a mistake, not a value with a suffix. The text
// itself is not quoted back — a key pasted outside its quotes would otherwise
// land in a terminal and in the diagnostic log.
func onlyTrailingComment(rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.HasPrefix(rest, "#") {
		return nil
	}
	return errors.New("unexpected text after a quoted value")
}

// stripTrailingComment drops a `#` comment from an unquoted value. `#` starts a
// comment only after whitespace, never at the value's first byte, so `a#b` and
// `#abc` keep their hash while `a #b` and ` #b` do not.
func stripTrailingComment(s string) string {
	for i := 1; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}
		if s[i-1] == ' ' || s[i-1] == '\t' {
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

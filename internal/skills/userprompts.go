package skills

// Starter prompt templates: materialized into the user's slash-command
// directory (the config dir) on first run when missing — create-command is
// the workflow behind /help's and the dropdown's "saved commands" story. A
// seeded file is the user's from then on: an edited or replaced one is never
// overwritten, and deleting one brings the starter back (it is an example,
// not state).

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed builtin-prompts
var builtinPromptsFS embed.FS

// promptsDirPlaceholder in a starter template is substituted with the actual
// prompts directory (home-abbreviated when possible), so the file is correct
// on an XDG layout where ~/.config/moca would be the wrong path.
const promptsDirPlaceholder = "{{prompts_dir}}"

// SeedUserPrompts writes the embedded starter prompt templates into dir when
// they are missing; an existing file is never touched. Best-effort by design:
// a read-only config dir must not stop the TUI, so callers report a failure
// as a warning rather than aborting.
func SeedUserPrompts(dir string) error {
	ents, err := fs.ReadDir(builtinPromptsFS, "builtin-prompts")
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		b, err := fs.ReadFile(builtinPromptsFS, "builtin-prompts/"+e.Name())
		if err != nil {
			return err
		}
		b = bytes.ReplaceAll(b, []byte(promptsDirPlaceholder), []byte(displayPath(dir)))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		// O_EXCL: two processes seeding on first use — the loser keeps the
		// winner's file, never a torn one.
		f, err := os.OpenFile(filepath.Join(dir, e.Name()), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		_, werr := f.Write(b)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return werr
		}
	}
	return nil
}

// displayPath abbreviates a path under the user's home with ~/.
func displayPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// forkNameRe is the directory-name shape a fork may target or create. Built-in
// names are simple (`review`, `debt`), so the rule is deliberately narrow: it
// is also the guard that keeps a name from walking out of the skills dirs.
var forkNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// BuiltinNames lists the built-in skills found in dir, sorted. A directory
// without a SKILL.md is not a skill and is skipped.
func BuiltinNames(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if fi, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err != nil || fi.IsDir() {
			continue
		}
		out = append(out, e.Name())
	}
	slices.Sort(out)
	return out
}

// Fork copies the built-in skill name from builtinDir into dir, so the user
// owns an editable copy. Discovery reads the user dirs before the built-in one
// and keeps the first name it sees (SPECS §3.5), so the copy shadows the
// built-in from the next session on — without touching what every other user
// gets with the binary.
//
// It refuses an unknown name and never overwrites: an existing skill is the
// user's, and a fork that quietly replaced it would be the one destructive
// thing this command could do. A partial copy is removed on failure.
func Fork(builtinDir, dir, name string) (string, error) {
	if !forkNameRe.MatchString(name) || strings.Contains(name, "..") {
		return "", fmt.Errorf("%q is not a skill directory name (lowercase, digits, '.', '_' or '-')", name)
	}
	src := filepath.Join(builtinDir, name)
	if fi, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil || fi.IsDir() {
		if names := BuiltinNames(builtinDir); len(names) > 0 {
			return "", fmt.Errorf("%q is not a built-in skill (built-in: %s)", name, strings.Join(names, ", "))
		}
		return "", fmt.Errorf("%q is not a built-in skill", name)
	}
	dst := filepath.Join(dir, name)
	if _, err := os.Lstat(dst); err == nil {
		return "", fmt.Errorf("%s already exists — it is yours; delete or rename it to fork again", dst)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := copySkillTree(src, dst); err != nil {
		os.RemoveAll(dst)
		return "", err
	}
	return dst, nil
}

// copySkillTree copies a skill directory: regular files and directories only.
// A symlink is refused rather than followed — a fork must not become a way to
// copy something from outside the bundle into the user's skills dir.
func copySkillTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.Type()&os.ModeSymlink != 0:
			return fmt.Errorf("%s is a symlink; a skill is copied as plain files", p)
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			fi, err := d.Info()
			if err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fi.Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		default:
			return fmt.Errorf("%s is not a regular file", p)
		}
	})
}

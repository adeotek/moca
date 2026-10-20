package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PrunedSkills reports what a hygiene pass changed.
type PrunedSkills struct {
	RulesRemoved  int
	SkillsRemoved []string
	Touched       []string
	PinnedAged    []string // skill names holding a (user, …) rule older than SkillPinAgeNoticeDays — named in the startup warning, never acted on
}

// PruneAutoSkills drops expired, dated ×1 rules from auto skills under dir
// and removes the SKILL.md of an auto skill left with nothing but
// frontmatter and headings. Hand-written skills are never touched, and no
// line hygiene cannot date is ever dropped. dryRun reports what it would
// change and writes nothing (headless starts — M6; the PruneBuiltins(dataDir,
// dryRun) precedent). Injectable clock; it attempts every skill and joins the
// errors.
func PruneAutoSkills(dir string, now time.Time, dryRun bool) (PrunedSkills, error) {
	var rep PrunedSkills
	var errs []error
	ents, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return rep, nil
	}
	if err != nil {
		return rep, err
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") || e.Type()&fs.ModeSymlink != 0 || !e.IsDir() {
			continue
		}
		dirName := e.Name()
		p := filepath.Join(dir, dirName, "SKILL.md")
		data, err := os.ReadFile(p)
		if err != nil {
			continue // no SKILL.md: not a skill
		}
		fm, _, err := ParseFrontmatter(data)
		if err != nil || strings.TrimSpace(fm[SkillAutoKey]) != "true" {
			continue
		}
		name := strings.TrimSpace(fm["name"])
		if name == "" {
			name = dirName
		}
		off, err := BodyOffset(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		// The frontmatter is kept byte for byte: re-serializing the map would
		// drop nested keys, comments and key order.
		head, body := data[:off], string(data[off:])
		var kept []string
		removed := 0
		for _, line := range splitBodyLines(body) {
			if r, ok := ParseRule(strings.TrimRight(line, "\r\n")); ok && r.HasProv {
				// Calendar days, not elapsed hours (N-14): a rule is dated by
				// the host's calendar day and parses as UTC midnight, so
				// now.Sub(r.Last) made a rule written on 2026-09-20 expire at
				// 00:00 UTC on the 20th — 21:00 on the 19th for a reader in
				// EEST — and the same rule aged differently at noon and at
				// midnight. The dates the file and the clock show are compared.
				age := dayDiff(r.Last, now)
				switch {
				case r.Pinned:
					if age > SkillPinAgeNoticeDays {
						if !containsStr(rep.PinnedAged, name) {
							rep.PinnedAged = append(rep.PinnedAged, name)
						}
					}
				case r.Seen <= 1 && age > SkillExpiryDays:
					removed++
					continue
				}
			}
			kept = append(kept, line)
		}
		rest := strings.Join(kept, "")
		if IsEmptyBody([]byte(rest)) {
			// An auto skill left with nothing but headings: remove its
			// SKILL.md, then its directory — which fails harmlessly (and is
			// ignored) when the user keeps other files there. Never
			// RemoveAll: those files are the user's.
			if !dryRun {
				if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
					errs = append(errs, err)
					continue
				}
				os.Remove(filepath.Join(dir, dirName))
			}
			rep.RulesRemoved += removed
			// N-13: a skill that lost rules here goes in Touched as well.
			// Without it the summary counted them under "removed N stale rules
			// from <others>" — or dropped the count entirely when no other
			// skill was touched, leaving "removed empty X" for a skill that
			// lost two. A skill that was already empty (removed == 0) was not
			// touched by this pass and is named only by "removed empty X".
			if removed > 0 {
				rep.Touched = append(rep.Touched, name)
			}
			rep.SkillsRemoved = append(rep.SkillsRemoved, name)
			continue
		}
		if removed > 0 {
			if !dryRun {
				if err := rewriteBody(p, head, rest); err != nil {
					errs = append(errs, err)
					continue
				}
			}
			rep.RulesRemoved += removed
			rep.Touched = append(rep.Touched, name)
		}
	}
	return rep, errors.Join(errs...)
}

// splitBodyLines splits a body into lines, each keeping its own ending (CRLF
// included) so a rewrite preserves the file's line endings.
func splitBodyLines(b string) []string {
	var out []string
	for len(b) > 0 {
		i := strings.IndexByte(b, '\n')
		if i < 0 {
			out = append(out, b)
			break
		}
		out = append(out, b[:i+1])
		b = b[i+1:]
	}
	return out
}

// rewriteBody replaces p's content with head+body through a temp file in the
// same directory, keeping the previous mode (an atomic rename: a reader never
// sees a half-written SKILL.md).
func rewriteBody(p string, head []byte, body string) error {
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(p); err == nil {
		mode = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".tmp-prune-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() {
		f.Close()
		os.Remove(tmp)
	}
	if _, err := f.Write(head); err != nil {
		cleanup()
		return err
	}
	if _, err := f.WriteString(body); err != nil {
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// dayDiff is the calendar-day distance between the days two instants fall on,
// ignoring their times and zones: a rule dated 2026-09-20 and a clock reading
// 2026-10-20T23:30+03:00 are 30 days apart, whatever the UTC offset does to
// the elapsed hours.
func dayDiff(from, to time.Time) int {
	fy, fm, fd := from.Date()
	ty, tm, td := to.Date()
	a := time.Date(fy, fm, fd, 0, 0, 0, 0, time.UTC)
	b := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a) / (24 * time.Hour))
}

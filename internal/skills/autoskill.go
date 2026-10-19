package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/secret"
)

// Caps and markers for self-managed project skills (SPECS §12). Every cap
// lives here and nowhere else; each has a pinning test. Lengths are runes
// (utf8.RuneCountInString), never bytes.
const (
	SkillBodyCap          = 1500
	SkillMaxRules         = 12
	SkillRuleLineCap      = 240
	SkillWhenCap          = 80
	SkillDescriptionCap   = 160
	SkillMaxAuto          = 8
	SkillCreationsPerRun  = 2
	SkillNamePrefix       = "moca-"
	SkillAutoKey          = "auto"
	SkillExpiryDays       = 90
	SkillPinAgeNoticeDays = 365 // hygiene surfaces (user, …) rules older than this — never acts
)

// SkillFacts is what a skill file says about itself.
type SkillFacts struct {
	Name        string // frontmatter name, or the directory name when absent
	Description string
	Auto        bool
	BodyRunes   int
	Rules       int
	Empty       bool // the body is only headings and blank lines (what hygiene removes)
}

// AutoCount is how many non-empty auto skills a project already tracks.
type AutoCount struct {
	Auto  int
	Names []string
}

// WriteContext is what the guard knows about a write beyond its bytes.
type WriteContext struct {
	DirName     string    // the skill's directory name under .moca/skills
	Prev        []byte    // the file's current bytes; nil when the write creates it
	Others      AutoCount // non-empty auto skills under the dir, excluding this one
	UserSignal  bool      // set only by Agent.Remember (Decision 11)
	Named       bool      // the run's typed text names this skill (Decision 15; skills.Named)
	Today       time.Time // the host clock at write time (the provenance check, C3)
	SessionDate time.Time // the session's start date (long-running sessions)
}

var (
	ruleLineRe = regexp.MustCompile(`^- \*\*(.+?)\*\*:\s*(.+)$`)
	// An ASCII x is accepted for × — weaker models type it, and refusing
	// over the glyph teaches nothing.
	provRe = regexp.MustCompile(`\(([×x][0-9]+|user), ([0-9]{4}-[0-9]{2}-[0-9]{2})\)\s*$`)
	// Built from SkillNamePrefix (L9): one source for the prefix.
	nameRe = regexp.MustCompile(`^` + regexp.QuoteMeta(SkillNamePrefix) + `[a-z0-9]+(-[a-z0-9]+)*$`)
)

// ParseSkill reads what a skill file says about itself. dirName is the
// skill's directory name (Discover's rule: frontmatter name wins, directory
// is the fallback).
func ParseSkill(data []byte, dirName string) (SkillFacts, error) {
	fm, body, err := ParseFrontmatter(data)
	if err != nil {
		return SkillFacts{}, err
	}
	name := strings.TrimSpace(fm["name"])
	if name == "" {
		name = dirName
	}
	return SkillFacts{
		Name:        name,
		Description: strings.TrimSpace(fm["description"]),
		Auto:        strings.TrimSpace(fm[SkillAutoKey]) == "true",
		BodyRunes:   utf8.RuneCount(body),
		Rules:       countRules(body),
		Empty:       IsEmptyBody(body),
	}, nil
}

// IsEmptyBody reports a body carrying nothing but headings and blank lines —
// what hygiene removes. Prose is never "empty": a paragraph has content
// hygiene cannot date, so it stays (C2 left such a body reachable only by a
// hand edit — the guard refuses prose — and hygiene does not delete what it
// cannot date).
func IsEmptyBody(body []byte) bool {
	for _, l := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return false
	}
	return true
}

// countRules counts well-formed rule lines in a body.
func countRules(body []byte) int {
	n := 0
	for _, l := range strings.Split(string(body), "\n") {
		if _, ok := ParseRule(l); ok {
			n++
		}
	}
	return n
}

// Rule is one parsed rule line.
type Rule struct {
	Line    string // the full line, as it must be written back
	When    string
	Text    string
	Seen    int       // provenance ×N; 0 for a user rule
	Last    time.Time // provenance date
	Pinned  bool      // provenance "user"
	HasProv bool      // a well-formed, datable provenance
}

// ParseRule parses one rule line. ok is false for lines that are not rules.
func ParseRule(line string) (Rule, bool) {
	line = strings.TrimRight(line, "\r")
	m := ruleLineRe.FindStringSubmatch(line)
	if m == nil {
		return Rule{}, false
	}
	r := Rule{Line: line, When: strings.TrimSpace(m[1]), Text: strings.TrimSpace(m[2])}
	p := provRe.FindStringSubmatch(r.Text)
	if p == nil {
		return r, true
	}
	t, err := time.Parse("2006-01-02", p[2])
	if err != nil {
		return r, true // an undatable provenance is no provenance
	}
	r.Last, r.HasProv = t, true
	r.Text = strings.TrimSpace(strings.TrimSuffix(r.Text, p[0]))
	if p[1] == "user" {
		r.Pinned = true
	} else if n, err := strconv.Atoi(strings.TrimLeft(p[1], "×x")); err == nil {
		r.Seen = n
	} else {
		r.HasProv = false
	}
	return r, true
}

// CountAutoSkills counts non-empty auto skills under dir, skipping the skill
// DIRECTORY named excludeDir — by name, not by path: on a case-insensitive
// filesystem the target can be spelled .moca/Skills/x/skill.md, and a path
// comparison would count the target against itself at the cap (H11).
// Hidden and symlinked entries are skipped, as Discover does; a missing dir
// is zero, not an error.
func CountAutoSkills(dir, excludeDir string) (AutoCount, error) {
	var c AutoCount
	ents, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") || e.Type()&fs.ModeSymlink != 0 || !e.IsDir() {
			continue
		}
		if excludeDir != "" && strings.EqualFold(e.Name(), excludeDir) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		f, err := ParseSkill(data, e.Name())
		if err != nil || !f.Auto || f.Empty {
			continue
		}
		c.Auto++
		c.Names = append(c.Names, f.Name)
	}
	return c, nil
}

// Named reports whether any of the run's typed texts names a skill: its
// frontmatter name or its directory name, case-insensitive, as a whole word —
// [A-Za-z0-9_-] are word characters, so "deploy" does not match "deploy-tool"
// or "redeploy" (Decision 15). Empty typed → false. Pure; the guard passes
// Env.SkillTyped.
func Named(typed []string, name, dir string) bool {
	for _, t := range typed {
		for _, n := range []string{name, dir} {
			if n != "" && containsWord(t, n) {
				return true
			}
		}
	}
	return false
}

// containsWord reports whether w occurs in s as a whole word (a word byte is
// [A-Za-z0-9_-], so hyphens glue "deploy-tool" together).
func containsWord(s, w string) bool {
	s, w = strings.ToLower(s), strings.ToLower(w)
	for i := 0; ; {
		j := strings.Index(s[i:], w)
		if j < 0 {
			return false
		}
		j += i
		before := j == 0 || !wordByte(s[j-1])
		after := j+len(w) >= len(s) || !wordByte(s[j+len(w)])
		if before && after {
			return true
		}
		i = j + 1
	}
}

func wordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-'
}

// ValidateSkill is the whole write contract for <skills>/<dir>/SKILL.md
// (SPECS §11/§12). Every error starts with "refused: " and ends with the way
// out — the model reads it. Other files under the skills dir are the
// guard's call (task 4), not this function's.
func ValidateSkill(data []byte, wc WriteContext) error { return validate(data, wc, false) }

// ValidateStored re-validates a stored auto skill at discovery — the
// load-time backstop (Decision 12, C1): the format, caps, name == directory,
// single-file and secret-scan checks, but never the per-run creation cap, the
// user-signal gate or the per-change provenance rules (a stored file has no
// previous version to compare against). skillDir is the skills root, dirName
// the skill's directory; SKILL.md is read here, and the directory's contents
// checked (the single-file rule). A refusal drops the skill from the listing.
func ValidateStored(skillDir, dirName string) error {
	dir := filepath.Join(skillDir, dirName)
	p := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("%s: %v", p, err)
	}
	if err := validate(data, WriteContext{DirName: dirName}, true); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("%s: %v", p, err)
	}
	for _, e := range ents {
		// Hidden entries are skipped, as every skills walker skips them: an OS
		// or editor file (.DS_Store, a swap file) or a crashed hygiene temp
		// (.tmp-prune-*) must not drop a valid skill from the listing. The
		// write guard still refuses any companion the agent itself writes.
		if e.Name() == "SKILL.md" || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		return fmt.Errorf("%s: auto skills are single-file — %s must not be in this directory", p, e.Name())
	}
	return nil
}

// validate is ValidateSkill; stored skips every per-change rule (the checks
// that need a previous version, the run's signals and the per-run cap).
func validate(data []byte, wc WriteContext, stored bool) error {
	fm, body, err := ParseFrontmatter(data)
	if err != nil {
		return fmt.Errorf("refused: %v — a skill file starts with a --- block (name, description, auto: true) closed by a --- line", err)
	}
	name := strings.TrimSpace(fm["name"])
	if name == "" {
		name = wc.DirName
	}
	desc := strings.TrimSpace(fm["description"])
	auto := strings.TrimSpace(fm[SkillAutoKey]) == "true"
	runes := utf8.RuneCount(body)

	var prev SkillFacts
	havePrev := wc.Prev != nil
	if havePrev {
		prev, err = ParseSkill(wc.Prev, wc.DirName)
		if err != nil {
			return fmt.Errorf("refused: the existing %s/SKILL.md cannot be parsed as a skill (%v) — fix it by hand; the agent does not overwrite a file it cannot read", wc.DirName, err)
		}
	}

	if desc == "" {
		return fmt.Errorf("refused: %s has no description — it is the line every later session sees; write one short sentence", name)
	}
	if auto && (!havePrev || desc != prev.Description) {
		if n := utf8.RuneCountInString(desc); n > SkillDescriptionCap {
			return fmt.Errorf("refused: %s's description is %d chars (cap %d) — make it one short sentence", name, n, SkillDescriptionCap)
		}
	}
	if class, ok := newContentSecret(data, wc.Prev); ok {
		return fmt.Errorf("refused: the content looks like a credential (%s) — never record secrets, tokens or URLs with credentials in a skill; name the environment variable instead; the file is plain text in the repository", class)
	}

	if auto {
		baseLine := 1
		if off, oerr := BodyOffset(data); oerr == nil {
			baseLine = strings.Count(string(data[:off]), "\n") + 1
		}
		if err := strictBody(body, baseLine); err != nil {
			return err
		}
	}

	// Hand-written skills (prev not auto): a named edit is checked for the
	// description and credentials only — a user's own file may be long and
	// stay free prose.
	if havePrev && !prev.Auto {
		if !auto {
			if !wc.Named {
				return fmt.Errorf("refused: %s is a hand-written skill — edit it only when the user's request names it; record learned rules in a moca-<domain> skill instead", name)
			}
			return nil
		}
		if !wc.Named {
			return fmt.Errorf("refused: %s is a hand-written skill — edit it only when the user's request names it; record learned rules in a moca-<domain> skill instead", name)
		}
		return fmt.Errorf("refused: %s is a hand-written skill — leave its frontmatter as it is and record learned rules in a moca-<domain> skill", name)
	}
	if havePrev && prev.Auto && !auto {
		return fmt.Errorf("refused: %s is an auto skill — keep auto: true in its frontmatter; only the user removes the marker, by hand", name)
	}
	if !havePrev && !auto {
		return fmt.Errorf("refused: with self-managed skills on, a skill you create under .moca/skills must be an auto skill — add auto: true to the frontmatter and name it moca-<domain>; a hand-written skill is the user's to create")
	}

	// Auto skills from here on.
	if !nameRe.MatchString(name) || name != wc.DirName {
		return fmt.Errorf("refused: auto skills are named moca-<domain> (lowercase letters, digits, hyphens) and the frontmatter name must equal the directory name (got name %q in directory %q) — rename so both read moca-<domain>", name, wc.DirName)
	}
	if !stored {
		if runes > SkillBodyCap && (!havePrev || runes > prev.BodyRunes) {
			return fmt.Errorf("refused: this auto skill is %d chars (cap %d) — shorten the least useful rule, or split a second moca-* skill", runes, SkillBodyCap)
		}
		rules := countRules(body)
		if rules > SkillMaxRules && (!havePrev || rules > prev.Rules) {
			return fmt.Errorf("refused: %d rule lines (cap %d) — merge the least useful pair, or move a rule into another moca-* skill", rules, SkillMaxRules)
		}
	} else {
		if runes > SkillBodyCap {
			return fmt.Errorf("refused: this auto skill is %d chars (cap %d)", runes, SkillBodyCap)
		}
		if rules := countRules(body); rules > SkillMaxRules {
			return fmt.Errorf("refused: %d rule lines (cap %d)", rules, SkillMaxRules)
		}
	}

	rules := parseRules(body)
	if !stored {
		if !havePrev && len(rules) == 0 {
			return fmt.Errorf("refused: a new auto skill needs at least one rule line — a skill with no rules is a placeholder and still takes a listing line; record the rule you have, or skip it")
		}
		prevRules := map[string]Rule{}
		if havePrev {
			_, prevBody, _ := ParseFrontmatter(wc.Prev)
			for _, r := range parseRules(prevBody) {
				prevRules[ruleKey(r)] = r
			}
		}
		added := newLines(data, wc.Prev)
		var today string
		if !wc.Today.IsZero() {
			today = wc.Today.Format("2006-01-02")
		}
		allowed := map[string]bool{}
		if !wc.Today.IsZero() {
			allowed[wc.Today.AddDate(0, 0, -1).Format("2006-01-02")] = true
			allowed[wc.Today.Format("2006-01-02")] = true
			allowed[wc.Today.AddDate(0, 0, 1).Format("2006-01-02")] = true
		}
		if !wc.SessionDate.IsZero() {
			allowed[wc.SessionDate.Format("2006-01-02")] = true
		}
		for _, r := range rules {
			old, known := prevRules[ruleKey(r)]
			if lineIn(added, r.Line) {
				if n := utf8.RuneCountInString(r.Line); n > SkillRuleLineCap {
					return fmt.Errorf("refused: a rule line is %d chars (cap %d) — a rule is one line: \"- **<when>**: <rule> (×N, YYYY-MM-DD)\"", n, SkillRuleLineCap)
				}
				if n := utf8.RuneCountInString(r.When); n > SkillWhenCap {
					return fmt.Errorf("refused: a rule's <when> is %d chars (cap %d) — name the situation in a few words and put the detail in the rule", n, SkillWhenCap)
				}
			}
			if !known {
				if !r.HasProv || r.Seen != 1 && !r.Pinned {
					return fmt.Errorf("refused: a new rule starts at (×1, %s) — the count records separate sessions that confirmed it, and the date is the day you observed it", today)
				}
				if !allowed[r.Last.Format("2006-01-02")] {
					return fmt.Errorf("refused: the date %s is not today (%s) — the provenance date is the day you observed the rule", r.Last.Format("2006-01-02"), today)
				}
				if r.Pinned && !wc.UserSignal {
					return fmt.Errorf("refused: (user, …) marks a rule the user stated via /remember, and this run was not started with it — use (×1, YYYY-MM-DD) for what you observed, or ask the user to run /remember")
				}
				continue
			}
			if r.Line == old.Line || r.Pinned && old.Pinned && r.Last.Equal(old.Last) && old.HasProv {
				continue
			}
			if !old.Pinned && old.HasProv && r.HasProv && !r.Pinned && old.Seen == 1 && r.Seen == 2 && !r.Last.Before(old.Last) {
				continue
			}
			return fmt.Errorf("refused: a confirmed rule goes (×1, %s) → (×2, %s) once and then never changes — rewrite its text instead if the rule itself changed", old.Last.Format("2006-01-02"), today)
		}
	}

	// The count cap checks the CHANGE (H6): a repo already over the cap stays
	// editable, and an emptied skill frees its slot.
	if !stored && !IsEmptyBody(body) {
		already := havePrev && prev.Auto && !prev.Empty
		if !already && wc.Others.Auto+1 > SkillMaxAuto {
			return fmt.Errorf("refused: this project already has %d auto skills (%s) — merge into an existing one, or empty one (an auto skill left with only headings does not count, and the next start removes it)", wc.Others.Auto, nameList(wc.Others.Names))
		}
	}
	return nil
}

// strictBody enforces the rule-only body of an auto skill (C2). baseLine is
// the file line number of the body's first line.
func strictBody(body []byte, baseLine int) error {
	for i, l := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if _, ok := ParseRule(l); ok {
			continue
		}
		return fmt.Errorf("refused: line %d is not a rule — an auto skill body holds only headings and rule lines \"- **<when>**: <rule> (×1, YYYY-MM-DD)\"; put explanation inside the rule", baseLine+i)
	}
	return nil
}

// parseRules returns every well-formed rule line of a body.
func parseRules(body []byte) []Rule {
	var out []Rule
	for _, l := range strings.Split(string(body), "\n") {
		if r, ok := ParseRule(l); ok {
			out = append(out, r)
		}
	}
	return out
}

// ruleKey identifies a rule by its text without its provenance: a provenance
// bump keeps the key, a rewritten text is a new rule.
func ruleKey(r Rule) string { return r.When + "\x00" + r.Text }

// lineIn reports whether line is in the added-lines list.
func lineIn(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

// newLines is the line-multiset difference data\prev: the content a write
// adds. A nil prev means every line is new.
func newLines(data, prev []byte) []string {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if prev == nil {
		return lines
	}
	counts := map[string]int{}
	for _, l := range strings.Split(strings.ReplaceAll(string(prev), "\r\n", "\n"), "\n") {
		counts[l]++
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if counts[l] > 0 {
			counts[l]--
			continue
		}
		out = append(out, l)
	}
	return out
}

// newContentSecret scans only the content a write adds (H10): an edit that
// leaves a documented example alone stays allowed.
func newContentSecret(data, prev []byte) (string, bool) {
	for _, l := range newLines(data, prev) {
		if class, ok := secret.Match(l); ok {
			return class, true
		}
	}
	return "", false
}

// nameList renders up to three skill names for the count-cap refusal.
func nameList(names []string) string {
	if len(names) == 0 {
		return "…"
	}
	if len(names) > 3 {
		return strings.Join(names[:3], ", ") + ", …"
	}
	return strings.Join(names, ", ")
}

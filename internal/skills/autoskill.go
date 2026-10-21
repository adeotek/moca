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
	// Executed is the session's own shell calls, split into normalized
	// segments (tools.ExecutedSegments), for the command-claim rule: a rule
	// that cites a command in backticks is a promise the next session acts on
	// without checking, so a line this write ADDS may only cite a command the
	// session actually ran. Nil in ValidateStored (a stored file has no run).
	Executed []string
}

var (
	ruleLineRe = regexp.MustCompile(`^- \*\*(.+?)\*\*:\s*(.+)$`)
	// commandTokenRe/cmdHeadRe find a rule's cited commands (§15 clause (c)
	// mirrored in the guard): the same shape list the eval checkers score, so
	// the guard and the corpus agree on what counts as a command claim.
	commandTokenRe = regexp.MustCompile("`([^`]+)`")
	cmdHeadRe      = regexp.MustCompile(`^(\./|make |go |pytest|npm |cargo |dotnet |bash |sh |docker |python3? |node |yarn |pnpm |mvn |gradle )`)
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
	// The description is the one line a later session sees before choosing to
	// read the body (SPECS §12.1/§15): a skill whose rule cites a command must
	// say which one, or the listing steers nothing and the run re-derives the
	// command from the Makefile. Measured: `skills` scored 0/6 with
	// descriptions like "How the Go suite runs here" — and when the first cut
	// exempted an unchanged description, session 1 fixed the stale body rule
	// and left the steering-less description standing, so session 2 failed (b)
	// exactly as before (v073-cd2). The check is write-time only (never
	// `stored`): at discovery a pre-release skill must still be listed —
	// dropping it would silently blank every existing listing — but a write
	// that keeps a description that hides the body's command is refused, and
	// fixing both in one write is the way out.
	if auto && !stored {
		if cmd, ok := citedCommand(string(body)); ok && !namesCommand(desc, cmd) {
			return fmt.Errorf("refused: %s's description names no command, and it is the only line the next session sees — say how this repo is checked (e.g. \"... with %s\"), or state the rule without a backticked command", name, cmd)
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
	if stored {
		// The load-time re-validation is the state check: a hand edit or an
		// interpreter write can leave a contentless rule or ×N outside {1,2}
		// (§12.1's grammar), and the skill must not reach a later session's
		// listing. The write path checks CHANGES instead (H6): its added-line
		// branch below refuses the same shapes only when the write introduces
		// or rewrites them, so a stored file with a bad line stays editable.
		for _, r := range rules {
			if r.Text == "" {
				return fmt.Errorf("refused: a rule line needs text besides its provenance — write \"- **<when>**: <rule> (×N, YYYY-MM-DD)\"")
			}
			if r.HasProv && !r.Pinned && (r.Seen < 1 || r.Seen > 2) {
				return fmt.Errorf("refused: a rule's provenance count is ×1 or ×2 (got ×%d) — a ×2 line is final; rewrite the rule's text if the rule itself changed", r.Seen)
			}
		}
	}
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
				if r.Text == "" {
					return fmt.Errorf("refused: a rule line needs text besides its provenance — write \"- **<when>**: <rule> (×N, YYYY-MM-DD)\"")
				}
				if n := utf8.RuneCountInString(r.Line); n > SkillRuleLineCap {
					return fmt.Errorf("refused: a rule line is %d chars (cap %d) — a rule is one line: \"- **<when>**: <rule> (×N, YYYY-MM-DD)\"", n, SkillRuleLineCap)
				}
				if n := utf8.RuneCountInString(r.When); n > SkillWhenCap {
					return fmt.Errorf("refused: a rule's <when> is %d chars (cap %d) — name the situation in a few words and put the detail in the rule", n, SkillWhenCap)
				}
				// Clause (c): a rule may only cite a command this session ran.
				// A `/remember` rule is the user's own words — the user may
				// assert a command nobody ran here, and refusing it would make
				// the feature refuse its own input.
				if !wc.UserSignal {
					if tok := unprovenCommand(r.Text, wc.Executed); tok != "" {
						return fmt.Errorf("refused: this rule quotes `%s`, and this session never ran it — a cited command is a promise the next session acts on without checking; run it first, or state the rule without backticks", tok)
					}
				}
			}
			if !r.HasProv {
				// N-7: a provenance the model meant to write but did not end the
				// line with ("… (×1, 2026-10-20) extra words") parsed as no
				// provenance at all, and the refusal never said what it saw.
				if tail, ok := provAttempt(r.Text); ok {
					return fmt.Errorf("refused: the provenance must end the rule line — found %q; write \"- **<when>**: <rule> (×N, YYYY-MM-DD)\" with nothing after the closing parenthesis", tail)
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
			// D6: a known rule (its text is unchanged — ruleKey includes it) may
			// change its provenance in exactly these ways. Everything else is a
			// refusal that names the attempt, with no zero date on either side.
			switch {
			case !old.HasProv && r.HasProv && !r.Pinned && r.Seen == 1,
				!old.HasProv && r.HasProv && r.Pinned && wc.UserSignal,
				old.HasProv && !old.Pinned && r.Pinned && wc.UserSignal:
				// Back-fill to (×1, today), or pin to (user, today) under
				// /remember. The date is the host clock's: a (×1, <future>) would
				// pass on the model's word alone.
				if !allowed[r.Last.Format("2006-01-02")] {
					return fmt.Errorf("refused: the date %s is not today (%s) — %s goes %s → (×1, %s)", r.Last.Format("2006-01-02"), today, provLabel(old), provLabel(old), today)
				}
				continue
			case old.HasProv && !old.Pinned && old.Seen == 1 && r.HasProv && !r.Pinned && r.Seen == 2 && !r.Last.Before(old.Last):
				if !allowed[r.Last.Format("2006-01-02")] {
					return fmt.Errorf("refused: the date %s is not today (%s) — a confirmed rule goes %s → (×2, %s)", r.Last.Format("2006-01-02"), today, provLabel(old), today)
				}
				continue
			}
			return fmt.Errorf("refused: %s → %s is not an allowed provenance change — a rule may be back-filled to (×1, %s) once, confirmed as (×2, %s) once, or pinned to (user, %s) by /remember; a ×2 rule is final and a (user, …) rule is the user's — rewrite the rule's text instead if the rule itself changed", provLabel(old), provLabel(r), today, today, today)
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

// provLabel renders a rule's provenance for a refusal. A rule with none says
// so: formatting a zero time printed "0001-01-01", which named a date no rule
// was ever on (N-7).
func provLabel(r Rule) string {
	switch {
	case !r.HasProv:
		return "no provenance"
	case r.Pinned:
		return "(user, " + r.Last.Format("2006-01-02") + ")"
	default:
		return fmt.Sprintf("(×%d, %s)", r.Seen, r.Last.Format("2006-01-02"))
	}
}

// provAttempt finds a provenance marker that did not parse at the end of the
// line, and returns it with whatever trails it — the text the refusal quotes.
func provAttempt(text string) (string, bool) {
	for _, marker := range []string{"(×", "(x", "(user,"} {
		if i := strings.LastIndex(text, marker); i >= 0 {
			return strings.TrimSpace(text[i:]), true
		}
	}
	return "", false
}

// citedCommand returns the first command-shaped backticked token in body,
// reduced to the first two words — the form a description is expected to name
// (`make test`, `go test`, `./scripts/bootstrap.sh`), and the same pair the
// eval corpus compares (SPECS §15 clause (b)).
func citedCommand(body string) (string, bool) {
	for _, m := range commandTokenRe.FindAllStringSubmatch(body, -1) {
		f := strings.Fields(m[1])
		if len(f) == 0 || !cmdHeadRe.MatchString(strings.Join(f, " ")) {
			continue
		}
		if len(f) > 2 {
			f = f[:2]
		}
		return strings.Join(f, " "), true
	}
	return "", false
}

// namesCommand reports whether desc names cmd, whitespace- and case-insensitive:
// "How the suite runs — make test with CALC_STRICT=1" names "make test".
func namesCommand(desc, cmd string) bool {
	d := strings.ToLower(strings.Join(strings.Fields(desc), " "))
	c := strings.ToLower(strings.Join(strings.Fields(cmd), " "))
	if c == "" {
		return true
	}
	if strings.Contains(d, c) {
		return true
	}
	// Single-word commands (`pytest`) may be written with a trailing
	// call form: "run pytest" contains it; "pytests" must not.
	if !strings.Contains(c, " ") {
		for _, w := range strings.FieldsFunc(d, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '/' || r == '-' || r == '_')
		}) {
			if w == c {
				return true
			}
		}
	}
	return false
}

// unprovenCommand returns the first command text cites in backticks that
// executed does not satisfy. A cited token and a run segment satisfy each other
// when either is a prefix of the other, so a rule's `go test ./...` matches the
// session's `go test ./... -count=1` and a `make test` rule matches
// `make test CALC_STRICT=1`. Tokens that are not command-shaped (paths,
// identifiers, environment variables) are not claims about running anything
// and are ignored.
func unprovenCommand(text string, executed []string) string {
	for _, m := range commandTokenRe.FindAllStringSubmatch(text, -1) {
		t := strings.Join(strings.Fields(m[1]), " ")
		if !cmdHeadRe.MatchString(t) {
			continue
		}
		ok := false
		for _, e := range executed {
			if strings.HasPrefix(e, t) || strings.HasPrefix(t, e) {
				ok = true
				break
			}
		}
		if !ok {
			return t
		}
	}
	return ""
}

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

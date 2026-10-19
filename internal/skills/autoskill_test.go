package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/secret"
)

var testToday = time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)

func autoSkill(name, desc, body string) []byte {
	return []byte("---\nname: " + name + "\ndescription: " + desc + "\nauto: true\n---\n" + body)
}

func plainSkill(name, desc, body string) []byte {
	return []byte("---\nname: " + name + "\ndescription: " + desc + "\n---\n" + body)
}

func wc(dir string) WriteContext {
	return WriteContext{DirName: dir, Today: testToday, SessionDate: testToday}
}

func TestParseSkill(t *testing.T) {
	f, err := ParseSkill(autoSkill("moca-testing", "How tests run", "# Head\n\n- **Run**: `go test ./...` (×1, 2026-10-18)\n"), "moca-testing")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Auto || f.Name != "moca-testing" || f.Description != "How tests run" || f.Rules != 1 {
		t.Fatalf("%+v", f)
	}
	if f.Empty {
		t.Fatal("a body with a rule is not empty")
	}
	// auto absent
	if f, _ := ParseSkill(plainSkill("x", "d", "body\n"), "x"); f.Auto {
		t.Fatal("auto absent must not set Auto")
	}
	// a nested map is skipped by ParseFrontmatter, never read as the marker
	nested := []byte("---\nname: moca-x\ndescription: d\nmeta: {auto: true}\n---\n")
	if f, err := ParseSkill(nested, "moca-x"); err != nil || f.Auto {
		t.Fatalf("nested auto: %+v %v", f, err)
	}
	// missing description
	if f, _ := ParseSkill(autoSkill("moca-x", "", ""), "moca-x"); f.Description != "" {
		t.Fatal("missing description must read empty")
	}
	// dir-name fallback
	if f, _ := ParseSkill([]byte("---\ndescription: d\n---\n"), "moca-dir"); f.Name != "moca-dir" {
		t.Fatalf("fallback name: %q", f.Name)
	}
	// CRLF tolerated, and × counts as one rune even though it is two bytes
	crlf := []byte("---\r\nname: moca-x\r\ndescription: d\r\nauto: true\r\n---\r\n- **When**: rule (×1, 2026-10-18)\r\n")
	f, err = ParseSkill(crlf, "moca-x")
	if err != nil || !f.Auto || f.Rules != 1 {
		t.Fatalf("crlf: %+v %v", f, err)
	}
	wantRunes := []rune("- **When**: rule (×1, 2026-10-18)\n")
	if f.BodyRunes != len(wantRunes) {
		t.Fatalf("body runes = %d (× is one rune), want %d", f.BodyRunes, len(wantRunes))
	}
	// headings only → Empty
	if f, _ := ParseSkill(autoSkill("moca-x", "d", "# A\n\n## B\n"), "moca-x"); !f.Empty {
		t.Fatal("headings-only body must be Empty")
	}
	// prose is not empty (hygiene never deletes what it cannot date)
	if f, _ := ParseSkill(autoSkill("moca-x", "d", "# A\n\nsome prose.\n"), "moca-x"); f.Empty {
		t.Fatal("a prose body must not be Empty")
	}
}

func TestParseRule(t *testing.T) {
	r, ok := ParseRule("- **Running the suite**: `go test ./... -race` (×3, 2026-10-15)")
	if !ok || r.Seen != 3 || !r.HasProv || r.Pinned || r.Last.Format("2006-01-02") != "2026-10-15" {
		t.Fatalf("%+v ok=%v", r, ok)
	}
	if r.Text != "`go test ./... -race`" {
		t.Fatalf("provenance not stripped: %q", r.Text)
	}
	if r, _ := ParseRule("- **x**: y (x3, 2026-10-15)"); r.Seen != 3 {
		t.Fatalf("ASCII x counter: %+v", r)
	}
	if r, _ := ParseRule("- **x**: y (user, 2026-10-15)"); !r.Pinned {
		t.Fatal("user rule must be pinned")
	}
	if r, _ := ParseRule("- **x**: y"); r.HasProv || r.Pinned || r.Seen != 0 {
		t.Fatalf("no provenance: %+v", r)
	}
	if r, _ := ParseRule("- **x**: y (×1, 2026-13-45)"); r.HasProv {
		t.Fatal("an undatable provenance is no provenance")
	}
	if _, ok := ParseRule("* not a rule"); ok {
		t.Fatal("a bullet is not a rule")
	}
	if r, _ := ParseRule("- **x**: y (×1, 2026-10-18)"); r.Seen != 1 || r.Text != "y" {
		t.Fatalf("%+v", r)
	}
}

func TestValidateSkill(t *testing.T) {
	good := autoSkill("moca-testing", "How the Go suite must be run.", "- **Running the suite**: `go test ./... -race -count=1` (×1, 2026-10-18)\n")
	if err := ValidateSkill(good, wc("moca-testing")); err != nil {
		t.Fatalf("a conforming new auto skill must pass: %v", err)
	}
}

func TestValidateSkillRefusals(t *testing.T) {
	tests := []struct {
		name, data, dir string
		want            []string
		prev            []byte
	}{
		{name: "no frontmatter", data: "# hi\n", dir: "moca-x", want: []string{"refused:", "--- block"}},
		{name: "no description", data: "---\nname: moca-x\nauto: true\n---\n- **a**: b (×1, 2026-10-18)\n", dir: "moca-x", want: []string{"no description"}},
		{name: "long description", data: "---\nname: moca-x\ndescription: " + strings.Repeat("d", 200) + "\nauto: true\n---\n- **a**: b (×1, 2026-10-18)\n", dir: "moca-x", want: []string{"description is 200 chars", "cap 160"}},
		{name: "credential", data: "---\nname: moca-x\ndescription: d\nauto: true\n---\n- **token**: use ghp_abcdefgh (×1, 2026-10-18)\n", dir: "moca-x", want: []string{"looks like a credential", "credential token"}},
		{name: "non-rule line", data: "---\nname: moca-x\ndescription: d\nauto: true\n---\n# H\nthe suite is slow\n", dir: "moca-x", want: []string{"line 7 is not a rule"}},
		{name: "drops the marker", data: "---\nname: moca-x\ndescription: d\n---\n- **a**: b (×1, 2026-10-18)\n", dir: "moca-x",
			prev: autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-18)\n"), want: []string{"keep auto: true"}},
		{name: "new not auto", data: "---\nname: moca-x\ndescription: d\n---\n- **a**: b (×1, 2026-10-18)\n", dir: "moca-x", want: []string{"must be an auto skill"}},
		{name: "bad name", data: string(autoSkill("testing", "d", "- **a**: b (×1, 2026-10-18)\n")), dir: "moca-testing", want: []string{"named moca-<domain>", "directory name"}},
		{name: "name != dir", data: string(autoSkill("moca-other", "d", "- **a**: b (×1, 2026-10-18)\n")), dir: "moca-testing", want: []string{"directory name"}},
		{name: "no rule", data: string(autoSkill("moca-x", "d", "# Only a heading\n")), dir: "moca-x", want: []string{"needs at least one rule line"}},
		{name: "new rule without provenance", data: string(autoSkill("moca-x", "d", "- **a**: b\n")), dir: "moca-x", want: []string{"a new rule starts at (×1, 2026-10-18)"}},
		{name: "new rule at x2", data: string(autoSkill("moca-x", "d", "- **a**: b (×2, 2026-10-18)\n")), dir: "moca-x", want: []string{"a new rule starts at (×1, 2026-10-18)"}},
		{name: "future date", data: string(autoSkill("moca-x", "d", "- **a**: b (×1, 2099-01-01)\n")), dir: "moca-x", want: []string{"is not today (2026-10-18)"}},
		{name: "stale date", data: string(autoSkill("moca-x", "d", "- **a**: b (×1, 2026-01-01)\n")), dir: "moca-x", want: []string{"is not today"}},
		{name: "new user rule without a signal", data: string(autoSkill("moca-x", "d", "- **a**: b (user, 2026-10-18)\n")), dir: "moca-x", want: []string{"via /remember"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := wc(tc.dir)
			c.Prev = tc.prev
			err := ValidateSkill([]byte(tc.data), c)
			if err == nil {
				t.Fatalf("want a refusal, got nil")
			}
			msg := err.Error()
			if !strings.HasPrefix(msg, "refused: ") {
				t.Fatalf("every refusal starts with \"refused: \": %q", msg)
			}
			for _, w := range tc.want {
				if !strings.Contains(msg, w) {
					t.Fatalf("message %q does not contain %q", msg, w)
				}
			}
		})
	}
}

func TestValidateSkillLengthCaps(t *testing.T) {
	// A rule line over 240 runes, a <when> over 80.
	long := "- **" + strings.Repeat("w", 90) + "**: " + strings.Repeat("r", 200) + " (×1, 2026-10-18)\n"
	err := ValidateSkill(autoSkill("moca-x", "d", long), wc("moca-x"))
	if err == nil || !strings.Contains(err.Error(), "cap 240") {
		t.Fatalf("want the rule-line cap, got %v", err)
	}
	longWhen := "- **" + strings.Repeat("w", 90) + "**: short (×1, 2026-10-18)\n"
	err = ValidateSkill(autoSkill("moca-x", "d", longWhen), wc("moca-x"))
	if err == nil || !strings.Contains(err.Error(), "cap 80") {
		t.Fatalf("want the <when> cap, got %v", err)
	}
	// A body of many short rule lines whose byte length exceeds the cap while
	// its rune count stays under it (× is two bytes): the cap is in runes.
	var sb strings.Builder
	for i := 0; i < 9; i++ {
		sb.WriteString("- **rule" + string(rune('a'+i)) + "**: " + strings.Repeat("×", 120) + " (×1, 2026-10-18)\n")
	}
	runeBody := sb.String()
	if n := utf8len(runeBody); n > SkillBodyCap {
		t.Fatalf("fixture too long: %d runes", n)
	}
	if len(runeBody) <= SkillBodyCap {
		t.Fatalf("fixture must exceed the cap in BYTES to pin the rune count: %d", len(runeBody))
	}
	if err := ValidateSkill(autoSkill("moca-x", "d", runeBody), wc("moca-x")); err != nil {
		t.Fatalf("a body under the cap in runes must pass: %v", err)
	}
	// The BODY cap, not the line cap: one 1526-rune line would trip the 240
	// per-line cap first (checked first) and prove nothing about SkillBodyCap.
	var ob strings.Builder
	for i := 0; i < 12; i++ {
		ob.WriteString("- **rule" + string(rune('a'+i)) + "**: " + strings.Repeat("×", 180) + " (×1, 2026-10-18)\n")
	}
	over := ob.String()
	if n := utf8len(over); n <= SkillBodyCap {
		t.Fatalf("fixture too short: %d runes", n)
	}
	err = ValidateSkill(autoSkill("moca-x", "d", over), wc("moca-x"))
	if err == nil || !strings.Contains(err.Error(), "cap 1500") {
		t.Fatalf("want the body cap, got %v", err)
	}
}

func utf8len(s string) int { return len([]rune(s)) }

func TestValidateSkillUpdatesAndTransitions(t *testing.T) {
	prev := autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-15)\n")
	// ×1 → ×2 with a later date passes
	next := autoSkill("moca-x", "d", "- **a**: b (×2, 2026-10-18)\n")
	c := wc("moca-x")
	c.Prev = prev
	if err := ValidateSkill(next, c); err != nil {
		t.Fatalf("×1→×2 must pass: %v", err)
	}
	// ×2 → ×3 is refused
	prev2 := autoSkill("moca-x", "d", "- **a**: b (×2, 2026-10-15)\n")
	c.Prev = prev2
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (×3, 2026-10-18)\n"), c); err == nil || !strings.Contains(err.Error(), "never changes") {
		t.Fatalf("×2 is final: %v", err)
	}
	// ×1 → ×2 with an earlier date is refused
	c.Prev = autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-18)\n")
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (×2, 2026-10-15)\n"), c); err == nil {
		t.Fatal("a bump must move the date forward")
	}
	// ×1 → ×2 observed the SAME day (two sessions on one day) passes: the
	// provenance counts sessions, not dates — the bump date may equal the
	// stored one, never precede it.
	c.Prev = autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-18)\n")
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (×2, 2026-10-18)\n"), c); err != nil {
		t.Fatalf("×1→×2 on the same day must pass: %v", err)
	}
	// the bump date is the host clock's, like a new rule's: a future date
	// (or any day the session cannot be on) is refused
	c.Prev = prev
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (×2, 2099-12-31)\n"), c); err == nil || !strings.Contains(err.Error(), "not today") {
		t.Fatalf("a bump to a future date must be refused: %v", err)
	}
	// a changed text is a new rule: back to ×1, or refused at ×2
	c.Prev = prev
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: c (×2, 2026-10-18)\n"), c); err == nil || !strings.Contains(err.Error(), "a new rule starts") {
		t.Fatalf("changed text at ×2 must be refused as a new rule: %v", err)
	}
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: c (×1, 2026-10-18)\n"), c); err != nil {
		t.Fatalf("changed text at ×1 must pass: %v", err)
	}
	// an existing user line passes without the signal
	c.Prev = autoSkill("moca-x", "d", "- **a**: b (user, 2026-10-01)\n")
	if err := ValidateSkill(c.Prev, c); err != nil {
		t.Fatalf("an existing user line must not be re-checked: %v", err)
	}
	// a new user line needs the signal
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (user, 2026-10-18)\n"), c); err == nil {
		t.Fatal("a new user line needs /remember")
	}
	c.UserSignal = true
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **b**: new rule (user, 2026-10-18)\n"), c); err != nil {
		t.Fatalf("a new user line with the signal must pass: %v", err)
	}
}

func TestValidateSkillSessionDate(t *testing.T) {
	c := WriteContext{DirName: "moca-x", Today: testToday, SessionDate: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	if err := ValidateSkill(autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-14)\n"), c); err != nil {
		t.Fatalf("a long session's start date must be accepted: %v", err)
	}
}

func TestValidateSkillCapsCheckChanges(t *testing.T) {
	// An over-cap body may shrink; it may not grow. The shrinking edit
	// deletes the bloated rule and leaves the other one untouched.
	big := strings.Repeat("x", 2000)
	keep := "- **b**: keep me (user, 2026-10-01)\n"
	prev := autoSkill("moca-x", "d", "- **a**: "+big+" (user, 2026-10-01)\n"+keep)
	shrunk := autoSkill("moca-x", "d", keep)
	grown := autoSkill("moca-x", "d", "- **a**: "+big+" (user, 2026-10-01)\n"+keep+"- **c**: new rule (×1, 2026-10-18)\n")
	c := wc("moca-x")
	c.Prev = prev
	if err := ValidateSkill(shrunk, c); err != nil {
		t.Fatalf("a shrinking edit of an over-cap skill must pass: %v", err)
	}
	if err := ValidateSkill(grown, c); err == nil {
		t.Fatal("a growing edit of an over-cap skill must be refused")
	}
	// the same holds for the rule count: 13 rules shrinking to 12 passes.
	rule := func(i int) string { return "- **r" + string(rune('a'+i)) + "**: x (×1, 2026-10-01)\n" }
	var many strings.Builder
	for i := 0; i < 13; i++ {
		many.WriteString(rule(i))
	}
	prevMany := autoSkill("moca-x", "d", many.String())
	c.Prev = prevMany
	// A shrinking edit drops the last line and keeps the rest unchanged.
	cut := strings.TrimSuffix(many.String(), rule(12))
	if err := ValidateSkill(autoSkill("moca-x", "d", cut), c); err != nil {
		t.Fatalf("a shrinking edit of an over-count skill must pass: %v", err)
	}
}

func TestUpdateAllowedWhenRepoOverCountCap(t *testing.T) {
	// Nine other auto skills exist: an update to an existing one still lands.
	prev := autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-15)\n- **c**: d (×1, 2026-10-15)\n")
	c := wc("moca-x")
	c.Prev = prev
	names := []string{"moca-a", "moca-b", "moca-c", "moca-d", "moca-e", "moca-f", "moca-g", "moca-h", "moca-i"}
	c.Others = AutoCount{Auto: 9, Names: names}
	next := autoSkill("moca-x", "d", "- **a**: b (×1, 2026-10-15)\n")
	if err := ValidateSkill(next, c); err != nil {
		t.Fatalf("an edit of an existing skill in an over-cap repo must pass: %v", err)
	}
}

func TestCountCap(t *testing.T) {
	body := "- **a**: b (×1, 2026-10-18)\n"
	c := wc("moca-x")
	c.Others = AutoCount{Auto: 8, Names: []string{"moca-1", "moca-2", "moca-3", "moca-4", "moca-5", "moca-6", "moca-7", "moca-8"}}
	err := ValidateSkill(autoSkill("moca-x", "d", body), c)
	if err == nil {
		t.Fatal("the 9th auto skill must be refused")
	}
	if !strings.Contains(err.Error(), "already has 8 auto skills") || !strings.Contains(err.Error(), "moca-1") {
		t.Fatalf("message: %v", err)
	}
	// An emptied auto skill frees a slot.
	c.Prev = autoSkill("moca-x", "d", body)
	if err := ValidateSkill(autoSkill("moca-x", "d", "# Empty\n"), c); err != nil {
		t.Fatalf("emptying an existing skill must pass at the cap: %v", err)
	}
}

func TestLongUntouchedLinePasses(t *testing.T) {
	longWhen := strings.Repeat("w", 100)
	prev := autoSkill("moca-x", "d", "- **"+longWhen+"**: short (user, 2026-10-01)\n")
	c := wc("moca-x")
	c.Prev = prev
	// Add a conforming rule; the long untouched one passes.
	next := autoSkill("moca-x", "d", "- **"+longWhen+"**: short (user, 2026-10-01)\n- **b**: c (×1, 2026-10-18)\n")
	if err := ValidateSkill(next, c); err != nil {
		t.Fatalf("an untouched over-cap line must not block an edit: %v", err)
	}
}

func TestSecretScanOnlyNewLines(t *testing.T) {
	prev := autoSkill("moca-x", "d", "- **db**: DSN is password=hunter2 in .env (user, 2026-10-01)\n")
	c := wc("moca-x")
	c.Prev = prev
	next := autoSkill("moca-x", "d", "- **db**: DSN is password=hunter2 in .env (user, 2026-10-01)\n- **b**: c (×1, 2026-10-18)\n")
	if err := ValidateSkill(next, c); err != nil {
		t.Fatalf("a documented secret on an untouched line must not block: %v", err)
	}
	if _, ok := secret.Match("password=hunter2"); !ok {
		t.Fatal("fixture sanity: the line must look like a secret")
	}
}

func TestHandWrittenNeedsNaming(t *testing.T) {
	prev := plainSkill("deploy", strings.Repeat("d", 200), strings.Repeat("prose\n", 700))
	c := wc("deploy")
	c.Prev = prev
	// Unnamed: refused.
	if err := ValidateSkill(append(prev, []byte("\nmore\n")...), c); err == nil || !strings.Contains(err.Error(), "hand-written skill") {
		t.Fatalf("an unnamed hand-written edit must be refused: %v", err)
	}
	// Named: the same edit passes — no size cap on the user's own file.
	c.Named = true
	if err := ValidateSkill(append(prev, []byte("\nmore\n")...), c); err != nil {
		t.Fatalf("a named hand-written edit must pass: %v", err)
	}
	// A named edit still scans new lines for credentials.
	if err := ValidateSkill(append(prev, []byte("\nghp_abcdefgh\n")...), c); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("a named edit must still refuse a credential: %v", err)
	}
	// A hand-written skill cannot become auto.
	c.Named = true
	autoData := []byte("---\nname: deploy\ndescription: d\nauto: true\n---\n")
	if err := ValidateSkill(autoData, c); err == nil || !strings.Contains(err.Error(), "leave its frontmatter") {
		t.Fatalf("a hand-written skill must stay hand-written: %v", err)
	}
}

func TestNamed(t *testing.T) {
	if !Named([]string{"fix the Deploy skill"}, "deploy", "deploy") {
		t.Fatal("a case-insensitive whole-word match must pass")
	}
	if !Named([]string{".moca/skills/deploy/SKILL.md"}, "deploy", "deploy") {
		t.Fatal("a path mention must pass")
	}
	if Named([]string{"fix the redeploy script"}, "deploy", "deploy") {
		t.Fatal("redeploy must not match deploy")
	}
	if Named([]string{"fix the deploy-tool"}, "deploy", "deploy") {
		t.Fatal("deploy-tool must not match deploy")
	}
	if Named(nil, "deploy", "deploy") {
		t.Fatal("empty typed must be false")
	}
	if !Named([]string{"update moca-testing"}, "moca-qa", "moca-testing") {
		t.Fatal("the directory name counts too")
	}
}

func TestCountAutoSkills(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		d := filepath.Join(dir, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	body := "- **a**: b (×1, 2026-10-18)\n"
	write("moca-one", autoSkill("moca-one", "d", body))
	write("moca-two", autoSkill("moca-two", "d", body))
	write("moca-empty", autoSkill("moca-empty", "d", "# heading\n"))
	write("hand", plainSkill("hand", "d", body))
	write("moca-noread", []byte("---\nname: moca-noread\n"))
	os.Remove(filepath.Join(dir, "moca-noread", "SKILL.md"))
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "moca-one"), filepath.Join(dir, "moca-link")); err != nil {
		t.Fatal(err)
	}
	got, err := CountAutoSkills(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Auto != 2 || len(got.Names) != 2 {
		t.Fatalf("count = %+v, want 2", got)
	}
	// The target is excluded by DIRECTORY NAME, case-insensitively (H11).
	got, err = CountAutoSkills(dir, "MOCA-ONE")
	if err != nil || got.Auto != 1 || got.Names[0] != "moca-two" {
		t.Fatalf("exclude by name: %+v %v", got, err)
	}
	// A missing dir is zero, not an error.
	if got, err := CountAutoSkills(filepath.Join(dir, "absent"), ""); err != nil || got.Auto != 0 {
		t.Fatalf("missing dir: %+v %v", got, err)
	}
}

func TestValidateStoredDropsBrokenAutoSkills(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string, extra ...string) {
		d := filepath.Join(dir, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, e := range extra {
			if err := os.WriteFile(filepath.Join(d, e), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	body := "- **a**: b (×2, 2026-01-01)\n" // an old date is fine at load
	write("moca-ok", string(autoSkill("moca-ok", "d", body)))
	write("moca-cred", string(autoSkill("moca-cred", "d", "- **a**: ghp_abcdefgh (×2, 2026-01-01)\n")))
	write("moca-big", string(autoSkill("moca-big", "d", "- **a**: "+strings.Repeat("x", 2000)+" (×2, 2026-01-01)\n")))
	write("moca-name", string(autoSkill("moca-other", "d", body)))
	write("moca-extra", string(autoSkill("moca-extra", "d", body)), "references.md")
	write("moca-prose", string(autoSkill("moca-prose", "d", "just prose\n")))
	for name, wantErr := range map[string]bool{
		"moca-ok": false, "moca-cred": true, "moca-big": true, "moca-name": true, "moca-extra": true, "moca-prose": true,
	} {
		err := ValidateStored(dir, name)
		if wantErr && err == nil {
			t.Errorf("%s: want a refusal", name)
		}
		if !wantErr && err != nil {
			t.Errorf("%s: unexpected refusal: %v", name, err)
		}
	}
}

func TestDiscoverSetsAuto(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	global := filepath.Join(root, "global")
	builtin := filepath.Join(root, "builtin")
	for _, d := range []string{project, global, builtin} {
		if err := os.MkdirAll(filepath.Join(d, "moca-x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "moca-x", "SKILL.md"),
			autoSkill("moca-x", "d", "- **a**: b (×2, 2026-01-01)\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sk, errs := Discover([]Dir{{Path: project, Source: "project"}, {Path: global, Source: "global"}, {Path: builtin, Source: "builtin"}})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(sk) != 1 || !sk[0].Auto || sk[0].Source != "project" {
		t.Fatalf("first source wins, and only it: %+v", sk)
	}
	// The identical global file alone: listed, Auto false.
	sk, errs = Discover([]Dir{{Path: global, Source: "global"}})
	if len(sk) != 1 || sk[0].Auto {
		t.Fatalf("auto only applies to the project source: %+v %v", sk, errs)
	}
	// A broken project auto skill is dropped with its reason; the global copy
	// with the same name is then listed untouched.
	if err := os.WriteFile(filepath.Join(project, "moca-x", "SKILL.md"),
		autoSkill("moca-x", "d", "- **a**: ghp_abcdefgh (×2, 2026-01-01)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sk, errs = Discover([]Dir{{Path: project, Source: "project"}, {Path: global, Source: "global"}})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "credential") {
		t.Fatalf("the broken skill must report one reason: %v", errs)
	}
	if len(sk) != 1 || sk[0].Source != "global" || sk[0].Auto {
		t.Fatalf("the broken project copy must be dropped and the global one listed: %+v", sk)
	}
}

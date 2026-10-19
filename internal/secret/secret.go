// Package secret recognises credential-shaped strings. It is a leaf: it
// imports nothing internal, so the MCP importer and the skill write guard
// share one table (SPECS §2/§11).
package secret

import (
	"regexp"
	"strings"
)

var (
	// secretKey is the key-name table: a config/env/header key that usually
	// carries a credential.
	secretKey = regexp.MustCompile(`(?i)key|token|secret|password|auth`)
	// secretValue anchors a whole value that is a credential.
	secretValue = regexp.MustCompile(`^(?:(?:Bearer|Basic) \S+|(?:sk-|ghp_|gho_|github_pat_|glpat-|xox[abp]-|AKIA)\S+)`)
	// userinfo matches URL credentials like https://user:token@host/mcp.
	userinfo = regexp.MustCompile(`://[^/@\s:]+:[^/@\s]+@`)
	// passwordKV matches key=value connection-string secrets ("password=…").
	passwordKV = regexp.MustCompile(`(?i)\b(?:password|passwd|pwd)\s*=\s*\S+`)
	// tokenish matches credential-shaped tokens anywhere in a value, with a
	// length floor so benign strings ("sk-1") do not trip it: the same
	// prefixes as secretValue plus scheme-style header values.
	tokenish = regexp.MustCompile(`(?:(?:sk-|ghp_|gho_|github_pat_|glpat-|xox[abp]-|AKIA)[A-Za-z0-9_-]{8,}|(?:Bearer|Basic)\s+\S{8,})`)
)

// Match's skill-only patterns: shapes the importer never needed to know.
var (
	pemKey   = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
	jwtRe    = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{6,}\.eyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}`)
	stripeRe = regexp.MustCompile(`(?:sk|rk)_live_[A-Za-z0-9]{8,}`)
	googleRe = regexp.MustCompile(`AIza[A-Za-z0-9_-]{20,}`)
	npmRe    = regexp.MustCompile(`npm_[A-Za-z0-9]{20,}`)
	pypiRe   = regexp.MustCompile(`pypi-[A-Za-z0-9_-]{20,}`)
	slackRe  = regexp.MustCompile(`xoxe-[A-Za-z0-9-]{8,}`)
	webhook  = regexp.MustCompile(`hooks\.slack\.com/services/[A-Za-z0-9/]+`)
)

// varName matches a bare environment-variable name.
var varName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Value reports whether s is a credential value — a whole-string anchor:
// the importer's `secretValue` regex, moved verbatim. No length floor:
// Value("sk-1") is true (the importer only applies it to env/header values).
func Value(s string) bool { return secretValue.MatchString(s) }

// Tokenish reports whether s contains a credential-shaped token anywhere —
// the importer's `tokenish`, moved verbatim (its 8-char floor keeps "sk-1" out).
func Tokenish(s string) bool { return tokenish.MatchString(s) }

// Key reports whether a config/env key name is credential-bearing — the
// importer's `secretKey`, moved verbatim. Deliberately broad: "authorization"
// is (correctly) true, and so is "monkey" (a known, pinned false positive).
func Key(name string) bool { return secretKey.MatchString(name) }

// Userinfo reports URL credentials (scheme://user:secret@host) — the
// importer's `userinfo`, moved verbatim.
func Userinfo(s string) bool { return userinfo.MatchString(s) }

// Embedded reports a credential inside free text: Userinfo, Tokenish, or a
// password=… pair (`passwordKV`, moved verbatim) — the importer's
// `embeddedSecret`, moved here whole.
func Embedded(s string) bool {
	return userinfo.MatchString(s) || tokenish.MatchString(s) || passwordKV.MatchString(s)
}

// Match is the skill guard's scanner: Embedded's three classes plus
// skill-only patterns — PEM private-key blocks, JWTs (three eyJ segments),
// Stripe sk_live_/rk_live_, Google AIza…, npm npm_…, PyPI pypi-…, Slack
// xoxe-… and hooks.slack.com/services/… — with one exclusion the importer
// must never learn: a matched value that is only a variable reference
// (`${VAR}`, `$VAR`, `env:VAR`, `$env:VAR`, `%VAR%`, optionally closed by a
// quote or backtick) is not a credential (H10) — nor is URL userinfo whose
// password part is such a reference, nor a Bearer/Basic match whose value is
// a plain word (schemeProse). ok is false for none.
// The token classes are prefix-shaped (sk-, ghp_, AIza, npm_, …) and must
// start a word: unanchored, `sk-` matches inside prose — "task-scheduler",
// "risk-assessment" — and a skill naming a hyphenated target would be refused
// as a credential. The importer's Tokenish stays unanchored (moved verbatim).
// The refusal quotes the class.
func Match(s string) (class string, ok bool) {
	for _, c := range matchClasses {
		for _, loc := range c.re.FindAllStringIndex(s, -1) {
			if c.wordStart && loc[0] > 0 && wordChar(s[loc[0]-1]) {
				continue // the prefix sits inside a longer word
			}
			m := s[loc[0]:loc[1]]
			if onlyVarRef(m) {
				continue
			}
			if c.re == userinfo && onlyVarRef(userinfoSecret(m)) {
				continue // scheme://user:${PASS}@host names the variable, not the secret
			}
			if c.re == tokenish && schemeProse(m) {
				continue // "Basic validation …" is prose, not a scheme credential
			}
			return c.name, true
		}
	}
	return "", false
}

// matchClasses is Match's table, built once.
var matchClasses = []struct {
	name      string
	re        *regexp.Regexp
	wordStart bool // the match must not continue a word (prefix-shaped tokens)
}{
	{"URL credentials", userinfo, false},
	{"credential token", tokenish, true},
	{"password", passwordKV, false},
	{"private key", pemKey, false},
	{"JWT", jwtRe, true},
	{"Stripe key", stripeRe, true},
	{"Google API key", googleRe, true},
	{"npm token", npmRe, true},
	{"PyPI token", pypiRe, true},
	{"Slack token", slackRe, true},
	{"Slack webhook", webhook, false},
}

// userinfoSecret is the secret part of a userinfo match ("://user:pw@" → "pw").
// The user part cannot hold a ':' (userinfo excludes it), so the first one
// splits — the secret may itself contain one ($env:PASS).
func userinfoSecret(m string) string {
	_, pw, _ := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(m, "://"), "@"), ":")
	return pw
}

// schemeProse reports a Bearer/Basic match whose value is an ordinary word
// ("Basic validation", "Bearer authentication-free"): a real scheme credential
// is base64 or a random token — it carries a digit, a +/= character, or mixed
// case past its first letter. A prefix token after the scheme ("Bearer
// sk-…") is never prose: the scheme match consumed it (FindAll matches do
// not overlap), so this is its only check.
func schemeProse(m string) bool {
	f := strings.Fields(m)
	if len(f) != 2 || (f[0] != "Bearer" && f[0] != "Basic") || tokenish.MatchString(f[1]) {
		return false
	}
	upper, lower := false, false
	for i, c := range f[1] {
		switch {
		case c >= '0' && c <= '9', c == '+', c == '/', c == '=':
			return false
		case c >= 'A' && c <= 'Z':
			upper = upper || i > 0
		case c >= 'a' && c <= 'z':
			lower = true
		}
	}
	return !(upper && lower)
}

// wordChar is an ASCII letter or digit: what a token prefix must not follow.
func wordChar(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// onlyVarRef reports whether a matched value is nothing but an environment
// variable reference — a skill may name the variable a secret lives in.
func onlyVarRef(m string) bool {
	s := strings.TrimSpace(m)
	s = strings.Trim(s, "\"'`")
	// A scheme word ("Bearer ${TOKEN}") or a key prefix ("password=$DB_PASS")
	// is not the value.
	if f := strings.Fields(s); len(f) == 2 && (strings.EqualFold(f[0], "bearer") || strings.EqualFold(f[0], "basic")) {
		s = f[1]
	}
	if i := strings.IndexByte(s, '='); i >= 0 {
		s = strings.TrimSpace(s[i+1:])
	}
	s = strings.Trim(s, "\"'`")
	var name string
	switch {
	case strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}"):
		name = s[2 : len(s)-1]
	case strings.HasPrefix(s, "$env:"):
		name = strings.TrimPrefix(s, "$env:")
	case strings.HasPrefix(s, "env:"):
		name = strings.TrimPrefix(s, "env:")
	case len(s) >= 3 && strings.HasPrefix(s, "%") && strings.HasSuffix(s, "%"):
		name = s[1 : len(s)-1]
	case strings.HasPrefix(s, "$"):
		name = s[1:]
	default:
		// A bare word is a value, not a reference ("password=hunter2").
		return false
	}
	return varName.MatchString(name)
}

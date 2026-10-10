package applog

import (
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// errMax caps error text (vendor errors can echo request details).
const errMax = 500

var secretWords = map[string]bool{"key": true, "apikey": true, "token": true, "secret": true,
	"password": true, "authorization": true, "cookie": true}

// replaceAttr is the handler's safety net. The policy is metadata-only call
// sites; this catches mistakes: secret-named attributes, URLs carrying
// credentials or queries, unbounded error text.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if isSecretKey(groups, a.Key) {
		return slog.String(a.Key, "[redacted]")
	}
	if a.Value.Kind() != slog.KindAny {
		return a
	}
	switch v := a.Value.Any().(type) {
	case *url.URL:
		if v != nil {
			return slog.String(a.Key, safeURL(v))
		}
	case url.URL:
		return slog.String(a.Key, safeURL(&v))
	case error:
		return slog.String(a.Key, cutString(v.Error(), errMax))
	}
	return a
}

// isSecretKey matches whole segments of the key and its groups, so
// "tokens_before" stays readable while "api_key" and "oauthToken" do not.
func isSecretKey(groups []string, key string) bool {
	for _, part := range append(slices.Clone(groups), key) {
		for _, seg := range segments(part) {
			if secretWords[seg] {
				return true
			}
		}
	}
	return false
}

// segments splits a key on '_', '-', '.' and lower→upper case changes,
// lower-cased: "oauthToken" → oauth, token; "APIKey" → apikey.
func segments(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	prevLower := false
	for _, r := range s {
		if r == '_' || r == '-' || r == '.' {
			flush()
			prevLower = false
			continue
		}
		if unicode.IsUpper(r) && prevLower {
			flush()
		}
		cur = append(cur, r)
		prevLower = unicode.IsLower(r) || unicode.IsDigit(r)
	}
	flush()
	return out
}

// safeURL keeps scheme, host and path: no userinfo, query or fragment.
func safeURL(u *url.URL) string {
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}

// cutString trims s to at most max bytes on a rune boundary.
func cutString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}

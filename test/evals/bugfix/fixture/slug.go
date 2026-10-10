// Package slug builds URL slugs from titles.
package slug

import "strings"

// Make turns a title into a URL slug: lowercase ASCII letters and digits,
// words joined by single hyphens, no leading or trailing hyphen.
func Make(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	return b.String()
}

// Limit shortens a slug to at most n bytes, cutting at a hyphen so no word
// is split; the result never ends with a hyphen. A slug whose first word is
// longer than n is cut hard at n.
func Limit(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := strings.LastIndexByte(s[:n], '-')
	if cut <= 0 {
		return s[:n]
	}
	return s[:cut+1]
}

package shop

import (
	"errors"
	"strings"
)

// normalizeEmail trims and lowercases email and checks its shape.
func normalizeEmail(email string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndexByte(e, '@')
	if at < 1 || at == len(e)-1 {
		return "", errors.New("invalid email: " + e)
	}
	domain := e[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "", errors.New("invalid email: " + e)
	}
	if strings.ContainsAny(e, " \t,;") {
		return "", errors.New("invalid email: " + e)
	}
	return e, nil
}

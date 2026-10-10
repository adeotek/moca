// Package shop holds the account and order models.
package shop

import (
	"errors"
	"strings"
)

type User struct {
	Name  string
	Email string
}

// NewUser validates and normalizes a user.
func NewUser(name, email string) (User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return User{}, errors.New("name is required")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndexByte(email, '@')
	if at < 1 || at == len(email)-1 {
		return User{}, errors.New("invalid email: " + email)
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return User{}, errors.New("invalid email: " + email)
	}
	if strings.ContainsAny(email, " \t,;") {
		return User{}, errors.New("invalid email: " + email)
	}
	return User{Name: name, Email: email}, nil
}
